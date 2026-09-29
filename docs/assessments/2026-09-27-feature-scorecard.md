# The feature scorecard, 2026-09-27

Created: 2026-09-27 (summarized into the repository 2026-09-28).
Status: a hand-built reading, scored and verdicted by the owner on 2026-09-27 from fathom's own logs,
ledgers, benches and plans; not re-run. It is the oracle for the feature map's first reading (FM-M1)
and the calibration label set for a model verdict proposal (F51, FM-M2, deferred). The machine-readable
form is [`data/2026-09-27-feature-scorecard.json`](data/2026-09-27-feature-scorecard.json) (86 rows).
Decisions it drove: ADR-053 (the surface reckoning), ADR-054 (the support matrix), ADR-055 decision 6.

## Verdict

Fathom's strongest feature is the one only its index can answer: **done means proven** (verify scores
5 of 5, with 1,325 plan rows proven). Its resolved call graph, receipts, refactor conservation and its
honest bench are next. Its weakest ground is governance that does not bite (the rule reaches the agent
and agents break it anyway), payload (its arm still returns 0.86 to 2.66 times grep's bytes on
lookups), accurate impact tools that go unused, call-edge recall beyond Go (0.03 for TypeScript, 0.12
for Python), and tenancy (the daemon was paused after a 20.3 GiB wasm spike). Of 86 features, 30 are
keepers, 24 need improving, 12 fold into others, 10 are ditched and 10 are new on the roadmap. The
verdicts were carried out as ADR-053: the registry went from 59 tools to 25 and the default agent
surface from 25 to 8, and every removal is proven absent.

## Sources (the scorecard's own list; windows as stated)

| Source | Size and window |
|---|---|
| MCP call log | 97,898 calls, 2026-07-07 to 2026-09-27 |
| Hook logs | 286,758 hook events, 2026-09-14 to 2026-09-28 |
| Verify, thread, session, console-view and decision ledgers | 17,125 verify readings; 1,054 thread rows; 3,359 token-counted sessions; 87 console views; 120 decision-log rows |
| The fleet store, read-only | claims, claim events, typed edges, certify tables |
| Repository documents | the bench README and readings, `docs/mcp-tools.md`, `fathom help`, `docs/intent/scope.md`, the plans (1,325 done rows and 64 open), `.fathom/observations.md` |

All counts above are *measured* over the windows stated. Scores are the owner's judgment: implementation and
value each run 1 to 5 (an *estimate*, never a computed figure); the verdict is the owner's call.

## Tally

| Verdict | Count | Roadmap mapping |
|---|---|---|
| KEEP | 30 | accepted; some carry a "better" line |
| IMPROVE | 24 | accepted; each better line became a testable row (SR-40 to SR-58) |
| MERGE | 12 | accepted; folded into another tool (SR-21 to SR-32) |
| DITCH | 10 | refused (deleted or frozen with reasons; SR-10 to SR-20) |
| NEW-ON-ROADMAP | 10 | accepted; a capability the roadmap adds (F-59, F-76, F-79 to F-86) |

By group: code navigation 11, impact 11, context packing 3, governance 10, proof 9, memory and
continuity 4, intent system 3, observability 5, fleet 4, tenancy 3, semantic layers 5, decision port 3,
bench 1, substrate 6, roadmap 8.

## Strengths

1. **Verify and done-means-proven (F-36)**: 5 of 5; the one question only the index answers; 1,325
   proven rows. Keep and prove on a public corpus (milestone M2).
2. **Resolved call graph**: `find_callers` and `blast_radius` at precision 1.00 against `go/types`.
3. **Receipts**: 50 of 50 re-derived by a stranger's tools; 93 percent of answers trusted without a
   re-check.
4. **Refactor conservation**: six calibration readings pass, including precision 0.960 and recall
   1.000 on API deltas.
5. **An honest bench**: pre-registered bars, nulls published, voided runs named.

## Weaknesses

1. **Payload**: fathom's arm still returns 0.86 to 2.66 times grep's bytes on lookups; H2-3 never passed.
2. **Governance does not bite yet**: the rule reaches the agent, agents break it anyway (the M1 bar
   fails; see `2026-09-28-semantic-layers-evidence.md`).
3. **Accurate impact tools go unused**: `blast_radius` 9 calls, rename and safe-delete 2 each.
4. **Beyond Go, call-edge recall is 0.03 (TypeScript) and 0.12 (Python)**: the reason for the language
   program (`2026-09-29-language-atlas.md`).
5. **Tenancy**: the daemon was paused on the reading date after a 20.3 GiB wasm spike (INC-23,
   ADR-047).

## Underused: capabilities that exist and do little, with the fix the owner chose

| Capability | Why it is underused | The fix | Where it landed |
|---|---|---|---|
| `blast_radius` and the impact verbs | Precision 1.00, 0.006 s per query, 9 lifetime calls | Put their output inside `review_diff` and the pre-edit hook so the agent never has to ask | SR-60 |
| Semantic layers | 4,025 claims for $27.50, but 0 human verdicts and no per-rule serve or violation count | Show served, ignored and violated per rule; ask for a one-key verdict when an ignored rule is served | SR-61; GL-10 to GL-18 |
| Certifications | Stage 0 join rate 94.8 percent, yet 0 declarations in the live store | Declare certificates on fathom itself and bind liveness to the commit (O114) | SR-62 |
| Refactor conservation | Six passing calibrations; used only on dogfood ranges | Run conserve in the pre-push gate by default and show its planes in review | SR-63 |
| Findings loop | Tracer passed; 0 findings in the live store | Arm Review Kit publishing on one real repository | SR-64 |
| SCIP import | Mapping fixed against real clones; 0 SCIP-tier edges live | Have `init` run `scip-typescript` or `scip-python` when installed | SR-65 |
| ui-appkit pack | Shipped as a pack; never compiled on a consumer | Compile on app-kit consumers and measure UI rule violations | SR-66 |
| The agent activity record (call log, hook log) | 97,898 calls and 286,758 hook events feed adoption gauges nobody reads | Raw material for cost per proven row, debt interest, the friction map, rule effectiveness and the model scorecard: build those views over them | SR-67 |
| Verify ledger and session ledger | 17,125 readings and 3,359 sessions already on disk | The data for burn-up and cost per proven row: render them before building new collectors | SR-68 |

## Surprises

1. `package_deps` was already deleted by the surface prune while the roadmap memo still listed it
   (SR-11).
2. The doctor probed `list_modules`, which no longer exists: 338 errors in the call log (SR-2).
3. `spec_refs` failed 170 of 306 edit-hook calls on the old `sextant` service name (SR-1).
4. **All 70 guard denials and claim-moment refusals happened inside bench subjects; none in real
   work.** The governance path is proven to fire, not proven to help. (A later cut of the same
   record, in `docs/design/served-not-kept.md`, counts 8 denials in 15 days, all in the bench.)
5. The prompt hook injected 26 times in real work while 955 of its injections were bench (SR-46).
6. `semantic_claims` already had `valid_from` and `valid_to` columns: the bi-temporal schema was half
   built (SU-19; vision 38 section 10.3).
7. **0 anchored memories exist in any repository** (F-45; ADR-053 decision 2).

## Verdicts refined by evidence during the surface reckoning

`docs/design/surface-reckoning.md` section 8 corrects eight rows against the tree at the time:
F-53's `brief` covers a CLI verb and an MCP tool (the console's brief page stays), `research` is frozen
not deleted (ADR-046 decision 6), F-68 removes the module tools and not the module system, F-45's
retire list over-reached (the runtime memory limit is a different thing), F-68's hosted mode takes two
more rows, F-77 includes `batch`, F-22's `reachable` has no internal consumer, and the agent surface
is 25 tools, not 19. In the table below the scorecard's own verdict stands; the design records each
refinement.

## The oracle and the label set

The JSON is fathom's only hand-built reading of its own features, and two decisions depend on it.
**FM-M1** (`docs/plans/feature-map.md`) re-derives the 86-feature scorecard with the feature map and
requires the evidence columns to match this file's on at least 90 percent of cells: without the file in
the repository that measurement has nothing to compare against. **F51 and FM-M2** (a model proposing
keep, improve, merge or ditch, deferred by the owner on 2026-09-29) calibrate against the 86 owner
verdicts as labels, at agreement of at least 0.8 with n of at least 3; the critics estimate 86 labels
over four classes cannot support that bar, which is why the path waits.

## All 86 rows

`sr` in the last column is `docs/design/surface-reckoning.md` (section numbers) and, for an `SR-nn`,
the rows of `docs/plans/surface-reckoning.md`; ADR-053 carries the decision. Impl and value are the
owner's 1 to 5 scores. A "roadmap" tag is the roadmap note the scorecard attached (N is a note of the
2026-09-27 memo, T and U are the testing and work-intelligence joins of ADR-051).

### Code navigation (11)

| Id | Feature | Impl / value | Verdict | Usage and evidence | The better line | Landed |
|---|---|---|---|---|---|---|
| F-01 | find_symbol | 5 / 4 | KEEP | 3,772 MCP calls (3,695 from agents); the most-used agent tool. Gold source for 25% of bug-fix and 19% of localization answers in scenario runs; a 492-byte hit answered H1-T06 outright. | Return the implementations and the one-line answer first so the agent stops after the hit instead of chaining references/repo_map. | sr §7 KEEP; ADR-053 |
| F-02 | references | 4 / 3 | IMPROVE | 883 calls, 140 errors (free-text queries refused, unreadable stores). L0 oracle: text mode precision 0.466, exact mode 1.00 / recall 0.997 (SB-45). Gopls 1.00/0.96, Serena 1.00/0.79. | Default to exact mode; route free-text queries to search automatically instead of refusing them. | sr §5 SR-40; ADR-053 |
| F-03 | symbol_context | 4 / 4 | KEEP | 1,271 calls, 53 errors (25 over-budget refusals, 20 missing args). Unbounded answers for hub symbols fixed (O90); now budgeted. | Serve a degraded partial answer instead of an over-budget refusal. | sr §7 KEEP; ADR-053 |
| F-04 | find_callers | 5 / 4 | KEEP | 324 calls. L0 oracle vs go/types: precision 1.000, recall 1.000 (was 0.607/0.899). Competitor ledger: graphify 0.95/0.93. | Extend the same resolved tier to TS/Python (file-resolved recall is 0.03/0.12 today). (roadmap: N5, T1) | sr §7 KEEP; ADR-053 |
| F-05 | find_callees | 3 / 1 | MERGE | 2 lifetime calls (both July). symbol_context already returns callees. | Fold into symbol_context and remove from the registry (law 4). | sr §3 SR-21; ADR-053 |
| F-06 | outline | 4 / 3 | KEEP | 1,359 calls. H1-T08: every cell spent 5.9 KB outlining docs/usage.md, a doc file, past the answer. | Budget doc-file outlines to headings only; point doc questions at doc search (N7). (roadmap: N7) | sr §7 KEEP; ADR-053 |
| F-07 | repo_map | 3 / 2 | IMPROVE | 201 calls. H1-T06: one cell spent 12.2 KB on repo_map after find_symbol had already answered. | Cut the default budget and mark it orientation-only; or merge into pack_context. | sr §5 SR-41; ADR-053 |
| F-08 | read_source | 4 / 4 | KEEP | 2,460 calls, 137 errors (ranges over the 200-line cap). Added on evidence: H3-1 cost 16 turns / 601k tokens without it. Largest per-tool payload (38 calls, 123 KB in one run); BP-42 fixed envelope over-count. | Auto-clamp an over-wide range with a note instead of refusing it. | sr §7 KEEP; ADR-053 |
| F-09 | search | 4 / 4 | KEEP | 857 calls since 2026-09-14. Gold source for 19% of concern-search and 13% of localization answers; added with a pre-registered kill criterion. | Read its kill criterion on the next lookups run and publish the verdict. (roadmap: N7) | sr §7 KEEP; ADR-053 |
| F-10 | entrypoints | 3 / 2 | MERGE | 5 lifetime calls. No bench reading. | Fold into repo_map's header; entrypoints become nodes in the system graph. (roadmap: N6, U14 (deferred)) | sr §3 SR-22; ADR-053 |
| F-11 | list_services / capabilities / server_stats / list_sources | 4 / 2 | MERGE | list_services 37,155 (most from hooks and verify CLI), capabilities 86, server_stats 288, list_sources 400. Mostly internal plumbing called by hooks and doctor, not by agents. | Keep list_services; fold capabilities, server_stats and list_sources into one status tool to shrink the surface. (roadmap: G16) | sr §3 SR-23; ADR-053 |

### Impact (11)

| Id | Feature | Impl / value | Verdict | Usage and evidence | The better line | Landed |
|---|---|---|---|---|---|---|
| F-12 | blast_radius | 5 / 4 | IMPROVE | 9 MCP calls (4 errors: missing changed_files). L0 oracle: precision 1.000, recall 0.983, 0.006 s/query (was 0.086 precision, 10.9 s). Cross-process on the blind route table P 1.00 / R 1.00. | Accurate and fast but unused: feed it through review_diff and the edit hook, add the co-change oracle. (roadmap: N6, T1, U7) | sr §6 SR-60; ADR-053 |
| F-13 | rename_impact + safe_delete_check | 4 / 2 | MERGE | 2 calls each, lifetime. No bench reading; the refactor plan verb covers the same need with a ratchet. | One impact verb with kind=rename\|delete, or fold into refactor plan; U9's migration tracker counts the same remaining sites. (roadmap: U9) | sr §3 SR-24; ADR-053 |
| F-14 | test_impact | 4 / 3 | KEEP | 103 calls. No direct reading; shares the reach engine that tests select uses. | Rank by coverage evidence when a profile is present (CV-M13 found 3.7% reach slack). (roadmap: T4) | sr §7 KEEP; ADR-053 |
| F-15 | tests select | 3 / 2 | IMPROVE | CLI only, no usage log. Replay of 105 commits: recall 0.28 vs a 0.98 bar, time 26.6% vs 25%. Stays advisory. | Rebuild as certifying-first selection (T4): the minimal set that re-proves every touched certificate, scored against a coverage oracle. (roadmap: T4) | sr §5 SR-50; ADR-053 |
| F-16 | diff_impact | 3 / 1 | MERGE | 0 lifetime calls. review_diff composes the same answer. | Remove; review_diff is the verb. | sr §3 SR-25; ADR-053 |
| F-17 | cross_repo_blast_radius | 2 / 2 | DITCH | 0 lifetime calls. Named in vision 15 as unused; service_ref recall 0.20 on real code. | Delete now; rebuild as system_impact over the system graph's provides/consumes links. (roadmap: N6, U7) | sr §2 SR-10; ADR-053 |
| F-18 | dep_drift | 2 / 1 | DITCH | 0 lifetime calls. No reading. | Delete; upgrade impact over a dependency API index replaces it. (roadmap: N1, T6) | sr §2 SR-10; ADR-053 |
| F-19 | package_deps | 1 / 1 | DITCH | 0 calls; already deleted in the surface prune (34 zero-call tools). Listed in surface_prune_test as removed. The roadmap memo still counts it as existing. | Already gone; correct the memo and vision 15. (roadmap: N1) | sr §2 SR-11; ADR-053 |
| F-20 | api_diff (MCP tool) | 3 / 2 | MERGE | 0 lifetime calls. The api: delta inside conserve is calibrated (CV-M4: P 0.960, R 1.000 over 50 release pairs); the standalone tool is not used. | Serve the calibrated delta through conserve and upgrade impact only; drop the tool. (roadmap: N1, T6) | sr §3 SR-26; ADR-053 |
| F-21 | Cross-repo callers (service_ref, platform-callers) | 3 / 4 | IMPROVE | CLI only. VB-31: precision 1.00, recall 0.20 on real corpora (regex route detector, literal URLs only). | Replace the regex detectors with framework route extractors and contract files. (roadmap: N6, T5, U10) | sr §5 SR-51; ADR-053 |
| F-22 | http_client_calls / fleet_drift / reachable / traverse / history | 2 / 1 | DITCH | 0 lifetime calls each. Some are used internally by verify; none by an agent. | Remove from the registry; keep the internal functions that verify needs. (roadmap: G16) | sr §2 SR-12; ADR-053 |

### Context packing (3)

| Id | Feature | Impl / value | Verdict | Usage and evidence | The better line | Landed |
|---|---|---|---|---|---|---|
| F-23 | pack_context | 3 / 3 | IMPROVE | 202 calls. BP-46 (pack_context vs bare tools) never run; no bench cell called it. | Run BP-46; if it does not beat bare tools on bytes, merge into query_symbols. | sr §5 SR-42; ADR-053 |
| F-24 | query_symbols | 3 / 3 | IMPROVE | 2,111 calls (633 from the session hook, 135 from prime). H1-T06: 6.6 KB responses spent past the answer in two of three cells. | Shrink default top-N and add an answer-first summary line. | sr §5 SR-43; ADR-053 |
| F-25 | localize | 3 / 3 | IMPROVE | 31 calls. LCA bug localization: 94 vs 93 of 117 at 2x tokens (null); the model never called it (O102). Jev routing lifted its gold-source rate 4.6% to 23.4%. | Route issue-shaped prompts to it deterministically in the prompt hook, then re-run LCA. | sr §5 SR-44; ADR-053 |

### Governance (10)

| Id | Feature | Impl / value | Verdict | Usage and evidence | The better line | Landed |
|---|---|---|---|---|---|---|
| F-26 | obligations (tool) | 4 / 3 | IMPROVE | 26,118 calls (22,560 from the edit hook, 1,182 from agents). Guards pilot 8: rule reached the agent before the edit on 4 of 8 tasks; violations tie control 4 vs 4; clean successes trail 7/11 vs 10/14 at 1.56x tokens. M1 fails. | Serving the rule is solved; make it bite: turn checkable rules into enforce:true guards and lint output. (roadmap: N8, U3, U4) | sr §5 SR-45; ADR-053 |
| F-27 | Rules at the edit (edit hook) | 4 / 3 | KEEP | 110k real edit events in 14 days; 7,605 injections (6.9%). Fires reliably with dedup; payload capped. Value measured as above (M1 not yet a win). | Record served / complied / violated per rule (U4) so ignored rules are rewritten and silent ones pruned. (roadmap: N8, U4) | sr §7 KEEP; §6 SR-61; ADR-053 |
| F-28 | Prompt hook | 3 / 2 | IMPROVE | 1,196 events, 1,121 of them bench; 26 real injections. No real-work usage to speak of. | Wire it in `fathom init` by default and measure first-edit violations with and without it. | sr §5 SR-46; ADR-053 |
| F-29 | Guards (enforce: true) | 4 / 3 | IMPROVE | 8 denials in 14 days, all inside bench subjects; 4 enforced rules in this repo. ADR-038 shipped; no real-work denial yet. | Ship more mechanically checkable rules per pack; emit SARIF so reviewers see the same verdicts. (roadmap: N8, U4, U7) | sr §5 SR-47; ADR-053 |
| F-30 | Layer compiler (compile / compile_layer) | 4 / 3 | KEEP | 82 compile runs, $27.50 lifetime; 4,025 claims across 15 services. Packs pass precision 1.00 / recall 1.00 on small labeled evals; fleet recompile 2,798 claims for $7.05 (CV-M8). | Publish per-claim serve counts so compiled claims earn their cost. (roadmap: N8, U4) | sr §7 KEEP; ADR-053 |
| F-31 | Claim triage (refute / corroborate) | 4 / 2 | IMPROVE | 0 human verdicts ever; 1,843 automated sweep events. No signal exists for rules that learn. | Ask for a one-key verdict at the moment a rule is served and ignored; that is the signal N8.1 needs. (roadmap: N8, U4) | sr §5 SR-48; ADR-053 |
| F-32 | Rules to tests (rules gen-test / replay) | 4 / 2 | KEEP | CLI only. Rule replay over 200 commits caught nothing (VB-16): governance stays advisory. | Replay on repos with real violations (other fleet repos) before judging. | sr §7 KEEP; ADR-053 |
| F-33 | Findings loop (findings import, findings_near) | 4 / 2 | IMPROVE | 0 external findings in the live store; findings_near 0 calls. Tracer passed end to end (2026-09-18); findings-loop runs r1-r3 lost to instrument faults (O103-O107). | Arm Review Kit publishing on one real repo so findings actually flow. (roadmap: N8) | sr §6 SR-64; ADR-053 |
| F-34 | Review verb (review CLI, review_diff) | 4 / 3 | IMPROVE | review 303 CLI calls, review_diff 63 MCP calls. 42 historical PR pairs: recall 14.3% vs a 30% bar, zero false findings. | Become the change risk receipt (U7): blast radius, certificates, conservation, rules and reviewers, as SARIF/GitHub check. (roadmap: N8, U7) | sr §5 SR-49; ADR-053 |
| F-35 | arch_conformance / spec_coverage / spec_refs | 3 / 2 | MERGE | 0 direct MCP calls; spec_refs 306 edit-hook calls, 170 failing on the old 'sextant' service name. Used inside review; spec_refs is silently broken for one repo. | Keep as internals of review and the architecture drift radar (U11); fix the spec_refs service resolution bug. (roadmap: U11) | sr §3 SR-27; ADR-053 |

### Proof (9)

| Id | Feature | Impl / value | Verdict | Usage and evidence | The better line | Landed |
|---|---|---|---|---|---|---|
| F-36 | verify / verify_plan (done means proven) | 5 / 5 | KEEP | 16,890 MCP calls; 17,125 ledger readings; wired into stop hook and pre-push. 1,325 plan rows proven here. The verify-reachability task is the one question only the index answers (every grep arm failed it). False-done reduction (M2) not yet measured. | Measure M2 on a public corpus with plan-grammar rows so the core claim has a number. (roadmap: N2, N4, U12) | sr §7 KEEP; ADR-053 |
| F-37 | Plan grammar v2 | 4 / 5 | KEEP | Used by 65 plan files here. The only spec format with anchors; foreign formats are prose. | Add weight:, accept: (EARS) and tests: fields; read Spec Kit/OpenSpec in place. (roadmap: N2, N4) | sr §7 KEEP (tests: superseded by certified_by:, ADR-049 d6); ADR-053 |
| F-38 | plans status / next / history / leases | 4 / 4 | KEEP | CLI; no call log. Feeds G4/G5/G13 goals as the instrument. | Burn-up and proof regressions from the existing verify ledger. (roadmap: N2, U12) | sr §7 KEEP; ADR-053 |
| F-39 | Claim moment (stop hook) | 4 / 4 | KEEP | 145 checks, 62 refusals, all in bench subjects. Catches false done in bench runs; no real-work refusal recorded yet. | Report the refusal rate per agent as claim honesty. (roadmap: N2, U5) | sr §7 KEEP; ADR-053 |
| F-40 | Receipts (receipt verify) | 5 / 4 | KEEP | CLI; receipts on every answer. 50 of 50 re-derived; envelope 7.6% of median payload; answers trusted without re-check 93%. | Extend re-derivation beyond Go. | sr §7 KEEP; ADR-053 |
| F-41 | Certifications (certify status/gaps/suggest/orphans) | 3 / 2 | IMPROVE | 0 certify declarations in the live store; 120 decision-port suggestions. Join rate 94.8% (stage 0 passes); CV-44 read a false ok when the service name was guessed; O114: a certificate stays live through any code change. | Bind liveness to the commit, fix service resolution, and declare certificates on one real repo. (roadmap: N4, T1, T2, T5, T7) | sr §6 SR-62; ADR-053 |
| F-42 | Refactor conservation (conserve, refactor plan/status) | 4 / 4 | KEEP | CLI; dogfood only. CV-M1, M2 (0% false flags), M4 (P 0.960 / R 1.000), M5, M6, M14 all pass. | Run it in the pre-push gate on every range; show its plane verdicts in review. (roadmap: U9, U12, U7) | sr §7 KEEP; §6 SR-63; ADR-053 |
| F-43 | Execution evidence (tests coverage / tests ingest) | 4 / 3 | KEEP | CLI. CV-M14 passes; CV-M13: static reach says 'attested' for 168 functions that never ran (3.7%). | Ingest lcov/Cobertura/JaCoCo/coverage.py/Istanbul (T3) so coverage exists beyond Go. (roadmap: N4, T1, T3) | sr §7 KEEP; ADR-053 |
| F-44 | CUE shadows (gen-status-cue, gen-plan-cue) | 3 / 1 | MERGE | CLI; no usage record. Plumbing under ADR-018. | Hide as internal; drop from the top-level help. (roadmap: G16) | sr §3 SR-30; ADR-053 |

### Memory & continuity (4)

| Id | Feature | Impl / value | Verdict | Usage and evidence | The better line | Landed |
|---|---|---|---|---|---|---|
| F-45 | Anchored memory (memory, memory_search) | 3 / 1 | DITCH | 0 memories in any repo; memory_search 2 calls. Excluded from the bench by design; no reading. | Remove; continuity threads already carry the hand-off. Revisit only with demand. | sr §2 SR-13; §8 refined; ADR-053 |
| F-46 | Continuity threads (thread note/point/handoff) | 4 / 3 | IMPROVE | 1,054 entries (431 hand-off notes, 530 pre-compact). G3 bet at 4 of 20 units; M4 (resume without re-discovery) refuted in a run later voided (O41). | Reach the 20-unit sample and let the flywheel keep or kill it. (roadmap: U3) | sr §5 SR-52; ADR-053 |
| F-47 | discoveries / routes | 3 / 1 | MERGE | discoveries 13 calls; routes CLI only. Kill criteria still undecided (G11). | Fold into the session brief; the discovery-not-reused signal becomes an input to the friction map (U3). (roadmap: U3) | sr §3 SR-29; ADR-053 |
| F-48 | Session brief (session hook, prime) | 4 / 3 | KEEP | 1,446 session hooks fired. Payload cost tracked per brief; token effect unmeasured. | Measure first-edit tokens with and without it (G2 A/B). | sr §7 KEEP; ADR-053 |

### Intent system (3)

| Id | Feature | Impl / value | Verdict | Usage and evidence | The better line | Landed |
|---|---|---|---|---|---|---|
| F-49 | Intent audit / doctrine / render | 4 / 3 | KEEP | 49 audits, 43 doctrine runs. ADR-027 caught an audit that silently read 0 of 37 invariants. K0 demand signal (3 annotations) unmet. | Evaluate computable revisit triggers by machine and list decisions due for review (U11). (roadmap: U11) | sr §7 KEEP; ADR-053 |
| F-50 | Config custodian (config audit / propose) | 4 / 3 | KEEP | 209 audits, 110 attention lines. VB-29 code-fact classifier precision 0.649 (bar 0.8); anchored-fresh alone 0.96. | Serve only the anchored-fresh class until the other classes pass. (roadmap: N8) | sr §7 KEEP; ADR-053 |
| F-51 | doctor | 3 / 4 | IMPROVE | 24 logged runs. It calls list_modules, a tool that no longer exists: 338 errors in the call log. | Fix the list_modules probe; add a timed stranger check (G16). (roadmap: G16) | sr §5 SR-53; ADR-053 |

### Observability (5)

| Id | Feature | Impl / value | Verdict | Usage and evidence | The better line | Landed |
|---|---|---|---|---|---|---|
| F-52 | Impact metrics (metrics, instrument, flywheel, retro, rate, arm) | 3 / 2 | MERGE | CLI; 3,359 session-ledger rows. G2: 0 admissible A/B units; every savings claim stays inadmissible. | Collapse the verbs into one metrics verb; the call log and hook log they read become the data for U1-U5 views. (roadmap: N3, U1, U2, U3, U5) | sr §3 SR-31; ADR-053 |
| F-53 | narrate / brief / research | 2 / 1 | DITCH | No usage record. No reading. | Remove; the console and bench readings cover the need. (roadmap: G16) | sr §2 SR-14..16; §8 (research frozen, not deleted); ADR-053 |
| F-54 | Observatory console | 4 / 3 | IMPROVE | 87 page views since 2026-09-25 (34 statusz). UX-M1 fails narrowly: 9 of 10 tasks in 3 clicks or fewer; all 10 answers equal their verb's JSON. | Add the progress and economics views; fix the plan-row list click depth. (roadmap: N2, N3, U1-U5, U10) | sr §5 SR-54; ADR-053 |
| F-55 | staleness_report | 4 / 3 | KEEP | 298 calls. Freshness left positioning: graphify watch answered 4 of 5 post-edit questions (VB-18). | Add doc drift: flag doc spans whose named symbol changed. (roadmap: N7) | sr §7 KEEP; ADR-053 |
| F-56 | stale_docs | 2 / 2 | MERGE | 0 lifetime calls. No reading. | Fold into staleness_report's doc drift. (roadmap: N7) | sr §3 SR-28; ADR-053 |

### Fleet (4)

| Id | Feature | Impl / value | Verdict | Usage and evidence | The better line | Landed |
|---|---|---|---|---|---|---|
| F-57 | Fleet report / reconcile / receipts / undo | 3 / 2 | IMPROVE | 3 fleet report snapshots, last 2026-08-14. Every write receipted and reversible; not run for six weeks. | Run reconcile from the console on a schedule the user triggers; show drift on the fleet page. (roadmap: U10) | sr §5 SR-55; ADR-053 |
| F-58 | init / onboard / plugin render | 3 / 4 | IMPROVE | CLI. v0.7.0: a fresh clone did not build and adopters were told one machine's launchd label; fixed. RT-182/183 onboarding bugs fixed today. | Timed stranger check to first proven answer in 5 minutes; publish the plugin. (roadmap: G16) | sr §5 SR-56; ADR-053 |
| F-59 | Harness port (Claude Code, Codex, Gemini, Cursor rows) | 3 / 4 | NEW-ON-ROADMAP | Claude Code and Codex verified; others 'unverified on this machine'. Byte-identical Claude Code golden. | Verify Cursor, Copilot and Gemini hooks live; read every client's rules files. (roadmap: N8) | sr §7 note (vision 38; support matrix client kind); ADR-053 |
| F-60 | Skills pack (fathom-* skills) | 4 / 3 | KEEP | Installed; no invocation count. Rendered for Claude Code and the .agents/skills path. | Log skill invocations so the call log can judge them. | sr §7 KEEP; ADR-053 |

### Tenancy (3)

| Id | Feature | Impl / value | Verdict | Usage and evidence | The better line | Landed |
|---|---|---|---|---|---|---|
| F-61 | Responsible tenant (budget, admission, resources, daemon controls) | 3 / 5 | IMPROVE | resources/daemon CLI; 40+ named over-budget refusals in the call log. Waves C-F: footprint under 0.8 GiB on the real fleet. But the daemon is paused today after tree-sitter wasm growth hit 20.3 GiB. | Ship RT-99 (no inline ingest) and RT-107 (wasm memory cap); read RT-M1's seven days. (roadmap: G14) | sr §5 SR-57; ADR-053 |
| F-62 | scope suppress / purge | 4 / 2 | KEEP | CLI. No reading. | Surface suppressions on the console system page. | sr §7 KEEP; ADR-053 |
| F-63 | Ingest and refresh (ingest, watch, post-commit hook) | 4 / 5 | KEEP | 1,996 ingest calls, 50 errors (40 over-budget). An ingest that read no file is never recorded complete (F2 fix today). | Incremental equals full equivalence check before live updates. | sr §7 KEEP; ADR-053 |

### Semantic layers (5)

| Id | Feature | Impl / value | Verdict | Usage and evidence | The better line | Landed |
|---|---|---|---|---|---|---|
| F-64 | obligations pack | 4 / 3 | KEEP | 634 fresh claims. ADR-016 baseline; stale claims once rendered as binding (O88, fixed). | Retire rules never served in 90 days. (roadmap: N8) | sr §7 KEEP; ADR-053 |
| F-65 | adr-rules pack | 4 / 3 | KEEP | 852 fresh claims. Precision 1.00 / recall 1.00 on 7 rules + 2 traps. | Bi-temporal validity so a superseded ADR's rule has an end date. (roadmap: N8) | sr §7 KEEP; ADR-053 |
| F-66 | spec-bindings pack | 4 / 3 | KEEP | 1,612 fresh claims. Precision 1.00 / recall 1.00 on 6 rules + 2 traps. | Feed foreign spec formats through the same pack. (roadmap: N4) | sr §7 KEEP; ADR-053 |
| F-67 | ui-appkit pack | 3 / 2 | IMPROVE | 0 compiled claims in the fleet store. Pack exists in .fathom/layers; never compiled on a consumer. | Compile it on app-kit and one consumer, then measure UI rule violations. (roadmap: N8) | sr §6 SR-66; ADR-053 |
| F-68 | Expert modules (module verbs, module_query/obligations/staleness) | 3 / 1 | DITCH | 0 MCP calls to module tools. Plans proven (expert-suite J, O-R) but no consumer. | Remove the module MCP tools; keep the pack format that layers already use. (roadmap: G16) | sr §2 SR-17; §8 refined; ADR-053 |

### Decision port (3)

| Id | Feature | Impl / value | Verdict | Usage and evidence | The better line | Landed |
|---|---|---|---|---|---|---|
| F-69 | Decision port (Jev / llm adapters) | 3 / 2 | IMPROVE | 120 rulings, all certify suggest; off by default. Route site fails calibration: 45.7% right at confidence >= 0.9. Obligations re-rank measured net negative; default flipped off. | Keep to proposals: anchor inference (N4) and task preflight scoring (U6), each only after a passing calibration and an A/B against the deterministic baseline. (roadmap: N4, U6) | sr §5 SR-58; ADR-053 |
| F-70 | Assess lanes (priced model judge) | 3 / 1 | DITCH | No spend recorded. Designed and proven (ADR-046) but never used. | Freeze and remove from help until an owner pull. | sr §2 SR-18 (frozen, ADR-053 d4); ADR-053 |
| F-71 | providers / smoke | 4 / 2 | MERGE | CLI. Needed for compile. | Fold into doctor as a row. | sr §3 SR-32; ADR-053 |

### Bench (1)

| Id | Feature | Impl / value | Verdict | Usage and evidence | The better line | Landed |
|---|---|---|---|---|---|---|
| F-72 | Bench (context, scenario, index, review benches) | 5 / 4 | KEEP | 15+ recorded runs; ~$87 next round planned, not run. Honest nulls published (LCA, M1, M4). Level with best lookup tools; payload still 0.86-2.66x grep. | Run the planned next round; make bench-targeted the per-change loop. (roadmap: G16) | sr §7 KEEP; ADR-053 |

### Substrate (6)

| Id | Feature | Impl / value | Verdict | Usage and evidence | The better line | Landed |
|---|---|---|---|---|---|---|
| F-73 | Typed edges | 4 / 5 | KEEP | 141,103 edges across 21 services. Go: find_callers P/R 1.00. TS/Python file-resolved: precision 0.87/0.95, recall 0.03/0.12. | Raise non-Go recall, the biggest single accuracy gap. (roadmap: N5, T1, U13) | sr §7 KEEP; ADR-053 |
| F-74 | SCIP import | 3 / 3 | IMPROVE | 0 SCIP-tier edges in the live store. O115 fixed; mapping bugs closed against real zod/flask clones. | Run the SCIP indexer automatically in init when the toolchain is present. (roadmap: N5) | sr §6 SR-65; ADR-053 |
| F-75 | SCIP export | 4 / 2 | KEEP | CLI. VB-32. | Cheap interop; leave as is. | sr §7 KEEP; ADR-053 |
| F-76 | Language extraction breadth | 3 / 5 | NEW-ON-ROADMAP | See the Language Atlas. ADR-048 ladder running. | M2: 40 languages at L1. (roadmap: N5) | sr §7 note; vision 38 §9.3 (M2 = 45 at L1 supersedes the row's 40); ADR-053 |
| F-77 | Legacy analyzers (complexity, god_object, community, dead_code, cloc, secret_scan, shell_out, proto_conformance, traceability, evidence_summary, contradiction_scan, batch) | 2 / 1 | DITCH | 0 to 5 lifetime calls each. Exempted from the 34-tool prune; still no demand. | Remove the tools (law 4); keep complexity as an input to the U13 hotspot view and rebuild traceability as the U8 evidence-pack join, not a tool. (roadmap: G16, U8, U13) | sr §2 SR-19 (+batch, §8); ADR-053 |
| F-78 | Canned prompts (investigate_incident, onboard_service, plan_requirement, review_change) | 2 / 1 | DITCH | No usage record. They chain the legacy analyzers above. | Remove with the analyzers. (roadmap: G16) | sr §2 SR-20; ADR-053 |

### Roadmap (8)

| Id | Feature | Impl / value | Verdict | Usage and evidence | The better line | Landed |
|---|---|---|---|---|---|---|
| F-79 | System graph (system_impact, contract_drift, links) | 1 / 5 | NEW-ON-ROADMAP | Not built. Pieces exist: service_ref (P 1.00 / R 0.20), route table blast radius (P/R 1.00 blind). | Start with OpenAPI/proto plus net/http and Express extractors on a fixture fleet. (roadmap: N6, T5, U10) | sr §7 note; ADR-050; vision 38 §9.2 wave 2; ADR-053 |
| F-80 | Dependency API index and upgrade impact | 1 / 4 | NEW-ON-ROADMAP | Not built. conserve's api: delta (P 0.96 / R 1.00) is the reusable core. | Go module cache first, reusing the calibrated api delta. (roadmap: N1, T6) | sr §7 note; ADR-050 d4; ADR-053 |
| F-81 | Spec-dialect port (Spec Kit, OpenSpec, Kiro) | 1 / 5 | NEW-ON-ROADMAP | Not built. Plan grammar v2 is the canonical model; foreign formats lack anchors. | Spec Kit and OpenSpec adapters with a converge report. (roadmap: N4) | sr §7 note; ADR-049 d6; ADR-053 |
| F-82 | Doc search (SQLite FTS5) and doc drift | 1 / 4 | NEW-ON-ROADMAP | Not built. Doc questions are where fathom measured worst (gold source mostly Grep/Read). | Ship FTS5 plus the doc-question bench slice together. (roadmap: N7) | sr §7 note; ADR-049 d8; ADR-053 |
| F-83 | Progress views and proof history | 2 / 4 | NEW-ON-ROADMAP | Not built as views. The data exists: 17,125 verify-ledger readings since 2026-08-15 with total/claimed/verified per plan. | Render burn-up and proof regressions from the existing verify ledger first. (roadmap: N2, U9, U12) | sr §7 note; ADR-049 d4; ADR-053 |
| F-84 | Outcome economics | 2 / 3 | NEW-ON-ROADMAP | Not built as views. session-ledger already holds 3,359 sessions with token totals. | Join session-ledger to verify-ledger by session; show tokens per proven row. (roadmap: N3, U1, U2, U5) | sr §7 note; ADR-049 d5; ADR-053 |
| F-85 | Rules that learn, bi-temporal claims, SARIF | 1 / 4 | NEW-ON-ROADMAP | Not built. Schema already has valid_from/valid_to on semantic_claims (0 rows use valid_to); 0 human verdicts to learn from. | Collect verdicts first; retirement without signal is guessing. (roadmap: N8, U4, U7) | sr §7 note; ADR-049 d9; ADR-053 |
| F-86 | Small default surface (<= 8 tools) | 2 / 4 | NEW-ON-ROADMAP | Agent surface is 19 tools today. Schema tax was ~10k tokens/turn before O101/O85 cuts. | find_symbol, search, read_source, symbol_context, obligations, verify_plan, blast_radius, review_diff. (roadmap: G16) | sr §1.1, SR-33; ADR-053 d1; ADR-053 |

## What this does not claim

That the scores are computed or reproducible (they are the owner's judgment); that counts are current
after 2026-09-27 (the surface has since shrunk, so many "calls" figures describe tools that no longer
exist); that a KEEP is permanent (a feature with no consumer is deleted by the 30-day clock once a
consumer population exists, P8); that the verdict column is a status (the surface's status is computed
by `fathom support` and the plans by `fathom plans status`, never kept here).

## Sources

The files and ledgers listed in "Sources" above, on their stated dates. The rows are copied from the
JSON, which is the durable form; the last column was added by the sweep that summarized this reading.
