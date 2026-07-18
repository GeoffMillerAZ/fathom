package sink

import (
	"encoding/json"
	"io"
	"sync"

	ev "github.com/geoffmilleraz/agentic-kit/foundations/telemetry/event"
)

// JSONLSink writes each emitted event as a single JSON object on its own
// line (JSON Lines / NDJSON). Field order is stable: encoding/json serialises
// the Event struct fields in declaration order (ID, Kind, Source, Timestamp,
// Attributes — matching the #Event field names in
// foundations/telemetry/event/schema.cue) and map keys in sorted order, so
// identical events always produce byte-identical lines. Timestamps render as
// RFC 3339 (time.Time's standard JSON encoding).
//
// JSONLSink is safe for concurrent use: a sync.Mutex serialises encoding and
// writing, so lines from concurrent Emit calls never interleave. The caller
// retains ownership of the underlying writer (e.g. closing files).
type JSONLSink struct {
	mu  sync.Mutex
	enc *json.Encoder
}

// NewJSONLSink returns a JSONLSink writing one JSON line per event to w.
func NewJSONLSink(w io.Writer) *JSONLSink {
	return &JSONLSink{enc: json.NewEncoder(w)}
}

// Emit writes e as one JSON object followed by a newline. Marshalling and
// write errors are propagated to the caller; on error the line may be
// partially written, so callers that need atomic lines should wrap w in a
// buffering layer they control.
func (s *JSONLSink) Emit(e ev.Event) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.enc.Encode(e)
}
