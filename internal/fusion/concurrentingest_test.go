package fusion_test

import (
	"context"
	"errors"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/fusion"
	"github.com/geoffmilleraz/fathom/internal/store"
)

// TestIngestRefusesLiveConcurrentIngestByName (GDEV-02): an ingest whose
// service has an in_progress row owned by another process the installed
// probe reports alive is refused by name (service, owner pid, start) before
// Discover and before any write, and the row is left as it was. A dead
// owner, this process's own row, a complete row, and no installed probe
// all ingest as before.
func TestIngestRefusesLiveConcurrentIngestByName(t *testing.T) {
	ctx := context.Background()
	svc := core.ServiceRef{ID: "svc", Path: "/repo"}
	started := time.Date(2026, 10, 26, 12, 0, 0, 0, time.UTC)
	foreign := os.Getpid() + 1
	live := store.IngestState{ServiceID: "svc", State: store.IngestInProgress, PID: foreign, StartedAt: started.UnixNano(), TargetCommit: "t", Path: "/repo"}
	var asked []int
	probe := func(pid int, at time.Time) bool {
		asked = append(asked, pid)
		return pid == foreign && at.Equal(started)
	}
	t.Cleanup(fusion.InstallIngestOwnerProbe(probe))

	t.Run("live foreign owner is refused", func(t *testing.T) {
		mem := store.NewMemoryStore()
		_ = mem.SetIngestState(ctx, live)
		heads := 0
		p := fusion.New(discoverFailConn{}, mem, fusion.Config{Concurrency: 1})
		p.WithGitHead(func(string) string { heads++; return "new" })
		p.WithGitLog(func(string) string { return "" })
		_, err := p.Ingest(ctx, svc)
		var cie *fusion.ConcurrentIngestError
		if !errors.As(err, &cie) {
			t.Fatalf("want a *ConcurrentIngestError before Discover, got %v", err)
		}
		if cie.Service != "svc" || cie.PID != foreign || !cie.StartedAt.Equal(started) {
			t.Fatalf("the refusal names %+v, want svc, pid %d, %s", cie, foreign, started)
		}
		if msg := err.Error(); !strings.Contains(msg, `"svc"`) || !strings.Contains(msg, strconv.Itoa(foreign)) || !strings.Contains(msg, "2026-10-26T12:00:00Z") {
			t.Fatalf("message must name the service, pid and start: %q", msg)
		}
		if heads != 0 {
			t.Fatal("a refused ingest read the git head: the check must come before beginIngestState")
		}
		if got, found, _ := mem.GetIngestState(ctx, "svc"); !found || got != live {
			t.Fatalf("ingest_state = %+v found=%v, want the live owner's row untouched", got, found)
		}
	})

	conn := memConn{files: map[string]string{"a.go": "package a\n\nfunc A() {}\n", "b.go": "package a\n\nfunc B() {}\n"}}
	passes := []struct {
		name  string
		row   *store.IngestState
		probe bool
	}{
		{"dead owner", &store.IngestState{ServiceID: "svc", State: store.IngestInProgress, PID: foreign + 1, StartedAt: started.UnixNano(), Path: "/repo"}, true},
		{"own pid", &store.IngestState{ServiceID: "svc", State: store.IngestInProgress, PID: os.Getpid(), StartedAt: started.UnixNano(), Path: "/repo"}, true},
		{"complete row", &store.IngestState{ServiceID: "svc", State: store.IngestComplete, PID: foreign, StartedAt: started.UnixNano(), Path: "/repo"}, true},
		{"no probe installed", &live, false},
	}
	for _, tc := range passes {
		t.Run(tc.name+" ingests", func(t *testing.T) {
			if !tc.probe {
				undo := fusion.InstallIngestOwnerProbe(nil)
				defer undo()
			}
			asked = nil
			mem := store.NewMemoryStore()
			_ = mem.SetIngestState(ctx, *tc.row)
			p := fusion.New(conn, mem, fusion.Config{Concurrency: 1})
			p.WithGitHead(func(string) string { return "new" })
			p.WithGitLog(func(string) string { return "" })
			if _, err := p.Ingest(ctx, svc); err != nil {
				t.Fatalf("ingest: %v", err)
			}
			got, found, _ := mem.GetIngestState(ctx, "svc")
			if !found || got.State != store.IngestComplete || got.PID != os.Getpid() {
				t.Fatalf("ingest_state = %+v, want complete under this pid", got)
			}
			for _, pid := range asked {
				if pid == os.Getpid() {
					t.Fatal("the probe was asked about this process's own row")
				}
			}
		})
	}
}

// ctxStateStore is a memory store whose ingest-state read fails once the
// context is cancelled, as the SQLite store's does.
type ctxStateStore struct{ *store.MemoryStore }

func (s ctxStateStore) GetIngestState(ctx context.Context, id string) (store.IngestState, bool, error) {
	if err := ctx.Err(); err != nil {
		return store.IngestState{}, false, err
	}
	return s.MemoryStore.GetIngestState(ctx, id)
}

// TestIngestCancelledBeforeCheckLeavesLiveOwnerRow (GDEV-02, review F2): a
// run signalled before the concurrent-ingest check stops there. A read that
// failed on the cancelled context must not count as "no owner", since
// beginIngestState writes under WithoutCancel and would overwrite a live
// owner's in_progress row.
func TestIngestCancelledBeforeCheckLeavesLiveOwnerRow(t *testing.T) {
	svc := core.ServiceRef{ID: "svc", Path: "/repo"}
	started := time.Date(2026, 10, 26, 12, 0, 0, 0, time.UTC)
	foreign := os.Getpid() + 1
	live := store.IngestState{ServiceID: "svc", State: store.IngestInProgress, PID: foreign, StartedAt: started.UnixNano(), TargetCommit: "t", Path: "/repo"}
	t.Cleanup(fusion.InstallIngestOwnerProbe(func(pid int, at time.Time) bool { return pid == foreign && at.Equal(started) }))

	mem := ctxStateStore{store.NewMemoryStore()}
	_ = mem.SetIngestState(context.Background(), live)
	p := fusion.New(discoverFailConn{}, mem, fusion.Config{Concurrency: 1})
	p.WithGitHead(func(string) string { return "new" })
	p.WithGitLog(func(string) string { return "" })
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := p.Ingest(ctx, svc); !errors.Is(err, context.Canceled) {
		t.Fatalf("a cancelled run must stop at the check with context.Canceled, got %v", err)
	}
	if got, found, _ := mem.MemoryStore.GetIngestState(context.Background(), "svc"); !found || got != live {
		t.Fatalf("ingest_state = %+v found=%v, want the live owner's row untouched", got, found)
	}
}
