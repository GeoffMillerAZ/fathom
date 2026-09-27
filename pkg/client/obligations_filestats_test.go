package client_test

import (
	"context"
	"fmt"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/store"
	"github.com/geoffmilleraz/fathom/pkg/client"
)

// noFileStatsStore hides file_stats, as a store written before RT-76 has
// none until the service's next ingest.
type noFileStatsStore struct{ store.Store }

func (noFileStatsStore) FileStats(context.Context, string) ([]store.FileStat, bool, error) {
	return nil, false, nil
}

// TestLayerCoverageFromFileStatsNeverReadsContents is RT-82 (§4.3):
// obligations' layer coverage takes the corpus paths from the file_stats
// rows the ingest wrote — no discovery walk, no connector fetch, no file
// outside the layer's guides read — memoized per (service, watermark,
// layer digest) and single-flighted; a store without file_stats yet falls
// back to one names-only walk per watermark (RT-143's path).
func TestLayerCoverageFromFileStatsNeverReadsContents(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	tree := map[string]string{
		"CLAUDE.md":          "# Rules\n\n- Run tests before committing.\n",
		"internal/CLAUDE.md": "# internal\n\n- Sort all outputs.\n",
	}
	for i := 0; i < 40; i++ {
		tree[fmt.Sprintf("internal/pkg%02d/file.go", i)] = fmt.Sprintf("package pkg%02d\n\nfunc F%02d() {}\n", i, i)
	}
	writeTree(t, dir, tree)

	st, err := store.NewSQLiteStore(filepath.Join(t.TempDir(), "o.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	cc := &countingConn{Connector: connector.NewLocalConnector()}
	c := client.New(cc, st)
	defer c.Close()
	if _, err := c.Ingest(ctx, client.ServiceRef{ID: "svc", Path: dir}); err != nil {
		t.Fatal("Ingest:", err)
	}
	if fs, ok, _ := st.FileStats(ctx, "svc"); !ok || len(fs) != 42 {
		t.Fatalf("ingest wrote %d file_stats rows (ok=%v), want 42", len(fs), ok)
	}
	if err := st.RecordCompileRun(ctx, core.CompileRun{RunID: "r1", ServiceID: "svc", Layer: "obligations", RanAt: time.Unix(1, 0).UTC(), SectionsTotal: 2, BudgetStopped: true}); err != nil {
		t.Fatal(err)
	}
	cc.reset()
	stop := client.ObserveReadsUnder(dir)
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			cov, found, err := c.ObligationsLayerCoverage(ctx, "svc")
			if err != nil || !found || cov.SectionsTotal != 2 {
				t.Errorf("coverage: cov=%+v found=%v err=%v", cov, found, err)
			}
		}()
	}
	wg.Wait()
	reads := stop()
	discovers, fetched := cc.counts()
	if discovers != 0 || len(fetched) != 0 {
		t.Fatalf("coverage walked %d time(s) and fetched %v; the paths come from file_stats", discovers, fetched)
	}
	guides := map[string]bool{filepath.Join(dir, "CLAUDE.md"): true, filepath.Join(dir, "internal", "CLAUDE.md"): true}
	for _, r := range reads {
		if !guides[r] {
			t.Errorf("read %s: only the layer's guides may be read", r)
		}
	}

	// Before a service's first post-RT-76 ingest there are no file_stats:
	// one names-only walk per watermark, shared by concurrent callers.
	legacy := client.New(cc, noFileStatsStore{st})
	defer legacy.Close()
	cc.reset()
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _, _ = legacy.ObligationsLayerCoverage(ctx, "svc")
		}()
	}
	wg.Wait()
	if discovers, fetched := cc.counts(); discovers > 1 || len(fetched) != 0 {
		t.Fatalf("legacy store: %d walks, %d fetches; want at most one names-only walk", discovers, len(fetched))
	}
}
