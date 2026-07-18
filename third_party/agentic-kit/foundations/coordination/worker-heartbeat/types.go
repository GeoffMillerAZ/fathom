// Package workerheartbeat tracks liveness signals from parallel workers.
// A Monitor records the most recent Heartbeat for each worker and exposes
// Stale, which returns workers that have not checked in within a configurable TTL.
// It is a pure tracking primitive: it does NOT spawn workers or manage execution.
// Time injection (no time.Now() in logic) ensures deterministic testing.
//
// Plan-only status (D7): this package ships as a planner primitive. The worker
// executor (kits/worker-pool with agent-harness as the worker unit) is not yet
// built; it will be added when a real fan-out workload subscribes.
//
// Concurrency contract: Monitor is not safe for concurrent use; callers that
// share a Monitor across goroutines must guard access with their own
// synchronization.
package workerheartbeat

import (
	"errors"
	"time"
)

// ErrEmptyWorkerID is returned when a heartbeat is recorded with an empty WorkerID.
var ErrEmptyWorkerID = errors.New("workerheartbeat: WorkerID must not be empty")

// Heartbeat is a single liveness signal from a worker process.
type Heartbeat struct {
	// WorkerID is the stable, unique identifier for the worker.
	WorkerID string

	// At is the wall-clock time when the heartbeat was sent.
	At time.Time
}

// Monitor tracks the most recent Heartbeat for each known worker.
// It is safe for use from a single goroutine; callers that need concurrent
// access should synchronize externally.
type Monitor struct {
	latest map[string]Heartbeat
}

// NewMonitor returns an empty Monitor ready to receive heartbeats.
func NewMonitor() *Monitor {
	return &Monitor{latest: make(map[string]Heartbeat)}
}

// Record stores the heartbeat. If a heartbeat for the same WorkerID already
// exists and the new one is older, the existing record is kept (monotonic).
// Returns ErrEmptyWorkerID if hb.WorkerID is "".
func (m *Monitor) Record(hb Heartbeat) error {
	if hb.WorkerID == "" {
		return ErrEmptyWorkerID
	}
	if existing, ok := m.latest[hb.WorkerID]; ok && !hb.At.After(existing.At) {
		// Keep the newer existing record; ignore stale signals.
		return nil
	}
	m.latest[hb.WorkerID] = hb
	return nil
}

// Stale returns all workers whose most recent heartbeat is older than now-ttl.
// Workers that have never sent a heartbeat are not tracked and will not appear.
// The returned slice is sorted by WorkerID for determinism.
// now and ttl are injected by the caller; no time.Now() is called internally.
func (m *Monitor) Stale(now time.Time, ttl time.Duration) []Heartbeat {
	cutoff := now.Add(-ttl)
	var out []Heartbeat
	for _, hb := range m.sorted() {
		if hb.At.Before(cutoff) {
			out = append(out, hb)
		}
	}
	if out == nil {
		out = []Heartbeat{}
	}
	return out
}

// Workers returns all tracked worker IDs in sorted order.
func (m *Monitor) Workers() []string {
	ids := make([]string, 0, len(m.latest))
	for id := range m.latest {
		ids = append(ids, id)
	}
	sortStrings(ids)
	return ids
}

// Latest returns the most recent Heartbeat for the given workerID and whether
// the worker is tracked.
func (m *Monitor) Latest(workerID string) (Heartbeat, bool) {
	hb, ok := m.latest[workerID]
	return hb, ok
}

// sorted returns all heartbeats sorted by WorkerID for deterministic iteration.
func (m *Monitor) sorted() []Heartbeat {
	ids := make([]string, 0, len(m.latest))
	for id := range m.latest {
		ids = append(ids, id)
	}
	sortStrings(ids)
	out := make([]Heartbeat, 0, len(ids))
	for _, id := range ids {
		out = append(out, m.latest[id])
	}
	return out
}

// sortStrings sorts a slice of strings in place (insertion sort).
func sortStrings(ss []string) {
	for i := 1; i < len(ss); i++ {
		for j := i; j > 0 && ss[j] < ss[j-1]; j-- {
			ss[j], ss[j-1] = ss[j-1], ss[j]
		}
	}
}
