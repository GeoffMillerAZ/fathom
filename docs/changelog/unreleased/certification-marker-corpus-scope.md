### Fixed

- Certification marker scans now use the same local discovery scope as source ingest, including default cache ignores, scoped ignores, explicit includes, and configured Git discovery. A subsequent ingest or `tests ingest` replaces stale declarations from newly excluded files without changing test runs.
