package fetcher

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestIsLikelyMarkdown(t *testing.T) {
	md := []byte("# Title\n\n- item\n\nThis is markdown.\n")
	html := []byte("<!doctype html><html><body><h1>Title</h1></body></html>")
	jsonPayload := []byte(`{"items":[{"id":1,"name":"alpha"},{"id":2,"name":"beta"},{"id":3,"name":"gamma"}],"meta":{"total":203,"note":"this payload is intentionally long enough to trigger old fallback behavior"}}`)

	if !isLikelyMarkdown(md, "text/plain") {
		t.Fatal("expected markdown sample to be detected as markdown")
	}
	if isLikelyMarkdown(html, "text/html") {
		t.Fatal("expected HTML sample to not be treated as markdown")
	}
	if isLikelyMarkdown(jsonPayload, "application/json") {
		t.Fatal("expected JSON sample to not be treated as markdown")
	}
}

func TestStaticHTMLToMarkdown(t *testing.T) {
	html := []byte(`<!doctype html>
<html><body>
<article>
  <h1>Alpha</h1>
  <p>This is a long enough paragraph for testing static conversion quality score.</p>
  <p>Another paragraph to avoid low-quality result.</p>
</article>
</body></html>`)

	md, ok, err := staticHTMLToMarkdown(html, "https://example.com/post", 20)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Fatal("expected quality check to pass")
	}
	if !strings.Contains(md, "Alpha") {
		t.Fatalf("expected converted markdown to contain title, got: %q", md)
	}
}

func TestFetchAutoUsesMarkdownWhenProvided(t *testing.T) {
	sawAcceptMarkdown := false
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.Header.Get("Accept"), "text/markdown") {
			sawAcceptMarkdown = true
		}
		w.Header().Set("Content-Type", "text/plain")
		fmt.Fprint(w, "# Direct Markdown\n\n- item one\n- item two\n\nRead [more](https://example.com).\n")
	}))
	defer ts.Close()

	cfg := DefaultConfig()
	cfg.Mode = ModeAuto
	cfg.Timeout = 5 * time.Second

	res, err := Fetch(context.Background(), ts.URL, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !sawAcceptMarkdown {
		t.Fatal("expected at least one request with Accept: text/markdown")
	}
	if res.Source != "http-markdown" {
		t.Fatalf("expected source http-markdown, got %q", res.Source)
	}
	if !strings.Contains(res.Markdown, "Direct Markdown") {
		t.Fatalf("unexpected markdown output: %q", res.Markdown)
	}
}

func TestFetchAutoRespectsTextMarkdownForMDXPayload(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/markdown; charset=utf-8")
		fmt.Fprint(w, `# Overview

export const LogoCarousel = () => {
  return <img src="/logo.svg" alt="logo" />;
};

## Why Agent Skills?

Agents are increasingly capable.
`)
	}))
	defer ts.Close()

	cfg := DefaultConfig()
	cfg.Mode = ModeAuto
	cfg.Timeout = 5 * time.Second

	res, err := Fetch(context.Background(), ts.URL, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Source != "http-markdown" {
		t.Fatalf("expected source http-markdown, got %q", res.Source)
	}
	if !strings.Contains(res.Markdown, "# Overview") {
		t.Fatalf("expected markdown to preserve overview heading, got: %q", res.Markdown)
	}
	if !strings.Contains(res.Markdown, "## Why Agent Skills?") {
		t.Fatalf("expected markdown to preserve why heading, got: %q", res.Markdown)
	}
}

func TestFetchStaticConvertsHTML(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, `<!doctype html>
<html><body>
<main>
  <h1>Static Page</h1>
  <p>This body should be extracted and converted into markdown with enough text to pass quality checks.</p>
  <p>Second paragraph with additional details for readability extraction.</p>
</main>
</body></html>`)
	}))
	defer ts.Close()

	cfg := DefaultConfig()
	cfg.Mode = ModeStatic
	cfg.Timeout = 5 * time.Second
	cfg.MinQualityText = 20

	res, err := Fetch(context.Background(), ts.URL, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Source != "http-static" {
		t.Fatalf("expected source http-static, got %q", res.Source)
	}
	if !strings.Contains(res.Markdown, "Static Page") {
		t.Fatalf("unexpected markdown output: %q", res.Markdown)
	}
}

func TestFetchReturnsErrorOnHTTPStatus(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprint(w, "<html><body><h1>Not Found</h1></body></html>")
	}))
	defer ts.Close()

	cfg := DefaultConfig()
	cfg.Mode = ModeStatic
	cfg.Timeout = 5 * time.Second

	_, err := Fetch(context.Background(), ts.URL, cfg)
	if !errors.Is(err, ErrHTTPStatus) {
		t.Fatalf("expected ErrHTTPStatus, got %v", err)
	}
}

func TestFetchAutoFallsBackToBrowserOnHTTPStatus(t *testing.T) {
	originalBrowserFn := browserHTMLToMarkdownFn
	browserHTMLToMarkdownFn = func(_ context.Context, _ string, _ Config) (string, string, error) {
		return "# Browser Rendered\n", "https://browser.example/final", nil
	}
	defer func() {
		browserHTMLToMarkdownFn = originalBrowserFn
	}()

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusForbidden)
		fmt.Fprint(w, "<html><body><h1>Forbidden</h1></body></html>")
	}))
	defer ts.Close()

	cfg := DefaultConfig()
	cfg.Mode = ModeAuto
	cfg.Timeout = 5 * time.Second

	res, err := Fetch(context.Background(), ts.URL, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Source != "browser" {
		t.Fatalf("expected source browser, got %q", res.Source)
	}
	if res.FinalURL != "https://browser.example/final" {
		t.Fatalf("unexpected final URL: %q", res.FinalURL)
	}
	if !strings.Contains(res.Markdown, "Browser Rendered") {
		t.Fatalf("unexpected markdown output: %q", res.Markdown)
	}
}

func TestToCDPHeadersCookieFormatting(t *testing.T) {
	h := make(http.Header)
	h.Add("Cookie", "a=1")
	h.Add("Cookie", "b=2")
	h.Add("X-Test", "one")
	h.Add("X-Test", "two")

	got := toCDPHeaders(h)
	if got["Cookie"] != "a=1; b=2" {
		t.Fatalf("expected cookie header to use '; ' separator, got %v", got["Cookie"])
	}
	if got["X-Test"] != "one, two" {
		t.Fatalf("expected generic headers to use ', ' separator, got %v", got["X-Test"])
	}
}
