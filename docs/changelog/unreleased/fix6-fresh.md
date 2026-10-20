### Fixed

- After edits that timed out against a busy daemon, the post-commit hook no longer skips its refresh request because the edit-time breaker is open: the request is queued and nothing waits on it, so it now always goes out (the kill switch still stops it), and the index catches up to the commit.
- When a read is answered from an index that may be behind the working tree, the disclosure is now a `freshness` field inside the result an agent reads (the JSON text and `structuredContent`), not only in an envelope key clients hide from the model.
- `obligations` with no file now serves the repository root guide's current sections (AGENTS.md, CLAUDE.md) while that guide is uncompiled or stale, ranked above compiled rules from design documents and decision records, and its summary names a non-guide document's kind (for example "design document rule") instead of calling it a root-scope rule.
