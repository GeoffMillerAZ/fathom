# findings — eval results

Gate: docs/design/semantic-layers.md §4.6 — precision ≥ 0.90 on (claim, probe)
application pairs, recall ≥ 0.75 on labeled rules. Run via the shared harness
(`internal/semantic/eval_test.go`'s `runLayerEval`,
`TestFindingsEval_Precision`).

## Status: NOT YET RUN

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
