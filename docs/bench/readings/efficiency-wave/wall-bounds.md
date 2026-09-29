# EW-7: the wall-bound test sweep

EW-7 reading: 189 wall-clock test sites swept across cmd/, internal/, pkg/ and scripts/ (151 moved to CPU time, a progress count or a large safety bound; 38 kept with a named reason). The row's 38 (12 at millisecond scale) counted elapsed-time assertions only (46 found here, 33 moved, 13 kept; 14 of them are at millisecond scale, bounds under 1 s, and 12 of those moved, the other 2 being the hook-deadline property, which is wall-clock by definition); the timer-raced waits, deadline polls and context bounds are the same failure class and are swept with them.

This is the row's oracle file; the table below is its evidence.

## What counts as a wall-bound assertion

A test statement whose pass or fail is decided by real elapsed time against a fixed bound. Five shapes, each grepped for in `*_test.go` (and `scripts/*_test.sh`) under `cmd/`, `internal/` and `pkg/`; `third_party/` is vendored upstream code and out of scope:

| Class | Shape | Grep |
|---|---|---|
| E | an elapsed or remaining time compared to a bound (`time.Since`, `time.Until`, a returned duration) | `time.Since\|time.Until\|elapsed\|took\|waited` next to `>` or `<` |
| T | a timer-raced wait whose timeout branch fails the test (`case <-time.After(d): t.Fatal`); a timeout branch that is the pass path is a negative wait | `time.After(` |
| P | a poll against a deadline (`deadline := time.Now().Add(d)`) that fails when it passes | `time.Now().Add(` next to `After(deadline)` / `Before(deadline)` |
| C | a `context.WithTimeout`/`WithDeadline` used as a hang guard, or as the input the test exercises | `context.WithTimeout\|WithDeadline` |
| S | a stub whose artificial delay a wall-clock bound was measured against | the stub next to a class E site |

Out of scope, and named: 75 `time.Sleep` calls in 48 test files (72 of them under a second), used as stagger, settle or "let the watcher register" waits. A sleep asserts nothing; it can only make a race less likely, so it cannot flip a gate red by itself, and the fix for a too-short one is a progress wait, one site at a time, not a sweep.

## The rule the sweep applied

- **A wait on progress fails only after `waitSafety` (one minute)**, a hang guard sized for a machine starved for a minute, never a latency the code is held to. The bound is a per-package test constant (`waitsafety_test.go`); a timeout under 30 s that failed a test was moved to it. Bounds already at 30 s or more stay.
- **"X does not wait for Y" is proven by progress, not by a stopwatch.** Y stays blocked until the test releases it (or never answers on its own), so X returning at all is the claim, and `returnsUnaided` (a per-package test helper: run the call, fail after `waitSafety`) turns a wedge into a named failure. Where the claim is "the budget was applied", the stub records the time its ctx had left on entry and the test asserts it was within the budget, which load can only shrink. A regression that ignored the budget was checked to fail (see Verification).
- **Cost is CPU time.** A render or hook budget reads `processCPUTime` and skips the bound where there is no reading. Wall clock stays only for properties that are wall-clock by definition (a deadline), named below.
- **A deadline window is a bracket.** "Deadline is now+60 s" is asserted as between the clock readings either side of the call, which a stalled machine only widens.
- **A negative wait can only under-detect.** A timer whose expiry is the pass path cannot fail a correct build, so it stays.
- **Inputs are not assertions.** A short `context.WithTimeout` handed to the code under test is the parameter the test exercises; its expiry is not compared to anything.

## Kept with a reason, in one place

The sites that stay wall-bound and why: the hook deadline property (E, `hookguard_test.go`, the property is a wall-clock deadline; residual load risk named in its row), the large hang guards (a subprocess that sleeps 30 s against a bound of 5 to 10 s and a deadline of 200 to 300 ms, 16x to 50x; typed-edge, tree-sitter and replay bounds of 10 s or more), the lower bound in the stagger test, the negative waits, the inputs, and the bounds already at 30 s or more. Each is a row below marked `kept`.

## The sweep

Columns: the site by file and test function, its class, what it was, what became of it, and how or why.


### cmd/fathom

| Site | Class | Was | Disposition | How or why |
|---|---|---|---|---|
| `admission_http_test.go::TestClientDisconnectCancelsReadWork` | E | rpcContext deadline outside a 100-250 ms window | moved: bracket | the deadline is 250 ms from the instant rpcContext read the clock: it must lie between the readings taken either side of the call |
| `admission_http_test.go::TestClientDisconnectCancelsReadWork` | T | timer 1 s | moved: large safety bound | the wait on progress (a channel, a state) fails the test only after waitSafety (1 min); the awaited event either happens or never does |
| `admission_http_test.go::TestDaemonServerTimeoutsAndLoopbackHost` | E | /rpc write deadline outside 140-150 s from now | moved: bracket | 150 s from the instant the handler read the clock: between the readings either side of the request |
| `calllog_rotate_hygiene_test.go::TestCallLogRotationOffRequestPath` | T | timer 5 s | moved: large safety bound | the wait on progress (a channel, a state) fails the test only after waitSafety (1 min); the awaited event either happens or never does |
| `console_feeders_test.go::TestFeedersIdleWithoutSubscribers` | T | timer 10 s | moved: large safety bound | the wait on progress (a channel, a state) fails the test only after waitSafety (1 min); the awaited event either happens or never does |
| `console_live_test.go::TestConsoleLive_PanelsCarrySourcesAndMarkWithoutMoving` | C | SSE request ctx 5 s | moved: large safety bound | hang guard on the request, now waitSafety |
| `console_memo_test.go::TestConsoleMemoWaiterHonoursContext` | C | waiter ctx 30 ms | kept: input | the waiter's own deadline, the input of the test |
| `console_memo_test.go::TestConsoleMemoWaiterHonoursContext` | E | a memo waiter stayed over 1 s after its ctx ended | moved: progress | the first computation stays blocked, so a waiter that outstayed its ctx could not return |
| `console_memo_test.go::TestConsoleMemoWaiterHonoursContext` | P | poll deadline 2 s | moved: large safety bound | the poll on a state (or on the spans a hung RPC settles into) is bounded by waitSafety |
| `console_memo_test.go::TestConsoleMemo_ColdKeyNeverBlocksWarmKey` | T | timer 2 s | moved: large safety bound | the wait on progress (a channel, a state) fails the test only after waitSafety (1 min); the awaited event either happens or never does |
| `console_model_stale_test.go::TestColdModelWaitHonoursTheRequest` | C | cold reader request ctx 100 ms | kept: input | the request context that ends while the build is blocked |
| `console_model_stale_test.go::TestColdModelWaitHonoursTheRequest` | T | timer 3 s | moved: large safety bound | the wait on progress (a channel, a state) fails the test only after waitSafety (1 min); the awaited event either happens or never does |
| `console_model_stale_test.go::TestObservatoryModelNeverServesAnEmptyModel` | T | timer 5 s | moved: large safety bound | the wait on progress (a channel, a state) fails the test only after waitSafety (1 min); the awaited event either happens or never does |
| `console_model_stale_test.go::TestObservatoryModelServesStaleWhileRebuilding` | E | a stale read waited over 1 s for the rebuild | moved: progress | the rebuild stays blocked, so a read that waited could not return (eight reads under returnsUnaided) |
| `console_model_stale_test.go::TestObservatoryModelServesStaleWhileRebuilding` | T | timer 2 s | moved: large safety bound | the wait on progress (a channel, a state) fails the test only after waitSafety (1 min); the awaited event either happens or never does |
| `console_operate_tenancy_test.go::TestOperateResourcesOffTheRequestPath` | E | a stale read waited over 1 s for the census | moved: progress | the census build stays blocked until the test releases it, so a read that waited could not return (returnsUnaided) |
| `console_operate_tenancy_test.go::TestOperateResourcesOffTheRequestPath` | T | timer 2 s | moved: large safety bound | the wait on progress (a channel, a state) fails the test only after waitSafety (1 min); the awaited event either happens or never does |
| `console_plans_inprocess_test.go::TestConsolePlanVerifyPanicFinishes` | T | timer 10 s | moved: large safety bound | the wait on progress (a channel, a state) fails the test only after waitSafety (1 min); the awaited event either happens or never does |
| `console_plans_inprocess_test.go::TestConsolePlansVerifyInProcessNoRequeue` | E | a hung verify_plan held over 5 s | moved: progress | the rpc never answers on its own, so returning is the 100 ms deadline (returnsUnaided) |
| `console_plans_inprocess_test.go::TestConsolePlansVerifyInProcessNoRequeue` | T | timer 30 s | kept: already large | the timeout branch fails the test and the bound is already 30 s |
| `console_plansdata_test.go::TestConsolePlans_HeadOnlyChangeReusesDaemonVerdicts` | T | timer 30 s | kept: already large | the timeout branch fails the test and the bound is already 30 s |
| `console_plansdata_test.go::TestConsolePlans_VerifyEventsOnlyWhenVerdictsChange` | T | timer 30 s | kept: already large | the timeout branch fails the test and the bound is already 30 s |
| `console_project_index_pool_test.go::TestProjectIndexRunsInTheJobPool` | T | timer 5 s | moved: large safety bound | the wait on progress (a channel, a state) fails the test only after waitSafety (1 min); the awaited event either happens or never does |
| `console_project_index_pool_test.go::TestProjectIndexWaitHonoursTheRequest` | C | request ctx 200 ms | kept: input | the request context that ends while the read is blocked; its expiry is what the test exercises |
| `console_project_index_pool_test.go::TestProjectIndexWaitHonoursTheRequest` | T | timer 3 s | moved: large safety bound | the wait on progress (a channel, a state) fails the test only after waitSafety (1 min); the awaited event either happens or never does |
| `console_project_index_pool_test.go::TestProjectIndexWaitHonoursTheRequest` | T | timer 5 s | moved: large safety bound | the wait on progress (a channel, a state) fails the test only after waitSafety (1 min); the awaited event either happens or never does |
| `console_refactor_pool_test.go::TestRefactorReportBoundedPool` | E | a hung git held the page over 5 s | kept: large safety bound | 5 s is 25x the 200 ms git timeout it guards and under the 30 s the hung git sleeps |
| `console_test.go::TestConsoleBudgets` | E | warm page render over 150 ms (best of 3) | moved: CPU time | process CPU time (processCPUTime) around each render; skipped where the platform has no CPU reading, the size bound stays |
| `daemon_lifecycle_test.go::TestDaemonStartPauseResume` | P | poll deadline 5 s | moved: large safety bound | the poll on a state (or on the spans a hung RPC settles into) is bounded by waitSafety |
| `daemon_lifecycle_test.go::TestStopKillsDaemonIgnoringSIGTERM` | P | poll deadline 10 s | moved: large safety bound | the poll on a state (or on the spans a hung RPC settles into) is bounded by waitSafety |
| `daemon_lifecycle_test.go::TestStopKillsDaemonIgnoringSIGTERM` | T | timer 5 s | moved: large safety bound | the wait on progress (a channel, a state) fails the test only after waitSafety (1 min); the awaited event either happens or never does |
| `daemon_records_retention_test.go::TestUnmanagedServeHTTPRemovesRecordOnSIGTERM` | P | poll deadline 60 s | kept: already large | a 60 s poll, already a large safety bound |
| `daemonstdio_test.go::TestStdioRotationStreamsAndHardCaps` | P | poll deadline 5 s | moved: large safety bound | the poll on a state (or on the spans a hung RPC settles into) is bounded by waitSafety |
| `derivedoutcome_deadline_test.go::TestDerivedCommitsGitDeadlineAndRetry` | E | a hung git held the model read over 5 s | kept: large safety bound | 5 s is 16x the 300 ms git timeout and under the 30 s the hung git sleeps |
| `discoveries_test.go::TestSubagentDiscoveries_ReturnsWithinDeadlineWhenIndexIsSlow` | E | subagentDiscoveries took over deadline+2 s with a slow index | moved: progress | the resolver never answers on its own (held until the client leaves), so returning is the deadline firing |
| `discoveries_test.go::TestSubagentPrimer_ShipsAloneWithIdentityIntactWhenDiscoveriesIndexIsSlow` | E | subagentPrimer took over deadline+3 s with a slow index | moved: progress | same never-answering resolver |
| `discoveries_test.go::fakeSlowResolverDaemon` | S | resolver sleep deadline+3 s | moved: progress | the handler holds until the client leaves |
| `edithook_pre_freshness_bench_test.go::TestEditHookPre_FreshnessRefresh_P50Under150ms` | E | p50 of the pre-edit + refresh hook over 150 ms | moved: CPU time | p50 of the CPU time of 20 runs (processCPUTime); skipped where there is no CPU reading |
| `governor_diag_test.go::TestDiagCaptureRateLimitedAndRetained` | E | the exit capture took over 1 s | moved: dropped | the file set beside it (heap and in-flight table only, no trace or goroutine dump, the slow parts) is what keeps the capture cheap |
| `governor_diag_test.go::TestProductionDiagCaptureDoesNotDeadlockGovernor` | T | timer 5 s | moved: large safety bound | the wait on progress (a channel, a state) fails the test only after waitSafety (1 min); the awaited event either happens or never does |
| `hookguard_fetch_test.go::TestFetchServicesNeverRetriesAfterTimeout` | E | fetchServices took over 1.5 s against a 200 ms client | moved: dropped | the hung daemon holds every request until the client leaves, so "it retried" is a second request and the hit count (1) in the same test is the proof |
| `hookguard_test.go::TestHooksSilentWhenDisabled` | E | a disabled hook took over 250 ms | moved: dropped | "an immediate return" is the silent exit 0 with no RPC sent; the daemon's hit count in the same test is the proof |
| `hookguard_test.go::TestNoHookRPCOutlivesItsDescriptorBudget` | E | an RPC held open past budget+150 ms (budget = a descriptor budget x0.1) | kept: the property is a wall-clock deadline | no CPU or progress count says "abandoned on time"; both timestamps are taken by the stub daemon's own handler, so test-goroutine scheduling does not enter, only the handler noticing the disconnect (150 ms slack); residual load risk, named |
| `hookguard_test.go::TestNoHookRPCOutlivesItsDescriptorBudget` | E | hook wall time past budget+250 ms, only when getrusage is unavailable | kept: fallback | the CPU-time bound above it is the assertion wherever getrusage exists; this runs only on a platform without it |
| `hookguard_test.go::TestNoHookRPCOutlivesItsDescriptorBudget` | E | hook wall time past the descriptor's whole unscaled TimeoutSec | kept: large safety bound | the harness's own kill time (seconds to tens of seconds, unscaled): past it the harness would have killed the hook, and no load explains that |
| `hookguard_test.go::settledSpans` | P | poll deadline 2 s | moved: large safety bound | the poll on a state (or on the spans a hung RPC settles into) is bounded by waitSafety |
| `hookorigin_test.go::TestRPCSourcesScope` | T | timer 2 s | moved: large safety bound | the wait on progress (a channel, a state) fails the test only after waitSafety (1 min); the awaited event either happens or never does |
| `host_darwin_test.go::TestChildFootprintCountedTowardShedAndRefuse` | P | poll deadline 5 s | moved: large safety bound | the poll on a state (or on the spans a hung RPC settles into) is bounded by waitSafety |
| `host_darwin_test.go::TestChildFootprintCountedTowardShedAndRefuse` | T | timer 5 s | moved: large safety bound | the wait on progress (a channel, a state) fails the test only after waitSafety (1 min); the awaited event either happens or never does |
| `init_scip_test.go::TestRunSCIPProducerStopsAtItsDeadline` | C | producer ctx 200 ms | kept: input | the deadline handed to runSCIPProducer under test |
| `init_scip_test.go::TestRunSCIPProducerStopsAtItsDeadline` | E | a killed producer outlived its 200 ms deadline by over 10 s | kept: large safety bound | 10 s is 50x the deadline it guards and a third of the 30 s the hung child sleeps, so a regression shows as 30 s; only a 10 s stall of the whole test flips it |
| `killswitch_test.go::TestDisabledFileCheckedBeforeStoreOpen` | E | the kill-switch exit took 50 ms or more | moved: dropped | "under 50 ms" is the design's way of saying the switch is read before anything else; no store, no bind and no lifecycle row (asserted beside it) prove that |
| `lifecycle_findings_test.go::TestUnmanagedStoreWaitHonoursTTL` | T | timer 8 s | moved: large safety bound | the wait on progress (a channel, a state) fails the test only after waitSafety (1 min); the awaited event either happens or never does |
| `lifecycle_serve_test.go::TestSIGTERMDrainsWithinExitTimeout` | E | a drain took over 1 s against a 300 ms budget | moved: dropped | the shutdown hook blocks until its ctx ends and the exit channel, awaited under waitSafety, is the progress: the drain exiting at all is the budget firing |
| `lifecycle_serve_test.go::TestSIGTERMDrainsWithinExitTimeout` | T | timer 2 s | moved: large safety bound | the wait on progress (a channel, a state) fails the test only after waitSafety (1 min); the awaited event either happens or never does |
| `lifecycle_serve_test.go::TestServeHTTPWaitsForStoreWithoutExiting` | P | poll deadline 20 s | moved: large safety bound | the poll on a state (or on the spans a hung RPC settles into) is bounded by waitSafety |
| `lifecycle_serve_test.go::TestServeHTTPWaitsForStoreWithoutExiting` | T | timer 15 s | moved: large safety bound | the wait on progress (a channel, a state) fails the test only after waitSafety (1 min); the awaited event either happens or never does |
| `memscavenge_test.go::TestRunMemoryScavenger_CallsFreeOnEveryTick` | T | timer 2 s | moved: large safety bound | the wait on progress (a channel, a state) fails the test only after waitSafety (1 min); the awaited event either happens or never does |
| `memscavenge_test.go::TestRunMemoryScavenger_LogsHeapDelta` | T | timer 2 s | moved: large safety bound | the wait on progress (a channel, a state) fails the test only after waitSafety (1 min); the awaited event either happens or never does |
| `memscavenge_test.go::TestRunMemoryScavenger_ZeroIntervalIsNoOp` | T | timer 2 s | moved: large safety bound | the wait on progress (a channel, a state) fails the test only after waitSafety (1 min); the awaited event either happens or never does |
| `postcommit_enqueue_test.go::TestPostCommitEnqueuesRefresh` | E | the post-commit hook took over 3 s | moved: progress | the queued ingest stays blocked until cleanup, so a hook that waited on it could not return |
| `postcommit_enqueue_test.go::TestPostCommitEnqueuesRefresh` | T | timer 5 s | moved: large safety bound | the wait on progress (a channel, a state) fails the test only after waitSafety (1 min); the awaited event either happens or never does |
| `postcommit_enqueue_test.go::TestRefreshFileDefersToQueuedServiceRefresh` | P | poll deadline 5 s | moved: large safety bound | the poll on a state (or on the spans a hung RPC settles into) is bounded by waitSafety |
| `prompthook_test.go::TestPromptHook_SlowDaemonReturnsWithinBudget` | E | prompt-hook took over 2 s against a 300 ms shared deadline | moved: progress | the slow file's daemon call never answers on its own, so the hook returning is the shared deadline firing |
| `prompthook_test.go::fakeSlowPromptDaemon` | S | slow file delay 5 s | moved: progress | no delay: the daemon holds the call until the client leaves |
| `review_test.go::TestReviewNonInteractive` | E | a 1 s budget did not bound the run (over 2.5 s) | moved: progress | the budgeted body blocks until the run has returned, so runReview returning is the budget bounding it |
| `review_test.go::TestReviewNonInteractive` | S | slow git sleep 3 s | moved: progress | the body blocks until the test releases it after the run |
| `route_issue_shape_test.go::TestPromptHookRoutesIssueShapesToLocalize` | P | poll deadline 5 s | kept: input | the deadline handed to the hook under test (deadlineRemaining); its expiry is not asserted |
| `route_test.go::Decide` | S | slowDecider delay 2 s | moved: progress | no delay: the decider holds until its ctx ends |
| `route_test.go::TestRouteHandler_ConfigurableTimeout` | E | route handler took 1 s or more against a 100 ms budget | moved: progress | the decider never answers on its own, so the handler returning is the timeout firing, and the time the decider's ctx had left on entry (at most 100 ms, never the old fixed 3 s) says which timeout |
| `routeline_test.go::TestPromptHook_RouteLineBands` | P | poll deadline 5 s | kept: input | the deadline handed to routeLine under test; its expiry is not asserted |
| `rtb_reading_findings_test.go::TestWarmupRefusesHookOriginsAtOnce` | E | a hook origin waited over 500 ms for the warm-up slot | moved: progress | the gate's queue wait is an hour and the slot stays held, so a queued hook cannot return: returning is "refused at once" (returnsUnaided, waitSafety) |
| `scavengelog_test.go::TestScavengerLogsReleasedAndFootprint` | P | poll deadline 5 s | moved: large safety bound | the poll on a state (or on the spans a hung RPC settles into) is bounded by waitSafety |
| `selfrestart_tenant_test.go::TestWatchOwnBinaryHashesOnlyOnStatChange` | T | timer 5 s | moved: large safety bound | the wait on progress (a channel, a state) fails the test only after waitSafety (1 min); the awaited event either happens or never does |
| `selfrestart_tenant_test.go::TestWatchOwnBinaryHashesOnlyOnStatChange` | T | timer 5 s | moved: large safety bound | the wait on progress (a channel, a state) fails the test only after waitSafety (1 min); the awaited event either happens or never does |
| `selfrestart_tenant_test.go::TestWatchOwnBinaryHashesOnlyOnStatChange` | T | timer 5 s | moved: large safety bound | the wait on progress (a channel, a state) fails the test only after waitSafety (1 min); the awaited event either happens or never does |
| `selfrestart_tenant_test.go::TestWatchOwnBinaryRetriesAFailedHash` | T | timer 5 s | moved: large safety bound | the wait on progress (a channel, a state) fails the test only after waitSafety (1 min); the awaited event either happens or never does |
| `selfrestart_tenant_test.go::TestWatchOwnBinaryReverifiesAfterATransientRefusal` | T | timer 5 s | moved: large safety bound | the wait on progress (a channel, a state) fails the test only after waitSafety (1 min); the awaited event either happens or never does |
| `selfrestart_tenant_test.go::TestWatchOwnBinaryUnreadableLedgerFallsBackToUptime` | T | timer 1 s | moved: progress first; window kept as a negative wait | the check now waits on progress first (the watcher has named the unreadable ledger) and keeps this 1 s as a negative window: nothing may be adopted; expiry is the pass path |
| `selfrestart_tenant_test.go::TestWatchOwnBinaryUnreadableLedgerFallsBackToUptime` | T | timer 5 s | moved: large safety bound | the wait on progress (a channel, a state) fails the test only after waitSafety (1 min); the awaited event either happens or never does |
| `selfrestart_tenant_test.go::waitHashes` | P | poll deadline 10 s | moved: large safety bound | the poll on a state (or on the spans a hung RPC settles into) is bounded by waitSafety |
| `selfrestart_test.go::TestSelfRestartAdoptsAVerifiedBinary` | T | timer 5 s | moved: large safety bound | the wait on progress (a channel, a state) fails the test only after waitSafety (1 min); the awaited event either happens or never does |
| `selfrestart_test.go::TestSelfRestartRefusesABinaryThatCannotRun` | P | poll deadline 2 s | moved: large safety bound | the poll on a state (or on the spans a hung RPC settles into) is bounded by waitSafety |
| `selfrestart_test.go::TestWatchOwnBinary_AdoptLineIncludesBuildAge` | T | timer 5 s | moved: large safety bound | the wait on progress (a channel, a state) fails the test only after waitSafety (1 min); the awaited event either happens or never does |
| `servehttp_test.go::TestServeHTTPEventsHello_ThroughLoggingMiddleware` | C | GET /events ctx 5 s | moved: large safety bound | hang guard on the request, now waitSafety |
| `servehttp_test.go::TestServeHTTPEventsHello_ThroughLoggingMiddleware` | T | timer 1.5 s | moved: large safety bound | the wait on progress (a channel, a state) fails the test only after waitSafety (1 min); the awaited event either happens or never does |
| `sessionledger_test.go::TestTranscriptSummaryFrom_NonRegularFile_Refused` | T | timer 3 s | moved: large safety bound | the wait on progress (a channel, a state) fails the test only after waitSafety (1 min); the awaited event either happens or never does |
| `stophook_test.go::TestRunStopHook_SlowVerifyDegradesToWarning` | E | runStopHook blocked 300 ms or more on a slow verify | moved: progress | the fake daemon holds verify_plan until the client leaves, so returning is "bailed at the client timeout" |
| `stophook_test.go::slowStopDaemon` | S | verify_plan sleep 300 ms | moved: progress | the handler holds until the client leaves |
| `unmanaged_test.go::TestUnmanagedServeHTTPExitsWhenParentKilled` | E | serve-http still alive over 10 s after its parent was killed | moved: large safety bound | a poll for the exit, bound waitSafety |
| `unmanaged_test.go::TestUnmanagedServeHTTPExitsWhenParentKilled` | P | poll deadline 60 s | kept: already large | a 60 s poll for the daemon to listen, already a large safety bound |
| `unmanaged_test.go::TestUnmanagedServeHTTPExitsWhenParentKilled` | T | timer 3 s | moved: large safety bound | the kqueue wait is now made on darwin only, where its expiry is the failure; off darwin only the poll path exists and there was nothing to wait for |
| `unmanaged_test.go::TestUnmanagedServeHTTPExitsWhenParentKilled` | T | timer 3 s | moved: large safety bound | the wait on progress (a channel, a state) fails the test only after waitSafety (1 min); the awaited event either happens or never does |
| `waituntil_test.go::waitUntil` | P | poll deadline 2 s | moved: large safety bound | the poll on a state (or on the spans a hung RPC settles into) is bounded by waitSafety |

### cmd/scenariobench

| Site | Class | Was | Disposition | How or why |
|---|---|---|---|---|
| `signals_test.go::TestInstallKillSwitch_InvokesOnKillSignalOnSIGTERM` | T | timer 5 s | moved: large safety bound | the wait on progress (a channel, a state) fails the test only after waitSafety (1 min); the awaited event either happens or never does |
| `signals_test.go::TestInstallKillSwitch_StopDisarms` | T | timer 5 s | moved: large safety bound | the wait on progress (a channel, a state) fails the test only after waitSafety (1 min); the awaited event either happens or never does |
| `signals_test.go::TestInstallKillSwitch_StopDisarms` | T | timer 200 ms | kept: negative wait or stub work | a negative wait: the stopped kill switch must NOT fire again, so the timer expiring is the pass path; it can only under-detect, never fail a correct build |

### internal/console

| Site | Class | Was | Disposition | How or why |
|---|---|---|---|---|
| `events_test.go::TestEventBus_GapNamesEverySource` | C | SSE request ctx 5 s | moved: large safety bound | hang guard on the request, now waitSafety |
| `events_test.go::TestEventBus_ReplayBoundAndKeepalive` | C | SSE request ctx 5 s | moved: large safety bound | hang guard on the request, now waitSafety |
| `events_test.go::TestEventBus_ReplayBoundAndKeepalive` | T | timer 5 s | moved: large safety bound | the wait on progress (a channel, a state) fails the test only after waitSafety (1 min); the awaited event either happens or never does |
| `events_test.go::TestEventBus_ReplaySurvivesDaemonRestart` | C | SSE request ctx 5 s | moved: large safety bound | hang guard on the request, now waitSafety |
| `jobs_test.go::TestJobs_BoundedSingleFlightDropsSuperseded` | T | timer 2 s | moved: large safety bound | the wait on progress (a channel, a state) fails the test only after waitSafety (1 min); the awaited event either happens or never does |
| `jobs_test.go::TestJobs_BoundedSingleFlightDropsSuperseded` | T | timer 5 s | moved: large safety bound | the wait on progress (a channel, a state) fails the test only after waitSafety (1 min); the awaited event either happens or never does |
| `jobs_test.go::waitFor` | P | poll deadline 5 s | moved: large safety bound | the poll on a state (or on the spans a hung RPC settles into) is bounded by waitSafety |

### internal/corpus

| Site | Class | Was | Disposition | How or why |
|---|---|---|---|---|
| `corpus_test.go::TestReadFileCapped_StatsBeforeRead` | T | timer 5 s | moved: large safety bound | the wait on progress (a channel, a state) fails the test only after waitSafety (1 min); the awaited event either happens or never does |

### internal/extractor

| Site | Class | Was | Disposition | How or why |
|---|---|---|---|---|
| `tsextract_test.go::TestTSExtractor_PathologicalInputRecovery` | E | a pathological Extract took over 10 s | kept: large safety bound | 10 s against the parse deadline the test sets (1 ms class), a 10,000x margin |

### internal/extractor/tsengine

| Site | Class | Was | Disposition | How or why |
|---|---|---|---|---|
| `tsengine_test.go::TestEngine_HardBackstopAbandonsRunawayCall` | E | a pathological Run took over 30 s | kept: already large | 30 s against a 2 s hard backstop |
| `tsengine_test.go::TestParser_GuestBudgetBoundsPathologicalParse` | E | a pathological parse took over 30 s | kept: already large | 30 s against a 1 ms guest budget that expires at the first progress check |

### internal/fusion

| Site | Class | Was | Disposition | How or why |
|---|---|---|---|---|
| `ingeststate_test.go::TestFirstIngestKilledKeepsCorpusPath` | T | timer 10 s | moved: large safety bound | the wait on progress (a channel, a state) fails the test only after waitSafety (1 min); the awaited event either happens or never does |
| `ingeststate_test.go::TestWatermarkNotAdvancedByKilledIngest` | T | timer 10 s | moved: large safety bound | the wait on progress (a channel, a state) fails the test only after waitSafety (1 min); the awaited event either happens or never does |
| `proctree_test.go::TestCommandTreeKillsGrandchildOnCancel` | E | Wait took over 5 s after cancel | moved: progress | the grandchild sleeps for an hour holding the pipe Wait drains (killed in cleanup), so an unkilled tree leaves Wait blocked for good |
| `proctree_test.go::TestCommandTreeKillsGrandchildOnCancel` | P | poll deadline 3 s | moved: large safety bound | the poll on a state (or on the spans a hung RPC settles into) is bounded by waitSafety |
| `typedcalls_perf_test.go::TestTypedEdges_PerformanceGate` | E | wall ratio of the typed load to the syntax-only baseline | kept: already CPU | the enforced bound is 10x the baseline CPU; the wall ratio is only logged |
| `typedcalls_test.go::TestEnrichTypedEdges_NoGoModule` | E | EnrichTypedEdges on an empty dir took over 10 s | kept: large safety bound | the test's own comment says it is not strict; 10 s against a go/packages call that fails fast |
| `typedgate_test.go::TestCUEPassGatedAndSerialized` | T | timer 60 s | kept: already large | the timeout branch fails the test and the bound is already 60 s |
| `typedpass_serial_test.go::TestAcquireTypedPassSlot_CancelWhileWaiting` | T | timer 2 s | moved: large safety bound | the wait on progress (a channel, a state) fails the test only after waitSafety (1 min); the awaited event either happens or never does |
| `typedpass_serial_test.go::TestAcquireTypedPassSlot_SerializesAndCounts` | P | poll deadline 1 s | moved: large safety bound | the poll on a state (or on the spans a hung RPC settles into) is bounded by waitSafety |
| `typedpass_serial_test.go::TestAcquireTypedPassSlot_SerializesAndCounts` | T | timer 100 ms | kept: negative wait or stub work | a negative wait: the second acquire must NOT return before the first is released; expiry is the pass path |
| `typedpass_serial_test.go::TestAcquireTypedPassSlot_SerializesAndCounts` | T | timer 2 s | moved: large safety bound | the wait on progress (a channel, a state) fails the test only after waitSafety (1 min); the awaited event either happens or never does |

### internal/indexbench

| Site | Class | Was | Disposition | How or why |
|---|---|---|---|---|
| `mcpprobe_test.go::TestProbeReferencesViaMCP_GoplsProse` | C | probe ctx 10 s | moved: large safety bound | hang guard on the probe, now waitSafety |
| `mcpprobe_test.go::TestProbeReferencesViaMCP_MultipleSymbolsAndSites` | C | probe ctx 10 s | moved: large safety bound | hang guard on the probe, now waitSafety |
| `mcpprobe_test.go::TestProbeReferencesViaMCP_SerenaJSON` | C | probe ctx 10 s | moved: large safety bound | hang guard on the probe, now waitSafety |
| `mcpprobe_test.go::TestProbeReferencesViaMCP_UnknownToolRefuses` | C | probe ctx 10 s | moved: large safety bound | hang guard on the probe, now waitSafety |
| `replay_test.go::TestPayloadReplay` | E | a replay row over 10x its authored budget (1 s floor) | kept: large safety bound | the authored budget is the contract; the ceiling is 10x with a 1 s floor and the file says bytes, counts and elisions are the gate while time is a smoke alarm |

### internal/mcp

| Site | Class | Was | Disposition | How or why |
|---|---|---|---|---|
| `admission_test.go::TestDisconnectStopsTheHandler` | P | poll deadline 2 s | moved: large safety bound | the poll on a state (or on the spans a hung RPC settles into) is bounded by waitSafety |
| `admission_test.go::TestDisconnectStopsTheHandler` | T | timer 2 s | moved: large safety bound | the wait on progress (a channel, a state) fails the test only after waitSafety (1 min); the awaited event either happens or never does |
| `admission_test.go::TestRefusalIsNamedNeverEmpty` | C | caller ctx 30 ms | kept: input | the caller's own deadline, the input of the test |
| `arrival_test.go::TestNavMemoWaiterLeavesAtItsDeadline` | C | waiter ctx 100 ms | kept: input | the waiter's own deadline, the input of the test |
| `arrival_test.go::TestNavMemoWaiterLeavesAtItsDeadline` | T | timer 3 s | moved: large safety bound | the wait on progress (a channel, a state) fails the test only after waitSafety (1 min); the awaited event either happens or never does |
| `autorefresh_queue_test.go::TestEnsureNeverIngestsInline` | T | timer 2 s | moved: large safety bound | the wait on progress (a channel, a state) fails the test only after waitSafety (1 min); the awaited event either happens or never does |
| `autorefresh_queue_test.go::TestEnsureNeverIngestsInline` | T | timer 5 s | moved: large safety bound | the wait on progress (a channel, a state) fails the test only after waitSafety (1 min); the awaited event either happens or never does |
| `autorefresh_queue_test.go::TestEnsureNeverIngestsInline` | T | timer 2 s | moved: large safety bound | the wait on progress (a channel, a state) fails the test only after waitSafety (1 min); the awaited event either happens or never does |
| `autorefresh_tenant_test.go::TestEnsureIngestSurvivesRequestCancelAndSerializes` | T | timer 2 s | moved: large safety bound | the wait on progress (a channel, a state) fails the test only after waitSafety (1 min); the awaited event either happens or never does |
| `autorefresh_tenant_test.go::TestEnsureIngestSurvivesRequestCancelAndSerializes` | T | timer 5 s | moved: large safety bound | the wait on progress (a channel, a state) fails the test only after waitSafety (1 min); the awaited event either happens or never does |
| `autorefresh_tenant_test.go::TestEnsureJoinersServeStaleDuringInflightIngest` | P | poll deadline 1.5 s | moved: progress | the 1.5 s poll for a counted "blocked" read is gone with the timers above; the reads are awaited under waitSafety |
| `autorefresh_tenant_test.go::TestEnsureJoinersServeStaleDuringInflightIngest` | T | timer 5 s | moved: large safety bound | the wait on progress (a channel, a state) fails the test only after waitSafety (1 min); the awaited event either happens or never does |
| `autorefresh_tenant_test.go::TestEnsureJoinersServeStaleDuringInflightIngest` | T | timer 1 s | moved: progress | a 1 s timer used to count a read as "blocked"; every read now must return while the ingest is held (waitSafety is the hang guard) and the count of unreturned reads is named on failure |
| `autorefresh_tenant_test.go::TestEnsureJoinersServeStaleDuringInflightIngest` | T | timer 1 s | moved: progress | a 1 s timer used to count a read as "blocked"; every read now must return while the ingest is held (waitSafety is the hang guard) and the count of unreturned reads is named on failure |
| `autorefresh_test.go::settle` | P | poll deadline 5 s | moved: large safety bound | the poll on a state (or on the spans a hung RPC settles into) is bounded by waitSafety |
| `ingest_slot_test.go::TestIngestToolHoldsTheIngestSlot` | T | timer 5 s | moved: large safety bound | the wait on progress (a channel, a state) fails the test only after waitSafety (1 min); the awaited event either happens or never does |
| `new_tools_test.go::TestCallLogger_TimestampValid` | E | a call-log timestamp over 10 s in the past | moved: bracket | the entry is stamped during the call: between the clock readings either side of it |
| `refreshqueue_failure_test.go::TestQueuedIngestFailureIsNeverSilent` | P | poll deadline 5 s | moved: large safety bound | the poll on a state (or on the spans a hung RPC settles into) is bounded by waitSafety |

### internal/memo

| Site | Class | Was | Disposition | How or why |
|---|---|---|---|---|
| `flight_test.go::TestGraphFor_NestedFlightAtShedNeverWedges` | C | setup ctx 2 s | moved: large safety bound | hang guard on the snapshot and graph loads, now waitSafety |
| `flight_test.go::TestGraphFor_NestedFlightAtShedNeverWedges` | C | token waiter ctx 50 ms | kept: input | the waiter's own deadline, the input of the test |
| `flight_test.go::TestGraphFor_NestedFlightAtShedNeverWedges` | E | a waiter for the rebuild token outstayed its ctx by over 1 s | moved: progress | the typed load holds the token until the gate closes, so a waiter that outstayed its ctx could not return |
| `flight_test.go::TestGroup_KeepNeedsExpectedCostAndRefuseCancelsFlights` | T | timer 1 s | moved: large safety bound | the wait on progress (a channel, a state) fails the test only after waitSafety (1 min); the awaited event either happens or never does |
| `flight_test.go::TestGroup_KeepNeedsExpectedCostAndRefuseCancelsFlights` | T | timer 30 ms | kept: negative wait or stub work | a negative wait: the flight must NOT be cancelled yet; expiry is the pass path |
| `flight_test.go::TestGroup_KeepNeedsExpectedCostAndRefuseCancelsFlights` | T | timer 1 s | moved: large safety bound | the wait on progress (a channel, a state) fails the test only after waitSafety (1 min); the awaited event either happens or never does |
| `memo_test.go::TestGroup_StampedeOneBuildCtxAwareWaiters` | C | waiter ctx 20 ms | kept: input | the waiter's own deadline, the input of the test |
| `memo_test.go::TestGroup_StampedeOneBuildCtxAwareWaiters` | E | a ctx-bound waiter took over 2 s to leave | moved: progress | the flight's build stays blocked until released, so a waiter that ignored its ctx could not return |
| `memo_test.go::TestGroup_StampedeOneBuildCtxAwareWaiters` | P | poll deadline 2 s | moved: large safety bound | the poll on a state (or on the spans a hung RPC settles into) is bounded by waitSafety |
| `memo_test.go::TestGroup_StampedeOneBuildCtxAwareWaiters` | T | timer 2 s | moved: large safety bound | the wait on progress (a channel, a state) fails the test only after waitSafety (1 min); the awaited event either happens or never does |
| `memo_test.go::TestGroup_StampedeOneBuildCtxAwareWaiters` | T | timer 50 ms | kept: negative wait or stub work | the fake build's own work (50 ms) in a Keep flight, not an assertion |
| `refresh_test.go::TestGroup_InvalidationDuringAFlightIsNeverCached` | C | refresh-after-invalidate ctx 3 s | moved: large safety bound | hang guard on a fresh build, now waitSafety |
| `refresh_test.go::TestGroup_UpdateRunsOutsideTheRegistryLock` | T | timer 2 s | moved: large safety bound | the wait on progress (a channel, a state) fails the test only after waitSafety (1 min); the awaited event either happens or never does |

### internal/refresh

| Site | Class | Was | Disposition | How or why |
|---|---|---|---|---|
| `refresh_test.go::TestIngestSurvivesDisconnectAndCoalesces` | T | timer 2 s | moved: large safety bound | the wait on progress (a channel, a state) fails the test only after waitSafety (1 min); the awaited event either happens or never does |
| `refresh_test.go::TestRefresh_DemandOrderAndStartupStagger` | E | two staggered starts less than 100 ms apart | kept: a lower bound | a lower bound on elapsed time: load only widens the gap, so it cannot fail a correct queue |
| `refresh_test.go::TestRefresh_DemandOrderAndStartupStagger` | T | timer 60 ms | kept: negative wait or stub work | a negative wait: the paused job must not start; expiry is the pass path |
| `refresh_test.go::TestRefresh_DemandOrderAndStartupStagger` | T | timer 2 s | moved: large safety bound | the wait on progress (a channel, a state) fails the test only after waitSafety (1 min); the awaited event either happens or never does |
| `refresh_test.go::TestRefresh_OneInFlightOneFollowUp` | T | timer 50 ms | kept: negative wait or stub work | a negative wait: b must NOT start while a holds the heavy slot; expiry is the pass path |
| `refresh_test.go::TestRefresh_OneInFlightOneFollowUp` | T | timer 5 s | moved: large safety bound | poll for the idle state, bound is waitSafety |
| `refresh_test.go::TestRefresh_OneInFlightOneFollowUp` | T | timer 5 ms | kept: negative wait or stub work | a 5 ms poll tick inside the idle-state poll above, not an assertion |
| `refresh_test.go::waitStarted` | T | timer 5 s | moved: large safety bound | the wait on progress (a channel, a state) fails the test only after waitSafety (1 min); the awaited event either happens or never does |

### internal/scenariobench

| Site | Class | Was | Disposition | How or why |
|---|---|---|---|---|
| `concurrent_test.go::TestRunArmsConcurrently_ActuallyRunsConcurrently` | T | timer 2 s | moved: large safety bound | the wait on progress (a channel, a state) fails the test only after waitSafety (1 min); the awaited event either happens or never does |
| `guards_test.go::gitReadAt` | C | gitHistoryTimeout | kept: input | the product constant the guards scenario runs git history under |
| `liveprocs_test.go::TestKillAllLiveProcessGroups` | T | timer 5 s | moved: large safety bound | the wait on progress (a channel, a state) fails the test only after waitSafety (1 min); the awaited event either happens or never does |
| `sessionpid_test.go::waitDead` | T | timer 5 s | moved: large safety bound | the wait on progress (a channel, a state) fails the test only after waitSafety (1 min); the awaited event either happens or never does |
| `subject_reap_test.go::TestReapStaleScratchDaemon` | T | timer 5 s | moved: large safety bound | the wait on progress (a channel, a state) fails the test only after waitSafety (1 min); the awaited event either happens or never does |
| `subject_test.go::TestPrepareSubject_RefusesResetWithLiveSessionInWorktree` | T | timer 5 s | moved: large safety bound | the wait on progress (a channel, a state) fails the test only after waitSafety (1 min); the awaited event either happens or never does |

### internal/semantic

| Site | Class | Was | Disposition | How or why |
|---|---|---|---|---|
| `contract_tenant_test.go::TestCitedTextResolverStatsBeforeReading` | T | timer 3 s | moved: large safety bound | the wait on progress (a channel, a state) fails the test only after waitSafety (1 min); the awaited event either happens or never does |
| `decisionrank_test.go::Decide` | S | slowDecider sleep 5 s / 50 ms | moved: progress / kept | the tight-budget case uses a decider that never answers; the generous-budget case keeps 50 ms of stub work with a waitSafety budget |
| `decisionrank_test.go::TestRerankWithinTiers_BudgetBoundsWallTime` | E | a 100 ms budget took 500 ms or more against a 5 s decider | moved: progress | the decider never answers and records the time its ctx had left on entry (at most 100 ms): the call returning is the budget firing, and the ctx carried it |

### internal/store

| Site | Class | Was | Disposition | How or why |
|---|---|---|---|---|
| `sqlite_test.go::TestSQLite_ReaderDuringWriter` | T | timer 5 s | moved: large safety bound | the wait on progress (a channel, a state) fails the test only after waitSafety (1 min); the awaited event either happens or never does |

### internal/tenancy

| Site | Class | Was | Disposition | How or why |
|---|---|---|---|---|
| `gate_review_test.go::TestGate_TimingUsesInjectedClock` | E | tree ticket deadline outside 55-61 s from now | moved: bracket | 60 s from the instant Admit built the ctx: between the readings either side of the call |
| `gate_test.go::TestGate_ClassDeadlineAndRefuseRung` | E | tree ticket deadline outside 55-60 s from now; index ticket deadline off the client's by over 50 ms | moved: bracket | the gate reads one frozen instant, so each ticket has exactly 60 s (class) or 3 s (client) left and its wall-clock deadline lies between the readings either side of Admit |
| `gate_test.go::TestGate_HookNeverWaitsForHeavy` | E | a hook waited over 50 ms for a heavy slot | moved: progress | both heavy slots stay held, so an Admit that queued could not return (returnsUnaided) |
| `gate_test.go::TestGate_ObserveNeverRefuses` | E | observe mode waited over 50 ms | moved: progress | each ticket stays held, so an Admit that queued behind them could not return |
| `gate_test.go::TestGate_WaiterHonoursContext` | T | timer 2 s | moved: large safety bound | the wait on progress (a channel, a state) fails the test only after waitSafety (1 min); the awaited event either happens or never does |
| `gate_test.go::TestGate_WaiterHonoursContext` | T | timer 2 s | moved: large safety bound | the wait on progress (a channel, a state) fails the test only after waitSafety (1 min); the awaited event either happens or never does |
| `gate_test.go::waitFor` | P | poll deadline 2 s | moved: large safety bound | the poll on a state (or on the spans a hung RPC settles into) is bounded by waitSafety |

### pkg/client

| Site | Class | Was | Disposition | How or why |
|---|---|---|---|---|
| `obligations_tenant_test.go::TestLayerCoveragePathsLeaderHonoursCancel` | P | poll deadline 2 s | moved: large safety bound | the poll on a state (or on the spans a hung RPC settles into) is bounded by waitSafety |
| `obligations_tenant_test.go::TestLayerCoveragePathsLeaderHonoursCancel` | T | timer 2 s | moved: large safety bound | the wait on progress (a channel, a state) fails the test only after waitSafety (1 min); the awaited event either happens or never does |
| `obligations_tenant_test.go::TestObligationsHotPathReaderAgnosticTripwire` | T | timer 5 s | moved: large safety bound | the wait on progress (a channel, a state) fails the test only after waitSafety (1 min); the awaited event either happens or never does |
| `sourcesslot_test.go::TestScanSourcesAbandonStopsWalk` | T | timer 2 s | moved: large safety bound | the wait on progress (a channel, a state) fails the test only after waitSafety (1 min); the awaited event either happens or never does |
| `sourcesslot_test.go::TestSourcesScopeAbandonEndsWait` | T | timer 2 s | moved: large safety bound | the wait on progress (a channel, a state) fails the test only after waitSafety (1 min); the awaited event either happens or never does |
| `sourcesslot_test.go::TestSourcesScopeHoldsSlotUntilDone` | C | scope re-entry ctx 200 ms | moved: large safety bound | both slots are held by scopes, so a call that queued could not be served: the bound is now waitSafety, not a 200 ms window a stall could close |
| `sourcesslot_test.go::TestSourcesSlotHooksNeverWait` | C | third caller ctx 50 ms | kept: input | the caller's own deadline: it waits until it, and the test asserts the wait ended in DeadlineExceeded |
| `sourcesslot_test.go::TestSourcesSlotHooksNeverWait` | E | a hook-origin caller waited over 100 ms for a slot | moved: progress | both slots stay held, so a caller that queued could not return |

### scripts

| Site | Class | Was | Disposition | How or why |
|---|---|---|---|---|
| `gate-lease_test.sh::script` | E | the lease after a killed holder took 5 s or more | moved: progress | the killed holder's orphan sleeps for an hour (one process, killed by the test), so a lease still held would leave the next holder waiting for good; a 600-poll loop is only the hang guard |

## Verification

- `go test -race -p 2 -count=1 ./...` over the whole tree with the sweep in place: 117 packages ok, exit 0 (one later change, the failure text in `TestGate_HookNeverWaitsForHeavy`, re-ran `./internal/tenancy` green).
- Mutation checks, each in a scratch copy with `waitSafety` cut to 2 to 5 s, each turning the converted test red and then reverted: a hook queued for a full heavy class (`TestGate_HookNeverWaitsForHeavy`: "still waiting although nothing was released for it"); `rerankWithinTiers` ignoring its budget (`TestRerankWithinTiers_BudgetBoundsWallTime`); `routeDecideTimeout` back to a fixed 3 s (`TestRouteHandler_ConfigurableTimeout`: "the decider's ctx had 2.999985833s left"); the warm-up gate queueing a hook origin (`TestWarmupRefusesHookOriginsAtOnce`). EW-2's barrier failed the same way with its four reads run in sequence.
- Shell tests (`gate-lease_test.sh`, `build-cache_test.sh`, `deploy-wait_test.sh`) pass under macOS `/bin/bash` 3.2 and bash 5.

## Not done, and why

- The 75 `time.Sleep` waits (above) are not swept: none asserts anything, and each needs its own progress wait.
- No lint ratchet refuses a new `time.After(2 * time.Second)`-shaped wait. It would fail the gate for branches written before this rule existed, so it belongs after the wave lands, as a follow-up row, not inside it.
- Wall clock stays where the property is a wall-clock deadline (`hookguard_test.go`, the RPC span and the harness's kill time), the large hang guards, the negative waits and the inputs: each is a `kept` row above with its reason.

