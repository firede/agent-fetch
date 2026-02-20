# Changelog

All notable changes to this project are documented in this file.

## [Unreleased]

### Added
- Added `--doctor` to run environment diagnostics and report browser/runtime readiness for `auto`/`browser` modes.
- Added actionable remediation output for missing or non-working headless browser setups across Linux/macOS/Windows environments.
- Added `--browser-path` to explicitly configure the browser executable (useful for containers and custom installations).
- Added unit tests for doctor-mode browser detection and probe behavior.

### Changed
- Updated README (EN/ZH) with `--doctor` usage and diagnostics guidance.
- Updated `skills/agent-fetch/SKILL.md` with `--doctor` / `--browser-path` command patterns and browser troubleshooting guidance.
- Unified browser executable resolution between `--doctor` and runtime `browser` mode.

## [0.3.0] - 2026-02-19

### Added
- Added concurrent multi-URL fetching with shared flags by accepting `agent-fetch [options] <url> [url ...]`.
- Added `--concurrency` to control maximum parallel fetch tasks for multi-URL runs.
- Added LLM-friendly batch output markers with per-task URL mapping and per-task error markers for failed tasks.
- Added tests for batch output formatting and stable input-order output under concurrent execution.

### Changed
- Updated exit code behavior for batch mode: `0` when all tasks succeed, `1` when any task fails, and `2` for usage/argument errors.
- Updated `--version` behavior to resolve version from Go build info when release ldflags are absent, improving `go install ...@vX.Y.Z` experience.
- Updated README (EN/ZH) and skill docs to describe multi-URL behavior, batch output format, and concurrency controls.

## [0.2.1] - 2026-02-19

### Added
- Added `--version`/`-v` output with build metadata (`version`, `commit`, `date`) and wired release-time injection via GoReleaser ldflags.
- Added Agent Skills support files at `skills/agent-fetch` with installation guidance and usage instructions for agent-driven workflows.

### Changed
- Added CI verification for the version flag to prevent regressions in release metadata output.
- Documented Chrome/Chromium runtime dependency for `browser` mode and `auto` fallback in README and skill docs.

## [0.2.0] - 2026-02-18

### Added
- Added `raw` mode for markdown-preferred single-pass output (`--mode raw`), returning the HTTP response body without extraction or fallback conversion.
- Added metadata front matter injection (`title`, `description`) for non-raw outputs, enabled by default with `--meta` and disabled via `--meta=false`.
- Added metadata enrichment for direct markdown responses in `auto`/`static` mode by fetching HTML metadata when needed.
- Added tests for MDX/markdown handling, metadata injection behavior, and front matter preservation.

### Changed
- Migrated CLI argument parsing to `urfave/cli/v3`.
- Standardized CLI usage around `--long-flag` style and more flexible flag/argument ordering.

### Fixed
- Honored `text/markdown` responses directly (including MDX-like content) to avoid dropping sections during fallback conversion.
- Improved fallback behavior for HTTP status errors in `auto` mode.
- Tightened markdown detection and related review cleanups.
