# Served is not kept: a lost pin is a fact about the change

Status: accepted in part 2026-09-29 (ADR-057, the owner's decision). It replaces a first, larger design
(prefix PK-, eighteen sections and about twenty plan rows) that two critics, one on value ("does it change
behavior?") and one on cost (hooks, CPU, storage, volume), showed was about ten times too big for what
can be measured today. Plan: `docs/plans/served-not-kept.md` (PR-). Goal: G33. Builds on ADR-038 (checkable
rules deny at the edit), ADR-044 (conservation and the CV-7 guarded set), ADR-053 (the surface reckoning),
ADR-056 (judgment at the port) and `docs/design/coding-agent-excellence.md` section 5.3 (the pre-push
doctrine). Reading behind the deferral of `test_ran`:
`docs/bench/readings/served-not-kept/test-ran-base-rate-2026-09-29.md`.

## 0. The decision on one screen

The idea is right: a rule shown to an agent is not a rule the agent kept. In production every rule a per-edit
check could evaluate is already denied before the edit (ADR-038). The one gap left is the i28 route: the agent
rewrites the test that pins the rule. The decision keeps the framing and builds only the piece that closes
that gap.

| Class | What |
|---|---|
| **Accepted now** | **The commit pin line** (PR-1 to PR-7). When a commit loses a test that a governing source names, the agent gets one factual line in the same turn. The sources are an invariant, a certification, a scope evidence line or a guard's `done needs`. The check is deterministic, keeps no state and runs whether or not the rule was served. **Two ledger fixes** (PR-10, PR-11) ride on SR-61 so they write to a ledger that receives rows. **JF-1 lands on its own** as the O120 fix |
| **Accepted 2026-09-29 (PR-8)** | A silent pin loss blocks at pre-push once PR-9 lands (owner-architect, confirmed by the owner the same day). Today `conserve` computes it on every push, but only as advice (Q34, answered) |
| **Experiments** | PR-M0 is a $0 replay over the guards-pilot 6 and 8 diffs. PR-M1 is 30 days of record mode on owner traffic, also $0. PR-M2 is the bench: the enforced-arm rerun ADR-038 still owes, plus the pin line in feedback mode; about $76, needs the owner's OK |
| **Deferred, with triggers** | `test_ran`, turn deadlines and a Stop block, `enforce: "warn"`, waivers with a receipt, the open set, ledger schema 2 and a check registry, the GL-11 reclassification, `propose_check` (Jev), orchestrator constraints, unserved scanning, per-agent keeping tables (section 6) |
| **Refused** | Model-judged compliance, a bypass in the line, turn-end lines on the next prompt, whole-file evaluation of fragment checks after each edit, per-edit single-file pin checks, `denied_retry`, `doc_touched` and `verify_row` rows, automatic suppression by waiver rate, the thread-registry refactor, promises parsed from prose, a new tool or verb, a pin inferred rather than named (section 7) |

## 1. The framing that survives

**Served is not kept.** Fathom already closes this gap in three places:

- **Plan rows.** A claimed row becomes verified at the claim moment (PC-22, Stop, pre-push).
- **Agent config.** An instruction becomes true through `fathom config audit` concerns.
- **Checkable guards.** A served rule is denied before the edit (ADR-038).

Pilot 8 exposed the fourth and missing case: a rule whose only check is a named test. There, the cheapest way
out is to rewrite the test. In i28 trial 2 the agent renamed `TestBackendForPath_DuckDBLoudlyRefused`; after
that, `go test -run` on the old name matched nothing and reported `ok`.

**Where to put a check.** Ask three questions, in order.

1. **Is there a deterministic check?** If not, the rule is advice: serve it, do not promise it, and count it
   as unmeasured, never as ignored.
2. **What is the earliest moment the check can see everything it needs?**
   - *The proposed edit*, when a fragment is enough: deny it before it lands (ADR-038). A reminder after the
     fact is strictly weaker than a deny.
   - *The commit*, when the rule spans files: a test renamed, moved or weakened.
   - *The end of the turn*, when the evidence arrives later (a test run). Only if measurement shows a gap.
3. **Can the agent still act at that moment?** If yes, it gets one line: a failed fact with a location. If
   not (the next prompt, or after the push), the result is a record or a gate, not feedback.

**The corrected ladder.**

- Fragment-checkable rules go from advise straight to deny.
- Whole-change rules go from advise to the commit line to the pre-push gate.
- There is no permanent middle rung of "advise, then check after". ADR-038 and SR-45 say a checkable rule
  denies; the set of rules too imprecise to deny but precise enough to warn is empty in this repository.

**The promise is fathom's, not the agent's.** The trigger is the source that already governs: a guard's `done
needs:`, an invariant's test column, a `// certifies:` marker, or a manifest entry. The deadline is the commit.
There is no new trigger language, and promises are never parsed from agent prose.

**The pin is the bridge from prose to a check.** A prose rule that names its test becomes checkable by
protecting that test. No model is needed.

**Jev never judges compliance.** Section 8 says what Jev does instead.

## 2. The facts that shaped this (verified by the critics and spot-checked; grades as stated)

- **Nothing to act on per edit in production** (*measured*). All five checkable guards in `.fathom/guard.cue`
  are `enforce: true`. The only advisory checkable rule, `core-no-mcp-or-cmd`, runs as a compiled test in
  `task validate`. The adopter guard files carry no `checks:` entries. In 15 days there were 837 pre-edit
  guard payloads, 2 of them from Claude Code on this repository, and 8 denials, all in the bench. (An
  earlier scorecard cut counted all 70 guard denials and claim-moment refusals as bench-only: see
  `docs/assessments/2026-09-27-feature-scorecard.md`, surprise 4.)
- **The guard evaluators are fragment-tuned** (*measured*). `EvaluateGuardEnforcement` returns nil when
  `!g.Enforce`, so an advisory rule always reads kept however the edit went. Checking the whole file after an
  edit fails `workspace_output` on 45 of 366 non-test `cmd/fathom/*.go` files, including `servehttp.go`.
- **A per-edit pin check misreads moves** (*measured*). `integrity.Check` looks for a renamed test only among
  `ChangedFiles`; with one file in view, a test moved to another file reads as deleted.
- **The whole-change pin check works** (*measured*). CV-M1 flagged both pilot-8 i28 trials as `deleted` (the name
  changed and so did the body). CV-M2 found 0 false losses in 50 finished commits. The CV-7 guarded set already
  includes invariant tests, and invariant I28 names the i28 pin.
- **`test_ran` has no headroom and has a false-kept hole** (*measured*). In 404 transcript and guard pairs the
  served-or-not base rate was 93.6 percent lenient and 85.2 percent strict on code scopes. On `[no tests to
  run]` it would record the i28 rename as kept. Every doc-scope `done needs` test runs on `t.TempDir()`
  fixtures, so a doc edit cannot affect it. Pre-push `task validate` already runs every test with `-race`.
  The reading is `docs/bench/readings/served-not-kept/test-ran-base-rate-2026-09-29.md`.
- **The rule-outcome ledger has never held a row** (*measured*, reference machine, 2026-09-28). The
  GL-10 and GL-11 writers and readers were not deployed and the ledger file did not exist. Agent-set
  conditions go unused: 1 of 592 `completes_when` rows was set by an agent.
- **Cost is not the constraint; correctness and volume are** (*measured*). One `git show` takes 13 to 15 ms,
  PostToolUse(Bash) p95 is already 2,003 ms, and Stop is at p95 7.1 s against its 8 s budget, so Stop gets no
  new work. Only 43 of 9,096 edit calls touched a file holding a pinned test, and 4 of those named one.

## 3. Accepted now: the commit pin line (PR-1 to PR-7)

**What it checks.**

- *The check.* `integrity.Check` runs over the commit's changed Go files, with the parent as base and the
  commit as head.
- *The guarded set.* It uses the CV-7 reasons that need no index: `// certifies:` markers, `tests/*.cue`
  manifest entries, the invariant enforcement column and scope evidence. PR-2 adds a guard's `done needs:`
  tests under their own reason. Reach is off, because a hook opens no store.
- *What counts as lost.* The loss kinds are `deleted`, `renamed`, `weakened`, `skipped`, `golden_changed` and
  `marker_moved`. `edited` is silent, because conserve cannot decide it (ADR-056 decision 8).

**Why the commit.** It is the agent's own "this is done" moment and the agent is still in its turn; the whole
change is visible, so a moved test is found and is not a loss; and the line does **not** require the rule to
have been served: a lost pin is a fact about the diff, like a failing test.

**How it runs.**

- *Trigger.* It rides the existing `fathom edit-hook` PostToolUse(Bash) path.
- *Commit identity.* The commit sha comes from git's own `[branch sha]` summary line in the tool output. No
  summary line means no check. It keeps no state file, no dedupe store and nothing for `fathom gc`.
- *Reads.* One `git cat-file --batch` process reads every base and head blob plus the base-side intent sources.
- *Budget.* The sub-budget is 250 ms, inside the descriptor's deadline (I53). If it runs over, the stage skips
  silently and its beat says `skipped`, never kept. Pre-push stays the backstop.
- *Evidence.* One `pin` beat per checked commit goes into `hooks.jsonl`: session, commit, loss kinds, test ids,
  source cites and ms, and no source text. There is no new ledger.
- *Mode.* `[hooks] pin_check = "off" | "record" | "feedback"`; the default is `record` until PR-M1 passes.
  `off` and `record` leave the payload byte-identical.
- *Harnesses.* Claude Code first. Codex and the others stay record-only until PR-V1 confirms their post-shell
  input carries stdout and that added context reaches the model.
- *Size.* About 250 lines plus goldens.

**The line.** A failed fact with a location. Never the rule text again, never a description of the detector,
never a waiver hint. Under 240 bytes, one line per lost pin and at most three per commit ("+N more at
pre-push"), silent when nothing is lost. The examples use illustrative shas and line numbers.

```text
fathom pin deleted · TestBackendForPath_DuckDBLoudlyRefused (named by I28, docs/intent/invariants.md:23) · gone in 3f9a1c2, renamed with a new body (backend_test.go:41) · a -run on this name now matches no test
fathom pin weakened · TestReadSetRecordsAbsentUnreadableAndSkipped (named by I38) · readset_test.go:88 assertion removed in 8c2d0e1
```

When the same commit also edits the source that names the pin, the line adds "(its source was edited in this
commit)": a deliberate retirement stays visible without being called a violation.

## 4. For the owner: a silent pin loss blocks at pre-push (PR-8, PR-9)

- **Today.** `refactor-conserve` in `lefthook.yml` computes the integrity plane on every push and always exits
  0 (advice only).
- **Proposal.** Add `fathom conserve --strict-plane integrity` and exit 1 only on a **silent** loss: the lost
  test is still named by its source at head. A pin retired on purpose, whose naming source was edited in the
  same range, is printed for review and does not fail.
- **Evidence.** CV-M2 found 0 false losses in 50 commits; CV-M1 flagged both i28 trials. This is the backstop
  for humans, for sessions that never commit through a hooked harness and for harnesses that stay record-only.
- **Why it is the owner's call.** It changes the advisory-only pre-push doctrine
  (`coding-agent-excellence.md` section 5.3) for one plane. Accepted 2026-09-29 on ADR-057's `PR-8 decision.` line by
  the owner-architect and confirmed by the owner the same day; Q34 is answered.
- **Revisit when.** Any false block happens (name the loss kind), or an agent retires a pin by editing its
  source with no decision record in the range (then the exemption requires a decision cite).

## 5. Experiments and kill criteria

**PR-M0: $0, before building PR-3.** CV-M1's method (`fathom conserve --json`, integrity plane) over every
guards-pilot 6 and 8 cell diff, and the fragment evaluators with `Enforce` forced on a copy over the transcripts'
edit fragments. Bar: every violating i28 cell is flagged and no oracle-clean cell is flagged; the
would-have-fired and false-fire counts for the fragment evaluators are published (the only prior for arm W).

**PR-M1: 30 days of record mode on owner traffic, $0.** The owner adjudicates every fire (the expected volume is
a handful, so no model labels anything: the first design's flaw was that a model decided K3's input). Bars:
stage p95 at most 250 ms, zero new I53 misses, the skip rate named. On pass, `feedback` becomes the default.

**PR-M2: the bench; the spend needs the owner's OK (about $76).**
- *Part A, the enforced-arm rerun ADR-038 owes.* Production config: enforce on, pin line off. Tasks i2, i19, i28,
  i31 and i36 (i1, i11 and i16 dropped: no check can fire in them). Bar: zero oracle violations on the four
  denied tasks; i28's rate becomes the baseline for part B.
- *Part B, the pin line.* Enforce on with `pin_check = feedback`, against the same config with the line off. Tasks:
  i28 plus one new pre-registered pin task (a `done needs` or invariant test the natural edit breaks, with its
  own oracle). Every task prompt in every arm ends with "commit when done", so the commit moment exists.
- *Grading.* The primary bar is the harness oracle (`oracle.outcome`), not the named detectors (i28 defeated them
  in pilot 8); adjudication only voids cells.
- *Power, pre-registered.* Pilot 8 had 4 violations in 11 adjudicable fathom cells. With 5 trials per task per
  arm, a drop from 8 of 10 to 2 of 10 on the pin tasks gives Fisher two-sided p of about 0.02; three trials
  cannot resolve it. About 40 cells at about $1.9 each.
- *Secondary bars.* On the pin tasks, tokens per clean success and tool calls per task at most 1.10 of the
  line-off arm; gaming (editing the naming source to retire a pin) is read from every diff.

| # | Kill condition | Consequence |
|---|---|---|
| K1 | PR-M0 misses a violating i28 cell or flags an oracle-clean cell | PR-3 is not built until the check is fixed |
| K2 | PR-M1: one owner-adjudicated false fire | that loss kind goes record-only until narrowed; two in 30 days keep the whole line record-only |
| K3 | stage p95 above 250 ms, or any new I53 miss | the line moves to pre-push only |
| K4 | an agent retires a pin by editing its source with no decision record (bench or production) | file an incident; the pre-push exemption then requires a decision cite |
| K5 | PR-M2 B: the line-on arm is not at most half the line-off violations, or the interval includes 0, or a secondary bar exceeds 1.10 | the line stays record-only; the pre-push gate (if accepted) is the enforcement |
| K6 | PR-M1 shows fewer than 20 pin checks per week on registered roots | nothing in section 6 is built on volume grounds |
| K7 | a waiver rate of 30 percent or more (only once waivers exist) | a human proposal to rescope the rule, never automatic suppression |
| K8 | if `test_ran` is ever revived: one false kept on a renamed or deleted pin | `test_ran` is off |
| K9 | PR-M2 A shows a violation a deny should have caught | ADR-038's revisit clause fires; name the check |

Two facts are opened only with the item that needs them: **PR-V2** (what a Stop block does in Claude Code)
and **PR-V3** (whether hooks fired inside a sub-agent carry `agent_id`). PR-V1 gates non-Claude harnesses.

## 6. Deferred, with triggers

| Item | Why not now | Trigger |
|---|---|---|
| `test_ran` at the turn deadline (PR-D1) | 85 to 94 percent base rate on code scopes; the `[no tests to run]` false-kept hole; doc-scope tests cannot be affected; pre-push runs everything | a served-population reading under 70 percent on code scopes **and** the pin line does not already catch what it would. If revived: `.go` edits only, and it needs a `--- PASS: Name` line or a JSON pass event for the name, never a package `ok` (K8) |
| Turn-end feedback through a one-shot Stop block (PR-D2) | no in-turn channel is verified (PR-V2); Stop is at 7.1 s of 8 s | PR-M1 or pre-push shows pin losses from sessions that ended without a commit, and PR-V2 establishes what a Stop block does |
| `enforce: "warn"` and `FATHOM_ENFORCE=warn` (bench arm W) (PR-D3) | no rule anywhere would use it today (this departs from the value critic, who recommended it: a mode no rule uses is a pre-built future) | an adopter authors a checkable guard and wants a shadow period, or the owner wants arm W (a fact line against a deny) answered after PR-M0; it reuses the deny path verbatim, so its record is exactly the would-have-denied count |
| `fathom claim waive` with a review receipt (PR-D4) | nothing to waive until the line is live | PR-M1 records 3 or more owner-judged legitimate fires in 30 days; ship the verb with the receipt (`review_diff`, pre-push), never alone |
| The open set, ledger schema 2, an `internal/promise` registry (PR-D5) | no accepted check needs state across hooks | a revived check needs it; add `fathom-promises` to `gcTmpCacheDirs` in the same change |
| GL-11 reclassification and the D-8 trigger (PR-D6) | the rule-outcome ledger has never held a row | SR-61 deployed plus 30 days of rows; it must exclude `enforce: true` rules from effectiveness; D-8 is computed per rule |
| `propose_check` (Jev, F53) (PR-D7) | about 21 labeled rules, under the bar of 50 | J0 passes, the labeled set reaches 50, and prose rules are at least half of served firings |
| Orchestrator constraints on sub-agents (F54) (PR-D8) | no consumer | the handoff maps exist (XM-M1 reads BUILD at 15 percent or more re-derivation and XM-M2 reads PASS), and PR-V3 shows sub-agent hooks carry `agent_id` |
| Unserved scanning of guard and layer rules (PR-D9) | it checks rules the agent never saw | PR-M2 shows retrieval misses dominate; a receipt-only counter |
| Per-agent keeping tables, a view keyed by agent identity (I58) (PR-D10) | no volume | K6 clears |

## 7. Refused

| Item | Reason |
|---|---|
| A model (Jev or any other) setting kept or broken (PR-X1) | R7, I62, ADR-056 decision 13: a judgment never sets a status |
| A bypass in the line, for example `or: fathom claim waive <id>` (PR-X2) | in i28 the agent took the cheapest exit; a sanctioned bypass beside the fact becomes the new cheapest exit |
| Turn-end lines delivered on the next UserPromptSubmit (PR-X3) | they arrive after the agent reported done, next to a new task: the served-but-ignored channel again |
| `guard_eval` and `layer_eval` on the whole file after each edit (PR-X4) | fragment-tuned checks misfire on existing content (45 of 366 files) and read kept vacuously when `!Enforce` |
| A per-edit, single-file pin check (PR-X5) | a moved test reads as deleted |
| `denied_retry`, `doc_touched` and `verify_row` promise rows (PR-X6) | kept about 100 percent of the time by construction (a tautology that would feed GL-11); it checks "the doc was edited", not "the doc is still true"; and the verify ledger already records claims (19,639 rows) |
| Automatic suppression of a rule's checks at a waiver rate (PR-X7) | model-written waivers would switch checking off, and the governed agent can game it |
| Moving thread evaluators into a shared registry (PR-X8) | a refactor with no consumer |
| Promises parsed from agent prose (PR-X9) | GL-X4, ADR-041 decision 8 |
| A new MCP tool or top-level verb (PR-X10) | none is needed; a line, a beat and a conserve flag cover it |
| A pin inferred rather than named: a test from `test_impact` reach, a blast-radius trailer or the tests near an edited file (PR-X11) | precision over coverage: only a test a governing source already names is a pin; an inferred pin fires on tests no rule depends on, and a false fire teaches the agent to skip the line |

## 8. Jev, weighed

- **Jev never resolves anything here.** Kept and broken are statuses (R7, I62). Judged compliance is refused
  (ADR-056 decision 13), and a hook never opens the port (I14 as amended). With provider none (the default)
  every element of this design runs unchanged: nothing here calls the port.
- **Its only credible role is upstream.** `propose_check` would be a typed choice over the closed check kinds
  for a prose rule, accepted by a human.
- **It chooses only the kind.** The arguments (globs, test names) still have to be generated, and generation
  is Jev's weak side (ADR-056, the notes corrected as A5).
- **The labeled set is too small.** About 21 rules already have a check, under the design's own bar of 50, so
  the first reading may only decide the site is not worth building.
- **Deferred** behind F53's trigger. The measurement that proves it: J0 (`docs/plans/judged-predicates.md`
  JP-M1) passes and the labeled set reaches 50. What kills it: J0 killing the site, or the labeled set staying
  under 50 for a year (F53's drop condition).

## 9. Plan rows

`docs/plans/served-not-kept.md` holds the rows (PR-V1, PR-M0, PR-1 to PR-11, PR-M1, PR-M2 and the deferred and
refused rows PR-D1 to PR-D10 and PR-X1 to PR-X11), grammar v2, all open, nothing claimed.

## 10. Records this design creates or changes

- **ADR-057**, "Served is not kept: a lost pin is a fact about the change" (2026-09-29): the three placement
  questions, the corrected ladder, the commit line, the deferrals and refusals, and PR-8 (accepted 2026-09-29 by the owner-architect, confirmed by the owner the same day).
- **JF-1** lands on its own as the O120 fix and drops its clause that keeps `violated` "for a rule with a
  checkable condition": the tracker evaluates no check, checkable rules deny and record `blocked` (PR-10), and
  pin losses have their own beat, so the name would have no writer and would invite O120 back. O120 carries the
  addendum.
- **Futures** F53 (`propose_check`) and F54 (orchestrator constraints on a sub-agent).
- **Observations** (through `fathom observe`): O123 (SR-45's "the obligations answer marks it checkable" half
  has only test callers) and, in the same pass, the I53 rung re-read in `docs/intent/invariants.md`.
- **CHANGELOG.** A line with PR-6, for the new `[hooks] pin_check` key, when the row lands.

## 11. Guidance for agentic-kit

The promise pattern for agent harnesses (deny at the fragment, a line at the commit, a gate at the push; a fact
with a location, never the rule again; no bypass in the message; nothing after the turn; no model sets pass or
fail; the pinned test as the bridge; replay before building) is project-agnostic. It belongs in agentic-kit's
guidance, not here; this design is fathom's application of it.
