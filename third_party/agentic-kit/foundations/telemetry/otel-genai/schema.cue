// Package otelgenai CUE schema.
//
// This file captures the semconv mapping table: kit event/attribute vocabulary
// → OTel GenAI semantic-convention names.  Assumed semconv version: v1.27.0
// (https://opentelemetry.io/docs/specs/semconv/gen-ai/).
//
// Signal choice: span (trace) for llm_call events; OTLP Trace signal
// (ExportTraceServiceRequest).  The kit's llm_call event is the atomic unit
// of LLM interaction — start, duration, input/output token counts — which maps
// cleanly onto a span (start + end timestamps, attributes).  Journal entries
// and info/error events map onto span events within the same span.  One signal
// done well beats two half-done; span traces are the highest-value signal for
// LLM observability (Grafana, Honeycomb, Jaeger all have GenAI trace views).
package otelgenai

// #SemconvVersion is the OTel semantic-convention version assumed by this
// package.  Bump this when the mapping table is updated to a newer spec.
#SemconvVersion: "1.27.0"

// #GenAISystem is the value written into gen_ai.system for kit-originated
// spans.  Callers that front a specific provider should override this via
// Config.ExtraAttributes.
#GenAISystem: "agentic-kit"

// #AttrMapping documents the stable attribute mapping from kit vocabulary to
// OTel GenAI semconv names.  This is normative: otelgenai.go implements
// exactly this table.  Drift is a bug.
#AttrMapping: {
	// Kit telemetry/event attribute → OTel span attribute.
	//
	// Reserved trace-correlation attributes (event.AttrRunID etc.) are mapped
	// to the W3C TraceID/SpanID fields in the OTLP envelope, not as span
	// attributes — they are structural, not observational.

	// Model name  →  gen_ai.request.model  (string)
	"model": "gen_ai.request.model"

	// Input token count  →  gen_ai.usage.input_tokens  (int)
	"input_tokens": "gen_ai.usage.input_tokens"

	// Output token count  →  gen_ai.usage.output_tokens  (int)
	"output_tokens": "gen_ai.usage.output_tokens"

	// Cache-read token count  →  gen_ai.usage.input_tokens_cached  (int)
	// (semconv v1.27.0 uses this key for cache-hit tokens)
	"cache_read_tokens": "gen_ai.usage.input_tokens_cached"

	// Operation performed  →  gen_ai.operation.name  (string)
	// Fixed value "chat" for standard completion calls.
	"operation": "gen_ai.operation.name"

	// System (provider) identifier  →  gen_ai.system  (string)
	"system": "gen_ai.system"

	// Stop reason  →  gen_ai.response.finish_reasons  (string[])
	// Rendered as a single-element JSON array string in the span attribute.
	"stop_reason": "gen_ai.response.finish_reasons"

	// Tool names dispatched  →  gen_ai.tool.name  (string)
	// Comma-joined list when multiple tools were called.
	"tool_names": "gen_ai.tool.name"
}
