package otelgenai

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"sync"
	"time"

	"github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/telemetry/event"
)

// OTLPSink implements telemetry/sink.Sink.  It batches kit telemetry events
// into OTLP spans and POSTs them as ExportTraceServiceRequest JSON to the
// configured endpoint.
//
// Only KindLLMCall events produce a span; KindInfo and KindError events are
// recorded as span events on the most recent LLM-call span for the same RunID.
// KindToolCall events are recorded as span events on the same span.
//
// Failure policy: drop-and-count (see package doc).
// Batching: flush at BatchSize or on explicit Flush() (see package doc).
//
// Concurrency: safe for concurrent use.
type OTLPSink struct {
	mu        sync.Mutex
	cfg       Config
	batchSize int
	buffer    []event.Event
	client    Doer
	now       func() time.Time
}

// NewOTLPSink constructs an OTLPSink from cfg.  Returns an error when
// cfg.Endpoint is empty.
func NewOTLPSink(cfg Config) (*OTLPSink, error) {
	if cfg.Endpoint == "" {
		return nil, fmt.Errorf("otelgenai: Config.Endpoint must not be empty")
	}
	bs := cfg.BatchSize
	if bs <= 0 {
		bs = DefaultBatchSize
	}
	cl := cfg.HTTPClient
	if cl == nil {
		cl = http.DefaultClient
	}
	now := cfg.Now
	if now == nil {
		now = time.Now
	}
	return &OTLPSink{
		cfg:       cfg,
		batchSize: bs,
		client:    cl,
		now:       now,
	}, nil
}

// Emit accepts one kit telemetry event. It appends the event to the buffer and
// flushes when the buffer reaches BatchSize.  Emit never returns an error for
// export failures (drop-and-count policy); it returns an error only for
// internal encoding bugs (practically unreachable).
func (s *OTLPSink) Emit(e event.Event) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.buffer = append(s.buffer, e)
	if len(s.buffer) >= s.batchSize {
		s.flushLocked()
	}
	return nil
}

// Flush exports all buffered events immediately.  It is safe to call
// concurrently.  Export failures are dropped and counted; Flush itself returns
// nil for export failures (drop-and-count policy).
func (s *OTLPSink) Flush() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.flushLocked()
}

// flushLocked drains the buffer.  Must be called with s.mu held.
func (s *OTLPSink) flushLocked() {
	if len(s.buffer) == 0 {
		return
	}
	batch := s.buffer
	s.buffer = nil

	payload, err := s.buildPayload(batch)
	if err != nil {
		// Encoding bug — drop and count.
		s.incDrop()
		return
	}
	if err := s.post(payload); err != nil {
		s.incDrop()
	}
}

// incDrop increments the drop counter if a Registry is configured.
func (s *OTLPSink) incDrop() {
	if s.cfg.Registry != nil {
		s.cfg.Registry.Counter("otelgenai_drops_total").Inc()
	}
}

// post sends the JSON payload to the OTLP endpoint.
func (s *OTLPSink) post(payload []byte) error {
	req, err := http.NewRequest(http.MethodPost, s.cfg.Endpoint, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("otelgenai: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("otelgenai: POST: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("otelgenai: OTLP endpoint returned %d", resp.StatusCode)
	}
	return nil
}

// ─── payload builder ──────────────────────────────────────────────────────────

// buildPayload converts a batch of kit events into an OTLP JSON payload.
//
// Grouping logic:
//   - Events with a RunID attribute are grouped by RunID. All events for a
//     RunID are emitted into one span (the llm_call events set the span
//     timestamps; other events become span events within the same span).
//   - Events without a RunID each become their own span.
//   - Within a RunID group, llm_call events set Start/End timestamps (first
//     llm_call sets Start; last llm_call sets End). If no llm_call is present,
//     Start = first event timestamp, End = last event timestamp.
func (s *OTLPSink) buildPayload(batch []event.Event) ([]byte, error) {
	// Group events by RunID.
	type group struct {
		runID  string
		events []event.Event
	}
	var (
		groups  []group
		byRunID = map[string]int{} // runID → index in groups
	)
	for _, ev := range batch {
		runID := ev.Attributes[event.AttrRunID]
		if runID == "" {
			// Ungrouped: its own span.
			groups = append(groups, group{runID: "", events: []event.Event{ev}})
			continue
		}
		idx, ok := byRunID[runID]
		if !ok {
			idx = len(groups)
			byRunID[runID] = idx
			groups = append(groups, group{runID: runID})
		}
		groups[idx].events = append(groups[idx].events, ev)
	}

	spans := make([]otlpSpan, 0, len(groups))
	for _, g := range groups {
		spans = append(spans, s.groupToSpan(g.runID, g.events))
	}

	serviceName := s.cfg.ServiceName
	if serviceName == "" {
		serviceName = "agentic-kit"
	}

	payload := exportTraceServiceRequest{
		ResourceSpans: []resourceSpans{
			{
				Resource: otlpResource{
					Attributes: []keyValue{
						stringAttr("service.name", serviceName),
						stringAttr("telemetry.sdk.language", "go"),
						stringAttr("otel.semconv.version", SemconvVersion),
					},
				},
				ScopeSpans: []scopeSpans{
					{
						Scope: instrumentationScope{
							Name:    "github.com/geoffmilleraz/agentic-kit/foundations/telemetry/otel-genai",
							Version: SemconvVersion,
						},
						Spans: spans,
					},
				},
			},
		},
	}
	return json.Marshal(payload)
}

// groupToSpan converts one RunID-group of events into a single OTLP span.
func (s *OTLPSink) groupToSpan(runID string, events []event.Event) otlpSpan {
	// Extract trace/span correlation from the first event that has them.
	var traceID, spanID, parentSpanID string
	for _, ev := range events {
		if ev.Attributes[event.AttrRunID] != "" && traceID == "" {
			// Pad or use run ID as trace ID (W3C trace IDs are 32 hex chars;
			// kit RunIDs are already 32 hex chars by convention).
			traceID = padHex(ev.Attributes[event.AttrRunID], 32)
		}
		if ev.Attributes[event.AttrSpanID] != "" && spanID == "" {
			spanID = padHex(ev.Attributes[event.AttrSpanID], 16)
		}
		if ev.Attributes[event.AttrParentSpanID] != "" && parentSpanID == "" {
			parentSpanID = padHex(ev.Attributes[event.AttrParentSpanID], 16)
		}
	}
	if traceID == "" {
		// Ungrouped event: generate stable IDs from the event's own ID.
		if len(events) > 0 {
			traceID = padHex(events[0].ID, 32)
			spanID = padHex(events[0].ID, 16)
		} else {
			traceID = zeroPad(32)
			spanID = zeroPad(16)
		}
	}
	if spanID == "" {
		spanID = zeroPad(16)
	}

	// Determine span start/end from llm_call event timestamps; fall back to
	// first/last event timestamp.
	var startTime, endTime time.Time
	for _, ev := range events {
		if ev.Kind == event.KindLLMCall {
			if startTime.IsZero() || ev.Timestamp.Before(startTime) {
				startTime = ev.Timestamp
			}
			if endTime.IsZero() || ev.Timestamp.After(endTime) {
				endTime = ev.Timestamp
			}
		}
	}
	if startTime.IsZero() {
		for _, ev := range events {
			if startTime.IsZero() || ev.Timestamp.Before(startTime) {
				startTime = ev.Timestamp
			}
		}
	}
	if endTime.IsZero() {
		for _, ev := range events {
			if ev.Timestamp.After(endTime) {
				endTime = ev.Timestamp
			}
		}
	}
	if endTime.IsZero() || !endTime.After(startTime) {
		endTime = startTime.Add(time.Nanosecond) // ensure end > start
	}

	// Build span attributes from the llm_call event (if any), then merge extras.
	spanAttrs := s.buildSpanAttributes(events)

	// Build span events from non-llm_call kit events.
	var spanEvents []spanEvent
	for _, ev := range events {
		if ev.Kind == event.KindLLMCall {
			continue // the llm_call event IS the span
		}
		spanEvents = append(spanEvents, eventToSpanEvent(ev))
	}

	// Determine status code.
	statusCode := 0 // unset
	for _, ev := range events {
		if ev.Kind == event.KindError {
			statusCode = 2 // ERROR
			break
		}
	}

	// Span name: "gen_ai chat <model>" if model is known, else "gen_ai llm_call".
	spanName := "gen_ai llm_call"
	for _, a := range spanAttrs {
		if a.Key == "gen_ai.request.model" && a.Value.StringValue != nil {
			spanName = "gen_ai chat " + *a.Value.StringValue
			break
		}
	}

	return otlpSpan{
		TraceID:           traceID,
		SpanID:            spanID,
		ParentSpanID:      parentSpanID,
		Name:              spanName,
		Kind:              3, // SPAN_KIND_CLIENT
		StartTimeUnixNano: fmtNano(startTime),
		EndTimeUnixNano:   fmtNano(endTime),
		Attributes:        spanAttrs,
		Events:            spanEvents,
		Status:            spanStatus{Code: statusCode},
	}
}

// buildSpanAttributes extracts semconv span attributes from the group's events.
// Priority: llm_call event attributes first, then other events' attributes.
// Extra attributes from Config are merged last (caller wins over kit).
func (s *OTLPSink) buildSpanAttributes(events []event.Event) []keyValue {
	// Always add gen_ai.system.
	seen := map[string]bool{"gen_ai.system": true}
	attrs := []keyValue{stringAttr("gen_ai.system", "agentic-kit")}

	// Always add gen_ai.operation.name = "chat".
	seen["gen_ai.operation.name"] = true
	attrs = append(attrs, stringAttr("gen_ai.operation.name", "chat"))

	addAttr := func(k, v string) {
		if seen[k] || v == "" {
			return
		}
		seen[k] = true
		// Integer-typed semconv keys.
		switch k {
		case "gen_ai.usage.input_tokens", "gen_ai.usage.output_tokens", "gen_ai.usage.input_tokens_cached":
			if n, err := strconv.ParseInt(v, 10, 64); err == nil {
				attrs = append(attrs, intAttr(k, n))
				return
			}
		}
		attrs = append(attrs, stringAttr(k, v))
	}

	// Process llm_call events first.
	for _, ev := range events {
		if ev.Kind != event.KindLLMCall {
			continue
		}
		for kitKey, kitVal := range ev.Attributes {
			if otelKey := semconvAttr(kitKey); otelKey != "" {
				addAttr(otelKey, kitVal)
			}
		}
		// Kit llm_call events may carry model directly.
		if model := ev.Attributes["model"]; model != "" {
			addAttr("gen_ai.request.model", model)
		}
	}

	// Then all other events.
	for _, ev := range events {
		if ev.Kind == event.KindLLMCall {
			continue
		}
		for kitKey, kitVal := range ev.Attributes {
			if otelKey := semconvAttr(kitKey); otelKey != "" {
				addAttr(otelKey, kitVal)
			}
		}
	}

	// Merge ExtraAttributes (caller wins for conflicts).
	// Sort keys for determinism.
	extraKeys := make([]string, 0, len(s.cfg.ExtraAttributes))
	for k := range s.cfg.ExtraAttributes {
		extraKeys = append(extraKeys, k)
	}
	sort.Strings(extraKeys)
	for _, k := range extraKeys {
		v := s.cfg.ExtraAttributes[k]
		if seen[k] {
			// Overwrite: remove old, add new.
			newAttrs := attrs[:0]
			for _, a := range attrs {
				if a.Key != k {
					newAttrs = append(newAttrs, a)
				}
			}
			attrs = newAttrs
		}
		seen[k] = true
		attrs = append(attrs, stringAttr(k, v))
	}

	return attrs
}

// eventToSpanEvent converts a non-llm_call kit event to an OTLP span event.
func eventToSpanEvent(ev event.Event) spanEvent {
	se := spanEvent{
		TimeUnixNano: fmtNano(ev.Timestamp),
		Name:         string(ev.Kind) + ":" + ev.Source,
	}
	// Carry kit-specific attributes as span event attributes (non-semconv).
	for k, v := range ev.Attributes {
		if k == event.AttrRunID || k == event.AttrSpanID || k == event.AttrParentSpanID {
			continue // structural, not observational
		}
		se.Attributes = append(se.Attributes, stringAttr(k, v))
	}
	// Sort for determinism.
	sort.Slice(se.Attributes, func(i, j int) bool {
		return se.Attributes[i].Key < se.Attributes[j].Key
	})
	return se
}

// ─── formatting helpers ───────────────────────────────────────────────────────

// fmtNano formats a time as a decimal uint64 nanosecond string (OTLP JSON
// timestamp encoding).
func fmtNano(t time.Time) string {
	n := t.UnixNano()
	if n < 0 {
		return "0"
	}
	return strconv.FormatUint(uint64(n), 10)
}

// padHex pads or truncates s to exactly n hex characters by left-padding with
// zeros.  If s is longer than n, the rightmost n characters are used (so the
// least-significant nibbles are preserved).
func padHex(s string, n int) string {
	// Strip any non-hex characters that might come from non-standard IDs.
	clean := make([]byte, 0, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F') {
			clean = append(clean, c)
		}
	}
	if len(clean) >= n {
		return string(clean[len(clean)-n:])
	}
	pad := make([]byte, n-len(clean))
	for i := range pad {
		pad[i] = '0'
	}
	return string(pad) + string(clean)
}

// zeroPad returns a string of n '0' characters.
func zeroPad(n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = '0'
	}
	return string(b)
}
