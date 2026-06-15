# calibration/ — the extractor calibration corpus

This directory is the **measurement instrument** for sextant's language extractors.
Today only Go is parsed with a real AST (`go/parser`); every other language is a
regex/line heuristic that has only ever been tested against ~10 lines of synthetic
fixture. You cannot drive an extractor to "excellent" without measuring it against
**real-world code**, and you cannot measure recall without a **ground-truth oracle**.
This harness provides both.

## What's here

| File | Role |
|---|---|
| `corpus.yaml` | The curated manifest: per language/tool, the chosen public sample repos (pinned to a real commit SHA), plus the decided extraction approach (pure-Go parser vs tree-sitter vs SCIP) and the symbol kinds an excellent extractor should emit. **Checked in.** |
| `clone.sh` | Shallow-clones (`--depth 1`, single-branch) every repo in the manifest into `workspace/calibration/<lang>/<repo>` (gitignored). Idempotent. |
| `baseline.sh` | Ingests each cloned repo through the **real** sextant pipeline and reports symbols-found per extractor, vs a rough grep-based "expected definitions" proxy, to expose recall gaps. Writes `workspace/calibration/baseline-report.md`. |

The cloned source itself never enters git — only the manifest does. This keeps the
repo lean and license-clean while staying fully reproducible (URL + pinned SHA).

## Usage

```sh
task calibrate                       # clone the whole corpus + run the baseline
task calibrate-clone -- Python Helm  # clone only some languages/tools
task calibrate-baseline -- Python    # re-measure one language (uses existing clones)
```

Calibration is **deliberately excluded from `task validate`** — the hermetic gate must
not touch the network. Run it on demand.

## The oracle

`baseline.sh` ships a cheap grep-based proxy oracle so it runs with zero extra tooling.
It is *not* ground truth. The real oracles, layered in as the work matures:

- **Symbols:** `universal-ctags` (multi-language) or tree-sitter itself.
- **Call/reference edges:** **SCIP** indexes from the mature per-language indexers
  (`scip-python`, `scip-typescript`, `scip-java`, …). These are the only realistic
  source of reference-grade edges for non-Go languages.

## Why this gates the extractor work

Every extractor change — the tier-honesty fix, the tree-sitter migration, SCIP edge
ingestion, the pure-Go config parsers — should be required to **move a number** in the
baseline report. Establish the baseline first; improve against it; never regress it.
See `workspace/artifacts/reports/2026-06-14-sextant-language-parity.md` for the full
plan and `2026-06-15-sextant-30-language-plan.md` for the per-language approach.
