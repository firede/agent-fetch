package main

import (
	"context"
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
