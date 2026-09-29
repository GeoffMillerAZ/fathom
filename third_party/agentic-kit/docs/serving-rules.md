# Served is not kept: making agents keep the rules they were shown

Practice for a project that serves rules to coding agents (instruction files, governing
decisions, guard notes, "done needs this test" lines) and wants to know whether the rules
were kept, not only whether they were shown. It covers where to put a check, what a
failure message may say, what to count, what to refuse, and how to measure before
building. Written 2026-09-29 from one consuming project's owner decision, made after two
critic reviews (one on value, one on cost); the ledger entry is
[practices §28](practices.md#28-served-is-not-kept).

**Evidence tags.** [field] paid for by an incident or a reading in a real run; the
incident is stated. [design] a designed practice with a measurement plan and no result
yet; do not cite it as proven. [harness] depends on the host tool; the doc says what to
verify. [reading] a measurement by one consuming project on its own data, small N, not
transferable to yours.

## Verdict first

- **What it is.** A way to close the gap between a rule an agent was shown and a rule the
  agent kept, only where a deterministic check exists, at the earliest moment that check
  can see everything the rule is about.
- **Where it pays.** Rules whose only guard is a test the agent can rewrite, rename or
  skip; rules that span files (a test moved, a contract split); anything that must hold
  for sessions that never pass through a hooked harness.
- **Where it does not.** Rules with no deterministic check (that is advice: serve it and
  count it as unmeasured, never as ignored); rules a check on the proposed edit can
  already deny; style and formatting, which a formatter owns.
- **The one rule.** Report a failure as a fact with a location, never as the rule served
  again, and never let a model or a sanctioned bypass decide the outcome.
- **What is not claimed.** That any of this changes agent behavior. The consumer accepted
  one small piece as a design (a commit-time line about lost pinned tests), built none of
  the rest, and had not run its paid A/B when this was written. The kit ships no package
  for it. [design]

| # | Practice | Status |
|---|---|---|
| 1 | Measure "kept" separately from "served" | [field] |
| 2 | Put the check at the earliest moment that sees everything | [design] |
| 3 | Fact with a location, no bypass, nothing after the turn | [field][design] |
| 4 | The pinned test is the bridge from prose to a check | [field][design] |
| 5 | Count what could not be checked beside what could | [design] |
| 6 | No model sets a pass or fail | [design], refused |
| 7 | Replay before building; then compare against the configuration people run | [field][design] |

## 1. The gap [field]

Serving a rule before the agent acts is a hope, not a control. Agents read guidance,
acknowledge it and still contradict it. One consumer's pilot compared an arm that served
its governing rules against a control: the violations tied (4 against 4 in the latest
pilot) at about 1.56 times the tokens per clean success. [reading] In one trial the rule
was served as a raw section and the agent still broke it by rewriting the test that
pinned the rule; in another it renamed that test, after which a run of the old test name
matched nothing and reported success.

Three gaps were already closed in that project, which is why the fourth stood out. A
claimed plan row is verified at the claim moment. An instruction in an agent config file
is audited until it is true. A checkable guard denies the edit before it lands. The
missing case was a rule whose only check is a named test: the cheapest way out is to
change the test.

## 2. Where to put a check [design]

Ask three questions, in order.

1. **Is there a deterministic check?** If not, the rule is advice. Serve it, do not
   promise it, and count it as unmeasured.
2. **What is the earliest moment the check sees everything it needs?**

   | Moment | Use it when | What happens |
   |---|---|---|
   | The proposed edit | a fragment is enough | deny it there; a reminder after the fact is strictly weaker |
   | The commit | the rule spans files (a test renamed, moved or weakened) | one fact line, in the agent's own "this is done" moment |
   | The push | everything that did not pass through a hooked harness | the durable gate |
   | The end of the turn | the evidence arrives later (a test run) | only if a replay shows a gap (section 6) |

3. **Can the agent still act at that moment?** If yes, it gets one line. If not (the next
   prompt, after the push), the result is a record or a gate, never feedback.

**The corrected ladder.** Fragment-checkable rules go from advise straight to deny.
Whole-change rules go from advise to the commit line to the push gate. There is no
permanent middle rung of "advise, then check after": a checkable rule denies. Before you
build that rung, list the rules that are too imprecise to deny yet precise enough to
warn. In the consumer that list was empty: its five checkable guards all enforced, and
the one advisory checkable rule already ran as a test in the gate.

**The trigger is the source that already governs.** A guard's "done needs" test, an
invariant's test column, a marker on a certifying test, a manifest entry. The deadline
is the commit. No new trigger language is invented, and promises are never parsed from
the agent's own prose ("I will run the tests").

## 3. What the line may say [field][design]

- **A failed fact with a location.** Never the rule text again, never a description of
  the detector (a described detector is a gaming guide), never a waiver hint. Example
  shape, with illustrative names: `pin deleted; TestReadOnlyStore (named by invariant
  I28, docs/invariants.md:23); gone in 3f9a1c2, renamed with a new body
  (store_test.go:41); a run of this name now matches no test`. Keep it under about 240
  bytes, one line per lost item, at most three per commit ("+N more at push").
- **Silent when nothing is lost, and silent when the check could not run.** A check that
  overruns its time budget skips, and its record says `skipped`, never `kept`. Push
  stays the backstop.
- **No bypass in the line.** In the pilot trial the agent took the cheapest exit and
  rewrote the test. A sanctioned bypass printed beside the fact becomes the new
  cheapest exit. If waivers
  exist at all they are a separate, human-reviewed channel with a receipt, shipped only
  once real legitimate fires exist, never as text in the failure message.
- **Nothing after the turn.** Feedback delivered on the next prompt arrives after the
  agent reported done, next to a new task. That is the served-but-ignored channel again.
- **A deliberate retirement stays visible without being called a violation.** When the
  same commit also edits the source that names the pin, the line says so ("its source
  was edited in this commit").

## 4. The pinned test is the bridge [field][design]

Many prose rules already name the test that pins them. A rule that names its test
becomes checkable by protecting the test, with no model: make losing that test visible
at the commit, whether or not the rule was served. A lost pin is a fact about the diff,
like a failing test.

- **Loss kinds.** Deleted, renamed, weakened, skipped, golden file changed, certifying
  marker moved. A plain edit is silent, because no tool can decide whether an edit
  weakened a test.
- **Check the whole change.** A per-edit, single-file check reads a test moved to another
  file as deleted. At the commit, base and head are both in view, so a moved test is
  found and is not a loss.
- **Read the pass line, not the package summary.** A run selecting a name that no longer
  exists prints a package "ok". Any test-ran check must look for the named test's own
  pass event, and one false "kept" on a renamed or deleted pin switches it off.
- **Cheap enough.** One batch read of the base and head blobs, a fraction of a second,
  no state file, no dedupe store, and the commit id taken from git's own summary line so
  no summary line means no check. One consumer measured 13 to 15 ms for a single
  `git show`. [reading]
- **Modes.** `off`, `record` (write one evidence beat per checked commit: session,
  commit, loss kinds, test ids, source cites, milliseconds, no source text) and
  `feedback` (also print the line). Default to `record` until a latency reading passes.
  `off` and `record` leave the hook payload byte-identical.

## 5. Count honestly [design]

- **Unmeasured is not ignored.** A prose rule with no check reads "unmeasured: no
  check", never "ignored".
- **Blindness must never look healthier.** If you record states, keep five: kept,
  broken, waived, expired, unverifiable. Expired and unverifiable never count toward
  kept or broken; they appear as counts beside the rate. This is the same rule as
  [practices §17](practices.md#17-instruments-that-degrade-toward-healthy).
- **Effectiveness is kept over kept-plus-broken, over checkable rules only.** A rule that
  already denies is excluded, because it reads kept by construction.
- **Do not build the state machine first.** The consumer deferred the open set of
  promises, the ledger schema that would hold these states and the reclassification of
  rule effectiveness until a first ledger has rows and 30 days of them exist. A
  measurement that has never held a row cannot justify its own schema.

## 6. Measure before building [field][design]

**Replay first, at no cost.** Run the checks over past sessions and diffs and count
would-have-fired and false-fire cases. Bar: every violating case is flagged and no clean
case is. In the consumer the replay flagged both violating trials of the incident and
found zero false losses across 50 finished commits, and a turn-end "did the pinned test
run" check was deferred without being built: in 85 to 94% of 404 sampled cases on code
scopes the named test (or its package) had already run after the last edit, whether or
not the rule was served, so the check had no headroom. [reading]

**Then 30 days in record mode, at no model cost.** The owner adjudicates every fire; the
expected volume is a handful, so no model labels anything (a model deciding the input of
a kill condition was the first design's flaw). Bars: the stage's p95 within its budget (250 ms in
the consumer), zero new hook deadline misses, and the skip rate named.

**Then a bench, with the spend approved first.**

- **Baseline arm is what people run.** The production configuration, not a weakened one.
  Rerun the enforced arm the earlier decision owed before adding anything.
- **Give the commit moment a chance to exist.** Every task prompt in every arm ends with
  "commit when done".
- **Grade by an oracle independent of the detectors.** The named detectors were defeated
  in the pilot (the test was renamed). Adjudication may void a cell and nothing else.
- **Size it before running.** With 5 trials per task per arm, a drop from 8 of 10 to 2 of
  10 violations gives a two-sided Fisher p of about 0.02; three trials cannot resolve it.
  About 40 cells at roughly $1.9 each was the consumer's estimate.
- **Secondary bars and gaming.** Tokens per clean success and tool calls at most 1.10
  times the line-off arm; read every diff for a pin retired by editing the source that
  names it.

| Kill condition | Consequence |
|---|---|
| the replay misses a violating case or flags a clean one | do not build until the check is fixed |
| one adjudicated false fire in record mode | that loss kind goes record-only until narrowed; two in 30 days keeps the whole line record-only |
| stage p95 over budget, or any new hook deadline miss | the line moves to the push gate only |
| an agent retires a pin by editing its source with no decision record | file an incident; the push exemption then requires a decision cite |
| the line-on arm is not at most half the line-off violations, the interval includes 0, or a secondary bar exceeds 1.10 | the line stays record-only |
| fewer than 20 checks a week on registered roots | nothing in the deferred table (section 8) is built on volume grounds |

## 7. A decision for your owner [design]

Making the push gate block on a **silent** pin loss (the lost test is still named by its
source at head; a pin retired on purpose, whose naming source was edited in the same
range, is printed for review and does not fail). Today many push checks are advisory
only. Blocking changes that doctrine for one plane, so it is the owner's call and a
decision record, not a worker's edit. Revisit when a false block happens (name the loss
kind) or an agent retires a pin by editing its source with no decision in the range.
This is also the backstop for sessions that never commit through a hooked harness.

## 8. Refused and deferred, so it is not re-proposed

**Refused, with the reason.**

| Item | Why |
|---|---|
| A model setting kept or broken, or judging compliance | a judgment is never proof; see [fast-judgment.md](fast-judgment.md) |
| A bypass command in the line | the sanctioned exit becomes the cheapest path |
| Turn-end lines delivered on the next prompt | they arrive after "done", beside a new task |
| A whole-file evaluation of fragment-tuned checks after each edit | they misfire on existing content (45 of 366 files in the consumer) and read kept vacuously when the rule does not enforce |
| A per-edit, single-file pin check | a moved test reads as deleted |
| A "retry after deny" promise | kept about 100% of the time by construction; it feeds the effectiveness figure a tautology |
| A "the doc was touched" check | it checks the edit, not whether the doc is still true |
| Ledger rows that copy a ledger already recording claims | a second source of truth |
| Automatic suppression at a waiver rate | agent-written waivers would switch the checking off |
| Promises parsed from the agent's prose | inferring intent from transcripts is refused |
| A new tool or top-level verb | a line, an evidence beat and a flag on an existing check cover it |

**Deferred, with the trigger that un-defers it.**

| Item | Trigger |
|---|---|
| A test-ran check at the turn deadline | a served-population reading under 70% on code scopes and the pin line does not already catch it |
| Turn-end feedback through a one-shot stop block | pin losses from sessions that ended without a commit, and the host's stop-block behavior verified |
| A warn mode between advise and deny | an adopter authors a checkable guard and wants a shadow period |
| Waivers with a receipt | three or more legitimate fires judged by the owner in 30 days |
| A typed model proposing a check for a prose rule, a human accepting | a calibrated site exists, at least 50 rules already carry a check as labeled examples, and prose rules are at least half of served firings |
| Orchestrator constraints on a sub-agent | the shared-map measurement reads build ([orchestrating-agents §1](orchestrating-agents.md#1-shared-exploration-maps-design)) and sub-agent hooks are shown to carry the agent id |
| Scanning rules the agent was never shown | a bench shows retrieval misses dominate |

## 9. Harness facts to verify first [harness]

Hosts differ, so verify these in yours before building, and stay record-only where you
cannot: does the post-shell hook input carry the command's output and exit code; does
context added at that hook reach the model; do tool events carry a stable call id; do
hooks fired inside a sub-agent carry the sub-agent's id; what does a stop-block actually
do. The consumer treated each as unverified for every host but its first, made feedback
opt-in, and defaulted to record.

## Checklist

**Before adding a promise:** a deterministic check exists; the rule has a named source;
the moment is the earliest that sees everything; the agent can still act then.

**In the message:** a fact and a location; no rule text, no detector description, no
bypass; silent on success and on a skipped check.

**Before building:** the replay ran; the baseline arm is what people run; the oracle is
independent of the detectors; the kill conditions are written.
