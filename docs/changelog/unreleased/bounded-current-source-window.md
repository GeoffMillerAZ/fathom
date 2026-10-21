### Changed

- Default daemon `verify` and `plans status` can validate sealed source-reference facts against the current tree in a bounded two-pass observation window. Each continuation accepts at most 64 MiB and has a five-second deadline. A synchronous filesystem read can overrun that deadline; its return invalidates the window instead of publishing a current verdict. Failed identity or close refuses the current verdict; excluded files keep absence Unknown. Older daemons retain the live scan with its partial disclosure.
- The console's cached plan section explicitly reports current-worktree freshness as `unknown`; it does not inherit a CLI run's completed-window receipt.
- A refused current window returns exit 2 and a JSON operational refusal with `current_worktree_freshness: unknown`, a reason, and no plan verdicts. A later re-ingest can supply a new seal for a fresh window.
- Closed current-source-window reports use the collection response budget, preserving every verdict recorded in pending receipts.
