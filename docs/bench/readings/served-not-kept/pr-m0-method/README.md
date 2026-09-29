# PR-M0 method and data

Scripts and result tables behind [`../pr-m0-replay.md`](../pr-m0-replay.md), kept so the reading can
be re-derived after the scratch directory it ran in is gone. Nothing here is product code, and nothing
here calls a model.

- `replay.sh`: part A. For each cell's `attempt-N/agent.diff`, reset a scratch clone to `f67f06a`,
  apply the diff without the instrument's own `.claude/scenariobench-mcp.json`
  (`strip_instrument.py`), commit, scratch-ingest, then `fathom conserve --since f67f06a --json`.
  Each worker keeps one clone path, because the ingest's typed pass compiles through the Go build
  cache and its keys include the source directory.
- `extract_fragments.py`: part B's input. Every `Edit`, `Write` and `MultiEdit` proposal in the cell
  transcripts, made into the hook's own fragment (`internal/harness/codec.go`
  `claudeProposedNewContent`/`OldContent`), with a repo-relative path.
- `zz_prm0_fragments_test.go.txt` and `fragments.sh`: part B. A throwaway `internal/guardspec` test
  that runs `EvaluateGuardEnforcement` and `EvaluateLayerRuleEnforcement` over every fragment, with
  `Enforce` forced on for every guard and layer rule, against two copies of `.fathom/guard.cue`:
  `base` (at `f67f06a`, the checkout the cells ran against) and `head` (at `fa240083`, the production
  config an `enforce: warn` arm would run). `fragments.sh` drops the `.txt`, copies the file into
  `internal/guardspec/` of a scratch clone at `fa240083`, runs it and removes it.
- `summarize.py`: joins parts A and B with the oracle (the ledger rows of the run each directory
  came from) and writes `part-a.tsv` (with `part-a.md`, the reading's per-cell table), `part-b.tsv`
  and `summary.json`.

Environment: `PRM0_DIR` (the work directory: `base/` is a clone at `f67f06a`, `eval/` a clone at
`fa240083`), `PRM0_RAW` (the raw cell root, `benchroot-raw-2026-09-23`), `PRM0_FATHOM` (a
`fathom` binary built at `fa240083`), `PRM0_JOBS` (default 3), `PRM0_LEDGER`
(`docs/bench/scenarios-ledger.jsonl`), and optionally `PRM0_PRE_CELL`, a shell command run before every try
of a cell (the reading's host needed one to clear a stale file-access verdict on its external volume;
a try that fails is retried twice).

```sh
export PRM0_DIR=<scratch> PRM0_RAW=<benchroot-raw-2026-09-23> PRM0_FATHOM=<fathom@fa240083>
export PRM0_LEDGER=docs/bench/scenarios-ledger.jsonl
git clone <fathom> "$PRM0_DIR/base" && git -C "$PRM0_DIR/base" checkout f67f06a
git clone <fathom> "$PRM0_DIR/eval" && git -C "$PRM0_DIR/eval" checkout fa240083
./replay.sh && ./fragments.sh && ./summarize.py
```
