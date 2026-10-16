package sink_test

import (
	"fmt"
	"sync"
	"testing"
	"time"

	ev "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/telemetry/event"
	"github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/telemetry/sink"
)

// fixedTime is a stable reference instant so tests are deterministic.
var fixedTime = time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)

// makeEvent returns a minimal valid event.
func makeEvent(kind ev.Kind, source string) ev.Event {
	return ev.Event{
		ID:        "test-" + source,
		Kind:      kind,
		Source:    source,
		Timestamp: fixedTime,
	}
}

// errSink is a Sink that always returns the given error (for testing MultiSink
// error collection).
type errSink struct{ err error }

func (e *errSink) Emit(_ ev.Event) error { return e.err }

// ---- MemorySink ------------------------------------------------------------

func TestMemorySink_RecordsEvents(t *testing.T) {
	var m sink.MemorySink

	e1 := makeEvent(ev.KindInfo, "src-a")
	e2 := makeEvent(ev.KindToolCall, "src-b")

	if err := m.Emit(e1); err != nil {
		t.Fatalf("Emit should never fail on MemorySink; got %v", err)
	}
	if err := m.Emit(e2); err != nil {
		t.Fatalf("Emit should never fail on MemorySink; got %v", err)
	}

	events := m.Events()
	if len(events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(events))
	}
	if events[0].Source != "src-a" || events[1].Source != "src-b" {
		t.Errorf("unexpected event order: %v", []string{events[0].Source, events[1].Source})
	}
}

func TestMemorySink_EventsReturnsCopy(t *testing.T) {
	var m sink.MemorySink
	m.Emit(makeEvent(ev.KindInfo, "src"))

	snap := m.Events()
	origID := snap[0].ID
	snap[0].ID = "mutated"

	// Internal state must be unchanged.
	snap2 := m.Events()
	if snap2[0].ID != origID {
		t.Errorf("mutating snapshot changed internal state; got %q, want %q", snap2[0].ID, origID)
	}
}

func TestMemorySink_Len(t *testing.T) {
	var m sink.MemorySink
	if m.Len() != 0 {
		t.Fatalf("Len should be 0 initially; got %d", m.Len())
	}
	m.Emit(makeEvent(ev.KindError, "src"))
	if m.Len() != 1 {
		t.Errorf("Len should be 1 after one Emit; got %d", m.Len())
	}
}

func TestMemorySink_Reset(t *testing.T) {
	var m sink.MemorySink
	m.Emit(makeEvent(ev.KindInfo, "src"))
	m.Emit(makeEvent(ev.KindInfo, "src"))
	m.Reset()
	if m.Len() != 0 {
		t.Errorf("after Reset, Len should be 0; got %d", m.Len())
	}
	if got := m.Events(); len(got) != 0 {
		t.Errorf("after Reset, Events should be empty; got %v", got)
	}
}

func TestMemorySink_ConcurrentEmit(t *testing.T) {
	var m sink.MemorySink
	const n = 100
	var wg sync.WaitGroup
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func(i int) {
			defer wg.Done()
			m.Emit(ev.Event{
				ID:        fmt.Sprintf("id-%d", i),
				Kind:      ev.KindInfo,
				Source:    "race-test",
				Timestamp: fixedTime,
			})
		}(i)
	}
	wg.Wait()
	if m.Len() != n {
		t.Errorf("expected %d events after concurrent emit; got %d", n, m.Len())
	}
}

// ---- MultiSink -------------------------------------------------------------

func TestMultiSink_ForwardsToAll(t *testing.T) {
	var a, b sink.MemorySink
	ms := sink.NewMultiSink(&a, &b)

	e := makeEvent(ev.KindLLMCall, "src")
	if err := ms.Emit(e); err != nil {
		t.Fatalf("MultiSink.Emit returned unexpected error: %v", err)
	}

	if a.Len() != 1 {
		t.Errorf("sink a: expected 1 event, got %d", a.Len())
	}
	if b.Len() != 1 {
		t.Errorf("sink b: expected 1 event, got %d", b.Len())
	}
}

func TestMultiSink_CollectsErrors(t *testing.T) {
	var mem sink.MemorySink
	bad := &errSink{err: fmt.Errorf("backend down")}
	ms := sink.NewMultiSink(&mem, bad)

	err := ms.Emit(makeEvent(ev.KindInfo, "src"))
	if err == nil {
		t.Fatal("expected error from MultiSink when a child fails")
	}
	// The non-failing sink must still have received the event.
	if mem.Len() != 1 {
		t.Errorf("good sink should still receive event; got %d events", mem.Len())
	}
}

func TestMultiSink_AllChildrenCalledOnError(t *testing.T) {
	bad1 := &errSink{err: fmt.Errorf("err1")}
	bad2 := &errSink{err: fmt.Errorf("err2")}
	var good sink.MemorySink

	ms := sink.NewMultiSink(bad1, &good, bad2)
	err := ms.Emit(makeEvent(ev.KindError, "src"))
	if err == nil {
		t.Fatal("expected joined error from MultiSink")
	}
	// The good sink in the middle must still receive the event.
	if good.Len() != 1 {
		t.Errorf("middle good sink should receive event; got %d", good.Len())
	}
}

func TestMultiSink_NilSinksIgnored(t *testing.T) {
	var mem sink.MemorySink
	ms := sink.NewMultiSink(nil, &mem, nil)
	ms.Emit(makeEvent(ev.KindInfo, "src"))
	if mem.Len() != 1 {
		t.Errorf("expected 1 event; got %d", mem.Len())
	}
}

func TestMultiSink_EmptyIsNoop(t *testing.T) {
	ms := sink.NewMultiSink()
	if err := ms.Emit(makeEvent(ev.KindInfo, "src")); err != nil {
		t.Errorf("empty MultiSink should return nil; got %v", err)
	}
}

// ---- FilterSink ------------------------------------------------------------

func TestFilterSink_ForwardsMatchingEvents(t *testing.T) {
	var mem sink.MemorySink
	fs := sink.NewFilterSink(&mem, func(e ev.Event) bool {
		return e.Kind == ev.KindError
	})

	fs.Emit(makeEvent(ev.KindInfo, "src"))     // filtered out
	fs.Emit(makeEvent(ev.KindError, "src"))    // forwarded
	fs.Emit(makeEvent(ev.KindToolCall, "src")) // filtered out
	fs.Emit(makeEvent(ev.KindError, "src"))    // forwarded

	if mem.Len() != 2 {
		t.Errorf("expected 2 error events to pass through; got %d", mem.Len())
	}
	for _, e := range mem.Events() {
		if e.Kind != ev.KindError {
			t.Errorf("non-error event leaked through filter: %v", e.Kind)
		}
	}
}

func TestFilterSink_DropDoesNotReturnError(t *testing.T) {
	var mem sink.MemorySink
	fs := sink.NewFilterSink(&mem, func(e ev.Event) bool { return false })

	if err := fs.Emit(makeEvent(ev.KindInfo, "src")); err != nil {
		t.Errorf("dropping an event must return nil; got %v", err)
	}
	if mem.Len() != 0 {
		t.Errorf("no events should reach inner sink; got %d", mem.Len())
	}
}

func TestFilterSink_NilPredicateForwardsAll(t *testing.T) {
	var mem sink.MemorySink
	fs := sink.NewFilterSink(&mem, nil)

	fs.Emit(makeEvent(ev.KindInfo, "src"))
	fs.Emit(makeEvent(ev.KindError, "src"))

	if mem.Len() != 2 {
		t.Errorf("nil predicate should forward all events; got %d", mem.Len())
	}
}

func TestFilterSink_PropagatesInnerError(t *testing.T) {
	bad := &errSink{err: fmt.Errorf("inner failed")}
	fs := sink.NewFilterSink(bad, func(ev.Event) bool { return true })

	err := fs.Emit(makeEvent(ev.KindInfo, "src"))
	if err == nil {
		t.Error("FilterSink must propagate inner sink error")
	}
}

// ---- Sink interface satisfaction -------------------------------------------

// TestSinkInterfaceSatisfied ensures each concrete type satisfies Sink at
// compile time without needing an explicit var declaration in types.go.
func TestSinkInterfaceSatisfied(t *testing.T) {
	var _ sink.Sink = &sink.MemorySink{}
	var _ sink.Sink = sink.NewMultiSink()
	var _ sink.Sink = sink.NewFilterSink(&sink.MemorySink{}, nil)
}
