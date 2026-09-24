# Diagnostics sweep — logging, fleet cost, doctor correctness, typed-edge recall (O112)

Source: the 2026-09-23 diagnostics sweep of the daemon, CLI and hooks
(`diag-plan.md`, synthesized against the pushclone checkout at `959bddff`).
23 raw findings across four investigator groups (`logs`, `fleet`, `logging`,
`backlog`) collapsed to 21 fixable code items (DG-1..DG-21, including three
owner-amended/new items — DG-16's corrected memory metric, DG-18's
`list_services` cache, DG-19's bind-retry, and DG-21's selector/interface
call-edge recall fix), 4 owner-decision items, and a handful of
verified-no-action findings recorded directly in `.fathom/observations.md`
O112. Grammar: `docs/plans/README.md`. Verify:
`fathom verify --service fathom --plan docs/plans/diagnostics-sweep.md`.

## A. Logging infrastructure (DG-1..DG-6)

- [x] Core leveled, timestamped logger: `internal/dlog.New` wraps `log/slog` with an injected clock and `FATHOM_LOG_LEVEL` (debug/info/warn/error) {id: DG-1, symbol: New, package: internal/dlog, reachable_from: TestNew_WritesTimestampedLeveledLines}
- [x] Dedupe-with-count: `(*Logger).Dedupe` logs a repeating condition once and folds the rest into a flushed summary line {id: DG-2, symbol: (*Logger).Dedupe, package: internal/dlog, reachable_from: TestLogger_DedupeCountsRepeatsAndFlushesSummary}
- [x] `fleetAdapter` routes its warnings through the deduped logger instead of raw `fmt.Fprintf`, and `UnsupportedLanguagesFor` stops re-walking a service whose recorded corpus root is gone {id: DG-3, symbol: (*fleetAdapter).UnsupportedLanguagesFor, package: cmd/fathom, reachable_from: TestFleetAdapter_UnsupportedLanguagesFor_MissingCorpusSkipsRewalk}
- [x] `classifyStoreError` tags a SQLite CANTOPEN condition distinctly from a generic store failure in `fleetAdapter`'s deduped log lines (DG-17, folded into the DG-3 wiring pass) {id: DG-17, symbol: (*fleetAdapter).SymbolsFor, package: cmd/fathom, reachable_from: TestSymbolsFor_TagsCantopenDistinctly}
- [x] `list_services`'s per-service unsupported-language + root-missing lookup is memoized per (service, store watermark) — DG-18, no re-walk on a read path, computed once per ingest {id: DG-18, symbol: (*fleetAdapter).unsupportedLanguagesCached, package: cmd/fathom, reachable_from: TestFleetAdapter_UnsupportedLanguagesFor_CallsDiscoverAtMostOncePerServiceWatermark}
- [x] Remaining daemon call sites (`memscavenge`, `selfrestart`, `fusion.Pipeline`) thread the same `*dlog.Logger`; the scavenger logs an actual heap_sys/heap_inuse delta and the self-restart adopt line names the running binary's build age {id: DG-4, symbol: runMemoryScavenger, package: cmd/fathom, reachable_from: TestRunMemoryScavenger_LogsHeapDelta}
- [x] Self-restart's adopt-line build-age suffix {id: DG-4b, symbol: buildAgeSuffix, package: cmd/fathom, reachable_from: TestWatchOwnBinary_AdoptLineIncludesBuildAge}
- [x] `fusion.Pipeline`'s typed-edge pass summary routes through an injected `*dlog.Logger` instead of bare stderr {id: DG-4c, symbol: (*Pipeline).SetLogger, package: internal/fusion, reachable_from: TestPipeline_SetLogger_RoutesTypedEdgeSummaryThroughLogger}
- [x] HTTP per-request access log carries method, path, status, duration and an `X-Fathom-Request-Id` {id: DG-5, symbol: loggingMiddleware, package: cmd/fathom, reachable_from: TestLoggingMiddleware_RecordsMethodPathStatusDuration}
- [x] `internal/dlog.RotateIfNeeded` bounds a log file's size, keeping one prior generation, so a launchd-piped daemon log cannot grow unbounded {id: DG-6, symbol: RotateIfNeeded, package: internal/dlog, reachable_from: TestRotateIfNeeded_OverCap_KeepsOneGeneration}
- [x] The daemon's own stdio redirection truncates in place without leaving a sparse hole once its target file is over cap {id: DG-6b, symbol: truncateStdioIfNeeded, package: cmd/fathom, reachable_from: TestTruncateStdioIfNeeded_OverCap_ResetsOffsetWithoutSparseHole}
- [x] `fetchObligations` names the reason on every failure branch (unreachable daemon, non-2xx, malformed body) instead of discarding the error {id: DG-7, symbol: fetchObligations, package: cmd/fathom, reachable_from: TestFetchObligations_ReturnsReasonOnEachFailureBranch}

## B. Typed-edge / index diagnostics (DG-8..DG-10, DG-21)

- [x] A typed-loading failure names the failing package(s) instead of a bare N/M count {id: DG-8, symbol: EnrichTypedEdges, package: internal/fusion, reachable_from: TestEnrichTypedEdges_LoadFailure}
- [x] Discovery gaps and out-of-scope edge drops are carried on `TypedEdgeStatus`, not just logged to stderr, so `/statusz` and `doctor` can say why a graph is syntax-only {id: DG-9, symbol: (*Pipeline).noteTypedEdgeStatus, package: internal/fusion, reachable_from: TestNoteTypedEdgeStatus_CarriesDiscoveryGaps}
- [x] Last-known typed-edge status persists across a daemon restart (SQLite-backed, not memory-only) {id: DG-10, symbol: (*Pipeline).TypedEdgeStatus, package: internal/fusion, reachable_from: TestSQLiteStore_TypedEdgeStatusSurvivesRestart}
- [x] O39/O40 (selector-method call edges and interface dispatch): `MentionConfidence` resolves a value-receiver method called via a selector on a value and on a pointer, a pointer-receiver method called via a selector on a pointer and on an addressable value, and an interface-dispatch call, all to `EdgeConfidenceResolved` — reproduced against the real `fusion.Pipeline.Ingest` → `EnrichTypedEdges` → `MentionConfidence` path, no hand-built graph {id: DG-21, symbol: MentionConfidence, package: internal/analysis/callgraph, reachable_from: TestMentionConfidence_SelectorOnValueAndPointer}

## C. Identity resolution / doctor correctness (DG-11..DG-15)

- [x] Post-edit identity resolution never lets an unreachable candidate win over a reachable one (O15 regression) {id: DG-11, symbol: renderPostEdit, package: cmd/fathom, reachable_from: TestRenderPostEdit_PrefersReachableCandidateOverBasename}
- [x] The identity-resolution timeout survives load (a slow daemon), serving a stale cached candidate rather than falling back to basename matching {id: DG-12, symbol: resolveIndexedCandidateCached, package: cmd/fathom, reachable_from: TestResolveIndexedCandidateCached_ServesStaleOnTimeoutOverBasenameFallback}
- [x] `doctor`'s `binary_fresh` remedy names which side of HEAD is behind instead of a directionless generic remedy (C2/DG-13) {id: DG-13, symbol: checkBinaryFresh, package: cmd/fathom, reachable_from: TestCheckBinaryFresh}
- [x] `fleetWiringNotes` detects fathom hooks wired at user scope instead of false-negatifying them as absent (O15, the doctor-facing half) {id: DG-14, symbol: fleetWiringNotes, package: cmd/fathom, reachable_from: TestFleetWiringNotes_DetectsUserScopeWiring}
- [x] `unproven_claims`'s reason carries the evidence timestamp it already computes, instead of dropping it {id: DG-15, symbol: checkUnprovenClaims, package: cmd/fathom, reachable_from: TestCheckUnprovenClaims_ReasonIncludesEvidenceTimestamp}

## D. Metrics and daemon lifecycle (DG-16, DG-19)

- [x] `/metrics` exports live resident-memory gauges — never a fabricated zero, never `getrusage`'s peak Maxrss mislabeled as current: `process_resident_memory_bytes`/`process_phys_footprint_bytes` (darwin, `proc_pid_rusage`/`RUSAGE_INFO_V2`) or `process_resident_memory_bytes` alone (linux, `/proc/self/status` VmRSS); an optional, honestly-named `process_max_resident_memory_bytes` peak gauge; omitted entirely where no live reading exists {id: DG-16, symbol: processResidentMemoryBytes, package: cmd/fathom, reachable_from: TestProcessResidentMemoryBytes_ReturnsPositiveValue}
- [x] Startup bind retries an `EADDRINUSE` failure with backoff for up to 10s (a self-restarted or redeployed previous instance still exiting), logging each attempt at warn, before failing with the original error {id: DG-19, symbol: listenWithBackoff, package: cmd/fathom, reachable_from: TestListenWithBackoff_RetriesThenSucceeds}

## Owner-decision (deferred — not applied here)

- [x] OWN-1 — deregister stale fleet service records (`demo`'s vanished corpus root, `esp-thumper`, scratchpad-rooted services) via the existing `remove_service` MCP tool {id: OWN-1, oracle: lexical, path: .fathom/observations.md, pattern: "OWN-1 executed"}
- [x] OWN-2 — `docs/intent/questions.md` Q21: amend ADR-016's 150-token obligations-injection ceiling to a measured value, or accept the overrun, is a standing product decision {id: OWN-2, oracle: lexical, path: docs/intent/invariants.md, pattern: "320-token whole-payload ceiling"}
- [-] OWN-4 — `.claude/settings.json` 6-entry wiring-drift diff: run `fathom config audit` and, if benign, `fathom fleet reconcile --repo <path> --apply --rebaseline` against the live settings file {id: OWN-4, disposition: deferred, reason: "diagnosed benign 2026-09-24: the six hook entries changed through the project's own commits (shim spelling) after the 2026-09-12 recording in a gitignored per-checkout ledger; origin/main audits clean; the documented remedy could not clear hook drift until the reconcile fix in this change; the volume checkout re-records on its next update"}

## Status

Wave landed 2026-09-23: DG-1..DG-21 (P0/P1/P2) merged from four parallel
integration branches (dg-a: DG-1..DG-6, DG-16, DG-19; dg-b: DG-11, DG-12;
dg-c: DG-8, DG-9, DG-10, DG-17, DG-21; dg-d: DG-13, DG-14, DG-15). Owner
items (OWN-1, OWN-2, OWN-4) recorded and deferred per the plan's own
amendment (e) — never actioned by this sweep. See `.fathom/observations.md`
O112 for the sweep's evidence and root-cause narrative.
