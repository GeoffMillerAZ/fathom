package fusion

import (
	"context"
	"fmt"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/store"
)

// ingestOwnerProbe is the process-wide liveness check behind
// refuseConcurrentIngest; nil turns the check off.
var ingestOwnerProbe atomic.Pointer[func(pid int, started time.Time) bool]

// InstallIngestOwnerProbe installs the check an ingest asks before it
// starts (GDEV-02): whether pid, recorded as starting an ingest of the
// service at started, is that same process and still running. It must be
// pid-reuse safe: a bare kill(pid, 0) reads pid 1 alive, and a pid a later
// process took as the row's owner. nil turns the check off, which is the
// default: a library caller ingests as before. It returns the undo, which
// restores the previous probe unless a later install replaced it, and is
// safe to call twice (InstallProcessToolchain's pattern).
func InstallIngestOwnerProbe(fn func(pid int, started time.Time) bool) (undo func()) {
	var next *func(int, time.Time) bool
	if fn != nil {
		next = &fn
	}
	prev := ingestOwnerProbe.Swap(next)
	var once sync.Once
	return func() {
		once.Do(func() { ingestOwnerProbe.CompareAndSwap(next, prev) })
	}
}

// ConcurrentIngestError refuses an ingest whose service has an in_progress
// ingest_state row owned by another process the installed probe reports
// alive. Nothing was read or written. The row proves that the owner began
// an ingest that has not committed, not that it is still running one: an
// ingest that stopped after a write leaves in_progress under its owner's
// pid (RT-109) until that owner, or the next start, ingests again.
type ConcurrentIngestError struct {
	Service   string
	PID       int
	StartedAt time.Time
}

func (e *ConcurrentIngestError) Error() string {
	return fmt.Sprintf("fusion: ingest of %q refused: pid %d (alive) began an ingest of it at %s that has not committed",
		e.Service, e.PID, e.StartedAt.UTC().Format(time.RFC3339))
}

// refuseConcurrentIngest is the check an ingest runs before it writes its
// own in_progress row: a live foreign owner of the service's row refuses it
// by name, so two ingests of one service never run side by side in one
// store (ADR-047 bounds one). This process's own row, a dead owner, a
// complete row, an unreadable row and no installed probe all pass. Two
// processes that start in the same instant can both pass before either
// writes its row: closing that needs a conditional store write.
func (p *Pipeline) refuseConcurrentIngest(ctx context.Context, svc core.ServiceRef) error {
	probe := ingestOwnerProbe.Load()
	if probe == nil {
		return nil
	}
	iss, ok := p.st.(store.IngestStateStore)
	if !ok {
		return nil
	}
	row, found, err := iss.GetIngestState(ctx, svc.ID)
	if err != nil || !found || row.State != store.IngestInProgress || row.PID <= 0 || row.PID == os.Getpid() {
		return nil
	}
	started := time.Unix(0, row.StartedAt)
	if !(*probe)(row.PID, started) {
		return nil
	}
	return &ConcurrentIngestError{Service: svc.ID, PID: row.PID, StartedAt: started.UTC()}
}
