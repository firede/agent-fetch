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
	defaultCfg := fetcher.DefaultConfig()
	cmd := &cli.Command{
		Name:      "agent-fetch",
		Usage:     "Fetch web content and return markdown-friendly output",
		UsageText: "agent-fetch [options] <url>",
		Version:   fmt.Sprintf("%s (%s, %s)", version, commit, date),
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "mode", Value: defaultCfg.Mode, Usage: "fetch mode: auto|static|browser|raw"},
			&cli.BoolFlag{Name: "meta", Value: defaultCfg.IncludeMeta, Usage: "include title/description front matter for static/browser outputs"},
			&cli.DurationFlag{Name: "timeout", Value: defaultCfg.Timeout, Usage: "HTTP timeout"},
			&cli.DurationFlag{Name: "browser-timeout", Value: defaultCfg.BrowserTimeout, Usage: "browser mode timeout"},
			&cli.DurationFlag{Name: "network-idle", Value: defaultCfg.NetworkIdle, Usage: "required network idle time in browser mode"},
			&cli.StringFlag{Name: "wait-selector", Usage: "CSS selector to wait for in browser mode"},
			&cli.StringFlag{Name: "user-agent", Value: defaultCfg.UserAgent, Usage: "User-Agent header"},
			&cli.Int64Flag{Name: "max-body-bytes", Value: defaultCfg.MaxBodyBytes, Usage: "max response bytes to read"},
			&cli.StringSliceFlag{
				Name:  "header",
				Usage: "custom request header, repeatable. Example: --header 'Authorization: Bearer token'",
			},
		},
		Action: func(ctx context.Context, c *cli.Command) error {
			if c.Args().Len() != 1 {
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

			url := c.Args().First()
			reqCtx, cancel := context.WithTimeout(ctx, maxDuration(cfg.Timeout, cfg.BrowserTimeout)+5*time.Second)
			defer cancel()

			res, err := fetcher.Fetch(reqCtx, url, cfg)
			if err != nil {
				return &exitStatusError{code: 1, msg: fmt.Sprintf("fetch failed: %v", err)}
			}

			if _, err := os.Stdout.WriteString(res.Markdown); err != nil {
				return &exitStatusError{code: 1, msg: fmt.Sprintf("write failed: %v", err)}
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
