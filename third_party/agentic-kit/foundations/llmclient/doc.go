// Package llmclient provides the single LLM interface for the agentic-kit
// platform. Everything that calls a model goes through here: one interface
// (LLMClient), one retry strategy (WithRetry), one escalation model (Ladder),
// one late-binding wrapper (Swappable, for credentials that arrive at
// runtime), one test double (MockClient), and one conformance suite (the
// llm-test subpackage). This doc comment doubles as the contract's ADR: it
// records the binding design decisions of the v2 contract and why they were
// made.
//
// Composition: the package is provider-agnostic and imports nothing beyond the
// standard library. Provider adapters (HTTP clients), telemetry, cost
// accounting, and policy enforcement are middleware that wrap LLMClient from
// the outside; none of them are referenced here. Foundations import only
// stdlib and sibling foundations — this package is a leaf.
//
// Concurrency contract: LLMClient implementations must be safe for concurrent
// use. MockClient is safe for concurrent use (guarded by a sync.Mutex).
// Swappable is safe for concurrent use (atomic inner-client pointer; Swap
// never interrupts in-flight calls). Ladder is NOT safe for concurrent use
// without external synchronisation. Each channel returned by Stream is owned
// by exactly one producer goroutine and is closed exactly once by that
// producer.
//
// # Requests: single-turn and multi-turn
//
// Request supports both the v1 single-turn form and the v2 conversation form.
// When Request.Messages is empty, clients treat Request.Prompt as a single
// user message — the v1 form keeps working unchanged. When Messages is
// non-empty it is the conversation and Prompt is ignored. Request.Model names
// the model to use; when empty, the client's configured default applies (this
// is what lets Ladder compose with the interface — see Rung.Apply).
//
// # Tool use
//
// Request.Tools declares the tools the model may call; Request.ToolChoice
// constrains how (the zero value means "auto": the model decides). A Response
// whose StopReason is StopToolUse is a SUCCESSFUL completion, not an error —
// it is never retried and never an escalation signal. The agentic loop is:
//
//	resp, _ := client.Complete(ctx, req)
//	req.Messages = append(req.Messages, resp.AsMessage())
//	var results []ContentBlock
//	for _, call := range resp.ToolCalls() {
//		out := runTool(call.Name, call.Input)
//		results = append(results, ToolResultBlock(call.ID, out, false))
//	}
//	req.Messages = append(req.Messages, Message{Role: RoleUser, Content: results})
//	// ... call Complete again with the extended history.
//
// ALL tool results answering one assistant turn travel in ONE user message,
// as above. Appending one user message per result produces consecutive
// single-result user messages under parallel tool use (a model may request
// several tools in a single turn — Anthropic does so by default), and
// providers reject that conversation shape. The foundations/agent/conversation
// package (Conversation.AppendToolResults) enforces this invariant
// structurally and makes the mistake unrepresentable — prefer it over
// hand-rolling the history bookkeeping.
//
// # Unknown content blocks and raw preservation
//
// Providers emit block types this package does not model (thinking blocks,
// citations, future inventions), and also annotate known types with extra
// fields (e.g. citation annotations on text blocks, server-tool payloads on
// tool_use blocks). All inbound content blocks — typed (text, tool_use) and
// untyped alike — carry their verbatim provider JSON in ContentBlock.Raw.
// Clients must send them back untouched when assistant content is appended to
// history via AsMessage. Multi-turn round-trips never drop data; that
// invariant is what keeps this contract forward-compatible with provider
// evolution.
//
// # Raw precedence rule (outbound)
//
// When an adapter serialises a ContentBlock for an outbound request:
//
//   - If Raw is non-empty, the adapter emits Raw VERBATIM. All typed fields
//     (Text, ID, Name, Input, …) are ignored for wire encoding.
//   - If Raw is empty, the adapter serialises from the typed fields.
//
// This gives callers lossless round-trip fidelity: receive a block → include
// it in the next request → the provider sees the exact same bytes.
//
// IMPORTANT: Raw wins when non-empty, even if typed fields were modified.
// A caller that edits a received block's typed fields (e.g. changing Text)
// MUST clear Raw (set it to nil) to force re-serialisation from the updated
// typed fields. Leaving Raw non-empty after editing typed fields means the
// provider sees the original, unmodified bytes — the edit is silently dropped.
//
// Caller-constructed blocks (Raw is nil) are unaffected: the adapter always
// serialises from typed fields, as before.
//
// CAVEAT: the verbatim-preservation invariant holds for Complete AND Stream.
// Streamed unknown blocks now accumulate their deltas (thinking_delta,
// signature_delta, and unrecognized delta types) and emit a final ContentBlock
// whose Raw matches what Complete would have produced for the same response.
//
// # Stop reasons
//
// StopReason is an OPEN string type. The constants cover the reasons known
// today (StopEndTurn, StopToolUse, StopMaxTokens, StopSequenceHit,
// StopRefusal, StopPauseTurn, StopContextExceeded), but providers add new
// reasons over time. Never switch exhaustively over StopReason — handle the
// values you know and treat anything else as "stopped for a reason this build
// does not recognise".
//
// # Prompt caching
//
// CacheHint on ContentBlock and ToolDef, plus Request.SystemCacheHint, ask the
// provider to place a cache breakpoint at that position. The semantics are
// Anthropic's: breakpoints attach to content blocks, tool definitions, and the
// system prompt; a request may carry at most 4 breakpoints; only prefixes
// longer than a model-specific minimum (roughly 1k–4k tokens depending on the
// model) are cacheable; below that minimum the hint is a SILENT no-op. Hints
// are therefore best-effort by design — verify cache effectiveness via
// Usage.CacheReadInputTokens on the response, never by assumption.
//
// # Streaming semantics
//
// Stream(ctx, req) returns (<-chan StreamChunk, error). The returned error
// covers pre-stream failures only (the request never started); once a channel
// is returned, all subsequent outcomes — including failures — travel on the
// channel. The contract:
//
//   - The producer closes the channel exactly once, and callers MUST drain
//     the channel until it closes.
//   - Text arrives incrementally as chunks with non-empty TextDelta.
//   - tool_use (and any other non-text) blocks are buffered inside the client
//     and emitted as a single complete *ContentBlock via StreamChunk.Block —
//     v2 deliberately has no partial-JSON tool-input deltas.
//   - Unknown block types (e.g. thinking blocks) are accumulated from their
//     start JSON and all subsequent deltas (thinking_delta, signature_delta,
//     and any unrecognised delta types), then emitted as a complete
//     BlockUnknown on content_block_stop. The Raw field carries the
//     fully-reconstructed provider JSON, equivalent to what Complete produces
//     for the same response. Streamed BlockUnknown blocks MAY be safely
//     round-tripped into history.
//   - The terminal chunk carries StopReason and a non-nil Usage on success,
//     or a non-nil Err on failure. After the terminal chunk the channel
//     closes; no chunk ever follows a chunk with Err != nil.
//   - Producers select on ctx.Done() and terminate without goroutine leaks.
//     On cancellation the channel closes promptly; a terminal chunk carrying
//     ctx.Err() is delivered best-effort (the caller already knows it
//     cancelled, so delivery is not guaranteed if the caller stopped
//     reading).
//
// # Retry and escalation
//
// WithRetry retries RateLimitError and ServerError with exponential backoff
// (plus optional seeded jitter); InvalidRequestError, AuthError,
// UnavailableError (an unconfigured Swappable — retrying cannot conjure a
// credential), and every other error type fail fast. A nil error is always success: in particular, a
// Response with StopReason == StopToolUse is a successful completion — it is
// never retried. When a RateLimitError carries a RetryAfter hint (> 0), that
// hint takes precedence over the computed backoff for that attempt.
//
// Ladder is an ordered sequence of model tiers, cheapest first, that advances
// only on an explicit escalation Signal from the caller. Rungs carry a Model;
// Rung.Apply injects it into any Request whose Model is empty, which is how a
// Ladder composes with the LLMClient interface:
//
//	resp, err := client.Complete(ctx, ladder.Current().Apply(req))
//
// # Extended thinking and effort
//
// Request.Thinking (a *ThinkingConfig) activates extended thinking on models
// that support it. Set Thinking.Type to "enabled" and optionally set
// Thinking.BudgetTokens. The anthropic adapter maps this to the "thinking"
// wire object; the bedrock adapter passes it through identically (both
// InvokeModel and InvokeModelWithResponseStream use the same Anthropic
// Messages body on Bedrock). A nil Thinking is a no-op — no wire field is
// sent.
//
// Request.Effort is an open-ended string (e.g. "low", "medium", "high") for
// models with a reasoning-effort knob. Where no provider knob exists, the
// adapter returns a typed InvalidRequestError. This is deliberately not
// silently dropped: an Effort value the caller set explicitly is a statement
// of intent, and silently ignoring it would let quota-saving errors masquerade
// as success. Use Request.ProviderParams to pass experimental effort fields
// directly until the provider stabilises the API surface.
//
// TEMPERATURE-ON-FRONTIER HAZARD: current frontier models (e.g. Claude 3.7+
// with extended thinking active) return a 400 InvalidRequestError when
// Temperature is set alongside thinking. The kit does not enforce this locally
// because the constraint is model-specific; model-catalog-driven validation is
// the correct seam (Config.CapabilityCheck, wired by callers). Until catalog
// validation is wired, leave Temperature at zero when thinking is enabled.
//
// # Provider extension seam (ProviderParams)
//
// Request.ProviderParams is a json.RawMessage containing a JSON object that is
// merged VERBATIM into the outbound wire request body at the top level, after
// the adapter maps the typed fields. Provider-side keys the adapter already
// set are OVERRIDDEN by ProviderParams. This is the structural escape hatch
// for provider-specific fields not yet modeled by the contract (experimental
// betas, future sampling parameters, etc.). Precedence is explicit by design:
// ProviderParams is a power tool — the caller owns the semantics, the
// serialisation, and any provider errors that result.
//
// Empty or nil ProviderParams is a no-op. Both Complete and Stream honour it
// identically. The adapter validates that ProviderParams, when non-empty, is a
// JSON object (not an array, string, or other scalar); a non-object value is
// rejected with a typed InvalidRequestError before any HTTP. MockClient applies
// the same object-or-empty check.
//
// The v2 canonical hash drops ProviderParams when nil or empty (the zero-value
// dropping rule), so adding ProviderParams to a new field does not invalidate
// existing cassettes recorded without it.
//
// # Structured outputs (OutputSchema)
//
// Request.OutputSchema is a json.RawMessage containing a JSON Schema object
// that constrains the response format. When non-empty, adapters map it to the
// provider's structured-outputs surface. For both Anthropic and
// Anthropic-on-Bedrock the wire shape is:
//
//	"output_config": {
//	    "format": {
//	        "type": "json_schema",
//	        "schema": <OutputSchema>
//	    }
//	}
//
// The adapter validates that OutputSchema is a JSON object (not an array,
// string, or null) before any HTTP and returns a typed InvalidRequestError on
// failure. MockClient applies the same object-or-empty check.
//
// # Belt-and-suspenders design (non-guarantee)
//
// OutputSchema constrains the provider's GENERATION — the provider attempts to
// emit JSON conforming to the schema. This is NOT a correctness guarantee:
// providers may produce non-conforming output under error conditions or model
// limitations. Callers SHOULD validate the parsed artifact against the
// originating CUE definition (via cue vet or equivalent). The design intent is
// "the contract that validates the artifact is the same one that constrained
// generation" — redundancy by design, not accident.
//
// # CUE→schema→constrained-generation loop
//
// The toolchains/jsonschema package (DeriveFromDir / DeriveFromBytes) derives
// a JSON Schema draft 2020-12 from a CUE definition. Cast the result to
// json.RawMessage and assign to OutputSchema:
//
//	schema, err := jsonschemagen.DeriveFromBytes(cueSrc, jsonschemagen.Options{})
//	req.OutputSchema = json.RawMessage(schema)
//
// Layering note: foundations packages must NOT import toolchains. Committed
// testdata/output_schema_golden.json fixtures in the anthropic package record
// the derived schema (annotated with their CUE source); the derivation
// round-trip test lives in toolchains/jsonschema (which MAY import foundations)
// and proves the full loop.
//
// # Precedence with ProviderParams
//
// ProviderParams is merged AFTER the adapter maps OutputSchema to
// output_config.format. A ProviderParams key of "output_config" will OVERRIDE
// the adapter-set value — that is intentional: ProviderParams remains the
// escape hatch for fine-grained control.
//
// # Streaming
//
// OutputSchema applies to Stream identically to Complete — the same
// output_config.format mapping is used on both paths.
//
// # ToolDef.Strict
//
// ToolDef.Strict, when true, sets "strict":true on the tool wire object. The
// provider treats this as a structured-outputs hint constraining tool-input
// generation to the declared InputSchema. Ignored when ToolDef.Raw is set —
// the caller owns the raw JSON and must include "strict":true inside Raw if
// desired.
//
// # Model-capability validation seam
//
// Both adapter Configs expose an optional CapabilityCheck field:
//
//	type Config struct {
//	    ...
//	    CapabilityCheck func(req Request) error
//	}
//
// When non-nil, CapabilityCheck is called in buildBody before any wire
// encoding. It is the integration point for model-catalog-driven validation —
// checking that the named model supports thinking, effort, structured outputs,
// etc. The kit ships no catalog; callers wire their own check here. A nil
// CapabilityCheck is a no-op (permissive). This seam deliberately avoids
// importing foundations/llmclient/model-catalog so the adapter stays a leaf.
//
// # Breaking changes from v1
//
//   - Stream's signature changed from (<-chan string, error) to
//     (<-chan StreamChunk, error). The old channel-of-string form could not
//     report mid-stream errors, stop reasons, usage, or tool calls; that
//     behaviour is gone.
//   - RateLimitError.RetryAfter changed from int (seconds) to time.Duration
//     (zero = unknown).
//
// Everything else is source-compatible with v1: Request.Prompt / System /
// MaxTokens / Temperature / StopSequences, Response.Text (still a populated
// field equal to the concatenation of the response's text blocks),
// Response.Attempts / Rung / InputTokens / OutputTokens (the token fields are
// now promoted from the embedded Usage struct), and all MockClient
// constructors (NewMockClient, MockEntryOK, MockEntryErr, MockEntryRateLimit,
// MockEntryInvalid).
package llmclient
