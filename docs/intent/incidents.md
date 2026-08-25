# Incidents

Every incident yields an invariant, a test, or a decision. Open records are ≤10 lines;
closed ones compress to one line. The full friction catalogs stay where they are and are
not copied here: `docs/dogfood-observations.md` (O1–O29, root-cause narratives) and
`.fathom/observations.md` (O1–O19, `fathom observe` intake). Only incidents with a
consequence for intent are recorded below. Rationale: `agentic-kit/docs/project-intent/operating.md` §2.

Classes: gate-bypassed · wrong-done-claim · intent-violated · platform-fact ·
contract-broken · other. Record with `intent-incident`.

## Open

### INC-1 2026-08-25 Stale agent worktrees and branches accumulate
What: 24 checkouts under `.claude/worktrees/` and 22 `worktree-*` branches, the oldest
from July, none owned by a live session.
Evidence: `git worktree list`; `git branch` at bootstrap.
Would have caught it: the parallel-agent protocol's cleanup rule (operating.md §4: a
worktree older than its session is an incident).
Class: gate-bypassed
Outcome: pending (deletion needs the owner: `questions.md` Q3; then `autonomy.md` "Ask first" carries the rule)

### INC-2 2026-08-21 A wedged reader wrote a zero-symbol index that reported "0 failed"
What: five `bin/fathom ingest` runs under the external-volume EPERM wedge wrote 0 symbols
and exited 0; `fathom verify` then refuted 24 freshly proven rows.
Evidence: `.fathom/observations.md` O16; commit e1fd587.
Would have caught it: a read error counted in the failed column; a refusal to overwrite a
populated index with an empty one.
Class: wrong-done-claim
Outcome: pending (IT-14/15 in `docs/plans/instrument-truth.md`; scope G1)

### INC-3 2026-08-25 The post-commit hook does what the Index-first rule forbids
What: `lefthook.yml` post-commit runs a shell `bin/fathom ingest --service fathom … --store
workspace/db/fleet.sqlite` against the store the daemon serves, on every commit; this is
INC-2's exact write path, and `CLAUDE.md` says "refresh via the `ingest` MCP tool, never a
shell `fathom ingest` against the shared store."
Evidence: `lefthook.yml` post-commit `fathom-refresh`; `CLAUDE.md` Index-first row.
Would have caught it: a custodian concern that the hook's command matches the documented rule.
Class: intent-violated
Outcome: pending (`questions.md` Q2: route through the daemon or land IT-14/15 first)

### INC-12 2026-08-25 Law 15 holds in the engine, not on the production write path
What: `fathom init` and `fathom fleet reconcile --apply` (the verbs that write
`CLAUDE.md`, `AGENTS.md`, `.claude/settings.json` across 17 repos) do not call
`internal/wiring`: a hand edit inside the marker region is replaced, not refused, and no
receipt is written, so `fleet undo` cannot reverse them.
Evidence: `cmd/fathom/init.go` `applyStanza` (unconditional region replace, v1 markers
without a digest); `cmd/fathom/reconcile.go`; `internal/wiring` has one importer (`fleetwiring.go`).
Would have caught it: a test that the production verbs' writes appear in `receipts.jsonl`.
Class: intent-violated
Outcome: pending (`questions.md` Q11)

### INC-13 2026-08-25 The raw-`cp` deploy instruction survives outside the audit's corpus
What: `docs/adoption/claude-code.md` instructs `cp bin/fathom ~/.local/libexec/fathom/fathom`
in three places; the custodian's `deploy_safety` concern scans `CLAUDE.md`/`AGENTS.md` only
and reports "hold". INC-4 (O25) was this instruction being followed.
Evidence: `.fathom/concerns.cue` `where: "agent-config"`; `fathom config audit` 4 hold.
Would have caught it: the concern's corpus including `docs/adoption/**`.
Class: intent-violated
Outcome: pending (widen the concern or fix the doc; the doc edit is not part of this bootstrap)

## Closed

- INC-4 2026-08-12 The documented deploy (raw `cp`) took the daemon down fleet-wide (O25) → `.fathom/concerns.cue` deploy_safety; `CLAUDE.md` non-negotiable row; `task deploy` serving assertion.
- INC-5 2026-08-08 Every redeploy re-rolled the TCC grant (O10) → `domain.md` platform fact; `fathom-dev` signing identity in `task deploy`.
- INC-6 2026-07-24 Obligations injection inert for six days after the rename while hooks looked healthy (O15) → deterministic, never-collapsing service resolution (IO-1..3, `cmd/fathom/serviceident_test.go`); doctor `obligations_served` probe.
- INC-7 2026-08-09 "188 sessions" were 8; `Stop` fires per turn (MI-1..3) → `domain.md` platform fact; distinct-session denominators pinned by test.
- INC-8 2026-08-12 The transcript reader hand-copied three of twelve fields, silencing four instruments for weeks (O24) → `TestTranscriptSummaryCarriesEveryField` (reflection over the struct).
- INC-9 2026-07-07 The first gated run caught the plan's author over-claiming → the pre-push `fathom verify` gate itself (`lefthook.yml`).
- INC-10 2026-08-23 Two C++ rows flipped when unrelated CUE tests landed; cause was typed edges silently dropped on a discovery gap → VS-9 `DiscoveryGaps` named on stderr; resolver stability pinned (`docs/plans/verify-spine.md`).
- INC-11 2026-08-21 Push with `LEFTHOOK=0` after the gate ran green but the wedge corrupted transfer → `autonomy.md` "Never" tier names the only allowed form.
