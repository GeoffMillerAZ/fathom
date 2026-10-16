# Interception seams: where fathom can see or shape what an agent receives

Owner-architect assessment, 2026-10-16, summarized into the repository the same day. Read at fathom `ebc3ce70` with
the readouts and mods records on top. The pass's notes sit in a gitignored workspace file and are not the record. The
map, the value scale and the conflicts live in `docs/design/readouts.md` section 20; this file is the reading that
led to it.

## Verdict

Thirteen hook seams, H1 to H13. Two are Very high: the readouts view on PostToolUse(Bash) (H1, already planned) and
a deterministic claim check at Stop and SubagentStop against the session's recorded test runs (H2, new). Four are
High (H3 the post-compaction working set, H4 tests to run after an edit, H6 a static gate preflight on `git commit`
and `git push`, H9 subagent briefs from the task prompt); six are Medium (H5, H7, H8, H10, H11, and H12 in the mods
program); one is Low (H13, recorded as F79). The owner asked that every Medium or higher seam be
implemented and working; each now ends with a plan row whose bar is a reading over at least 7 days of real sessions
with no false answer (goal G41).

## What was read, graded

| Claim | Grade | Source |
|---|---|---|
| Claude Code documents `additionalContext` on Stop, SubagentStop, SubagentStart, PreToolUse, PostToolUse, PostToolBatch and UserPromptSubmit | read from fathom's codec comment (2.1.270); delivery to the model at Stop and SubagentStop unmeasured | `internal/harness/codec.go:438-450`; Q32 (PR-V2, PR-V3) |
| fathom has no SubagentStop registration today | measured | `internal/harness/harness.go` (no such event) |
| SubagentStart carries no task prompt | measured (XM-V1) | `docs/intent/questions.md` Q32 |
| Post-compaction re-reads are rare | measured: CA-M1 STOP, 0.5 against a bar of 2 | `docs/bench/readings/carry-across/reading0.md` |
| "About 550 pre-compact pointers in 4 days" | not reproducible: 9 compactions in 14 days | the same reading |
| Commit-to-commit cache-from is cheap | refuted: 0.81x cold (IXS-M0 CUT); a near-zero diff cost 16 to 24 s | `docs/bench/readings/index-snapshots/cache-from-timing.md` |
| Exact repeats are worth deduplicating | refuted for exact repeats (0.03% of calls); overlap unmeasured | `docs/design/readouts.md` section 1 |
| ADR-071 amended R-12 (ADR-056 decision 13) | it had not; amended in this change | `ADR.md` ADR-071 and ADR-056 status lines |

## Decisions it drove, and where they are recorded

- The map, the scale and the conflicts: `docs/design/readouts.md` section 20.
- Rows: `docs/plans/readouts.md` (RO-V1, RO-V2, RO-37 to RO-47, RO-M8 to RO-M20, and RO-M3's 7-day window),
  `docs/plans/claude-code-mods.md` (MOD-20, MOD-M6), `docs/plans/index-snapshots.md` (IXS-M10, IXS-14, IXS-M11).
- The R-12 amendment: ADR-071 and ADR-056 status lines; TR-X2's reason.
- The bet: goal G41. The Low seam: future F79. The sequence: roadmap section 9.8.
