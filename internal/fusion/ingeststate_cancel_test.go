package fusion_test

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/fusion"
	"github.com/geoffmilleraz/fathom/internal/store"
)

// cancelInDiscoverConn's Discover waits for the ingest's cancel (a Ctrl-C
// during the walk) and returns it.
type cancelInDiscoverConn struct {
	memConn
	entered chan struct{}
}

func (c cancelInDiscoverConn) Discover(ctx context.Context, _ core.ServiceRef) ([]core.SourceRef, error) {
	c.entered <- struct{}{}
	<-ctx.Done()
	return nil, ctx.Err()
}

// TestCancelledIngestPutsIngestStateBack (GDEV-02, RT-109 §6): an ingest
// cancelled before its first write (a signal during Discover, or a context
// already cancelled when the ingest starts) puts the previous ingest_state
// row back, or leaves none, on a store whose writes honour the context.
// Before, the restore ran under the cancelled context and failed, leaving
// in_progress under this pid; and a read that failed under it took the
// previous row for absent and deleted it.
func TestCancelledIngestPutsIngestStateBack(t *testing.T) {
	svc := core.ServiceRef{ID: "svc", Path: "/repo"}
	prev := store.IngestState{ServiceID: "svc", State: store.IngestComplete, PID: 1, StartedAt: 5, TargetCommit: "old", Path: "/repo"}
	for _, stop := range []string{"in discover", "before start"} {
		for _, seed := range []bool{true, false} {
			name := stop + " no row"
			if seed {
				name = stop + " complete row"
			}
			t.Run(name, func(t *testing.T) {
				st, err := store.Open(filepath.Join(t.TempDir(), "s.sqlite"))
				if err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() { st.Close() })
				iss := st.(store.IngestStateStore)
				if seed {
					if err := iss.SetIngestState(context.Background(), prev); err != nil {
						t.Fatal(err)
					}
				}
				conn := cancelInDiscoverConn{entered: make(chan struct{}, 1)}
				p := fusion.New(conn, st, fusion.Config{Concurrency: 1})
				p.WithGitHead(func(string) string { return "new" })
				p.WithGitLog(func(string) string { return "" })
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()
				if stop == "before start" {
					cancel()
				}
				done := make(chan error, 1)
				go func() {
					_, err := p.Ingest(ctx, svc)
					done <- err
				}()
				if stop == "in discover" {
					select {
					case <-conn.entered:
					case <-time.After(waitSafety):
						t.Fatal("the ingest never reached Discover")
					}
					cancel()
				}
				select {
				case err := <-done:
					if err == nil {
						t.Fatal("a cancelled ingest returned no error")
					}
				case <-time.After(waitSafety):
					t.Fatal("the cancelled ingest never returned")
				}
				got, found, err := iss.GetIngestState(context.Background(), "svc")
				if err != nil {
					t.Fatal(err)
				}
				switch {
				case !seed && found:
					t.Fatalf("ingest_state = %+v, want no row", got)
				case seed && (!found || got != prev):
					t.Fatalf("ingest_state = %+v found=%v, want the previous row %+v", got, found, prev)
				}
			})
		}
	}
}
