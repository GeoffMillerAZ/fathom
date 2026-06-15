# Changelog

All notable changes to sextant. Format loosely follows
[Keep a Changelog](https://keepachangelog.com/); dates are ISO-8601.

## [Unreleased]

### Fixed
- **`outline` honors its repo-relative contract.** `symbolnav.Outline` matched the stored
  (absolute) `FilePath` against the `file` arg by exact string, so any repo-relative path
  — the documented contract, and what agents naturally pass — returned `{"symbols":[]}`.
  Now resolves a relative query by path-segment suffix (absolute queries still match
  exactly). Regression: `TestOutline_relativePathMatchesAbsoluteStore`. Found via fleet
  dogfooding (throughline). (O1, `docs/dogfood-observations.md`.)

### Added
- **Build-identity / staleness signal.** `sextant version` now reports the embedded VCS
  revision, commit time, and dirty flag (via Go `-buildvcs` build info — no ldflags);
  `serve-mcp` logs the same line at startup. A long-running MCP server can outlive several
  commits; this makes a stale server self-evident — compare against `git rev-parse HEAD`.
  (O3.)

### Changed
- **`post-commit` hook rebuilds `bin/sextant` on Go changes** before re-indexing, so the
  on-disk binary tracks HEAD. Previously the hook re-ingested *data* with the old binary
  but never rebuilt it, so query-engine fixes stayed stale in the deployed server for days
  (root cause of an O2 false alarm — a "find_symbol can't find methods" symptom that was
  really a stale server, not a code bug). A running `serve-mcp` still needs a restart to
  load a new binary. (O3.)
