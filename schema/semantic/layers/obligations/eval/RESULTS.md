# obligations — eval results

Gate: docs/design/semantic-layers.md §4.6 — precision ≥ 0.90 on (claim,
probe) application pairs, recall ≥ 0.75 on labeled rules. Run via the shared
harness (`internal/semantic/eval_test.go`'s `runLayerEval`,
`TestObligationsEval_Precision`). Fixtures: `standards.md` + `security.md`
(7 labeled rules incl. 2 scope traps and one glob-hostile behavioral rule) —
unchanged since the layer's original ADR-016 admission; the prompt is
identity-pinned (`obligations/c02b1aa1`) and was NOT iterated here.

## 2026-07-17 — production-model check (PRE-OP-6 SIGNAL, escalated)

First recorded runs against the PRODUCTION compile default —
`claude-haiku-4-5-20251001` (`compileModel()`'s default,
cmd/fathom/compile.go) via the `claude-cli` subscription lane. The layer's
original gate passes predate the provider matrix and ran on a different
(stronger) model.

```
FATHOM_EVAL_LLM=1 FATHOM_LLM_PROVIDER=claude-cli \
  go test ./internal/semantic -run TestObligationsEval_Precision -v
```

| Run | Model | Claims emitted | Rejected | Precision | Recall | Missed rules | Gate |
|-----|-------|-----------------|----------|-----------|--------|--------------|------|
| 1   | claude-haiku-4-5-20251001 | 5 | 2 | 1.00 | **0.71** | no-time-now, no-secret-logging | **FAIL (recall)** |
| 2   | claude-haiku-4-5-20251001 | 6 | 1 | 1.00 | 0.86 | no-secret-logging | pass |
| (ref) | claude-cli default model, unpinned (2026-07-17, earlier session) | 4 | 3 | 1.00 | **0.57** | no-time-now, bounded-output, no-secret-logging | **FAIL (recall)** |

**Reading:** precision is a stable 1.00 across every run — the trust-fatal
axis holds; served rules are never wrong. Recall under the production model
is UNSTABLE around the 0.75 bar (0.71 / 0.86), with `no-secret-logging`
(security.md's fleet-wide rule) missed in BOTH pinned-model runs and
`no-time-now` missed in one. The recurring `sorted-output` scope FN
(`docs/notes.md` not matched by `**/*.go`) is the known glob-hostile
fixture case, not a regression.

**Disposition:** the obligations prompt is identity-pinned — iterating it
rotates `obligations/c02b1aa1` and forces a fleet recompile (a deliberate
flag-day, docs/vision/02-shape.md M1), so no fix is attempted in the
layer-catalog slice. Escalated to the coordinator as a pre-OP-6 input: the
production compile lane under-recalls this layer's own fixtures, so either
(a) the fleet compile model moves up a tier for obligations, or (b) the
next planned prompt bump (the serve-time precision pass, vision 04 Gap 3)
folds in recall hardening and re-gates under the pinned production model.
