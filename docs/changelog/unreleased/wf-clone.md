### Fixed

- Resolve verified clones and worktrees to their indexed service for MCP paths and hook reach queries. Clone searches and source reads use the clone's files, retain index receipts only for unchanged content, and label changed or new code unknown with clone/index freshness notes.

- Restrict clone reads to Git-visible paths, verify linked-worktree registration, preserve direct service ranking and explicit-service search behavior, and reuse immutable Git facts and hook identities within existing deadlines.
