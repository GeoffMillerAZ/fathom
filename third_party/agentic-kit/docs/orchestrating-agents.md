# Orchestrating many agents: shared maps, triage, gates under load, durable state, done means wired, consolidation

Practice for a lead agent (or a script) that fans work out to sub-agents and worker
waves, and for the gate those workers' output must pass. It extends the
[parallel-agent protocol](project-intent/operating.md#4-parallel-agents) (one unit, one
worktree, the orchestrator integrates) and the
[push-gate rules](project-intent/operating.md#5-push-gates-lefthook-pre-push-task-push)
with what a consuming project learned running large waves in September 2026. Written
2026-09-29; the ledger entries are [practices §26](practices.md#26-blind-waves-and-fragile-gates)
and, for sections 7 to 9, [§28](practices.md#28-served-is-not-kept),
[§30](practices.md#30-done-without-a-caller) and
[§29](practices.md#29-research-left-in-artifacts). Sections 10 and 11 have no ledger entry yet.

**Evidence tags.** [field] each rule was paid for by an incident in a real orchestrated
run; the incident is stated. [design] a designed practice with a measurement plan and no
result yet; do not cite it as proven. [harness] depends on the host tool; the doc says
what it was verified against.

| # | Practice | Status |
|---|---|---|
| 1 | Shared exploration maps, so sub-agents do not repeat exploration | [design], measure first |
| 2 | Triage before dispatch | [field] |
| 3 | Run the gate while workers load the machine | [field] |
| 4 | Durable snapshots before any stop or restart | [field] |
| 5 | Stop, edit and resume a workflow with cached stages | [field][harness] |
| 6 | Isolation, brief hygiene and user-level state | [field] |
| 7 | Serve, then check what you served: put a deterministic check where it sees the whole rule | [design] |
| 8 | Done means wired: a row is done when a product entry point reaches it | [field] |
| 9 | Research and assessment waves end in records | [field] |
| 10 | Consolidate from time to time: harvest what is not on the main branch, land the valuable, back up, then remove | [field] |
| 11 | Wave practice: a sound gate clone, strict cleanup, capped concurrency, language owners, honest readings | [field] |

The kit ships no package for these. They are practices for orchestrators; the closest
shipped pieces are `kits/agent-harness`'s stop receipt ([practices §22](practices.md#22-handoff-less-stops)),
`foundations/coordination/*` (plan-only, no executor: [decision D7](intent/decisions/README.md)),
and the push-gate scripts in `fathom-pack/scripts/`. Future entries for briefs and
worker pools are [F8 and F16.9](intent/futures.md).

## 1. Shared exploration maps [design]

**The problem.** Each sub-agent starts blind and re-reads, re-searches and re-derives
what its parent or a sibling already knew. But many sub-agent reads are reads-to-edit
and are not waste, so the size of the problem is an empirical question, not a given.

**Measure before building (no model, no product change).** Scan the last two weeks of
transcripts. For every sub-agent, count the share of its reads that hit files the parent
had read or edited before spawning it, split into "followed by an edit" (legitimate) and
"not followed by an edit" (re-derivation). Build nothing unless re-derivation is at
least 15% of sub-agent reads. [plan] Also confirm what your harness passes to a
sub-agent start event: if the event does not carry the sub-agent's prompt, choosing
entries by relevance inside the hook is impossible, and only a caller-named map id will
work.

**What a map is.** A named set of anchored entries that answers one question.

- **Entry.** An anchor (a symbol or a span, plus a content hash) and a short annotation
  of why it matters. Anchors are re-resolved at read time. An entry whose hash changed
  or whose symbol vanished is flagged stale and never silently trusted.
- **Purpose line.** Required: the question the map answers. A named map without one is
  refused. The creator writes it; the system computes scope, freshness counts and last
  use. Write the description as you would a good tool description (what the map is for,
  when to use it, the computed scope, live freshness counts, the creator and when it was
  last used), so an agent can tell whether to mount it. A model drafting the purpose is
  deferred (below).
- **Composition by reference.** A map holds its own entries plus `includes` of other map
  ids. Entries are never copied. Includes form a directed acyclic graph: shared bases
  are allowed, cycles are refused, depth is capped (4) so expansion stays bounded.
  Several maps requested at once become one tiny composite made only of references.
  A sub-agent can hand its own composite to its sub-agents.
- **Resolution.** Union, deduplicated by anchor (symbol or span plus content hash).
  Conflicting annotations: the nearest include wins and both are kept with provenance.
  Freshness is checked once per entry however many maps include it. Budgeting and
  ranking happen once, at expansion, not in every consumer.
- **One id in the handoff.** The brief carries a single map id. Why not a list of ids:
  every sub-agent would re-implement merge, dedup and budgeting, and ids could not be
  handed down a level.
- **Lifetimes follow references.** A task-scoped (ephemeral) map is pinned while a live
  map includes it, and its idle clock restarts when the last reference drops. A saved
  map is permanent only when it is named and used; idle unnamed maps decay.

**Live maps versus semantic layers.** They are different axes and complementary. A
semantic layer says what is true or binding about code (governed claims: rules,
bindings, labels). A live map says which part of the code matters for a purpose: a
saved question that is re-run as the code changes, written in the layers' vocabulary
("handlers where 'validates input' is no", "code governed by these rules and touched by
this program"). Layers make maps sharper as they grow; maps scope layers, so an
expensive judged column ([fast-judgment catalog, B](fast-judgment-catalog.md)) is
computed only inside the maps that need it; a map many agents reuse can graduate to a
named concern in a layer; both invalidate on the same content-hash drift.

**Durable live maps worth having.** The auth surface (handlers, middleware, policies)
for security review; the payment path from entry point to data store; release blockers
(claimed-but-unproven plan rows for the next release intersected with touched code); the
public API with no live certificate; a migration's remaining sites; the hot zone (high
debt interest intersected with churn); everything that reads a given environment
variable or flag; the footprint of an active program that every worker on it mounts;
dead-code candidates; the product's core loop for onboarding.

**Delivery: maps arrive, agents do not have to look.** Anything an agent must choose to
call is adopted rarely (one consumer's logs: a localizer had 40 calls and a test-impact
tool 103 out of about 98,000 tool calls; an anchored-memory feature that agents had to
choose to write recorded no memories and was deleted). Forced moments, by contrast, fired
hundreds of times in about four days: 584 sub-agent starts, 452 handoff notes and 550
pre-compaction pointers. So deliver by configuration:

1. **Handoff.** The brief's map id is expanded by the sub-agent start hook, within
   budget.
2. **Relevance by location.** When a prompt or an edit touches code inside a map's
   footprint, inject one line naming the map, its purpose, size and freshness and how to
   expand it. Deduplicate per session and budget it, as governing rules are at an edit.
3. **Session brief.** At most three maps, by recent use and overlap with the thread's
   recent files; never a full list.
4. **Continuity.** Thread notes may reference maps, the pre-compaction pointer carries
   the active map ids, and resuming a thread re-mounts them.

No new default tool: the map surface lives on an extended tool or the command line, and
the reminders are injected context.

**Proof bar** [plan]. An A/B on identical briefs: at least 25% fewer sub-agent tokens,
no loss of accuracy, and under 5% stale entries. Measure pointer precision (expanded
over shown) and reuse rate (mounted after handoff), and cut any pointer expanded under
10% of the time. **Kill:** re-derivation under 15% means no handoff maps (durable live
maps for people are unaffected); an A/B that misses its bar means removing the handoff
channel. Live maps for people are unaffected by that reading in both directions: they
need their own named consumer and their own metric, so the handoff experiment neither
promotes nor kills them.

**Deferred.** Set algebra over maps (intersection, difference). Promote when a real
brief needs "the base map minus what is already done"; drop if plain union covers every
observed handoff for 60 days. Trimming at handoff (one yes or no per entry when a map is
over its budget): promote when the sub-agent start event carries the prompt and a map
exceeds its budget; trimming prunes, so it faces the strict bar in
[fast-judgment.md](fast-judgment.md) and lists every entry it removes. A model drafting
purpose lines: promote when refusals show creators omit them and a calibrated
text-shaped judgment site exists. How long an unreferenced map lives is left open on
purpose: no data exists, so set the idle clock from the measured reuse gap when live
maps are built.

**Refused.** A map store that spawns, schedules or retries agents. The store serves
maps; the orchestrator dispatches.

**Live maps as documentation for agents [design].** Structure (what exists, what
connects, where to look) is the half of documentation that rots, and a live map is that
half computed. Reasons (why it is built this way, how to proceed) cannot be computed and
stay written: decision records, intent files, rules. Design each to point at the other.
Nothing below is built; it waits for live maps, which need their own named consumer.

1. **Maps carry their reasons.** Each live map joins, deterministically and by scope,
   the decisions and rules governing its footprint, so a handoff delivers structure
   plus rationale under one id. [design]
2. **Explainable membership.** An entry's why-line is the query clause that selected
   it: computed, never authored, so it cannot rot or be invented. [design]
3. **Structural claims become checkable.** A sentence in a doc or an agent instruction
   file about structure ("only the handlers call the store") compiles to a map
   assertion, and a changed answer flags the sentence. This extends "an instruction is
   true" from rules to structure. [design]
4. **Instruction files point to maps.** Propose replacing structural prose in agent
   instruction files with named map references, each accepted by a human. Keep it only
   if the files shrink without losing facts that were being served. [experiment]
5. **Membership diffs as signals.** A structural changelog per named map, and a change
   that alters a named map's membership shows on its review receipt. Needs live maps
   first. [experiment]
6. **Human docs rendered from maps.** A deterministic view first, model narration
   later. [deferred until live maps exist]
7. **Undocumented-territory coverage.** Code that no map and no doc covers. [deferred
   until live maps exist]

## 2. Triage before dispatch [field]

**Incident.** A wave dispatched 15 work items. Nine were blocked on one unbuilt
foundation row and five more scored below the first level because of measurement-harness
defects (name normalization, an unmeasurable range, a file-locator format). About 3.8
million tokens bought two proven levels. Blocked workers on a cheaper model tier burn
tokens for nothing.

**Rule.** Before a wave, run a triage stage: a plain script, or one cheap agent.

1. **Dependency check.** Every item's declared dependencies are done or explicitly
   deferred. Anything else is held, with the reason logged.
2. **Harness sanity check.** Push exactly one item through the real measurement
   harness end to end. If it cannot score at all, no item in the wave can.
3. **Fix foundation and harness gaps first**, with one strong worker, before any fan-out.
4. **Dispatch only the eligible items** and log what was held and why (no silent caps).

Track multi-step work with a task list so held items stay visible, and default
mechanical fan-out to the cheaper tier, reserving the strongest for the foundation fix.

**Depend on the level you need, not on the row being closed.** An item that builds on
another (a dialect on its parent) is eligible once the parent has a committed reading at
the level the child needs, not only once the parent's row is fully done; a parent row can
stay open for a higher target for months. Foundation rows still need to be done or
explicitly deferred. Compute eligibility with a script, not from memory.

**Cap the fan-out by what you can merge and gate.** One consumer's recipe for a long
lane: at most two cheaper-tier workers at once, results merged serially into one
integration line, the full gate once per wave, and a script choosing the next eligible
items after triage. Its stated reason was token efficiency; the cap is that consumer's
setting, not a measured optimum.

## 3. Run the gate while workers load the machine [field]

**Incident.** The pre-push gate ran alongside parallel waves (load average above 10 on 10
cores). Wall-clock timing tests failed: a typed-edge performance gate, a hook budget, a
guest budget, language throughput. Three consecutive pushes each died on one, each cost
a gate run of about 40 minutes, and retrying merely hoped for a quiet machine.

**Rule 1: measure work, not elapsed time.** Convert each timing test to CPU time (own
plus children's user and system time from resource usage) or to a progress count; keep
wall time as a log line only. Where the deadline itself is the behavior under test (an
RPC deadline), keep that part wall-clock and bound the rest on CPU time. When a
reading is published, record which basis it used and disclose the other figure beside
it; never mix bases in one reading, and re-measure readings taken on the old basis
rather than converting them ([measuring-tools.md](measuring-tools.md)).

**Rule 2: prove the converted test still fails.** Inject the regression it exists to
catch (a spin loop, an extra pass over the data), confirm the test fails, then remove
it. A timing test that cannot fail is worse than a flaky one, because it reads as
protection.

**Rule 3: give the gate a private build cache.** Workers that "cleaned up" a shared build
cache while the gate was compiling failed two gates with "could not import" errors, and a
shared cache had a prune race. The gate gets its own cache and temp roots that nothing
else touches, and a finishing worker never empties a cache the gate might be reading.
The gate also gets a throwaway home for any user-level ledger its tools append to
(section 6).

**Related, from the same runs.** A hook exports repository variables (`GIT_DIR` and its
siblings) to everything it runs, so any test or script that builds a scratch repository
must unset them first, and simulating a hook environment must only ever target a
throwaway repository. Leave disk headroom: one full race-detector gate left about 9 GB
in the build cache, and a full disk killed a pilot and two gates. The transport, hook
and verify-the-tree rules are already in
[operating.md §5](project-intent/operating.md#5-push-gates-lefthook-pre-push-task-push).

## 4. Durable snapshots before any stop or restart [field]

**Incident.** Scratch space in the temporary directory does not survive a restart, and
a rate-limit outage killed every worker at once with its partial work uncommitted. A
planned restart for a model upgrade found three integration lanes and four
half-finished worker branches living only in temporary scratch, so all of it had to be
bundled first.

**Before a planned stop (and at each wave boundary as a habit):**

1. **Snapshot every working copy.** Commit each unfinished worker branch as a clearly
   labeled WIP snapshot whose last commit is easy to undo on resume
   (`git reset --soft HEAD~1`).
2. **Bundle everything** into a durable directory that lives inside the repository's
   ignored workspace, never the temp directory: `git bundle create <lane>.bundle --all`
   (or a named branch) for each integration clone and each WIP branch, then
   `git bundle verify` each. Full-history bundles were about 34 MB each in the run this
   comes from; a bundle over a base range (`<base>..<branch>`) is small but restores only
   onto a repository that already has the base.
3. **Copy the scripts** the lanes use (environment setup, push and deploy scripts).
4. **Write a RESUME file** in that directory: the state at shutdown with hashes (what is
   on the main branch and deployed, what exists only in bundles, per-lane head), each
   lane's work in progress and what remains unstarted, the resume steps in order
   (rebuild scratch from the local repository and the bundles, restore scripts, merge
   lanes, push through the gate, relaunch what is left), watch-items, and where
   workflow scripts persist.
5. **Point durable memory at the RESUME file** so the next session reads it first, and
   lead with it in the notes.
6. **Prove it restores.** Clone from a bundle into a temporary directory and compare
   heads with the RESUME file before trusting it.

**After an unplanned outage (rate limit, crash).** Never delete a worker's clone after a
429. Inspect `git status` and `git log` per clone, then relaunch a "finish from the
partial work" worker with a compact brief. Tell workers to `cd` into their clone
explicitly: one left a stray file in the real checkout through the default working
directory.

**Do not delete what still carries evidence.** Before deleting any scratch clone, search
the other clones' `.git/objects/info/alternates` for it: a shared clone chains alternates,
and removing the source silently broke every export downstream. Never delete a pilot's
per-run worktrees until its cells are scored and read; in one run the only copy of the
agent's diff lived there.

## 5. Stop, edit and resume a workflow [field][harness]

Workflows are scripts that fan agents out. A long one can be stopped, its script
edited, and relaunched, without paying again for finished work, provided you respect how
resume works.

**How resume works** (verified against the workflow tool reference loaded in a Claude
Code session on 2026-09-29, version not recorded; another harness may differ, so check
yours). Relaunch with the same script path and the previous run id. The longest
unchanged prefix of agent calls returns its cached results instantly; the first edited or
new call, and everything after it, runs live. The same script with the same arguments is a
full cache hit. A workflow whose stages failed on an outage replays its finished stages
from cache.

**Practices that follow.**

- Put stable, expensive stages first and tentative or new logic last. Append new
  stages rather than editing early prompts, labels or arguments: any edit invalidates
  everything after it.
- Keep the script deterministic. No clock or randomness inside it (they would break the
  cache): pass timestamps in as arguments, stamp results afterwards, and vary an agent's
  prompt or label by index when you need variation.
- Give each parallel agent a unique label built from its lane and index, so results map
  cleanly onto calls.
- Before diagnosing an empty or odd result, read the run journal, which records each
  agent's actual return value. Do not assume a cached result was non-empty.
- Resume a benchmark or pilot with the same run id only after the fixes that were in
  flight to the driver are merged and the harness binary is rebuilt; scored cells are
  borrowed, lost cells re-run.
- Watch long runs with the host's monitor facility or a background loop that terminates
  on a condition. A background `tail -F | grep` never exits, so nothing notifies you.

## 6. Isolation and brief hygiene [field]

- **Unique scratch per agent.** Build each parallel agent's scratch directory from its
  label and index, and tell it to delete only its own. In one run a verifier removed a
  shared scratch directory while siblings were mid-reproduction, and the harness flagged
  it as interfering with other workloads.
- **Private caches for anything with a prune step** (section 3).
- **A temporary home for tools that append to user-level state.** In every scratch
  clone and gate run, point the tool's home at a temporary directory, and treat any
  row whose root is not a registered project as noise (below).
- **Short briefs, compressed reports.** Briefs of at most about 30 lines; ask workers
  for reports of at most about 20 lines, data only. Orchestrator context is the scarce
  resource.
- **One path owner at a time** and a worktree per agent, as in
  [operating.md §4](project-intent/operating.md#4-parallel-agents).

**Tools that write user-level logs.** *Incident.* A verifier and an intent audit each
appended a row to a per-user ledger on every run. Scratch clones and gate runs produced
thousands of rows keyed by throwaway paths: 13,514 of 19,639 rows in one ledger and 91 of
180 in another described clones that no longer existed, and the proof-history and cost
figures read from them were distorted. *For the run:* set the tool's home to a temporary
directory in scratch clones and gates. *For a tool author:* key every row by the
canonical, symlink-resolved project root; offer one variable that relocates all
user-level state, with per-file overrides winning; make every reader keep only rows whose
root is a registered project and print the ignored count beside each figure; never let a
reader rewrite the log; and make archiving unregistered rows an explicit, human-run move
(compress beside the ledger, never delete) that no daemon or health check applies
silently. Until the fix is trusted, publish each figure with and without the pollution.

## 7. Serve, then check what you served [design]

**The problem.** Serving a rule to an agent before it acts is a hope, not a control.
Agents read guidance, acknowledge it and still contradict it, so measure "kept"
separately from "served". Close the loop only where a deterministic check exists, and put
that check at the earliest moment that sees everything the rule is about:

- **The proposed edit**, when a fragment is enough. Deny it there, because a reminder
  after the fact is weaker.
- **The commit**, when the rule spans files. Examples are a test renamed, moved or
  weakened, or a contract split across modules.
- **The push**, as the durable gate for everything that did not pass through a hooked
  harness.

**What the failure says.** Report a failure as a fact with a location, not as the rule
text served again, and say nothing on success. Never put a bypass command in the failure
message, because the sanctioned exit becomes the cheapest path. Never deliver feedback
after the agent has finished its turn, because the next prompt is the ignored channel
again. Never let a model judge compliance or set a pass or fail state; a model may at
most propose a check for a human to accept.

**The pinned test is the bridge from prose to a check.** A rule that names its test
becomes checkable by protecting the test, so every governing source that names a test
should also make losing that test visible. This matters because the cheapest way around
such a rule is to rewrite, rename or skip the test that guards it: a run of the old test
name then matches nothing and reports success.

**Measure before building.** Replay the checks over past sessions to count
would-have-fired cases and false fires. Then compare against the configuration people
actually run, not a weakened one. One consuming project's replay of a commit-time
lost-test check flagged both known violations and raised no false loss over 50 commits of
its own history; whether feedback at the commit changes agent behavior is not yet
measured, and it has a bench planned before any default changes. Treat that as one
project's reading, not a result. The full guide, with the refused and deferred lists and
the measurement plan, is [serving-rules.md](serving-rules.md).

## 8. Done means wired [field]

**Incident.** Worker slices anchored plan rows to library functions that only tests
reached. The verification tool accepted them, because a test reached the anchor, and 17
roadmap rows plus several partial rows were claimed done with no product caller (a
learning step, a link resolver, a price table, a session feed, a selection strategy, an
exporter). An adversarial audit before a push caught it. That is one incident: how often
the audit would find it again is unmeasured.

**Rule 1: a row is done when a product entry point reaches it.** A program's main, a tool
or command handler, a route, a hook. A test reaching a symbol proves the symbol works,
not that the product does the thing.

**Rule 2: split library rows from wiring rows when you write the plan.** The library row
can be done alone and is labeled as a library, with a stated reason (a published API for
consumers, say). The wiring row names the entry point and stays open until that entry
point reaches the code. If your verifier can, give "reached only from tests" its own
state (one consumer's verifier reports `library-only` unless the row opts out with a
reason); if it cannot, add a reachability step to the gate.

**Rule 3: audit adversarially before every push of a worker wave.** By someone who did
not write the rows: a fresh agent, or the orchestrator with a script. Two mechanical
steps. First, list the code no product entry point reaches (in Go, `deadcode ./cmd/...`;
other languages have equivalents) and confirm no anchor of a newly done row is on the
list. Second, spot-check rows chosen by the auditor, not the worker: open the anchor,
find a call path from a real entry point, run the product surface and see the effect. If
one row fails, audit every row from that worker.

**Rule 4: make the brief ask for the caller.** State the entry point that should call the
code and require the worker's report to name the caller. Reports are claims; the audit is
the evidence.

The kit's own ledger records the same failure for a shipped component with no wired
consumer ([practices §1](practices.md#1-lossy-progressive-summarization), status
paragraph) and separates implementation, enforcement and adoption in
[workflow assurance](intent/assurance.md).

## 9. Research and assessment waves end in records [field]

**Incident.** A three-day research and decision program tracked 526 items. A closing
sweep found 60 in no repository record and 35 recorded without their reason, verdict or
trigger; another 170 lived only on unmerged branches, and everything newer than the last
durable snapshot existed only under a temporary directory.

**Rule.** The last stage of a research or assessment workflow is a records stage, and the
workflow is not done until it reports zero missing items. The stage enumerates every
idea, assessment and decision the program produced from the transcripts and artifacts,
never from the previous index; maps each to its record by searching the tree; and
classifies it: recorded, recorded only on an unmerged branch, partial (the reason,
verdict, trigger or finding is absent), or missing. Fix the gaps in the same change.
What a record contains, where it lives and how claims are graded is in
[changes.md §10](project-intent/changes.md#10-research-and-assessment-records). Sweep at
wave boundaries too, as a habit; a sweep that waits for the end finds what a restart
already took.

## 10. Consolidate from time to time: harvest, land, then remove [field]

**Incident.** Waves leave branches, worktrees, stashes and scratch clones behind, and
nothing removes them, because each one might hold work. One project's first
consolidation found 46 linked worktrees (5.0 GB beside a 61 MB repository), 53 local
branches, 2 stashes and 2 archive refs. Forty-two worktrees and every branch were already
on the main branch, by ancestry or by patch equivalence; the stashes were 98% and 89%
landed, the rest renumbered or rewritten; the archive refs were fixture noise and a
rewrite equivalent to main. Four detached worktrees held commits not on main. A second
repository held 7 worktrees, 32 branches and 1 stash, all landed or identical to a
remote branch. The fear of losing work had kept 5 GB; a check made removing it safe, and
it also surfaced the four that were not landed.

**Cadence.** After every landed wave the orchestrator removes what the wave made: its
worker clones, worktrees and merged branches. At least weekly, and before any planned
stop (section 4), a harvest sweeps everything, including what earlier waves and other
sessions left.

**The harvest.**

1. **Enumerate every place work hides:** local branches, linked worktrees (detached
   ones too), stashes, archive and backup refs, scratch clones outside the repository,
   remote-tracking refs.
2. **Classify by evidence, never by name or age.** On the main branch by ancestry
   (`git merge-base --is-ancestor`); by patch equivalence (`git cherry <main> <ref>`
   prints no `+` line); a branch whose tip equals a remote branch loses nothing
   locally; a stash by the share of its added lines already on main, with the rest
   explained as superseded (renumbered, rewritten, moved), never waved through on the
   percentage. A dirty worktree is removable only when every dirty file is
   byte-identical to the formatter's output of its committed version; anything else is
   work.
3. **Land what is valuable.** Work not on main and still wanted goes to main through the
   normal gate, in its own commit. Work that is not landed and not provably superseded
   is kept, clean, and named to its owner; the harvest does not decide it.
4. **Back up before every removal** (section 4): `git bundle create --all` after giving
   each detached worktree HEAD a ref, because a bundle carries no unreferenced HEAD; a
   patch per dirty file and per stash; `git bundle verify` on each; all under the
   durable directory, never the temporary one. A bundle holds no ignored files, so a
   worktree's bench cells, logs and local databases go into a checksummed archive too;
   skip only what regenerates (build output, downloaded datasets, third-party clones).
5. **Remove, then check.** `git worktree remove` on the classified set, `git branch -d`
   for ancestors (it refuses an unmerged one), `-D` only after `git cherry` shows none
   left, `git worktree prune`, and `git gc` with its default grace, never
   `--prune=now`: the grace is the last undo. Finish with `git fsck --connectivity-only`.
6. **Report counts:** found, removed, kept and why, sizes before and after.

**Never touch:** an owner's working tree and its uncommitted edits; an in-flight lane
(the brief lists the paths to skip); anything on a remote. A harvest deletes no remote
branch and never force-pushes. Run it from a separate clone or worktree, so the owner's
checkout is not the place it works.

**Status.** Guidance only: the kit ships no command that classifies unlanded work, so
the check is written each time and the classification is a discipline, not a gate.

## 11. Wave practice: gate clone, strict cleanup, caps, language owners, honest readings [field]

Learned over three days of large waves (2026-10-01 to 03). Each rule is short; the
incident is one line.

- **Gate from a sound clone.** When the repository's volume is flaky (bursts of
  "operation not permitted" under load), run the pre-push gate from a clone on the
  internal disk, outside the temporary directory and the home directory, so the
  gate's reads cannot be wedged by the volume. Incident: gates died mid-run on read
  errors that no code change explained.
- **Strict cleanup.** Remove a worker clone only when every git command in the check
  succeeds and proves the work landed (ancestry or patch equivalence, section 10).
  Any error, from any command, means keep it. Incident: a check that ignored a failed
  command read "nothing unlanded" from empty output.
- **Cap concurrency.** Two or three concurrent agent waves, and one Linux test VM of at
  most 8 GB. More waves only queue behind the same disk, cores and gate (section 3);
  more VMs starved the host.
- **Language-owner waves.** One worker owns one language end to end (extractor,
  fixtures, oracle, re-measurement) and re-measures locally; the orchestrator pays
  integration and the gate once for the wave, not per item. Incident: per-item merges
  repeated a 40-minute gate for changes that did not interact.
- **Cap oracle-blind exclusions.** A rule that excludes what the oracle cannot see
  needs a hard cap (for example 10% of the reading's population) and full disclosure
  of the excluded set. Past the cap the reading is not published as that level.
- **A reading never claims more than its oracle proves.** The level claimed is capped
  by the oracle's tier ([practices §31](practices.md#31-unaudited-oracles)); record
  oracle, tier and basis with it, and name the stronger oracle that would be needed
  for the next level.

## Checklist

**Before a wave:** dependencies satisfied, one item proven through the harness, held items
logged, unique scratch and explicit working directories in every brief, the gate's cache
private, disk headroom checked, each brief naming the entry point that should call the
code and, by id, the records that govern it ([operating.md §4](project-intent/operating.md#4-parallel-agents)),
and the user-level home of the tools the workers run private too.

**During:** resume rather than restart; read the journal before diagnosing; keep the
gate's tests CPU-bound and proven able to fail.

**Before a push:** someone other than the workers audited the done rows (the
unreachable-code list, and spot-checks from a real entry point).

**Before serving a rule to agents:** name its deterministic check and the earliest moment
that check sees the whole rule; if there is none, serve it as advice and count it as
unmeasured. Word any failure as a fact with a location and no bypass.

**Before any stop:** every branch snapshotted, every working copy bundled and verified,
scripts copied, RESUME file written, memory pointed at it.

**After each wave, weekly, and before a planned stop:** the harvest ran (section 10):
everything classified by patch equivalence, the valuable landed, a verified backup
before every removal, and the unlanded and undecided kept and named to their owner;
weekly and at wave planning, `intent-audit`'s bounded review judged the ten oldest
condition lines.

**Wave hygiene (section 11):** gate run from a sound clone when the volume is flaky, concurrency within the caps, cleanup only on all-green git proof, every reading's level within its oracle's tier.

**Before closing a research wave:** the records sweep ran and reports zero missing.
