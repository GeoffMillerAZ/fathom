### Changed
- Contributor gates reject merge conflict markers, tracked backup files, and commit messages without a conventional subject or verification evidence; provenance trailers are refused. Main pushes check the same message policy in CI.
- `task t [RUN=pattern] -- PACKAGES` runs isolated race tests and shows package summaries and failure blocks. `task fmt:changed` checks Go files changed since the merge-base with `origin/main` without rewriting them.
- Main CI runs no longer cancel each other. Fail-open hook messages include an error diagnostic from stderr, and the stray conflict marker at the end of the decision records is removed.
