package main

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/firede/agent-fetch/internal/fetcher"
)

func TestWriteBatchMarkdown(t *testing.T) {
	results := []taskResult{
		{
			index:    1,
			inputURL: "https://example.com/hello",
			markdown: "# hello\n",
		},
		{
			index:    2,
			inputURL: "https://abc.com",
			err:      errors.New("http request failed: timeout"),
		},
		{
			index:    3,
			inputURL: "https://example.net/hi",
			markdown: "hi",
		},
	}

	var b strings.Builder
	if err := writeBatchMarkdown(&b, results); err != nil {
		t.Fatalf("write batch markdown: %v", err)
	}

	got := b.String()
	want := strings.Join([]string{
		"<!-- count: 3, succeeded: 2, failed: 1 -->",
		"<!-- task[1]: https://example.com/hello -->",
		"# hello",
		"<!-- /task[1] -->",
		"",
		"<!-- task[2](failed): https://abc.com -->",
		"<!-- error[2]: http request failed: timeout -->",
		"",
		"<!-- task[3]: https://example.net/hi -->",
		"hi",
		"<!-- /task[3] -->",
		"",
	}, "\n")

	if got != want {
		t.Fatalf("unexpected output\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

func TestWriteBatchJSONL(t *testing.T) {
	results := []taskResult{
		{
			index:    1,
			inputURL: "https://example.com/hello",
			finalURL: "https://example.com/final",
			source:   "http-static",
			markdown: "---\n" +
				"title: 'Hello'\n" +
				"description: 'World'\n" +
				"---\n\n" +
				"# hello\n",
		},
		{
			index:    2,
			inputURL: "https://abc.com",
			err:      errors.New("http request failed: timeout"),
		},
	}

	var b strings.Builder
	if err := writeBatchJSONL(&b, results, true); err != nil {
		t.Fatalf("write batch jsonl: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(b.String()), "\n")
	if len(lines) != 2 {
		t.Fatalf("unexpected line count: %d (%q)", len(lines), b.String())
	}

	var first map[string]any
	if err := json.Unmarshal([]byte(lines[0]), &first); err != nil {
		t.Fatalf("unmarshal first line: %v", err)
	}
	if first["seq"] != float64(1) {
		t.Fatalf("unexpected seq: %v", first["seq"])
	}
	if first["url"] != "https://example.com/hello" {
		t.Fatalf("unexpected url: %v", first["url"])
	}
	if first["resolved_url"] != "https://example.com/final" {
		t.Fatalf("unexpected resolved_url: %v", first["resolved_url"])
	}
	if first["resolved_mode"] != "static" {
		t.Fatalf("unexpected resolved_mode: %v", first["resolved_mode"])
	}
	if first["content"] != "# hello\n" {
		t.Fatalf("unexpected content: %q", first["content"])
	}
	meta, ok := first["meta"].(map[string]any)
	if !ok {
		t.Fatalf("unexpected meta payload: %#v", first["meta"])
	}
	if meta["title"] != "Hello" || meta["description"] != "World" {
		t.Fatalf("unexpected meta: %#v", meta)
	}

	var second map[string]any
	if err := json.Unmarshal([]byte(lines[1]), &second); err != nil {
		t.Fatalf("unmarshal second line: %v", err)
	}
	if second["seq"] != float64(2) {
		t.Fatalf("unexpected seq: %v", second["seq"])
	}
	if second["url"] != "https://abc.com" {
		t.Fatalf("unexpected url: %v", second["url"])
	}
	if second["error"] != "http request failed: timeout" {
		t.Fatalf("unexpected error: %v", second["error"])
	}
	if _, exists := second["resolved_mode"]; exists {
		t.Fatalf("unexpected resolved_mode in error payload: %#v", second)
	}
}

func TestWriteBatchJSONL_MetaDisabledKeepsFrontMatter(t *testing.T) {
	results := []taskResult{
		{
			index:    1,
			inputURL: "https://example.com/hello",
			source:   "http-markdown",
			markdown: "---\n" +
				"title: 'Hello'\n" +
				"---\n\n" +
				"# hello\n",
		},
	}

	var b strings.Builder
	if err := writeBatchJSONL(&b, results, false); err != nil {
		t.Fatalf("write batch jsonl: %v", err)
	}

	var row map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(b.String())), &row); err != nil {
		t.Fatalf("unmarshal row: %v", err)
	}
	if row["resolved_mode"] != "markdown" {
		t.Fatalf("unexpected resolved_mode: %v", row["resolved_mode"])
	}
	if !strings.HasPrefix(row["content"].(string), "---\n") {
		t.Fatalf("expected front matter preserved, got: %q", row["content"])
	}
	if _, exists := row["meta"]; exists {
		t.Fatalf("expected meta omitted when disabled, got: %#v", row["meta"])
	}
}

func TestExtractInjectableMeta_UnknownFieldsNotStripped(t *testing.T) {
	input := "---\n" +
		"title: 'Hello'\n" +
		"date: '2026-02-22'\n" +
		"---\n\n" +
		"Body\n"
	body, meta, ok := extractInjectableMeta(input)
	if ok {
		t.Fatalf("expected parse to be rejected for unknown keys: body=%q meta=%+v", body, meta)
	}
	if body != input {
		t.Fatalf("expected content unchanged, got: %q", body)
	}
}

func TestFetchBatchPreservesInputOrder(t *testing.T) {
	urls := []string{
		"https://example.com/1",
		"https://example.com/2",
		"https://example.com/3",
	}

	delayByURL := map[string]time.Duration{
		urls[0]: 50 * time.Millisecond,
		urls[1]: 5 * time.Millisecond,
		urls[2]: 25 * time.Millisecond,
	}

	fetch := func(ctx context.Context, url string, cfg fetcher.Config) (fetcher.Result, error) {
		select {
		case <-ctx.Done():
			return fetcher.Result{}, ctx.Err()
		case <-time.After(delayByURL[url]):
		}
		return fetcher.Result{Markdown: "content-" + url}, nil
	}

	cfg := fetcher.DefaultConfig()
	results := fetchBatch(context.Background(), urls, cfg, 3, fetch)
	if len(results) != 3 {
		t.Fatalf("unexpected result count: %d", len(results))
	}

	for i := range urls {
		if results[i].index != i+1 {
			t.Fatalf("unexpected index at %d: got %d want %d", i, results[i].index, i+1)
		}
		if results[i].inputURL != urls[i] {
			t.Fatalf("unexpected url at %d: got %q want %q", i, results[i].inputURL, urls[i])
		}
		if results[i].err != nil {
			t.Fatalf("unexpected error at %d: %v", i, results[i].err)
		}
	}
}
