# 41 - Stewardship: the lines, leftovers, inheritance and promises around the code

Status: accepted 2026-09-30 (ADR-060; accepted by the owner-architect under the owner's delegation, the sequencing in
section 4.6 (decision 7) included, and the licence boundary in section 4.5 settled as default deny). Owner notes of
2026-09-30: six ideas, with the instruction not to overload fathom with bad features. Plan:
`docs/plans/stewardship.md` (SV-). Futures F58 and F59. Goal G34 in `docs/intent/scope.md`. Invariant I63. Builds on
the system graph (ADR-050), the feature map and the review family (ADR-055), judgment at the port (ADR-056), incident
maps (ADR-058) and the proof-layer roadmap's kitchen-sink gate (ADR-049 decision 1).

## 1. The question

The owner asked six things on 2026-09-30:

1. Track the branches, worktrees, stashes and scratch clones agents leave behind; find the finished, abandoned and
   defunct ones; nudge to land them; squash and re-summarize by default.
2. Call attention when a vendored or pinned dependency changes significantly upstream.
3. Clean up temporary artifacts: planning documents needed only while building, stale documents, stray binaries,
   agent trash, perhaps with nudges before or after compaction or at stop.
4. SRE as code: SLOs and error budgets tied to features and business processes, observability hygiene (alerts on
   metrics no code emits, dashboards on dead metrics, routes with no SLO), readiness levels 1 to 5 per service,
   failure-mode playbooks, one cross-project compliance dashboard; perhaps Review Kit grows into it, or a separate
   Reliability Kit uses fathom as its evidence.
5. Minimal vendoring: vendor only the parts of a dependency that are used. The concrete case is app-kit with
   components derived from a licensed commercial catalog (TailwindPlus), which must never ship publicly as a
   catalog, so a public project may carry only what it uses.
6. Prioritize by ditching, then merging features, before other work; versus development speed first; versus
   excellent support for the most common languages and tools.

## 2. Before designing: what applies

- *Non-goals* (`docs/intent/scope.md`): no domain connectors; no dispatch loop (fathom never watches, schedules,
  retries, spawns or kicks off); no transformation or migration engine; no hosted multi-tenant backend; no incident
  command center; no net tool growth (law 4). ADR-058 adds that outside systems are reached by a rendered link and a
  typed parameter only.
- *Principles*: P15 (fathom writes only what it owns), P17 (state is computed), P19 and P23 (a request pays for its
  answer; a guest on the operator's machine; no polling), P20 (a named consumer before a build), P21 (split on an
  enforced boundary only), P22 (measured, not argued).
- *Invariants*: I9, I12 and I53 bound every hook (fail open, a 3,200-byte primer, a deadline under the descriptor's
  timeout; the Stop hook already runs close to its 8-second deadline); I56 (fathom never downloads on its own
  authority); I57 (a link below the typed tier never gates); I62 (a judgment is never a fact or a gate).
- *Decisions*: ADR-049 decision 1 (every item names the letter it serves and its instrument, or goes to futures);
  ADR-050 decision 4 (dependencies are the `pkg:` kind read from local caches; the usage map is `links`, upgrade
  impact is `contract_drift`); ADR-053 (the surface reckoning); ADR-055 decisions 7 and 8 (review is one family:
  standards, dispositions and the non-code domains stay in Review Kit, joined by a versioned evidence contract);
  ADR-058 (incident maps are committed declarations; no connector).
- *Autonomy*: deleting a branch or worktree and rewriting history are ask-first; model spend is held.
- *Futures*: F50 (durable live maps), F52 (Review Kit concern packs scoped by live-map queries), F56 and F57
  (incident maps and probes). None of their Promote-when conditions has fired.
- *A corrected premise*: note 2 says `dep_drift` exists. It does not: the feature scorecard verdict DITCH (F-18,
  zero lifetime calls) removed it under ADR-053, and ADR-050 names its replacement (upgrade impact over a
  dependency API index, plan rows SG-37 to SG-39). `task vendor-check` (I15) proves a mirror equals its pin; it
  says nothing about upstream movement.
- *Fired triggers*: none. No Revisit-when, Promote-when or Drop-if condition in the records named above has fired.

## 3. The frame: stewardship facts

Around the code at HEAD lie four kinds of things nobody is looking after:

| Kind | What it is | Who already handles part of it | The question only fathom can answer |
|---|---|---|---|
| Lines | work not landed: branches, linked worktrees, stashes, scratch clones | git (`branch --merged`, `cherry`), manual harvests | which open plan rows would this line prove? |
| Leftovers | files nothing owns: strays outside declared homes, binaries in the tree, finished plans, documents nothing live cites | `fathom gc` (fathom's own files), `git clean`, `fathom plans archive` | what sits outside every declared home, and what does nothing live cite? |
| Inheritance | code we did not write: vendored mirrors and pinned modules | Renovate and Dependabot (new versions), `go mod vendor` (package pruning), `task vendor-check` | what of this dependency does our product reach, did upstream change that part, and did a licensed unit cross a boundary? |
| Promises | what a service says it keeps: SLOs, alerts, dashboards, runbooks | Prometheus, Grafana and SLO tools (live values), Review Kit (standards, the reliability domain) | is every promise wired to code that still keeps it? |

Each existing tool misses the same thing: it does not know what the code is for. Fathom holds plan rows, reach from
product entry points (the wired check, ADR-018 as amended), contract keys (ADR-050) and conservation planes
(ADR-044). So the rule for the whole family is:

- **A stewardship fact is a join** of git or file state with a fact fathom already computes. No new engine
  (ADR-051 decision 1).
- **Fathom reports and never acts.** It names the command a human or agent would run and runs none. It never
  deletes, merges, squashes, rebases or checks out a line it did not create (invariant I63, P15). Landing a line
  belongs to the harness or the owner; the dispatch boundary holds.
- **The cost is the operator's explicit ask.** Each fact is a CLI report. The one ambient channel is a single count
  on the SessionStart attention line, and only after a reading earns it. No stewardship work runs in the Stop hook
  or around compaction.
- **Kitchen-sink letters** (ADR-049 decision 1): lines and leftovers serve (c) make proof visible; the used closure
  and upgrade impact serve (b) deliver proof where people work; the boundary plane and promises serve (a) widen what
  fathom can prove. Each accepted row below names its instrument.

## 4. The six decisions

### 4.1 Lines: accepted, as a report under `fathom plans`

On 2026-09-29 a manual harvest looked at about 230 lines across this repository and agentic-kit (the local backup
manifest holds all-refs bundles of 117 and 88 refs, 5 stashes, 59 worktree records and 28 scratch clones, 7 of
them with commits on no remote) and found 2 valuable
unmerged items (a kit gate change and a kit skills move, both waiting on the owner) plus one unfinished worker to
resume; the consolidation then removed 46 worktrees and 53 branches. That cost a session, and the evidence needed
to sort the lines already existed in git and in the plans.

- *First slice.* `fathom plans lines [--json]` classifies each local branch, linked worktree and stash of one
  repository: **landed** (an ancestor of the integration branch, or patch-equivalent to it), **claims rows** (its
  plan edits flip rows that are open on the integration branch), **touches rows** (its diff touches the anchors of
  open rows; lexical tier, labelled so), **at risk** (a dirty worktree, a stash, commits on no remote) and **idle**
  (none of these, no commit for 14 days). It lives under `plans` because its one unique fact is plan progress that
  has not landed. For landed lines it prints the removal command; it runs nothing (SV-1, SV-2).
- *Measurement.* SV-M1 is a $0 replay over the 2026-09-29 backup bundles: both valuable held items and the
  unfinished worker must fall in claims rows, touches rows or at risk; at least 9 in 10 of the lines the
  consolidation removed as merged must read landed; and at most 25 lines may be flagged for a look. A CUT keeps only
  the landed class. The attention-line count (SV-3) waits on SV-M1 reading KEEP and uses ancestry only (no
  patch-id), inside I12 and I53.
- *Squash and re-summarize by default: refused as fathom behaviour.* Rewriting history is ask-first, and a squash is
  a write fathom does not own. The report lists the rows a line claims, which is what a landing summary needs; the
  agent or owner writes it.
- *Deferred.* "Would prove" (a scratch verify at each line's tip) instead of "claims or touches" costs an ingest per
  line; lines across the fleet and in scratch clones under operator-named roots come after the single-repository
  report reads (SV-D2, SV-D3).

### 4.2 Upstream drift of vendored and pinned dependencies: merged into the system graph

Renovate and Dependabot already know when a new version exists and open the change; fathom must not poll or fetch
(P23, I56). What they cannot say is whether the change touches anything the product uses. That answer is ADR-050's
upgrade impact (SG-39), restricted to the used closure of 4.5, with one addition: for a vendored mirror, the "to"
version is the sibling checkout already on disk (agentic-kit and app-kit are both in this workstation's fleet), so
nothing is downloaded.

- *"Significant" is deterministic*: a removed or re-signatured symbol our product reaches, a changed file inside
  the used closure, or a licence-file change (ADR-052 decision 4's re-check). Never a commit count, never a model.
- *First slice: a reading, no build.* SV-M4 records, monthly for a quarter, each mirror's pin against its sibling's
  HEAD: commits, changed files, changed files inside the used closure, and any licence change. The attention line
  itself is deferred (F58, SV-D1) until SG-39 lands and SV-M4 shows at least one reached change in a quarter.

### 4.3 Temporary artifacts: merged, as report-only classes, and no hook nudges

- *First slice.* `fathom gc` gains report-only classes for untracked, non-ignored files outside declared homes
  (`workspace/` and the build-cache roots of ADR-039) and for executable binaries committed or lying in the tree
  (SV-4). `--apply` never removes them: they are not fathom's. `fathom plans status` marks a plan with 0 open and 0
  claimed-unproven rows as archivable; `fathom plans archive` already acts on it (SV-5). SV-M2 is the owner's
  adjudication of one list on this repository and the kit: at least 8 in 10 flagged items must be trash or
  forgotten work.
- *Merged elsewhere.* Stale documents are the doc-drift work of the knowledge-retrieval plan (a doc span naming a
  changed symbol is flagged); `stale_docs` was already merged into `staleness_report` (scorecard F-56). Documents
  nothing live cites are deferred until those doc-to-code links exist (SV-D4).
- *Refused.* Nudges in the Stop hook (its deadline) and before or after compaction (carry-across read STOP on
  2026-09-29: CA-M1, a median of 0.5 files re-read after compaction against a bar of 2). A `Lifetime:` header for
  planning documents is refused too: whether a live plan, ADR or router cites a document already answers it, with
  no authoring cost.

### 4.4 SRE as code: promises as code, framed bigger; the fathom half deferred, the standards in Review Kit

The bigger frame is a chain of promises, each link a fact: a feature (ADR-055) is served by a route or RPC (ADR-050
keys); the route's handler reaches the code that emits an SLI metric; an SLO spec, an alert rule and a dashboard
panel consume that metric; a runbook or incident declaration (ADR-058) names the route; an error-budget policy says
what happens when the budget burns. Fathom can prove the code-grounded links and catch the breaks:

- **Dangling promises**: an alert or dashboard consuming a metric no code emits, a route or feature with no SLI or
  no SLO. This is a new contract-key kind (`metric:`, provided by metric registrations in code and consumed by alert
  rules, dashboards and SLO specs) matched by the existing link model; `contract_drift` reports the dangling ends.
- **Observability conservation**: a refactor that stops emitting a metric an alert consumes is a lost promise,
  reported at review by the conservation planes (ADR-044), exactly like a lost route.
- **Readiness facts, not grades**: per service, the facts a readiness level needs (routes inventoried, SLI emitted
  on the handler's reach, SLO declared, alert wired, runbook declared). Which facts make level 1 to 5 is a Standard,
  and a Standard is Review Kit's (ADR-055 decision 7; its review domains already include "Reliability and
  operations": recovery, rollback, alerting, SLOs, dependencies). A level renders only from a committed reading
  (ADR-054's rule).

What stays out of fathom: live SLI values, burn rates and alert state (the ops system's data; a rendered link at
most, ADR-058); an organization-wide compliance dashboard (hosted: Review Kit's DATUM, a fathom non-goal); failure-mode
playbooks beyond incident declarations (already F56). A separate Reliability Kit is refused now: the domain already
lives in Review Kit, and a third tool would split the family without an enforced boundary (P21). What works today
with no fathom build: a Review Kit concern pack can compile a reliability convention ("every HTTP handler records a
latency histogram") into obligations served at the edit (ADR-055 decision 8c).

The build is deferred (F59, SV-D5). A census on 2026-09-30 found no alert-rule, SLO-spec or dashboard files in the
repositories on this workstation (a name search to depth 7; the only matches were app-kit fixtures and service deploy
manifests), so there is no consumer (P20) and no oracle. Promote when a named consumer commits alert rules, SLO
specs or dashboards and the contract-key storage (SG-2) and the first per-kind reading (SG-M1) have landed.

### 4.5 Minimal vendoring: accepted, as the used closure and a licence boundary plane

Would fathom help? Partly, and the part it helps with is the part the owner's case needs.

- *What is already solved.* For Go, `go mod vendor` and `go list -deps` are exact and minimal at package
  granularity, and pruning files inside a Go package is unsafe: a type or constant reference is not a call edge, so a
  call-graph file closure would drop files the compiler needs. Fathom must not compete there.
- *What fathom adds.* (1) The reason: which product entry point and which first importer reach each unit ("why do I
  vendor this?"). (2) The non-Go half: Tailwind source lists, `go:embed` patterns, TypeScript and JavaScript imports
  in a component catalog, which no single toolchain command covers. (3) The join nobody else has, which is the
  owner's actual constraint: **licensed sources must not ship publicly**. Default deny: a public project carries a
  licensed path only where the owner has recorded an explicit allow for it (the owner's reading of the licence,
  declared by the owner), never the catalog.
- *Fathom's own tree is the first case.* The binary imports 22 of app-kit's 93 non-test Go packages and 19 of
  agentic-kit's 242, yet `third_party/` mirrors both trees whole (1,806 and 1,602 files), and `scripts/ui-css.sh`
  keeps a hand-written Tailwind source list of 18 of app-kit's 28 UI packages.
- *First slice.* `UsedUnits` computes a dependency directory's closure from product entry points: Go packages by
  the import graph (it must equal `go list -deps` exactly, or it is a bug), TypeScript and JavaScript modules by
  import edges, declared asset roots; each unit with its reason, the rest listed unreached (SV-6). `fathom report
  used <dir> [--json]` prints it (SV-7), a sub-report beside `report evidence` and `report packs`, so no verb is
  added. SV-M3 reads it on fathom's own mirrors: Go units equal `go list -deps`, the CSS units equal the hand-kept
  list, and a mirror pruned to the closure passes `task build` and `scripts/ui-css.sh --check-sources`; the
  reduction is recorded.
- *The boundary: default deny.* Licensed sources (Tailwind Plus and Catalyst are the cases) never ship in a public
  repository, which is app-kit's own delivery rule ("Licensed derivative: keep this directory and generated outputs
  private", `internal/designadapt/*_delivery.go`). A `.fathom/licensed.cue` declaration names the licensed paths, their
  licence and the owner's explicit allows; a new conservation plane blocks a push whose tree holds a declared path
  with no allow, and lists reached and allowed licensed units as a fact (SV-8). An explicit owner allow per path is
  the only exception, and fathom never grants one. The declaration is committed by the repository that ships
  publicly; a repository without one is not checked, and fathom never infers visibility. Today a `forbidden_paths`
  guard already denies an agent's write to a licensed path at the edit (ADR-038), but it cannot see a bulk copy by a
  sync script, which is how vendoring happens; the push is the earliest moment that sees the whole tree (ADR-057's
  placement rule). SV-8 waits on a named consumer (SV-V1): the public repository and the owner's explicit allow, if
  any, per licensed path.
- *Refused.* Fathom copying, pruning or vendoring files (a package manager, and the transformation-engine non-goal):
  the copy stays a script (`scripts/sync-vendor.sh`) that reads the closure JSON. Fathom inferring what is licensed,
  judging derived-work similarity or interpreting a licence: a legal judgment, declared by the owner, never a model.

### 4.6 Prioritization: ditch-then-merge is done; the order after the wind-down

The owner's rule has already run: the 2026-09-29 feature scorecard shows all 10 DITCH verdicts executed (deleted or
frozen) and all 12 MERGE verdicts folded, on `main` (SR-10 to SR-32, ADR-053). What remains of its spirit is
"finish before building": the rows the wired audit reopened and the 24 IMPROVE verdicts come before any new engine.

The order once the owner lifts the 2026-09-30 wind-down (no new waves until then):

1. *Development speed, bounded.* The efficiency wave has 24 rows proven and 6 open; EW-M1 waits 14 days after stop 1,
   which is a wait, not work. ADR-049's amendment already stops efficiency rows jumping the queue if EW-M1 misses its
   bar. Speed is not a standing first priority; it is this wave, then measured.
2. *Language excellence, most common first,* as the main line (G15: 60 rows proven, 138 open, no committed reading
   yet). Every other proof rides on extraction, and it is the first thing a stranger's repository meets. The queue
   is already ordered most common first.
3. *Stewardship first slices* as small $0 fillers between language batches, the lines report first, because the
   2026-09-29 harvest shows it pays back in development speed.

Status: accepted 2026-09-30 by the owner-architect (ADR-060 decision 7). The owner decides when the wind-down ends.

## 5. The decision port

It has no role in any of the five. Every classification above is exact (ancestry, patch-id, plan rows, import
edges, key equality, declared paths). The tempting site, a model drafting a landing summary for a line, is
generation, the port's weak side (ADR-056), and the rows a line claims already carry the summary's content.
Licence questions are never judged (I62's rule for security properties applies in spirit: a wrong "fine to ship" is
the costly error). This is the Jev check the program asks of every architectural decision: considered, no site, no
future.

## 6. What we do not claim

- That the lines report finds valuable work anyone would miss: SV-M1 reads that on the one harvest we have.
- That the used closure beats `go list -deps` for Go: it must equal it. Its value is the reason, the non-Go units and
  the boundary.
- That fathom can tell whether a file is derived from a licensed source. It knows only what the owner declares.
- That promises as code has a user today: the census found none.

## 7. Kill criteria in one place

| Reading | Kills |
|---|---|
| SV-M1 CUT (a valuable held item missed, or more than 25 lines flagged) | the rows classes; the landed class stays |
| SV-M2 CUT (fewer than 8 in 10 flagged leftovers were trash or forgotten work) | the stray classes in `gc` |
| SV-M3 CUT (Go units differ from `go list -deps` after one fix round, or the pruned mirror does not build) | `report used` and the reached-units fact; default deny survives |
| SV-M4 shows no reached upstream change in two quarters | F58 |
| no promise consumer within 365 days | F59 |
