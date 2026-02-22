# Changelog

All notable changes to this project are documented in this file.

## [Unreleased]

### Breaking
- Removed root-level `--doctor`; diagnostics must now be invoked via `agent-fetch doctor`.

### Added
- Added `--format` for fetch output selection: `markdown` (default) or `jsonl`.
- Added token-efficient JSONL task payloads with `seq`, `url`, optional `resolved_url`, `resolved_mode`, `content`, optional `meta`, and per-task error rows.
- Added tests for JSONL batch writing, metadata extraction behavior, and safeguards for unknown front matter fields.

### Changed
- Changed metadata behavior in JSONL mode: `--meta` now emits structured `meta` fields instead of front matter injection in `content`.
- Changed CLI command routing to use an internal `web` command for fetch flags while preserving shorthand (`agent-fetch <url>`), so `doctor --help` no longer shows fetch options as global flags.
- Changed root help output to include a dedicated "DEFAULT WEB OPTIONS" section for shorthand discoverability from `agent-fetch -h`.
- Updated README (EN/ZH) and `skills/agent-fetch/SKILL.md` for `--format jsonl` and the new doctor command shape.

## [0.4.0] - 2026-02-21

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
