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

const (
	formatMarkdown = "markdown"
	formatJSONL    = "jsonl"
	webCommandName = "web"
)

const rootHelpTemplate = `NAME:
   {{template "helpNameTemplate" .}}

USAGE:
   {{if .UsageText}}{{wrap .UsageText 3}}{{else}}{{.FullName}} {{if .VisibleFlags}}[global options]{{end}}{{if .VisibleCommands}} [command [command options]]{{end}}{{if .ArgsUsage}} {{.ArgsUsage}}{{else}}{{if .Arguments}} [arguments...]{{end}}{{end}}{{end}}{{if .Version}}{{if not .HideVersion}}

VERSION:
   {{.Version}}{{end}}{{end}}{{if .Description}}

DESCRIPTION:
   {{template "descriptionTemplate" .}}{{end}}
{{- if len .Authors}}

AUTHOR{{template "authorsTemplate" .}}{{end}}{{if .VisibleCommands}}

COMMANDS:{{template "visibleCommandCategoryTemplate" .}}{{end}}{{if .VisibleFlagCategories}}

GLOBAL OPTIONS:{{template "visibleFlagCategoryTemplate" .}}{{else if .VisibleFlags}}

GLOBAL OPTIONS:{{template "visibleFlagTemplate" .}}{{end}}{{range .Commands}}{{if eq .Name "web"}}

DEFAULT WEB OPTIONS:
   ` + "`agent-fetch <url>` is shorthand for `agent-fetch web <url>`." + `
{{if .VisibleFlagCategories}}{{template "visibleFlagCategoryTemplate" .}}{{else if .VisibleFlags}}{{template "visibleFlagTemplate" .}}{{end}}{{end}}{{end}}{{if .Copyright}}

COPYRIGHT:
   {{template "copyrightTemplate" .}}{{end}}
`

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
		UsageText:                     "agent-fetch <url> [url ...]\n   agent-fetch web [options] <url> [url ...]\n   agent-fetch doctor [options]",
		Version:                       versionString(),
		CustomRootCommandHelpTemplate: rootHelpTemplate,
		Commands: []*cli.Command{
			newWebCommand(defaultCfg),
			{
				Name:  "doctor",
				Usage: "run environment checks (browser/runtime) and print remediation guidance",
				Flags: []cli.Flag{
					&cli.StringFlag{Name: "browser-path", Value: defaultCfg.BrowserPath, Usage: "browser executable path/name override for doctor checks"},
				},
				Action: func(ctx context.Context, c *cli.Command) error {
					status, err := runDoctor(ctx, os.Stdout, c.String("browser-path"))
					if err != nil {
						return &exitStatusError{code: 1, msg: fmt.Sprintf("doctor failed: %v", err)}
					}
					if status == doctorStatusWarn {
						return &exitStatusError{code: 1, msg: "doctor: environment check failed, see output above for details"}
					}
					return nil
				},
			},
		},
		Action: func(ctx context.Context, c *cli.Command) error {
			_ = cli.ShowRootCommandHelp(c)
			return &exitStatusError{code: 2}
		},
	}

	if err := cmd.Run(context.Background(), routeToDefaultWeb(os.Args)); err != nil {
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

func newWebCommand(defaultCfg fetcher.Config) *cli.Command {
	return &cli.Command{
		Name:   webCommandName,
		Hidden: true,
		Usage:  "fetch web pages",
		UsageText: "agent-fetch [options] <url> [url ...]\n" +
			"   agent-fetch web [options] <url> [url ...]",
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "mode", Value: defaultCfg.Mode, Usage: "fetch mode: auto|static|browser|raw"},
			&cli.StringFlag{Name: "format", Value: formatMarkdown, Usage: "output format: markdown|jsonl"},
			&cli.BoolFlag{Name: "meta", Value: defaultCfg.IncludeMeta, Usage: "include title/description metadata (markdown: front matter; jsonl: meta field; default true)"},
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
			&cli.StringFlag{Name: "browser-path", Value: defaultCfg.BrowserPath, Usage: "browser executable path/name override for browser/auto modes"},
		},
		Action: runWebFetch,
	}
}

func runWebFetch(ctx context.Context, c *cli.Command) error {
	if c.Args().Len() < 1 {
		_ = cli.ShowSubcommandHelp(c)
		return &exitStatusError{code: 2}
	}

	cfg := fetcher.DefaultConfig()
	cfg.Mode = c.String("mode")
	cfg.IncludeMeta = c.Bool("meta")
	cfg.Timeout = c.Duration("timeout")
	cfg.BrowserTimeout = c.Duration("browser-timeout")
	cfg.BrowserPath = c.String("browser-path")
	cfg.NetworkIdle = c.Duration("network-idle")
	cfg.WaitSelector = c.String("wait-selector")
	cfg.UserAgent = c.String("user-agent")
	cfg.MaxBodyBytes = c.Int64("max-body-bytes")

	parsedHeaders, err := parseHeaders(c.StringSlice("header"))
	if err != nil {
		return &exitStatusError{code: 2, msg: fmt.Sprintf("invalid header: %v", err)}
	}
	cfg.Headers = parsedHeaders
	format := strings.ToLower(strings.TrimSpace(c.String("format")))
	switch format {
	case formatMarkdown, formatJSONL:
	default:
		return &exitStatusError{code: 2, msg: "invalid format: must be markdown or jsonl"}
	}

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
			if format == formatJSONL {
				results := []taskResult{{
					index:    1,
					inputURL: urls[0],
					err:      err,
				}}
				if writeErr := writeBatchJSONL(os.Stdout, results, cfg.IncludeMeta); writeErr != nil {
					return &exitStatusError{code: 1, msg: fmt.Sprintf("write failed: %v", writeErr)}
				}
				return &exitStatusError{code: 1}
			}
			return &exitStatusError{code: 1, msg: fmt.Sprintf("fetch failed: %v", err)}
		}

		if format == formatJSONL {
			results := []taskResult{{
				index:    1,
				inputURL: urls[0],
				finalURL: res.FinalURL,
				source:   res.Source,
				markdown: res.Markdown,
			}}
			if err := writeBatchJSONL(os.Stdout, results, cfg.IncludeMeta); err != nil {
				return &exitStatusError{code: 1, msg: fmt.Sprintf("write failed: %v", err)}
			}
			return nil
		}
		if _, err := os.Stdout.WriteString(res.Markdown); err != nil {
			return &exitStatusError{code: 1, msg: fmt.Sprintf("write failed: %v", err)}
		}
		return nil
	}

	results := fetchBatch(ctx, urls, cfg, concurrency, fetcher.Fetch)
	var writeErr error
	if format == formatJSONL {
		writeErr = writeBatchJSONL(os.Stdout, results, cfg.IncludeMeta)
	} else {
		writeErr = writeBatchMarkdown(os.Stdout, results)
	}
	if writeErr != nil {
		return &exitStatusError{code: 1, msg: fmt.Sprintf("write failed: %v", writeErr)}
	}
	if failed := failedCount(results); failed > 0 {
		return &exitStatusError{code: 1}
	}
	return nil
}

func routeToDefaultWeb(args []string) []string {
	if len(args) <= 1 {
		return args
	}

	first := strings.TrimSpace(args[1])
	if first == "" {
		return args
	}
	switch first {
	case webCommandName, "doctor", "help", "h", "--help", "-h", "--version", "-v":
		return args
	}

	rewritten := make([]string, 0, len(args)+1)
	rewritten = append(rewritten, args[0], webCommandName)
	rewritten = append(rewritten, args[1:]...)
	return rewritten
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
