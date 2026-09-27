package fusion_test

// RT-109 (docs/design/responsible-tenant.md §6): the watermark advances only
// at an ingest's final commit. An ingest writes ingest_state = in_progress
// first; a killed or halted ingest leaves it (and the old watermark), so the
// next start re-ingests the service instead of serving a half-written index
// under a watermark that claims HEAD.

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/fusion"
	"github.com/geoffmilleraz/fathom/internal/store"
)

// memConn serves fixed Go files from memory.
type memConn struct{ files map[string]string }

func (c memConn) Discover(_ context.Context, svc core.ServiceRef) ([]core.SourceRef, error) {
	var out []core.SourceRef
	for _, p := range []string{"a.go", "b.go"} {
		if _, ok := c.files[p]; ok {
			out = append(out, core.SourceRef{Service: svc.ID, Path: p})
		}
	}
	return out, nil
}

func (c memConn) Fetch(_ context.Context, ref core.SourceRef) ([]byte, error) {
	return []byte(c.files[ref.Path]), nil
}

func (memConn) Watermark(context.Context, core.ServiceRef) (string, error) { return "", nil }

// blockingStore blocks (or fails) the ingest at its first observation write:
// the moment a kill would land mid-ingest.
type blockingStore struct {
	*store.MemoryStore
	entered chan struct{}
	release chan struct{}
	fail    error
}

func (s *blockingStore) PutObservations(ctx context.Context, obs []core.Observation) error {
	if s.entered != nil {
		select {
		case s.entered <- struct{}{}:
		default:
		}
		<-s.release
	}
	if s.fail != nil {
		return s.fail
	}
	return s.MemoryStore.PutObservations(ctx, obs)
}

func TestWatermarkNotAdvancedByKilledIngest(t *testing.T) {
	ctx := context.Background()
	svc := core.ServiceRef{ID: "svc", Path: "/repo"}
	conn := memConn{files: map[string]string{
		"a.go": "package a\n\nfunc A() { B() }\n",
		"b.go": "package a\n\nfunc B() {}\n",
	}}
	mem := store.NewMemoryStore()
	if err := mem.SetServiceMeta(ctx, store.ServiceMeta{ServiceID: "svc", Path: "/repo", IndexedCommit: "old"}); err != nil {
		t.Fatal(err)
	}
	newPipe := func(st store.Store) *fusion.Pipeline {
		p := fusion.New(conn, st, fusion.Config{Concurrency: 1})
		p.WithGitHead(func(string) string { return "new" })
		p.WithGitLog(func(string) string { return "" })
		return p
	}
	watermark := func(t *testing.T) string {
		t.Helper()
		m, found, err := mem.GetServiceMeta(ctx, "svc")
		if err != nil || !found {
			t.Fatalf("GetServiceMeta: found=%v err=%v", found, err)
		}
		return m.IndexedCommit
	}
	state := func(t *testing.T) store.IngestState {
		t.Helper()
		st, found, err := mem.GetIngestState(ctx, "svc")
		if err != nil || !found {
			t.Fatalf("GetIngestState: found=%v err=%v", found, err)
		}
		return st
	}

	t.Run("killed mid-write", func(t *testing.T) {
		bs := &blockingStore{MemoryStore: mem, entered: make(chan struct{}, 1), release: make(chan struct{})}
		done := make(chan error, 1)
		released := false
		defer func() {
			if !released {
				close(bs.release)
				<-done
			}
		}()
		go func() { _, err := newPipe(bs).Ingest(ctx, svc); done <- err }()
		select {
		case <-bs.entered:
		case <-time.After(10 * time.Second):
			t.Fatal("ingest never reached its first write")
		}
		// A SIGKILL here leaves the store exactly as it is now.
		if got := watermark(t); got != "old" {
			t.Errorf("watermark mid-ingest = %q, want %q (it advances only at the final commit)", got, "old")
		}
		st := state(t)
		if st.State != store.IngestInProgress || st.PID != os.Getpid() || st.TargetCommit != "new" || st.StartedAt == 0 {
			t.Errorf("ingest_state mid-ingest = %+v, want in_progress, this pid, target new, a start time", st)
		}
		released = true
		close(bs.release)
		if err := <-done; err != nil {
			t.Fatalf("ingest: %v", err)
		}
		if got := watermark(t); got != "new" {
			t.Errorf("watermark after a completed ingest = %q, want new", got)
		}
		if st := state(t); st.State != store.IngestComplete {
			t.Errorf("ingest_state after a completed ingest = %+v, want complete", st)
		}
	})

	t.Run("halted", func(t *testing.T) {
		fresh := store.NewMemoryStore()
		_ = fresh.SetServiceMeta(ctx, store.ServiceMeta{ServiceID: "svc", Path: "/repo", IndexedCommit: "old"})
		mem = fresh
		bs := &blockingStore{MemoryStore: fresh, fail: errors.New("over budget: ingest producer halted at exit")}
		if _, err := newPipe(bs).Ingest(ctx, svc); err == nil {
			t.Fatal("a halted ingest returned no error")
		}
		if got := watermark(t); got != "old" {
			t.Errorf("watermark after a halted ingest = %q, want old", got)
		}
		if st := state(t); st.State != store.IngestInProgress {
			t.Errorf("ingest_state after a halted ingest = %+v, want in_progress", st)
		}
	})
}
