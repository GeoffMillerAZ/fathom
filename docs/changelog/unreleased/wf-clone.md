### Fixed

- Resolve verified clones and worktrees to their indexed service for MCP paths and hook reach queries. Clone searches and source reads use the clone's files, retain index receipts only for unchanged content, and label changed or new code unknown with clone/index freshness notes.

- Restrict clone reads and searches to tracked paths in the service subtree. Untracked clone files return unknown without source bytes. Refuse home and indexed-root case aliases by file identity, fail closed when home is unavailable, and canonicalize absolute changed-file paths.

- Verify linked-worktree registration, preserve direct service ranking, and reuse immutable Git facts and hook identities within existing deadlines.
