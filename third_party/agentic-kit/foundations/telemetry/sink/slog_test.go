package sink_test

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"

	ev "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/telemetry/event"
	"github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/telemetry/sink"
)

// newJSONLogger returns a slog.Logger writing JSON to buf with the record
// time dropped, so output depends only on the emitted event (deterministic).
func newJSONLogger(buf *bytes.Buffer) *slog.Logger {
	h := slog.NewJSONHandler(buf, &slog.HandlerOptions{
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			if len(groups) == 0 && a.Key == slog.TimeKey {
				return slog.Attr{} // drop non-deterministic record time
			}
			return a
		},
	})
	return slog.New(h)
}

// ---- golden output ------------------------------------------------------------

func TestSlogSink_GoldenJSONWithTraceAttrs(t *testing.T) {
	e := ev.Event{
		ID:        "llmclient/ladder/0001",
		Kind:      ev.KindToolCall,
		Source:    "llmclient/ladder",
		Timestamp: fixedTime,
		Attributes: map[string]string{
			"tool_name": "search",
		},
	}
	ev.Apply(ev.TraceContext{
		RunID:        "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		SpanID:       "bbbbbbbbbbbbbbbb",
		ParentSpanID: "cccccccccccccccc",
	}, &e)

	var buf bytes.Buffer
	s := sink.NewSlogSink(newJSONLogger(&buf))
	if err := s.Emit(e); err != nil {
		t.Fatalf("Emit returned unexpected error: %v", err)
	}

	want := `{"level":"INFO","msg":"telemetry_event",` +
		`"event_id":"llmclient/ladder/0001","kind":"tool_call","source":"llmclient/ladder",` +
		`"timestamp":"2024-01-01T12:00:00Z",` +
		`"attributes":{"tool_name":"search","trace.parent_span_id":"cccccccccccccccc",` +
		`"trace.run_id":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","trace.span_id":"bbbbbbbbbbbbbbbb"}}` + "\n"
	if got := buf.String(); got != want {
		t.Errorf("golden mismatch:\n got: %s\nwant: %s", got, want)
	}
}

func TestSlogSink_NoAttributesOmitsGroup(t *testing.T) {
	var buf bytes.Buffer
	s := sink.NewSlogSink(newJSONLogger(&buf))
	if err := s.Emit(makeEvent(ev.KindInfo, "src")); err != nil {
		t.Fatalf("Emit returned unexpected error: %v", err)
	}

	want := `{"level":"INFO","msg":"telemetry_event",` +
		`"event_id":"test-src","kind":"info","source":"src",` +
		`"timestamp":"2024-01-01T12:00:00Z"}` + "\n"
	if got := buf.String(); got != want {
		t.Errorf("golden mismatch:\n got: %s\nwant: %s", got, want)
	}
}

// ---- level mapping ---------------------------------------------------------------

func TestSlogSink_LevelMapping(t *testing.T) {
	tests := []struct {
		kind      ev.Kind
		wantLevel string
	}{
		{kind: ev.KindToolCall, wantLevel: `"level":"INFO"`},
		{kind: ev.KindLLMCall, wantLevel: `"level":"INFO"`},
		{kind: ev.KindInfo, wantLevel: `"level":"INFO"`},
		{kind: ev.KindError, wantLevel: `"level":"ERROR"`},
	}
	for _, tt := range tests {
		t.Run(string(tt.kind), func(t *testing.T) {
			var buf bytes.Buffer
			s := sink.NewSlogSink(newJSONLogger(&buf))
			if err := s.Emit(makeEvent(tt.kind, "src")); err != nil {
				t.Fatalf("Emit returned unexpected error: %v", err)
			}
			if !strings.Contains(buf.String(), tt.wantLevel) {
				t.Errorf("kind %q: expected %s in output; got: %s", tt.kind, tt.wantLevel, buf.String())
			}
		})
	}
}

// ---- attribute key ordering is deterministic ---------------------------------------

func TestSlogSink_AttributeKeysSorted(t *testing.T) {
	e := makeEvent(ev.KindInfo, "src")
	e.Attributes = map[string]string{
		"zebra": "z",
		"alpha": "a",
		"mango": "m",
	}

	var buf bytes.Buffer
	s := sink.NewSlogSink(newJSONLogger(&buf))
	if err := s.Emit(e); err != nil {
		t.Fatalf("Emit returned unexpected error: %v", err)
	}

	got := buf.String()
	wantOrder := `"attributes":{"alpha":"a","mango":"m","zebra":"z"}`
	if !strings.Contains(got, wantOrder) {
		t.Errorf("attribute keys must render sorted:\n got: %s\nwant fragment: %s", got, wantOrder)
	}
	// Determinism: a second identical emit produces an identical line.
	var buf2 bytes.Buffer
	if err := sink.NewSlogSink(newJSONLogger(&buf2)).Emit(e); err != nil {
		t.Fatalf("second Emit failed: %v", err)
	}
	if buf2.String() != got {
		t.Errorf("output not deterministic:\nfirst:  %s\nsecond: %s", got, buf2.String())
	}
}

// ---- nil logger falls back to slog.Default ----------------------------------------

func TestSlogSink_NilLoggerUsesDefault(t *testing.T) {
	var buf bytes.Buffer
	old := slog.Default()
	slog.SetDefault(newJSONLogger(&buf))
	defer slog.SetDefault(old)

	s := sink.NewSlogSink(nil)
	if err := s.Emit(makeEvent(ev.KindInfo, "src")); err != nil {
		t.Fatalf("Emit returned unexpected error: %v", err)
	}
	if !strings.Contains(buf.String(), `"event_id":"test-src"`) {
		t.Errorf("nil logger must fall back to slog.Default(); got: %s", buf.String())
	}
}

// ---- Emit never errors -------------------------------------------------------------

func TestSlogSink_EmitAlwaysNil(t *testing.T) {
	var buf bytes.Buffer
	s := sink.NewSlogSink(newJSONLogger(&buf))
	for _, k := range []ev.Kind{ev.KindToolCall, ev.KindLLMCall, ev.KindError, ev.KindInfo} {
		if err := s.Emit(makeEvent(k, "src")); err != nil {
			t.Errorf("Emit(%q) must return nil; got %v", k, err)
		}
	}
}

// ---- Sink interface satisfaction ----------------------------------------------------

func TestSlogSink_SatisfiesSink(t *testing.T) {
	var _ sink.Sink = sink.NewSlogSink(nil)
}
