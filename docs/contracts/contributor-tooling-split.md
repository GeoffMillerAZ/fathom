# CTR-1–9 tooling: work split

Artifact: `contributor-tooling-split`, branch `lane/contract-contributor-tooling-split`.
Source-only extraction from `a598d4bbfcecbcc0108164382dc2a22bf6c915eb`.
This branch adds only `docs/contracts/contributor-tooling-split.md`.
Companion citations resolve to that preserved source until individually
landed. Ports, assertions, prerequisites and authority choices are unchanged.

Status: contract preparation only, 2026-11-15. Base:
`83226aece9b0f421381373f8dafc4a912d3888e5`. No implementation dispatch,
reservation, actual RED, integration proof or forge mutation is granted here.
Reuse issue #140. The existing merge-driver work is independently grounded,
not replaced by these scripts. Governing rows: `docs/plans/contributor-model.md`.

| Turn | Rows | Exact implementation ownership | Prerequisites |
|---|---|---|---|
| steward | CTR-1 | `docs/bench/readings/contributor-model/steward.md` | Independent steward; frozen valid/invalid examples; trusted REC-1 manifest handoff |
| queue A | CTR-2 | `scripts/lane_queue.py`, `scripts/lane_queue_test.py`, `docs/bench/readings/contributor-model/queue.md` | CTR-1 and actual REC-1 on main; paginated issue/claim snapshots and operator capacity |
| paths | CTR-3 | `scripts/lane_paths.py`, `scripts/lane_paths_test.py`, `docs/bench/readings/contributor-model/paths.md` | CTR-1 and actual REC-1 on main; trusted base grant; NUL diff fixtures |
| wiring | CTR-4 | `Taskfile.yml`, `.github/workflows/ci.yml`, `docs/bench/readings/contributor-model/wiring.md` | CTR-2/3 on main; integrator-only serial turn; fork-safe fixture repository |
| queue B | CTR-5 | `scripts/lane_queue.py`, `scripts/lane_queue_test.py`, `docs/bench/readings/contributor-model/expiry.md` | Queue A returned and landed; push-history/recheck snapshots |
| queue C | CTR-7 | `scripts/lane_checkin.py`, `scripts/lane_checkin_test.py`, `docs/bench/readings/contributor-model/checkin.md` | Queue B landed; imports queue interface read-only; plan/CI/reading/event snapshots |
| handoff | CTR-6 | `docs/bench/readings/contributor-model/handoff.md` | CTR-4 landed; independent critic; local fixture replay |
| WSL2 | CTR-8 | `docs/bench/readings/contributor-model/windows.md` | CTR-6; actual WSL2 worker and red fixture lane |
| gate reading | CTR-9 | `docs/bench/readings/realignment/gate-time.md` | Existing completed gate logs; no new full gate |

Queue A → B → C is the shared interface's serial order. CTR-7 owns only its
check-in files, never queue files. A required queue-interface change returns
to the integrator for a separate queue turn, never an implicit wider grant.
Paths and queue A can proceed independently after their common prerequisites.
CTR-9 can proceed independently when its raw inputs exist. Readings cannot
replace missing native inputs with a drafted command or another platform.

## Exact future manifest handoff

This is a contract for prior integrator commits, not live reservations. Each
manifest has exactly the paths in its turn's table row, `id_ranges: []` unless
separately granted, the turn's row id, the actual grounded base/proof, and the
same REC-1 closed shape. No defaults authorize record creation.

| Turn | Trusted manifest path / lane id |
|---|---|
| steward | `docs/lanes/contributor-steward.cue` / contributor-steward |
| queue A then B | `docs/lanes/contributor-queue.cue` / contributor-queue |
| paths | `docs/lanes/contributor-paths.cue` / contributor-paths |
| wiring | `docs/lanes/contributor-wiring.cue` / contributor-wiring |
| queue C | `docs/lanes/contributor-checkin.cue` / contributor-checkin |
| handoff | `docs/lanes/contributor-handoff.cue` / contributor-handoff |
| WSL2 | `docs/lanes/contributor-windows.cue` / contributor-windows |
| gate reading | `docs/lanes/contributor-gate-time.cue` / contributor-gate-time |

Queue A returns, lands and releases its shared file turn before the integrator
updates the **same** contributor-queue manifest for B in a separate trusted-base
commit. Replace CTR-2's rows/reading grant with CTR-5's exact rows/reading grant,
retain the two queue files, then re-ground at that new base. Do not create two
lane declarations granting those files at once. CTR-7 reads queue files and
grants only its own check-in files. A Taskfile owner (including REC-7 #319)
returns before CTR-4 receives its turn, or vice versa. No active-phase field,
implicit historical grant, sibling-plan or epoch allowance is introduced.

## Draft packet

Contract documents in this track, each on its own isolated branch: this map and
`contributor-tooling-trust-paths.md`, `contributor-tooling-queue.md`,
`contributor-tooling-wiring-readings.md`, all under `docs/contracts/`.
They freeze exact interfaces and prospective tests before implementation.
No plan boxes change. W2-3 approval requires a separate critic's LAND and
settling citations for every decision. Foreman checks F5 after approval.

Source map: contributor plan CTR-1–9; contributor guide CT-D1–8;
parallel-lanes PL-D1–6, PL-D8–12, PL-D14 and §§8–9; approved
`docs/contracts/rec-1.md` D1–6 and its trusted-base checker/allocator contract;
`internal/ownership/ownership.go` for review matcher semantics only;
issue #140 and current Taskfile/CI/issue form/merge-driver sources.

Approved source reuse after main advanced: `ctr-11.md` and
`records-integration.md` own REC-7 and its later Taskfile/lefthook turn;
`ctr-12.md` and `command-reservations.md` preserve full CI and serial shared
turns. These contracts are not proof those implementations exist.
