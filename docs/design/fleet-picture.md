# Fleet picture: a live operating picture across the fleet and inside each project

Status: Proposed 2026-11-12 (architect ruling, owner review pending). Plan: `docs/plans/fleet-picture.md` (FLT-). Depends on
REC-8 (plan keys `wave`, `lane`, `feature`) and on ENG-7, ENG-9 and ENG-10 in `docs/plans/realignment.md`. Work plane:
ADR-098. Sources: `docs/vision/35-observatory-console.md`, ADR-045, ADR-049 decision 4, ADR-051, ADR-078 D5, ADR-081,
`docs/plans/work-signals.md`, `docs/plans/progress-and-economics.md`, the console files under `cmd/fathom`.

Codes in this doc: FV-U uplift, FV-F finding, FV-D decision, FV-R risk. The build order is the last section.

## 1. The uplifted ask

The ask: stale projects and engine health first, then progress bars on waves and features, what each agent is doing, and how
work relates. Session boards over worktrees already exist and are free. The better question:

> What must a person steering a fleet decide in the next minute, and which of those decisions need facts, intent and proof
> joined together?

Added to the ask:

- FV-U1 An attention list ranked by what blocks proof, each item phrased as one typed question.
- FV-U2 Overlap forecast: which live lanes will collide, before they merge.
- FV-U3 Proven against claimed, everywhere. A bar never shows a claim as progress.
- FV-U4 The age and source of every number, with "unknown" as a state.
- FV-U5 Proof freshness: which readings and certificates went stale because code under them moved.
- FV-U6 The same picture as JSON from a verb.

## 2. What exists, and is reused

- FV-F1 The console is real: 39 server-rendered pages, each panel renders a verb, every page answers as JSON, values carry a
  truth state (present, zero, absent, stale, partial, error).
- FV-F2 Refresh is notify and reload. A stream says a panel changed and the person refreshes it. The kit's live script inserts
  no HTML, and a kit test forbids it. ADR-045 refused pages that move content under the reader.
- FV-F3 Progress pages exist: proof bars, weighted rollup, burn-up, regressions, a derived read-only board, cost per proven
  row. They read markdown at render.
- FV-F4 Work pages exist for ADR-051's joins: effort by zone and agent identity, friction, hotspots, the model and harness
  scorecard.
- FV-F5 Stall detection is designed (the work-signals plan) as a daemon fold, which the table freeze defers (ADR-049
  decision 2). Computed at render it needs no job and no freeze.
- FV-F6 There is no in-flight view. The session ledger is written at Stop.
- FV-F7 Lanes are scattered: `plans lines` is a CLI, leases decorate two pages, threads have no page.
- FV-F8 Attribution is thin. 20% of recent call-log rows carry a session id.
- FV-F9 Engine tuning has no panel. Operate shows latency, errors and tenancy. It does not separate answered, declined and
  abandoned, or show graph build time, eviction causes or ingest cost trends.
- FV-F10 The queue is on the forge: 108 open queue items, 15 blocked, 4 waiting on a ruling.
- FV-F11 Browser proof of the console has never run, because the browser aborts at launch.
- FV-F12 Outside: harness vendors ship session views and telemetry export. None shows progress verified against a plan,
  staleness of the facts agents rely on, or cross-lane overlap.

## 3. The picture: four questions, in the order asked

### 3.1 Is the ground solid? (first)

One row per project. All from existing sources.

| Signal | Source | States |
|---|---|---|
| Index freshness | store watermark against the checkout's HEAD | fresh, behind by N commits, never ingested, unknown |
| Binary freshness | daemon build commit against the installed binary | current, deploy needed |
| Wiring | doctor rows, the harness descriptor, the last session's wire stamp | firing, wired and silent, drifted, absent |
| Intent freshness | record conditions: ReviewDue, fired conditions, unresolved links | counts by kind |
| Proof freshness | readings and certificates whose inputs changed | live, stale, void |
| Gate | last gate receipt for HEAD, last CI run | green, red, none for this tree |

Engine strip, per daemon:

| Metric | Why |
|---|---|
| Answered, declined by name, abandoned: share by tool and caller | The three are different failures and are merged today. |
| Graph build time and re-warm counts | The cause of declines on the hook path. |
| Evictions by cause | 67% are watermark moves. |
| Hook latency against each hook's budget | Hooks are fast. The daemon call is not. |
| Shed and refuse minutes, self-caused apart from host-starved | The split the freeze needs. |
| Ingest duration and peak footprint per service, as a trend | Tuning needs a trend. |
| Store size and growth per service | The cost side of every new fact kind. |
| Minutes per gated landing, lease wait, CI pass rate on main | The landing bottleneck, unmeasured today. |

Each tuning knob gets one row: its value, the reading that set it, the date.

### 3.2 How far is the work, by proof?

- Program, wave, feature, row. Three segments per bar: proven, claimed but unproven, open. The middle segment is the headline.
- Stale proofs and regressions sit under the bar that owns them.
- This extends the existing progress page. It needs `wave`, `lane` and `feature` as plan keys (REC-8).

### 3.3 What is each agent doing?

One row per lane. A lane's id is explicit: the lane variable that the provenance verb already reads, set in the lane's
environment from its record. Branch and worktree root are a fallback, and the share of rows attributed by fallback is a
reported number, because a hook's working directory is the session's and a sub-agent in a clone inherits the parent's.

| Field | Source |
|---|---|
| State: active, idle, waiting on a person, gating, landed, at risk | hook beats, lease, gate lease, `plans lines` classes |
| Row in hand | the lease, else the lane record's rows |
| Touching | changed files since the lane base, mapped to symbols |
| Last event and its age | the lane file |
| Last gate | the gate receipt for the lane head |
| Spend | the session ledger at Stop. An in-flight estimate only if the owner wants it, labelled. |
| Identity | harness, model, effort, host label. Never a person (invariant I58). |

Stall signals come from the work-signals design. They are computed at render from the lane file's tail, so they need no
background job. Fathom shows them with their evidence and acts on none.

### 3.4 How does the work relate?

- **Needs tree.** Rows and features with `needs` edges, blocked chains shown from the root blocker.
- **Overlap forecast.** For each pair of live lanes: shared files, shared functions, intersecting reach. Shown with its
  evidence, never as a lock. The integrator uses it to order merges. CHG-7 computes it.
- **Record links.** The `why` walk from an ask or a decision to rows, symbols and receipts, as a tree.
- **Attention list.** Proof regressions, red CI on main, lanes waiting on a ruling, overdue reviews, stale proofs, forecast
  collisions. Each item is one typed question with its options. It is a read-only view.

## 4. Architecture

### FV-D1 State is a fold over files

Existing files: hook beats, hook latency, the call log, the session ledger, the verify ledger, gate receipts, leases, the
daemon lifecycle log. One new file per machine: a lane file, appended by the existing hooks, with its own rotation and an
archive. A row carries the lane id, session id, harness, model, host label, event kind, target path and row id. No content.
Hooks read no record to write it. Every row that edits a hook file runs through the one hooks lane.

Two fixes come first: session and lane on every call and hook row (ENG-9), and gate-clone verify runs recorded under the
checkout they verify (ENG-10).

The fold is computed at render over a bounded tail. A resident fold is a derived cache and waits for the freeze amendment
(ADR-095) with its own cost row.

### FV-D2 Across machines: the operator moves files, fathom reads them

Fathom dials nothing. Invariant I14 and ADR-083 are unchanged. Fathom exports lane state to a local path and reads lane state
from a local path. The operator's own tools carry the bytes. This is the ADR-078 posture: every machine its own loopback
daemon, receipts move evidence, placement is not fathom's.

| Transport the operator may run | Freshness |
|---|---|
| A synced or shared directory | seconds to a minute |
| A git ref per machine, pushed with normal pushes | push cadence |
| The operator's own ssh pipe from an export on one machine to a file on another | seconds |

The export is content-free: ids, counts, paths, conditions, ages. Each panel shows the age of its newest row per machine, so a
slow transport looks slow and never looks idle.

### FV-D3 Honest freshness

- Local: under 2 seconds from event to notification.
- Remote: as fresh as the operator's transport, with the age shown.
- A daemon that is down, a file that cannot be read and a machine not heard from are each a named state.

### FV-D4 Rendering

UI work follows the app-kit doctrine in `cmd/fathom/AGENTS.md`: never hand-write observatory markup, role utilities only, CSS
through `task ui:css`. Before any panel, read `docs/app-kit.md` and route through the app-kit guidance.

- New panels are projections in the console package, each a verb first: `fleet health`, `fleet lanes`, `fleet overlap`,
  `fleet attention`. Kit components only.
- Notify and refresh until the kit offers a live region. A live region that updates rows in place without reordering is a kit
  contribution with its own kit decision (FLT-7).
- Relations start as indented trees and tables. The layered graph renderer waits for its kit row.

### FV-D5 The board writes nothing

ADR-049 decision 4 holds: no view sets a phase, a claim or an assignment. A ruling is entered through a record verb. Fathom
writes no issue and no label. The forge's queue is shown from a file the operator exports.

### FV-D6 Do not rebuild what harnesses ship

Session lists, transcripts, token meters and worktree management belong to the harness and the terminal multiplexer. The board
links to them by lane. It adds proof, intent, overlap and staleness.

## 5. What it must not become

A tracker, a scheduler, a token dashboard, a leaderboard of people, a hosted service.

## 6. Proof

| Id | Reading | Bar | Outside oracle |
|---|---|---|---|
| FLT-M1 | Event to notification, local | p95 under 2 seconds | a scripted event with a timestamp |
| FLT-M2 | Board equals verbs | every panel's JSON equals its verb's JSON | the existing parity test, extended |
| FLT-M3 | Overlap forecast on a 30-day replay | at least 70% of same-function conflicts forecast, under 30% false alarms | git merge history |
| FLT-M4 | Attribution coverage | at least 95% of hook and call rows carry session and lane | row counts |
| FLT-M5 | Ruling latency | median time from "needs a ruling" to a recorded ruling falls by half | the forge's label history |
| FLT-M6 | Browser proof | both themes, wide and narrow | a browser run, which has never succeeded here |

Page views are fathom's own log. They are used only to cut pages, never as proof of value.

## 7. Decisions

- FV-D1 to FV-D6 as above.
- FV-D7 Build in the order asked: ground, progress, lanes, relations.
- FV-D8 Reuse the progress, work, sessions and operate pages. Extend them. Add a lanes page and the attention list.
- FV-D9 Amend the dispatch non-goal's "never watches" in the open, and split ADR-078 D5 (ADR-098).
- FV-D10 Fix attribution before any lane panel ships. A board over a fifth of the rows would mislead.
- FV-D11 Fix the browser proof (FLT-0) before new pages land, or they inherit an uncertified gate. FLT-1 extends existing pages
  and does not wait.

## 8. Risks

- FV-R1 The board becomes the product. Mitigation: the ground view ships alone and doubles as the engine program's instrument.
- FV-R2 Paths cross machines. Mitigation: only between the operator's own machines, by the operator's own transport, never in
  a shareable bundle.
- FV-R3 The overlap forecast cries wolf. Mitigation: FLT-M3 before it is shown by default.
- FV-R4 The lane file shortens hook-log retention. Mitigation: its own file with its own rotation.
- FV-R5 The attention list reads as a tracker. Mitigation: it holds no state. Closing an item means the underlying fact
  changed.

## 9. Build order

1. **Ground.** Attribution, the ledger fix, the fleet freshness table and the engine strip (FLT-1 with ENG-7, ENG-9, ENG-10).
   New projections over existing files. No dependency on records.
2. **Progress.** Wave and feature bars on the existing progress page, after the plan keys land (FLT-2).
3. **Lanes.** The lane file, the lanes page, stall signals at render, then operator-moved state from other machines (FLT-3,
   FLT-4).
4. **Relations.** Needs tree, the overlap forecast that CHG-7 computes, record links, the attention list (FLT-5, FLT-6).

Step 1 is what the owner asked to see first, and it is the instrument the engine program needs. Build it with that program.

## Decision models

None applies. Every panel is computed. The stall signals stay deterministic at provider none, as the work-signals design says.
Opting out is the default: no panel calls a provider. Nothing to kill.
