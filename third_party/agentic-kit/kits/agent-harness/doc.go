// Package agentharness is the governed tool-calling agent loop — the kit
// that composes the platform's foundations into one Run(ctx, input) call:
// model turn → tool dispatch → governance screens → repeat, until a stop
// condition. The harness owns the LOOP and its governance; everything else
// is a seam.
//
// # Seams (what this kit composes, and why)
//
//   - foundations/llmclient (REQUIRED, injected via Deps.Client) — every
//     model call goes through the single LLMClient seam. The harness adds no
//     retry, no provider logic, and no per-call instrumentation of its own:
//     the caller wraps the client BEFORE injecting it — see "Recommended
//     client composition" below.
//
//   - foundations/agent/conversation (REQUIRED, constructed per Run) — the
//     loop's backbone. Its pairing/alternation invariants (user-first,
//     strict role alternation, tool_use answered by exactly-covering
//     tool_result blocks in the single next user message) make malformed
//     transcripts unrepresentable: a loop whose appends all succeed yields a
//     conversation the Messages API accepts. An append failure therefore
//     means a harness bug or a degenerate provider response, and is one of
//     the few cases where Run returns a Go error.
//
//   - foundations/agent/tool-registry (REQUIRED, injected via Deps.Registry)
//     — Registry.Defs() feeds Request.Tools on every call; DispatchAll
//     executes the model's tool_use blocks sequentially, surfacing every
//     failure mode (unknown tool, policy denial, handler error/panic,
//     timeout, cancellation) in-band as an is_error tool_result, never as a
//     Go error.
//
//   - foundations/governance/budget-guard (OPTIONAL, Deps.Budget) — the
//     per-run spend/request governor. Charge(Config.EstimatedUSDPerCall) is
//     called before EVERY model call; a denial ends the run with StopBudget
//     — a result, not an error. Nil Guard = no budget governance.
//
//   - foundations/security/injection-detector (OPTIONAL, Deps.Detector) —
//     scans every tool result BEFORE it re-enters the conversation. Per
//     Config.InjectionAction a flagged result is blocked (replaced with an
//     is_error tool_result naming the detection — the model sees the
//     refusal; the raw payload never enters the transcript) or annotated
//     (raw payload kept, a warning trailer appended). Nil Detector = no
//     scanning.
//
//   - foundations/telemetry/event + sink (OPTIONAL, Deps.Sink) — one event
//     per loop step (turn index, stop reason or tool names, duration via the
//     injected clock), plus budget-denial and injection events. Events are
//     trace-correlated via event.TraceFrom/event.Apply, so harness steps and
//     llm_call events emitted by an instrument-wrapped client share one
//     RunID; when ctx carries no trace, Run mints one at start. Nil Sink =
//     no telemetry. Emit failures are swallowed — telemetry never breaks the
//     loop.
//
//   - foundations/investigation/investigation-journal (OPTIONAL,
//     Deps.Journal) — a human-readable run trace: run-started,
//     turn-completed, budget-denied, injection-blocked/annotated, run-ended.
//     Journaling is best-effort: append errors are ignored so trace
//     recording can never fail a run. Nil Journal = no journaling.
//
// Nil optional dependencies turn the corresponding feature OFF; the loop
// itself never requires them.
//
// # Recommended client composition
//
// Wrap the provider client before injecting it, retry INNERMOST so cost and
// telemetry observe one logical call rather than one record per attempt (see
// foundations/llmclient/instrument's package doc for the full rationale):
//
//	client := llminstrument.WithCost(
//	        llminstrument.WithTelemetry(
//	                llmclient.WithRetry(provider, retryCfg),
//	                snk, "kits/agent-harness", now),
//	        tracker, table, "kits/agent-harness")
//
// # Stop conditions (v1)
//
// v1 deliberately has NO convergence-detector coupling — the loop stops only
// on these conditions, reported as RunResult.Stop:
//
//   - StopCompleted — the model returned end_turn or stop_sequence (or
//     max_tokens with Config.TreatMaxTokensAsError set false).
//   - StopMaxTurns — Config.MaxTurns model calls completed without a
//     terminal stop reason.
//   - StopBudget — the budget guard denied the pre-call charge.
//   - StopRefusal — the model declined (stop reason refusal). Surfaced
//     distinctly and never retried.
//   - StopMaxTokens — generation hit the MaxTokens cap and
//     TreatMaxTokensAsError is true (the default). Surfaced distinctly;
//     RunResult.FinalText carries the truncated text.
//   - StopCanceled — ctx was cancelled (observed at the top of the loop, or
//     as a Canceled/DeadlineExceeded error from the client).
//
// A tool_use stop reason CONTINUES the loop: it is a successful completion —
// the calls are dispatched and the next turn proceeds.
//
// # Errors vs stops
//
// Run returns a non-nil error ONLY for infrastructure failures: empty user
// input, a client error that survived caller-side retries, a conversation
// invariant violation, or a stop reason this build does not model (e.g.
// pause_turn — fail closed rather than loop blindly). Governance stops —
// budget, max turns, refusal, max_tokens, cancellation — are RunResult.Stop
// values with a nil error. On an error return, RunResult.Stop is StopError
// and RunResult.Messages still carries the transcript so far.
//
// # Multi-run statelessness
//
// A Harness holds only resolved configuration and injected dependencies:
// every Run builds a fresh conversation, so nothing leaks between runs.
// Cross-run state lives in the deps by design (the budget guard's counters,
// the journal's entries, the sink's backlog).
//
// # Concurrency contract
//
// A Harness is safe for concurrent Run calls IF the injected dependencies
// are: LLMClient implementations and Sinks must be concurrency-safe by their
// own contracts, Registry and budget-guard Guard are, and Detector.Scan is
// read-only after construction. A shared Guard across concurrent runs is the
// POINT — all runs draw down one budget, and the guard's mutex arbitrates.
// The EXCEPTION is Deps.Journal: investigation-journal is single-goroutine
// by contract, so either serialize Runs or give each Harness its own
// Journal when journaling is on.
package agentharness
