package tenancy

// inflight.go — the in-flight registry the ladder acts on (design §2.1;
// RT-172, RT-173). Heavy work registers here with its cancel: the refuse
// rung cancels every tree and fleet entry, youngest first, as
// abandoned-by-governor; the exit rung halts an in-flight ingest first and
// quarantines its service. The admission gate's tickets (RT-53) register
// here too; until it lands, serve-http registers each /rpc call by its
// tool's class and pkg/client registers every ingest.

import (
	"context"
	"errors"
	"sort"
	"sync"
	"time"
)

// ErrAbandonedByGovernor is the cancel cause of work the refuse rung
// cancelled (outcome: abandoned-by-governor).
var ErrAbandonedByGovernor = errors.New("abandoned-by-governor: fathom is over its memory budget (refuse) — fathom resources")

// ErrHaltedOverBudget is the cancel cause of an ingest the exit rung halted.
var ErrHaltedOverBudget = errors.New("ingest halted: fathom reached its exit mark; the service is quarantined until an explicit ingest")

// InFlightEntry is one piece of registered work.
type InFlightEntry struct {
	ID      uint64
	Class   Class
	Tool    string
	Service string
	Origin  string
	Started time.Time
}

// InFlight is a registry of cancellable work.
type InFlight struct {
	mu      sync.Mutex
	next    uint64
	entries map[uint64]*inflightEntry
	now     func() time.Time
	// last is when work last began or ended: the idle clock (§5: "30 min
	// without tickets") runs from it, so a call that begins and ends
	// between two samples still counts.
	last time.Time
}

type inflightEntry struct {
	InFlightEntry
	cancel context.CancelCauseFunc
}

// NewInFlight returns an empty registry; now is its clock (nil: time.Now).
func NewInFlight(now func() time.Time) *InFlight {
	if now == nil {
		now = time.Now
	}
	return &InFlight{entries: map[uint64]*inflightEntry{}, now: now}
}

// DefaultInFlight is the process's registry.
var DefaultInFlight = NewInFlight(nil)

// Begin registers work of class c under a cancellable child of ctx (marked
// with c as its producer class) and returns it with the release func.
func (r *InFlight) Begin(ctx context.Context, c Class, tool, service, origin string) (context.Context, func()) {
	cctx, cancel := context.WithCancelCause(WithProducerClass(ctx, c))
	r.mu.Lock()
	r.next++
	id := r.next
	r.last = r.now()
	r.entries[id] = &inflightEntry{
		InFlightEntry: InFlightEntry{ID: id, Class: c, Tool: tool, Service: service, Origin: origin, Started: r.now()},
		cancel:        cancel,
	}
	r.mu.Unlock()
	var once sync.Once
	return cctx, func() {
		once.Do(func() {
			r.mu.Lock()
			delete(r.entries, id)
			r.last = r.now()
			r.mu.Unlock()
			cancel(nil)
		})
	}
}

// Len is how much work is registered now (no allocation).
func (r *InFlight) Len() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.entries)
}

// LastActive is when work last began or ended (zero before any).
func (r *InFlight) LastActive() time.Time {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.last
}

// Snapshot lists the registered work, oldest first (ties by id).
func (r *InFlight) Snapshot() []InFlightEntry {
	r.mu.Lock()
	out := make([]InFlightEntry, 0, len(r.entries))
	for _, e := range r.entries {
		out = append(out, e.InFlightEntry)
	}
	r.mu.Unlock()
	sort.Slice(out, func(i, j int) bool {
		if !out[i].Started.Equal(out[j].Started) {
			return out[i].Started.Before(out[j].Started)
		}
		return out[i].ID < out[j].ID
	})
	return out
}

// Cancel cancels every entry whose class is in classes, youngest first,
// with cause, and returns them in the order cancelled. Entries stay
// registered until their own release.
func (r *InFlight) Cancel(cause error, classes ...Class) []InFlightEntry {
	want := map[Class]bool{}
	for _, c := range classes {
		want[c] = true
	}
	r.mu.Lock()
	var hit []*inflightEntry
	for _, e := range r.entries {
		if want[e.Class] {
			hit = append(hit, e)
		}
	}
	r.mu.Unlock()
	sort.Slice(hit, func(i, j int) bool {
		if !hit[i].Started.Equal(hit[j].Started) {
			return hit[i].Started.After(hit[j].Started)
		}
		return hit[i].ID > hit[j].ID
	})
	out := make([]InFlightEntry, 0, len(hit))
	for _, e := range hit {
		e.cancel(cause)
		out = append(out, e.InFlightEntry)
	}
	return out
}

// OriginPipeline is the origin of the pipeline's own ingest registration
// (BeginIngest): every ingest, explicit or auto-refresh, registers under
// it; an explicit ingest also has its /rpc ticket (tool ingest or
// onboard_service, the caller's origin).
const OriginPipeline = "pipeline"

// BeginIngest registers an ingest of service in DefaultInFlight: its ctx
// is ingest-class for the producer governor, and the exit rung can halt it.
func BeginIngest(ctx context.Context, service string) (context.Context, func()) {
	return DefaultInFlight.Begin(ctx, ClassIngest, "ingest", service, OriginPipeline)
}

// ExplicitIngestInFlight reports whether an explicit ingest — an /rpc
// `ingest` or `onboard_service` call, not auto-refresh's pipeline ingest —
// is registered now (RT-162: disk writes are judged outside explicit
// ingest). No allocation.
func (r *InFlight) ExplicitIngestInFlight() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, e := range r.entries {
		if e.Class == ClassIngest && e.Origin != OriginPipeline && (e.Tool == "ingest" || e.Tool == "onboard_service") {
			return true
		}
	}
	return false
}
