package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/firede/agent-fetch/internal/fetcher"
)

type fetchFunc func(context.Context, string, fetcher.Config) (fetcher.Result, error)

type taskResult struct {
	index    int
	inputURL string
	finalURL string
	source   string
	markdown string
	err      error
}

func fetchBatch(ctx context.Context, urls []string, cfg fetcher.Config, concurrency int, fetch fetchFunc) []taskResult {
	if concurrency < 1 {
		concurrency = 1
	}

	results := make([]taskResult, len(urls))
	sem := make(chan struct{}, concurrency)

	var wg sync.WaitGroup
	for i, url := range urls {
		i, url := i, url
		wg.Add(1)
		go func() {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			reqCtx, cancel := context.WithTimeout(ctx, maxDuration(cfg.Timeout, cfg.BrowserTimeout)+5*time.Second)
			defer cancel()

			res, err := fetch(reqCtx, url, cfg)
			results[i] = taskResult{
				index:    i + 1,
				inputURL: url,
				finalURL: res.FinalURL,
				source:   res.Source,
				markdown: res.Markdown,
				err:      err,
			}
		}()
	}
	wg.Wait()

	return results
}

func writeBatchMarkdown(w io.Writer, results []taskResult) error {
	total := len(results)
	failed := failedCount(results)
	succeeded := total - failed

	if _, err := fmt.Fprintf(w, "<!-- count: %d, succeeded: %d, failed: %d -->\n", total, succeeded, failed); err != nil {
		return err
	}

	for i, result := range results {
		if i > 0 {
			if _, err := io.WriteString(w, "\n"); err != nil {
				return err
			}
		}

		url := sanitizeForComment(result.inputURL)
		if result.err != nil {
			if _, err := fmt.Fprintf(w, "<!-- task[%d](failed): %s -->\n", result.index, url); err != nil {
				return err
			}
			errMsg := sanitizeForComment(result.err.Error())
			if _, err := fmt.Fprintf(w, "<!-- error[%d]: %s -->\n", result.index, errMsg); err != nil {
				return err
			}
			continue
		}

		if _, err := fmt.Fprintf(w, "<!-- task[%d]: %s -->\n", result.index, url); err != nil {
			return err
		}
		if _, err := io.WriteString(w, result.markdown); err != nil {
			return err
		}
		if !strings.HasSuffix(result.markdown, "\n") {
			if _, err := io.WriteString(w, "\n"); err != nil {
				return err
			}
		}
		if _, err := fmt.Fprintf(w, "<!-- /task[%d] -->\n", result.index); err != nil {
			return err
		}
	}

	return nil
}

func failedCount(results []taskResult) int {
	n := 0
	for _, result := range results {
		if result.err != nil {
			n++
		}
	}
	return n
}

func sanitizeForComment(s string) string {
	s = strings.ReplaceAll(s, "\r", " ")
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "-->", "-- >")
	return strings.TrimSpace(s)
}

type jsonlMeta struct {
	Title       string `json:"title,omitempty"`
	Description string `json:"description,omitempty"`
}

type jsonlSuccessPayload struct {
	Seq          int        `json:"seq"`
	URL          string     `json:"url"`
	ResolvedURL  string     `json:"resolved_url,omitempty"`
	ResolvedMode string     `json:"resolved_mode"`
	Content      string     `json:"content"`
	Meta         *jsonlMeta `json:"meta,omitempty"`
}

type jsonlErrorPayload struct {
	Seq   int    `json:"seq"`
	URL   string `json:"url"`
	Error string `json:"error"`
}

func writeBatchJSONL(w io.Writer, results []taskResult, includeMeta bool) error {
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)

	for _, result := range results {
		if result.err != nil {
			payload := jsonlErrorPayload{
				Seq:   result.index,
				URL:   result.inputURL,
				Error: strings.TrimSpace(result.err.Error()),
			}
			if err := enc.Encode(payload); err != nil {
				return err
			}
			continue
		}

		content := result.markdown
		var meta *jsonlMeta
		if includeMeta {
			trimmed, extracted, ok := extractInjectableMeta(content)
			if ok {
				content = trimmed
				if extracted.Title != "" || extracted.Description != "" {
					meta = &extracted
				}
			}
		}

		payload := jsonlSuccessPayload{
			Seq:          result.index,
			URL:          result.inputURL,
			ResolvedMode: resolveMode(result.source),
			Content:      content,
			Meta:         meta,
		}
		if strings.TrimSpace(result.finalURL) != "" && result.finalURL != result.inputURL {
			payload.ResolvedURL = result.finalURL
		}
		if err := enc.Encode(payload); err != nil {
			return err
		}
	}

	return nil
}

func resolveMode(source string) string {
	switch strings.TrimSpace(source) {
	case "http-markdown":
		return "markdown"
	case "http-static":
		return "static"
	case "browser":
		return "browser"
	case "http-raw":
		return "raw"
	default:
		return strings.TrimSpace(source)
	}
}

func extractInjectableMeta(md string) (string, jsonlMeta, bool) {
	input := strings.TrimPrefix(md, "\ufeff")
	if !strings.HasPrefix(input, "---\n") && !strings.HasPrefix(input, "---\r\n") {
		return md, jsonlMeta{}, false
	}

	rest := strings.TrimPrefix(input, "---\n")
	if rest == input {
		rest = strings.TrimPrefix(input, "---\r\n")
	}

	lines := make([]string, 0, 8)
	for {
		line, tail, ok := nextLine(rest)
		if !ok {
			return md, jsonlMeta{}, false
		}
		trimmedLine := strings.TrimSpace(strings.TrimSuffix(line, "\r"))
		rest = tail
		if trimmedLine == "---" {
			break
		}
		lines = append(lines, line)
	}

	meta, safe := parseInjectableMeta(lines)
	if !safe {
		return md, jsonlMeta{}, false
	}

	body := rest
	if strings.HasPrefix(body, "\r\n") {
		body = body[2:]
	} else if strings.HasPrefix(body, "\n") {
		body = body[1:]
	}
	return body, meta, true
}

func parseInjectableMeta(lines []string) (jsonlMeta, bool) {
	meta := jsonlMeta{}
	knownFieldCount := 0

	for _, line := range lines {
		item := strings.TrimSpace(strings.TrimSuffix(line, "\r"))
		if item == "" || strings.HasPrefix(item, "#") {
			continue
		}

		key, val, ok := strings.Cut(item, ":")
		if !ok {
			return jsonlMeta{}, false
		}
		k := strings.ToLower(strings.TrimSpace(key))
		v := parseYAMLScalar(strings.TrimSpace(val))

		switch k {
		case "title":
			meta.Title = v
			knownFieldCount++
		case "description":
			meta.Description = v
			knownFieldCount++
		default:
			return jsonlMeta{}, false
		}
	}

	if knownFieldCount == 0 {
		return jsonlMeta{}, false
	}
	return meta, true
}

func parseYAMLScalar(v string) string {
	v = strings.TrimSpace(v)
	if len(v) >= 2 && strings.HasPrefix(v, "'") && strings.HasSuffix(v, "'") {
		return strings.ReplaceAll(v[1:len(v)-1], "''", "'")
	}
	if len(v) >= 2 && strings.HasPrefix(v, "\"") && strings.HasSuffix(v, "\"") {
		unquoted, err := strconv.Unquote(v)
		if err == nil {
			return unquoted
		}
	}
	return v
}

func nextLine(s string) (line string, tail string, ok bool) {
	if s == "" {
		return "", "", false
	}
	if idx := strings.IndexByte(s, '\n'); idx >= 0 {
		return s[:idx], s[idx+1:], true
	}
	return s, "", true
}
