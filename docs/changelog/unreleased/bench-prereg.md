### Added

- `scenariobench run -prereg docs/bench/prereg/<id>.json` loads the prereg against `#Prereg` and refuses a prereg outside `docs/bench/prereg/`, one that differs from HEAD, a dirty tree, a `-root` outside the prereg's repository, `-rescore-lost`, and a stored cell run under another prereg. It stamps the prereg's sha256 on every cell and ledger row as `prereg_sha256` (BX-62).
- `scenariobench outcome -prereg -ledger -run-id` refuses (exit 2) a run whose prereg was last committed after its first cell started, whose prereg sha differs from its rows, or whose rows lack `prereg_sha256` or `first_cell_at`, which `run` and `ledger` now stamp from each cell's `started_at`. A run that passes reads as a no-read (exit 3) until `ClassifyOutcome` lands (BX-88, NP-B1).
