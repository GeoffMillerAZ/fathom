// Package sink defines the behavioral contracts and implementations for
// telemetry event sinks. A Sink is anything that can accept a telemetry Event;
// the package provides several composable implementations suitable for
// production fan-out and in-process testing.
//
// There is no schema.cue in this package because the contracts here are
// behavioral (interfaces and algorithms), not structural data shapes. CUE is
// applied only where data shape is the primary concern.
//
// Boundary with foundations/persistence/jsonl-log: JSONLSink here is
// fire-and-forget event emission — it writes one JSON line per telemetry
// event for downstream collectors and offers no read-back contract.
// foundations/persistence/jsonl-log is the durable append-log seam: writer
// plus streaming reader with surfaced torn-tail recovery, for state you
// rebuild from on restart. If the lines are state rather than telemetry,
// use jsonl-log.
//
// Concurrency: all Sink implementations in this package are safe for concurrent use.
package sink

import (
	"errors"
	"sync"

	ev "github.com/geoffmilleraz/agentic-kit/foundations/telemetry/event"
)

// Sink is the single emission interface. Every telemetry backend — stdout,
// Kafka, in-memory test double, etc. — implements this interface.
type Sink interface {
	// Emit forwards the event to the backend. Implementations must be safe
	// for concurrent calls. Emit may return an error if the event cannot be
	// accepted (e.g., a buffer is full or the backend is unavailable).
	Emit(ev.Event) error
}

// MultiSink fans a single event out to a slice of Sinks. All Sinks are called
// even when one returns an error; errors are collected and joined.
// MultiSink is safe for concurrent use after construction.
type MultiSink struct {
	sinks []Sink
}

// NewMultiSink returns a MultiSink that forwards every event to each of the
// provided sinks in order. Nil entries in sinks are ignored.
func NewMultiSink(sinks ...Sink) *MultiSink {
	filtered := make([]Sink, 0, len(sinks))
	for _, s := range sinks {
		if s != nil {
			filtered = append(filtered, s)
		}
	}
	return &MultiSink{sinks: filtered}
}

// Emit calls Emit on each contained Sink. All Sinks are always called. If one
// or more return an error, the errors are joined and returned together.
func (m *MultiSink) Emit(e ev.Event) error {
	var errs []error
	for _, s := range m.sinks {
		if err := s.Emit(e); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

// MemorySink records every emitted event in an in-process slice. It is
// intended for tests — it never drops events and never returns an error.
// MemorySink is safe for concurrent use.
type MemorySink struct {
	mu     sync.Mutex
	events []ev.Event
}

// Emit appends the event to the in-memory record. It always returns nil.
func (m *MemorySink) Emit(e ev.Event) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.events = append(m.events, e)
	return nil
}

// Events returns a copy of all recorded events in emission order.
// The returned slice is independent of the MemorySink's internal state —
// mutating it does not affect the sink.
func (m *MemorySink) Events() []ev.Event {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]ev.Event, len(m.events))
	copy(out, m.events)
	return out
}

// Len returns the number of events recorded so far.
func (m *MemorySink) Len() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.events)
}

// Reset clears all recorded events. Useful for table-driven tests that share
// a single MemorySink across sub-tests.
func (m *MemorySink) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.events = m.events[:0]
}

// FilterSink wraps a downstream Sink and only forwards events for which the
// predicate returns true. Events that do not match are silently dropped (no
// error). FilterSink is safe for concurrent use.
type FilterSink struct {
	inner     Sink
	predicate func(ev.Event) bool
}

// NewFilterSink returns a FilterSink wrapping inner that forwards only events
// for which pred returns true. If pred is nil, all events are forwarded.
func NewFilterSink(inner Sink, pred func(ev.Event) bool) *FilterSink {
	if pred == nil {
		pred = func(ev.Event) bool { return true }
	}
	return &FilterSink{inner: inner, predicate: pred}
}

// Emit calls pred(e) and, if true, forwards the event to the wrapped Sink.
// Returns the wrapped Sink's error on forward, or nil if the event is filtered
// out.
func (f *FilterSink) Emit(e ev.Event) error {
	if !f.predicate(e) {
		return nil
	}
	return f.inner.Emit(e)
}
