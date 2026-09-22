# The decision port — plan (vision 32, ADR-042)

Design: `docs/vision/32-decision-port.md`. Decision: ADR-042 (owner-directed,
2026-09-22). Grammar: `docs/plans/README.md`. Rows are proven by `fathom verify`; the
A/B rows at the end are operator decisions (LLM spend) and are never claimed by this plan.

## A. The port and its wiring (ADR-042 decisions 1, 2, 4)

- [ ] `[decide]` config block parsed with defaults (`provider = none`, `compile_threshold = 0.5`, `route_confidence = 0.9`, `rank_within_tier = true`); an unknown provider is refused by name at load {id: DP-1, symbol: Decide, package: internal/config, reachable_from: TestConfig_DecideBlock}
- [ ] `decide.Open` builds the port's `Decider` from config: `none` → the kit's `None`; `llm` → the kit's `LLM` over the configured profile lane (default: the compile lane); the daemon opens it once and every site receives it by injection, never by global {id: DP-2, symbol: Open, package: internal/decide, reachable_from: TestOpen_NoneAndLLM}
- [ ] Every decision is logged to the call log with site, provider, model, question kind, confidence and latency (`schema_version`-carrying rows, I32), and a decision in the unsure band is recorded as `applied: false` {id: DP-3, symbol: Record, package: internal/decide, reachable_from: TestRecord_UnsureIsNotApplied}
- [ ] `fathom doctor` gains a `decision_provider` row: `off` for `none`; for `llm` the profile lane's own ping and the last recorded latency {id: DP-4, symbol: checkDecisionProvider, package: cmd/fathom, reachable_from: TestDoctorDecisionProvider}

## B. Compile pre-filter (ADR-042 decision 2; ADR-037 preserved)

- [x] `PrefilterSections` asks one batched `noul` per section ("does this section state a binding rule for code in this repository") in chunks of at most 20 sections and returns the sections at or above `compile_threshold`; with `none` it returns every section untouched and makes no call {id: DP-5, symbol: PrefilterSections, package: internal/semantic, reachable_from: TestCompile_PrefilterSkipsAndDiscloses}
- [x] A skipped section is counted in `CompileRun.SectionsPrefiltered` and the additive `compile_runs.sections_prefiltered` column, listed in `uncompiled_scopes_json` like any uncompiled section, and the `layer_coverage` envelope names `prefilter_skipped`; the raw fallback (ADR-037) serves a skipped section exactly as it serves a budget-stopped one {id: DP-6, symbol: ObligationsLayerCoverage, package: internal/semantic, reachable_from: TestCompile_PrefilterSkipsAndDiscloses}

## C. Obligations re-rank within a tier (ADR-042 decision 2; ADR-035 preserved)

- [ ] `rerankWithinTiers` asks one batched `noul` per eligible rule ("does this rule govern this edit") with the edited path (and hunk when the hook supplies one) as state, and re-orders rules only inside their governing-scope tier, stable on ties; a rule never crosses a tier, never drops, and the ≤6 cap and displaced counts are computed after the re-rank exactly as before {id: DP-7, symbol: rerankWithinTiers, package: internal/semantic, reachable_from: TestObligations_RerankWithinTierOnly}
- [ ] The obligations envelope carries `ranked_by` (`none` | `llm`) and, per served rule, `relevance` (the noul probability, absent under `none`); `docs/mcp-tools.md` documents both {id: DP-8, oracle: lexical, path: docs/mcp-tools.md, pattern: "ranked_by"}

## D. Prompt routing (ADR-042 decision 2; I9/I12/I14 preserved)

- [ ] `POST /route` on the loopback daemon takes `{prompt}` and returns `{shape, verb, confidence, provider}` from one `choice` over the verb set (`localize`, `find_symbol`, `references`, `find_callers`, `obligations`, `staleness_report`, `search`, `none`); under `none` it returns `provider: none` and an empty verb without calling anything {id: DP-9, symbol: routeHandler, package: cmd/fathom, reachable_from: TestRouteHandler_NoneAndLLM}
- [ ] `fathom prompt-hook` calls `/route` inside its existing shared deadline and injects one line: firm at or above `route_confidence`, hedged in the confirm band, nothing below 0.5 or on any error; the whole injection stays within the hook's byte budget and the hook exits 0 on every failure {id: DP-10, symbol: routeLine, package: cmd/fathom, reachable_from: TestPromptHook_RouteLineBands}

## E. Docs, bench recipe, and the A/B (ADR-042 decision 5)

- [ ] `docs/configuration.md` documents the `[decide]` block and its defaults {id: DP-11, oracle: lexical, path: docs/configuration.md, pattern: "\[decide\]"}
- [ ] `docs/bench/scenarios.md` documents the corpus-variant recipe (an arm `setup` that appends the `[decide]` block; a different corpus hash is a different cell key) {id: DP-12, oracle: lexical, path: docs/bench/scenarios.md, pattern: "\[decide\]"}
- [ ] (op) Compile A/B on the fathom repo at one commit, `none` against `llm`: sections compiled, served rules lost, dollars, wall clock — bar: no served rule lost, under ten minutes and a dollar; LLM spend, owner asks {id: DP-13, skip: "operator decision: LLM spend"}
- [ ] (op) Serving and routing A/B: `obligations` gold-source rate on guards-fathom; fraction of issue-shaped prompts reaching `localize` and hit@1 on LCA, `none` against `llm` {id: DP-14, skip: "operator decision: LLM spend"}
