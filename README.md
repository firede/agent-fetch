# agent-fetch

A Go CLI that always tries to return Markdown for web pages in AI-agent workflows.

[中文](./README.zh.md) | **English**

## Why

Web fetch results are often raw HTML/JS/CSS, which is noisy for LLMs. This tool wraps a fallback pipeline so agents can expect Markdown output.

## Behavior

`agent-fetch` uses three modes:

- `auto` (default):
  - Request with `Accept: text/markdown`
  - If response already looks like Markdown, return it
  - Else do static HTML extraction + convert to Markdown
  - If static result quality is too low, fallback to headless browser render and convert
- `static`: never uses browser fallback
- `browser`: always uses headless browser

## Install (with Go)

If Go is already installed locally:

```bash
go install github.com/firede/agent-fetch/cmd/agent-fetch@latest
```

Install a specific version:

```bash
go install github.com/firede/agent-fetch/cmd/agent-fetch@v0.1.0
```

Make sure `$(go env GOPATH)/bin` (usually `~/go/bin`) is in your `PATH`.

## Install (from Releases)

1. Download the archive for your platform from the [GitHub Releases](https://github.com/firede/agent-fetch/releases) page.
2. Extract it and make the binary executable:

```bash
chmod +x ./agent-fetch
```

### macOS note

Current release binaries are not notarized by Apple (no Apple Developer notarization yet), so Gatekeeper may show:

`“agent-fetch” cannot be opened because Apple cannot check it for malicious software.`

For local validation, remove the quarantine attribute and run:

```bash
xattr -dr com.apple.quarantine ./agent-fetch
./agent-fetch https://example.com
```

## Usage

```bash
agent-fetch <url>
```

Common flags:

```bash
agent-fetch --mode auto --timeout 20s --browser-timeout 30s https://example.com
agent-fetch --mode browser --wait-selector 'article' https://example.com
agent-fetch --header 'Authorization: Bearer <token>' https://example.com
```

All fetched content is printed to `stdout` as Markdown. Errors are printed to `stderr`.

## Build

```bash
go build -o agent-fetch ./cmd/agent-fetch
```

## License

This project is open-sourced under the [MIT License](./LICENSE).
