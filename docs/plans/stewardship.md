# Stewardship: the plan (ADR-060)

Grammar v2 (`docs/plans/README.md`). A row flips to `[x]` only when
`fathom verify --service fathom --plan docs/plans/stewardship.md` proves it. Vision:
`docs/vision/41-stewardship.md` (section numbers below are its). Decision: ADR-060. Futures: F58 (upstream drift of
vendored and pinned dependencies) and F59 (promises as code). Goal G34. Invariant I63.

Every reading here is $0 (no provider or agent spend). No row adds a table, a cache or a background job (ADR-049
decision 2). Nothing starts before the owner lifts the wind-down of 2026-09-30. A reading's pattern accepts either
verdict; the change that records a CUT also marks the rows it gated `[-]` with a refused disposition. The deferred
and refused rows at the end are recorded so they are not re-proposed.
2026-10-01: the wind-down is treated as lifted (language waves la, lb and lc have run since); wave cd starts the
lines, leftovers and used-closure slices (SV-1, SV-2, SV-4 to SV-7) and the $0 readings SV-M1 and SV-M3.

## Wave 1: lines (section 4.1)

- [x] Lines of one repository (local branches, linked worktrees, stashes) are classified landed (an ancestor, patch-equivalent or content-equal to the integration branch), at risk (dirty, stashed, or commits on no remote), idle or active, each with its age; the claims-rows and touches-rows classes were removed when SV-M1 read CUT (ADR-060 amendment 2026-10-01) {id: SV-1, symbol: ClassifyLines, package: internal/worklines, reachable_from: TestClassifyLinesLandedAtRiskIdle}
- [x] `fathom plans lines [--json]` prints every line's class and age, and for landed lines the removal command it would take; it runs no git write, deletes nothing and rewrites no history (I63) {id: SV-2, symbol: cmdPlansLines, package: cmd/fathom, reachable_from: TestCmdPlansLinesReportsAndNeverActs, needs: "SV-1"}
- [x] (op) SV-M1: the $0 replay over the 2026-09-29 backup bundles: both valuable held items and the unfinished worker read claims rows, touches rows or at risk; at least 9 in 10 lines the consolidation removed as merged read landed; at most 25 lines flagged for a look; bars fixed before the run {id: SV-M1, oracle: lexical, path: docs/bench/readings/stewardship/lines-replay.md, pattern: "^SV-M1 verdict: (KEEP|CUT)", needs: "SV-2", weight: 2}
- [-] The SessionStart attention line gains one count of unlanded lines that claim or touch open rows, from ancestry-only reads inside I12's cap and I53's deadline, silent at zero {id: SV-3, disposition: refused, reason: "SV-M1 read CUT on 2026-10-01 (36 lines flagged against a bar of 25): the claims and touches classes it would count are killed (vision 41 section 7)"}

## Wave 2: leftovers (section 4.3)

- [x] `fathom gc` reports untracked non-ignored files outside declared homes (`workspace/`, the ADR-039 build-cache roots) and executable binaries in the tree as report-only classes that `--apply` never removes (I63) {id: SV-4, symbol: gcStrays, package: cmd/fathom, reachable_from: TestGCStraysReportOnlyNeverApplied}
- [x] `fathom plans status` marks a plan with 0 open and 0 claimed-unproven rows archivable and names `fathom plans archive` {id: SV-5, symbol: planArchivable, package: cmd/fathom, reachable_from: TestPlansStatusMarksArchivable}
- [ ] (op) SV-M2: the owner adjudicates one strays and archivable list on this repository and on agentic-kit; at least 8 in 10 flagged items were trash or forgotten work {id: SV-M2, oracle: lexical, path: docs/bench/readings/stewardship/leftovers.md, pattern: "^SV-M2 verdict: (KEEP|CUT)", needs: "SV-4,SV-5"}

## Wave 3: inheritance (sections 4.2 and 4.5)

- [x] The used closure of a dependency directory from product entry points: Go packages by the import graph (equal to `go list -deps` of the main packages), TypeScript and JavaScript modules by import edges, declared asset roots (a Tailwind source list, `go:embed` patterns), each unit with its entry point and first importer, the rest listed unreached {id: SV-6, symbol: UsedUnits, package: internal/usedset, reachable_from: TestUsedUnitsGoMatchesListDepsAndAssetsDeclared}
- [x] `fathom report used <dir> [--json]` prints the closure with reasons and the unreached remainder, a sub-report beside `report evidence` and `report packs` (no new verb, no MCP tool) {id: SV-7, symbol: cmdReportUsed, package: cmd/fathom, reachable_from: TestCmdReportUsedJSONReasonsAndUnreached, needs: "SV-6"}
- [x] (op) SV-M3: on fathom's own mirrors, Go units equal `go list -deps`, the CSS units equal `scripts/ui-css.sh`'s declared list, and app-kit's mirror pruned to the closure passes `task build` and `scripts/ui-css.sh --check-sources`; units, files and bytes before and after recorded {id: SV-M3, oracle: lexical, path: docs/bench/readings/stewardship/used-closure.md, pattern: "^SV-M3 verdict: (KEEP|CUT)", needs: "SV-7", weight: 2}
- [ ] (op) SV-M4: monthly for a quarter, each mirror's pin against its sibling checkout's HEAD: commits, changed files, changed files inside the used closure, and any licence-file change; nothing built, nothing fetched {id: SV-M4, oracle: lexical, path: docs/bench/readings/stewardship/pin-lag.md, pattern: "^SV-M4 reading:", needs: "SV-6"}
- [ ] (op) SV-V1: a named consumer for the licence boundary is recorded: a repository that ships publicly and consumes a licensed catalog, with the owner's explicit allow, if any, per licensed path (default deny: no allow means the path never ships) {id: SV-V1, oracle: lexical, path: docs/bench/readings/stewardship/licence-consumer.md, pattern: "^SV-V1 consumer: [A-Za-z0-9]"}
- [ ] The boundary plane: `.fathom/licensed.cue` names the licensed paths, their licence and the owner's explicit allows; default deny (ADR-060 amendment 2026-09-30): a pushed tree holding a declared licensed path with no allow is a loss that `fathom conserve --strict-plane boundary` blocks at pre-push; reached and allowed licensed units are listed as a fact; nothing licensed is ever inferred (the test name below predates the amendment and must assert default deny) {id: SV-8, symbol: boundaryPlane, package: internal/conserve, reachable_from: TestBoundaryPlaneBlocksNeverAndUnreachedLicensed, needs: "SV-6,SV-V1"}

## Deferred and refused (recorded so they are not re-proposed)

- [-] The upstream-moved attention line for vendored and pinned dependencies (F58) {id: SV-D1, disposition: deferred, reason: "upgrade impact (system-graph SG-39) does not exist yet and SV-M4 has not shown a reached change", trigger: "system-graph.md#SG-39 proven and SV-M4 shows at least one reached change in a quarter"}
- [-] Would prove instead of claims or touches: a scratch verify at each line's tip {id: SV-D2, disposition: refused, reason: "its trigger needed SV-M1 to read KEEP; SV-M1 read CUT on 2026-10-01 and the claims and touches classes it would replace were removed (ADR-060 amendment 2026-10-01)"}
- [-] Lines across the fleet and in scratch clones under operator-named roots {id: SV-D3, disposition: deferred, reason: "the single-repository report has not read", trigger: "a harvest finds valuable work in a scratch clone the report did not see"}
- [-] Documents that nothing live cites, as orphan candidates {id: SV-D4, disposition: deferred, reason: "needs the doc-to-code links of the knowledge-retrieval plan", trigger: "those links land and SV-M2 reads KEEP"}
- [-] Promises as code: `metric:` keys, dangling alerts and dashboards, routes with no SLO, observability conservation, readiness facts for Review Kit (F59) {id: SV-D5, disposition: deferred, reason: "no alert-rule, SLO-spec or dashboard file on this workstation (census 2026-09-30), so no consumer and no oracle (P20)", trigger: "a named consumer commits alert rules, SLO specs or dashboards, and system-graph.md#SG-2 and SG-M1 have landed"}
- [-] Fathom deletes, merges, squashes, rebases or checks out a line, or squashes and re-summarizes by default {id: SV-X1, disposition: refused, reason: "P15 and I63: not fathom's to write; history rewrite is ask-first; the dispatch boundary"}
- [-] Stewardship nudges in the Stop hook or before or after compaction {id: SV-X2, disposition: refused, reason: "the Stop hook runs near its deadline (I53); carry-across read STOP (CA-M1)"}
- [-] Polling upstream or checking registries for new versions {id: SV-X3, disposition: refused, reason: "P23 and I56; Renovate and Dependabot own it"}
- [-] Fathom copying, pruning or vendoring files {id: SV-X4, disposition: refused, reason: "a package manager and the transformation-engine non-goal; the copy stays a script reading SV-7's JSON"}
- [-] Inferring licensed-ness, judging derived-work similarity or interpreting a licence {id: SV-X5, disposition: refused, reason: "a legal judgment; the owner declares it; never the decision port"}
- [-] Live SLI values, burn rates, alert state or an organization-wide compliance dashboard in fathom {id: SV-X6, disposition: refused, reason: "connectors and hosted are non-goals (ADR-058); Review Kit's DATUM is the hosted view"}
- [-] A separate Reliability Kit {id: SV-X7, disposition: refused, reason: "Review Kit already has the reliability and operations domain; a third tool splits the family without an enforced boundary (P21)"}
- [-] A `Lifetime:` header for planning documents {id: SV-X8, disposition: refused, reason: "citations by live plans and records already compute it at no authoring cost"}
