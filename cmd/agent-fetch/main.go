package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/firede/agent-fetch/internal/fetcher"
)

type headerFlags struct {
	values []string
}

func (h *headerFlags) String() string {
	return strings.Join(h.values, ",")
}

func (h *headerFlags) Set(value string) error {
	h.values = append(h.values, value)
	return nil
}

func main() {
	cfg := fetcher.DefaultConfig()
	headers := &headerFlags{}

	flag.StringVar(&cfg.Mode, "mode", cfg.Mode, "fetch mode: auto|static|browser")
	flag.DurationVar(&cfg.Timeout, "timeout", cfg.Timeout, "HTTP timeout")
	flag.DurationVar(&cfg.BrowserTimeout, "browser-timeout", cfg.BrowserTimeout, "browser mode timeout")
	flag.DurationVar(&cfg.NetworkIdle, "network-idle", cfg.NetworkIdle, "required network idle time in browser mode")
	flag.StringVar(&cfg.WaitSelector, "wait-selector", "", "CSS selector to wait for in browser mode")
	flag.StringVar(&cfg.UserAgent, "user-agent", cfg.UserAgent, "User-Agent header")
	flag.Int64Var(&cfg.MaxBodyBytes, "max-body-bytes", cfg.MaxBodyBytes, "max response bytes to read")
	flag.Var(headers, "header", "custom request header, repeatable. Example: -header 'Authorization: Bearer token'")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s [flags] <url>\n\n", os.Args[0])
		flag.PrintDefaults()
	}
	flag.Parse()

	if flag.NArg() != 1 {
		flag.Usage()
		os.Exit(2)
	}
	url := flag.Arg(0)

	parsedHeaders, err := parseHeaders(headers.values)
	if err != nil {
		fmt.Fprintf(os.Stderr, "invalid header: %v\n", err)
		os.Exit(2)
	}
	cfg.Headers = parsedHeaders

	ctx, cancel := context.WithTimeout(context.Background(), maxDuration(cfg.Timeout, cfg.BrowserTimeout)+5*time.Second)
	defer cancel()

	res, err := fetcher.Fetch(ctx, url, cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "fetch failed: %v\n", err)
		os.Exit(1)
	}

	if _, err := os.Stdout.WriteString(res.Markdown); err != nil {
		fmt.Fprintf(os.Stderr, "write failed: %v\n", err)
		os.Exit(1)
	}
}

func parseHeaders(raw []string) (http.Header, error) {
	h := make(http.Header)
	for _, item := range raw {
		parts := strings.SplitN(item, ":", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("%q: expected 'Key: Value'", item)
		}
		k := strings.TrimSpace(parts[0])
		v := strings.TrimSpace(parts[1])
		if k == "" {
			return nil, fmt.Errorf("%q: empty key", item)
		}
		h.Add(k, v)
	}
	return h, nil
}

func maxDuration(a, b time.Duration) time.Duration {
	if a >= b {
		return a
	}
	return b
}
