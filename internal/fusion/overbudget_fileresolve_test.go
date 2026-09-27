package fusion_test

// RT-153 review finding 1 (I38): an ingest that reaches the refuse rung
// between the main fetch pass and the file-resolved edge pass must fail with
// the named over-budget error — never replace the service's file-resolved
// edges with an empty set and report ok, and never leave the freshness
// watermark claiming the halted run's HEAD.

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/fusion"
	"github.com/geoffmilleraz/fathom/internal/store"
	"github.com/geoffmilleraz/fathom/internal/tenancy"
)

// flipConn serves the TS fixture and, like LocalConnector, asks the producer
// governor before each file. After flipAfter fetches it publishes refuse, so
// the main pass completes and the later file-resolved pass is refused.
type flipConn struct {
	dir       string
	files     []string
	flipAfter int64
	n         atomic.Int64
}

func (c *flipConn) Discover(_ context.Context, svc core.ServiceRef) ([]core.SourceRef, error) {
	out := make([]core.SourceRef, len(c.files))
	for i, f := range c.files {
		out[i] = core.SourceRef{Service: svc.ID, Path: f}
	}
	return out, nil
}

func (c *flipConn) Fetch(ctx context.Context, ref core.SourceRef) ([]byte, error) {
	if err := tenancy.ProducerAllowsCtx(ctx, tenancy.ClassTree); err != nil {
		return nil, err
	}
	if c.flipAfter > 0 && c.n.Add(1) == c.flipAfter {
		defer tenancy.PublishLevel(tenancy.LevelRefuse)
	}
	return os.ReadFile(filepath.Join(c.dir, ref.Path))
}

func (c *flipConn) Watermark(context.Context, core.ServiceRef) (string, error) { return "", nil }

func fileResolvedCount(t *testing.T, st store.Store, svc string) int {
	t.Helper()
	edges, err := st.TypedEdges(context.Background(), svc)
	if err != nil {
		t.Fatalf("TypedEdges: %v", err)
	}
	n := 0
	for _, e := range edges {
		if e.Kind == core.EdgeFileResolved {
			n++
		}
	}
	return n
}

func TestIngestRefusedMidPassFailsAndKeepsFileResolvedEdges(t *testing.T) {
	t.Cleanup(func() { tenancy.PublishLevel(tenancy.LevelNormal) })
	src, err := filepath.Abs(filepath.Join("testdata", "tsfixture"))
	if err != nil {
		t.Fatal(err)
	}
	files := []string{"index.ts", "util.ts", "wrapper.ts"}
	// A copy, so the second run can change a file: since RT-104/RT-108 an
	// ingest with nothing changed runs no file-resolved pass at all (and so
	// has nothing left to halt).
	dir := t.TempDir()
	for _, f := range files {
		b, rerr := os.ReadFile(filepath.Join(src, f))
		if rerr != nil {
			t.Fatal(rerr)
		}
		if werr := os.WriteFile(filepath.Join(dir, f), b, 0o644); werr != nil {
			t.Fatal(werr)
		}
	}
	st := store.NewMemoryStore()
	svc := core.ServiceRef{ID: "ts", Path: dir}
	ctx := context.Background()
	heads := []string{"head-1", "head-2"}
	newPipe := func(conn *flipConn, head string) *fusion.Pipeline {
		p := fusion.New(conn, st, fusion.Config{Concurrency: 1})
		p.WithGitHead(func(string) string { return head })
		p.WithGitLog(func(string) string { return "" })
		return p
	}

	if _, err := newPipe(&flipConn{dir: dir, files: files}, heads[0]).Ingest(ctx, svc); err != nil {
		t.Fatalf("baseline ingest: %v", err)
	}
	before := fileResolvedCount(t, st, "ts")
	if before == 0 {
		t.Fatal("baseline ingest resolved no file-resolved edges; the fixture must produce some")
	}

	// The main pass fetches every file once; the flip lands on its last one.
	uf, err := os.OpenFile(filepath.Join(dir, "util.ts"), os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = uf.WriteString("\n// changed\n")
	_ = uf.Close()
	_, err = newPipe(&flipConn{dir: dir, files: files, flipAfter: int64(len(files))}, heads[1]).Ingest(ctx, svc)
	if !errors.Is(err, tenancy.ErrOverBudget) {
		t.Fatalf("ingest halted at refuse returned %v, want an error wrapping ErrOverBudget", err)
	}
	if got := fileResolvedCount(t, st, "ts"); got != before {
		t.Fatalf("file-resolved edges = %d after a refused pass, want the prior %d kept (a halt is never a shorter index)", got, before)
	}
	meta, found, mErr := st.GetServiceMeta(ctx, "ts")
	if mErr != nil || !found {
		t.Fatalf("GetServiceMeta: found=%v err=%v", found, mErr)
	}
	if meta.IndexedCommit == heads[1] {
		t.Fatalf("watermark claims the halted run's head %q; a halted ingest must not look fresh", heads[1])
	}
}
