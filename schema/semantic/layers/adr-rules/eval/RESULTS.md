# adr-rules — eval results

Gate: docs/design/semantic-layers.md §4.6 — precision ≥ 0.90 on (claim, probe)
application pairs, recall ≥ 0.75 on labeled rules. Run via the shared harness
(`internal/semantic/eval_test.go`'s `runLayerEval`,
`TestADRRulesEval_Precision`).

## 2026-07-17 — PASS (production model, expanded fixtures; 2 consecutive runs)

The AUTHORITATIVE gate record. Fixture corpus: `decisions.md`, expanded to
the obligations precedent's shape — 7 labeled rules across 7 packages, 2
labeled scope traps (Consequences paragraphs that name what a decision does
NOT bind), plus a rejected-alternative section that must produce no claim.

- **Lane:** `claude-cli` (subscription, no API key)
- **Model:** `claude-haiku-4-5-20251001` — pinned to the PRODUCTION compile
  default (`compileModel()`, cmd/fathom/compile.go), so these metrics
  transfer to the fleet-compile lane
- **Command:**

```
FATHOM_EVAL_LLM=1 FATHOM_LLM_PROVIDER=claude-cli \
  go test ./internal/semantic -run TestADRRulesEval_Precision -v
```

| Run | Model | Claims emitted | Rejected | Precision | Recall | TP | FP | FN | Missed |
|-----|-------|-----------------|----------|-----------|--------|----|----|----|--------|
| 1   | claude-haiku-4-5-20251001 | 7 | 0 | 1.00 | 1.00 | 7 | 0 | 0 | none |
| 2   | claude-haiku-4-5-20251001 | 7 | 0 | 1.00 | 1.00 | 7 | 0 | 0 | none |

Exactly one claim per labeled rule in both runs; both traps and the
rejected-alternative section correctly produced nothing.

**Result: PASS.** adr-rules ships in the embedded set.

## History

### 2026-07-17 (earlier, superseded) — PASS on the initial 3-rule fixture

Two consecutive 1.00/1.00 runs on the original 3-rule/1-trap fixture,
`claude-cli` lane with the CLI's own (unpinned) default model. Superseded by
the authoritative record above after the coordinator's critic asked for (a)
fixture counts matching the obligations precedent and (b) an explicitly
recorded model.

### Iteration note (fixture authoring, kept for future pack authors)

The very first run scored precision 0.50 / recall 0.67 — both FIXTURE bugs,
not prompt/model quality: `labels.json` `cite_substr` values spanned a
Markdown line-wrap boundary in `eval/decisions.md` (`\n`-joined cited text
broke a substring check that assumed single-line contiguity), and one
Decision paragraph packed two independent clauses the model reasonably
split into two claims, one of which then had no matching labeled rule.
Fixed by keeping each Decision to one governing clause and choosing
`cite_substr` values that live entirely within one source line. The
adr-rules prompt itself was never iterated.
