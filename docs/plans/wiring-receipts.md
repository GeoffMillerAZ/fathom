# Wiring receipts

Grammar: v2. Prefix: WR-.
Scope: every bench cell and every new hook record carries a wiring receipt, so a reading says whether fathom was wired when it was taken. $0 work only: fixtures, harness gates, stamps, oracles and dry-run registrations. No paid run, no new table, no daemon-side write, no hook, guard or verify-gate behavior change (those stay local with Opus review).
Dependencies: L8 (AD-21 compliance replay) and L10 (AD-22 exploration instrument) are rows in `docs/plans/advice-delivery.md`; L9 (MOD-14, MOD-15 payload fixtures) is in `docs/plans/claude-code-mods.md`; L12 (language dialects) is the ADR-048 queue, `docs/plans/language-breadth.md`. None is repeated here.

## Honesty protocol

Binding for any reading this plan produces.

- H1 Receipt or void: every cell or run captures a receipt at run time (MCP connected, hooks fired by name, primer names the service, index commit equals subject commit, probe rule served, no channel off, the `wire` stamp); a cell without one is recorded `instrument_defect` and never scored. A hook-free arm still proves hooks were absent.
- H2 Positive control: every run includes one deliberately broken arm built from the WR-3 corpus; the receipt must read it as broken or the whole run is void.
- H3 Pre-register before the first cell: criterion, primary metric, n, stop rule and flaky-task exclusions are committed first; a later criterion is a new registration and the old one is still reported as failed.
- H4 Baselines: reuse control arms by cell key, pin the full model id (never an alias), always run the `none` arm.
- H5 n and statistics: n=2 is "pilot" and supports no claim; a claim needs the registered n with a paired bootstrap; an interval including 0 is null; `error_max_turns` cells are counted separately.
- H6 Truth from oracles (go/types, SCIP, `cmd/loctruth`, publicloc), never from fathom's authors; fathom-repo results are their own row, never the headline.
- H7 Scorer: precision scoring is the default for new registrations; a scorer change re-scores old cells as a new reading and old readings are never edited.
- H8 Evidence classes: proof is an oracle reading or `fathom verify` exit 0 with "N claimed: N verified"; measurement is a pre-registered run with receipts; everything else, including every record without `wire` and every run voided before 2026-09-13, is anecdote.
- H9 Cost order: free oracles first; a paid run needs a dry `scenariobench plan` with its spend estimate and the owner's approval.

## Table freeze

Freeze-compatible (hook-written files, outside the freeze per the Q-RO-1 precedent, `ADR.md` under "Owner questions, answered 2026-10-16"; owner-architect decided D1, not blocking): WR-1, WR-2, WR-3, WR-4, WR-5, WR-6, WR-7, WR-8, WR-9, WR-10, WR-11, WR-12, WR-13. WR-9 and WR-12 are readers only.
Wait for the freeze to lift (RT-M1 PASS, `docs/vision/38-proof-layer-roadmap.md`): WR-14 (W1 queryable doctor history, W2 `wire`/`idx` columns on test_runs, W3 real config digest and any cache keyed on it, W4 periodic sampler job) and WR-15 (daemon-side call-log fields).

## Rows

L1 Home-bench receipt gate
- [ ] Receipt gate in the home bench: a cell whose stream shows no connected MCP server, no named hook firing, or a primer without the service is voided as `instrument_defect`, mirroring scenariobench's `hooksInstrumentError`; planned new symbol `ReceiptDefect`. Bar: table tests over recorded stream fixtures; a fixture with `mcp_servers []` is voided; a good fixture's `Score` is byte-equal to today's. Opus reviews once. [H1] {id: WR-1, symbol: ReceiptDefect, package: internal/contextbench, ctx: hooksInstrumentError internal/scenariobench/sessions.go Score internal/contextbench/scorer.go CellKey internal/contextbench/cell.go}
- [ ] Positive-control rule: a run is void unless its deliberately broken arm (from WR-3 fixtures) reads as broken in the receipt; planned new symbol `VoidWithoutPositiveControl`. [H2] {id: WR-2, symbol: VoidWithoutPositiveControl, package: internal/contextbench, needs: "WR-1, WR-3"}

L2 Seeded-breakage wiring corpus
- [ ] Fixture sessions (beats, call-log lines, settings, skip beats) each labelled with exactly one fault (hook unregistered, stale binary, daemon down, rules empty, channel off, MCP not connected, index behind HEAD) plus healthy ones, under `testdata/wiring-faults/`. Fixtures are produced by the product's own writers with an injected clock, never hand-written. Bar: regeneration is byte-stable; each label is the fault the generator injected (truth by construction, never recomputed by diagnosis logic, so WR-12 cannot grade itself); each faulty case differs from its healthy twin only in that fault's inputs; planned new symbol `wiringFaultCorpus`. [H2] {id: WR-3, symbol: wiringFaultCorpus, package: cmd/fathom}

L3 Wiring stamp (S1, S2, S6)
- [ ] S1 One `wire` per session computed once at SessionStart (hash of `fathom_version`, `fusion.ExtractorEpoch`, digest of fathom-owned hook entries in effective settings, MCP surface) plus `idx` (fresh, behind or unknown) from the identity call SessionStart already makes, written to the sessions.jsonl row; planned new symbol `computeWireStamp`. {id: WR-4, symbol: computeWireStamp, package: cmd/fathom, ctx: recordSessionWindow cmd/fathom/discoveries.go cmdSessionHook cmd/fathom/sessionhook.go}
- [ ] S2 `stampBeatMeta` adds `wire` read from the session stamp; beats written before the stamp keep reading wiring-unknown (extends the v1 version-unknown rule, never backfilled). Bar: extend `beats_test.go`, every beat after SessionStart carries `wire`. Verify first whether beats already carry a session key to join on; if so carry only that key. {id: WR-5, symbol: stampBeatMeta, package: cmd/fathom, needs: WR-4}
- [ ] Skip beats carry readiness inputs in meta (rule count, `uncompiled`, `instructions_known`) so "not-ready:no-instructions" can be audited later. {id: WR-6, symbol: channelReady, package: cmd/fathom, ctx: cmd/fathom/hookreadiness.go}
- [ ] S6 Each doctor run appends one `doctor` channel beat of kind `snapshot` holding the row verdicts (history without a new store); planned new symbol `appendDoctorSnapshotBeat`. {id: WR-7, symbol: appendDoctorSnapshotBeat, package: cmd/fathom, needs: WR-5}
- [ ] The `hook_budget` doctor row stays within budget with the stamp in place. {id: WR-8, symbol: checkHookBudgetFromFiles, package: cmd/fathom, needs: "WR-5, WR-6", ctx: cmd/fathom/doctor_retention.go}

L4 Readers split by stamp era
- [ ] `adoptionCallEntry` decodes `fathom_version` and `schema_version`; rows without stamps are labelled wiring-unknown; planned new symbol `adoptionEraRows`. Bar: a fixture log written by two versions yields two era rows and totals still add up. The split by `wire` itself follows WR-5. {id: WR-9, symbol: adoptionEraRows, package: cmd/fathom, ctx: adoptionCallEntry cmd/fathom/instrument_adoption.go}

L5 Bench bootstrap corpus
- [-] About 6 pinned public repos (tiny under 5k LOC, one Go CLI, one Python, one Node, one Rust, one Java, plus one monorepo-scale on cloud only), each with a manifest: commit pin, `fathom init` and ingest receipt, every `fathom doctor` row, index commit equals pin, guards and layer packs activated with each guard shown firing once. Bar: re-bootstrap from the manifest gives the same doctor verdicts and the same symbol and edge counts; planned new symbol `LoadCorpusManifest`. {id: WR-10, disposition: deferred, reason: "owner decision D5: approve the repo picks and a size ceiling", trigger: "owner approves the L5 repo list in this plan or autonomy.md"}

L6 Oracle truth and free readings on L5
- [-] Task truth derived with loctruth, publicloc, SCIP and go/types; indexbench L0, L1 and freshness plus reviewbench run on the L5 repos, oracle versions pinned in the reading. Bar: truth regenerates byte-identical. Store-direct viability of indexbench and reviewbench on helper lanes is unverified. {id: WR-11, disposition: deferred, reason: "blocked by WR-10, which waits on owner decision D5", trigger: "WR-10 lands"}

L7 Deterministic wiring_firing doctor row
- [ ] One `wiring_firing` doctor row per recent session joining skip beats, beats, the call log and `rule_outcome_writer`, giving a reason code. No prose, no store, writes no file. Bar: reason code matches the label on 100% of WR-3 seeded faults and healthy fixtures PASS; changelog fragment in the same change. Opus reviews (doctor has known false signals). {id: WR-12, symbol: checkWiringFiring, package: cmd/fathom, needs: "WR-3, WR-5"}

L11 Bench-ready kit, dry run only
- [-] Registrations for the L5 corpus plus `scenariobench plan` dry cells with spend estimates (arms: hooks on vs off, guards on a repo other than fathom, receipt-gated), under `docs/bench/` and `testdata/scenariobench/`. Bar: the plan emits cells and an estimate at $0, registration committed before any data exists (H3). Running it is WR-16. {id: WR-13, disposition: deferred, reason: "blocked by WR-1, WR-10 and WR-11", trigger: "WR-1, WR-10 and WR-11 are claimed and verified", needs: "WR-1, WR-10, WR-11"}

Held work
- [-] W1 to W4 (queryable doctor history, test_runs `wire`/`idx`, config digest, periodic sampler). {id: WR-14, disposition: deferred, reason: "table freeze: waits on RT-M1 PASS (window 2026-10-20 to 2026-10-27)", trigger: "docs/vision/38-proof-layer-roadmap.md reads RT-M1 PASS"}
- [-] Daemon-side call-log fields carry `wire`/`fathom_version` per call. {id: WR-15, disposition: deferred, reason: "daemon-written, so inside the freeze until RT-M1 PASS", trigger: "RT-M1 reads PASS"}
- [-] First paid run: L11's receipt-gated corpus run with guards on a repo other than fathom (D7). {id: WR-16, disposition: deferred, reason: "spend hold in docs/intent/autonomy.md", trigger: "owner lifts the spend hold and approves the dry plan estimate"}
- [-] `judge_wiring` decision-model site (see Decision models). {id: WR-17, disposition: deferred, reason: "no-go until the WR-3 corpus and WR-12 row exist; OpenAI spend held and EXP-M4 CLOSED", trigger: "WR-3 and WR-12 verified and a calibrated band declared"}

## Decision models

Typed question: `judge_wiring`, asked through the hexagonal decision port on sessions the deterministic WR-12 row marks unknown or contradictory (the residual only), 1 session in N from an opt-in doctor flag. No timer or sampler (a periodic job is frozen, W4) and hooks never ask the port.
Deterministic fallback with provider `none`: the WR-12 `wiring_firing` doctor row's reason code, which is always computed.
Opt-out: the doctor flag is off by default; provider `none` or `[decision] off` disables it; the answer never gates, prunes or asserts a code fact.
Measurement that keeps or kills it: agreement with 100 hand-labelled real sessions on the WR-12-unknown residual of at least 0.8, and it resolves more residual cases than WR-12 does; below that it is killed. Prior evidence is negative (failure-kind agreement 0.629; route confidence right 45.7% at 0.9 or higher).
Status: no-go until the WR-3 corpus and WR-12 row exist; spend is held, and this plan authorizes none.
