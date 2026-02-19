package main

import (
	"context"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/firede/agent-fetch/internal/fetcher"
)

type fetchFunc func(context.Context, string, fetcher.Config) (fetcher.Result, error)

type taskResult struct {
	index    int
	inputURL string
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
