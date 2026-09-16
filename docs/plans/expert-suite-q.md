# Expert Suite Phase Q — observability surfaces (fathom's side: the verbs)

Design (normative — owner-commissioned 2026-07-30; see
`docs/vision/13-expert-suite.md` §9's row **Q** for the scope this plan
implements):

- `docs/design/observability-surfaces.md` §8 deliverables 1-6 — fathom-side
  only. Studio's screens (§8 item 7) are a separate, later slice per §9's
  own sequencing ("fathom's verbs land first (they are the contract),
  studio's screens second").
- Doctrine: `docs/vision/13-expert-suite.md` §8 laws 12 and 13 (already
  landed; this plan is implementation, not a doctrine change).
- Builds on `docs/design/production-tokenomics.md` (phase P, merged) —
  this plan REUSES phase P's instruments (the beat log's payload-cost/
  discovery/ident-cache channels, the session ledger,
  `discoveryIndexTools`) rather than duplicating their aggregators.

**Anchor lessons carried from phase P (binding on every row below).**
1. FUNCTIONS/METHODS only as `symbol:` anchors — never struct/type names.
   Checked for symbol/type name collisions before anchoring (`grep -rn
   "^func <Name>(" / "^type <Name> "` across `internal/`, `cmd/fathom/`,
   `pkg/`) — zero collisions found for every symbol anchored below.
2. Annotations sit ON THE CHECKBOX LINE ITSELF — a continuation-line
   annotation is silently dropped by `verify.ParseMarkdownPlan` and the row
   parses as contract-less (the phase-Q grounding's own explicit lesson).
3. Doc-only rows carry `skip_test: true` with a `skip:` reason naming the
   actual acceptance evidence, never a fabricated `Test*` claim.

## Key engineering decisions this plan pins

- **`internal/metrics` is the new canonical home for cross-verb
  aggregation** — pure functions over already-parsed inputs
  (`[]core.CompileRun`, a package-local `CallEntry` slice mirroring
  `adoptionCallEntry`'s JSON shape, `[]heartbeat.Beat`, session-ledger
  entries), never doing I/O itself (mirrors `buildResearchReport`'s own
  I/O-outside-the-pure-core discipline). `cmd/fathom` (package main) still
  owns every file read/write, exactly like `research_report.go` already
  does — `internal/metrics` cannot import `cmd/fathom` (a `package main`),
  so the relationship is the other direction: `cmd/fathom`'s new `metrics
  *.go` files AND `research_report.go` both import `internal/metrics`.
- **`discoveryIndexTools` moves to `internal/metrics` as `DiscoveryIndexTools`
  (+ `IsDiscoveryIndexTool` accessor)** — the design's own instruction
  ("reusing phase P's `discoveryIndexTools` set — import it, don't
  redefine, a second definition WILL drift"). `research_report.go` is
  updated to call `metrics.IsDiscoveryIndexTool` instead of a local map,
  with `buildDiscoverySection`'s existing behavior and every existing
  phase-P test unchanged (same set, same lookup, moved home only).
- **A single shared zero-denominator guard**, `metrics.Ratio(numerator,
  denominator int) (*float64, string)`: nil + an honest note when
  denominator is zero, a computed percentage otherwise. Every ratio this
  phase computes (cost-per-claim, error rate, injection coverage, ident
  cache hit rate, interventions instrumented-share) goes through it —
  one implementation, one direct test, closing the design §6 zero-
  denominator requirement structurally rather than per-call-site.
- **Compile rejection-reason capture (O19's fix) is a three-layer change,
  bottom-up:** (1) `internal/semantic/compile.go`'s `verifyDraft` wraps each
  rejection in a `*claimRejectionError{cause, err}` (four causes:
  `malformed`, `anchor-missing`, `scope-unverifiable`, `over-broad` — every
  one of `verifyDraft`'s seven existing rejection sites classified, no
  behavior change to WHICH drafts are rejected, only what is now recorded
  about why); `classifyRejectionCause` unwraps it (defaulting unclassified
  errors to `malformed` rather than dropping them). (2) `buildRejectionReasons`
  turns the run's accumulated cause→count/samples map into a deterministic,
  sorted `[]core.RejectionBreakdown` (capped at
  `maxRejectionSamplesPerCause` = 3 example messages per cause — enough to
  act on, never the full corpus). (3) `internal/store`: a new
  `rejection_reasons_json` column on `compile_runs` (the same conditional-
  migration pattern `escape_note`/`anchor_text` used —
  `PRAGMA table_info`, additive, never breaking a pre-existing store),
  `RecordCompileRun`/`CompileRuns` JSON-marshal/unmarshal it.
  `printCompileRunCore` (cmd/fathom/compile.go) renders it when non-empty;
  `module compile`/`module runs` get it for free (both already print/JSON-
  encode `core.CompileRun` unchanged). `fathom metrics spend` surfaces the
  same breakdown per layer (design's own instruction: "surface in `metrics
  spend`").
- **Interventions row 2 ("review findings raised then superseded") needed
  real wiring, not just a reader** — `cmd/fathom/review.go`'s fixed-finding
  reconcile (`review.BuildPersist`'s `supersede` list) called
  `SetClaimStatus` only; no `claim_events` row was ever written, so the
  design's own claimed mechanism ("from `claim_events`") did not exist in
  code. Closed by: `pkg/client.AppendClaimEvent` (a thin exported wrapper
  over the already-private `c.st.AppendClaimEvent` `ClaimFeedback` already
  uses internally), a new `"superseded"` `ClaimEvent.Verdict` value
  (`internal/core/claims.go`'s doc comment extended, five values now,
  machine-authored exactly like `"contradicted"`/`"re-anchored"`), and
  `recordReviewSupersedeEvents` (`cmd/fathom/review.go`) — called from the
  persist path only (never under `--no-persist`), best-effort (a logging
  failure never fails the review run, mirroring every other instrumentation
  write in this program). `internal/store` gains `ClaimEventsSince(ctx,
  since, until)` (SQLite + memory) since no bulk, time-windowed
  `claim_events` read existed — `claim_events` carries no `service_id`
  column, so the read is honestly store-wide, documented in the
  `Interventions` report's own note (the same honesty pattern
  `InjectionScopeNote` already established for a differently-scoped
  denominator).
- **Row 3 of the design's own Interventions table ("non-conforming
  artifacts refused") has NO durable evidence source today** —
  `module doctor`'s promotion refusals and `store.MountReadOnly`
  conformance failures are computed fresh per invocation and never
  persisted anywhere (no beat, no log line). The design's own §1 table
  already omits a "live number today" for this row (unlike rows 1/4/5,
  which have one) — this plan does not fabricate one either:
  `AggregateInterventions` reports it as an explicit
  `NotConformingRefusedNote` ("not measurable: ...") — never a zero, never
  omitted — and this is disclosed honestly in the final report rather than
  silently deferred. Wiring a persistent log for module-doctor refusals is
  named as a deferred item with its trigger.
- **`fathom metrics series`'s bucket granularity is honestly asymmetric.**
  Daily rollups (`workspace/artifacts/metrics/daily/YYYY-MM-DD.json`) are
  the only persisted granularity (design §4) — `--bucket day` reads
  rollups plus today's live tail; `--bucket hour` has no persisted rollup
  to read at all and is computed ENTIRELY from live raw logs (call log +
  beat log) still on disk, so an hour bucket older than the raw logs'
  retention window is unavailable and `SeriesReport.Note` says so
  explicitly — never silently returned as zero.
- **The retention/rotation verb is explicit and archives, never deletes**
  (design §4: "never delete raw data implicitly"). `fathom metrics rotate`
  defaults to a dry-run report of what WOULD move; `--apply` renames
  matching call-log lines older than `--keep-days` into a dated
  `<path>.archive-<cutoff>` sibling file (never truncates the live log in
  place, never removes the archive). The beat log's own existing
  size-based rotation (`rotateBeatLogIfNeeded`, phase P) is unchanged and
  is documented as the beat log's own answer to the same retention
  question — `fathom metrics rotate` does not duplicate it.
- **`/statusz` sits inside the hosted-mode allowlist gate, not outside
  it** (phase-O critic finding #2's exact class, GKE-MH-7): `newHTTPHandler`
  registers `GET /statusz`; `modulesAuthMiddleware` (already the ONE gate
  point for `/rpc/modules` when `--modules-token-env` is configured) is
  extended to also require the same bearer token for `/statusz` — every
  other route (`/rpc`, `/healthz`, `/metrics`, `/events`) is untouched, and
  `/statusz` stays fully open (today's loopback-only default) whenever no
  modules token is configured, exactly matching the existing trust
  boundary for everything else.
- **`FATHOM_METRICS` tiering is read-only reporting, not a new gate.**
  `ResolveMetricsTier()` (`internal/metrics`) reads `FATHOM_METRICS`
  (`off`/`basic`/`full`, default `basic` on empty/unset, any other value
  falls back to `basic` with the fallback itself observable via the
  returned note) — no code in this phase actually branches production
  behavior on the tier (payload sampling / per-section compile diagnostics
  for `full` are `deferred`, named with a trigger, per the design's own
  "on while tuning, off after" framing needing a real tuning window to
  design against, not synthesized here). `fathom doctor`'s
  `metrics_tier` check (`checkMetricsTier`) reports the active tier,
  mirroring `checkChannelsEnv`'s honesty discipline exactly (`doctorPass`
  always — an operator's deliberate choice, never a wiring failure).

## Tasks

### A — compile rejection-reason capture (O19's fix, docs/design/observability-surfaces.md §8 item 5)

- [x] `verifyDraft`'s seven rejection sites each wrap their error with a classified cause (malformed / anchor-missing / scope-unverifiable / over-broad); `classifyRejectionCause` unwraps it, defaulting an unclassified error to malformed rather than dropping it {id: Q-1, symbol: classifyRejectionCause, package: internal/semantic, reachable_from: TestBuild_PropagatesListError}
  - `internal/semantic/compile.go`: `claimRejectionError` (cause + wrapped
    err, `Unwrap` for `errors.As`), `rejectErr(cause, format, args...)
    error` (the seven `verifyDraft` return sites switched to it, behavior
    unchanged — same rejections, now classified), `classifyRejectionCause(err
    error) string`.
  - Tests (`internal/semantic/rejectioncause_test.go`):
    `TestClassifyRejectionCause_EmptyRule_Malformed`,
    `_WordBudget_Malformed`, `_CitationOutsideRange_AnchorMissing`,
    `_CitationDoesNotResolve_AnchorMissing` (both `HashLines`/`LinesText`
    failure paths), `_EmptyScope_Malformed`, `_ScopeMatchesNoFile_ScopeUnverifiable`,
    `_ScopeOverBroad_OverBroad`, `_UnclassifiedError_DefaultsMalformed`.

- [x] The compile loop accumulates a deterministic, sorted rejection-cause breakdown (count + up to 3 example messages per cause) onto the run {id: Q-2, symbol: buildRejectionReasons, package: internal/semantic, reachable_from: TestBuild_PropagatesListError}
  - `internal/semantic/compile.go`: `buildRejectionReasons(counts
    map[string]int, samples map[string][]string) []core.RejectionBreakdown`
    — sorted by cause name, samples capped at `maxRejectionSamplesPerCause`
    (3); `Compile`'s section loop calls `classifyRejectionCause` at the
    existing `run.ClaimsRejected++` site and feeds both maps; `run.RejectionReasons
    = buildRejectionReasons(...)` set once, after the section loop, exactly
    like `run.ClaimsEmitted`.
  - `internal/core/claims.go`: `RejectionBreakdown{Cause, Count, Samples}`;
    `CompileRun.RejectionReasons []RejectionBreakdown`.
  - Tests (`internal/semantic/compile_test.go`):
    `TestCompile_RejectionReasons_GroupedByCause` (a fixture corpus that
    trips at least two distinct causes in one run, asserting both the
    counts and that samples are non-empty and capped),
    `TestBuildRejectionReasons_DeterministicOrder`, `_SamplesCapped`,
    `TestCompile_DryRun_NeverPopulatesRejectionReasons` (dry runs skip
    `verifyDraft` entirely — the field must stay empty, not zero-valued
    per cause).

- [x] `compile_runs` persists the rejection breakdown (a new JSON column, additive migration on a pre-existing store) {id: Q-3, symbol: marshalRejectionReasons, package: internal/store, reachable_from: TestAdoptionServiceScoped}
  - `internal/store/sqlite.go`: `rejection_reasons_json TEXT NOT NULL
    DEFAULT '[]'` added to the `compile_runs` CREATE TABLE shape AND to the
    conditional-migration list (the `escape_note`/`anchor_text` pattern —
    a pre-existing store predates the column); `RecordCompileRun`/
    `CompileRuns`' marshal/unmarshal steps are factored into two free
    functions, `marshalRejectionReasons`/`unmarshalRejectionReasons` — a
    name unique to this file, unlike the `RecordCompileRun` method name
    itself (both `SQLiteStore` and `MemoryStore` implement it, which made
    it an unusable, ambiguous anchor for a symbol-based verify tool;
    discovered when `fathom verify --strict` failed `test_reaches` on it
    despite real, passing tests — fixed by extracting these two
    unambiguous helpers rather than weakening the claim). `nil` marshals
    to `"[]"` explicitly (`json.Marshal(nil slice)` encodes `"null"`, not
    `"[]"` — a real bug this extraction's own direct test caught); a
    malformed/legacy empty string degrades to `nil`, never an error —
    mirrors this file's existing lenient-read discipline.
  - `internal/store/memory.go`: unchanged (struct-copy semantics already
    carry the new field) — asserted directly by a test, not by inspection.
  - Tests (`internal/store/sqlite_test.go`):
    `TestMarshalRejectionReasons_NilRoundTripsAsEmptyArray`,
    `_RoundTripsRealData`, `TestUnmarshalRejectionReasons_LegacyEmptyString_Nil`,
    `_Malformed_DegradesToNilNotError` (the direct, unambiguous-symbol
    tests `test_reaches` now finds), plus the end-to-end
    `TestSQLiteStore_RecordCompileRun_PersistsRejectionReasons`,
    `_EmptyRejectionReasons_RoundTripsAsEmptySlice`,
    `TestSQLiteStore_CompileRuns_LegacyRowMissingColumn_DegradesToNil`
    (a store created before this migration — column absent, migration
    adds it, pre-existing rows read back with `RejectionReasons == nil`,
    never an error); `internal/store/memory_test.go`'s
    `TestMemoryStore_RecordCompileRun_RoundTripsRejectionReasons`.

- [x] `printCompileRunCore` renders the rejection breakdown (module compile / fathom compile share this renderer unchanged) {id: Q-4, symbol: printCompileRunCore, package: cmd/fathom}
  - `cmd/fathom/compile.go`: `printCompileRunCore` gains a `rejection_reasons:`
    block (cause, count, one sample line each) printed only when
    `len(run.RejectionReasons) > 0` — a run with zero rejections prints
    nothing new, byte-for-byte identical to before this row.
  - Tests (`cmd/fathom/compile_test.go`):
    `TestPrintCompileRunCore_RejectionReasons_Rendered`,
    `_NoRejections_OutputUnchanged` (pins the exact pre-existing output for
    a zero-rejection run — the non-regression proof).

### B — interventions row 2: review-finding supersede events (docs/design/observability-surfaces.md §1)

- [x] `pkg/client` exposes an `AppendClaimEvent` wrapper so a CLI command (not just the MCP feedback tool) can record a claim verdict {id: Q-5, symbol: AppendClaimEvent, package: pkg/client}
  - `pkg/client/semantic.go`: `AppendClaimEvent(ctx, ev core.ClaimEvent)
    error` — thin passthrough to `c.st.AppendClaimEvent`, mirrors
    `PutClaims`/`SetClaimStatus`'s own one-line wrapper style.
  - `internal/core/claims.go`: `ClaimEvent.Verdict`'s doc comment extended
    with the fifth value, `"superseded"` — machine-authored by the review
    CLI's fixed-finding reconcile, pairs with `ClaimSuperseded` (already
    the status `SetClaimStatus` sets on the same call site).
  - Tests (`pkg/client/semantic_test.go`): `TestClient_AppendClaimEvent_RoundTrips`.

- [x] `internal/store` gains a bulk, time-windowed `claim_events` read (no prior bulk query existed — only per-claim-ID) {id: Q-6, symbol: queryClaimEventsSince, package: internal/store, reachable_from: TestCmdFindingsStatus_JSON}
  - `internal/store/store.go`: `ClaimEventsSince(ctx, since, until
    time.Time) ([]core.ClaimEvent, error)` added to the `Store` interface,
    doc comment states plainly that `claim_events` carries no `service_id`
    column so this read is store-wide, not service-scoped (matches the
    schema `internal/store/conformance.go` already documents).
  - `internal/store/sqlite.go` / `memory.go`: implementations, both
    ordered by `(event_at, verdict)` — the same deterministic order
    `ClaimEvents` already uses. `SQLiteStore.ClaimEventsSince`'s query body
    is factored into `queryClaimEventsSince(ctx, db, since, until)`, a free
    function unique to this file — `ClaimEventsSince` itself is ambiguous
    as a verify-tool anchor (both `SQLiteStore` and `MemoryStore` implement
    a method of that name in the same package), the same class of finding
    Q-3 hit; caught the same way, before it shipped, by running
    `fathom verify --strict` against this plan pre-emptively rather than
    only at final sign-off.
  - Tests: `TestSQLiteStore_ClaimEventsSince_FiltersByWindow`,
    `_EmptyStore_EmptySlice`, `TestQueryClaimEventsSince_FiltersByWindow`
    (the direct, unambiguous-symbol test); `TestMemoryStore_ClaimEventsSince_FiltersByWindow`
    (`internal/store/sqlite_test.go`, `memory_test.go`).

- [x] `fathom review`'s persist path records a `"superseded"` claim_event for every finding it reconciles as fixed — best-effort, never fails the review run {id: Q-7, symbol: recordReviewSupersedeEvents, package: cmd/fathom}
  - `cmd/fathom/review.go`: `recordReviewSupersedeEvents(ctx, c
    *client.Client, supersededIDs []string, now func() time.Time, stderr
    io.Writer) int` — called immediately after each successful
    `SetClaimStatus` in `runReview`'s existing persist loop (never under
    `--no-persist`, matching that loop's own gating); a write failure is
    logged to stderr (mirrors the loop's existing warning discipline) and
    never changes `runReview`'s exit code.
  - Tests (`cmd/fathom/review_test.go`):
    `TestRecordReviewSupersedeEvents_WritesOneEventPerID`,
    `_NoPersist_NeverCalled`, `_StoreWriteFails_LoggedNotFatal`.

### C — `internal/metrics`: shared aggregation core (docs/design/observability-surfaces.md §8 item 1)

- [x] `DiscoveryIndexTools` moves from `cmd/fathom` to `internal/metrics` (the design's own "import it, don't redefine" instruction) with an accessor {id: Q-8, symbol: IsDiscoveryIndexTool, package: internal/metrics, reachable_from: TestAnsweredBeatCarriesDisplacedCount}
  - `internal/metrics/discovery.go`: `DiscoveryIndexTools map[string]bool`
    (the exact 10-tool set, moved verbatim with its explaining comment),
    `IsDiscoveryIndexTool(tool string) bool`.
  - `cmd/fathom/research_report.go`: local `discoveryIndexTools` var
    deleted; `buildDiscoverySection` calls `metrics.IsDiscoveryIndexTool`
    — every pre-existing phase-P discovery-share test in
    `research_report_test.go`/`research_report_phasep_test.go` stays green
    unchanged (same set, same lookup, different package).
  - Tests (`internal/metrics/discovery_test.go`):
    `TestIsDiscoveryIndexTool_KnownTool_True`, `_PlumbingTool_False`
    (asserts `capabilities`/`ingest`/`list_services`/`obligations`/
    `staleness_report`/`verify_plan` are excluded, pinning the design §4
    exclusion list directly).

- [x] `Ratio` is the one zero-denominator guard every ratio in this phase goes through {id: Q-15, symbol: Ratio, package: internal/metrics, reachable_from: TestAnsweredBeatCarriesDisplacedCount}
  - `internal/metrics/ratio.go`: `Ratio(numerator, denominator int) (pct
    *float64, note string)` — `denominator == 0` returns `(nil, "not
    measurable: zero denominator")`; otherwise a computed percentage and
    an empty note.
  - Tests (`internal/metrics/ratio_test.go`): `TestRatio_ZeroDenominator_NeverFabricates100Percent`
    (numerator > 0, denominator == 0 — the exact bug class phase P's
    critic finding F1 fixed once, now guarded structurally),
    `TestRatio_ZeroNumerator_ZeroPercent`, `TestRatio_ComputesExpectedPercentage`.

- [x] `AggregateSpend` — runs/tokens/USD/cost-per-claim/budget-headroom/escape-hatch flags, grouped by layer, day, or model {id: Q-9, symbol: AggregateSpend, package: internal/metrics, reachable_from: TestAnsweredBeatCarriesDisplacedCount}
  - `internal/metrics/spend.go`: `CompileRunBudgetLookup func(layer
    string) (maxUSD float64, found bool)` (injected — `internal/metrics`
    does no I/O, so `cmd/fathom` resolves budgets via
    `semantic.LoadLayerIn` and passes the closure in);
    `SpendOptions{Since, Until time.Time; GroupBy string}`; `SpendRow{Group
    string; Runs int; TokensIn, TokensOut int; CostUSD float64;
    ClaimsEmitted int; CostPerClaimUSD *float64; CostPerClaimNote string;
    BudgetMaxUSD *float64; BudgetHeadroomUSD *float64; BudgetNote string;
    EscapeHatchRuns int}`; `SpendReport{Since, Until string; GroupBy
    string; Rows []SpendRow; TotalRuns int; TotalCostUSD float64; Note
    string}`; `AggregateSpend(runs []core.CompileRun, lookup
    CompileRunBudgetLookup, opts SpendOptions) SpendReport` — cost-per-
    claim via `Ratio`-style zero-claims guard (an explicit
    `CostPerClaimNote` when `ClaimsEmitted == 0`, never a `+Inf`/NaN);
    budget headroom populated ONLY for `GroupBy == "layer"` (a per-run
    ceiling has no cross-layer meaning under day/model grouping — `
    BudgetNote` says so explicitly for the other two).
  - Tests (`internal/metrics/spend_test.go`):
    `TestAggregateSpend_GroupByLayer_CostPerClaimAndBudget`,
    `_GroupByDay`, `_GroupByModel_BudgetNotApplicable`,
    `_ZeroClaimsEmitted_CostPerClaimHonestNote` (the zero-denominator
    proof for THIS instrument), `_EscapeHatchRunsFlagged`,
    `_EmptyInput_EmptyReport`.

- [x] `AggregateTraffic` — volume/p50/p95/error-rate/plumbing-vs-discovery split, grouped by surface, tool, or hour {id: Q-10, symbol: AggregateTraffic, package: internal/metrics, reachable_from: TestAnsweredBeatCarriesDisplacedCount}
  - `internal/metrics/traffic.go`: `CallEntry{TS string; Tool, Service,
    Surface string; LatencyMS float64; Error string}` (mirrors
    `adoptionCallEntry`'s JSON-relevant fields — `cmd/fathom` converts);
    `TrafficOptions{Since, Until time.Time; GroupBy string}`;
    `TrafficRow{Group string; Calls int; P50MS, P95MS *float64; ErrorRate
    *float64; ErrorRateNote string}`; `TrafficReport{... Rows []TrafficRow;
    DiscoveryCalls, PlumbingCalls, OtherCalls int; TotalCalls int; Note
    string}` — the discovery/plumbing/other split uses
    `metrics.IsDiscoveryIndexTool` (Q-8) plus an explicit plumbing set
    (`capabilities`/`ingest`/`list_services`/`obligations`/
    `staleness_report`/`verify_plan` — the same six the design's own
    exclusion list names) so every tool call lands in exactly one bucket.
  - Tests (`internal/metrics/traffic_test.go`):
    `TestAggregateTraffic_GroupBySurface`, `_GroupByTool`, `_GroupByHour`,
    `_ErrorRate_ZeroCallsInGroup_HonestNote`, `_DiscoveryPlumbingOtherSplit_Exhaustive`
    (every tool lands in exactly one bucket, no double count), `_EmptyInput_EmptyReport`.

- [x] `AggregateChannels` — injection coverage, guarded pre/post, dedup, module serves, with phase P's "not measurable" strings carried through VERBATIM {id: Q-11, symbol: AggregateChannels, package: internal/metrics, reachable_from: TestAnsweredBeatCarriesDisplacedCount}
  - `internal/metrics/channels.go`: `ChannelsReport{InjectionSeen,
    InjectionAnswered int; InjectionCoverageRate *float64;
    InjectionCoverageNote string; ObligationsServed int;
    ObligationsEvictedNote string; GuardPreSeen, GuardPreServed,
    GuardPostSeen, GuardPostServed, GuardDedupSuppressed int;
    GuardCoverageNote string; ModuleRulesServed map[string]int;
    ModuleServingNote string}`; `AggregateChannels(beats []heartbeat.Beat,
    since, until time.Time) ChannelsReport` — the phase/seen/answered/
    dedup/module-served accounting is the SAME beat-shape logic
    `buildResearchReport`'s edit-hook section already implements (channel
    `"edit-hook"`, `phase` meta defaulting to `"post"`), reimplemented
    here as the canonical, reusable form; `ObligationsEvictedNote` is the
    exact phase-P string ("not measurable: no eviction event is logged
    anywhere in fathom today...") — pinned directly by test, not
    paraphrased.
  - Tests (`internal/metrics/channels_test.go`):
    `TestAggregateChannels_InjectionCoverage`, `_GuardPreServed_DedupSuppressed`,
    `_ModuleRulesServed`, `_NeverMeasurable_NotesVerbatim` (pins the exact
    phase-P note strings), `_EmptyInput_HonestZeros`.

- [x] `AggregateInterventions` — the design §1 table, mechanism-labeled, row 3 honestly "not measurable" {id: Q-12, symbol: AggregateInterventions, package: internal/metrics, reachable_from: TestAnsweredBeatCarriesDisplacedCount}
  - `internal/metrics/interventions.go`: `InterventionRow{Name,
    Mechanism string; Count *int; Note string}`;
    `InterventionsReport{Since, Until string; Rows []InterventionRow}`;
    `AggregateInterventions(callVerdicts []CallVerdictOutcome, beats
    []heartbeat.Beat, claimEvents []core.ClaimEvent, claimEventsQueried
    bool, since, until time.Time) InterventionsReport` — `claimEventsQueried`
    distinguishes "no --store given" from "queried a real store with
    genuinely zero events this window" (a nil vs. empty slice can't make
    that distinction on its own — `Store.ClaimEventsSince` also returns
    nil on a real empty result). Five rows, in the design's own order:
    (1) unproven-done refusals (sum of `verify_plan` calls' `failing`
    verdict-outcome field in window); (2) review findings superseded
    (`claimEvents` filtered to `Verdict == "superseded"` in window —
    Q-6/Q-7's own data); (3) non-conforming
    artifacts refused — ALWAYS `Count: nil`, `Note: "not measurable: ..."`
    (no code path in fathom persists this event today — stated as a
    limitation, not inferred as zero); (4) obligations served (beat log,
    reuses the exact `"edit-hook"`/`"post"`/`emitted:true` accounting
    `AggregateChannels` (Q-11) already computes — this function calls it
    once internally, no second implementation); (5) guarded pre-warns
    (same channels aggregate's `GuardPreServed`).
  - `CallVerdictOutcome{Tool string; Outcome map[string]int}` — the
    minimal shape needed from a call-log entry's `verdict_outcome` field,
    decoupled from `adoptionCallEntry` the same way `CallEntry` is.
  - Tests (`internal/metrics/interventions_test.go`):
    `TestAggregateInterventions_AllFiveRowsPresent`,
    `_UnprovenDoneRefusals_SumsFailingVerdicts`,
    `_ReviewSupersededCount_FromClaimEvents`,
    `_NonConformingRow_AlwaysNotMeasurable` (pins the honest-gap note
    directly — the load-bearing "no inferred agent behavior" proof),
    `_ObligationsAndGuardRows_MatchChannelsAggregate`, `_EmptyInput_AllRowsPresentWithNotes`.

- [x] Daily rollups: deterministic, additive-only, `workspace/artifacts/metrics/daily/YYYY-MM-DD.json` {id: Q-13, symbol: BuildDailyRollup, package: internal/metrics, reachable_from: TestAnsweredBeatCarriesDisplacedCount}
  - `internal/metrics/rollup.go`: `DailyRollup{Date string; SpendTotalUSD
    float64; SpendCostPerClaimUSD *float64; TrafficTotalCalls int;
    TrafficErrorRate *float64; ChannelsInjectionCoverageRate *float64;
    ChannelsObligationsServed int; InterventionsUnprovenDoneRefusals *int;
    InterventionsReviewSuperseded *int; GeneratedAt string}` (a flat,
    named-field summary — the fields `metrics series` (Q-20) looks up by
    name); `BuildDailyRollup(date time.Time, spend SpendReport, traffic
    TrafficReport, channels ChannelsReport, interventions
    InterventionsReport, now time.Time) DailyRollup` — pure; `WriteDailyRollup(dir
    string, r DailyRollup) error` (one file per date, overwritten
    idempotently on re-run for the SAME date — additive across DATES,
    never across a single date's own re-computation); `ReadDailyRollups(dir
    string, from, to time.Time) ([]DailyRollup, error)` (missing files
    skipped, never an error — a rollup that was never built for a date is
    an honest gap, not a zero).
  - Tests (`internal/metrics/rollup_test.go`):
    `TestBuildDailyRollup_FlattensAllFourReports`,
    `TestWriteDailyRollup_ThenReadDailyRollups_RoundTrips`,
    `_OverwritesSameDateIdempotently`, `_MissingDatesSkippedNotZeroed`.

- [x] `ResolveMetricsTier` — `FATHOM_METRICS` off/basic/full, default basic, invalid value falls back to basic with an observable note {id: Q-14, symbol: ResolveMetricsTier, package: internal/metrics, reachable_from: TestAnsweredBeatCarriesDisplacedCount}
  - `internal/metrics/tiering.go`: `MetricsTier` (`"off"`/`"basic"`/`"full"`);
    `ResolveMetricsTier() (tier MetricsTier, note string)` — reads
    `FATHOM_METRICS`; doc comment states plainly that NO tier spends LLM
    tokens (design §3's own headline) and that `full`'s payload-sampling/
    per-section-diagnostics behavior is deferred (this phase reports the
    tier; it does not yet branch behavior on it beyond what Q-1..Q-4
    already capture unconditionally).
  - Tests (`internal/metrics/tiering_test.go`): `TestResolveMetricsTier_Unset_DefaultsBasic`,
    `_Off`, `_Full`, `_InvalidValue_FallsBackToBasicWithNote`.

### D — the five `fathom metrics` verbs (docs/design/observability-surfaces.md §8 item 2)

- [x] `fathom metrics spend` {id: Q-16, symbol: cmdMetricsSpend, package: cmd/fathom}
  - `cmd/fathom/metrics_spend.go`: `cmdMetricsSpend(args, stdout, stderr)`
    — `--store` (required), `--service`, `--module` (layer filter),
    `--since` (duration) / `--from`/`--to` (RFC3339, mutually exclusive
    with `--since`), `--by layer|day|model` (default `layer`), `--root`
    (budget-lookup resolution via `semantic.LoadLayerIn`), `--json`.
    Mounts read-only (`store.MountReadOnly`, LevelL0 — never a writable
    open), reads `CompileRuns`, builds the budget-lookup closure, calls
    `metrics.AggregateSpend`.
  - Tests (`cmd/fathom/metrics_spend_test.go`): `TestCmdMetricsSpend_JSON_GroupByLayer`,
    `_SinceFlag_FiltersWindow`, `_FromTo_FiltersWindow`, `_ModuleFilter`,
    `_MissingStore_ErrorExit`, `_BudgetLookup_LayerNotFoundUnderRoot_HonestNote`.

- [x] `fathom metrics traffic` {id: Q-17, symbol: cmdMetricsTraffic, package: cmd/fathom}
  - `cmd/fathom/metrics_traffic.go`: `cmdMetricsTraffic(args, stdout,
    stderr)` — `--service`, `--since`/`--from`/`--to`, `--by
    surface|tool|hour` (default `surface`), `--call-log`, `--json`. Reuses
    `readCallLog`/`resolveCallLogPath`, converts `adoptionCallEntry` →
    `metrics.CallEntry`, calls `metrics.AggregateTraffic`.
  - Tests (`cmd/fathom/metrics_traffic_test.go`): `TestCmdMetricsTraffic_JSON_GroupBySurface`,
    `_GroupByHour`, `_ServiceFilter`, `_MissingCallLog_HonestZeros`.

- [x] `fathom metrics channels` {id: Q-18, symbol: cmdMetricsChannels, package: cmd/fathom}
  - `cmd/fathom/metrics_channels.go`: `cmdMetricsChannels(args, stdout,
    stderr)` — `--since`/`--from`/`--to`, `--hooks-log`, `--json`. Reuses
    `readBeatLog`/`beatLogPath`, calls `metrics.AggregateChannels`.
  - Tests (`cmd/fathom/metrics_channels_test.go`): `TestCmdMetricsChannels_JSON`,
    `_MissingHooksLog_HonestNotes`.

- [x] `fathom metrics interventions` {id: Q-19, symbol: cmdMetricsInterventions, package: cmd/fathom}
  - `cmd/fathom/metrics_interventions.go`: `cmdMetricsInterventions(args,
    stdout, stderr)` — `--since`/`--from`/`--to`, `--store` (optional —
    needed only for the `claim_events`-backed row; absent means that ONE
    row degrades to its own honest note, every other row still computes),
    `--call-log`, `--hooks-log`, `--json`. Calls `metrics.AggregateInterventions`.
  - Tests (`cmd/fathom/metrics_interventions_test.go`): `TestCmdMetricsInterventions_JSON_AllFiveRows`,
    `_NoStoreFlag_ReviewRowDegradesHonestly`.

- [x] `fathom metrics series <metric> --bucket hour|day --since` {id: Q-20, symbol: cmdMetricsSeries, package: cmd/fathom}
  - `cmd/fathom/metrics_series.go`: `cmdMetricsSeries(args, stdout,
    stderr)` — positional `<metric>` (one of the `DailyRollup` field
    names, exposed as a small documented name→accessor map so an unknown
    metric lists valid names rather than silently returning empty);
    `--bucket day` reads `ReadDailyRollups` plus today's live tail
    (recomputed via the same aggregators over just-today's window);
    `--bucket hour` (no persisted rollup exists at hour granularity —
    computed entirely from live raw logs still on disk, `SeriesReport.Note`
    states the retention-window caveat explicitly); `--since`, `--root`
    (rollup dir), `--json`.
  - Tests (`cmd/fathom/metrics_series_test.go`): `TestCmdMetricsSeries_DayBucket_RollupsPlusLiveTail`,
    `_HourBucket_LiveOnly`, `_UnknownMetric_ListsValidNames`.

- [x] `fathom metrics rollup build` and `fathom metrics rotate` (retention/rotation verb, deliverable 4's second half) {id: Q-21, symbol: cmdMetricsRollupBuild, package: cmd/fathom}
  - `cmd/fathom/metrics_rollup.go`: `cmdMetricsRollupBuild(args, stdout,
    stderr)` — `--date` (default: today), `--root`, `--store` (compile
    runs), `--call-log`, `--hooks-log`; runs all four aggregators for the
    named date's window and calls `metrics.WriteDailyRollup`.
    `cmdMetricsRotate(args, stdout, stderr)` — `--call-log`, `--keep-days`,
    `--apply` (default false: dry-run report only); moves matching lines
    to `<path>.archive-<cutoff-date>`, never deletes, never touches the
    live file without `--apply`.
  - Tests (`cmd/fathom/metrics_rollup_test.go`): `TestCmdMetricsRollupBuild_WritesFile`,
    `_OverwritesSameDate`; `TestCmdMetricsRotate_DryRunByDefault_NoFileChange`,
    `_Apply_ArchivesOldLines_LiveLogKeepsRecent`, `_NeverDeletesArchive`.

- [x] `fathom metrics` dispatch wires all six verbs into `main.go`'s subcommand table {id: Q-23, symbol: cmdMetrics, package: cmd/fathom}
  - `cmd/fathom/metrics.go`: `cmdMetrics(args, stdout, stderr)` — the
    `spend | traffic | channels | interventions | series | rollup |
    rotate` switch, mirroring `cmdModule`'s dispatch shape exactly;
    `cmd/fathom/main.go` gains `case "metrics": return cmdMetrics(args[1:],
    stdout, stderr)` plus the usage-line addition.
  - Tests (`cmd/fathom/metrics_test.go`): `TestCmdMetrics_DispatchesEachVerb`
    (one table-driven case per verb name, asserting it reaches the right
    handler rather than the unknown-verb error), `_UnknownVerb_ErrorExit`.

### E — `FATHOM_METRICS` tiering in doctor (docs/design/observability-surfaces.md §8 item 3)

- [x] `fathom doctor` reports the active `FATHOM_METRICS` tier {id: Q-24, symbol: checkMetricsTier, package: cmd/fathom}
  - `cmd/fathom/doctor_phaseq.go`: `checkMetricsTier() doctorCheck` —
    always `doctorPass` (an operator's deliberate choice, mirrors
    `checkChannelsEnv`'s exact discipline), `Reason` names the resolved
    tier and, on an invalid env value, the fallback note from
    `metrics.ResolveMetricsTier`. Wired into `runDoctorChecks`'s existing
    phase-P block (`doctor.go`).
  - Tests (`cmd/fathom/doctor_phaseq_test.go`): `TestCheckMetricsTier_Unset_Basic`,
    `_Full`, `_Off`, `_InvalidValue_NotesFallback`.

### F — `/statusz` on `serve-http` (docs/design/observability-surfaces.md §8 item 6)

- [x] `buildStatuszReport` — daemon health, per-service index freshness, channel liveness, last compile per layer {id: Q-25, symbol: buildStatuszReport, package: cmd/fathom}
  - `cmd/fathom/statusz.go`: `StatuszReport{GeneratedAt string; Uptime
    string; Services []StatuszServiceRow; ChannelsNote string}`;
    `StatuszServiceRow{Service, IndexedCommit string; FreshnessNote
    string; LastCompilePerLayer map[string]string}`; `buildStatuszReport(c
    *client.Client, startTime, now time.Time) StatuszReport` — read-only
    over the already-open client (`ServiceMeta`, `CompileRuns` via the
    client's store handle), no new store connection; channel liveness
    reuses `fathomChannelsDisabled`'s OWN env read (the same honesty
    caveat `checkChannelsEnv` already carries: this reports the
    SERVER process's own environment, not the harness's).
  - Tests (`cmd/fathom/statusz_test.go`): `TestBuildStatuszReport_ServicesListed`,
    `_NoServices_EmptyNotError`, `_ChannelsOff_NotedInReport`.

- [x] `GET /statusz` (HTML + JSON) wired on `serve-http`, gated inside the hosted-mode allowlist when `--modules-token-env` is configured {id: Q-26, symbol: modulesAuthMiddleware, package: cmd/fathom}
  - `cmd/fathom/servehttp.go`: `newHTTPHandler` registers `GET /statusz`
    (`Accept: application/json` or `?format=json` → JSON; otherwise a
    minimal, dependency-free HTML render — no template engine, no new
    import); `modulesAuthMiddleware`'s path check extended from
    `/rpc/modules` alone to `/rpc/modules` OR `/statusz` — every other
    route's behavior is byte-for-byte unchanged (existing
    `newHTTPHandler`/auth tests keep passing unmodified).
  - Tests (`cmd/fathom/servehttp_test.go`, extending the existing suite):
    `TestStatusz_JSON_Loopback_NoAuthRequired`,
    `TestStatusz_HostedMode_RequiresBearerToken` (the load-bearing
    phase-O-critic-finding-#2-class proof: a request to `/statusz` with no
    token, modules token configured, gets 401 — not a 200), `TestStatusz_HostedMode_ValidToken_Serves`.

### Docs (docs/design/observability-surfaces.md §8 item 8)

- [x] `docs/use-cases/metrics-surfaces.md`: new use-case guide — the five verbs, `--json` shapes, `/statusz`, tiering {id: Q-27, symbol: cmdMetrics, package: cmd/fathom, skip_test: true, skip: "documentation-only row; the verbs/shapes it documents are proven by Q-16 through Q-26's own tests"}

- [x] `docs/adoption/research-flywheel.md`: new "Observability surfaces (phase Q)" section — turning `FATHOM_METRICS=full` on for a tuning window and off after, `/statusz`'s hosted-mode gating {id: Q-28, symbol: ResolveMetricsTier, package: internal/metrics, skip_test: true, skip: "documentation-only row; the tiering it documents is proven by Q-14/Q-24's own tests"}

- [x] `GET /statusz` catalog entry — corrected destination: `docs/mcp-tools.md` turned out to be MCP-JSON-RPC-surfaces-only by its own established convention (CLI-only verbs like `fathom research report`/`module doctor` are NOT listed there either — confirmed by direct read before editing); `/statusz` is a plain HTTP route on `serve-http` like `/healthz`/`/metrics`, so it was added to `docs/usage.md`'s existing HTTP-endpoints table (where those routes already live) and `docs/architecture.md`'s one-line `serve-http` endpoint summary instead {id: Q-29, symbol: cmdMetrics, package: cmd/fathom, skip_test: true, skip: "documentation-only row; verified by direct read of docs/usage.md and docs/architecture.md at authoring time"}

## Deferred (named triggers, not silently dropped)

- **`FATHOM_METRICS=full`'s actual behavior** (payload sampling at a
  configurable rate, per-section compile diagnostics beyond Q-1..Q-4's
  always-on rejection-reason capture) — `ResolveMetricsTier` (Q-14) reports
  the tier; nothing yet branches on `full` specifically, since Q-1 through
  Q-4 already capture rejection reasons unconditionally (the design's
  highest-value item, per the mission grounding, did not need to wait for
  a tier gate). **Trigger:** a real tuning window (design §3's own framing)
  surfacing a concrete need for sampled payload retention beyond what's
  already captured.
- **A persistent log for `module doctor` promotion refusals / `store.MountReadOnly`
  conformance failures** — needed to make Interventions row 3
  ("non-conforming artifacts refused") measurable; today it is an honest
  `not measurable` note (Q-12). **Trigger:** a documented case where an
  author needed this count and had no fallback (mirrors phase P's own
  deferred-item discipline).
- **fathom-studio's four Charm boards + the web app** (design §8 items 7,
  §5) — explicitly out of scope for this plan per the design's own
  sequencing ("fathom's verbs land first ... studio's screens second") and
  the mission's own framing (Phase Q here is fathom-side only). **Trigger:**
  this plan merging (the verbs are the contract studio consumes).
- **Prometheus/OTel exporter, alerting** — design §7 non-goals, unchanged.
