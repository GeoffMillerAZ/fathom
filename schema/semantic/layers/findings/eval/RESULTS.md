# findings — eval results

Gate: docs/design/semantic-layers.md §4.6 — precision ≥ 0.90 on (claim, probe)
application pairs, recall ≥ 0.75 on labeled rules. Run via the shared harness
(`internal/semantic/eval_test.go`'s `runLayerEval`,
`TestFindingsEval_Precision`).

## 2026-09-24 — PASS (CV-M10, docs/plans/refactor-conservation.md)

The 2026-07-30 blocker below is FIXED: `RunEval` (`internal/semantic/eval.go`)
now seeds a deterministic fingerprint for every `labels.json` probe path
before compiling — the same contract a real ingest's fusion pipeline
satisfies via `SetFingerprint` on every processed file, which a fresh
in-memory eval store never had. Regression-locked, no tokens spent, by
`TestRunEval_DriftTrackedLayerSeedsProbeFingerprints`
(`internal/semantic/eval_mock_test.go`).

```
FATHOM_EVAL_LLM=1 FATHOM_LLM_PROVIDER=anthropic FATHOM_EVAL_MODEL=claude-haiku-4-5-20251001 \
  go test ./internal/semantic -run TestFindingsEval_Precision -v
```

| Run | Model | Claims emitted | Rejected | Precision | Recall | Missed rules | Gate |
|-----|-------|-----------------|----------|-----------|--------|--------------|------|
| 1   | claude-haiku-4-5-20251001 | 6 | 0 | 1.00 | 1.00 | none | pass |
| 2   | claude-haiku-4-5-20251001 | 6 | 0 | 1.00 | 1.00 | none | pass |

**Reading:** both runs are a clean sweep — all 6 labeled findings (one per
class: resource-leak, error-handling, concurrency, security, performance,
contract-violation) matched with zero false positives against the 7-probe
corpus (including the two traps, `docs/design/notes.md`'s background-only
section and the resolved-status finding, correctly ungoverned). The findings
prompt's `class` field discipline (docs/design/findings-as-evidence.md §6
item 5) held under the production compile model with no iteration needed.

**Disposition:** `schema/semantic/layers/findings/layer.cue`'s `evaled` is
now `true` (mirrored identically into
`internal/semantic/layers/findings/layer.cue`, `TestPackFilesMatchSource`).
`fathom compile --layer findings` no longer refuses a non-dry-run compile
without `--unsafe-skip-eval-gate`, and `internal/analysis/findings.PromoteTier`
may promote a pattern's tier per its own §3.5 rules — the eval gate that was
blocking both is cleared. `docs/layers.md`'s catalog row is updated to match.

## Status (historical): RUN ATTEMPTED 2026-07-30 — BLOCKED BEFORE SCORING (not a precision failure)

    FATHOM_EVAL_LLM=1 go test -run TestFindingsEval_Precision ./internal/semantic/
    eval lane="anthropic" model="claude-sonnet-4-6"
    RunEval: semantic: eval compile: semantic: compile
      corpus.md#wa-101-connection-pool-leak-on-error-path:
      semantic: finding drift anchor: indexed file internal/store/pool.go
      has no stored fingerprint
    --- FAIL (6.42s)

**The gate did not score.** The compile aborted before emitting claims, so
no precision/recall number exists and none may be quoted. The cause is a
fixture-shape gap, not model quality: the **findings layer requires
scope-fp links** (the `code_drifted` remediation-drift mechanism, FT-6) and
therefore requires every scoped file to carry a stored fingerprint in the
eval store. `adr-rules` and `obligations` — whose fixture shape these
fixtures were modeled on — have no such requirement, so the convention that
was copied is insufficient for this layer.

Fix direction (one focused slice, see O21): the eval harness must seed the
scratch store with the fixture's scoped files (or the fixtures must scope
only to files the harness already indexes). Until then `layer.cue` keeps
`evaled: false`, and `PromoteTier` structurally refuses promotion whenever
`!layerEvaled` — so nothing serves on an unproven gate regardless of caller
discipline.

`eval/{labels.json,corpus.md}` were authored by docs/design/findings-as-
evidence.md's §6 item 5 (2026-07-30): 6 labeled findings, one per class
(resource-leak, error-handling, concurrency, security, performance,
contract-violation), across 6 probe files, plus 2 traps (a finding marked
`Status: resolved` and a background/context-only section) that must produce
no claim — mirroring adr-rules' own fixture shape and admission bar. The
prompt (`prompt.md`) was extended in the same change to require the `class`
field the reviewer must emit alongside `rule`/`scope`/`cite_start`/
`cite_end`.

**This gate has not yet been run against a real model** — no
`ANTHROPIC_API_KEY` or logged-in `claude` CLI was available in the session
that authored the fixtures, and spending real tokens on a fresh layer's eval
run is a deliberate, budgeted decision, not something to do silently inside
an unrelated task. `layer.cue`'s `evaled` field stays at its default
(`false`) — `fathom compile --layer findings` still refuses a non-dry-run
compile without `--unsafe-skip-eval-gate`, and
`internal/analysis/findings.PromoteTier` still refuses to promote any
pattern's tier (§3.5) — until a real run passes and this file is updated
with the record, exactly like adr-rules/spec-bindings were.

**To run it:**

```
FATHOM_EVAL_LLM=1 FATHOM_LLM_PROVIDER=claude-cli \
  go test ./internal/semantic -run TestFindingsEval_Precision -v
```

(or `FATHOM_LLM_PROVIDER=anthropic` with `ANTHROPIC_API_KEY` set — see
`internal/semantic/eval_test.go`'s `evalLLM` for both lanes). On a PASS
(precision ≥ 0.90, recall ≥ 0.75), record the run here in the adr-rules
format (model, lane, per-run TP/FP/FN table) and flip
`schema/semantic/layers/findings/layer.cue`'s `evaled` to `true` (updating
`internal/semantic/layers/findings/layer.cue`'s embedded mirror
identically, per `TestPackFilesMatchSource`'s drift gate) in the SAME
change as this file's update — never one without the other.
