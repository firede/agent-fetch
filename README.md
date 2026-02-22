# agent-fetch

Fetch web pages as clean Markdown for AI-agent workflows.

[中文](./README.zh.md) | **English**

[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](./LICENSE)
[![Go](https://github.com/firede/agent-fetch/actions/workflows/ci.yml/badge.svg)](https://github.com/firede/agent-fetch/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/v/release/firede/agent-fetch)](https://github.com/firede/agent-fetch/releases)

## Highlights

- **Markdown-first output pipeline** -- readability extraction + HTML-to-Markdown conversion, so agents receive clean text instead of noisy HTML/JS/CSS
- **Headless browser fallback** -- renders JavaScript-heavy pages (SPAs, dynamic dashboards) when static extraction falls short
- **Custom request headers** -- send `Authorization`, `Cookie`, or any header to access authenticated endpoints
- **Multi-URL batch fetching** -- fetch multiple pages concurrently with structured, per-URL output

## Quick Start

```bash
# Install
go install github.com/firede/agent-fetch/cmd/agent-fetch@latest

# Fetch a page
agent-fetch https://example.com
```

Or download a prebuilt binary from [Releases](https://github.com/firede/agent-fetch/releases).

## How It Works

In the default `auto` mode, agent-fetch runs a three-stage fallback pipeline:

```
Request with Accept: text/markdown
        |
        v
  Markdown response? --yes--> Return as-is
        | no
        v
  Static HTML extraction
  + Markdown conversion --quality OK?--> Return
        | no
        v
  Headless browser render
  + extraction + conversion --> Return
```

This means most pages are handled without a browser, keeping things fast, while JS-heavy pages still get rendered correctly.

## Modes

| Mode | Behavior | Browser needed |
|------|----------|----------------|
| `auto` (default) | Three-stage fallback: native Markdown -> static extraction -> browser render | Only when static quality is low |
| `static` | Static HTML extraction only, no browser | No |
| `browser` | Always use headless Chrome/Chromium | Yes |
| `raw` | Send `Accept: text/markdown`, return HTTP body verbatim | No |

## Installation

### From Releases

1. Download the archive for your platform from [GitHub Releases](https://github.com/firede/agent-fetch/releases).
2. Extract and make the binary executable:

```bash
chmod +x ./agent-fetch
```

3. Move the binary to a directory on your `PATH`, or run it directly:

```bash
./agent-fetch https://example.com
```

#### macOS note

Release binaries are not yet notarized by Apple, so Gatekeeper may block execution. Remove the quarantine attribute to proceed:

```bash
xattr -dr com.apple.quarantine ./agent-fetch
```

### With Go

```bash
go install github.com/firede/agent-fetch/cmd/agent-fetch@latest
```

Install a specific version:

```bash
go install github.com/firede/agent-fetch/cmd/agent-fetch@v0.3.0
```

Ensure `$(go env GOPATH)/bin` (usually `~/go/bin`) is in your `PATH`.

## Usage

```bash
agent-fetch [options] <url> [url ...]
agent-fetch web [options] <url> [url ...]
agent-fetch doctor [options]
```

### Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--mode` | `auto` | Fetch mode: `auto` \| `static` \| `browser` \| `raw` |
| `--format` | `markdown` | Output format: `markdown` \| `jsonl` |
| `--meta` | `true` | Include `title`/`description` metadata (`markdown`: front matter, `jsonl`: `meta` field; use `--meta=false` to disable) |
| `--timeout` | `20s` | HTTP request timeout (applies to static/auto modes) |
| `--browser-timeout` | `30s` | Page-load timeout (applies to browser/auto modes) |
| `--network-idle` | `1200ms` | Wait time after last network activity before capturing content |
| `--wait-selector` | | CSS selector to wait for before capturing, e.g. `article` |
| `--header` | | Custom request header, repeatable. e.g. `--header 'Authorization: Bearer token'` |
| `--user-agent` | `agent-fetch/0.1` | User-Agent header |
| `--max-body-bytes` | `8388608` | Max response bytes to read |
| `--concurrency` | `4` | Max concurrent fetches for multi-URL requests |
| `--browser-path` | | Browser executable path/name override for `browser` and `auto` modes |

### Examples

```bash
# Default auto mode
agent-fetch https://example.com

# Force browser rendering for a JS-heavy page
agent-fetch --mode browser --wait-selector 'article' https://example.com

# Force a specific browser binary (useful in containers/custom installs)
agent-fetch --mode browser --browser-path /usr/bin/chromium https://example.com

# Static extraction without front matter
agent-fetch --mode static --meta=false https://example.com

# Get raw HTTP response body
agent-fetch --mode raw https://example.com

# Authenticated request
agent-fetch --header "Authorization: Bearer $TOKEN" https://example.com

# Batch fetch with concurrency control
agent-fetch --concurrency 4 https://example.com https://example.org

# Structured JSONL output
agent-fetch --format jsonl https://example.com

# Check environment readiness
agent-fetch doctor

# Check environment readiness with explicit browser path
agent-fetch doctor --browser-path /usr/bin/chromium
```

## Multi-URL Batch (Markdown)

When multiple URLs are provided, requests run concurrently (controlled by `--concurrency`) and output is emitted in input order using task markers:

```text
<!-- count: 3, succeeded: 2, failed: 1 -->
<!-- task[1]: https://example.com/hello -->
...markdown...
<!-- /task[1] -->
<!-- task[2](failed): https://abc.com -->
<!-- error[2]: ... -->
```

Exit codes: `0` all succeeded, `1` any task failed, `2` argument/usage error.

## JSONL Output Contract

When `--format jsonl` is used, each task emits one JSON line (no summary line):

```json
{"seq":1,"url":"https://example.com","resolved_mode":"static","content":"...","meta":{"title":"...","description":"..."}}
{"seq":2,"url":"https://bad.example","error":"http request failed: timeout"}
```

Field notes:
- `url`: input URL
- `resolved_url`: emitted only when different from `url`
- `resolved_mode`: one of `markdown`, `static`, `browser`, `raw`
- `meta`: emitted only when `--meta=true` and metadata exists

## Agent Integration

This project ships a [SKILL.md](./skills/agent-fetch/SKILL.md) that can be used with coding agents that support skill files. Point your skill directory to `skills/agent-fetch` and the agent will be able to invoke `agent-fetch` when its built-in fetch capability is insufficient.

`agent-fetch` reads from the command line and writes results to stdout (`markdown` or `jsonl`), making it easy to integrate into any agent pipeline or shell-based tool call:

```bash
result=$(agent-fetch --mode static https://example.com)
```

## When Do You Need This?

The table below compares agent-fetch with the built-in web-fetch capabilities found in some coding agents. Actual built-in capabilities vary by product and version.

| Scenario | Built-in web fetch | agent-fetch |
|----------|:------------------:|:-----------:|
| Basic page fetch with HTML simplification | Yes | Yes |
| JavaScript-rendered pages (SPAs) | Varies | Yes (headless browser) |
| Custom headers (auth, cookies) | Varies | Yes (`--header`) |
| No AI summarization (outputs extracted body as-is) | Varies | Yes (subject to `--max-body-bytes`) |
| Batch fetch multiple URLs concurrently | Varies | Yes (`--concurrency`) |
| CSS selector-based wait/extraction | Varies | Yes (`--wait-selector`) |
| Works outside coding agents (CLI, CI/CD) | N/A | Yes (standalone CLI) |

**How built-in web fetch typically works:** Tools like Claude Code's WebFetch and Codex's built-in fetch retrieve a page over HTTP, convert the HTML to Markdown, and then pass the content through an AI model that may summarize or truncate it to fit the context window. This pipeline is fast and sufficient for most pages, but it usually does not execute JavaScript (so SPA or JS-rendered pages may return incomplete content), does not support custom request headers, and processes one URL at a time.

- **No built-in web fetch available** (other agent frameworks, CLI pipelines, CI/CD) -- use agent-fetch as your primary fetch tool.
- **Built-in web fetch available** -- use agent-fetch as a complement for JS-heavy pages, authenticated endpoints, batch fetching, or when you need the extracted content without summarization.

## Runtime Dependencies

`browser` and `auto` modes may require Chrome or Chromium on the host.

Use `--mode static` or `--mode raw` to avoid the browser dependency entirely.

- Run `agent-fetch doctor` to validate runtime/browser readiness and get guided fixes.
- Use `--browser-path` when the browser is installed in a non-default location (common in container images).

## Build

```bash
go build -o agent-fetch ./cmd/agent-fetch
```

## License

This project is open-sourced under the [MIT License](./LICENSE).
