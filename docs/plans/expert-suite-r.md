# Expert Suite Phase R — findings as evidence

Design (normative — owner-commissioned 2026-07-30; see
`docs/vision/13-expert-suite.md` §9's row **R**): `docs/design/findings-as-
evidence.md` §6 deliverables 1-8. §4's honesty rules and §5's non-goals are
binding on every row below. Grounding docs: `docs/dogfood-observations.md`
O20 (the count bug fixed first, item 1) and O19 (the rejection-cause
precedent item 2's vocabulary mirrors), `docs/design/product-vision.md` §3
(the incident-investigation entry condition item 7 reports against),
`docs/vision/13-expert-suite.md` §8 laws 11/12.

**Anchor lessons carried from phases P/Q (binding on every row below).**
1. FUNCTIONS/METHODS only as `symbol:` anchors — never struct/type names,
   never a method name shared by two receivers in the same package (the
   `ClaimEventsSince`/`RecordCompileRun`/marshal-helper class of collision
   phase Q hit twice — `grep -rn "^func.*<Name>("` across `internal/`,
   `cmd/fathom/`, `pkg/` before anchoring each row below; zero collisions
   found for every symbol anchored here). `core.FindingClass.Valid()` is a
   live instance of this trap (it would collide with
   `core.ClaimStatus.Valid()` and `core.EvidenceTier.Valid()`, all three in
   `internal/core`) — no row anchors to `Valid`; the vocabulary's
   enforcement point (`verifyDraft`) is used instead.
2. Annotations sit ON THE CHECKBOX LINE ITSELF — a continuation-line
   annotation is silently dropped by `verify.ParseMarkdownPlan` and the row
   parses as contract-less.
3. Doc-only rows carry `skip_test: true` with a `skip:` reason naming the
   actual acceptance evidence, never a fabricated `Test*` claim.

## Key engineering decisions this plan pins

- **O20's root cause is a call-log AGGREGATION bug, not a store count bug.**
  `research_report.go`/`research_bundle.go` never open a store (by design —
  see `research_report.go`'s own header comment); `review_active_findings`
  summed every "review" call-log entry's `verdict_outcome` (a per-run
  SNAPSHOT of active findings, `review.reviewVerdictOutcome`) across every
  call in the window instead of reporting the most recent one. A repo with
  many pushes summed many identical snapshots — `LatestReviewActiveFindings`
  (`internal/metrics`) fixes it by reporting the latest snapshot only, with
  a new honest `review_active_findings_note` disclosing which call it's "as
  of". Pinned by a 50-call fixture whose pre-fix sum (12,650) and post-fix
  value (253) are both asserted directly.
- **The class vocabulary is Go-authoritative, CUE-cross-checked.**
  `internal/core.ValidFindingClasses` is the single source of truth (mirrors
  `internal/analysis/review`'s own "Go mirrors CUE, a test cross-checks"
  convention for `ClaimsLayer`/`ClaimsKind`); `schema/semantic/
  findingclass.cue`'s `FindingClasses` list derives `#FindingClass` via
  CUE's `or()` builtin, and `TestFindingClassVocabularyMatchesSchema`
  (`internal/semantic`) fails the moment the two lists diverge. Enforcement
  lives in ONE place, `verifyDraft` (`internal/semantic/compile.go`) for the
  LLM-compiled corpus — a fifth `rejectionCause`, `invalid-class`, alongside
  O19's four — and deterministically in `internal/analysis/review`'s
  dimension functions (`runArch` always `conformance`, `runSpecCoverage`
  always `test-gap`) for the review-verb bridge, carried through
  `AttachClaims` onto the persisted claim's new `Class` field.
- **Pattern derivation's grouping key is (class, `NormalizeRuleShape(rule)`),
  stable across sites.** A finding's raw `Rule` text names the SPECIFIC
  site (which symbol, which spec ID) — grouping on the literal string would
  never recur. `NormalizeRuleShape` strips the arch-violation bridge's own
  "`<from>` → `<to>`: `<reason>`" shape down to `<reason>`, replaces
  bracketed spec IDs/quoted tokens/path-shaped tokens/bare numbers with
  placeholders, lowercases, and collapses whitespace — deliberately coarse,
  same philosophy as the class vocabulary itself (§3.2). `DerivePatterns`
  groups by that key and keeps only groups spanning >=2 DISTINCT `Subject`
  values (sites) — computed fresh on every call, never persisted (§4
  honesty rule 2).
- **Resolution outcomes: three cases, two detected at write time, one
  already recorded.** `internal/analysis/review.ClassifyResolution`
  distinguishes code-change-in-scope (every scoped file still exists) from
  scope-disappearance (one was deleted) at the moment
  `recordReviewSupersedeEvents` (`cmd/fathom/review.go`) writes a
  "superseded" `claim_event` — two NEW fields, `ResolutionKind`/
  `ResolutionCommit` (`core.ClaimEvent`), additive-migrated onto
  `claim_events` (`resolution_kind`/`resolution_commit` columns, the
  escape_note/rejection_reasons_json precedent). The THIRD case,
  refutation, needs no new field: `pkg/client.ClaimFeedback` already writes
  `ClaimEvent{Verdict:"refuted"}` on every `fathom claim refute` — resolution-
  outcome derivation (`DerivePatterns`, `BuildStatus`) reads `Verdict` for
  that case and `ResolutionKind` for the other two, never conflating them.
- **The promotion rule is gated BEHIND the eval gate, structurally, not by
  caller discipline.** `PromoteTier` (`internal/analysis/findings`) takes
  `layerEvaled bool` and refuses to promote — returning the base tier
  unchanged with an explicit note — whenever it is false, regardless of
  resolved-member count; only once >=3 members carry independent, recorded
  code-change resolutions AND the layer has passed its eval gate does a
  pattern rise its one rung (`sme-stated`/`doc-claimed` -> `verified`, the
  only ladder findings-kind claims ever populate). Recomputed and reversible
  by construction (no persisted "promoted" flag exists to un-set) — proven
  directly by `TestPromoteTier_Reversible`. The findings layer's own
  `eval/{labels.json,corpus.md}` now exist (6 labeled findings across 6
  classes + 2 traps, mirroring adr-rules' fixture shape) and
  `TestFindingsEval_Precision` is wired, but **has not been run against a
  real model in this change** — no API key/CLI session was available; see
  `schema/semantic/layers/findings/eval/RESULTS.md` for the honest "not yet
  run" record and the exact command. `layer.cue`'s `evaled` stays `false`.
- **`findings near`'s traversal reuses `internal/analysis/callgraph`
  verbatim — no new storage.** `FileNeighborhood` builds one
  `callgraph.Graph` and walks `CallersDetail`/`CalleesDetail` bidirectionally
  (a finding is relevant whether the subject calls it or is called by it),
  bounded by `--hops`; `RankNear` ranks by ascending hops then descending
  `CompiledAt`, excluding superseded/refuted claims unless
  `--include-superseded`, always labeling them (`Superseded: true`) when
  included (§4 honesty rule 4). The CLI (`fathom findings near`) and the
  MCP tool (`findings_near`, `internal/mcp/findings_tools.go`) share the
  identical pure core (`buildFindingsNearReport`/`RankNear`) — one
  implementation, two surfaces. `findings_near` is registered `group:
  GroupInvestigation` ONLY: `TestFindingsNear_ToolIsOnInvestigationSurfaceNotAgentSurface`
  proves it is absent from `SurfaceAgent`/`SurfaceCompanion` — no net
  agent-surface tool growth, per the mission's own constraint.

## Tasks

### A — O20's count audit + fix (docs/design/findings-as-evidence.md §6 item 1, BLOCKING)

- [x] `LatestReviewActiveFindings` reports the most recent review call's own active-finding snapshot, never a sum across the window — pinned against a 50-call fixture of known size {id: R-1, symbol: LatestReviewActiveFindings, package: internal/metrics, reachable_from: TestAggregateBundleInput_HashesServiceNames}
  - `internal/metrics/reviewfindings.go`: `ReviewCallSnapshot{At,
    Outcome}`, `LatestReviewActiveFindings(snapshots
    []ReviewCallSnapshot) (count, sampleSize int, ok bool)`.
  - Tests (`internal/metrics/reviewfindings_test.go`):
    `TestLatestReviewActiveFindings_FixtureOfKnownSize_ReportsLatestSnapshotNotSum`
    (50 snapshots, asserts the exact 253 vs. the pre-fix 12,650 sum),
    `_LatestSnapshotWinsRegardlessOfInputOrder`, `_EmptyInput_NotOK`,
    `_SingleCall_ReportsItsOwnSum`.

- [x] `fathom research report` wires the fix and adds an honest `review_active_findings_note` disclosing which call the number is "as of" {id: R-2, symbol: buildResearchReport, package: cmd/fathom}
  - `cmd/fathom/research_report.go`: the call-log loop collects
    `metrics.ReviewCallSnapshot` per "review" row instead of summing
    inline; `ResearchReport.ReviewActiveFindingsNote` (new field) is set
    from `LatestReviewActiveFindings`'s `ok`/`sampleSize`.
  - `cmd/fathom/research_report_render.go`: renders the new note line.
  - Tests (`cmd/fathom/research_report_test.go`):
    `TestBuildResearchReport_ReviewActiveFindings_O20Fix` (40-call
    fixture, asserts 253 not 40*253),
    `_ReviewActiveFindings_NoReviewCalls_HonestNote`.

- [x] `fathom research bundle` gets the identical numeric fix (the bundle's own `ReviewActiveFindings` field, unchanged JSON shape) {id: R-3, symbol: aggregateBundleInput, package: cmd/fathom}
  - `cmd/fathom/research_bundle.go`: same snapshot-collection change as
    R-2; `bundle.Input.ReviewActiveFindings` now receives
    `LatestReviewActiveFindings`'s count.
  - Tests: covered by the existing `TestAggregateBundleInput_*` suite
    staying green plus R-1's own direct coverage of the shared fix
    function (no second numeric bug class to pin — the aggregation
    change is identical code shape to R-2, reviewed for behavior parity).

### B — typed failure classes (docs/design/findings-as-evidence.md §6 item 2)

- [x] The closed finding-class vocabulary: Go-authoritative (`core.ValidFindingClasses`), CUE-derived (`or()` over the same list), cross-checked by a test that fails the moment the two diverge {id: R-4, symbol: verifyDraft, package: internal/semantic, reachable_from: TestCompile_FindingClass_Unknown_Rejected}
  - `internal/core/findingclass.go`: `FindingClass` + 10 consts +
    `ValidFindingClasses` + `Valid() bool`.
  - `schema/semantic/findingclass.cue`: `FindingClasses` list,
    `#FindingClass: or(FindingClasses)`.
  - `internal/semantic/compile.go`: `claimDraft.Class`
    (`json:"class,omitempty"`), `claimDraftSchema` gains the `class`
    property, `causeInvalidClass` (fifth `rejectionCause`), `verifyDraft`
    validates `d.Class` against `core.FindingClass.Valid()` for
    `kind==KindFinding` ONLY — empty or unrecognized is a compile-time
    rejection, never silently bucketed.
  - `schema/semantic/layers/findings/prompt.md` (+ its embedded mirror,
    `internal/semantic/layers/findings/prompt.md`): instructs the
    reviewer to emit `class` from the closed vocabulary.
  - Tests: `internal/semantic/schema_test.go`'s
    `TestFindingClassVocabularyMatchesSchema`,
    `TestFindingClassSchema_UnknownClassRejected`;
    `internal/semantic/compile_test.go`'s
    `TestCompile_FindingClass_Valid_Persisted`,
    `_Unknown_Rejected`, `_Empty_Rejected`,
    `_NotEnforcedForNonFindingKind`;
    `TestFindingsPromptVersion_Pinned` bumped (a deliberate prompt-identity
    rotation, not drift).

- [x] The review-verb bridge assigns class deterministically at emit time (never an LLM step in this path) and carries it onto the persisted claim {id: R-5, symbol: AttachClaims, package: internal/analysis/review}
  - `internal/analysis/review/engine.go`: `Finding.Class`; `runArch`
    always emits `core.FindingClassConformance`; `runSpecCoverage` always
    emits `core.FindingClassTestGap`.
  - `internal/analysis/review/claims.go`: `AttachClaims` copies
    `f.Class` onto the minted `core.SemanticClaim.Class`.
  - Tests (`internal/analysis/review/review_test.go`):
    `TestRun_Arch_ReportsViolationsWithLineCitation` (extended with a
    Class assertion), `TestRun_SpecCoverage_FlagsUncoveredRefWithLineCitation`
    (same), `TestAttachClaims_OnlyOracleDimensionsPersist` (same).

- [x] `class` persists additively on `semantic_claims` (a pre-existing store predates the column) {id: R-6, symbol: scanClaims, package: internal/store, reachable_from: TestAdoptionServiceScoped}
  - `internal/store/sqlite.go`: `class TEXT NOT NULL DEFAULT ''` on the
    CREATE TABLE shape AND the conditional-migration list;
    `PutClaims`/`ClaimsForService`/`scanClaims` read/write it.
  - Tests (`internal/store/contract_test.go`):
    `finding_class_round_trips` (both SQLite and MemoryStore, via
    `storeContract`); `internal/store/sqlite_test.go`'s
    `TestSQLiteStore_ClaimsForService_LegacyRowMissingClassColumn_DegradesToEmpty`,
    `TestScanClaims_RoundTripsClass` (calls `scanClaims` DIRECTLY against a
    real `*sql.Rows` — `ClaimsForService`, `scanClaims`'s only caller, is
    itself an ambiguous bare method name at the syntax-only resolver's
    `byName` index, implemented by both `SQLiteStore` and `MemoryStore`;
    `test_reaches` cannot resolve a hop through it even into an
    otherwise-unique target, so the anchor's own test calls the free
    function by name directly — the same fix Q-3 used).

### C — pattern derivation (docs/design/findings-as-evidence.md §6 item 3)

- [x] `NormalizeRuleShape` reduces a finding's Rule text to a coarse, deterministic grouping shape — per-site identifiers placeholder'd, the failure-mode language preserved {id: R-7, symbol: NormalizeRuleShape, package: internal/analysis/findings}
  - `internal/analysis/findings/pattern.go`: `NormalizeRuleShape(rule
    string) string`.
  - Tests (`internal/analysis/findings/pattern_test.go`):
    `TestNormalizeRuleShape` (table: arch-violation reason extraction,
    spec-ID placeholder, quoted/numeric placeholders),
    `_SameRuleDifferentSites_Identical`.

- [x] `DerivePatterns` groups claims by (class, normalized rule shape) across >=2 distinct sites, computed fresh on every call — never a second source of truth {id: R-8, symbol: DerivePatterns, package: internal/analysis/findings}
  - `internal/analysis/findings/pattern.go`: `MemberStatus`,
    `PatternMember{ClaimID,Subject,Scope,Status,ResolutionKind,
    ResolutionCommit,CompiledAt,CompiledAtWatermark}`,
    `Pattern{Class,RuleShape,Members,FirstSeenAt,LastSeenAt,
    FirstSeenWatermark,LastSeenWatermark}`,
    `Pattern.ResolvedCodeChangeCount`/`SiteCount`,
    `DerivePatterns(claims, events) []Pattern`.
  - Tests (`internal/analysis/findings/pattern_test.go`):
    `TestDerivePatterns_GroupsAcrossDistinctSites`,
    `_SingleSite_NoPattern`, `_DifferentClassNeverMerges`,
    `_ResolutionAndStatus_MemberFieldsPopulated`, `_Deterministic`,
    `_EmptyInput_EmptyOutput`.

### D — resolution outcomes at reconciliation (docs/design/findings-as-evidence.md §6 item 4)

- [x] `ClassifyResolution` distinguishes code-change-in-scope from scope-disappearance from data already in hand (the claim's own Scope plus the current source set) {id: R-9, symbol: ClassifyResolution, package: internal/analysis/review}
  - `internal/analysis/review/claims.go`: `ClassifyResolution(scope
    []string, fileExists func(string) bool) string`, returning
    `core.ResolutionCodeChange` or `core.ResolutionScopeDisappearance`.
  - `internal/core/claims.go`: `ClaimEvent.ResolutionKind`/
    `ResolutionCommit` (additive fields, set ONLY on "superseded"
    events); `core.ResolutionCodeChange`/`ResolutionScopeDisappearance`
    consts. Refutation needs no new field (already `Verdict:"refuted"`).
  - Tests (`internal/analysis/review/review_test.go`):
    `TestClassifyResolution_AllScopedFilesExist_CodeChange`,
    `_ScopedFileDeleted_ScopeDisappearance`,
    `_NilFileExists_ConservativelyScopeDisappearance`.

- [x] `fathom review`'s supersede-event write path classifies and records the resolution at the moment it reconciles a fixed finding {id: R-10, symbol: recordReviewSupersedeEvents, package: cmd/fathom}
  - `cmd/fathom/review.go`: `recordReviewSupersedeEvents` gains a
    `resolve func(claimID string) (kind, commit string)` parameter
    (nil-safe); `runReview` builds it from the pre-run `existing` claims'
    own Scope, `review.ClassifyResolution`, and the current source set,
    with the review run's watermark as the commit.
  - Tests (`cmd/fathom/review_test.go`):
    `TestRecordReviewSupersedeEvents_WritesOneEventPerID` (extended with
    resolution-field assertions), `_NilResolve_EmptyResolutionFields`;
    `TestReview_Reconcile_FixedFindingSuperseded` (extended: the e2e
    fixture's fix EDITS a file rather than deleting it, so the recorded
    event must carry `ResolutionKind=="code-change"` and a non-empty
    `ResolutionCommit`).

- [x] `resolution_kind`/`resolution_commit` persist additively on `claim_events` {id: R-11, symbol: queryClaimEventsSince, package: internal/store, reachable_from: TestCmdFindingsStatus_JSON}
  - `internal/store/sqlite.go`: two new columns on the CREATE TABLE shape
    AND the conditional-migration list;
    `AppendClaimEvent`/`ClaimEvents`/`ClaimEventsSince`/
    `queryClaimEventsSince` read/write both.
  - `pkg/client/semantic.go`: `Client.ClaimEventsSince` (new thin
    passthrough — pattern derivation and `findings status`/`near` need
    the bulk claim_events read `AppendClaimEvent`'s existing wrapper
    doesn't provide).
  - Tests (`internal/store/contract_test.go`):
    `claim_event_resolution_kind_round_trips` (both backends);
    `pkg/client/semantic_test.go`'s `TestClient_ClaimEventsSince_RoundTrips`.

### E — eval gate + promotion rule (docs/design/findings-as-evidence.md §6 item 5)

- [-] `PromoteTier` applies the promotion rule, gated structurally behind the layer's eval status — recomputed and reversible, never sticky {id: R-12, symbol: PromoteTier, package: internal/analysis/findings, disposition: refused, reason: "PromoteTier has no production caller: blast_radius kind delete at 9ea6da5f reads 0 typed referents and only its six TestPromoteTier_* tests in internal/analysis/findings/tier_test.go reach it, so the findings layer never promotes a tier and the rule is unbuilt in any product path"}
  - `internal/analysis/findings/tier.go`: `PromoteTier(base
    core.EvidenceTier, p Pattern, layerEvaled bool) (tier
    core.EvidenceTier, note string)` — refuses whenever `!layerEvaled`
    OR `ResolvedCodeChangeCount() < 3`; promotes exactly one rung
    (`sme-stated`/`doc-claimed` -> `verified`) otherwise.
  - Tests (`internal/analysis/findings/tier_test.go`):
    `TestPromoteTier_LayerNotEvaled_NeverPromotes`,
    `_BelowThreeResolved_NotPromoted`,
    `_ThreeResolved_LayerEvaled_Promotes`, `_AlreadyAtCeiling_NoRungAbove`,
    `_Reversible` (the §4 honesty-rule-3 direct proof),
    `_IndependenceRequiresSupersededStatus`.

- [x] The findings layer gets eval fixtures (6 labeled findings across 6 classes + 2 traps, mirroring adr-rules' shape) and a wired (currently skipped, not-yet-run) gate test {id: R-13, symbol: SplitMarkdownSections, package: internal/semantic, skip_test: true, skip: "the eval gate itself (TestFindingsEval_Precision) needs a real model and was not run this change (no API key/CLI session available) — RESULTS.md records this honestly; the fixture's STRUCTURAL validity (parses, cite_substrs resolve, splits into the expected section count) is proven instead by TestFindingsEvalFixture_LabelsLoadAndCiteSubstrsResolve, which always runs"}
  - `schema/semantic/layers/findings/eval/corpus.md`,
    `eval/labels.json`, `eval/RESULTS.md` (honest "not yet run" record +
    exact command).
  - `schema/semantic/layers/findings/KILL.md`: updated from "fixtures not
    yet seeded" to the current, accurate state.
  - `internal/semantic/eval_test.go`: `TestFindingsEval_Precision`
    (skipped without `FATHOM_EVAL_LLM=1`, matching adr-rules/
    spec-bindings exactly).
  - Test (`internal/semantic/findings_fixture_test.go`, ALWAYS runs, no
    tokens): `TestFindingsEvalFixture_LabelsLoadAndCiteSubstrsResolve`.

### F — `fathom findings near` (docs/design/findings-as-evidence.md §6 item 6)

- [x] The call-graph neighborhood traversal, bidirectional and hop-bounded — reuses `internal/analysis/callgraph`, adds no storage {id: R-14, symbol: FileNeighborhood, package: internal/analysis/findings}
  - `internal/analysis/findings/neighborhood.go`: `symbolNeighborhood`
    (unexported bidirectional BFS over `callgraph.CallersDetail`/
    `CalleesDetail`), `FileNeighborhood(nodes, typed, subject, maxHops)
    map[string]int` — a symbol key or a file path, expanding a file to
    every symbol it defines as hop-0 seeds; nil for an unresolvable
    subject.
  - Tests (`internal/analysis/findings/neighborhood_test.go`):
    `TestFileNeighborhood_BidirectionalHopDistances`,
    `_HopBudgetRespected`, `_SubjectIsFilePath_ExpandsToItsSymbols`,
    `_UnresolvableSubject_Nil`.

- [x] `RankNear` ranks findings by proximity then recency, excluding superseded/refuted unless asked, always labeling them when included {id: R-15, symbol: RankNear, package: internal/analysis/findings}
  - `internal/analysis/findings/neighborhood.go`:
    `NeighborhoodEntry{Claim,Hops,Superseded}`, `RankNear(claims,
    fileHops, includeSuperseded) []NeighborhoodEntry`.
  - Tests: `TestRankNear_ProximityThenRecency`,
    `_ScopeOutsideNeighborhood_Excluded`,
    `_SupersededExcludedByDefaultAlwaysLabeledWhenIncluded` (the §4
    honesty-rule-4 direct proof), `_EmptyInput_EmptyOutput`.

- [x] `fathom findings near <symbol|file> [--hops N] [--include-superseded] [--json]` {id: R-16, symbol: cmdFindingsNear, package: cmd/fathom}
  - `cmd/fathom/findings.go`: `cmdFindings` (dispatch: `near | status`),
    `cmdFindingsNear`, `openFindingsClient` (read-only
    `store.MountReadOnly`, mirroring `cmd/fathom/metrics_*.go`),
    `buildFindingsNearReport` (the pure core shared with the MCP tool),
    `findingsNearReport`/`findingsNearEntry` (the shared JSON shape).
  - `cmd/fathom/main.go`: `findings` subcommand wired + usage line.
  - Tests (`cmd/fathom/findings_test.go`):
    `TestCmdFindingsNear_JSON_RanksByProximity`,
    `_SupersededExcludedUnlessAsked`, `_UnresolvableSubject_HonestNote`,
    `_MissingFlags_ErrorExit`; `TestCmdFindings_UnknownVerb_ErrorExit`,
    `_NoVerb_ErrorExit`.

- [x] `findings_near` MCP tool on the INVESTIGATION surface only — no net agent-surface tool growth {id: R-17, symbol: registerFindingsNear, package: internal/mcp, reachable_from: TestAdoptionServiceScoped}
  - `internal/mcp/findings_tools.go`: `findingsBackend` (optional
    capability, `ClaimsForService`), `registerFindingsNear(s,
    backend)` — `group: GroupInvestigation`; degrades honestly
    (`found:true`, a "not measurable" note, empty findings) when the
    backend lacks `findingsBackend`.
  - `internal/mcp/extra_tools.go`: wired into `registerExtraTools`'s
    investigation block.
  - Tests (`internal/mcp/findings_tool_test.go`):
    `TestFindingsNear_ToolIsOnInvestigationSurfaceNotAgentSurface` (pins
    surface membership directly — present on bare `/rpc` and
    `investigation`, absent from `agent`/`companion`),
    `_RanksByProximityAndLabelsSuperseded`,
    `_UnresolvableSubject_HonestNotFound`,
    `_MissingRequiredArgs_ParamsError`.

### G — `fathom findings status` (docs/design/findings-as-evidence.md §6 item 7)

- [x] `BuildStatus` reports the incident-investigation module's entry condition from data — resolved-finding count, distinct classes, patterns with >=3 resolved members, against product-vision.md §3's 15-20 range {id: R-18, symbol: BuildStatus, package: internal/analysis/findings}
  - `internal/analysis/findings/status.go`: `EntryBarLow`/`EntryBarHigh`
    (15/20, product-vision.md §3's own range), `StatusReport` (JSON-
    tagged), `BuildStatus(claims, events) StatusReport` — "resolved"
    means an independent, recorded code-change resolution (the same
    definition `PromoteTier`'s own numerator uses); a refutation or
    scope-disappearance is not precedent and does not count.
  - Tests (`internal/analysis/findings/status_test.go`):
    `TestBuildStatus_ResolvedFindingCount_OnlyCodeChangeResolutionsCount`,
    `_DistinctClasses_Sorted`, `_EntryConditionThresholds` (14/15/19/20
    boundary table), `_PatternsWithThreeResolvedMembers`,
    `_EmptyInput_HonestZeros`.

- [x] `fathom findings status [--json]` {id: R-19, symbol: cmdFindingsStatus, package: cmd/fathom}
  - `cmd/fathom/findings.go`: `cmdFindingsStatus` — reads the service's
    `findings`-layer claims plus the store-wide `claim_events`
    population (disclosed: `claim_events` carries no `service_id`
    column), calls `findings.BuildStatus`.
  - Tests (`cmd/fathom/findings_test.go`): `TestCmdFindingsStatus_JSON`,
    `_MissingFlags_ErrorExit`.

### H — docs (docs/design/findings-as-evidence.md §6 item 8)

- [x] `docs/use-cases/incident-investigation.md` gains the neighborhood query (tool table row, a worked-example step 9, the `investigation` profile note, the Maintained line) {id: R-20, symbol: registerFindingsNear, package: internal/mcp, skip_test: true, skip: "documentation-only row; the tool it documents is proven by R-17's own tests"}

- [-] `docs/layers.md` records the findings layer's new (still not-gated) eval status and the class/pattern/promotion mechanics {id: R-21, symbol: PromoteTier, package: internal/analysis/findings, skip_test: true, skip: "documentation-only row; the mechanics it documents are proven by R-4/R-12/R-13's own tests", disposition: refused, reason: "documents mechanics that are not wired: PromoteTier (R-12) has no production caller (blast_radius kind delete at 9ea6da5f reads 0 typed referents, only its six tests reach it), so docs/layers.md would describe a promotion no product path performs; retired with R-12"}

- [x] `docs/mcp-tools.md` gains the `findings_near` catalog row (full arg/return/purpose reference, CLI mirror noted) {id: R-22, symbol: registerFindingsNear, package: internal/mcp, skip_test: true, skip: "documentation-only row; verified by direct read of docs/mcp-tools.md's existing investigation-tools table convention at authoring time"}

## Deferred (named triggers, not silently dropped)

- **A real `TestFindingsEval_Precision` run** (R-13) — no `ANTHROPIC_API_KEY`
  or logged-in `claude` CLI was available in the session that authored the
  fixtures, and spending a fresh layer's eval tokens is a deliberate,
  budgeted decision. **Trigger:** the next session with real model access;
  on PASS, record it in `eval/RESULTS.md` and flip `layer.cue`'s `evaled`
  to `true` (both mirrors, per `TestPackFilesMatchSource`) in the SAME
  change.
- **`internal/mcp/review_tool.go`'s (review_diff MCP tool) own supersede
  path does not record a resolution-classified `claim_event`** — only the
  CLI path (`cmd/fathom/review.go`'s `recordReviewSupersedeEvents`, R-10)
  does, exactly matching phase Q's own pre-existing scope for the
  "superseded" event itself (Q-7 wired the CLI path only; this phase
  extends that same path with resolution classification, not a new gap).
  **Trigger:** a documented case where an MCP-driven review's fixed-finding
  reconcile needed durable resolution evidence and the CLI path was not
  also run.
- **Cross-repo finding propagation** — explicitly out of scope per
  `docs/design/findings-as-evidence.md` §5's own non-goals (repo-scoped
  context in v1; the platform-module binding path is the eventual
  mechanism, on evidence).
