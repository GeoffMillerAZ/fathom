// Package evalrunner is the CUE-configured batch eval runner — the kit that
// drives scenario-bank scenarios through an LLM-backed qaharness.Judge and
// refuses, by construction, the standard ways an eval loop lies: a judge
// grading its sibling model, a verdict that never reached a journal, a
// budget silently blown, a "stable" claim nobody probed, and a run over
// nothing exiting green.
//
// # Seams (what this kit composes — nothing here is re-derived)
//
//   - foundations/quality/qa-harness — the Judge interface. This package
//     contributes LLMJudge, the model-backed implementation the foundation
//     deliberately does not ship.
//   - foundations/llmclient/model-catalog — ValidateIndependence at
//     construction, for every (judge lane, producer) pair. The catalog is
//     ALWAYS caller data; the kit ships no rows.
//   - foundations/llmclient/instrument — WithBudget wraps every judge lane
//     at construction: fail-closed pre-charge via the caller's
//     budget-guard Guard, settle to actuals. The price table is caller
//     data; a nil table prices every call at $0 (the guard's request-count
//     dimensions still govern — stated, not hidden).
//   - foundations/llmclient — ProbeAll over the judge lanes before the
//     first scenario of every Run (lanes without a Pinger are NOT probed;
//     see non-guarantees).
//   - foundations/quality/eval-journal — every judge call in the main loop
//     is appended as an evaljournal.Record via evaljournal.Append over the
//     caller's jsonl-log Writer.
//   - foundations/quality/scenario-bank — LoadBank is the runner's bank
//     path (scenariobank.Load underneath: production-captured scenarios
//     are refused there), and Run re-validates every scenario before
//     judging, so a bank that dodged validation still cannot run.
//   - foundations/quality/determinism-gate — Probe (K >= 3) before any run
//     is reported Stable; FitnessFunctions compiled from LoopConfig().
//   - foundations/quality/diff-reporter + eval-gate — the regression gate
//     (Regression method): baseline JSONL vs current records, verdict
//     flips and score drops fed through a weighted-threshold Gate.
//   - foundations/quality/reliability-stats — Regression also reports
//     per-scenario pass^3 (RegressionResult.PassPowK) over the current
//     records: reported next to the gate, never gated on.
//   - foundations/quality/fitness-function-registry — Fitness() registers
//     the four eval.* loop-honesty functions and executes them.
//   - foundations/llmclient/replay — replay mode is composition, not a
//     code path: back the judge lanes with replay.NewReplayClient (with
//     MatchByRequestHash) and Run is fully offline. Run judges lanes and
//     scenarios SEQUENTIALLY, in config and bank order, precisely so that
//     ordinal cassette replay is deterministic: determinism-probe entries
//     first (K of them, scheduling order), then one entry per
//     (scenario, lane) pair.
//
// # Honesty invariants (construction- or run-time, never vibes)
//
//   - Decisive verdicts REQUIRE validated independence: any
//     modelcatalog.ValidateIndependence violation is construction-fatal.
//     The ONLY way an all-same-family config constructs is
//     Config.NonDecisive = true, and then no record is ever marked
//     decisive. Record.Decisive additionally requires
//     Scenario.DecisiveEligible, a non-empty rationale, and a
//     pass/fail/absent_proven verdict.
//   - A budget denial mid-run journals verdict:"error" records for every
//     remaining (scenario, lane) pair — StopBudget, never silent
//     truncation, never missing records.
//   - An empty bank returns ErrEmptyBank: a run over nothing cannot exit
//     zero by construction.
//   - LoopConfig() is computed from observed wiring events (validation
//     results at New, journal presence, bank validation at New via
//     Config.ScenarioBankPath or during Run, probe completion during Run)
//     — never from literal booleans.
//
// # Non-guarantees (stated as plainly as the guarantees)
//
//   - The runner proves independence, journaling, budget enforcement, and
//     probe wiring. It NEVER proves judge correctness: an LLM judge is a
//     model with all of a model's failure modes (position bias, verbosity
//     bias, style bias survive family separation — see model-catalog's
//     package doc).
//   - Replay-green is not live-green: a cassette-backed Run proves the
//     code path against a recorded world, not current model behavior.
//   - ProbeAll proves lane reachability and credential acceptance, not
//     model access or quota (see llmclient.Pinger). Lanes with no Pinger
//     in Deps.Pingers and whose client does not implement Pinger are not
//     probed at all.
//   - A Stable probe result is measured consistency over K calls, not
//     determinism (see determinism-gate's non-guarantees).
//   - The regression gate compares only (scenario, concern) pairs present
//     in the baseline or current set; scenarios added since the baseline
//     are not regressions by definition.
//   - A per-scenario pass^3 from fewer than three judged trials is a
//     point on too little evidence, not a reliability claim; Trials and
//     Excluded are reported so a consumer can apply its own floor.
//
// # Concurrency contract
//
// A Runner is safe for concurrent use: construction-time state is
// immutable, and the observed-wiring flags behind LoopConfig are guarded
// by a sync.Mutex. Each Run call is internally SEQUENTIAL by design (see
// replay ordering above); concurrent Run calls on one Runner are safe —
// the journal Writer and the budget Guard are mutex-guarded by their own
// contracts — but interleave journal lines and are not cassette-replayable.
// LLMJudge is safe for concurrent Score calls; the per-call verdict sink
// is invoked synchronously inside Score under the judge's mutex, so a
// non-reentrant sink is safe. Deps.Now, when nil, defaults to time.Now
// (the documented default for this seam).
package evalrunner
