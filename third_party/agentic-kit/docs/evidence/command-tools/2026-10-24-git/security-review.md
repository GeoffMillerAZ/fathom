# GT6 security review and repaired failures

An independent reviewer applied differential-review and sharp-edges to the candidate
against `e22ca0d`, starting with Fathom review_diff. The shared index was stale, so
the reviewer used current source and bounded scratch probes, not its stale verdict.
A later independent reread covered root integration's help/parser delta. Final source
identities and native binary identity are in [the qualification record](README.md).
No confirmed security blocker remained in either final review scope.

| Repaired finding | Evidence and resulting behavior |
|---|---|
| Selectors escaped disclosure checks | A secret-marked selector leaked on failure or zero-row success. Scope now participates in normalized scanning and is absent from every non-complete report. |
| Unborn ref probe assumed the wrong Git exit grammar | Native Git returned exit 128/stderr for the old `show-ref` probe. Fixed quiet ref verification accepts only the qualified silent missing-ref result. |
| HEAD resolved twice without provenance | Each distinct ref now resolves once; successful diff/history reports carry scanned resolved commit identities. HEAD-to-HEAD uses one immutable commit. |
| Unexpected path spellings were skipped | Any path outside the literal request now holds instead of silently disappearing from the result. |
| Normal signed commits were rejected | Bounded signature/mergetag continuations remain raw scanned source, without claiming signature verification. Empty messages are accepted. |
| Failure output exceeded tiny valid budgets | Serialized report plus newline must fit before writing. Otherwise exit 2 emits no bytes. This rule applies to the new Git profile. |
| CUE allowed source data on failed reports | A direct CUE counterexample previously admitted secret-marked scope/rows. Closed failure variants now forbid them; complete variants bind the typed operation and required provenance. |
| Empty records hid malformed output | Five independent root regressions failed before the parser fix. Empty streams remain valid; empty records, ignored diff tails and invalid/duplicate history lookahead IDs now hold. |

The root regression failures are retained along with the independent reviewer report.
Focused regression/help tests passed after the final narrow reread; full race/gate
and native corpus results are separate instruments in the qualification record.

Two remaining limitations were accepted explicitly: the 65,536-entry metadata ceiling
can hold a healthy large repository, and a context cannot interrupt a blocked filesystem
operation. They are documented support limits, not relaxed gates or security claims.
Operator write protection remains an external prerequisite. No OS sandbox, universal
Git safety, complete history, atomic status, source truth or disclosure authority was
established by this review.
