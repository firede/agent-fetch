---
name: agent-fetch
description: Fetch web pages as markdown-first content with static and browser fallbacks using the agent-fetch CLI. Use when an agent needs cleaner content than raw HTML, must handle dynamic pages, or needs explicit fetch controls such as mode, headers, selectors, and timeouts.
---

# Agent Fetch

Use this skill to retrieve web content in Markdown-first form for downstream agent workflows.

## Ensure the CLI exists

Require `agent-fetch` in `PATH`.

If `agent-fetch` is not installed, install it first.

Preferred method (no Go required):

1. Download a release artifact from:
   - `https://github.com/firede/agent-fetch/releases`
2. Extract the archive.
3. Make the binary executable.
4. Put the binary in a directory on `PATH`.

macOS note (for unsigned binaries):

If Gatekeeper blocks execution, remove the quarantine attribute, then run again:

```bash
xattr -dr com.apple.quarantine /path/to/agent-fetch
```

Alternative method (requires Go):

```bash
go install github.com/firede/agent-fetch/cmd/agent-fetch@latest
```

Verify installation:

```bash
agent-fetch --help
```

## Fetch workflow

Browser runtime requirement:

- `--mode browser` requires Chrome/Chromium available on the host.
- `--mode auto` may fall back to browser rendering on some pages.
- Use `--mode static` or `--mode raw` when browser runtime is unavailable.

1. Start with `auto` mode for general pages.
2. Use `browser` mode for JavaScript-heavy pages.
3. Add `--wait-selector` in `browser` mode when content appears late.
4. Use `static` mode when browser execution is not desired.
5. Use `raw` mode when the exact HTTP response body is needed.
6. Add repeated `--header` flags for auth/session requirements.
7. Tune `--timeout`, `--browser-timeout`, `--network-idle`, `--max-body-bytes`, and `--concurrency` for slow or large batches.
8. For multiple URLs, rely on task markers in output to map each result back to its input URL.

## Command patterns

Default:

```bash
agent-fetch https://example.com
```

Multiple URLs with shared flags:

```bash
agent-fetch --mode static --concurrency 4 https://example.com https://example.org
```

Static-only:

```bash
agent-fetch --mode static https://example.com
```

Browser with wait selector:

```bash
agent-fetch --mode browser --wait-selector "article" https://example.com
```

Raw response body:

```bash
agent-fetch --mode raw https://example.com
```

Auth header:

```bash
agent-fetch --header "Authorization: Bearer <token>" https://example.com
```

## Output contract

- Read fetched content from `stdout`.
- For multiple URLs, parse output sections in this shape:

```text
<!-- count: N, succeeded: X, failed: Y -->
<!-- task[1]: <input-url> -->
...markdown...
<!-- /task[1] -->
<!-- task[2](failed): <input-url> -->
<!-- error[2]: <error text> -->
```

- Treat exit code `0` as all successful, `1` as partial/complete task failure, and `2` as argument/usage error.
