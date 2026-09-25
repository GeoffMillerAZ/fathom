# 35 — The Observatory, grown into a console

Status: accepted 2026-09-25 (ADR-045). Owner directive of 2026-09-25: "come up with the
vision of what the UI/UX should be, be very ambitious for what it should cover, then
execute it to completion, adopting and contributing back to the app-kit." Design:
`docs/design/observatory-console.md`. Plan: `docs/plans/observatory-console.md`.
Builds on vision 17 (the stance and laws stay binding); supersedes its §5 "not now" for
live updates only, for the reason in §7.

## 1. What is wrong today

Seven pages exist and each is honest in isolation, but together they fail the job:

- **Almost everything fathom knows is invisible.** 1,057 plan rows with a proof state
  and 13,443 verify-ledger rows. 4,025 governed claims across five layers. 82 compile
  runs. 71 certification targets. The reading set of every refactor. 88,875 tool calls
  with latency and errors. 41 doctor checks per repo, 9 flywheel loops, 120 decision-port
  rulings. All of it lives only behind CLI verbs.
- **The pages contradict each other.** One page grades a repo "A", another calls it idle.
  "≥114" becomes "114" elsewhere. Ledger rows are counted as sessions. 50,747 calls on
  one page, 88,875 on another. Colored zeros stand in for tools with no instrument.
  Each page computes its own number.
- **Nothing explains fathom.** No page shows how a rule reaches an edit, what a layer or
  a dialect pack is, or which question goes to which tool. 52 of 59 tools have never
  been called.
- **The shell is thin.** Flat navigation, no search, no breadcrumbs and no theme choice.
  Error pages are plain text. Charts have no axes, and no chart drills down to its rows.

## 2. The idea

**The Observatory becomes the place a person goes to see what fathom knows, why it
believes it, and what to do next.** It covers three jobs in one shell:

1. **Steer.** See the fleet and each project: what is wired, fresh, proven, stale,
   refused or waiting. Every problem carries its evidence link and the exact command
   that fixes it.
2. **Understand.** See fathom's machinery at work on your own code: the index a dialect
   pack produced, the claims a layer compiled, the rule served at an edit and the ones
   displaced, and a plan row's proof chain.
3. **Operate.** Run fathom itself: daemon health, surfaces and latency, the flywheel,
   configuration, the decision port, wiring and receipts.

It stays what vision 17 made it. It is honest first, local only (law 13), and read-mostly:
mutations happen only behind the local-mutation gate. It is verb-first (P10): **every
panel is the rendering of a CLI verb, and the panel names that verb.**

## 3. Information architecture

A grouped sidebar, breadcrumbs, a page heading with meta and actions, and ⌘K from
anywhere.

| Group | Page | Question it answers |
|---|---|---|
| **Now** | Home | What needs my attention, and what changed since I last looked? |
| **Fleet** | Fleet | How is every project doing: wired, fresh, proven, used? |
| | Project workspace | Everything about one repo, in tabs (below) |
| **Proof** | Plans | What is planned, what is proven done, what remains, and how is it trending? |
| | Certifications | Which invariants, ports and incidents have live test proof? |
| | Refactors | What did a change keep, lose, or leave for a human to read? |
| **Knowledge** | Semantic layers | What governed claims exist, where they came from, and how fresh they are |
| | Packs | Which dialect and layer packs exist, what each contributes, and where each is active |
| | Serving explainer | What governs this file, and why this rule and not that one? |
| **Evidence** | Impact | Has fathom helped, measured honestly, with ranges and arm counts? |
| | Sessions | What agents did, with fathom and without |
| | Intent | Invariants, decisions, futures and questions, and whether each still holds |
| **System** | Operate | Daemon health, surfaces, latency and errors, memory, flywheel, capabilities |
| | Wiring | Doctor per repo, drift, preview/apply/undo, receipts |
| | Decision port | Jev's rulings, calibration and latency |
| **Learn** | How fathom works | The pipeline, live, on your repo |
| | Ask the right way | Question shape to tool, with a live example |
| | Guides and glossary | Use-case recipes, laws and terms |

**The project workspace** gets tabs:

- Overview: the realization ladder, wired → fresh → queried → preventing → gating, with
  evidence and a remedy on the first broken rung.
- Index: the dialect packs at work.
- Governance: layers, claims and the serving explainer.
- Plans.
- Proof.
- Refactor.
- Intent.
- Activity.
- Wiring.

## 4. The signature views

These are the views that make fathom legible. Each has a table alternative and survives
both themes and a 390 px screen.

1. **The pipeline.** Source → extract (dialect packs) → index (symbols, typed edges) →
   compile (layer packs → claims) → serve (rules at the edit) → gate (verify, conserve)
   → evidence (ledgers). Each stage shows live counts for the selected repo and links to
   its page. It is the map of the product and the answer to "how does fathom help".
2. **The strata.** Horizontal bands for dialect, provider (parked), platform and concern.
   Packs sit as blocks in their band, and each layer shows claim counts, a lifecycle
   strip (compiled → fresh → stale/contradicted → superseded/refuted), coverage of its
   corpus, and cost. A services × layers matrix shows where knowledge exists and where
   it is missing. Missing is hatched, never zero.
3. **The serving explainer.** Pick a file. The page shows:
   - the candidate claims in four columns by scope tier (file, package, ancestor
     directory, root), in ranking order;
   - the cap line, the served set, and every displaced rule with its reason;
   - raw-fallback rows, flagged.

   This is the obligations algorithm made visible: why the agent saw what it saw.
4. **The proof ledger.** Each plan gets a proof bar: proven, claimed-unproven, open, op,
   deferred, refused. A board sorts rows into Remaining, Claimed, Proven, Deferred and
   Op. Each card names its contract (symbol and test reach, lexical oracle, `gone:`,
   `certified_by`). A drawer shows the evidence the gate used, with a burn-up of claimed
   against proven from the verify ledger. Across the fleet this becomes small multiples,
   one per repo.
5. **The dialect matrix.** Languages × capabilities:
   - extraction engine, symbol tier, typed edges, call-edge share;
   - certificates, integrity support, inventory keys.

   Measured precision and recall appear where they exist (L0 oracle, CV-M7), and
   "unmeasured" is said where they don't. It shows what a dialect pack actually delivers.
6. **The code neighborhood.** A symbol's callers and callees to depth 2 as a layered
   graph. Resolved edges are solid, heuristic ones dashed, test-reaching paths
   highlighted. Blast radius uses the same renderer. A bounded node count aggregates the
   remainder and says so.
7. **The reading set.** The conserve report's planes as verdict chips. Each unexecuted
   changed line range renders as an inline code excerpt, so the reviewer reads exactly
   what no test ran.
8. **The catches timeline.** Verify refusals, conserve losses, integrity catches and
   guard refusals on one filterable timeline, next to the counterfactual arms as range
   bars with their counts (law 12).

## 5. How it behaves

- **Every number carries its instrument.** Each panel states:
  - the question it answers;
  - the verb that produces it, with a copy button (`fathom plans status --service S --json`);
  - its source, denominator and freshness;
  - its truth state: zero, missing, stale, partial or error, each drawn differently.

  One shared projection per metric feeds every page that shows it, so pages cannot
  disagree.
- **Every number drills down.** A count links to its rows. Filter state lives in the URL,
  and back returns to where you were.
- **Remedies, not buttons.** A problem shows the command that fixes it. Only wiring
  preview/apply/undo mutates, behind the existing local gate.
- **Works without JavaScript; better with it.** Every command-palette result, filter and
  drill-down is a real route. A small L2 layer adds:
  - ⌘K;
  - the theme toggle;
  - drawers;
  - live freshness.
- **Live, gently.** Server-sent events announce index refreshes, verify runs, compile
  completions and new sessions. The page shows "updated — refresh this panel" and never
  moves content under the reader.
- **Budgets.**
  - Warm render ≤ 150 ms server-side.
  - HTML ≤ 250 KB per page.
  - Cold first paint ≤ 1 s from a warm model cache.
  - Session replay and tables are windowed or paginated; nothing ships a 9 MB payload.

## 6. App-kit: adopt fully, contribute back

The Observatory moves to the current app-kit and uses what the kit already ships and
fathom ignores today:

- `table` (sortable, row links);
- `chrome` (breadcrumbs, tabs, empty states, description lists, pagination);
- `topology`, `kanban` and `overlay` drawers;
- `appearance` (theme);
- `docs` (table of contents);
- the analytics comparison recipe;
- `viz` heatmap, funnel, steps, stack bar, range bars and timeline filters.

Where the kit lacks a piece, fathom builds it **just in time as the originating
consumer** and returns it under the kit's contribution loop (ADR-0066). Each piece gets
a catalog entry, specimens, golden tests and a contribution record. Each stays
experimental until a second consumer adopts it:

- a command palette (L2 + L3b);
- a deterministic layered-graph renderer for flows and neighborhoods (L3a);
- truth-state cells for the heatmap: missing, not zero (L3a);
- a code-excerpt viewer with line anchors (L3b);
- a unified text diff (L3b);
- a live-freshness helper over SSE (L2);
- a page heading with meta and actions (L3b);
- catalog entries for the shipped-but-uncatalogued packages.

Domain views stay fathom's own: the proof ledger, the strata, the serving explainer and
the dialect matrix.

## 7. Revisited from vision 17

Vision 17 deferred live updates "until meta-refresh isn't enough". It no longer is. The
fleet model is 297 KB and takes 2 s cold, a meta-refresh would re-render it all, and the
daemon already holds an idle `/events` stream. Live freshness over SSE is admitted.

Two things stay out:

- **Chart libraries.** Every chart is a kit SVG primitive.
- **Hosted sharing and per-screen auth.** Law 13 still applies.

The non-goals hold: no chat, no dispatch loop, no Node toolchain.

## 8. The decision port

Jev gets one narrow job. The ⌘K palette may offer a *suggested destination* for a
free-text question: "which view answers 'who calls X'". It uses the existing route site,
is off by default and requires a site opt-in (`[decide] sites`). The suggestion is
always hedged and never answers the question itself.

It never:

- asserts a fact;
- ranks evidence;
- decides what is shown.

Measurement: the share of suggestions the person follows. Below one in three over 30
days, it comes out.

## 9. Kill criteria

- **UX-M1.** Ten operator tasks are answered in three clicks or fewer from Home, by
  scripted browser journeys. Each answer must equal its CLI verb's JSON. Examples:
  - which rows of plan P remain;
  - what governs file F and why rule R was displaced;
  - which layers are stale;
  - what fathom caught this week;
  - the daemon's p95.
- **UX-M2.** Every page passes both themes, 390 px with no horizontal overflow, a
  keyboard path, no console errors, and its HTML and render budgets.
- **UX-M3.** A consistency test finds zero contradictions across pages sharing a metric.
- **UX-M4.** After 30 days of panel-traffic counting, a page nobody opened is a candidate
  to remove. This is judged under law 8's consumer-population rule, never on zero alone.
