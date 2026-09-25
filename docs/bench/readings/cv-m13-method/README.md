# CV-M13 method and data

Scripts and result tables behind
[`../cv-m13-reading-set-calibration-2026-09-25.md`](../cv-m13-reading-set-calibration-2026-09-25.md),
kept so the reading can be re-derived after the scratch directory it ran in is gone.

- `fathom/` — the fathom ranges: `attest.sh` (a commit-bound test-kit envelope, the
  `tests:attest` recipe), `readingset.sh` (the static reading set plus a class dump),
  `cover.sh` (execution truth: `go test -coverpkg=./... -coverprofile`),
  `join.py` / `wholejoin.py` (join classes to `go tool cover -func`).
  `zz_cvm13_{dump,whole}_test.go.txt` are throwaway `internal/conserve` tests that print
  every changed function's class; drop the `.txt` and copy them into
  `internal/conserve/` of a scratch clone at commit d41cdf14 to run them.
  Tables: `r1-per-function.csv`, `r2-per-function.csv`, `run-summary.json`,
  `r2-wholehead-per-function.csv.gz` (every non-test function at 78e508d3).
- `conjure/` — the weaker-tested control: `run_range.sh`, `dump_ablate.sh`,
  `join_cover.py`, `summarize.py`; per-range `rN-range.txt` and
  `rN-classes-with-coverage.tsv`; `handcheck.tsv`; `summary.txt`. conjure needs
  `GOFLAGS=-mod=vendor` at these heads.

Every script reads its working directory from `CVM13_DIR` (the original run used a
session scratch directory); set it before running. Tools: fathom built at d41cdf14 and
test-kit's `surveyor` from platform-engineering (`kits/test-kit/cmd/surveyor`).
