# The surface reckoning — the feature scorecard as decisions

Status: accepted 2026-09-27 (ADR-053). Input: the owner-accepted feature scorecard (86 features,
2026-09-27; sources: 97,898 MCP calls, 286,758 hook events, the verify and session ledgers,
the fleet store read-only) and the roadmap memo (N1–N8, T1–T8, U1–U16). Plan:
`docs/plans/surface-reckoning.md` (prefix `SR`). Governing rules: law 4 / principle P4 (no
net tool growth), ADR-014 and its amendments (surfaces; execute-and-flag), ADR-035 (verbs,
not lookups), `docs/plans/archive/surface-prune.md`'s two-part exemption check, the compatibility
surface in `docs/intent/scope.md`.

This document does not re-decide verdicts. It turns each one into a change that can be
proven, and it names what the code and the records say about each deletion. Where the
evidence forced a narrower or wider change than the scorecard's one-line verdict, the row
says **refined** and gives the evidence (§8 collects them).

---

## 1. The counts

Measured on `86d4c5e2` with a throwaway test over `tools/list` (fake backend, default
diet, no `outputSchema`; hosted-modules mode off, so the four module tools are not in the
bare-`/rpc` count):

| Surface | Before | After | Schema bytes before | After (target) |
|---|---|---|---|---|
| Full registry (`POST /rpc`) | 59 (63 with the four module tools) | 25 | 34,246 | ≤ 19,000 |
| Agent surface (`POST /rpc/agent`, `/rpc/companion`) | 25 | 8 (default) | 18,310 | ≤ 9,000 |
| Canned prompts (`prompts/list`) | 4 | 0 | 1,539 | 0 |

Measured after (SR-M1, `TestDefaultSurfaceSchemaBytesBudget`, base `4ae46d03`, same method): `/rpc/agent`
8 tools, 8,339 bytes (the ≤ 9,000 bar holds); `/rpc/extended` and `/rpc/companion` 21 tools, 18,308; bare `/rpc` 25
tools, 20,143, which stands 1,143 bytes over the table's ≤ 19,000 target (the row gates the default only; the other
surfaces are pinned at what they measure).

The scorecard's "19 tools today" is the base allowlist in `internal/mcp/surface.go`;
`promotedVerbs()` (five) and `safe_delete_check` bring the served agent surface to 25.
`docs/intent/scope.md` still says "the 19-tool agent surface allowlist" — a stale line the
orchestrator corrects in the same wave (§9).

The eight tools of the current schemas alone weigh 7,706 bytes; the target leaves room for
the merges (blast_radius gains `kind`, repo_map gains entrypoints, obligations gains spec
references).

### 1.1 The default surface (F-86, ≤ 8 tools)

`find_symbol`, `search`, `read_source`, `symbol_context`, `obligations`, `verify_plan`,
`blast_radius`, `review_diff`.

Everything else that survives is **opt-in**: a new path-scoped surface `extended`
(`POST /rpc/extended`) carries the default eight plus `find_callers`, `outline`,
`references`, `query_symbols`, `pack_context`, `repo_map`, `localize`, `test_impact`,
`staleness_report`, `list_services`, `status`, `findings_near`, `ingest` (21). Bare `/rpc`
keeps the full 25, including the admin tools `compile_layer`, `remove_service`,
`rename_service` and `cross_repo_callers`. A new surface is not a new tool (law 4 counts
tools); `/rpc/companion`, the compatible alias, moves to `extended` so a client that pinned
it keeps its old tool set minus the deletions.

Why this is safe to shrink: surfaces are execute-and-flag (ADR-014 amendment 2026-07-16).
An agent that calls `outline` on the default surface still gets its answer; the call log
records `off_surface: true`, so demand for anything left off the default is measured, not
argued (SR-M3). Two consequences are handled as rows: the stanza must stop telling agents
to call `ingest` (auto-refresh, RT-99, queues a refresh on read; the stanza region is on the
compatibility surface), and `surface_diet_test.go`'s 19-tool cap becomes an 8-tool cap on
the default.

## 2. Deletion safety — the rule every removal row follows

A removal row is proven by an **absence contract** (`gone: pattern` over the registration
`Name:` literal, scoped to `internal/mcp/`, or `gone: import` for a package) plus the
pinned registry test (`TestRegistryCarriesNoReckonedTools`, the `surface_prune_test.go`
precedent). Before the row is claimed, its commit carries:

1. **Callers evidence.** `safe_delete_check` (or `find_callers` at typed tier) on every
   `register*` handler and helper removed, recorded in the commit's `Verified:` line.
   Helpers reachable from surviving code stay (they are "the internal function verify
   needs"); only helpers reachable solely from the removed handler go.
2. **The exemption check, re-run** (surface-prune SP-B1): (a) no fathom code calls the tool
   over MCP — the in-repo callers found on `86d4c5e2` are listed per row below and each is
   rewired first; (b) no shipped skill, stanza or `docs/use-cases/*.md` worked example
   directs an agent to it — each hit is rewritten in the same commit (a `## Maintained`
   line updated per `docs/AGENTS.md`).
3. **Dependent plan rows.** A claimed `[x]` row anchored on a removed symbol would turn
   claimed-unproven. Where the capability survives under another symbol, the row is
   **re-anchored** in the same commit (title unchanged, new `symbol:`/`reachable_from:`).
   Where the capability is retired, the row becomes `[-]` with
   `disposition: refused, reason: "retired by SR-<n> (surface reckoning)"`. Rows are never
   left to fail and never silently deleted.
4. **Compatibility.** Every removed or renamed MCP tool name is on the compatibility
   surface ("tool names and schemas"; the `module_query` wire contract v0 is listed on its
   own). Each wave's removals get one `### Breaking` block in `CHANGELOG.md` and one
   migration note, `docs/upgrades/0.9.0-surface-reckoning.md` (tool → replacement table).

`docs/mcp-tools.md` and `internal/mcp/surface.go` change in the same commit as each
registration (`TestMcpToolsDocMatchesRegistry` holds the three-place contract).

## 3. DITCH — what is deleted

| Row | Feature | What goes | What stays (and why) | In-repo callers to rewire first | Docs/skills to rewrite | Dependent claimed rows |
|---|---|---|---|---|---|---|
| SR-10 | F-17 `cross_repo_blast_radius` | registration + handler (`registerCrossRepoBlastRadius`) | `serviceRefHops*` helpers only if another tool reaches them (blast_radius does via `withServiceRefHops`) | none | `use-cases/cross-repo-blast-radius.md` | none |
| SR-10 | F-18 `dep_drift` | registration + handler | `internal/depgraph` (vision 15 §3: ci-kit's engine seam; N1 rebuilds on it) | none | `use-cases/change-review.md` | none |
| SR-11 | F-19 `package_deps` | nothing — already deleted in the 2026-08-23 prune (`prunedZeroCallTools`) | — | — | memo N1 and vision 15 §3 still name it (orchestrator edit, §9) | none |
| SR-12 | F-22 `http_client_calls`, `fleet_drift`, `reachable`, `traverse`, `history` | the five registrations and handlers | graph reachability used by verify's `reachable_from` lives in `internal/verify`/`internal/graph`, not in `registerReachable`; `cmd/fathom`'s `"reachable"` hits are JSON field names, `fathom purge history`/`plans history` are CLI verbs — none call the tools | none | `use-cases/fleet-health.md`, `incident-investigation.md`, `cross-repo-blast-radius.md`, `getting-started.md` | none |
| SR-13 | F-45 anchored memory | `memory_search` tool, `fathom memory` verb, the edit hook's memory injection (`injectMemories`), `internal/memory`, `.fathom/memories.jsonl` handling | nothing (0 memories in any repo; threads carry the hand-off) | `cmd/fathom/edithook.go` (calls `memory_search`), `internal/contextbench/exercises.go` (lists it), the tenancy memory-index caches (DF-16/17, RT-36) | `use-cases/coding-agents.md`, `coding-context.md`, `getting-started.md`, `incident-investigation.md`; skill `fathom-onboard` (+ `dist/`) | **retire** AM-1..AM-11 (ambient-fathom); ~~DF-16, DF-17 (daemon-footprint), RT-36 (responsible-tenant)~~ stay proven (§8) |
| SR-14 | F-53 `narrate` | `fathom narrate`, story-mode rows (`recordStoryRow`, `narrateActive` checks in hooks) | — | hooks that call `recordStoryRow` | none found | **retire** IO-21, IO-22 |
| SR-14 | F-53 `brief` (CLI) | `fathom brief` (`cmdBrief`) | `renderBriefMD`, `briefTuning` — the console's `/brief` page renders them; that page is the "console covers the need" | none | skill `fathom-impact` (+ `dist/`) | **retire** OB-7; BR-1, BR-3 stay proven |
| SR-15 | F-53 `brief` (MCP tool) — **refined** | `registerBrief`, `truncatedBriefResult` | — | none | `use-cases/coding-agents.md`, `coding-context.md`, `spec-verification.md` | **retire** D1-1, D1-2 (expert-depth) |
| SR-16 | F-53 `research` — **refined to freeze** | `fathom research` leaves top-level help | `internal/research` and the verb, hidden: ADR-046 decision 6 names the research bundle as the only route for the retro corpus, and `schema/research-bundle` is its closed allowlist | — | none | P-2, P-4, P-13, P-14, GKE-W1-7/8/10, IR-5, R-2, R-3, O-22, O-23 stay proven |
| SR-17 | F-68 expert-module MCP tools | `list_modules`, `module_obligations`, `module_query`, `module_staleness`, the `/rpc/modules` surface and hosted-modules mode (`hostedModulesTools`, `--modules-token-env`) | the Expert Module v0 manifest, `schema/module`, `.fathom/modules.cue`, `internal/modspec`, `fathom module` verbs (eval/kill gates) — the pack format layers already use; all compatibility-surface items except the wire contract | `cmd/fathom/doctor_modules.go` (fixed in `edcc2352` to ask `tools/list` first — becomes an always-absent row, then removed) | `use-cases/platform-modules.md`, `getting-started.md`; skill `fathom-doctor` (+ `dist/`) | **retire** GKE-MH-5 (+ GKE-MH-7, Q-26, §8); J-*, FA-4..6, O-5, O-6 anchor the kept module code and stay proven |
| SR-18 | F-70 assess lanes — **freeze** | `fathom assess` leaves help | `internal/assess`, the `assess` capability (default off), the ledger: ADR-046 is accepted; ADR-053 marks it *frozen*, not superseded, until an owner pull | — | none | DO-3, IR-3 stay proven |
| SR-19 | F-77 twelve legacy analyzers as tools | `complexity`, `god_object`, `community`, `dead_code`, `cloc`, `secret_scan`, `shell_out`, `proto_conformance`, `traceability`, `evidence_summary`, `contradiction_scan`, `batch` registrations and handlers | the complexity metric (a stored column, `internal/store/conformance.go`) as U13's input; traceability's join logic moves to U8's evidence pack when that is built (not a tool); `batch` admission code in the governor becomes dead and goes with it | `cmd/fathom/governor_rpc.go`, `lifecycle_gate.go` special-case `batch` (delete the branches) | `use-cases/fleet-health.md`, `incident-investigation.md`, `change-review.md`, `spec-verification.md`, `coding-context.md`, `getting-started.md`; skill `fathom-plan` (+ `dist/`) names `traceability` | **retire** RT-60 (`admitBatchCall`) |
| SR-20 | F-78 four canned prompts | `investigate_incident`, `onboard_service`, `plan_requirement`, `review_change` | `registerPromptsHandlers` stays and serves an empty `prompts/list` (the protocol capability is kept; C3-1 stays proven) | `governor.go` and `tenancy/inflight.go` name `onboard_service` as an ingest-class tool — delete the dead string | `docs/mcp-tools.md` §Prompts | none |

The scorecard's F-77 names "12 legacy analyzers"; `batch` is not an analyzer but is on the
list with the same zero-use evidence (0 calls in `calls.json`), so it goes with them.

## 4. MERGE — what folds into what

| Row | Feature | Into | The concrete change | Rewire / re-anchor |
|---|---|---|---|---|
| SR-21 | F-05 `find_callees` | `symbol_context` | `symbol_context` already returns callees; the tool is unregistered | docs: `coding-context.md`, `cross-repo-blast-radius.md` |
| SR-22 | F-10 `entrypoints` | `repo_map` | `repo_map`'s header gains an `entrypoints` list (same detector, capped at 10, with a `more` count); the tool is unregistered; the system graph (N6) later makes entrypoints nodes | `internal/metrics/discovery.go`, `internal/contextbench/exercises.go`; docs: `coding-context.md`, `incident-investigation.md`; skill `fathom-onboard` |
| SR-23 | F-11 `capabilities`, `server_stats`, `list_sources` | one `status` tool | `status {section: tools|daemon|sources, service?}` returns what the three returned, byte-compatible per section; `list_services` stays separate (37,155 calls, hooks and verify CLI) | `cmd/fathom/doctor.go` (`server_stats`), `doctor_phaseo.go` (`list_sources`), `internal/console/callers.go` plumbing set; skills `fathom-doctor`, `fathom-onboard`; **re-anchor** AD-1, AD-2, AD-4, A1-1, A1-2, K3-1, K4-1, P1-7 onto `registerStatus` |
| SR-24 | F-13 `rename_impact` + `safe_delete_check` | `blast_radius` | `blast_radius` gains `kind: change (default) | rename | delete`; `rename` takes `symbol` and returns every call site (today's rename_impact), `delete` returns remaining referents with tiers or a proven-zero receipt (today's safe_delete_check). The verb on the default surface now answers all three; U9's migration tracker counts the same sites | `promotedVerbs()` loses `rename_impact`; docs `cross-repo-blast-radius.md`; **re-anchor** the verbs.md safe-delete and rename rows onto the kept implementation functions |
| SR-25 | F-16 `diff_impact` | `review_diff` | unregistered; review_diff is the verb | docs `change-review.md`, `cross-repo-blast-radius.md` |
| SR-26 | F-20 `api_diff` (tool) | `conserve` / upgrade impact | tool unregistered; the calibrated API delta stays in `conserve` (ADR-044) and N1 reuses it | docs `change-review.md` |
| SR-27 | F-35 `arch_conformance`, `spec_coverage`, `spec_refs` | review internals (and U11) | unregister the three; `review_diff`'s engine (`internal/analysis/review`) already calls the analyses in-process; the edit hook's `spec_refs` call (306 calls, the one real consumer) moves to an additive `spec_refs` section on the `obligations` answer the hook already fetches | `cmd/fathom/edithook.go`; skill `fathom-plan`; docs `change-review.md`, `spec-verification.md`; **re-anchor** P1-4, C4-3, C4-5 onto the review engine's conformance function |
| SR-28 | F-56 `stale_docs` | `staleness_report` | `staleness_report` gains `doc_drift` (doc spans naming a symbol that no longer exists, today's scan); N7's changed-symbol drift extends the same field | docs `change-review.md` |
| SR-29 | F-47 `discoveries` tool, `routes` verb | the session brief | `prime`/session hook already renders discoveries; the MCP tool is unregistered, `routes` becomes `prime --routes`; the discovery-not-reused count becomes a U3 input | **re-anchor** RC-27 onto the session brief's discovery renderer |
| SR-30 | F-44 CUE shadows | internal | `gen-status-cue`, `gen-plan-cue` leave top-level help (ADR-018 decision 3 keeps the round-trip; V0-2, V0-3 stay proven) | — |
| SR-31 | F-52 metrics verbs | one `metrics` verb | `metrics {instrument|flywheel|retro|rate|arm}`; the old verbs stay as hidden aliases for one release (hooks and skills that call them keep working), then go | skills `fathom-impact`; the 30 claimed rows anchor functions that survive — no re-anchor |
| SR-32 | F-71 `providers`, `smoke` | `doctor` | a `providers` doctor row runs the auth and one-token probes; the verbs become hidden aliases (CF-5, CF-6 stay proven) | skill `fathom-doctor` |

## 5. IMPROVE — each "better" line as a testable change

| Row | Feature | The change | Proven by |
|---|---|---|---|
| SR-40 | F-02 references | default `exact: true`; a free-text query (whitespace, or not identifier-shaped) is answered by `search` with a `routed_from: references` note instead of a refusal | a test that a free-text query returns search hits, never an error |
| SR-41 | F-07 repo_map | default budget cut (map class, `budget_defaults.go`) and an `orientation-only` label in the envelope; merges into pack_context only if SR-42 keeps pack_context | budget test on the new default |
| SR-42 | F-23 pack_context | run BP-46; if it does not beat bare tools on bytes, fold into query_symbols (a `pack: true` mode) and unregister | the BP-46 reading, then either a keep note or an absence contract |
| SR-43 | F-24 query_symbols | default top-N lowered; the answer opens with a one-line summary (`answer:`) | a test that the first field is the summary and N defaults to the new value |
| SR-44 | F-25 localize | the prompt hook routes issue-shaped prompts (a stack trace, a failing test name, an error string) to `localize` deterministically; LCA re-run | prompt-hook test with the three shapes |
| SR-45 | F-26 obligations | checkable rules compile to `enforce: true` guards and a lint output; the obligations answer marks which served rules are checkable | a test that a checkable layer rule yields a guard |
| SR-46 | F-28 prompt hook | `fathom init` wires it by default; first-edit violations measured with and without | init test + a measurement row |
| SR-47 | F-29 guards | more mechanically checkable rules per pack, and SARIF output of guard verdicts | SARIF golden test |
| SR-48 | F-31 claim triage | when a served rule that has a claim id is served again (no violation alleged: serving twice is not evidence of one), the next edit-hook turn asks for a one-key verdict; the answer is written as refute/corroborate | edit-hook test of the ask and the write |
| SR-49 | F-34 review | `review_diff` becomes the change risk receipt (U7): blast radius, certificates, conservation, rules touched, reviewers from ownership, as SARIF and a GitHub check payload | receipt golden test |
| SR-50 | F-15 tests select | rebuilt as certifying-first selection (T4) scored against a coverage oracle | selection test over a fixture with certificates |
| SR-51 | F-21 cross-repo callers | regex detectors replaced by framework route extractors and contract files (N6 system graph) | owned by the system-graph plan; this row closes when `cross_repo_callers` reads links from it |
| SR-52 | F-46 continuity threads | reach the 20-unit sample and let the flywheel keep or kill | a ledger reading with n ≥ 20 |
| SR-53 | F-51 doctor | a timed stranger check (install → first proven answer ≤ 5 min, G16) | `doctor --stranger` test with an injected clock; the `list_modules` fix is SR-2 |
| SR-54 | F-54 console | progress and economics views (N2/N3, owned by the progress plan); fix plan-row click depth (UX-M1 failed 4 clicks on "plan rows remaining") | UX-M1 re-measured |
| SR-55 | F-57 fleet reconcile | reconcile triggered from the console (user-triggered, never scheduled by fathom) and drift on the fleet page | console test of the trigger path |
| SR-56 | F-58 init / plugin | stranger check timed; the plugin published (owner's Q4 call) | stranger reading; publish is an owner op row |
| SR-57 | F-61 responsible tenant | ship RT-99 and RT-107; read RT-M1's seven days | owned by `responsible-tenant.md`; this row needs those rows |
| SR-58 | F-69 decision port | proposals only: anchor inference (N4) and preflight (U6), each behind a passing calibration and an A/B | owned by the spec-interop and work-intelligence plans; this row records the constraint |

F-12 blast_radius, F-33 findings, F-41 certifications, F-67 ui-appkit and F-74 SCIP
import are IMPROVE rows whose "better" line is a delivery change; they are planned once, in
wave 4 (§6).

## 6. Underused delivery (wave 4)

The nine underused items share one fix: put the evidence where the agent or the owner
already is, instead of waiting to be asked.

| Row | Item | The change |
|---|---|---|
| SR-60 | blast_radius (F-12) | `review_diff` includes the blast radius summary; the pre-edit hook adds a one-line "N callers, M tests reach this" when the edited symbol has callers; co-change oracle is a later row |
| SR-61 | per-rule counts (F-27, U4) | the edit hook records served / complied / violated per claim id; `obligations` and the console show the counts. **Owner bar (2026-09-28):** the claim id adds at most 3% bytes to an obligations call — it is served only to the edit hook's own request, an agent's payload does not carry it — and `TestPayloadReplay` only re-baselines downward. The plan splits the row: SR-61 the recording, SR-61a the reader (`fathom obligations`, `/explain`), SR-61b the payload bar |
| SR-62 | certifications (F-41) | liveness bound to the commit (O114), service resolution fixed, certificates declared on fathom itself |
| SR-63 | conservation (F-42) | `fathom init` wires `conserve` on the pushed range into every adopter's pre-push gate by default (today only this repo's `lefthook.yml` runs it); review shows plane verdicts |
| SR-64 | findings loop (F-33) | Review Kit publishing armed on one real repository; the import receipt shows > 0 findings |
| SR-65 | SCIP import (F-74) | `fathom init` runs `scip-typescript`/`scip-python` when installed and imports the index. **Owner decision (2026-09-28):** the `ingest` MCP tool gains an additive optional `indexers` parameter (a closed set: `scip-typescript`, `scip-python`); init passes a name only when that toolchain is already on PATH and never installs anything; the daemon imports the produced `.fathom/scip/<name>.scip` under the admission budget (the ingest slot, and a size ceiling from the gate) |
| SR-66 | ui-appkit pack (F-67) | compiled on app-kit and one consumer; UI rule violations measured |
| SR-67 | call log + hook log | refused 2026-09-29: the U1–U5 views read the call log through `callLogFold`, and the shared reader (`ReadActivityRecord`) has no production caller |
| SR-68 | verify + session ledgers | refused 2026-09-29 as a duplicate of `progress-and-economics.md#PE-15`, whose `JoinSessionProof` is the live join by session id |

## 7. KEEP — listed with their optional "better" line

Not planned as rows; each is optional and may be picked up by any plan.

| Id | Feature | Optional better |
|---|---|---|
| F-01 | find_symbol | Return the implementations and the one-line answer first so the agent stops after the hit instead of chaining references/repo_map. |
| F-03 | symbol_context | Serve a degraded partial answer instead of an over-budget refusal. |
| F-04 | find_callers | Extend the same resolved tier to TS/Python (file-resolved recall is 0.03/0.12 today). |
| F-06 | outline | Budget doc-file outlines to headings only; point doc questions at doc search (N7). |
| F-08 | read_source | Auto-clamp an over-wide range with a note instead of refusing it. |
| F-09 | search | Read its kill criterion on the next lookups run and publish the verdict. |
| F-14 | test_impact | Rank by coverage evidence when a profile is present (CV-M13 found 3.7% reach slack). |
| F-27 | Rules at the edit | Record served / complied / violated per rule (U4) — planned as SR-61 because it is also underused. |
| F-30 | Layer compiler | Publish per-claim serve counts so compiled claims earn their cost. |
| F-32 | Rules to tests | Replay on repos with real violations (other fleet repos) before judging. |
| F-36 | verify / verify_plan | Measure M2 on a public corpus with plan-grammar rows so the core claim has a number. |
| F-37 | Plan grammar v2 | Add `weight:`, `accept:` (EARS) and `tests:` fields; read Spec Kit/OpenSpec in place (N2, N4). |
| F-38 | plans status / next / history | Burn-up and proof regressions from the existing verify ledger (N2). |
| F-39 | Claim moment | Report the refusal rate per agent as claim honesty. |
| F-40 | Receipts | Extend re-derivation beyond Go. |
| F-42 | Refactor conservation | Run it in the pre-push gate — planned as SR-63 (underused). |
| F-43 | Execution evidence | Ingest lcov/Cobertura/JaCoCo/coverage.py/Istanbul (T3). |
| F-48 | Session brief | Measure first-edit tokens with and without it (G2 A/B). |
| F-49 | Intent audit | Evaluate computable revisit triggers by machine (U11). |
| F-50 | Config custodian | Serve only the anchored-fresh class until the other classes pass. |
| F-55 | staleness_report | Doc drift — its first half is SR-28. |
| F-60 | Skills pack | Log skill invocations so the call log can judge them. |
| F-62 | scope suppress / purge | Surface suppressions on the console system page. |
| F-63 | Ingest and refresh | Incremental equals full equivalence check before live updates. |
| F-64 | obligations pack | Retire rules never served in 90 days (needs SR-61's counts). |
| F-65 | adr-rules pack | Bi-temporal validity so a superseded ADR's rule has an end date. |
| F-66 | spec-bindings pack | Feed foreign spec formats through the same pack (N4). |
| F-72 | Bench | Run the planned next round; make bench-targeted the per-change loop. |
| F-73 | Typed edges | Raise non-Go recall, the biggest single accuracy gap. |
| F-75 | SCIP export | Cheap interop; leave as is. |

NEW-ON-ROADMAP rows (F-59, F-76, F-79–F-86) are owned by vision 38 and its plans, except
F-86 (the default surface, SR-33 here) and F-59's verification of other clients, which the
support matrix tracks as the `client` kind (`docs/design/support-matrix.md`).

## 8. Verdicts refined, with evidence

- **F-53 `brief` covers two things.** The scorecard's row names the CLI verbs. The MCP
  `brief` tool (worker pre-pack, `registerBrief`) has 0 calls in `calls.json` and
  overlaps `pack_context`; it goes under the same verdict. The CLI verb goes, but
  `renderBriefMD`/`briefTuning` stay: the console's `/brief` page is exactly the "console
  covers the need" the verdict relies on.
- **F-53 `research` is frozen, not deleted.** ADR-046 decision 6 (accepted) makes the
  research bundle the only route for the retro corpus, with a closed CUE allowlist
  (`schema/research-bundle`) and 12 claimed rows. The verb leaves help; deletion waits on
  an ADR that supersedes decision 6.
- **F-68 removes the module tools, not the module system.** The verdict's own "keep the
  pack format" plus the compatibility surface (Expert Module v0 manifest, `modules.cue`)
  and 17 claimed rows on the kept code bound the change to the four MCP tools, the
  `/rpc/modules` surface and hosted mode. The `module_query` wire contract v0 is a
  compatibility item and goes with a Breaking line.
- **F-45's retire list over-reached.** DF-16, DF-17 and RT-36 anchor `applyMemoryLimit`
  and `memoryLimitMetricLine` — the Go runtime memory limit (`debug.SetMemoryLimit`), not
  an anchored-memory index cache (no such cache exists in the tree). They stay proven;
  SR-13 retires AM-1..AM-11 only.
- **F-68's hosted mode takes two more rows.** GKE-MH-7 and Q-26 anchor
  `modulesAuthMiddleware`, hosted mode's bearer gate, which cannot outlive hosted mode;
  SR-17 retires them with GKE-MH-5. `/statusz` stays, loopback-only like every route.
- **F-77 includes `batch`.** Not an analyzer, same evidence (0 calls); its governor
  special cases and RT-60 go with it.
- **F-22 `reachable` has no internal consumer.** The scorecard hedged "keep the internal
  functions that verify needs"; on `86d4c5e2` no verify path calls the tool or its handler
  — verify's `reachable_from` walks the call graph in `internal/verify`. Nothing to keep.
- **F-11 counts.** The agent surface is 25 tools, not 19 (§1).

## 9. Edits for the orchestrator (records this subagent may not touch) — done

- `docs/intent/scope.md` compatibility surface: "The 19-tool agent surface allowlist" →
  "The default agent surface (8 tools), the `extended` surface, tool names and schemas,
  `POST /rpc/agent`, `/rpc/extended` and the `/rpc/companion` alias (→ extended)"; drop
  "`module_query` wire contract v0" when SR-17 lands (retired, Breaking). Done.
- `ADR.md`: folded in as ADR-053 (it amends ADR-014, freezes ADR-046, records the
  module-tool removal against the Expert Module line). Done.
- Vision 15 §3 and the roadmap memo's N1: `package_deps` is already deleted (SR-11). Done.
- `docs/intent/futures.md`: none required; anchored memory is removed with "revisit only
  with demand" — record it as a future only if the owner wants the trigger kept.
