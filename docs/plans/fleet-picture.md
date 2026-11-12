# Fleet picture: a live operating picture across the fleet and inside each project

Grammar v2 (`docs/plans/README.md`). Prefix: FLT- (FLT-n work, FLT-Mn readings). Design: `docs/design/fleet-picture.md`.
Decisions: ADR-098 (work plane), ADR-049 decision 4 (views never trackers), ADR-078 D5, ADR-045 (the console), invariants I14
(loopback only) and I58 (no ranking of people). Status: Proposed 2026-11-12 (architect ruling, owner review pending).

Depends on `typed-records.md#REC-8` (plan keys `wave`, `lane`, `feature`) and on `realignment.md#ENG-7`, `realignment.md#ENG-9`
and `realignment.md#ENG-10`. FLT-5 reads the overlap forecast that `realignment.md#CHG-7` computes.

No store table, column, cache or background job. Every panel is a projection of a verb over existing files plus one lane file
per machine. UI work follows the app-kit doctrine in `cmd/fathom/AGENTS.md`: kit components only, role utilities only, CSS via
`task ui:css`, and `docs/app-kit.md` consulted first. Readings live under `docs/bench/readings/fleet-picture/`, one file per
row, each opening with a verdict line. A row flips only through `fathom verify --service fathom --plan docs/plans/fleet-picture.md`.

## Phase 0: certify the gate

- [ ] (op) The console's browser proof launches and runs on one existing page in both themes, so new pages do not inherit an uncertified gate {id: FLT-0, oracle: lexical, path: docs/bench/readings/fleet-picture/browser-launch.md, pattern: "^FLT-0 verdict:"}

## Phase 1: ground

- [ ] `fleet health` verb and console panel: one row per project (index, binary, wiring, intent, proof freshness, gate) with named states, and the engine strip per daemon with a row per tuning knob, JSON equal to the panel {id: FLT-1, symbol: cmdFleetHealth, package: cmd/fathom, needs: "realignment.md#ENG-7,realignment.md#ENG-9,realignment.md#ENG-10"}

## Phase 2: progress

- [ ] Wave and feature bars on the existing progress page: proven, claimed but unproven, open, with stale proofs and regressions under the owning bar {id: FLT-2, symbol: progressSegments, package: cmd/fathom, needs: "typed-records.md#REC-8"}

## Phase 3: lanes

- [ ] `AppendLaneEvent` writes one content-free row (lane id, session id, harness, model, host label, event kind, target path, row id) to a per-machine lane file with its own rotation and archive, called by the existing hooks and reading no record {id: FLT-3, symbol: AppendLaneEvent, package: internal/lanefile, needs: "realignment.md#ENG-9"}
- [ ] `fleet lanes` verb and page: state, row in hand, touching, last event and age, last gate, spend at Stop, identity by agent never person (I58), stall signals computed at render from the lane file's tail {id: FLT-4, symbol: cmdFleetLanes, package: cmd/fathom, needs: FLT-3}
- [ ] `ExportLaneState` and `ReadLaneExports` write and read content-free lane state at a local path, never dial (I14), and each panel shows the age of its newest row per machine with a named state for a machine not heard from {id: FLT-10, symbol: ReadLaneExports, package: internal/lanefile, needs: FLT-3}

## Phase 4: relations

- [ ] `fleet overlap` verb and panel show each live lane pair's shared files, functions and reach with evidence, hidden by default until FLT-M3 passes {id: FLT-5, symbol: cmdFleetOverlap, package: cmd/fathom, needs: "realignment.md#CHG-7,FLT-4"}
- [ ] Needs tree, record-link walk and the read-only `fleet attention` list, each item one typed question with its options and no stored state {id: FLT-6, symbol: cmdFleetAttention, package: cmd/fathom, needs: "FLT-4,typed-records.md#REC-8"}

## Phase 5: kit and docs

- [ ] A live region that updates rows in place without reordering is recorded as a kit contribution with its own kit decision {id: FLT-7, oracle: lexical, path: docs/app-kit.md, pattern: "live region", needs: FLT-1}
- [ ] Docs: the use-case doc names `fleet health`, `fleet lanes`, `fleet overlap` and `fleet attention`, and a change note ships {id: FLT-8, oracle: lexical, path: docs/use-cases/fleet-picture.md, pattern: "^## Maintained", needs: "FLT-5,FLT-6"}
- [ ] (op) The console's 30-day page-view log is read and every page with no views is listed by name. The cut itself is a `gone:` row the integrator writes once the pages are named (ADR-104 decision 5), and no page with a view is cut {id: FLT-9, oracle: lexical, path: docs/bench/readings/fleet-picture/page-views.md, pattern: "^FLT-9 verdict:"}

## Readings

- [ ] (op) FLT-M1 event to notification, local: p95 under 2 seconds, against a scripted event with a timestamp {id: FLT-M1, oracle: lexical, path: docs/bench/readings/fleet-picture/m1-latency.md, pattern: "^FLT-M1 verdict:", needs: FLT-4}
- [ ] (op) FLT-M2 board equals verbs: every panel's JSON equals its verb's JSON, by the existing parity test extended {id: FLT-M2, oracle: lexical, path: docs/bench/readings/fleet-picture/m2-parity.md, pattern: "^FLT-M2 verdict:", needs: "FLT-1,FLT-4"}
- [ ] (op) FLT-M3 overlap forecast on a 30-day replay: at least 70% of same-function conflicts forecast and under 30% false alarms, against git merge history {id: FLT-M3, oracle: lexical, path: docs/bench/readings/fleet-picture/m3-overlap.md, pattern: "^FLT-M3 verdict:", needs: FLT-5}
- [ ] (op) FLT-M4 attribution coverage: at least 95% of hook and call rows carry session and lane, by row counts {id: FLT-M4, oracle: lexical, path: docs/bench/readings/fleet-picture/m4-attribution.md, pattern: "^FLT-M4 verdict:", needs: "realignment.md#ENG-9"}
- [ ] (op) FLT-M5 ruling latency: the median time from "needs a ruling" to a recorded ruling falls by half, against the forge's label history {id: FLT-M5, oracle: lexical, path: docs/bench/readings/fleet-picture/m5-ruling-latency.md, pattern: "^FLT-M5 verdict:", needs: FLT-6}
- [ ] (op) FLT-M6 browser proof of the new pages in both themes, wide and narrow {id: FLT-M6, oracle: lexical, path: docs/bench/readings/fleet-picture/m6-browser.md, pattern: "^FLT-M6 verdict:", needs: "FLT-0,FLT-6"}
