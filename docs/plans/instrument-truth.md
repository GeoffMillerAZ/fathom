# instrument truth — absence is a claim

> Governing sources: `docs/vision/24-work-frontier.md` §7,
> `docs/vision/14-impact-observatory.md` (law 12),
> `docs/design/intervention-outcomes.md` §3 (the honesty rule),
> 2026-08-21 flywheel audit (12-agent evaluation; DQ-1..6).
>
> **Covenant (ADR-0070 class):** failing test first, `task validate`
> green before commit, dated CHANGELOG entry, boxes flip only on
> `fathom verify --service fathom --plan docs/plans/instrument-truth.md`.
>
> The theme: every zero must be distinguishable — silent-by-design
> (arm), nothing-yet (ENOENT), and broken-reader (EPERM) are three
> different facts, and the 2026-08-21 audit caught the instruments
> conflating all three: a TCC wedge fabricated 3 IDLE loops + "call log
> not found" + zero verdicts in one run (all false, all gone after one
> `tccutil reset`), and the OFF arm's window rendered a PASSing wired
> hook as "not installed".
>
> **Wave 1 (IT-1..5) gates the re-arm:** the ON arm has 0 admissible
> units (DQ-3) and must not start its second collection cycle until the
> OFF arm stops zeroing its own counters and O2 stops suppressing the
> treatment effect.

## Wave 1 — before ON collection restarts

- [x] flywheel classifies call-log read outcomes three ways — readable, absent (ENOENT → IDLE), errored (EPERM/other → OPEN with the errno verbatim and, for paths under an external volume, the TCC-wedge hint naming the tccutil reset one-liner); today `loadFlywheelCallLog` discards the error and a wedged reader reports "nothing to measure" at exit 0 {id: IT-1, symbol: flywheelCallLog, package: cmd/fathom, reachable_from: TestFlywheelReadErrorIsOpenNeverIdle}
- [x] one shared arm-state helper answers "was an OFF arm active over this window", and every absence renderer consults it: the flywheel >24h staleness branch and the channels/guarded-edit not-measurable notes say "OFF measurement arm active (N session(s) remaining) — channels silent by design; absence is the experiment", never "hook not installed", when the arm covers the window {id: IT-2, symbol: ArmStateForWindow, package: internal/metrics, reachable_from: TestAbsenceNotesAreArmAware}
- [x] counting-only beats survive FATHOM_CHANNELS=off: the edit-hook's OFF-arm gate splits into inject-gates (obey, stdout byte-identical to today) and count-gates (seen/observer beats still emitted, stamped arm:off) so the OFF arm stops deleting the baseline denominator the pre-registered A/B needs {id: IT-3, symbol: cmdEditHook, package: cmd/fathom, reachable_from: TestOffArmStillCountsSeenBeats}
- [x] doctor surface_traffic falls back to persistent evidence when the process-lifetime counter reads zero: an agent-surface call-log row within 7 days → PASS noting "live counter reset by restart; persistent log shows N agent call(s), last <ts>"; no agent row ever → FAIL stands — the chronic post-deploy binary_fresh+surface_traffic co-FAIL that trains the reader to ignore red ends here {id: IT-4, symbol: checkSurfaceTraffic, package: cmd/fathom, reachable_from: TestSurfaceTrafficFallsBackToPersistentLog}
- [x] a session-hook fetch failure is a transient, named as one — never "repo is not an indexed service": fetchServices returns its error and the brief line says the daemon was unreachable (index-first tools may still work) instead of steering the agent off index-first discovery for the whole session; only a successful fetch that genuinely lacks the service may claim unindexed (observation O2) {id: IT-5, symbol: fetchServices, package: cmd/fathom, reachable_from: TestSessionHookNamesFetchFailureNotUnindexed}

## Wave 2 — cheap instrumentation the report already asks for

- [x] the answered beat carries the displaced-rule count the hook already computes and then drops — a count only, never the evicted rules themselves (the T5-2/O14 rule) — so obligations-cap pressure becomes a trendable number {id: IT-6, symbol: cmdEditHook, package: cmd/fathom, reachable_from: TestAnsweredBeatCarriesDisplacedCount}
- [x] the review tool's result payload includes a verdict_outcome map (active/fixed/superseded finding counts) mirroring verify_plan's, so the call logger's existing extraction picks reviews up and the review lane finally contributes to gate-catch counts {id: IT-7, symbol: reviewVerdictOutcome, package: internal/mcp, reachable_from: TestReviewEmitsVerdictOutcome}
- [x] `fathom metrics interventions` defaults --store to the repo's resolved store path instead of silently skipping claim_events, unlocking the already-recorded superseded-findings stream {id: IT-8, symbol: cmdMetricsInterventions, package: cmd/fathom, reachable_from: TestInterventionsDefaultsStorePath}
- [x] machine-minted thread notes carry a close condition at mint — session-end notes close when the named working tree is clean again (or a newer session-end supersedes them), pre-compact notes close when their session ends — so evaluateThreadConditions retires them and the open set stops burying real handoffs (lifetime closure today: 0/2 pre-compact, 5/8 hook auto-notes rotting) {id: IT-9, symbol: threadAutoNote, package: cmd/fathom, reachable_from: TestMachineMintedThreadsCarryConditions}
- [x] observation ids are minted under a cross-process file lock (the upgrade path observe.go's own comment pre-approves): two concurrent `fathom observe` calls cannot mint the same O<N> — durable-ID integrity is what the operator contract's reference-point rule depends on {id: IT-10, symbol: cmdObserve, package: cmd/fathom, reachable_from: TestObserveIdsSurviveConcurrentMint}

## Wave 3 — the numbers leave the repo only reconciled

- [x] instrument --adoption's verify-gate figure derives from the verify ledger with its denominator named in the sentence, replacing the 215/4940-family double-count (O6/DQ-5); the three circulating figures (lifetime-instrumented, window, ledger rows) either agree or are labeled as different measurements of different things {id: IT-11, symbol: gateCatchStatsFromLedger, package: cmd/fathom, reachable_from: TestAdoptionVerifyFigureMatchesLedger}
- [x] no surface sums or trends whole-session tokens_total when the transcript exceeded the head-read cap: 163/370 ledger rows carry totals frozen by the 16MB LimitReader (DQ-1) and any aggregate over them must use per-turn rates over the parsed span, with the truncation named {id: IT-12, symbol: buildImpactModel, package: cmd/fathom, reachable_from: TestFrozenTotalsNeverSummed}
- [x] a mechanical retro lands per session: the Stop hook runs `fathom retro --last --write` (LLM-free, zero tokens) so the firings→outcomes join becomes a standing corpus instead of a run-on-request, and per-session verify-catch attribution stops being the flywheel's permanent asterisk {id: IT-13, symbol: runStopHookRetro, package: cmd/fathom, reachable_from: TestStopHookLandsRetroArtifact}

## Wave 4 — the ingest cannot lie about what it could not read (O16)

- [x] the extractor's per-file read failures are counted and named in the ingest report's failed column — an ingest under a wedged reader prints the errno and the external-volume hint, never "188 pkg(s) (0 failed)" over a zero-symbol result {id: IT-14, symbol: cmdIngest, package: cmd/fathom, reachable_from: TestIngestReportNamesReadFailures}
- [x] an ingest that hit read failures and extracted zero symbols REFUSES to replace an existing non-empty index for that service — exits non-zero naming the cause — so a wedge can never overwrite a good index with a plausible empty one (the O16 incident: five consecutive runs did exactly that) {id: IT-15, symbol: cmdIngest, package: cmd/fathom, reachable_from: TestIngestRefusesEmptyOverwriteOnReadErrors}

## Explicitly deferred, with reasons

- Re-anchor event beats: staleness tooling had 0 calls in window — one
  emit is cheap but the signal is empty until the tooling gets traffic;
  after the waves above, not before.
- non_conforming_artifacts_refused: zero module bindings exist and the
  deployed daemon predates list_modules — instrumenting it now measures
  a structurally-empty stream. Bind one module first.
