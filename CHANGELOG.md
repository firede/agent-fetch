# Changelog

All notable changes to this project are documented in this file.

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
