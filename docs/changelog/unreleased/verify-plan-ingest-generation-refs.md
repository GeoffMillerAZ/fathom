### Added

- `verify_plan` accepts an explicit `source_scope: ingest_generation` for sealed ingest reference facts, and `fathom plans status --source-scope ingest_generation` passes it to the daemon. Responses name the generation, spec bindings, examined and excluded files, and unknown current-worktree freshness. Spec-backed rows remain Unknown under this scope until current source is independently proven. The default continues to scan current source under the existing budget; this option does not complete default current-source plan verification.
