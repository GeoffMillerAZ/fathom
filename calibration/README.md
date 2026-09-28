# calibration/ — the extractor calibration corpus

This directory is the **measurement instrument** for fathom's language extractors.
Today only Go is parsed with a real AST (`go/parser`); every other language is a
regex/line heuristic that has only ever been tested against ~10 lines of synthetic
fixture. You cannot drive an extractor to "excellent" without measuring it against
**real-world code**, and you cannot measure recall without a **ground-truth oracle**.
This harness provides both.

## What's here

| File | Role |
|---|---|
| `corpus.yaml` | The curated manifest: per language/tool, the chosen public sample repos (pinned to a real commit SHA), plus the decided extraction approach (pure-Go parser vs tree-sitter vs SCIP) and the symbol kinds an excellent extractor should emit. **Checked in.** |
| `corpus.d/<id>.yaml` | One shard per language-breadth queue item (`docs/design/language-breadth.md` §13.3): `id` (the queue id, equal to the file name), `language`, and `repos` with `name`, `url`, `ref` (a release tag), `sha` (from `git ls-remote`), `licence` (OSI-approved; read, never vendored), optional `subpath` (repos over 50 MB or 5,000 files of the language — cloned blob-less and sparse) and 3–6 `idioms`. Workers add shards here instead of editing `corpus.yaml`, so parallel items never collide. |
| `clone.sh` | Shallow-clones (`--depth 1`, single-branch) every repo in the manifest into `workspace/calibration/<lang>/<repo>` (gitignored). Idempotent. |
| `baseline.sh` | Ingests each cloned repo through the **real** fathom pipeline and reports symbols-found per extractor, vs a rough grep-based "expected definitions" proxy, to expose recall gaps. Writes `workspace/calibration/baseline-report.md`. |

The cloned source itself never enters git — only the manifest does. This keeps the
repo lean and license-clean while staying fully reproducible (URL + pinned SHA).

## Usage

```sh
task calibrate                       # clone the whole corpus + run the baseline
task calibrate-clone -- Python Helm  # clone only some languages/tools
task calibrate-clone -- swift        # clone one shard (corpus.d/swift.yaml) into workspace/calibration/swift/
task calibrate-clean -- swift        # delete that shard's clones
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
See `workspace/artifacts/reports/2026-06-14-fathom-language-parity.md` for the full
plan and `2026-06-15-fathom-30-language-plan.md` for the per-language approach.
