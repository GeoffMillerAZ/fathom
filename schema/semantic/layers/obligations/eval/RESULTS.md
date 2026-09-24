# obligations — eval results

Gate: docs/design/semantic-layers.md §4.6 — precision ≥ 0.90 on (claim,
probe) application pairs, recall ≥ 0.75 on labeled rules. Run via the shared
harness (`internal/semantic/eval_test.go`'s `runLayerEval`,
`TestObligationsEval_Precision`). Fixtures: `standards.md` + `security.md`
(7 labeled rules incl. 2 scope traps and one glob-hostile behavioral rule) —
unchanged since the layer's original ADR-016 admission; the prompt is
identity-pinned (`obligations/c02b1aa1`) and was NOT iterated here.

## 2026-09-24 — production-model recheck (CV-M9, docs/plans/refactor-conservation.md): CLEARS

```
FATHOM_EVAL_LLM=1 FATHOM_LLM_PROVIDER=anthropic FATHOM_EVAL_MODEL=claude-haiku-4-5-20251001 \
  go test -count=1 ./internal/semantic -run TestObligationsEval_Precision -v
```

| Run | Model | Claims emitted | Rejected | Precision | Recall | Missed rules | Gate |
|-----|-------|-----------------|----------|-----------|--------|--------------|------|
| 1   | claude-haiku-4-5-20251001 | 6 | 1 | 1.00 | 0.86 | no-secret-logging | pass |
| 2   | claude-haiku-4-5-20251001 | 6 | 1 | 1.00 | 0.86 | no-secret-logging | pass |

**Reading:** two independent runs (fresh `-count=1` invocations, not a cached
replay — output tokens differ 445/447, confirming both hit the API) land
exactly on the 2026-07-17 run 2 reading: precision 1.00, recall 0.86,
`no-secret-logging` (security.md's fleet-wide rule) missed both times. The
2026-07-17 instability (0.71 in run 1, an escalated pre-OP-6 signal) did NOT
reproduce here — both runs today clear the 0.75 bar with the same miss
pattern, not a worse one. No prompt iteration was attempted (the prompt
stays identity-pinned, same discipline as the 2026-07-17 entry); recall is
above the bar as measured, so CV-M9's "improve extraction" branch does not
apply this round.

**Disposition:** the pre-OP-6 escalation (`no-secret-logging` missed under
the production model) stands as an open, tracked gap — not a gate failure,
since both today's runs and 2026-07-17's run 2 clear 0.75 — but a real,
repeated miss worth the next prompt bump (vision 04 Gap 3) folding in
`no-secret-logging` recall, per the original escalation's option (b).

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
