package otelgenai_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/geoffmilleraz/agentic-kit/foundations/telemetry/event"
	"github.com/geoffmilleraz/agentic-kit/foundations/telemetry/metrics"
	otelgenai "github.com/geoffmilleraz/agentic-kit/foundations/telemetry/otel-genai"
)

// ─── clock ────────────────────────────────────────────────────────────────────

var epoch = time.Date(2025, 6, 1, 12, 0, 0, 0, time.UTC)

// ─── HTTP test server ─────────────────────────────────────────────────────────

// otlpServer starts an httptest server that accepts OTLP/JSON posts.  It
// returns the endpoint URL, a drain function that returns all recorded request
// bodies, and a cleanup registered with t.Cleanup.
func otlpServer(t *testing.T, statusCode int) (url string, bodies func() [][]byte) {
	t.Helper()
	var mu sync.Mutex
	var recorded [][]byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		mu.Lock()
		recorded = append(recorded, body)
		mu.Unlock()
		w.WriteHeader(statusCode)
	}))
	t.Cleanup(srv.Close)
	return srv.URL + "/v1/traces", func() [][]byte {
		mu.Lock()
		defer mu.Unlock()
		cp := make([][]byte, len(recorded))
		copy(cp, recorded)
		return cp
	}
}

// ─── NewOTLPSink ─────────────────────────────────────────────────────────────

func TestNewOTLPSink_EmptyEndpointErrors(t *testing.T) {
	_, err := otelgenai.NewOTLPSink(otelgenai.Config{})
	if err == nil {
		t.Fatal("expected error for empty endpoint, got nil")
	}
}

func TestNewOTLPSink_ValidConfig(t *testing.T) {
	url, _ := otlpServer(t, 200)
	s, err := otelgenai.NewOTLPSink(otelgenai.Config{
		Endpoint: url,
		Now:      func() time.Time { return epoch },
	})
	if err != nil {
		t.Fatalf("NewOTLPSink: %v", err)
	}
	if s == nil {
		t.Fatal("expected non-nil sink")
	}
}

// ─── Emit + batch flush ───────────────────────────────────────────────────────

func TestOTLPSink_FlushOnBatchSize(t *testing.T) {
	url, drain := otlpServer(t, 200)
	s, err := otelgenai.NewOTLPSink(otelgenai.Config{
		Endpoint:  url,
		BatchSize: 2,
		Now:       func() time.Time { return epoch },
	})
	if err != nil {
		t.Fatalf("NewOTLPSink: %v", err)
	}

	// Emit 2 events — should trigger a flush at the 2nd.
	for i := 0; i < 2; i++ {
		ev := event.Event{
			ID:        "ev",
			Kind:      event.KindLLMCall,
			Source:    "test",
			Timestamp: epoch,
			Attributes: map[string]string{
				event.AttrRunID:  "run-abc",
				event.AttrSpanID: "span-001",
			},
		}
		if err := s.Emit(ev); err != nil {
			t.Fatalf("Emit: %v", err)
		}
	}

	bodies := drain()
	if len(bodies) == 0 {
		t.Fatal("expected at least one OTLP POST after batch-size flush")
	}
}

func TestOTLPSink_ExplicitFlush(t *testing.T) {
	url, drain := otlpServer(t, 200)
	s, err := otelgenai.NewOTLPSink(otelgenai.Config{
		Endpoint:  url,
		BatchSize: 100, // large — won't auto-flush
		Now:       func() time.Time { return epoch },
	})
	if err != nil {
		t.Fatalf("NewOTLPSink: %v", err)
	}

	ev := event.Event{
		ID:         "ev-flush",
		Kind:       event.KindInfo,
		Source:     "test",
		Timestamp:  epoch,
		Attributes: map[string]string{event.AttrRunID: "run-xyz"},
	}
	if err := s.Emit(ev); err != nil {
		t.Fatalf("Emit: %v", err)
	}

	// No post yet.
	if len(drain()) != 0 {
		t.Fatal("expected no POST before explicit Flush")
	}

	s.Flush()

	bodies := drain()
	if len(bodies) == 0 {
		t.Fatal("expected POST after explicit Flush")
	}
}

// ─── OTLP payload structure ───────────────────────────────────────────────────

// testPayload is a minimal struct for asserting top-level OTLP payload shape.
type testPayload struct {
	ResourceSpans []struct {
		Resource struct {
			Attributes []struct {
				Key   string `json:"key"`
				Value struct {
					StringValue *string `json:"stringValue"`
				} `json:"value"`
			} `json:"attributes"`
		} `json:"resource"`
		ScopeSpans []struct {
			Spans []struct {
				TraceID    string `json:"traceId"`
				SpanID     string `json:"spanId"`
				Name       string `json:"name"`
				Kind       int    `json:"kind"`
				Attributes []struct {
					Key   string `json:"key"`
					Value struct {
						StringValue *string `json:"stringValue"`
						IntValue    *int64  `json:"intValue"`
					} `json:"value"`
				} `json:"attributes"`
				Events []struct {
					Name string `json:"name"`
				} `json:"events"`
				Status struct {
					Code int `json:"code"`
				} `json:"status"`
			} `json:"spans"`
		} `json:"scopeSpans"`
	} `json:"resourceSpans"`
}

func TestOTLPSink_PayloadStructure(t *testing.T) {
	url, drain := otlpServer(t, 200)
	reg := metrics.NewRegistry()
	s, err := otelgenai.NewOTLPSink(otelgenai.Config{
		Endpoint:    url,
		BatchSize:   1,
		ServiceName: "test-service",
		Now:         func() time.Time { return epoch },
		Registry:    reg,
	})
	if err != nil {
		t.Fatalf("NewOTLPSink: %v", err)
	}

	ev := event.Event{
		ID:        "ev-llm",
		Kind:      event.KindLLMCall,
		Source:    "test",
		Timestamp: epoch,
		Attributes: map[string]string{
			event.AttrRunID:  "aabbccdd",
			event.AttrSpanID: "11223344",
			"model":          "claude-3-5-sonnet",
		},
	}
	if err := s.Emit(ev); err != nil {
		t.Fatalf("Emit: %v", err)
	}

	bodies := drain()
	if len(bodies) == 0 {
		t.Fatal("no OTLP POST received")
	}

	var p testPayload
	if err := json.Unmarshal(bodies[0], &p); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}

	if len(p.ResourceSpans) == 0 {
		t.Fatal("expected resourceSpans")
	}
	rs := p.ResourceSpans[0]

	// service.name in resource.
	foundSvc := false
	for _, a := range rs.Resource.Attributes {
		if a.Key == "service.name" && a.Value.StringValue != nil && *a.Value.StringValue == "test-service" {
			foundSvc = true
		}
	}
	if !foundSvc {
		t.Error("service.name not found in resource attributes")
	}

	if len(rs.ScopeSpans) == 0 || len(rs.ScopeSpans[0].Spans) == 0 {
		t.Fatal("expected at least one span")
	}
	span := rs.ScopeSpans[0].Spans[0]

	// Span kind = 3 (CLIENT).
	if span.Kind != 3 {
		t.Errorf("span kind = %d, want 3 (CLIENT)", span.Kind)
	}

	// span name contains model.
	if !strings.Contains(span.Name, "claude-3-5-sonnet") {
		t.Errorf("span name %q should contain model name", span.Name)
	}

	// gen_ai.system attribute.
	foundSystem := false
	for _, a := range span.Attributes {
		if a.Key == "gen_ai.system" {
			foundSystem = true
		}
	}
	if !foundSystem {
		t.Error("gen_ai.system not found in span attributes")
	}

	// gen_ai.operation.name attribute.
	foundOp := false
	for _, a := range span.Attributes {
		if a.Key == "gen_ai.operation.name" && a.Value.StringValue != nil && *a.Value.StringValue == "chat" {
			foundOp = true
		}
	}
	if !foundOp {
		t.Error("gen_ai.operation.name='chat' not found in span attributes")
	}

	// gen_ai.request.model attribute.
	foundModel := false
	for _, a := range span.Attributes {
		if a.Key == "gen_ai.request.model" && a.Value.StringValue != nil && *a.Value.StringValue == "claude-3-5-sonnet" {
			foundModel = true
		}
	}
	if !foundModel {
		t.Error("gen_ai.request.model not found in span attributes")
	}

	// No drops: the drop counter should not appear in Expose() output (it's
	// zero and was never incremented).
	exposed := reg.Expose()
	if strings.Contains(exposed, "otelgenai_drops_total") {
		t.Errorf("expected no drop counter, but found it in metrics: %s", exposed)
	}
}

// ─── drop-and-count on failure ────────────────────────────────────────────────

func TestOTLPSink_DropOnServerError(t *testing.T) {
	url, _ := otlpServer(t, 500) // server returns 500
	reg := metrics.NewRegistry()
	s, err := otelgenai.NewOTLPSink(otelgenai.Config{
		Endpoint:  url,
		BatchSize: 1,
		Now:       func() time.Time { return epoch },
		Registry:  reg,
	})
	if err != nil {
		t.Fatalf("NewOTLPSink: %v", err)
	}

	ev := event.Event{
		ID:         "ev-drop",
		Kind:       event.KindInfo,
		Source:     "test",
		Timestamp:  epoch,
		Attributes: map[string]string{event.AttrRunID: "run-drop"},
	}
	// Emit must not return an error (drop-and-count policy).
	if err := s.Emit(ev); err != nil {
		t.Fatalf("Emit returned error (expected nil under drop policy): %v", err)
	}

	// The drop counter should appear in Expose() with value 1.
	exposed := reg.Expose()
	if !strings.Contains(exposed, "otelgenai_drops_total 1") {
		t.Errorf("expected otelgenai_drops_total 1 in metrics output, got:\n%s", exposed)
	}
}

// ─── semconv mapping ─────────────────────────────────────────────────────────

func TestOTLPSink_TokenCountsAsIntValue(t *testing.T) {
	url, drain := otlpServer(t, 200)
	s, err := otelgenai.NewOTLPSink(otelgenai.Config{
		Endpoint:  url,
		BatchSize: 1,
		Now:       func() time.Time { return epoch },
	})
	if err != nil {
		t.Fatalf("NewOTLPSink: %v", err)
	}

	ev := event.Event{
		ID:        "ev-tokens",
		Kind:      event.KindLLMCall,
		Source:    "test",
		Timestamp: epoch,
		Attributes: map[string]string{
			event.AttrRunID:     "run-tok",
			"input_tokens":      "150",
			"output_tokens":     "42",
			"cache_read_tokens": "10",
		},
	}
	if err := s.Emit(ev); err != nil {
		t.Fatalf("Emit: %v", err)
	}

	bodies := drain()
	if len(bodies) == 0 {
		t.Fatal("no OTLP POST received")
	}

	var p testPayload
	if err := json.Unmarshal(bodies[0], &p); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	spans := p.ResourceSpans[0].ScopeSpans[0].Spans
	if len(spans) == 0 {
		t.Fatal("no spans")
	}

	attrMap := map[string]interface{}{}
	for _, a := range spans[0].Attributes {
		if a.Value.IntValue != nil {
			attrMap[a.Key] = *a.Value.IntValue
		}
		if a.Value.StringValue != nil {
			attrMap[a.Key] = *a.Value.StringValue
		}
	}

	checks := map[string]int64{
		"gen_ai.usage.input_tokens":        150,
		"gen_ai.usage.output_tokens":       42,
		"gen_ai.usage.input_tokens_cached": 10,
	}
	for k, want := range checks {
		got, ok := attrMap[k].(int64)
		if !ok {
			t.Errorf("attribute %q missing or not IntValue", k)
			continue
		}
		if got != want {
			t.Errorf("attribute %q = %d, want %d", k, got, want)
		}
	}
}

// ─── extra attributes ─────────────────────────────────────────────────────────

func TestOTLPSink_ExtraAttributes(t *testing.T) {
	url, drain := otlpServer(t, 200)
	s, err := otelgenai.NewOTLPSink(otelgenai.Config{
		Endpoint:  url,
		BatchSize: 1,
		Now:       func() time.Time { return epoch },
		ExtraAttributes: map[string]string{
			"deployment.environment": "prod",
		},
	})
	if err != nil {
		t.Fatalf("NewOTLPSink: %v", err)
	}

	ev := event.Event{
		ID:         "ev-extra",
		Kind:       event.KindInfo,
		Source:     "test",
		Timestamp:  epoch,
		Attributes: map[string]string{event.AttrRunID: "run-extra"},
	}
	if err := s.Emit(ev); err != nil {
		t.Fatalf("Emit: %v", err)
	}

	bodies := drain()
	if len(bodies) == 0 {
		t.Fatal("no OTLP POST received")
	}

	var p testPayload
	if err := json.Unmarshal(bodies[0], &p); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	spans := p.ResourceSpans[0].ScopeSpans[0].Spans
	found := false
	for _, a := range spans[0].Attributes {
		if a.Key == "deployment.environment" && a.Value.StringValue != nil && *a.Value.StringValue == "prod" {
			found = true
		}
	}
	if !found {
		t.Error("deployment.environment extra attribute not found in span")
	}
}

// ─── concurrency ─────────────────────────────────────────────────────────────

func TestOTLPSink_ConcurrentEmit(t *testing.T) {
	url, _ := otlpServer(t, 200)
	s, err := otelgenai.NewOTLPSink(otelgenai.Config{
		Endpoint:  url,
		BatchSize: 10,
		Now:       func() time.Time { return epoch },
	})
	if err != nil {
		t.Fatalf("NewOTLPSink: %v", err)
	}

	const goroutines = 20
	var wg sync.WaitGroup
	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			ev := event.Event{
				ID:         "ev-conc",
				Kind:       event.KindInfo,
				Source:     "test",
				Timestamp:  epoch,
				Attributes: map[string]string{event.AttrRunID: "run-conc"},
			}
			_ = s.Emit(ev)
		}()
	}
	wg.Wait()
	s.Flush() // ensure all pending events are exported
}
