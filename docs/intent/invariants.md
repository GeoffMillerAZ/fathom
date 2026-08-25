# Invariants

Statements that must always or never be true, each worded as the code actually makes it
true on 2026-08-25, with what enforces it. Rungs: `gate` (pre-push/`task validate`
refuses), `test <Name>` (runs in `go test -race ./...`), `lint` (a check exists but no
gate runs it), `code` (structure makes violation loud, no test pins it), `stated`
(nobody checks; agents must), `VIOLATED` (the rule as written is false today). Every
row was verified by reading the named artifact; do not upgrade a rung without doing the same.

## Index, store, and extraction

| # | Invariant | Enforced by | Source |
|---|---|---|---|
| I1 | Query and analysis code injects clocks (`query.WithNow`, `review.Now`); sorted outputs are pinned on four paths (FindCallers, packagedeps, platformlink, platform graph). No lint covers map-order or `time.Now()` elsewhere. | test `TestDeterminism` ×4 (`pkg/client`, `internal/analysis/packagedeps`, `internal/platformlink`, `pkg/platform`) | `internal/CLAUDE.md`, ADR-012 |
| I2 | `internal/store` is pure Go: builds and passes `go test -race` under `CGO_ENABLED=0`. | `stated` — true by hand on 2026-08-25; no task or gate runs it | ADR-025 |
| I3 | `LocalConnector` emits root-relative anchors; an ingest self-heals legacy absolute anchors. The store does not reject an absolute ref at write time. | test `TestStorePortability_RoundTrip`, `TestIngest_SelfHealsLegacyAbsoluteAnchors` | ADR-017 |
| I4 | Every `Engine.Query` result carries a `StalenessReport`; the other tools defer to `staleness_report`. | test `TestQuery_StalenessReport_*` (5), `TestHybrid_StalenessAndContradictionsStillPresent` | ADR-008 |
| I5 | `Engine.Query` rejects an empty `ConcernID`. The concern is a parameter, not a filter, on unscoped stores, and 55 of 58 tools never pass through the engine. | test `TestQuery_EmptyConcernIDReturnsError` | ADR-006; `questions.md` Q12 |
| I21 | MCP tools hold no memoized derived graphs; every answer is recomputed from the store (`internal/resultcache` has zero importers). | `code` | ADR-010's cache clause is dead code |
| I23 | A function referenced as a value is a call edge; calls inside closures count for the enclosing test. The typed pass is proven alone; the syntax pass only through the combined graph. | test `TestFuncValueReferencesAreEdges`, `TestClosureCallsReachFromEnclosingTest`, `TestReachabilityFixtureEndToEnd` | VS-6/7/10 |
| I24 | Typed edges dropped for out-of-scope targets are counted (`DiscoveryGaps`) and the file is named on stderr. The ingest summary line does not carry them. | test `TestTypedEdgesNameDroppedOutOfScopeTargets` (stderr text unasserted) | VS-9 |
| I25 | A file the extractor cannot read is counted and named; an ingest never overwrites a populated index with an empty one. | **VIOLATED** — IT-14/15 open; the pipeline aborts on the first fetch error only | O16; `scope.md` G1 |
| I28 | `store.BackendForPath` refuses a `.duckdb` path with an error naming it; a `.sqlite` sibling wins a same-stem collision with a warning. | test `TestBackendForPath_DuckDBLoudlyRefused`, `TestOpenPlatform_LoneDuckDBFileErrorsLoud`, `TestResolveStoreFiles` | ADR-025 |
| I30 | Service resolution is a total order (symbols desc, basename affinity, name asc); dedup never collapses; daemon-side ingest refuses a root another service claims. The CLI `ingest --store` path bypasses that guard. | gate — plan rows IO-1..3 `reachable_from` `TestResolveFromServicesDeterministic`, `TestIngestHygiene`, `TestRemoveRenameService` | O1; `incidents.md` INC-3 |

## Claims and semantic layers

| # | Invariant | Enforced by | Source |
|---|---|---|---|
| I6 | Semantic claims live in additive tables; compile supersedes by status flip and never deletes (`RemoveService` and `PurgePaths` are the only deletion paths). | test `TestCompile_SupersedesReplacedClaims`; store contract `claims_set_status` | ADR-016 |
| I7 | Every ingest re-hashes citations; stale and contradicted claims are served with their status, never silently. A sweep error is swallowed (`_, _ =`). | test `TestSweep_FlipsAndHeals`, `TestObligationsFor_ScopedOrderedFlagged`, `TestExpertModule_NonGitCorpus_EndToEnd` | ADR-016 |
| I26 | The memory write path derives tier from evidence and caps confidence at the tier ceiling; a lying caller lands at the ceiling with the cap recorded. Compile, review, and extractor writers carry no confidence and no ceiling logic. | test `TestCeilingDerivedNotTrusted`, `TestCapConfidenceBoundsEveryTier` | AM-2/AM-11 |
| I27 | Compile rejects a claim whose citation lies outside its section or does not resolve, whose scope matches no indexed file, or is over-broad; hash drift is the sweep's job. Scope-trap evals skip unless `FATHOM_EVAL_LLM=1`. | test `TestCompile_RejectsBadAnchorsAndScopes`, `TestCompile_RejectsOverBroadScope` | ADR-016 |
| I35 | Obligations: ≤6 rules per edit, each ≤28 words at compile; spec lines are dropped once 150 tokens are spent; zero output on a clean file. The ≤150-token ceiling on served rules is not enforced (live payloads measure 210–350 tokens). | test `TestObligationsFor_ZeroOutputOnClean`, `TestEditHookSilentOnCleanFile`, `TestObligationsFor_CapsAtMax`; ceiling clause **VIOLATED** | ADR-016; `questions.md` Q13 |

## Surfaces, hooks, and the daemon

| # | Invariant | Enforced by | Source |
|---|---|---|---|
| I8 | `fathom verify` is static (`runtime_pass` always Unknown); exit 0/1/2; pre-push blocks on 1 and allows on 2; `scripts/ci.sh --strict` fails closed but nothing invokes it. | gate lefthook pre-push `verify`; test `TestCmdVerify_*` (10), `TestVerifyPlan_Verified` | ADR-018, §5.3 |
| I9 | Session, edit, and stop hooks exit 0 on any infrastructure failure; a down daemon yields a one-line notice (session) or silence (edit, stop). | test `TestSessionContext_DaemonDown`, `TestEditHookSilentOnDaemonError`, `TestRunStopHook_AdvisoryAndBlock` | §5.3 |
| I10 | `serve-http` binds `127.0.0.1:7575` absent `[daemon].addr`; an off-surface call executes and is flagged `off_surface:true`; `SetStrictSurfaces` restores the hard gate. | `code` (default address); test `TestServeHTTPOffSurfaceExecutesAndFlags`, `TestCompanionSurfaceGatesToolsCall` | ADR-014 amendment |
| I11 | Every registered tool name appears in `docs/mcp-tools.md` and every documented name is registered. | test `TestMcpToolsDocMatchesRegistry` | SP-B2 |
| I12 | `fathom thread brief` output is ≤2,400 bytes; `--source resume\|compact` prints nothing. The session hook's own source gate and total injected payload are untested. | test `TestBriefBudgetAndSourceGating` | vision 21, TH-3 |
| I14 | No verb or hook sends data off-host without explicit configuration (`compile` needs a provider and key). The research bundle is the only artifact designed for sharing: CLI-only, closed CUE allowlist, secret-shaped strings never leak. | test `TestBuild_NeverLeaksSecretShapedServiceNames`, `TestCmdResearchBundle_Adversarial_SecretsNeverLeak`, `TestResearchBundleSchema_Negatives`; the no-egress clause is code review only | law 13 |
| I22 | A `{…}` annotation block yielding zero keys but containing a contract keyword lints loud; quoted commas and braces never terminate a block. A partially malformed block drops keys silently. | test `TestQuotedBracesDoNotTerminateAnnotation`, `TestMalformedAnnotationLintsLoudly`, `TestLintPlan_UnparseableAnnotationIsLoud` | O4, VS-1 |
| I29 | `fathom purge` reports a tri-state verdict mapped to exit 0/2/1; `read_source` re-reads suppressions per call (`symbol_context` under suppression is untested). | test `TestPurgeVerdictTable`, `TestPurgeBytesUncheckedExitsDistinct`, `TestDiskReadingToolsHonorSuppression` | phase V |
| I32 | Call log, hook beats, session ledger, and daily rollups carry `schema_version` and `fathom_version`; threads, leases, purges, memories, suppressions, and the verify ledger do not. | test `TestCallLogVersionStamp`, `TestBeatVersionStamp`, `TestLedgerAttribution`, `TestRollupVersionStamp` | IO-11..13 |
| I33 | An A/B unit is an arm-covered session or window with ≥10 turns; truncated windows are excluded and named; the transcript summary carries every parser field by construction. The ≥20-units-per-arm floor is code only. | test `TestArmUnitAdmission`, `TestImpactAdmitsWindowedUnits`, `TestUncleanArmRowsAreExcludedAndNamed`, `TestTranscriptSummaryCarriesEveryField` | MI-4/5 |

## Wiring, gates, and repo hygiene

| # | Invariant | Enforced by | Source |
|---|---|---|---|
| I13 | Law 15: fathom writes only inside owned regions, stops on drift, writes a receipt. True of `internal/wiring`; `fathom init` and `fleet reconcile --apply` bypass it: they replace the marker region on drift and write no receipt. | **VIOLATED** on the production path; engine pinned by `TestDriftStopsAndReportsNeverMerges`, `TestReceiptCarriesPriorContent` | vision 20 §6; `incidents.md` INC-12 |
| I15 | `third_party/agentic-kit` matches the pinned commit per file (sha256); vacuous without the sibling checkout. | gate `task vendor-check` (in `task validate`) | ADR-019 |
| I16 | Agent-config files never instruct a raw-`cp` deploy; `task deploy` signs when the identity exists, temp+renames, and asserts the daemon serves HEAD. | lint `fathom config audit` (deploy_safety; no gate runs it; corpus is `CLAUDE.md`/`AGENTS.md` only) | O25; `incidents.md` INC-13 |
| I17 | A `[x]` row in a depth-1 `docs/plans/*.md` blocks push when verify exits 1. Exit 2, a missing `bin/fathom`, Unknown criteria, or `LEFTHOOK=0` let it through; `archive/` is excluded by the glob. | gate lefthook pre-push `verify` | `docs/CLAUDE.md` |
| I18 | Owned Go source is gofmt-clean; every `schema.cue` package vets with its testdata when `cue` is installed; every depth-1 `scripts/*.sh` parses. | gate `task validate` (fmt-check, cue-vet, scripts-check); pre-commit | `Taskfile.yml`, `lefthook.yml` |
| I19 | No third-party or customer org names, hosts, or catalogs in code, docs, tests, or fixtures. Sibling-project names appear as fixture values (`serviceid_test`, `symbolnav_test`, `extra_tools_test`) and the launchd label is hardcoded in `doctor.go`. | **VIOLATED** (letter of ADR-011); nothing scans | ADR-011 |
| I20 | `internal/connector` never imports `internal/query`; `internal/extractor` never imports `internal/mcp`; `internal/analysis/*` import neither `mcp` nor `store`. Clean today. | lint `.fathom/rules/archconformance.cue` via advisory `fathom review` (function-origin edges only; `findings`, `review` packages missing from its list); no import-graph test | ADR-001 |
| I31 | Project-emitted artifacts default to `workspace/` (gitignored); `bin/fathom`, `./fathom`, Go scratch, and `.fathom/` local state are sanctioned exceptions. CLI `-store` / `-call-log` defaults are cwd-relative (O11/O12 open). | **VIOLATED** in part; `.gitignore` plus one pinned default (`TestInterventionsDefaultsStorePath`) | `CLAUDE.md`, `docs/usage.md` |
| I34 | `CLAUDE.md` ≤120 lines; every agent-config pointer resolves; the gate command is documented and exists. | lint `fathom config audit` (doctor row, session brief; no gate runs it) | `.fathom/concerns.cue` |
| I36 | One doc per use case under `docs/use-cases/` whose `## Maintained` line names current tools. Four lines still name the removed `set_profile`. | **VIOLATED**; nothing reads the lines | `docs/CLAUDE.md` |

Revisit when: ADR-006 is reaffirmed or superseded (I5); `internal/wiring` becomes the
only production write path (I13); IT-14/15 land (I25); a serving-side token cap exists
(I35); a denylist test exists (I19); a `## Maintained` parser test exists (I36).
