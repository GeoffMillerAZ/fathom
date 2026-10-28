### Added

- `scenariobench run -prereg docs/bench/prereg/<id>.json` loads the prereg against `#Prereg` and refuses a prereg outside `docs/bench/prereg/`, one that differs from HEAD, a dirty tree, a `-root` outside the prereg's repository, `-rescore-lost`, and a stored cell run under another prereg. It stamps the prereg's sha256 on every cell and ledger row as `prereg_sha256` (BX-62).
