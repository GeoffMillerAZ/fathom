# Carry-across after compaction: a hook experiment, not a feature

Status: accepted 2026-09-29 (ADR-056) as an experiment with a stop rule; the compaction advisor itself is
not a fathom feature. Vision: `docs/vision/40-judgment-and-exploration-maps.md`. Plan:
`docs/plans/carry-across.md` (CA-). Goal G31. Builds on continuity threads
(`docs/design/continuity-threads.md`, `docs/vision/21`, TH rows) and receipts (RC). Constraints carried in:
I12 (the SessionStart primer cap and the deliberate silence on resume and compact), P12 (no saving claim
without a counterfactual), P16 (delivery is configuration), the dispatch boundary.

## 1. The question and the verdict

The owner asked whether a compaction advisor (level 7 of the ten-levels series: a turn-end question set
that decides when to compact) is a fathom feature or a hook experiment. It is a **hook experiment, not a
feature**, and the only fathom-side piece worth testing is carry-across.

**Fathom cannot compact.** In Claude Code a hook cannot trigger compaction; PreCompact can block but cannot
inject context; only SessionStart with source `compact` can add context afterwards. UserPromptSubmit and Stop
can advise the model, and a system message reaches the user only. Acting on "compact now" needs a harness
that owns compaction (agentic-kit's harness Compactor, or a pi-style harness). What fathom uniquely has is
the **computed working set**: files edited, discoveries, plan rows touched and their verify state now,
failing certificates on touched symbols, served rules still unresolved, the latest handoff note, and the
blast radius of uncommitted changes, all computed at read time rather than remembered.

**It must first overturn a deliberate decision.** `cmdThreadBrief` (cmd/fathom/thread.go) says nothing on
resume and compact, because "context survives" and injecting there would repeat the schema-tax mistake, and
Claude Code re-attaches recently read files after compaction. I12 pins it (`fathom thread brief --source
resume|compact` prints nothing). Any carry-across must overturn that with a reading, not a preference.

## 2. Verify first

Confirm that SessionStart with source `compact` fires in the current Claude Code and accepts
`additionalContext` (Q32, reading CA-V1). If it does not, the experiment ends here.

## 3. Reading 0: a transcript-scan bench script, no product change

- **Scope:** the last 14 days of the owner's session transcripts (about 551 pre-compact events per 4 days).
- **Pre set:** for each compaction, the files read or edited in the 40 turns before it.
- **Re-reads:** Read, Grep and Glob calls on the pre set in the 20 turns after compaction, split into
  re-read then edited within 3 turns (legitimate) and re-read only.
- **Tokens and turns to the next proof event:** the next passing test run (from the test-run hook's sensor
  if present), otherwise the next `fathom verify` exit 0, or a plan-row flip.
- **Stop rule:** stop if the median compaction has fewer than 2 re-reads of the pre set, or re-reads are
  under 10 percent of all reads after compaction. If it stops, thread.go's decision stands and is recorded
  as confirmed.

## 4. Build, only if Reading 0 shows waste

At SessionStart(source=compact), inject a computed working set of at most 1,200 bytes, no model and no
egress: dirty files with their changed symbols; plan rows touched, with their verify state now; the last test
run's failing ids (from the test-run hook); rules served on touched files and not yet resolved; the latest
handoff-note pointer; active map ids. Opt-in via `[hooks] carry_across`. PreCompact keeps writing its
pointer. If this passes, ADR-056 records the reversal of `cmdThreadBrief`'s rule and I12's compact clause is
amended in the same change.

## 5. A/B

- **Assignment:** a seeded hash of the session id; half the sessions on, half off, on the owner's real
  traffic.
- **Duration:** until each arm has at least 150 compactions and at least 7 days have passed.
- **Primary metric:** re-reads of the pre set in the 20 turns after compaction.
- **Secondary:** tokens and turns to the next proof; sessions abandoned within 20 turns of compaction;
  injected bytes.
- **Pass:** the primary metric drops at least 30 percent with a bootstrap 95 percent CI excluding 0, and
  tokens-to-proof is not worse. **Kill:** otherwise remove the injection and keep thread.go's decision,
  citing the reading in its comment.

## 6. Deferred: when-to-compact advice (F38)

At UserPromptSubmit, only when context is above a line (from the transcript fold's last usage) and the
prompt is a task switch, judged by a noul over the head of the previous prompt (from a session memo) and
the current prompt, through a daemon endpoint under the deadline, as a system message to the user and never
to the model. It is the same prompt-classification shape as the route site, so it waits for that site's
re-admission (`docs/design/judgment-foundations.md` section 6) as well as for carry-across to pass. Without
carry-across the advice is a generic token heuristic Claude Code already shows. The act of compacting
belongs to a harness that owns compaction (agentic-kit), outside fathom; a level 7 port there uses turn-end
questions (switched gears, at a boundary, mid-operation) and token lines, and fathom supplies the
carry-across through a CLI. Cost of the advice: one noul of about 1k tokens per qualifying prompt (about
$0.00004). Calibration: task switch against owner labels on 100 prompt pairs, or settled by the next plan row
or thread touched. Bar: advice followed at least 1 in 3 times over 30 days.

## 7. Cost and calendar

$0 in model spend; a bench script plus a small hook change; about 1 to 2 weeks of calendar time (the A/B
duration binds).

## 8. Readings

| Reading | Threshold |
|---|---|
| CA-V1 | SessionStart(source=compact) fires and accepts additionalContext in the current Claude Code |
| CA-M1 | Reading 0: median re-reads of the pre set at least 2, and at least 10 percent of post-compaction reads, or the stop rule is recorded |
| CA-M2 | the A/B: re-reads down at least 30 percent, CI excluding 0, tokens-to-proof not worse |
