# Changelog

All notable changes to this project are documented in this file.

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

