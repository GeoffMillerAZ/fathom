# Verbs, not lookups — the surface rebalance, obligations by scope, governance that bites, receipts a stranger can check, a corpus fathom does not own

Design: `docs/vision/29-verbs-not-lookups.md`. Decision: ADR-035. Waves A–D are
free and land first; E–J carry their own pilot-tier rows. Every paid row stays
inside vision 27 §9's tiers.

Verify: `fathom verify --service fathom --plan docs/plans/verbs.md`.

## Wave A — the verbs lead (R1, R2)

- [x] `blast_radius`, `review_diff`, `test_impact` and `rename_impact` are on the agent surface; the pinned surface contract, the schema-tax pin and `docs/mcp-tools.md` move with them {id: VB-1, symbol: promotedVerbs, package: internal/mcp}
- [x] `safe_delete_check` answers "can this symbol go": the remaining referents with their resolution tier, or a proven-zero receipt naming the index commit; a read-only sibling of `rename_impact`, never a mutator {id: VB-2, symbol: registerSafeDeleteCheck, package: internal/mcp}
- [x] The opening-moves stanza, the primer and the skills name a verb first and a lookup second for every task shape (review/verify: `review_diff` → `obligations` → `references`; refactor: `rename_impact` → `references`; debug: `blast_radius` → `find_callers`), regenerated through the real generator {id: VB-3, oracle: lexical, path: docs/guides/fleet-index.md, pattern: rename_impact}
- [x] Every scenario cell carries `gold_source_tool` — the tool whose result first held the gold token — with `gold_source_rate` per tool per question shape on the ledger row, and a tool is called a strength only when it was the gold source {id: VB-4, symbol: GoldSourceTool, package: internal/scenariobench}
- [x] The home bench publishes `gold_source_rate` per tool per question shape beside `trusted_without_recheck`, including the tools that score zero {id: VB-5, symbol: GoldSourceRate, package: internal/contextbench}
- [x] The L0 index oracle runs in `task validate`: precision and recall of `find_callers`, `references` and `blast_radius` against go/types over the fixture fleet, committed as goldens, a regression is a failing test {id: VB-6, symbol: IndexOracleGoldens, package: internal/indexbench}
- [x] The L1 payload replay runs in `task validate`: a committed list of (tool, args) against the fixture store with golden bytes, result counts, elision counts and a latency budget per entry, seeded with the five formerly uncapped tools and `obligations` with its `displaced` count {id: VB-7, symbol: PayloadReplay, package: internal/indexbench}
- [ ] The home bench's wired arm runs against a scratch daemon of the exported subject (the layer compiled once per export), so no served path names the live checkout; the scorer accepts spelling-equivalent tokens (`answer_any_of` groups); a Bash read outside the session root is noted on the cell {id: VB-36, oracle: lexical, path: scripts/context-bench.sh, pattern: scratch-daemon-default}

## Wave B — obligations names the rule that governs (R3)

- [x] A rule's scope is the narrowest path prefix, package or symbol it binds; proximity of the edited path to that scope is the primary sort key and the keyword score breaks ties inside a tier {id: VB-8, symbol: rankByGoverningScope, package: internal/semantic}
- [x] `obligations` splits its layers: `layer: rules` (human-authored: CLAUDE.md, ADR invariants, guards) is the default; `layer: claims` (plan-row bindings) is opt-in and binds by a row's `{symbol, package}` contract, never by prose keyword — a row with no contract binds to nothing {id: VB-9, symbol: obligationLayers, package: internal/mcp}
- [x] Stale rules are demoted below the budget line, never served first; the cited lines are inlined under a snippet cap so a trusted rule costs no round trip; `displaced` reports the scope tier of what was dropped {id: VB-10, symbol: renderObligationReceipt, package: internal/mcp}
- [x] Forty files across the fixture fleet and two public checkouts carry a hand-derived governing rule at HEAD, written by a reader who never saw the ranker; the baseline (today's scorer, grep of the nearest CLAUDE.md) is measured and committed before the change {id: VB-11, oracle: lexical, path: testdata/indexbench/governing-rules.json, pattern: governing_rule}
- [x] (op) M10 written from the free oracle: the governing rule in the top three for ≥ 80 % of the forty files and ≥ 20 points over the baseline, else `obligations` serves only exact scope bindings and the governance-lookup claim leaves positioning — 2026-09-14: top-1 0.875 → 1.000, top-3 1.000 → 1.000, grep of the nearest CLAUDE.md 0.975; the top-three bar was already met by the old ranker, so the twenty-point clause is unmeetable on this oracle and the honest reading is a top-1 gain plus the layer split that removes plan-row bindings (O23); the oracle needs files governed by a non-nearest document and decoy plan rows before it can discriminate further (the baseline was replicated byte-for-byte after the change, not pre-registered) {id: VB-12, skip_test: true, skip: "free oracle run; evidence is the committed baseline and post-change readings under testdata/indexbench/", needs: VB-11}

## Wave C — governance that bites (R4)

- [x] A `layer-rule` obligation kind in CUE: import direction, no cycles, package boundaries, cross-service dependency direction; checked live against the index at obligations time {id: VB-13, symbol: EvaluateLayerRule, package: internal/guardspec}
- [x] `fathom rules gen-test` emits a Go test per layer rule so the same rule fails `task validate` and any CI; a seeded violation proves the generated test fails {id: VB-14, symbol: cmdRulesGenTest, package: cmd/fathom}
- [x] A GitHub Ruleset template makes `fathom verify` and the generated rule tests a required check, documented for a stranger {id: VB-15, oracle: lexical, path: docs/adoption/github-rulesets.md, pattern: required_status_checks}
- [ ] (op) M13: this repository's last 200 commits replayed against its authored layer rules, and the gated arm against the advisory arm on the far-from-edit corpus (RC-50) at n=3; if nothing is caught anywhere, governance stays advisory and the row says so {id: VB-16, skip_test: true, skip: "replay and pilot spend; evidence is the replay report under workspace/ and the ledger rows", needs: VB-14}

## Wave D — the freshness receipt (R5, SB-51)

- [x] A scripted-edit freshness probe: N mechanical changes at HEAD, one question per change that only HEAD can answer, scored as post-edit correctness and time to a correct answer, run against fathom's own index for free {id: VB-17, symbol: FreshnessProbe, package: internal/indexbench}
- [ ] (op) M7 written: the repaired FRESH task across five arms (SB-51) plus the probe once per release against graphify in `--watch` mode; if graphify answers four of five post-edit questions, freshness leaves positioning {id: VB-18, skip_test: true, skip: "operator spend; evidence is the ledger rows", needs: VB-17}

## Wave E — receipts a stranger can check (R6)

- [ ] Every edge carries a resolution tier — `typed`, `heuristic`, or `external(<indexer>@<version>)` — and every answer's envelope reports the mix, the index commit, and per-fact anchors (file, symbol fingerprint, content hash) {id: VB-19, symbol: resolutionTierOf, package: internal/core}
- [ ] `fathom receipt verify <receipt>` re-derives a served claim from a bare checkout at the named commit with go/types and git alone — no daemon, no index — so exit 0 means a third party reproduced it {id: VB-20, symbol: cmdReceiptVerify, package: cmd/fathom}
- [ ] (op) M11b: fifty sampled receipts re-derived independently at ≥ 95 %, and the envelope under 10 % of median payload in the L1 replay, else the envelope is removed {id: VB-21, skip_test: true, skip: "sampling run; evidence is the report under workspace/ and the L1 replay goldens", needs: VB-20}

## Wave F — the review a human sees (R11)

- [ ] `fathom review --format github` renders one deterministic comment per PR: changed symbols with resolved callers and their tier, call-site completeness, affected tests and gaps, governing rules with cited lines, a verify line when a plan row is touched, each line carrying its claim id — no model prose on the required path {id: VB-22, symbol: renderGitHubReview, package: internal/analysis/review}
- [ ] A GitHub Action posts that comment and can be a required check under a Ruleset, documented for a stranger {id: VB-23, oracle: lexical, path: docs/adoption/github-action.md, pattern: fathom review --format github}
- [ ] (op) M15: on fifty to sixty historical merged-then-fixed PRs from two public Go repositories, the comment names the later-fixed call site or uncovered symbol in ≥ 30 % of cases with ≤ 1 false finding per PR, beside a model reviewer on the same diffs {id: VB-24, skip_test: true, skip: "replay spend; evidence is the ledger rows", needs: VB-22}

## Wave G — a corpus fathom does not own (R10, R12, BC-15)

- [ ] A harness adapter runs the same model with and without the agent surface on Long Code Arena's bug-localization track, emitting rows into the scenario ledger with instrument truth (server connected, hooks fired, surface calls) {id: VB-25, symbol: RunExternalCorpus, package: internal/scenariobench}
- [ ] `localize`: given a stack trace, failing test, error string or concern, one bounded, budgeted, logged loop over the index ranked by graph proximity, recency and open obligations, returning a diagnosis object with citations and the hops taken — never a bare list {id: VB-26, symbol: registerLocalize, package: internal/mcp}
- [ ] (op) M12 and M16 written with paired statistics at n ≥ 3 over ≥ 30 tasks; a confidence interval that includes zero publishes the null and closes BC-15 on it {id: VB-27, skip_test: true, skip: "operator spend; evidence is the ledger rows and the published raw cells", needs: VB-25}

## Wave H — the custodian over harness memory (R8)

- [ ] `fathom config audit` classifies code-fact-shaped sentences in every auto-loaded file — CLAUDE.md, AGENTS.md, the harness's memory files — as anchored-fresh, drifted (with the receipt), or unanchorable, behind a flag until precision ≥ 0.8 on twenty real files {id: VB-28, symbol: auditCodeFactSentences, package: internal/custodian}
- [ ] (op) M14 written from a hand label over ~100 sentences from ≥ 20 files across two harnesses {id: VB-29, skip_test: true, skip: "labelling run; evidence is the labelled set under testdata/custodian/ and the report", needs: VB-28}

## Wave I — across the process boundary, and out of the index (R7, R9)

- [ ] A `service_ref` edge kind keyed on route template and method, grown from the existing HTTP-client detections and `net/http`, chi, gin and echo handler registrations, joined across repositories in the fleet store, tier `heuristic`; `blast_radius`, `cross_repo_blast_radius` and `review_diff` walk it {id: VB-30, symbol: ServiceRefEdges, package: internal/analysis}
- [ ] (op) M8: precision ≥ 0.90 and recall ≥ 0.90 on a hand-verified route table over the fixture fleet and one real pair, else it stays a finding {id: VB-31, skip_test: true, skip: "hand-verified route table; evidence under testdata/ and the ledger row", needs: VB-30}
- [ ] `fathom export scip` writes the symbol and typed-edge tables as a SCIP index carrying the resolution tier, round-tripped against scip-go on the fixture; `ingest --scip` is a later, separate decision {id: VB-32, symbol: cmdExportSCIP, package: cmd/fathom}

## Wave J — tests that adjudicate (M2)

- [ ] `fathom tests select --since <ref>` ranks tests for a diff by typed-edge reachability, statement coverage over the changed lines and layer, emitting a `go test -run` invocation and the changed symbols no test reaches {id: VB-33, symbol: cmdTestsSelect, package: cmd/fathom}
- [ ] A mutation report (go-mutesting first) is ingested as a code fact and the verify gate distinguishes covered from covered-and-asserted {id: VB-34, symbol: ParseMutationReport, package: internal/analysis/coverage}
- [ ] (op) M12b and M2 from a replay of ~100 historical commits: selection recall ≥ 0.98 at ≤ 25 % of full-suite time, else advisory only {id: VB-35, skip_test: true, skip: "replay spend; evidence is the replay report", needs: VB-33}
