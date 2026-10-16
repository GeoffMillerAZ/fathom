package sink_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"

	ev "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/telemetry/event"
	"github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/telemetry/sink"
)

// errWriter fails every Write with a fixed error.
type errWriter struct{ err error }

func (w *errWriter) Write(_ []byte) (int, error) { return 0, w.err }

// ---- golden line output ------------------------------------------------------

func TestJSONLSink_GoldenLineWithTraceAttrs(t *testing.T) {
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
	s := sink.NewJSONLSink(&buf)
	if err := s.Emit(e); err != nil {
		t.Fatalf("Emit returned unexpected error: %v", err)
	}

	want := `{"ID":"llmclient/ladder/0001","Kind":"tool_call","Source":"llmclient/ladder",` +
		`"Timestamp":"2024-01-01T12:00:00Z",` +
		`"Attributes":{"tool_name":"search","trace.parent_span_id":"cccccccccccccccc",` +
		`"trace.run_id":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","trace.span_id":"bbbbbbbbbbbbbbbb"}}` + "\n"
	if got := buf.String(); got != want {
		t.Errorf("golden mismatch:\n got: %s\nwant: %s", got, want)
	}
}

func TestJSONLSink_GoldenLineNoAttributes(t *testing.T) {
	var buf bytes.Buffer
	s := sink.NewJSONLSink(&buf)
	if err := s.Emit(makeEvent(ev.KindInfo, "src")); err != nil {
		t.Fatalf("Emit returned unexpected error: %v", err)
	}

	want := `{"ID":"test-src","Kind":"info","Source":"src",` +
		`"Timestamp":"2024-01-01T12:00:00Z","Attributes":null}` + "\n"
	if got := buf.String(); got != want {
		t.Errorf("golden mismatch:\n got: %s\nwant: %s", got, want)
	}
}

// ---- one line per event, all parseable -----------------------------------------

func TestJSONLSink_OneLinePerEvent(t *testing.T) {
	var buf bytes.Buffer
	s := sink.NewJSONLSink(&buf)

	const n = 5
	for i := 0; i < n; i++ {
		e := makeEvent(ev.KindInfo, fmt.Sprintf("src-%d", i))
		if err := s.Emit(e); err != nil {
			t.Fatalf("Emit %d failed: %v", i, err)
		}
	}

	lines := strings.Split(strings.TrimSuffix(buf.String(), "\n"), "\n")
	if len(lines) != n {
		t.Fatalf("expected %d lines, got %d", n, len(lines))
	}
	for i, line := range lines {
		var decoded ev.Event
		if err := json.Unmarshal([]byte(line), &decoded); err != nil {
			t.Errorf("line %d is not valid JSON: %v\nline: %s", i, err, line)
		}
		if want := fmt.Sprintf("src-%d", i); decoded.Source != want {
			t.Errorf("line %d: Source = %q, want %q (order must be preserved)", i, decoded.Source, want)
		}
	}
}

// ---- round trip ---------------------------------------------------------------

func TestJSONLSink_RoundTrip(t *testing.T) {
	orig := makeEvent(ev.KindError, "security/detector")
	orig.Attributes = map[string]string{"severity": "high"}

	var buf bytes.Buffer
	if err := sink.NewJSONLSink(&buf).Emit(orig); err != nil {
		t.Fatalf("Emit failed: %v", err)
	}

	var decoded ev.Event
	if err := json.Unmarshal(buf.Bytes(), &decoded); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}
	if decoded.ID != orig.ID || decoded.Kind != orig.Kind || decoded.Source != orig.Source {
		t.Errorf("round trip mismatch: got %+v, want %+v", decoded, orig)
	}
	if !decoded.Timestamp.Equal(orig.Timestamp) {
		t.Errorf("Timestamp mismatch: got %v, want %v", decoded.Timestamp, orig.Timestamp)
	}
	if decoded.Attributes["severity"] != "high" {
		t.Errorf("Attributes mismatch: got %v", decoded.Attributes)
	}
}

// ---- error propagation -----------------------------------------------------------

func TestJSONLSink_PropagatesWriteError(t *testing.T) {
	wantErr := errors.New("disk full")
	s := sink.NewJSONLSink(&errWriter{err: wantErr})

	err := s.Emit(makeEvent(ev.KindInfo, "src"))
	if err == nil {
		t.Fatal("Emit must propagate the writer's error")
	}
	if !errors.Is(err, wantErr) {
		t.Errorf("expected wrapped %v; got %v", wantErr, err)
	}
}

// ---- concurrency: lines never interleave ------------------------------------------

func TestJSONLSink_ConcurrentEmitProducesWholeLines(t *testing.T) {
	var buf bytes.Buffer
	s := sink.NewJSONLSink(&buf)

	const n = 100
	var wg sync.WaitGroup
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func(i int) {
			defer wg.Done()
			e := makeEvent(ev.KindInfo, fmt.Sprintf("race-%03d", i))
			if err := s.Emit(e); err != nil {
				t.Errorf("Emit %d failed: %v", i, err)
			}
		}(i)
	}
	wg.Wait()

	lines := strings.Split(strings.TrimSuffix(buf.String(), "\n"), "\n")
	if len(lines) != n {
		t.Fatalf("expected %d lines, got %d", n, len(lines))
	}
	for i, line := range lines {
		var decoded ev.Event
		if err := json.Unmarshal([]byte(line), &decoded); err != nil {
			t.Errorf("line %d corrupted by interleaving: %v\nline: %s", i, err, line)
		}
	}
}

// ---- Sink interface satisfaction ----------------------------------------------

func TestJSONLSink_SatisfiesSink(t *testing.T) {
	var _ sink.Sink = sink.NewJSONLSink(&bytes.Buffer{})
}
