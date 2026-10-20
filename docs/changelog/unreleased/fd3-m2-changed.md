### Changed

- `owned.json`, `leases.jsonl` and `scip/` live in the state directory. A copy under `.fathom/` is read until the state directory holds one and is never deleted; `fathom doctor` names it (`legacy_files`).
