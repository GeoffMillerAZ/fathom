// Package otelgenai provides an OTel GenAI semconv mapping and a stdlib-only
// OTLP/JSON sink that implements foundations/telemetry/sink.Sink.
//
// # Signal choice
//
// Kit llm_call events are emitted as OTLP traces (spans) via
// ExportTraceServiceRequest.  Rationale: an llm_call is atomic LLM work with
// a start time, an end time, and well-defined attributes (model, token counts,
// stop reason).  That maps 1:1 onto an OTel span.  Traces are also the
// highest-value signal for GenAI observability — Grafana, Honeycomb, Jaeger,
// and the CNCF OTel Collector all have native GenAI trace views.  Journal
// (info) and error events from the same run are recorded as span events on the
// encompassing llm_call span, keyed by the shared RunID.
//
// # Semconv version
//
// Assumed: v1.27.0 (https://opentelemetry.io/docs/specs/semconv/gen-ai/).
// The mapping table is in schema.cue (#AttrMapping) and mirrored in
// semconvAttr() below.  Bump both when the spec changes.
//
// # Failure policy
//
// Drop-and-count: when the HTTP POST to the OTLP endpoint fails (network
// error, non-2xx response), the batch is silently dropped and an internal
// counter is incremented.  Rationale: observability signals MUST NOT block or
// fail the agent loop — a dropped span is far better than a hung or erroring
// Emit call.  Callers that need durability should wrap this sink in a fan-out
// (telemetry/sink.MultiSink) with a local JSONL sink as a secondary.
//
// # Batching
//
// Events accumulate in an in-memory buffer.  The buffer is flushed when it
// reaches Config.BatchSize (default 64) events OR when Flush() is called
// explicitly.  There is no time-based flush in this package (no goroutines, no
// timers) — callers that want periodic flushing wrap in a loop.
//
// # Concurrency
//
// OTLPSink is safe for concurrent use.  All mutable state is guarded by a
// sync.Mutex.
package otelgenai

import (
	"net/http"
	"time"

	"github.com/geoffmilleraz/agentic-kit/foundations/telemetry/metrics"
	agentharness "github.com/geoffmilleraz/agentic-kit/kits/agent-harness"
)

// DefaultBatchSize is the number of events that trigger an automatic flush
// when Config.BatchSize is zero.
const DefaultBatchSize = 64

// SemconvVersion is the OTel semantic-convention version assumed by this
// package.  Mirrors #SemconvVersion in schema.cue.
const SemconvVersion = "1.27.0"

// Doer is the HTTP client interface used by OTLPSink, satisfied by
// *http.Client.  Inject a custom implementation in tests to avoid network
// calls.
type Doer interface {
	Do(*http.Request) (*http.Response, error)
}

// Config holds the construction-time settings for OTLPSink.
type Config struct {
	// Endpoint is the OTLP/HTTP endpoint URL for ExportTraceServiceRequest,
	// e.g. "http://localhost:4318/v1/traces".  Required.
	Endpoint string

	// ServiceName is the value written into resource.attributes as
	// "service.name".  Defaults to "agentic-kit" when empty.
	ServiceName string

	// BatchSize is the number of events that trigger an automatic flush.
	// Zero or negative uses DefaultBatchSize (64).
	BatchSize int

	// ExtraAttributes is a static set of additional span attributes merged onto
	// every span (e.g. "deployment.environment" = "prod").  Values are strings.
	ExtraAttributes map[string]string

	// Now is the injected clock.  Nil defaults to time.Now (documented).
	// Always inject in tests.
	Now func() time.Time

	// Registry, when non-nil, receives the "otelgenai_drops_total" counter
	// incremented on each failed batch export.  A nil Registry means drops are
	// counted nowhere (Emit still no-ops on failure).
	Registry *metrics.Registry

	// HTTPClient is the HTTP client used for OTLP posts.  When nil OTLPSink
	// uses http.DefaultClient.  Inject an httptest-backed client in tests.
	HTTPClient Doer
}

// semconvAttr maps a kit attribute key to its OTel GenAI semconv name.
// Mirrors #AttrMapping in schema.cue.  Returns "" when the key has no mapping
// (the attribute is omitted from the span).
func semconvAttr(kitKey string) string {
	switch kitKey {
	case "model":
		return "gen_ai.request.model"
	case "input_tokens":
		return "gen_ai.usage.input_tokens"
	case "output_tokens":
		return "gen_ai.usage.output_tokens"
	case "cache_read_tokens":
		return "gen_ai.usage.input_tokens_cached"
	case "operation":
		return "gen_ai.operation.name"
	case "system":
		return "gen_ai.system"
	case agentharness.AttrStopReason: // "stop_reason"
		return "gen_ai.response.finish_reasons"
	case agentharness.AttrToolNames: // "tool_names"
		return "gen_ai.tool.name"
	default:
		return ""
	}
}

// ─── OTLP JSON wire types ─────────────────────────────────────────────────────
//
// Minimal subset of the OTLP protobuf-JSON encoding for trace spans, per
// https://opentelemetry.io/docs/specs/otlp/#otlphttp.  Field names are
// camelCase to match proto3 JSON encoding.  All timestamps are decimal strings
// (uint64 nanoseconds) to avoid float64 precision loss.

type exportTraceServiceRequest struct {
	ResourceSpans []resourceSpans `json:"resourceSpans"`
}

type resourceSpans struct {
	Resource   otlpResource `json:"resource"`
	ScopeSpans []scopeSpans `json:"scopeSpans"`
}

type otlpResource struct {
	Attributes []keyValue `json:"attributes"`
}

type scopeSpans struct {
	Scope instrumentationScope `json:"scope"`
	Spans []otlpSpan           `json:"spans"`
}

type instrumentationScope struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// otlpSpan is a single OTLP span.
// Kind 3 = SPAN_KIND_CLIENT (LLM call as a client call to an LLM service).
type otlpSpan struct {
	TraceID           string      `json:"traceId"`                // 32 hex lowercase
	SpanID            string      `json:"spanId"`                 // 16 hex lowercase
	ParentSpanID      string      `json:"parentSpanId,omitempty"` // 16 hex or ""
	Name              string      `json:"name"`
	Kind              int         `json:"kind"`              // 3 = CLIENT
	StartTimeUnixNano string      `json:"startTimeUnixNano"` // decimal uint64 string
	EndTimeUnixNano   string      `json:"endTimeUnixNano"`   // decimal uint64 string
	Attributes        []keyValue  `json:"attributes"`
	Events            []spanEvent `json:"events,omitempty"`
	Status            spanStatus  `json:"status"`
}

type spanEvent struct {
	TimeUnixNano string     `json:"timeUnixNano"`
	Name         string     `json:"name"`
	Attributes   []keyValue `json:"attributes,omitempty"`
}

type spanStatus struct {
	// Code: 0=STATUS_CODE_UNSET, 1=STATUS_CODE_OK, 2=STATUS_CODE_ERROR.
	Code int `json:"code"`
}

// keyValue encodes one OTLP AnyValue.  Only StringValue and IntValue are used
// in this package — all kit attributes are strings; integer-typed semconv
// attributes (token counts) are parsed and emitted as IntValue when possible.
type keyValue struct {
	Key   string   `json:"key"`
	Value anyValue `json:"value"`
}

type anyValue struct {
	StringValue *string `json:"stringValue,omitempty"`
	IntValue    *int64  `json:"intValue,omitempty"`
}

func stringAttr(k, v string) keyValue {
	s := v
	return keyValue{Key: k, Value: anyValue{StringValue: &s}}
}

func intAttr(k string, v int64) keyValue {
	return keyValue{Key: k, Value: anyValue{IntValue: &v}}
}
