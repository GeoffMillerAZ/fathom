### Fixed

- `fathom tests select` counts untracked files, both sides of a rename and new test functions the index does not hold yet, and with no `--store` reads the store the daemon records instead of a workspace file an earlier ingest left behind, which had selected nothing (TQ-34, TQ-36).
