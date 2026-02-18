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

func TestFetchStaticRespectsTextMarkdownForMDXPayload(t *testing.T) {
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
	cfg.Mode = ModeStatic
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

func TestFetchStaticMarkdownAddsMetaByHTMLLookup(t *testing.T) {
	var mdReqCount int
	var htmlReqCount int
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.Header.Get("Accept"), "text/markdown") {
			mdReqCount++
			w.Header().Set("Content-Type", "text/markdown; charset=utf-8")
			fmt.Fprint(w, "# Markdown Body\n\nContent.\n")
			return
		}
		if strings.Contains(r.Header.Get("Accept"), "text/html") {
			htmlReqCount++
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			fmt.Fprint(w, `<!doctype html>
<html><head>
  <title>Docs Title</title>
  <meta name="description" content="Docs Description">
</head><body><h1>HTML view</h1></body></html>`)
			return
		}
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		fmt.Fprint(w, "unexpected request")
	}))
	defer ts.Close()

	cfg := DefaultConfig()
	cfg.Mode = ModeStatic
	cfg.Timeout = 5 * time.Second

	res, err := Fetch(context.Background(), ts.URL, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Source != "http-markdown" {
		t.Fatalf("expected source http-markdown, got %q", res.Source)
	}
	if mdReqCount != 1 {
		t.Fatalf("expected one markdown request, got %d", mdReqCount)
	}
	if htmlReqCount != 1 {
		t.Fatalf("expected one html metadata request, got %d", htmlReqCount)
	}
	if !strings.HasPrefix(res.Markdown, "---\n") {
		t.Fatalf("expected front matter, got: %q", res.Markdown)
	}
	if !strings.Contains(res.Markdown, "title: 'Docs Title'") {
		t.Fatalf("expected title in front matter, got: %q", res.Markdown)
	}
	if !strings.Contains(res.Markdown, "description: 'Docs Description'") {
		t.Fatalf("expected description in front matter, got: %q", res.Markdown)
	}
}

func TestFetchStaticMarkdownSkipsHTMLLookupWhenFrontMatterExists(t *testing.T) {
	var htmlReqCount int
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.Header.Get("Accept"), "text/markdown") {
			w.Header().Set("Content-Type", "text/markdown; charset=utf-8")
			fmt.Fprint(w, "---\ntitle: Existing\n---\n\n# Markdown Body\n")
			return
		}
		if strings.Contains(r.Header.Get("Accept"), "text/html") {
			htmlReqCount++
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			fmt.Fprint(w, "<!doctype html><html><head><title>Should Not Fetch</title></head><body></body></html>")
			return
		}
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		fmt.Fprint(w, "unexpected request")
	}))
	defer ts.Close()

	cfg := DefaultConfig()
	cfg.Mode = ModeStatic
	cfg.Timeout = 5 * time.Second

	res, err := Fetch(context.Background(), ts.URL, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if htmlReqCount != 0 {
		t.Fatalf("expected no html metadata request, got %d", htmlReqCount)
	}
	if !strings.Contains(res.Markdown, "title: Existing") {
		t.Fatalf("expected existing front matter preserved, got: %q", res.Markdown)
	}
}

func TestFetchAutoMarkdownAddsMetaByHTMLLookup(t *testing.T) {
	var mdReqCount int
	var htmlReqCount int
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.Header.Get("Accept"), "text/markdown") {
			mdReqCount++
			w.Header().Set("Content-Type", "text/markdown; charset=utf-8")
			fmt.Fprint(w, "# Auto Markdown\n\nBody.\n")
			return
		}
		if strings.Contains(r.Header.Get("Accept"), "text/html") {
			htmlReqCount++
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			fmt.Fprint(w, `<!doctype html><html><head>
  <title>Auto Meta Title</title>
  <meta name="description" content="Auto Meta Description">
</head><body></body></html>`)
			return
		}
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		fmt.Fprint(w, "unexpected request")
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
	if mdReqCount != 1 {
		t.Fatalf("expected one markdown request, got %d", mdReqCount)
	}
	if htmlReqCount != 1 {
		t.Fatalf("expected one html metadata request, got %d", htmlReqCount)
	}
	if !strings.Contains(res.Markdown, "title: 'Auto Meta Title'") {
		t.Fatalf("expected title in front matter, got: %q", res.Markdown)
	}
	if !strings.Contains(res.Markdown, "description: 'Auto Meta Description'") {
		t.Fatalf("expected description in front matter, got: %q", res.Markdown)
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

func TestFetchStaticAddsMetaFrontMatterByDefault(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, `<!doctype html>
<html>
<head>
  <title>Cloudflare: Connect everywhere</title>
  <meta name="description" content="Make networks faster and more secure.">
</head>
<body>
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
	if !strings.HasPrefix(res.Markdown, "---\n") {
		t.Fatalf("expected markdown to start with front matter, got: %q", res.Markdown)
	}
	if !strings.Contains(res.Markdown, "title: 'Cloudflare: Connect everywhere'") {
		t.Fatalf("expected title in front matter, got: %q", res.Markdown)
	}
	if !strings.Contains(res.Markdown, "description: 'Make networks faster and more secure.'") {
		t.Fatalf("expected description in front matter, got: %q", res.Markdown)
	}
}

func TestFetchStaticDoesNotAddMetaWhenDisabled(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, `<!doctype html>
<html>
<head>
  <title>No Meta Header</title>
  <meta name="description" content="Should not be injected.">
</head>
<body>
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
	cfg.IncludeMeta = false
	cfg.Timeout = 5 * time.Second
	cfg.MinQualityText = 20

	res, err := Fetch(context.Background(), ts.URL, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.HasPrefix(res.Markdown, "---\n") {
		t.Fatalf("expected no front matter when meta is disabled, got: %q", res.Markdown)
	}
}

func TestFetchAutoFallsBackWhenMarkdownBodyIsEmpty(t *testing.T) {
	originalBrowserFn := browserHTMLToMarkdownFn
	browserHTMLToMarkdownFn = func(_ context.Context, _ string, _ Config) (string, string, error) {
		return "# Browser Fallback\n", "https://browser.example/final", nil
	}
	defer func() {
		browserHTMLToMarkdownFn = originalBrowserFn
	}()

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/markdown; charset=utf-8")
		fmt.Fprint(w, " \n\t ")
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
	if !strings.Contains(res.Markdown, "Browser Fallback") {
		t.Fatalf("unexpected markdown output: %q", res.Markdown)
	}
}

func TestFetchRawReturnsBodyAsIs(t *testing.T) {
	var sawAcceptMarkdown bool
	rawBody := "  <html><body><h1>Raw</h1></body></html>\n"
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.Header.Get("Accept"), "text/markdown") {
			sawAcceptMarkdown = true
		}
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, rawBody)
	}))
	defer ts.Close()

	cfg := DefaultConfig()
	cfg.Mode = ModeRaw
	cfg.Timeout = 5 * time.Second

	res, err := Fetch(context.Background(), ts.URL, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !sawAcceptMarkdown {
		t.Fatal("expected Accept: text/markdown in raw mode")
	}
	if res.Source != "http-raw" {
		t.Fatalf("expected source http-raw, got %q", res.Source)
	}
	if res.Markdown != rawBody {
		t.Fatalf("expected raw response body to be preserved, got: %q", res.Markdown)
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

func TestPrependMetaFrontMatterSkipsWhenFrontMatterAlreadyExists(t *testing.T) {
	input := "---\nexisting: true\n---\n\n# Doc\n"
	got := prependMetaFrontMatter(input, pageMeta{
		Title:       "New Title",
		Description: "New Description",
	})
	if got != input {
		t.Fatalf("expected existing front matter to be preserved, got: %q", got)
	}
}

func TestExtractMetaFromHTMLUsesExpectedPriority(t *testing.T) {
	doc := []byte(`<!doctype html>
<html>
<head>
  <title>Title Tag Value</title>
  <meta property="og:title" content="OG Title Value">
  <meta property="og:description" content="OG Description Value">
  <meta name="description" content="Meta Description Value">
</head>
<body><p>content</p></body>
</html>`)

	meta := extractMetaFromHTML(doc)
	if meta.Title != "Title Tag Value" {
		t.Fatalf("expected title from <title>, got %q", meta.Title)
	}
	if meta.Description != "Meta Description Value" {
		t.Fatalf("expected description from meta[name=description], got %q", meta.Description)
	}
}
