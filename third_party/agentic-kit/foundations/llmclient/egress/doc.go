// Package llmegress provides egress-gating middleware for
// llmclient.LLMClient: WithEgress passes every outbound text-bearing field of
// a Request through a caller-supplied Transformer before the inner client
// sees it. It is the kit's own "gate before egress" doctrine as a reusable
// middleware — tokenize, redact, or otherwise rewrite sensitive text at the
// trust boundary, in one place, instead of remembering to do it at every call
// site. Dependencies point one way:
//
//	llmegress → llmclient, security/tokenization, security/pre-llm-dlp-gate,
//	            provenance/ai-provenance
//
// There is no schema.cue here: the package is behavioral middleware with no
// data contract of its own. Per-org detection catalogs (what counts as
// sensitive, which classes, which namespaces) are the CALLER's concern,
// supplied through their Transformer — nothing org-specific is hard-coded.
//
// # The central guarantee: fail closed
//
// The inner client NEVER sees un-transformed outbound text. Every text field
// of every request is passed through the Transformer before delegation, and
// any Transformer error aborts the call with a typed *TransformError — the
// inner client is not invoked at all on failure. A gate that degrades to
// pass-through on error is not a gate; refusal is the only safe failure mode.
//
// # What is transformed (and what is not)
//
// Gating is by FIELD, not by declared block type, so a mis-tagged block
// cannot smuggle text past the gate. Transformed on every request:
//
//   - Request.Prompt (even when Messages is non-empty and clients would
//     ignore it — the guarantee is "no un-transformed text crosses", not
//     "no text the inner client happens to read crosses)";
//   - Request.System;
//   - every ContentBlock.Text and every ContentBlock.Content (tool_result
//     output) in every message.
//
// Empty fields are passed through without invoking the Transformer (nothing
// leaves, so there is nothing to gate); this keeps invocation counts
// meaningful.
//
// NOT transformed by default — tool_use Input and BlockUnknown Raw. They are
// structured JSON, and blindly string-transforming structured JSON corrupts
// it: a redaction label inserted mid-document breaks parsing, and a token
// substituted inside a JSON string silently changes tool semantics. This is
// the same boundary the replay package's scrubber draws, for the same reason.
// The TransformToolJSON option opts those fields in, with the corruption
// caveat above: use it only when your Transformer is JSON-shape-safe (e.g.
// it only substitutes values that cannot span JSON syntax), and prefer
// gating the text that PRODUCED the tool input instead.
//
// NEVER transformed — ToolDef Name/Description/InputSchema and the protocol
// identifiers ContentBlock.ID / Name / ToolUseID. Tool definitions are the
// caller's own authored text and schema, not user data flowing through the
// system; identifiers are provider-assigned plumbing. If you put secrets in
// your own tool descriptions, this middleware will not save you.
//
// # Deep copy
//
// The inner client receives a DEEP COPY of the request: fresh Messages and
// Content slices, byte-cloned json.RawMessage payloads, cloned StopSequences
// and Tools. The caller's Request, Messages, and blocks are never mutated,
// and nothing the inner client receives aliases caller-owned memory.
//
// # Streaming and inbound traffic
//
// Stream applies the same outbound transformation before delegating; the
// returned channel is the inner client's channel, untouched. Inbound traffic
// — Complete responses and stream chunks — passes through verbatim in both
// directions of the wrapper. Egress gating is about what LEAVES the trust
// boundary; the model's output is inbound, and inspecting, detokenizing, or
// validating it is a different concern with different tools
// (output-integrity-checker, the caller's own detokenization flow).
//
// # Composition order
//
// Recommended:
//
//	client := llminstrument.WithCost(
//	        llminstrument.WithTelemetry(
//	                llmegress.WithEgress(
//	                        llmclient.WithRetry(provider, retryCfg),
//	                        transformer),
//	                s, "my-component", nil),
//	        tracker, table, "my-component")
//
// WithEgress sits OUTSIDE WithRetry — WithEgress(WithRetry(inner)) — so the
// Transformer runs once per LOGICAL call and every retry attempt re-sends the
// already-gated request. The decision, and why:
//
//   - Transformers may be nondeterministic: GateFromVault mints a fresh
//     random token per detected value. Inside retry, each attempt would send
//     DIFFERENT bytes for the same logical call — breaking the "retrying the
//     identical request" semantics WithRetry is built on, breaking
//     deterministic request hashing (provenance digests, replay matching,
//     prompt-cache prefixes), and minting orphan vault entries plus audit
//     noise per failed attempt.
//   - Re-running the transform per attempt buys nothing: the outbound text is
//     the same text on every attempt.
//
// This ordering is pinned by a test (transformer invocation count == 1 per
// non-empty field across a retried call). Telemetry and cost sit outside
// WithEgress in the recommended order, which is harmless — they record
// attributes (model, tokens, durations), never request text. Middleware that
// DOES record request text (e.g. replay's RecordingClient) must be placed
// deliberately: inside WithEgress — WithEgress(Recording(inner)) — if its
// records must hold gated text; outside if it must capture raw text on
// purpose (and then its own scrubbing story applies).
//
// # Non-guarantees (honest contract)
//
// WithEgress guarantees every outbound text field passes THROUGH the
// Transformer — it cannot guarantee the Transformer removes anything.
// GateFromDLP inherits pre-llm-dlp-gate's honesty contract: best-effort
// secret-SHAPE redaction, not DLP-as-guarantee. GateFromVault detects only
// what its rules' regexps match; encoded, split, or novel-format values pass
// straight through. Tool JSON and tool definitions are un-gated by default,
// as documented above. And a vault-tokenized prompt means the model reasons
// over opaque tokens — the quality trade-off is the caller's to make.
//
// Concurrency contract: the wrapper returned by WithEgress holds no mutable
// state after construction and is safe for concurrent use provided the inner
// client and the Transformer are; Transformers used with a concurrently-used
// client MUST be safe for concurrent use (GateFromVault and GateFromDLP are,
// given their collaborators' contracts). Stream channels are owned by the
// inner client per the llmclient contract.
package llmegress
