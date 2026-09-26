# The responsible tenant — the plan (vision 36, ADR-047)

Grammar v2 (`docs/plans/README.md`). A row flips to `[x]` only when
`fathom verify --service fathom --plan docs/plans/responsible-tenant.md` proves it.
Design: `docs/design/responsible-tenant.md` (section numbers below are its). Incident:
INC-23. Invariants I47–I53 rise a rung as the rows that enforce them land. Every audit
hazard id maps to at least one row in the table at the end.

Rows name the contract they will be proven by: the function that will carry the
behaviour and the test that will reach it. Until the code exists they are open, and
`fathom verify` reports them open, never proven. RT-1 and RT-2 name the hotfix's real
symbols and test; they stay open here because this plan was written without a verify
run against an index that holds 4200482a (the machine was under memory pressure); the
first gate run after 4200482a merges may claim them.

Revised 2026-09-26 after design review (ADR-047's amendment). Row ids are stable: rows
moved between waves keep their ids, and new rows start at RT-143. Wave A now carries the
fixes for the known causes in forms that need no schema change; the schema-backed forms
stay in wave C and replace them.

## A. First deploy — the known causes bounded, and an off switch

What the wave-A binary does that the incident binary did not: obligations reads one file
and the path list of the tree (never contents), `list_services` answers from SQL
aggregates, "is X indexed" is a meta probe, the connector never reads a file over 2 MiB
or a default cache directory, at most two whole-tree `Sources` reads run at once and a
hook never waits for one, inline ingests run one at a time and survive their requester,
and the operator can switch the daemon off with nothing else working.

- [ ] `list_services` classifies unsupported languages by file NAME, never contents, with one in-flight walk per service (hotfix 4200482a) {id: RT-1, symbol: unsupportedLanguagesCompute, package: cmd/fathom, reachable_from: TestFleetAdapter_UnsupportedLanguagesFor_NeverReadsFileContents}
- [ ] `client.SourcePaths` discovers names only, reached from the list_services path (hotfix 4200482a) {id: RT-2, symbol: SourcePaths, package: pkg/client, reachable_from: TestFleetAdapter_UnsupportedLanguagesFor_NeverReadsFileContents}
- [ ] Obligations' raw fallback reads only the edited file under the cap (§4.3) {id: RT-81, symbol: RawFallbackObligations, package: pkg/client, reachable_from: TestRawFallbackReadsOnlyEditedFile}
- [ ] Obligations' layer coverage takes corpus paths from `SourcePaths`, memoized per (service, watermark, layer digest) and single-flighted, and never reads file contents; RT-82 replaces the walk with `file_stats` (§4.3) {id: RT-143, symbol: layerCoveragePaths, package: pkg/client, reachable_from: TestLayerCoverageFromSourcePathsSingleFlightNeverReadsContents}
- [ ] The CUE pack union is compiled once per (root, layer digest) (§4.3) {id: RT-83, symbol: loadUnion, package: internal/semantic, reachable_from: TestLoadUnionMemoizedPerDigest}
- [ ] `list_services` computes symbol counts and languages from SQL aggregates (`count(*) GROUP BY service_id`, distinct source paths streamed into an extension set), cached per watermark and single-flighted, never a symbol load; RT-79 replaces it with `service_summary` (§4.4) {id: RT-144, symbol: ServiceSymbolAggregates, package: pkg/client, reachable_from: TestServiceSummaries_AggregatesNeverLoadSymbols}
- [ ] "Is service X indexed" is a meta-only probe used by verify, the stop and edit hooks, plans and the console (§4.4) {id: RT-80, symbol: IsIndexed, package: pkg/client, reachable_from: TestDaemonServiceIndexedUsesMetaProbe}
- [ ] The connector never reads a file over 2 MiB and honours ctx in discovery and fetch (§4.2) {id: RT-69, symbol: fetchCapped, package: internal/connector, reachable_from: TestFetch_NeverReadsOversize}
- [ ] Discovery skips the default cache directories (`.cache`, `.tools`, `target`, `dist`, `build`, `__pycache__`, `.venv`, `.gradle`, `.next`, `.turbo`, `coverage`) on top of `skipDir`, `[connector] include` re-includes, and every exclusion is counted (§4.2) {id: RT-145, symbol: defaultIgnoredDir, package: internal/connector, reachable_from: TestDiscover_DefaultCacheIgnoresKeepSkipDir}
- [ ] `fathom ingest --dry-run --stats` reports discovered files and bytes, stat only (§4.2) {id: RT-70, symbol: runIngestDryRunStats, package: cmd/fathom, reachable_from: TestIngestDryRunStatsReadsNoContent}
- [ ] At most two `client.Sources` reads run at once process-wide; a hook-origin caller never waits for a slot and gets a named skip (§4.6) {id: RT-146, symbol: acquireSourcesSlot, package: pkg/client, reachable_from: TestSourcesSlotHooksNeverWait}
- [ ] Obligations' index-stale check stats before reading and skips files over 2 MiB, reported as not refreshed (§4.3) {id: RT-26, symbol: ObligationsIndexStale, package: pkg/client, reachable_from: TestObligationsIndexStaleSkipsOversize}
- [ ] `/refresh-file` stats before reading (2 MiB cap) and finds the file's packages by `source_ref`, not a full symbol load (§7.2) {id: RT-27, symbol: ReingestFile, package: internal/fusion, reachable_from: TestRefreshFileStatCapAndPackageLookup}
- [x] `ensure()` runs its inline ingest under `context.WithoutCancel`, holding one process-wide ingest ticket, so a requester's disconnect never cancels an ingest midway, two services never ingest inline at once, and only the read that starts an ingest waits for it (§3.5, §7.1) {id: RT-149, symbol: ensure, package: internal/mcp, reachable_from: TestEnsureIngestSurvivesRequestCancelAndSerializes}
- [x] `ensure()` records `seen[service]` only after a successful ingest, so a failed ingest is retried — after a per-(service, head) backoff of 30 s doubling to 30 min, never by every read (§7.1) {id: RT-151, symbol: ensure, package: internal/mcp, reachable_from: TestEnsureSeenOnlyAfterSuccess}
- [ ] The kill switch: `disabled.json` is read before config, store or bind and a disabled daemon exits 0 in under 50 ms (§9) {id: RT-3, symbol: checkKillSwitch, package: cmd/fathom, reachable_from: TestDisabledFileCheckedBeforeStoreOpen}
- [ ] `disabled.json` and `paused.json` govern only the managed daemon (and hooks addressing its URL); an unmanaged daemon ignores them (§9) {id: RT-167, symbol: checkKillSwitch, package: cmd/fathom, reachable_from: TestKillSwitchIgnoredByUnmanagedDaemon}
- [ ] `fathom daemon stop` writes the switch, kills with a pid-reuse guard, and proves the port closed before it reports success (§9) {id: RT-4, symbol: runDaemonStop, package: cmd/fathom, reachable_from: TestStopProofRequiresPortClosed}
- [ ] Under an installed schema-1 plist, `stop` and the breaker say launchd will relaunch every 10 s and each start exits in < 50 ms, print the `launchctl bootout` and `--reconcile` lines, and the kill-switch check writes no lifecycle row (§9) {id: RT-157, symbol: runDaemonStop, package: cmd/fathom, reachable_from: TestStopUnderV1PlistNamesRelaunchAndBootout}
- [ ] `fathom daemon stop` kills a daemon that ignores SIGTERM (§9) {id: RT-5, symbol: runDaemonStop, package: cmd/fathom, reachable_from: TestStopKillsDaemonIgnoringSIGTERM}
- [ ] `--for <dur>` exits 75 until expiry, an empty `disabled.json` disables, `daemon start` resets the breaker window, `pause`/`resume` move to and from safe mode (§9) {id: RT-6, symbol: EvaluateKillSwitch, package: internal/tenancy, reachable_from: TestTimedPauseExits75UntilExpiry}
- [ ] Every hook invocation carries one monotonic deadline (`TimeoutSec − max(1 s, 20%)`) that every RPC derives from and sends as `X-Fathom-Deadline` (§11.1) {id: RT-7, symbol: HookDeadline, package: internal/hookguard, reachable_from: TestNoHookRPCOutlivesItsDescriptorBudget}
- [ ] `fetchServices` retries only on refused/reset and only within the hook deadline, never after a timeout (§11.1) {id: RT-8, symbol: fetchServices, package: cmd/fathom, reachable_from: TestFetchServicesNeverRetriesAfterTimeout}
- [ ] Hooks share a per-URL breaker file that opens on 2 failures in 10 s (or at once on a 503 with `Retry-After`), backs off exponentially, and admits exactly one half-open prober under an O_EXCL lease (§11.2) {id: RT-9, symbol: BreakerAllows, package: internal/hookguard, reachable_from: TestHookBreakerSingleProber}
- [ ] Hooks `stat` `disabled.json`/`paused.json` before any RPC and return silently (§9) {id: RT-10, symbol: BreakerAllows, package: internal/hookguard, reachable_from: TestHooksSilentWhenDisabled}
- [x] Self-restart adoption exits 75, a code launchd relaunches on under both the schema-1 `KeepAlive true` plist and the schema-2 `{SuccessfulExit: false}` dict, where exit 0 would stay down (§8.2) {id: RT-11, symbol: adoptionExitCode, package: cmd/fathom, reachable_from: TestSelfRestartExitCodeMatchesPlistSchema}
- [x] The binary watcher hashes only when (size, mtime, inode) changed, keeps a cross-process 5-minute cooldown in the lifecycle ledger, and runs only when managed (§8.2) {id: RT-12, symbol: binaryStatChanged, package: cmd/fathom, reachable_from: TestWatchOwnBinaryHashesOnlyOnStatChange}
- [ ] A shared test entry point isolates HOME, state, call log and hook caches, points FATHOM_HTTP at 127.0.0.1:1, sets `FATHOM_TEST=1`, and exits 2 when it cannot (§10.4) {id: RT-16, symbol: IsolateAndRun, package: internal/testisolate, reachable_from: TestIsolateAndRun_ExitsWhenTempUnavailable}
- [ ] `defaultFathomURL` returns an unroutable URL under `testing.Testing()` or `FATHOM_TEST=1` when FATHOM_HTTP is unset (§10.4) {id: RT-17, symbol: defaultFathomURL, package: cmd/fathom, reachable_from: TestDefaultURLUnroutableUnderTest}
- [ ] `custodian.DefaultDaemonURL` has the same backstop (§10.4) {id: RT-18, symbol: DefaultDaemonURL, package: internal/custodian, reachable_from: TestDefaultDaemonURLUnroutableUnderTest}
- [ ] Every package that reaches a daemon, state or call-log default runs under the isolating entry point (§10.4) {id: RT-19, symbol: packagesNeedingIsolation, package: internal/testisolate, reachable_from: TestEveryPackageWithDaemonDefaultsIsIsolated}
- [ ] Every client derives `test=1` in `X-Fathom-Caller` from `FATHOM_TEST=1` as well as `testing.Testing()`, so a test that execs the built binary is still marked (§10.4) {id: RT-164, symbol: CallerHeader, package: pkg/client, reachable_from: TestCallerHeaderTestFlagFromEnv}
- [ ] An unmanaged serve-http exits within 10 s of its parent's death, SIGKILL included (§10.1) {id: RT-20, symbol: watchParentExit, package: cmd/fathom, reachable_from: TestUnmanagedServeHTTPExitsWhenParentKilled}
- [ ] An unmanaged serve-http exits on idle (20 min), TTL (8 h), or an unlinked executable or store, naming the reason; auto-refresh, feeders and the watcher are off (§10.1) {id: RT-21, symbol: EvaluateUnmanagedExit, package: internal/tenancy, reachable_from: TestUnmanagedExitReasons}
- [ ] An unmanaged serve-http refuses the managed daemon's store and call log without `--allow-operator-state` (§10.1) {id: RT-22, symbol: refuseOperatorState, package: cmd/fathom, reachable_from: TestUnmanagedRefusesOperatorStore}
- [ ] The stranger check uses a private `mktemp -d` work dir removed by its trap (§10.3) {id: RT-25, oracle: lexical, path: scripts/stranger-check.sh, pattern: "mktemp -d"}
- [ ] `task deploy` fails loudly when the `fathom-dev` signing identity is missing instead of installing an ad-hoc binary (§8.4) {id: RT-147, oracle: lexical, path: Taskfile.yml, pattern: "fathom-dev signing identity missing"}
- [ ] `fathom daemon status` reports the running binary's codesigning identity and names an ad-hoc signature (§8.4) {id: RT-148, symbol: binarySigningIdentity, package: cmd/fathom, reachable_from: TestDaemonStatusReportsSigningIdentity}
- [x] The post-commit hook's message says what it did (daemon refresh failed, nothing ingested, how to refresh) instead of announcing a fallback it does not run (§12.3) {id: RT-170, symbol: cmdPostCommitHook, package: cmd/fathom, reachable_from: TestPostCommitMessageNamesNoFallback}

## A2. Posture and census — ship with A, or next if time is short

RT-28 needs RT-14 (reconcile) and RT-24 (reap); if these slip, wave A deploys without
them and RT-28 waits.

- [ ] Plist schema 2: KeepAlive {SuccessfulExit: false}, ThrottleInterval 60, ProcessType Background, LowPriorityIO, LowPriorityBackgroundIO, Nice 5, ExitTimeOut 20, `--managed` (§8.1) {id: RT-13, symbol: plistBody, package: cmd/fathom, reachable_from: TestPlistV2RoundTripAndReconcileKeepsOperatorKeys}
- [ ] `fathom daemon install --reconcile` rewrites only the tenancy keys, keeps label, paths, store, call log, WorkingDirectory and env, writes a backup and receipt, needs `--adopt` for a hand-written plist, and refuses before the binary reports `tenancy_contract >= 1` (§8.3) {id: RT-14, symbol: reconcilePlist, package: cmd/fathom, reachable_from: TestPlistV2RoundTripAndReconcileKeepsOperatorKeys}
- [ ] Doctor and `daemon status` name plist drift with the exact reconcile command; the reload line is printed, not run, from an agent shell (§8.3) {id: RT-15, symbol: checkDaemonPosture, package: cmd/fathom, reachable_from: TestDoctorDaemonPostureNamesReconcile}
- [ ] Daemon records move to `~/.fathom/daemon/<id>/record.json`; the legacy `~/.fathom/daemons/` is read for one release and pruned (§10.2) {id: RT-166, symbol: daemonRecordPath, package: cmd/fathom, reachable_from: TestDaemonRecordUnderDaemonDirWithLegacyRead}
- [ ] `fathom daemon ps` joins records with a live census and names orphaned, idle, unlinked, no-record and stale-record daemons (§10.2) {id: RT-23, symbol: daemonCensus, package: cmd/fathom, reachable_from: TestDaemonCensusVerdicts}
- [ ] `fathom daemon reap` is dry-run by default and never touches the managed label (§10.2) {id: RT-24, symbol: runDaemonReap, package: cmd/fathom, reachable_from: TestDaemonReapDryRunKillsNothing}
- [ ] (op) RT-28: operator deploys the wave-A binary with `task deploy` (which now fails without the signing identity), reconciles this machine's plist (`--adopt`), reloads from their own terminal, and runs `fathom daemon reap --apply` on the leaked scratch daemons {id: RT-28, skip: "operator action on the live machine", needs: "RT-14,RT-24,RT-147"}

## B. Seatbelt — bounded whatever the cause

In waves A–B the admitted-work envelope does not hold yet (classes carry today's costs and
`Charge` has few callers until wave C); the ladder and the producer governor are what
bound memory here.

- [ ] The budget resolves from RAM and config (clamp(RAM/8, 1–4 GiB)), deriving marks, slots, per-call ceilings and deadlines, each derivation named (§2, §3.1) {id: RT-29, symbol: ResolveBudget, package: internal/tenancy, reachable_from: TestResolveBudget_PerRAMClass}
- [ ] `[tenancy]` is read at user scope only; a repository `[tenancy]` is ignored and named; `[daemon].memory_limit` migrates (§1) {id: RT-30, symbol: ResolveTenancyConfig, package: internal/config, reachable_from: TestBudgetConfigUserScopeOnly}
- [ ] The CUE schema carries `#Tenancy` with valid and invalid testdata pairs (§1) {id: RT-31, oracle: lexical, path: schema/config/schema.cue, pattern: "#Tenancy"}
- [ ] The host sampler reads phys_footprint, the child-process tree, runtime/metrics, pressure level, swap used and volume free space without stop-the-world calls or allocation; an unknown pressure or swap reading is disclosed and never a trigger; an unknown footprint disables the ladder, disclosed (§2) {id: RT-32, symbol: SampleHost, package: internal/tenancy, reachable_from: TestHostSampler_UnknownDisclosedNeverTriggers}
- [ ] The child-process tree's footprint (`go list`, git, rg) is sampled, disclosed in `resources` and `/metrics`, and counted toward the shed and refuse marks; the exit rung kills the tree first (§2) {id: RT-161, symbol: childTreeFootprint, package: cmd/fathom, reachable_from: TestChildFootprintCountedTowardShedAndRefuse}
- [ ] The ladder moves normal → shed → refuse → exit with hysteresis (§2.1) {id: RT-33, symbol: StepLadder, package: internal/tenancy, reachable_from: TestLadder_TransitionsAndHysteresis}
- [ ] The swap-used fraction alone never moves the ladder; host triggers are pressure level, swap growth > 2 GiB in 5 min while fathom is above the shed mark, and swap-volume free space (§2.1) {id: RT-154, symbol: StepLadder, package: internal/tenancy, reachable_from: TestLadder_SwapFractionAloneNeverSheds}
- [ ] A sample more than 3× late counts as a second over-mark sample, and the sampler does not allocate (§2) {id: RT-171, symbol: StepLadder, package: internal/tenancy, reachable_from: TestLadder_LateSampleEscalatesAndSamplerZeroAlloc}
- [ ] Byte producers (connector fetch, `Sources`, corpus reads, row decoding every 1k rows) read an atomic ladder level before each unit and return `ErrOverBudget`: tree, fleet and ingest producers at refuse, every producer at the exit mark (§2.1) {id: RT-153, symbol: ProducerAllows, package: internal/tenancy, reachable_from: TestProducerHaltsWithinOneFile}
- [ ] Shed pauses background work, serves stale with disclosure, drops the console and fleet memos but keeps the graph and symbol caches, and scavenges on entry; refuse drops every cache (§2.1) {id: RT-34, symbol: applyLadderLevel, package: cmd/fathom, reachable_from: TestShedDropsCachesAndPausesBackground}
- [ ] Refuse cancels every in-flight tree and fleet ticket, youngest first, as `abandoned-by-governor` (§2.1) {id: RT-172, symbol: applyLadderLevel, package: cmd/fathom, reachable_from: TestRefuseCancelsAllHeavyTicketsYoungestFirst}
- [ ] At the exit mark an in-flight ingest or typed pass is halted first and its service quarantined; the daemon exits only if the next sample is still over (§2.1) {id: RT-173, symbol: exitOverBudget, package: cmd/fathom, reachable_from: TestExitMarkHaltsIngestBeforeExiting}
- [ ] The exit rung captures diagnostics, writes the reason with the top in-flight work and exits 70 at once, with no drain (§2.1) {id: RT-35, symbol: exitOverBudget, package: cmd/fathom, reachable_from: TestLadderExitsOverBudget}
- [ ] A service whose ingest was halted or in flight at an over-budget exit is quarantined (`quarantined: exceeded budget during ingest` in `/statusz`, `resources` and the primer) and never re-queued until an explicit `ingest` (§2.3) {id: RT-155, symbol: QuarantinedServices, package: internal/tenancy, reachable_from: TestIngestInFlightAtOverBudgetExitQuarantines}
- [ ] The GC hint defaults to the exit mark and GOMAXPROCS defaults to max(2, NumCPU/2) for managed daemons (§2.2) {id: RT-36, symbol: applyMemoryLimit, package: cmd/fathom, reachable_from: TestMemoryLimitDerivedFromBudget}
- [ ] The watermark advances only at the final commit; a killed or halted ingest leaves in_progress and the next start re-ingests unless quarantined (§6) {id: RT-109, symbol: recordIngestMeta, package: internal/fusion, reachable_from: TestWatermarkNotAdvancedByKilledIngest}
- [ ] `ensure()` is skipped at shed and above, in safe mode, during warm-up and for quarantined services, answering stale with the reason (§7.1) {id: RT-150, symbol: ensure, package: internal/mcp, reachable_from: TestEnsureSkippedAtShedSafeModeWarmupQuarantine}
- [ ] The lifecycle ledger records every start and exit and names an unclean predecessor (with jetsam or resource-report evidence when found) at the next start (§2.3) {id: RT-37, symbol: SynthesizeUncleanExit, package: internal/tenancy, reachable_from: TestLifecycle_UncleanPredecessorNamed}
- [ ] The breaker starts safe mode at 3 counted exits in 30 min and stops (disabled.json, exit 0) at 5 in 60 min or a safe-mode over-budget exit; adoption, stop, store-wait, SIGTERM and a deploy-intent kickstart are not counted (§2.3) {id: RT-38, symbol: EvaluateBreaker, package: internal/tenancy, reachable_from: TestBreakerSafeModeThenStop}
- [ ] `task deploy` writes a `deploy` intent row (pid, binary hash) before the kickstart, and the next start classifies the predecessor's death as not counted (§8.5) {id: RT-156, symbol: recordDeployIntent, package: cmd/fathom, reachable_from: TestDeployKickstartNotCountedByBreaker}
- [ ] `serve-http --lifecycle-id <id>` gives an unmanaged daemon a named, persistent ledger so the stress harness exercises the breaker across relaunches (§2.3, §15.2) {id: RT-158, symbol: lifecycleID, package: cmd/fathom, reachable_from: TestLifecycleIDFlagSharesLedgerAcrossUnmanagedStarts}
- [ ] Safe mode refuses tree and fleet with a named reason, serves point and index, and turns off auto-refresh, feeders, jobs and the decision port (§2.3) {id: RT-39, symbol: applySafeMode, package: cmd/fathom, reachable_from: TestSafeModeRefusesHeavyServesIndex}
- [ ] Warm-up after any start gives heavy classes one slot and allows no inline ingest (§2.3) {id: RT-40, symbol: WarmupSlots, package: internal/tenancy, reachable_from: TestWarmupLimitsHeavySlots}
- [ ] Store distress (SQLITE_FULL, IOERR, store volume under 2 GiB) refuses writes and ingest and turns a failed read into a named error, never a zero count (§2.1) {id: RT-41, symbol: classifyStoreError, package: cmd/fathom, reachable_from: TestDiskFullIsDistressNeverZero}
- [ ] The daemon binds before opening the store and waits (not exits) for an unavailable store, labelling a TCC wedge apart from an unmounted volume (§8.4) {id: RT-42, symbol: openStoreWithWait, package: cmd/fathom, reachable_from: TestServeHTTPWaitsForStoreWithoutExiting}
- [ ] The `tcc-wedge` remedy names the daemon binary's path and signing identity and its Full Disk Access / Removable Volumes grant, never a terminal's bundle id (§8.4) {id: RT-165, symbol: storeWaitRemedy, package: cmd/fathom, reachable_from: TestTCCWedgeRemedyNamesDaemonBinary}
- [ ] A port held by a managed fathom of the same label exits 0 ("already served"); any other holder exits 75 and is named (§8.4) {id: RT-43, symbol: resolvePortHolder, package: cmd/fathom, reachable_from: TestPortHeldBySameLabelExitsZero}
- [ ] SIGTERM writes the `exit{reason: drain}` row first, closes heavy admission, lets point, index and an ingest's batch boundary finish within 12 s in total, and exits 0; runtime fatals go to crash.log (§8.5) {id: RT-44, symbol: drainAndExit, package: cmd/fathom, reachable_from: TestSIGTERMDrainsWithinExitTimeout}
- [ ] The scavenger logs total−released and phys_footprint before and after, on events as well as the tick (§13.3) {id: RT-45, symbol: runMemoryScavenger, package: cmd/fathom, reachable_from: TestScavengerLogsReleasedAndFootprint}
- [ ] The typed-pass line reads "process heap during pass" from runtime/metrics with the concurrent in-flight tickets beside it (§2.2) {id: RT-46, symbol: sampleProcessHeapDuringPass, package: internal/fusion, reachable_from: TestTypedPassLogLabelsProcessHeap}
- [ ] Disk bytes written (process rusage) are sampled, kept in the ring and judged against 2 GiB/day outside explicit ingest in `resources` and doctor (§12.4) {id: RT-162, symbol: DiskWriteVerdict, package: internal/tenancy, reachable_from: TestDiskWriteBudgetDisclosed}
- [ ] The resource ring persists one sample a minute for 24 h plus every transition, fixed-size (§13.1) {id: RT-47, symbol: WriteRingSample, package: internal/tenancy, reachable_from: TestRing_FixedSizeAndWrap}
- [ ] `fathom resources` renders budget, footprint (self and children) history, lifecycle, quarantine, host, in-flight, hooks, scratch daemons, disk and the last capture from files, and exits 0/1/2 (§13.2) {id: RT-48, symbol: runResources, package: cmd/fathom, reachable_from: TestResourcesRendersFromRingWhenDaemonDown}
- [ ] Doctor judges phys_footprint from the ring against the budget and never SKIPs an unreachable daemon into silence; the restart remedy is gone (§13.3) {id: RT-49, symbol: checkDaemonFootprint, package: cmd/fathom, reachable_from: TestDoctorDaemonBudgetNeverSkipsWhenDown}
- [ ] Doctor gains daemon_lifecycle, host_pressure, scratch_daemons and tenancy_scope rows (§13.3) {id: RT-50, symbol: checkDaemonLifecycle, package: cmd/fathom, reachable_from: TestDoctorTenancyRows}
- [ ] The session primer carries one attention line (≤160 B, inside I12) only when the daemon is not normal (§13.3) {id: RT-51, symbol: tenancyAttentionLine, package: cmd/fathom, reachable_from: TestPrimerTenancyLineOnlyWhenNotNormal}
- [ ] `/statusz` carries a tenancy block with the same projection (§13.3) {id: RT-52, symbol: tenancyStatus, package: cmd/fathom, reachable_from: TestStatuszTenancyBlockMatchesResources}
- [ ] The admission gate: per-class slots and per-origin waits, ctx-aware waiters, deadline feasibility refusal, a per-call byte meter that returns partial results (§3.2) {id: RT-53, symbol: Admit, package: internal/tenancy, reachable_from: TestGate_HookNeverWaitsForHeavy}
- [ ] `admission = "observe"` decides, logs and counts every would-refuse and admits; it is the default until RT-96 (§3.2) {id: RT-54, symbol: Admit, package: internal/tenancy, reachable_from: TestGate_ObserveNeverRefuses}
- [ ] Every registered tool declares a cost class; registration without one fails; `docs/mcp-tools.md` carries the class (§3.1) {id: RT-55, symbol: costClassOf, package: internal/mcp, reachable_from: TestEveryToolDeclaresCostClass}
- [ ] A tool's declared class is at least the class of its measured cost in the committed cost table; a class moves down only in the commit that rewrites the tool and carries its cost assertion (§3.1) {id: RT-152, symbol: costClassOf, package: internal/mcp, reachable_from: TestCostClassMatchesMeasuredCost}
- [ ] Dispatch admits before the handler runs and refuses with HTTP 503, Retry-After and JSON-RPC -32050 naming the reason (§3.3, §3.4) {id: RT-56, symbol: admitToolCall, package: internal/mcp, reachable_from: TestAdmissionReturnsNamed503WithRetryAfter}
- [ ] A partial or refused answer names itself; no response is an empty result that looks complete (§3.4) {id: RT-57, symbol: admitToolCall, package: internal/mcp, reachable_from: TestRefusalIsNamedNeverEmpty}
- [ ] Read tools run under the request's context and the min of the class deadline and X-Fathom-Deadline; a client disconnect cancels them and counts abandoned (§3.5) {id: RT-58, symbol: rpcContext, package: cmd/fathom, reachable_from: TestClientDisconnectCancelsReadWork, needs: "RT-149,RT-150,RT-109"}
- [ ] serve-http runs an http.Server with header, idle and per-route write timeouts and requires a loopback Host (and same-origin Origin) on every route (§3.5) {id: RT-59, symbol: newDaemonServer, package: cmd/fathom, reachable_from: TestDaemonServerTimeoutsAndLoopbackHost}
- [ ] `batch` caps calls at 16, admits each sub-call in its own class, and stops when its context ends (§3.3) {id: RT-60, symbol: admitBatchCall, package: internal/mcp, reachable_from: TestBatchCapsAndAdmitsEachCall}
- [ ] In-process console calls go through admission and the meters as origin console-inprocess and never trigger auto-refresh (§3.3) {id: RT-61, symbol: inProcessToolCall, package: cmd/fathom, reachable_from: TestInProcessCallsMeteredAsConsole}
- [ ] The hotfix's in-flight waiter and the console memo waiters honour ctx (§3.5) {id: RT-62, symbol: unsupportedLanguagesCached, package: cmd/fathom, reachable_from: TestUnsupportedWaiterHonoursContext}
- [ ] The managed daemon answers test callers 409 and counts them; every client sends X-Fathom-Caller (§10.4) {id: RT-63, symbol: refuseTestCaller, package: cmd/fathom, reachable_from: TestTestCallerRefusedByManagedDaemon}
- [ ] Call-log rows carry class, queued_ms, bytes_read, outcome and caller identity, with a schema_version bump (§13.3) {id: RT-64, symbol: recordCallOutcome, package: internal/mcp, reachable_from: TestCallLogRowCarriesClassOutcomeCaller}
- [ ] Decision-port calls are admitted as the egress class and skipped with disclosure at refuse and in safe mode (vision 36 §8) {id: RT-65, symbol: admitDecision, package: internal/mcp, reachable_from: TestDecisionPortClosedUnderRefuse}

## C. Brakes — no request costs more than its answer

- [ ] `corpus.ReadFileCapped` stats before reading, sniffs binaries on 8 KiB, charges the meter and returns named exclusions (§4.1) {id: RT-66, symbol: ReadFileCapped, package: internal/corpus, reachable_from: TestReadFileCapped_StatsBeforeRead}
- [ ] `corpus.ScanBudgeted` streams one file at a time, stops at bytes, files, time, hits or ctx, and reports a lower-bound coverage (§4.1) {id: RT-67, symbol: ScanBudgeted, package: internal/corpus, reachable_from: TestScanBudgeted_StopsAndReportsLowerBound}
- [ ] Discovery in a git worktree honours .gitignore via `git ls-files -co --exclude-standard`, `skipDir` and the default cache ignores still apply after it (a committed `vendor/` or `testdata/` stays excluded), `include` re-includes, and every exclusion is counted; the default flips only after RT-M11 reads (§4.2) {id: RT-68, symbol: discoverViaGit, package: internal/connector, reachable_from: TestDiscover_HonoursGitignoreAndKeepsSkipDir, needs: RT-M11}
- [ ] `Sources` is gone from `mcp.QueryInterface`: query code cannot name a whole-tree read (§4.6) {id: RT-71, oracle: lexical, path: internal/mcp/server.go, pattern: "^\s+Sources\(ctx", eq: 0}
- [ ] `memo` groups single-flight with ctx-aware waiters and cancel a flight when its last waiter leaves unless it is marked Keep (§5) {id: RT-72, symbol: NewGroup, package: internal/memo, reachable_from: TestGroup_StampedeOneBuildCtxAwareWaiters}
- [ ] All memo groups share one byte budget, evict a service's entries on a new watermark, never key an empty watermark, trim on shed and drop on refuse and idle (§5) {id: RT-73, symbol: Drop, package: internal/memo, reachable_from: TestGroup_ByteBudgetAndWatermarkEviction}
- [ ] Shed trims the memo registry by LRU to 50% of the cache budget and allows one rebuild flight process-wide while shed; the shed → rebuild cycle holds under hysteresis (§5) {id: RT-163, symbol: Trim, package: internal/memo, reachable_from: TestShedTrimToHalfSingleRebuildHysteresis}
- [ ] One graph store keyed (service, watermark, kind) serves MCP tools and the console; `callgraph.Build` has no other caller (§5) {id: RT-74, symbol: GraphFor, package: internal/memo, reachable_from: TestGraphStore_OneGraphPerServiceWatermark}
- [ ] One immutable symbol snapshot per (service, watermark) with name and callee indexes serves iterating tools, nav reloads and the console palette (§5) {id: RT-75, symbol: SnapshotFor, package: internal/memo, reachable_from: TestSnapshot_NameIndexSharedByReaders}
- [ ] Ingest writes file_stats and service_summary in its final transaction (§6) {id: RT-76, symbol: WriteServiceSummary, package: internal/store, reachable_from: TestServiceSummary_WrittenAtIngestCommit}
- [ ] Ingest normalizes calls into observation_calls with a callee index (§6) {id: RT-77, symbol: CallersOfKeys, package: internal/store, reachable_from: TestObservationCalls_IndexedReverseLookup}
- [ ] `service_summary` and `observation_calls` are backfilled by SQL (`GROUP BY`, `json_each(calls)`) at first use, one service per transaction under the ingest slot, with no content reads; only `file_stats` waits for an ingest (§6) {id: RT-160, symbol: BackfillDerivedTables, package: internal/store, reachable_from: TestSummaryBackfilledWithoutIngest}
- [ ] The store opens with cache_size, soft_heap_limit, hard_heap_limit and journal_size_limit, indexes (service_id, symbol), and sorts observations without a temp B-tree (§6) {id: RT-78, symbol: applyTenancyPragmas, package: internal/store, reachable_from: TestStorePragmasAndNoTempSort}
- [ ] `list_services` reads summary rows only, and a service not yet backfilled is a named absence, never a whole-tree fallback (§4.4, §6) {id: RT-79, symbol: ServiceSummaries, package: internal/mcp, reachable_from: TestServiceSummaries_ReadsSummaryRowsOnly}
- [ ] Obligations' layer coverage comes from file_stats paths, memoized per (service, watermark, layer digest), and never reads file contents (§4.3) {id: RT-82, symbol: ObligationsLayerCoverage, package: pkg/client, reachable_from: TestLayerCoverageFromFileStatsNeverReadsContents}
- [ ] Cross-service callers are one indexed query, and callees resolve once per key through the snapshot (§4.5) {id: RT-84, symbol: fleetCallers, package: internal/mcp, reachable_from: TestFleetCallers_OneIndexedQuery}
- [ ] find_symbol and find_callers are SQL point lookups, not full-table loads (§4.5) {id: RT-85, symbol: FindSymbol, package: internal/query, reachable_from: TestFindSymbol_PointLookupNoFullLoad}
- [ ] `references` defaults to the caller's own service when an identity resolves; `services: ["*"]` is fleet class; the scan is budgeted with a named partial (§4.5; Breaking) {id: RT-86, symbol: referencesScope, package: internal/mcp, reachable_from: TestReferences_DefaultsToOwnServiceFleetOptIn}
- [ ] `references` with no `services` argument and no resolvable caller identity returns a named -32602 listing the indexed services and the `["*"]` opt-in, never a fleet scan and never an empty result (§4.5; Breaking) {id: RT-159, symbol: referencesScope, package: internal/mcp, reachable_from: TestReferences_NoIdentityNamesChoices}
- [ ] Reference text search streams lines, skips binaries and stops at max (§4.5) {id: RT-87, symbol: Find, package: internal/analysis/references, reachable_from: TestFind_SkipsBinariesStreamsLines}
- [ ] `search` caps rg output bytes (killing rg), columns, file size, count and context, and restricts root to registered service roots (§4.5) {id: RT-88, symbol: searchRipgrep, package: internal/mcp, reachable_from: TestSearchRipgrep_ByteCapKillsAndRootRestricted}
- [ ] Routes and HTTP client calls are extracted at ingest and read per watermark, never per call (§6) {id: RT-89, symbol: BuildFromStore, package: internal/analysis/serviceref, reachable_from: TestServiceRefs_PersistedNotPerCall}
- [ ] The dispatch backstop checks size before copying and producers budget themselves; no marshal/unmarshal cascade (§4.5) {id: RT-90, symbol: budgetedResult, package: internal/mcp, reachable_from: TestBudgetedResultNoRemarshalCascade}
- [ ] The tenancy lint's syntactic rules forbid whole-file reads and walks in handler packages, whole-table and whole-tree loads, uncontexted exec, Background contexts in handler files, bare goroutines, package-level maps in `cmd/fathom`/`internal/mcp` without an allowlist entry, raw http.Serve, live daemon URLs in tests, and a test `cmd.Env` built without `os.Environ()` (§4.7) {id: RT-91, symbol: CheckTenancyRules, package: internal/lint/tenancy, reachable_from: TestTenancyRules}
- [ ] The tenancy allowlist may only shrink (§4.7) {id: RT-92, symbol: allowlistCount, package: internal/lint/tenancy, reachable_from: TestTenancyAllowlistOnlyShrinks}
- [ ] Every tool has a cost row: bytes read and allocations within its class ceiling, cold and warm, at 1× and 10× fleet scale (§4.8) {id: RT-93, symbol: MeasureToolCost, package: internal/mcp/costtest, reachable_from: TestEveryToolHasCostRow}
- [ ] Point and index tools grow at most 1.5× when the fleet grows 10× (§4.8, I48) {id: RT-94, symbol: MeasureToolCost, package: internal/mcp/costtest, reachable_from: TestPointIndexToolsScaleSublinearly}
- [ ] 32 cold concurrent callers cause one build and one walk's bytes per key; cancelling them all returns goroutines to baseline within 1 s (§4.8) {id: RT-95, symbol: MeasureStampede, package: internal/mcp/costtest, reachable_from: TestColdStampedeOneBuildPerKey}
- [ ] Admission defaults to enforce once RT-M0 reads {id: RT-96, symbol: DefaultAdmissionMode, package: internal/tenancy, reachable_from: TestDefaultAdmissionEnforce, needs: RT-M0}

## D. Ingest off the request path, streamed

- [ ] The refresh queue runs one ingest per service (a request mid-run sets dirty for exactly one follow-up) and one heavy slot machine-wide, shared by auto-refresh, the ingest tool, post-commit and /refresh-file (§7.1) {id: RT-97, symbol: RequestRefresh, package: internal/refresh, reachable_from: TestRefresh_OneInFlightOneFollowUp}
- [ ] An ingest survives its requester's disconnect and duplicate requests coalesce (§7.1) {id: RT-98, symbol: RequestRefresh, package: internal/refresh, reachable_from: TestIngestSurvivesDisconnectAndCoalesces}
- [ ] Auto-refresh enqueues and answers from the current index with index_stale, never ingesting inline; console origins never trigger it (§7.1) {id: RT-99, symbol: ensure, package: internal/mcp, reachable_from: TestEnsureNeverIngestsInline}
- [ ] The queue orders by demand, waits and staggers after a start (one service per 30 s during warm-up), pauses at shed and refuses at refuse, safe mode, quarantine and store distress (§7.1) {id: RT-100, symbol: nextRefresh, package: internal/refresh, reachable_from: TestRefresh_DemandOrderAndStartupStagger}
- [ ] HEAD is read from .git/HEAD and ref files without a fork, cached by mtime, with a 2 s contexted fallback that reads as freshness unknown (§7.1) {id: RT-101, symbol: Head, package: internal/gitro, reachable_from: TestHead_ReadsRefsWithoutFork}
- [ ] Pass 1 reads only files whose (size, mtime, inode) changed (§7.2) {id: RT-102, symbol: statChangedFiles, package: internal/fusion, reachable_from: TestIngest_ReadsOnlyStatChangedFiles}
- [ ] Pass 2 streams through a 64 MiB in-flight byte window and writes in batches; peak content bytes never exceed it (§7.2) {id: RT-103, symbol: fetchWindowed, package: internal/fusion, reachable_from: TestIngestPeakContentWithinInflightBytes}
- [ ] The typed pass runs only when a Go input changed and loads only changed packages plus reverse dependencies (§7.2) {id: RT-104, symbol: typedPassNeeded, package: internal/fusion, reachable_from: TestDocOnlyChangeRunsNoTypedPass}
- [ ] The CUE pass runs only when a .cue input changed, under the heavy slot (§7.2) {id: RT-105, symbol: cuePassNeeded, package: internal/fusion, reachable_from: TestCUEPassGatedAndSerialized}
- [ ] Toolchain children run with GOFLAGS=-p=2, GOMAXPROCS=2 and background priority (§7.2) {id: RT-106, symbol: backgroundToolchainEnv, package: internal/fusion, reachable_from: TestTypedPassChildrenNicedAndCapped}
- [ ] The file-resolve pass reuses the pipeline's extractors; TSExtractor closes its modules and caps module memory; module count is stable across ingests (§7.2) {id: RT-107, symbol: enrichFileResolvedEdges, package: internal/fusion, reachable_from: TestWazeroModulesStableAcrossIngests}
- [ ] File-resolve reuses file_imports for unchanged hashes, applies size, binary and known-fault gates, and discloses faults (§7.2) {id: RT-108, symbol: ResolveFileEdges, package: internal/fusion, reachable_from: TestFileResolve_ReusesImportsAndDisclosesFaults}
- [ ] A service whose watermark is a git SHA never takes the content-manifest path; non-git corpora use a capped stat manifest (§6, ADR-017 amended) {id: RT-110, symbol: ServiceWatermark, package: pkg/client, reachable_from: TestGitServiceNeverTakesManifestPath}
- [ ] The post-commit hook enqueues a refresh (202, coalesced) instead of running an ingest (§12.3) {id: RT-111, symbol: ingestViaDaemon, package: cmd/fathom, reachable_from: TestPostCommitEnqueuesRefresh}

## E. Console and hygiene

- [ ] The call-log tail keeps windowed aggregates and a 35-day ring, never a row per line forever (§14) {id: RT-112, symbol: callLogWindow, package: cmd/fathom, reachable_from: TestCallLogTailKeepsWindowOnly}
- [ ] The observatory model serves the last model while one flight rebuilds outside the lock, from tails, never whole-log re-parses (§14) {id: RT-113, symbol: cachedObservatoryModel, package: cmd/fathom, reachable_from: TestObservatoryModelServesStaleWhileRebuilding}
- [ ] Session pages seek a per-session offset index; nothing reads hooks.jsonl whole (§14) {id: RT-114, symbol: sessionBeatIndex, package: cmd/fathom, reachable_from: TestSessionPageSeeksIndexedBeats}
- [ ] Console pages make no loopback self-calls and fork no git per service (§14) {id: RT-115, symbol: fleetFreshness, package: cmd/fathom, reachable_from: TestFleetFreshnessNoLoopbackOrFork}
- [ ] Console plan verification runs in process under a deadline and never re-queues a key still running (§14) {id: RT-116, symbol: verifyPlanInProcess, package: cmd/fathom, reachable_from: TestConsolePlansVerifyInProcessNoRequeue}
- [ ] Console background work runs in one bounded job pool with single flight per key, dropping superseded keys (§14) {id: RT-117, symbol: SubmitJob, package: internal/console, reachable_from: TestJobs_BoundedSingleFlightDropsSuperseded}
- [ ] /refactors computes conserve through the job pool with contexted git and a diff-size cap (§14) {id: RT-118, symbol: refactorReport, package: cmd/fathom, reachable_from: TestRefactorReportBoundedPool}
- [ ] Event feeders run only while /events has a subscriber and poll compile runs from a cursor (§14) {id: RT-119, symbol: runConsoleEventSources, package: cmd/fathom, reachable_from: TestFeedersIdleWithoutSubscribers}
- [ ] The ⌘K palette reads the shared snapshot, not a private fleet copy (§14) {id: RT-120, symbol: lookupSymbols, package: cmd/fathom, reachable_from: TestConsoleSearchUsesSharedSnapshot}
- [ ] Operate's Tenancy panel renders `fathom resources --json` with JSON parity, through app-kit roles (§14) {id: RT-121, symbol: renderTenancyPanel, package: cmd/fathom, reachable_from: TestTenancyPanelRendersResourcesJSON}
- [ ] Threshold crossings capture heap, labelled goroutines, the in-flight table, the GC CPU fraction and a flight-recorder trace, rate-limited and retained (§13.4) {id: RT-122, symbol: captureDiagnostics, package: cmd/fathom, reachable_from: TestDiagCaptureRateLimitedAndRetained}
- [ ] pprof stays on TCP behind the loopback Host check with `seconds` capped at 30 and one profile at a time; /debug/inflight joins it (§13.5) {id: RT-123, symbol: guardPprof, package: cmd/fathom, reachable_from: TestPprofLoopbackCappedSingleProfile}
- [ ] /metrics exports the tenancy families (level, budget, pressure, swap, volume free, in-flight, queue, rejected, abandoned, bytes read and written, cache bytes, starts, released memory, GC CPU, SQLite memory, children footprint, quarantined services) (§13.3) {id: RT-124, symbol: writeTenancyMetrics, package: cmd/fathom, reachable_from: TestMetricsTenancyFamilies}
- [ ] `fathom gc` is dry-run by default and applies per-class retention; evidence is archived, never deleted (§12.1) {id: RT-125, symbol: runGC, package: cmd/fathom, reachable_from: TestGCDryRunDeletesNothing}
- [ ] The call log rotates and gzips at 64 MiB with a rollup and archives, never deletes; console tails read the rotated generation (§12.1) {id: RT-126, symbol: rotateCallLog, package: cmd/fathom, reachable_from: TestCallLogRotationArchivesNeverDeletes}
- [ ] Stdio logs rotate on their own ticker, stream the copy, and hard-cap without copying (§12.2) {id: RT-127, symbol: rotateStdioLog, package: cmd/fathom, reachable_from: TestStdioRotationStreamsAndHardCaps}
- [ ] The managed daemon truncates the WAL after 5 idle minutes, at most hourly (§12.2) {id: RT-128, symbol: checkpointWhenIdle, package: cmd/fathom, reachable_from: TestWALTruncatedWhenIdle}
- [ ] Gate runs take a machine-wide lease and run niced with `go test -p 4`; `scripts/gate-lease_test.sh` (run by `task scripts-check`) proves two concurrent holders serialize (§12.3) {id: RT-129, oracle: lexical, path: Taskfile.yml, pattern: "bash scripts/gate-lease_test\.sh"}
- [ ] `gitToplevel` is memoized and cached per cwd (§11.3) {id: RT-130, symbol: gitToplevelCached, package: cmd/fathom, reachable_from: TestGitToplevelCachedPerCwd}
- [ ] Observe-only hook paths exit before any git or daemon work (§11.3) {id: RT-131, symbol: observeOnlyExit, package: cmd/fathom, reachable_from: TestObserveOnlyHooksSkipGitAndDaemon}
- [ ] Observe-only hooks are registered non-blocking (async) where the harness supports it, so the agent's tool call never waits on them (§11.3) {id: RT-169, symbol: hookRegistrationAsync, package: internal/harness, reachable_from: TestObserveOnlyHooksRegisteredNonBlocking}
- [ ] The Stop hook reads the transcript from a per-session offset (§11.3) {id: RT-132, symbol: readTranscriptFromOffset, package: cmd/fathom, reachable_from: TestStopHookReadsTranscriptIncrementally}
- [ ] Identity comes from a cheap /identity endpoint through the cached resolver; SubagentStart reuses the parent's resolution (§11.3) {id: RT-133, symbol: handleIdentity, package: cmd/fathom, reachable_from: TestIdentityEndpointAndSubagentReuse}
- [ ] The custodian's config walk is capped in depth, entries and time, refuses non-git roots, and debounces FileChanged audits across sessions (§11.3) {id: RT-134, symbol: walkAgentConfigFiles, package: internal/custodian, reachable_from: TestWalkAgentConfigFilesCappedAndDeadlined}
- [ ] Doctor gains hook_budget (with breaker-open edits counted) and disk_retention rows (§11.3, §12.1) {id: RT-135, symbol: checkHookBudget, package: cmd/fathom, reachable_from: TestDoctorHookBudgetAndDiskRetention}
- [ ] `scripts/tenancy-stress.sh` exists, runs against a scratch unmanaged daemon on a store copy with `--lifecycle-id`, and writes a reading with the verdict first (§15.2) {id: RT-136, oracle: lexical, path: scripts/tenancy-stress.sh, pattern: "lifecycle-id"}
- [ ] `scripts/tenancy-stress_test.sh` (run by `task scripts-check`) proves the harness refuses the managed daemon's port, store and call log and a fixture dir on the internal disk, in `--self-check` mode (§15.2) {id: RT-168, oracle: lexical, path: Taskfile.yml, pattern: "bash scripts/tenancy-stress_test\.sh"}

## F. Close — honesty debt paid

- [ ] The tenancy allowlist is empty, or every remaining entry carries its reason {id: RT-137, oracle: lexical, path: .fathom/tenancy-allow.txt, pattern: "^[^#]", eq: 0}
- [ ] O108 and O112 are marked superseded by INC-23 in the observation log {id: RT-138, oracle: lexical, path: .fathom/observations.md, pattern: "superseded by INC-23"}
- [ ] CHANGELOG corrects "bounded at the peaks" and names INC-23 {id: RT-139, oracle: lexical, path: CHANGELOG.md, pattern: "INC-23"}
- [ ] fleet-index.md stops recommending heap_sys as a footprint and names phys_footprint and `fathom resources` {id: RT-140, oracle: lexical, path: docs/guides/fleet-index.md, pattern: "fathom resources"}
- [ ] configuration.md documents `[tenancy]` and states that `memory_limit` is a GC hint, not protection {id: RT-141, oracle: lexical, path: docs/configuration.md, pattern: "\[tenancy\]"}
- [ ] domain.md records the platform facts (HeapSys never falls, RSS vs phys_footprint, jetsam, swap and SQLITE_FULL, on-demand swap files make the used fraction meaningless, KeepAlive semantics, advisory ResidentSetSize, inherited Background QoS, .gitignore as cache authority) {id: RT-142, oracle: lexical, path: docs/intent/domain.md, pattern: "phys_footprint"}

## Measurements (op)

Pre-registered in vision 36 §6. Each closes with its committed reading as the contract.
Every harness run is gated on the operator's go-ahead and runs on a quiet machine.

- [ ] (op) RT-M0: three days in `admission = "observe"` on the post-wave-C binary report would-refuse volume by class and origin; > 5% of hook calls would-refused in ordinary hours retunes slots before enforcing {id: RT-M0, needs: "RT-79,RT-81,RT-82,RT-85"}
- [ ] (op) RT-M1: seven days of operator traffic, max phys_footprint ≤ exit mark at every sample and ≤ budget outside refuse windows, refuse windows ≤ 15 min/day in total, zero exit rungs, zero unclean exits {id: RT-M1}
- [ ] (op) RT-M2: refusals < 1% of calls outside shed/refuse windows; > 5% widens slots {id: RT-M2}
- [ ] (op) RT-M3: `scripts/tenancy-stress.sh` on the wave-B binary (seatbelt without brakes) holds the seatbelt (max footprint ≤ exit mark + 1 GiB, a diag capture, safe mode by start 3, stop by start 5, `/metrics` reachability recorded) and on the post-wave-D binary never exits and stays ≤ budget {id: RT-M3, needs: "RT-136,RT-158"}
- [ ] (op) RT-M4: 60 minutes of real traffic, request-path allocation per call ≤ 1/20 of the heap-0518 window's (730.7 GB ÷ that window's call count, computed and committed with the reading), no Sources/ListAllSymbols frame under a handler, obligations p50 ≤ 60 ms and p99 ≤ 300 ms, app-kit ingest ≤ 300 MiB, doc-only commit 0 typed passes, daemon footprint 10 min after an ingest ≤ 1.25× its pre-ingest footprint {id: RT-M4}
- [ ] (op) RT-M5: SIGKILL mid-load and relaunch, no ingest in the first 60 s, one service at a time, footprint ≤ budget {id: RT-M5, needs: RT-136}
- [ ] (op) RT-M6: after waves C–D, `ProcessType` alternates Background and Standard by day for 14 days; post-edit hook p99 on Background days is within +25% of Standard days; otherwise revert to Standard + Nice 5 {id: RT-M6}
- [ ] (op) RT-M7: GC CPU fraction ≤ 25% sustained below the refuse mark and ≤ 50% in refuse windows, read from the ring and the refuse diag captures; otherwise move the GC hint to `off` {id: RT-M7}
- [ ] (op) RT-M8: seven days, 0 orphaned unmanaged daemons older than 1 h, 0 test rows in the operator call log, post-* hook p99 ≤ 4.0 s, managed idle CPU ≤ 5 s/h, workspace/ ≤ 3 GiB after gc {id: RT-M8}
- [ ] (op) RT-M9: the tenancy layer's own idle overhead ≤ 1% CPU and ≤ 20 MB {id: RT-M9}
- [ ] (op) RT-M10: feasibility per RAM class: the fathom repo's full ingest with typed pass, run in the stress harness at forced budgets of 1, 2 and 4 GiB (the 8, 16 and 32 GiB classes), peaks under each exit mark (self + children); a class that fails raises the budget floor for that class before the exit rung ships there {id: RT-M10, needs: RT-136}
- [ ] (op) RT-M11: `fathom ingest --dry-run --stats` over every registered service with and without `.gitignore` discovery; no service loses more than 1% of its observations outside gitignored directories, or the default does not flip for it {id: RT-M11, needs: RT-70}
- [ ] (op) RT-M12: seven days, disk bytes written by the managed daemon ≤ 2 GiB/day outside explicit ingest {id: RT-M12, needs: RT-162}
- [ ] (op) RT-M13: seven days, time in shed or above ≤ 2% with no host-trigger rung entry while pressure is normal; otherwise retune the host triggers {id: RT-M13, needs: RT-154}

## Deferred

- [-] An ingest-time identifier-occurrence index for `references` {id: RT-D1, disposition: deferred, reason: "the budgeted own-service scan answers today; build only on measured demand", trigger: "references returns a partial scan on more than 10% of calls in a week after RT-86"}
- [-] Byte-weighted admission reservations from an EWMA of past calls {id: RT-D2, disposition: deferred, reason: "slots times per-call ceilings bound the envelope deterministically (ADR-047)", trigger: "RT-M3 shows the admitted envelope exceeded with slots alone"}
- [-] Idle exit of the managed daemon with launchd socket activation {id: RT-D3, disposition: deferred, reason: "a cold start is the stampede that caused INC-23", trigger: "RT-M5 passes twice and RT-M1 holds for 30 days"}
- [-] The decision port ordering the refresh backlog {id: RT-D4, disposition: deferred, reason: "the backlog is serialized and single-digit; unprovable against most-recently-queried order today (vision 36 §8)", trigger: "more than three services queued on 10 or more days in a month"}
- [-] The decision port classifying incident windows for doctor's remedies {id: RT-D5, disposition: deferred, reason: "two labelled incidents cannot prove a margin", trigger: "five or more labelled episodes exist"}
- [-] Ingest and the typed pass in a child worker process (`fathom ingest-worker`) whose memory returns to the OS on exit {id: RT-D6, disposition: deferred, reason: "streaming ingest and the producer governor bound the peak; a worker adds a process per ingest and a second store opener", trigger: "RT-M4 reads the daemon's footprint 10 min after an ingest above 1.25x its pre-ingest footprint, or app-kit ingest above 300 MiB after RT-103"}
- [-] pprof, metrics and /debug/inflight on a private unix socket with its own budget {id: RT-D7, disposition: deferred, reason: "the loopback Host check and admission may already keep /metrics reachable; a socket is a new breaking surface", trigger: "RT-M3 records /metrics unreachable under load"}
- [-] Linux PSI and cgroup budget readers, and a systemd unit {id: RT-D8, disposition: deferred, reason: "no Linux adopter exists; Linux reads RSS and reports pressure and swap Unknown; the systemd mapping is documented only", trigger: "a registered Linux adopter runs the managed daemon"}

## Hazard → row map

Every hazard id from the 2026-09-26 audit (82 entries; `launchd-keepalive-relaunch-loop`
appears in two classes and once here).

| Hazard | Rows |
|---|---|
| obligations-whole-tree-reads | RT-26, RT-55, RT-81, RT-143, RT-83, RT-82, RT-146 |
| connector-discovery-unbounded | RT-66, RT-67, RT-68, RT-69, RT-70, RT-145, RT-M11 |
| list-services-full-symbol-loads | RT-1, RT-2, RT-144, RT-76, RT-79, RT-80, RT-160 |
| references-fleet-wide-default | RT-86, RT-159, RT-87, RT-56 |
| fleet-serviceref-per-call | RT-89 |
| no-cancellation-no-admission-control | RT-53, RT-56, RT-58, RT-59, RT-62, RT-152 |
| autorefresh-ingest-on-request-path | RT-149, RT-150, RT-151, RT-97, RT-99, RT-101 |
| full-table-loads-per-call | RT-75, RT-78, RT-85, RT-94, RT-144 |
| cross-service-scans-in-nav | RT-77, RT-84, RT-160 |
| graph-builds-and-triplicate-graph-caches | RT-74, RT-73 |
| console-model-rebuild-whole-logs | RT-113, RT-112 |
| calllog-tail-cache-unbounded | RT-112 |
| search-tool-unbounded-output | RT-88 |
| batch-unbounded-fanout | RT-60 |
| other-tools-whole-tree-sources | RT-146, RT-71, RT-91, RT-92, RT-93, RT-137 |
| console-loopback-self-calls | RT-115, RT-116, RT-61 |
| refactors-unbounded-goroutines | RT-117, RT-118 |
| session-pages-scan-hook-logs | RT-114 |
| sqlite-uncapped-readers-temp-sorts | RT-78, RT-53 (ADR-020 kept: admission bounds concurrency) |
| budget-after-build | RT-90, RT-57 |
| console-search-cold-fanout | RT-120, RT-75 |
| hot-path-uncapped-single-file-reads | RT-26, RT-27, RT-69 |
| ingest-pass2-holds-whole-corpus | RT-69, RT-145, RT-102, RT-103, RT-68, RT-173, RT-155 |
| autorefresh-sync-unbounded-cascade | RT-149, RT-150, RT-97, RT-99, RT-100, RT-40 |
| abandoned-request-work-continues | RT-58, RT-72, RT-98, RT-172 |
| wazero-module-leak-per-ingest | RT-107 |
| typed-pass-without-go-change | RT-104, RT-105, RT-106 |
| fileresolve-reparses-all-js-py | RT-107, RT-108 |
| no-per-service-ingest-exclusion | RT-149, RT-97, RT-98 |
| soft-limit-no-admission-control | RT-33, RT-35, RT-36, RT-53, RT-153 |
| scavenger-fixed-cadence-blind-metric | RT-45, RT-34 |
| launchd-keepalive-relaunch-loop | RT-13, RT-37, RT-38, RT-39, RT-40, RT-155, RT-156, RT-157, RT-M3 |
| scratch-daemon-leak | RT-20, RT-21, RT-22, RT-24 |
| self-restart-hash-and-abrupt-exit | RT-11, RT-12, RT-44, RT-109 |
| console-refactor-unbounded-goroutines | RT-117, RT-118 |
| console-plans-loopback-verify-amplifier | RT-116, RT-61 |
| event-feeders-poll-without-subscribers | RT-119 |
| autorefresh-git-fork-per-read | RT-101 |
| manifest-watermark-full-read-per-query | RT-110 |
| log-rotation-coarse-and-buffered | RT-127 |
| retained-caches-no-byte-budget | RT-73, RT-74, RT-163 |
| plist-no-os-resource-class | RT-13, RT-35, RT-M6 |
| installed-plist-never-reconciled | RT-14, RT-15, RT-28, RT-157 |
| crash-loop-on-unavailable-store-or-port | RT-42, RT-43, RT-165 |
| scratch-serve-http-no-lifetime-bound | RT-20, RT-21, RT-23 |
| scratch-daemons-run-operator-singletons | RT-21, RT-22, RT-12, RT-167 |
| selfrestart-cooldown-not-cross-process | RT-12 |
| abandoned-requests-keep-running | RT-58, RT-8, RT-9 |
| hook-budget-exceeds-harness-timeout | RT-7, RT-8 |
| hook-retry-storm-no-shared-breaker | RT-9, RT-56 |
| uncached-list-services-in-lifecycle-hooks | RT-133, RT-80, RT-79 |
| hook-process-fanout-per-tool-call | RT-130, RT-131, RT-132, RT-169 |
| tests-default-to-live-daemon-and-operator-state | RT-16, RT-17, RT-18, RT-19, RT-63, RT-164 |
| bench-and-script-daemons-escape-cleanup | RT-20, RT-25, RT-168 |
| gates-spawn-parallel-heavy-work-per-push-and-commit | RT-129, RT-111, RT-170 |
| daemon-forks-git-and-go-toolchain-in-request-path | RT-99, RT-101, RT-106, RT-161 |
| state-files-without-retention | RT-125, RT-126 |
| workspace-bench-artifacts-unbounded | RT-125, RT-135 |
| sqlite-wal-and-store-never-shrink | RT-78, RT-128 |
| custodian-walk-in-hooks-for-non-git-roots | RT-134 |
| pprof-open-on-daemon | RT-123, RT-D7 |
| soft-memory-limit-is-not-a-cap | RT-33, RT-35, RT-36, RT-141 |
| heap-sys-is-the-wrong-instrument | RT-45, RT-49, RT-140 |
| typed-pass-peak-misattributes-process-heap | RT-46, RT-M10 |
| in-flight-work-invisible | RT-122, RT-123, RT-124, RT-52 |
| abandoned-work-runs-on-uncounted | RT-58, RT-64 |
| in-process-registry-bypasses-instruments | RT-61 |
| no-admission-control-or-overload-signal | RT-53, RT-55, RT-56, RT-152 |
| blind-to-host-pressure | RT-32, RT-33, RT-154, RT-M13 |
| launchd-posture-foreground-and-unconditional-keepalive | RT-13, RT-38, RT-157 |
| exits-and-restart-loops-unrecorded | RT-37, RT-44, RT-156 |
| no-automatic-diagnostic-capture | RT-122 |
| pprof-on-shared-unguarded-listener | RT-59, RT-123 |
| doctor-footprint-pull-only-wrong-gauge | RT-49, RT-51 |
| no-resource-history-statusz-operate | RT-47, RT-48, RT-52, RT-121 |
| caches-invisible-and-count-bounded | RT-73, RT-124 |
| sqlite-memory-outside-go-accounting | RT-78, RT-124 |
| disk-writes-and-disk-full-invisible-rendered-as-zero | RT-41, RT-124, RT-162, RT-M12 |
| caller-identity-and-scratch-daemons-unattributable | RT-63, RT-64, RT-23, RT-164 |
| observatory-reads-cost-uncounted | RT-112, RT-113, RT-126 |
| no-idle-shrink-or-exit | RT-73, RT-21; managed idle exit deferred as RT-D3 |
| intent-records-claim-bounded | RT-138, RT-139, RT-140, RT-142 (plus INC-23, INC-24, I47–I53 in this change) |
