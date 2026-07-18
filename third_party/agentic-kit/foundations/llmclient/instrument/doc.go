// Package llminstrument provides observability middleware for
// llmclient.LLMClient: WithTelemetry emits one llm_call telemetry event per
// LLM call, and WithCost records token usage and USD cost into a
// costtracker.Tracker. Both wrap any LLMClient from the outside; the core
// llmclient package never imports this one. Dependencies point one way:
//
//	llminstrument → llmclient, telemetry/event, telemetry/sink, cost/cost-tracker
//
// There is no schema.cue here: the package is behavioral middleware with no
// data contract of its own (the event shape it emits is owned by
// foundations/telemetry/event; the price table shape by
// foundations/cost/cost-tracker).
//
// # Recommended composition order
//
//	client := llminstrument.WithBudget(
//	        llminstrument.WithCost(
//	                llminstrument.WithTelemetry(
//	                        llmclient.WithRetry(provider, retryCfg),
//	                        s, "my-component", nil),
//	                tracker, table, "my-component"),
//	        guard, table, nil, "my-model")
//
// WithRetry sits INNERMOST (closest to the provider), so telemetry and cost
// observe one LOGICAL call rather than one record per attempt. The decision,
// and why:
//
//   - Event cardinality stays 1:1 with logical calls. Dashboards, budgets,
//     and joins between telemetry events and cost records keep a stable shape
//     during retry storms — exactly when you read them most.
//   - Failed attempts return no Usage, so there is nothing meaningful to bill
//     per attempt; the final response's usage is the single correct ledger
//     entry, and WithCost records it exactly once per logical call.
//   - The final outcome is the operationally meaningful one: a rate limit
//     retried into success IS a success. Per-attempt retry visibility is
//     preserved through the "attempts" event attribute, stamped from
//     Response.Attempts by WithRetry ("0" means no retry wrapper present).
//   - duration_ms then measures the caller-experienced latency of the logical
//     call, including backoff sleeps — the number a budget owner cares about.
//
// Callers who instead need one event per external attempt can invert the
// order — llmclient.WithRetry(llminstrument.WithTelemetry(provider, ...), cfg)
// — at the price of unstable event cardinality and an "attempts" attribute
// that is always "0" (WithRetry stamps the attempt count after telemetry has
// already observed each response). The recommended order is the tested one;
// see the composition test.
//
// WithCost goes outside WithTelemetry so the telemetry duration excludes
// tracker bookkeeping; the two are otherwise order-independent because both
// observe the same response.
//
// WithBudget sits OUTERMOST so that it charges once per LOGICAL call. Because
// WithRetry is inside the budget wrapper, retries are transparent to the guard:
// the budget sees one charge per caller-visible attempt, not one per network
// attempt. This is the correct billing semantics — the budget enforces spend
// on logical calls, not on individual network attempts. BudgetExceededError is
// non-retryable (it matches neither RateLimitError nor ServerError), so even
// if WithBudget were inside WithRetry it would not loop — but it would charge
// again on each retry, which is wrong.
//
// # Failure isolation
//
// Telemetry must never break the call path. Sink emit failures — and events
// that fail validation — are swallowed, never returned to the caller, and
// counted; see TelemetryClient.DroppedEvents. Responses and errors from the
// inner client always pass through unchanged.
//
// # Streaming
//
// Both middlewares wrap the stream channel: every chunk is forwarded
// unchanged, and the terminal chunk's StopReason/Usage (or Err) feeds the
// event or cost record after the inner channel closes. The wrapped channel
// closes only after that bookkeeping completes, so a caller that drained the
// stream may immediately read the sink or tracker. If the caller abandons the
// stream by cancelling ctx, forwarding stops and the wrapper goroutine still
// drains the inner channel until the producer closes it (the llmclient
// contract guarantees producers close promptly on cancellation), then
// terminates — no goroutine leaks. Abandoning a stream WITHOUT cancelling ctx
// violates the llmclient streaming contract ("callers MUST drain the channel
// until it closes") and blocks the wrapper exactly as it would block an
// un-wrapped producer.
//
// Concurrency contract: the wrappers returned by WithTelemetry and WithCost
// are safe for concurrent use provided their collaborators honour their own
// contracts (LLMClient implementations and Sinks must be concurrency-safe;
// Tracker is; PriceTable must not be mutated after construction). Each
// wrapped stream is owned by exactly one goroutine that closes it exactly
// once.
package llminstrument
