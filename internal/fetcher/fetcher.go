package fetcher

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	nurl "net/url"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	htmltomarkdown "github.com/JohannesKaufmann/html-to-markdown/v2"
	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/chromedp"
	readability "github.com/go-shiori/go-readability"
	"golang.org/x/net/html"
)

const (
	ModeAuto              = "auto"
	ModeStatic            = "static"
	ModeBrowser           = "browser"
	ModeRaw               = "raw"
	maxMarkdownSampleSize = 12000
)

var (
	ErrUnsupportedMode = errors.New("unsupported mode")
	ErrNoContent       = errors.New("no content could be extracted")
	ErrHTTPStatus      = errors.New("unexpected HTTP status code")

	browserHTMLToMarkdownFn = browserHTMLToMarkdown
)

type Config struct {
	Mode           string
	Timeout        time.Duration
	BrowserTimeout time.Duration
	NetworkIdle    time.Duration
	WaitSelector   string
	UserAgent      string
	Headers        http.Header
	MaxBodyBytes   int64
	MinQualityText int
	IncludeMeta    bool
}

type Result struct {
	Markdown string
	Source   string
	FinalURL string
}

type responseData struct {
	Body        []byte
	ContentType string
	FinalURL    string
	StatusCode  int
}

func DefaultConfig() Config {
	return Config{
		Mode:           ModeAuto,
		Timeout:        20 * time.Second,
		BrowserTimeout: 30 * time.Second,
		NetworkIdle:    1200 * time.Millisecond,
		UserAgent:      "agent-fetch/0.1",
		Headers:        make(http.Header),
		MaxBodyBytes:   8 << 20,
		MinQualityText: 220,
		IncludeMeta:    true,
	}
}

func Fetch(ctx context.Context, rawURL string, cfg Config) (Result, error) {
	if _, err := nurl.ParseRequestURI(rawURL); err != nil {
		return Result{}, fmt.Errorf("invalid URL: %w", err)
	}

	switch cfg.Mode {
	case ModeAuto:
		return fetchAuto(ctx, rawURL, cfg)
	case ModeStatic:
		return fetchStaticOnly(ctx, rawURL, cfg)
	case ModeBrowser:
		return fetchBrowserOnly(ctx, rawURL, cfg)
	case ModeRaw:
		return fetchRawOnly(ctx, rawURL, cfg)
	default:
		return Result{}, fmt.Errorf("%w: %s", ErrUnsupportedMode, cfg.Mode)
	}
}

func fetchAuto(ctx context.Context, rawURL string, cfg Config) (Result, error) {
	resp, err := fetchHTTP(ctx, rawURL, cfg, true)
	if err != nil {
		if errors.Is(err, ErrHTTPStatus) {
			return fetchBrowserOnly(ctx, rawURL, cfg)
		}
		return Result{}, err
	}

	// Honor explicit markdown responses from the server, even if the payload is MDX/JSX-heavy.
	if isMarkdownContentType(resp.ContentType) || isLikelyMarkdown(resp.Body, resp.ContentType) {
		md := normalizeMarkdown(resp.Body)
		if md != "" {
			if cfg.IncludeMeta {
				md = withMetaForMarkdownResponse(ctx, rawURL, cfg, md)
			}
			return Result{Markdown: md, Source: "http-markdown", FinalURL: resp.FinalURL}, nil
		}
	}

	md, qualityOK, err := staticHTMLToMarkdown(resp.Body, resp.FinalURL, cfg.MinQualityText)
	if err == nil && qualityOK {
		if cfg.IncludeMeta {
			md = prependMetaFrontMatter(md, extractMetaFromHTML(resp.Body))
		}
		return Result{Markdown: md, Source: "http-static", FinalURL: resp.FinalURL}, nil
	}

	return fetchBrowserOnly(ctx, rawURL, cfg)
}

func fetchStaticOnly(ctx context.Context, rawURL string, cfg Config) (Result, error) {
	resp, err := fetchHTTP(ctx, rawURL, cfg, true)
	if err != nil {
		return Result{}, err
	}

	if isMarkdownContentType(resp.ContentType) || isLikelyMarkdown(resp.Body, resp.ContentType) {
		md := normalizeMarkdown(resp.Body)
		if md != "" {
			if cfg.IncludeMeta {
				md = withMetaForMarkdownResponse(ctx, rawURL, cfg, md)
			}
			return Result{Markdown: md, Source: "http-markdown", FinalURL: resp.FinalURL}, nil
		}
		return Result{}, ErrNoContent
	}

	md, _, err := staticHTMLToMarkdown(resp.Body, resp.FinalURL, cfg.MinQualityText)
	if err != nil {
		return Result{}, err
	}
	if strings.TrimSpace(md) == "" {
		return Result{}, ErrNoContent
	}
	if cfg.IncludeMeta {
		md = prependMetaFrontMatter(md, extractMetaFromHTML(resp.Body))
	}

	return Result{Markdown: md, Source: "http-static", FinalURL: resp.FinalURL}, nil
}

func fetchBrowserOnly(ctx context.Context, rawURL string, cfg Config) (Result, error) {
	md, finalURL, err := browserHTMLToMarkdownFn(ctx, rawURL, cfg)
	if err != nil {
		return Result{}, err
	}
	if strings.TrimSpace(md) == "" {
		return Result{}, ErrNoContent
	}
	return Result{Markdown: md, Source: "browser", FinalURL: finalURL}, nil
}

func fetchRawOnly(ctx context.Context, rawURL string, cfg Config) (Result, error) {
	// Raw mode is a single pass that still prefers markdown from the server.
	// It returns that HTTP response body as-is without any extraction/conversion fallback.
	resp, err := fetchHTTP(ctx, rawURL, cfg, true)
	if err != nil {
		return Result{}, err
	}
	if len(resp.Body) == 0 {
		return Result{}, ErrNoContent
	}
	return Result{
		Markdown: string(resp.Body),
		Source:   "http-raw",
		FinalURL: resp.FinalURL,
	}, nil
}

func fetchHTTP(ctx context.Context, rawURL string, cfg Config, preferMarkdown bool) (responseData, error) {
	accept := ""
	if preferMarkdown {
		accept = "text/markdown, text/plain;q=0.9, text/html;q=0.8, */*;q=0.1"
	}
	return fetchHTTPWithAccept(ctx, rawURL, cfg, accept)
}

func fetchHTTPWithAccept(ctx context.Context, rawURL string, cfg Config, accept string) (responseData, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return responseData{}, fmt.Errorf("create request: %w", err)
	}
	if accept != "" {
		req.Header.Set("Accept", accept)
	}
	if cfg.UserAgent != "" {
		req.Header.Set("User-Agent", cfg.UserAgent)
	}
	for k, vals := range cfg.Headers {
		for _, v := range vals {
			req.Header.Add(k, v)
		}
	}

	client := &http.Client{Timeout: cfg.Timeout}
	resp, err := client.Do(req)
	if err != nil {
		return responseData{}, fmt.Errorf("http request failed: %w", err)
	}
	defer resp.Body.Close()

	lim := cfg.MaxBodyBytes
	if lim <= 0 {
		lim = 8 << 20
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, lim))
	if err != nil {
		return responseData{}, fmt.Errorf("read response body: %w", err)
	}

	finalURL := rawURL
	if resp.Request != nil && resp.Request.URL != nil {
		finalURL = resp.Request.URL.String()
	}
	if resp.StatusCode >= http.StatusBadRequest {
		return responseData{}, fmt.Errorf("%w: %d %s (%s)", ErrHTTPStatus, resp.StatusCode, http.StatusText(resp.StatusCode), finalURL)
	}

	return responseData{
		Body:        body,
		ContentType: resp.Header.Get("Content-Type"),
		FinalURL:    finalURL,
		StatusCode:  resp.StatusCode,
	}, nil
}

func withMetaForMarkdownResponse(ctx context.Context, rawURL string, cfg Config, md string) string {
	if strings.TrimSpace(md) == "" || hasLeadingYAMLFrontMatter(md) {
		return md
	}

	resp, err := fetchHTTPWithAccept(ctx, rawURL, cfg, "text/html, application/xhtml+xml;q=0.9, */*;q=0.1")
	if err != nil {
		return md
	}
	return prependMetaFrontMatter(md, extractMetaFromHTML(resp.Body))
}

func staticHTMLToMarkdown(body []byte, pageURL string, minQualityText int) (string, bool, error) {
	if len(body) == 0 {
		return "", false, ErrNoContent
	}

	htmlInput := string(body)
	articleHTML := ""

	parsedURL, _ := nurl.Parse(pageURL)
	if parsedURL != nil {
		article, err := readability.FromReader(bytes.NewReader(body), parsedURL)
		if err == nil {
			if strings.TrimSpace(article.Content) != "" {
				articleHTML = article.Content
			} else if strings.TrimSpace(article.TextContent) != "" {
				articleHTML = "<p>" + htmlEscape(article.TextContent) + "</p>"
			}
		}
	}

	target := articleHTML
	if strings.TrimSpace(target) == "" {
		target = htmlInput
	}

	md, err := htmltomarkdown.ConvertString(target)
	if err != nil {
		return "", false, fmt.Errorf("convert HTML to markdown: %w", err)
	}
	md = strings.TrimSpace(md)
	if md == "" {
		return "", false, ErrNoContent
	}

	return md + "\n", markdownQuality(md, minQualityText), nil
}

func browserHTMLToMarkdown(ctx context.Context, rawURL string, cfg Config) (string, string, error) {
	allocOpts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.NoDefaultBrowserCheck,
		chromedp.NoFirstRun,
	)
	if cfg.UserAgent != "" {
		allocOpts = append(allocOpts, chromedp.UserAgent(cfg.UserAgent))
	}

	allocCtx, cancelAlloc := chromedp.NewExecAllocator(ctx, allocOpts...)
	defer cancelAlloc()

	tabCtx, cancelTab := chromedp.NewContext(allocCtx)
	defer cancelTab()

	browserCtx, cancelTimeout := context.WithTimeout(tabCtx, cfg.BrowserTimeout)
	defer cancelTimeout()

	watcher := newNetworkIdleWatcher(cfg.NetworkIdle)
	chromedp.ListenTarget(browserCtx, watcher.Listen)

	extraHeaders := toCDPHeaders(cfg.Headers)

	var htmlDoc string
	var finalURL string
	actions := []chromedp.Action{network.Enable()}
	if len(extraHeaders) > 0 {
		actions = append(actions, network.SetExtraHTTPHeaders(extraHeaders))
	}
	actions = append(actions,
		chromedp.Navigate(rawURL),
	)
	if cfg.WaitSelector != "" {
		actions = append(actions, chromedp.WaitVisible(cfg.WaitSelector, chromedp.ByQuery))
	} else {
		actions = append(actions, chromedp.WaitReady("body", chromedp.ByQuery))
	}
	actions = append(actions,
		chromedp.ActionFunc(watcher.Wait),
		chromedp.OuterHTML("html", &htmlDoc, chromedp.ByQuery),
		chromedp.Location(&finalURL),
	)

	if err := chromedp.Run(browserCtx, actions...); err != nil {
		return "", "", fmt.Errorf("browser render failed: %w", err)
	}

	md, _, err := staticHTMLToMarkdown([]byte(htmlDoc), finalURL, cfg.MinQualityText)
	if err != nil {
		return "", "", err
	}
	if cfg.IncludeMeta {
		md = prependMetaFrontMatter(md, extractMetaFromHTML([]byte(htmlDoc)))
	}
	return md, finalURL, nil
}

func isLikelyMarkdown(body []byte, contentType string) bool {
	trimmed := strings.TrimSpace(string(body))
	if trimmed == "" {
		return false
	}

	sample := trimmed
	if len(sample) > maxMarkdownSampleSize {
		sample = sample[:maxMarkdownSampleSize]
	}
	lcType := strings.ToLower(contentType)
	if looksLikeJSONPayload(sample, lcType) {
		return false
	}
	lower := strings.ToLower(sample)

	if strings.Contains(lower, "<!doctype html") || strings.Contains(lower, "<html") || strings.Contains(lower, "<body") {
		return false
	}
	if strings.Contains(lower, "<script") || strings.Contains(lower, "<style") {
		return false
	}

	htmlTagCount := len(htmlTagRe.FindAllString(lower, -1))
	if htmlTagCount >= 6 {
		return false
	}

	score := markdownScore(sample)
	if score >= 2 {
		return true
	}

	if strings.Contains(lcType, "text/markdown") && htmlTagCount == 0 {
		return true
	}

	if htmlTagCount == 0 && score >= 1 && len(sample) >= 180 {
		return true
	}
	if htmlTagCount == 0 && !strings.Contains(lcType, "text/html") && len(sample) >= 80 {
		return true
	}

	return false
}

func isMarkdownContentType(contentType string) bool {
	return strings.Contains(strings.ToLower(contentType), "text/markdown")
}

func looksLikeJSONPayload(sample, contentType string) bool {
	if strings.Contains(contentType, "json") {
		return true
	}

	trim := strings.TrimSpace(sample)
	if trim == "" {
		return false
	}
	if trim[0] != '{' && trim[0] != '[' {
		return false
	}
	if json.Valid([]byte(trim)) {
		return true
	}
	// Treat truncated JSON-like payloads as structured data instead of markdown.
	return strings.Contains(trim, "\":")
}

var htmlTagRe = regexp.MustCompile(`</?[a-z][a-z0-9]*(\s+[^>]*)?>`)

func markdownScore(input string) int {
	lines := strings.Split(input, "\n")
	score := 0
	for _, line := range lines {
		l := strings.TrimSpace(line)
		if l == "" {
			continue
		}
		switch {
		case headingRe.MatchString(l):
			score += 2
		case listRe.MatchString(l):
			score++
		case quoteRe.MatchString(l):
			score++
		case tableRe.MatchString(l):
			score++
		}
		if strings.Contains(l, "```") {
			score += 2
		}
		if linkRe.MatchString(l) {
			score++
		}
		if score >= 5 {
			return score
		}
	}
	return score
}

var (
	headingRe = regexp.MustCompile(`^#{1,6}\s+\S`)
	listRe    = regexp.MustCompile(`^(?:[-*+]\s+\S|\d+\.\s+\S)`)
	quoteRe   = regexp.MustCompile(`^>\s+\S`)
	tableRe   = regexp.MustCompile(`^\|.+\|$`)
	linkRe    = regexp.MustCompile(`\[[^\]]+\]\([^\)]+\)`)
)

func markdownQuality(md string, minQualityText int) bool {
	trim := strings.TrimSpace(md)
	if trim == "" {
		return false
	}

	if minQualityText <= 0 {
		minQualityText = 220
	}

	textLen := 0
	for _, r := range trim {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r > 127 {
			textLen++
		}
	}
	if textLen < minQualityText {
		return false
	}

	lines := strings.Split(trim, "\n")
	nonEmpty := 0
	linkOnly := 0
	for _, line := range lines {
		l := strings.TrimSpace(line)
		if l == "" {
			continue
		}
		nonEmpty++
		if linkOnlyRe.MatchString(l) {
			linkOnly++
		}
	}
	if nonEmpty > 0 && linkOnly*2 > nonEmpty && textLen < minQualityText*3 {
		return false
	}

	if markdownScore(trim) >= 2 {
		return true
	}

	return textLen >= minQualityText*2
}

var linkOnlyRe = regexp.MustCompile(`^\[.+\]\(.+\)$`)

type pageMeta struct {
	Title       string
	Description string
}

func extractMetaFromHTML(body []byte) pageMeta {
	if len(body) == 0 {
		return pageMeta{}
	}

	doc, err := html.Parse(bytes.NewReader(body))
	if err != nil {
		return pageMeta{}
	}

	head := findFirstElement(doc, "head")
	if head == nil {
		head = doc
	}

	var (
		titleTag      string
		ogTitle       string
		description   string
		ogDescription string
	)

	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n == nil {
			return
		}
		if n.Type == html.ElementNode {
			switch strings.ToLower(n.Data) {
			case "title":
				if titleTag == "" {
					titleTag = normalizeMetaValue(nodeText(n))
				}
			case "meta":
				content := normalizeMetaValue(htmlAttr(n, "content"))
				if content != "" {
					name := strings.ToLower(strings.TrimSpace(htmlAttr(n, "name")))
					prop := strings.ToLower(strings.TrimSpace(htmlAttr(n, "property")))
					switch {
					case name == "description" && description == "":
						description = content
					case prop == "og:description" && ogDescription == "":
						ogDescription = content
					case prop == "og:title" && ogTitle == "":
						ogTitle = content
					}
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(head)

	meta := pageMeta{}
	if titleTag != "" {
		meta.Title = titleTag
	} else {
		meta.Title = ogTitle
	}
	if description != "" {
		meta.Description = description
	} else {
		meta.Description = ogDescription
	}
	return meta
}

func prependMetaFrontMatter(md string, meta pageMeta) string {
	if strings.TrimSpace(md) == "" {
		return md
	}
	if hasLeadingYAMLFrontMatter(md) {
		return md
	}

	meta.Title = normalizeMetaValue(meta.Title)
	meta.Description = normalizeMetaValue(meta.Description)
	if meta.Title == "" && meta.Description == "" {
		return md
	}

	var b strings.Builder
	b.WriteString("---\n")
	if meta.Title != "" {
		b.WriteString("title: ")
		b.WriteString(yamlQuote(meta.Title))
		b.WriteByte('\n')
	}
	if meta.Description != "" {
		b.WriteString("description: ")
		b.WriteString(yamlQuote(meta.Description))
		b.WriteByte('\n')
	}
	b.WriteString("---\n\n")
	b.WriteString(md)
	return b.String()
}

func hasLeadingYAMLFrontMatter(md string) bool {
	s := strings.TrimPrefix(md, "\ufeff")
	switch {
	case strings.HasPrefix(s, "---\n"):
		s = s[len("---\n"):]
	case strings.HasPrefix(s, "---\r\n"):
		s = s[len("---\r\n"):]
	default:
		return false
	}

	for len(s) > 0 {
		var line string
		if idx := strings.IndexByte(s, '\n'); idx >= 0 {
			line = s[:idx]
			s = s[idx+1:]
		} else {
			line = s
			s = ""
		}
		line = strings.TrimSuffix(line, "\r")
		if strings.TrimSpace(line) == "---" {
			return true
		}
	}
	return false
}

func yamlQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "''") + "'"
}

func normalizeMetaValue(s string) string {
	return strings.Join(strings.Fields(strings.TrimSpace(s)), " ")
}

func findFirstElement(root *html.Node, tag string) *html.Node {
	target := strings.ToLower(tag)
	var found *html.Node
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n == nil || found != nil {
			return
		}
		if n.Type == html.ElementNode && strings.ToLower(n.Data) == target {
			found = n
			return
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(root)
	return found
}

func htmlAttr(n *html.Node, key string) string {
	for _, a := range n.Attr {
		if strings.EqualFold(a.Key, key) {
			return a.Val
		}
	}
	return ""
}

func nodeText(n *html.Node) string {
	if n == nil {
		return ""
	}
	var b strings.Builder
	var walk func(*html.Node)
	walk = func(cur *html.Node) {
		if cur == nil {
			return
		}
		if cur.Type == html.TextNode {
			b.WriteString(cur.Data)
		}
		for c := cur.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)
	return b.String()
}

func normalizeMarkdown(body []byte) string {
	md := strings.TrimSpace(string(body))
	if md == "" {
		return ""
	}
	return md + "\n"
}

func htmlEscape(s string) string {
	replacer := strings.NewReplacer(
		"&", "&amp;",
		"<", "&lt;",
		">", "&gt;",
		`"`, "&quot;",
		"'", "&#39;",
	)
	return replacer.Replace(s)
}

func toCDPHeaders(h http.Header) network.Headers {
	if len(h) == 0 {
		return nil
	}
	keys := make([]string, 0, len(h))
	for k := range h {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	res := make(network.Headers, len(keys))
	for _, k := range keys {
		vals := h.Values(k)
		if strings.EqualFold(k, "Cookie") {
			res[k] = strings.Join(vals, "; ")
			continue
		}
		res[k] = strings.Join(vals, ", ")
	}
	return res
}

type networkIdleWatcher struct {
	idleAfter time.Duration

	mu      sync.Mutex
	pending map[network.RequestID]struct{}
	timer   *time.Timer
	idleCh  chan struct{}
}

func newNetworkIdleWatcher(idleAfter time.Duration) *networkIdleWatcher {
	if idleAfter <= 0 {
		idleAfter = 1200 * time.Millisecond
	}
	return &networkIdleWatcher{
		idleAfter: idleAfter,
		pending:   make(map[network.RequestID]struct{}),
		idleCh:    make(chan struct{}, 1),
	}
}

func (w *networkIdleWatcher) Listen(ev any) {
	w.mu.Lock()
	defer w.mu.Unlock()

	switch e := ev.(type) {
	case *network.EventRequestWillBeSent:
		if e.Type == network.ResourceTypeWebSocket || e.Type == network.ResourceTypeEventSource {
			return
		}
		w.pending[e.RequestID] = struct{}{}
		w.resetTimerLocked()
	case *network.EventLoadingFinished:
		delete(w.pending, e.RequestID)
		w.resetTimerLocked()
	case *network.EventLoadingFailed:
		delete(w.pending, e.RequestID)
		w.resetTimerLocked()
	}
}

func (w *networkIdleWatcher) Wait(ctx context.Context) error {
	w.mu.Lock()
	w.resetTimerLocked()
	w.mu.Unlock()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-w.idleCh:
		return nil
	}
}

func (w *networkIdleWatcher) resetTimerLocked() {
	if len(w.pending) > 0 {
		if w.timer != nil {
			w.timer.Stop()
		}
		return
	}
	if w.timer == nil {
		w.timer = time.AfterFunc(w.idleAfter, w.signalIdle)
		return
	}
	w.timer.Reset(w.idleAfter)
}

func (w *networkIdleWatcher) signalIdle() {
	select {
	case w.idleCh <- struct{}{}:
	default:
	}
}
