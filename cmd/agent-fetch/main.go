package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/firede/agent-fetch/internal/fetcher"
	"github.com/urfave/cli/v3"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

type exitStatusError struct {
	code int
	msg  string
}

func (e *exitStatusError) Error() string {
	return e.msg
}

func main() {
	cli.VersionPrinter = func(cmd *cli.Command) {
		w := cmd.Root().Writer
		if w == nil {
			w = os.Stdout
		}
		fmt.Fprintln(w, cmd.Version)
	}

	defaultCfg := fetcher.DefaultConfig()
	cmd := &cli.Command{
		Name:  "agent-fetch",
		Usage: "Fetch web pages as clean Markdown for AI-agent workflows",
		Description: "Extracts readable content from web pages and converts it to Markdown.\n" +
			"Uses a three-stage fallback pipeline: native Markdown -> static HTML\n" +
			"extraction -> headless browser rendering. Supports custom headers,\n" +
			"CSS selectors, and concurrent multi-URL batch fetching.",
		UsageText: "agent-fetch [options] <url> [url ...]",
		Version:   versionString(),
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "mode", Value: defaultCfg.Mode, Usage: "fetch mode: auto|static|browser|raw"},
			&cli.BoolFlag{Name: "meta", Value: defaultCfg.IncludeMeta, Usage: "prepend title/description front matter (default true; use --meta=false to disable)"},
			&cli.DurationFlag{Name: "timeout", Value: defaultCfg.Timeout, Usage: "HTTP request timeout for static/auto modes"},
			&cli.DurationFlag{Name: "browser-timeout", Value: defaultCfg.BrowserTimeout, Usage: "page-load timeout for browser/auto modes"},
			&cli.DurationFlag{Name: "network-idle", Value: defaultCfg.NetworkIdle, Usage: "wait this long after last network activity before capturing page content"},
			&cli.StringFlag{Name: "wait-selector", Usage: "CSS selector to wait for before capturing, e.g. 'article', '#content'"},
			&cli.StringFlag{Name: "user-agent", Value: defaultCfg.UserAgent, Usage: "User-Agent header"},
			&cli.Int64Flag{Name: "max-body-bytes", Value: defaultCfg.MaxBodyBytes, Usage: "max response bytes to read"},
			&cli.IntFlag{Name: "concurrency", Value: 4, Usage: "max concurrent URL fetches when multiple URLs are provided"},
			&cli.StringSliceFlag{
				Name:  "header",
				Usage: "custom request header, repeatable. Example: --header 'Authorization: Bearer token'",
			},
		},
		Action: func(ctx context.Context, c *cli.Command) error {
			if c.Args().Len() < 1 {
				_ = cli.ShowRootCommandHelp(c)
				return &exitStatusError{code: 2}
			}

			cfg := fetcher.DefaultConfig()
			cfg.Mode = c.String("mode")
			cfg.IncludeMeta = c.Bool("meta")
			cfg.Timeout = c.Duration("timeout")
			cfg.BrowserTimeout = c.Duration("browser-timeout")
			cfg.NetworkIdle = c.Duration("network-idle")
			cfg.WaitSelector = c.String("wait-selector")
			cfg.UserAgent = c.String("user-agent")
			cfg.MaxBodyBytes = c.Int64("max-body-bytes")

			parsedHeaders, err := parseHeaders(c.StringSlice("header"))
			if err != nil {
				return &exitStatusError{code: 2, msg: fmt.Sprintf("invalid header: %v", err)}
			}
			cfg.Headers = parsedHeaders

			urls := c.Args().Slice()
			concurrency := c.Int("concurrency")
			if concurrency < 1 {
				return &exitStatusError{code: 2, msg: "invalid concurrency: must be >= 1"}
			}

			if len(urls) == 1 {
				reqCtx, cancel := context.WithTimeout(ctx, maxDuration(cfg.Timeout, cfg.BrowserTimeout)+5*time.Second)
				defer cancel()

				res, err := fetcher.Fetch(reqCtx, urls[0], cfg)
				if err != nil {
					return &exitStatusError{code: 1, msg: fmt.Sprintf("fetch failed: %v", err)}
				}

				if _, err := os.Stdout.WriteString(res.Markdown); err != nil {
					return &exitStatusError{code: 1, msg: fmt.Sprintf("write failed: %v", err)}
				}
				return nil
			}

			results := fetchBatch(ctx, urls, cfg, concurrency, fetcher.Fetch)
			if err := writeBatchMarkdown(os.Stdout, results); err != nil {
				return &exitStatusError{code: 1, msg: fmt.Sprintf("write failed: %v", err)}
			}
			if failed := failedCount(results); failed > 0 {
				return &exitStatusError{code: 1}
			}
			return nil
		},
	}

	if err := cmd.Run(context.Background(), os.Args); err != nil {
		var exitErr *exitStatusError
		if errors.As(err, &exitErr) {
			if msg := strings.TrimSpace(exitErr.msg); msg != "" {
				fmt.Fprintln(os.Stderr, msg)
			}
			os.Exit(exitErr.code)
		}
		// urfave/cli already prints usage/parse errors to ErrWriter by default.
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
