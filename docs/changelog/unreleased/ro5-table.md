### Added
- Rich readouts choose raw or enriched through one deterministic policy table: a failure, or a success with protected diagnostic lines, gets its injected line only when a record names a file the session edited (or a definition the working tree changed) or a test the last run failed; every other call stays raw, and the hook row names the answer with `decided_by: table`.
