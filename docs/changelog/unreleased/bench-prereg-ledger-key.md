### Changed

- The contextbenchscore `ledger` and `backfill` verbs refuse, with exit 2 and the ledger left unchanged, a row whose (run_id, criterion) the bench ledger already holds or any append to a ledger with an unparseable line, so `scripts/context-bench.sh --resume` of a run already filed under its criterion now ends in a ledger error (NP-B9, BX-87).
