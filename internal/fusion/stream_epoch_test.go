package fusion

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/store"
)

// backdate sets every file under dir to an mtime an hour ago — older than
// the stat gate's racy window.
func backdate(t *testing.T, dir string) {
	t.Helper()
	old := time.Now().Add(-time.Hour)
	_ = filepath.Walk(dir, func(p string, fi os.FileInfo, err error) error {
		if err == nil && !fi.IsDir() {
			_ = os.Chtimes(p, old, old)
		}
		return nil
	})
}

// TestStatGateRequiresCurrentEpoch (RT-102, O9): a prior stat record from
// a persistent source vouches for a file only when it was written under the
// current ExtractorEpoch — after an epoch bump the recorded hash and the
// stored fingerprint still agree (both carry the old salt), yet the file
// must be read so the new extractor reaches it.
func TestStatGateRequiresCurrentEpoch(t *testing.T) {
	dir := t.TempDir()
	writeCorpus(t, dir, map[string]string{"a.go": "package a\n\nfunc A() {}\n", "b.go": "package a\n\nfunc B() {}\n"})
	backdate(t, dir)
	conn := newCountingConn()
	st := store.NewMemoryStore()
	p := New(conn, st, Config{Concurrency: 2, DisableTypedEdges: true})
	var recorded []StatRecord
	var source map[string]StatRecord
	p.WithStatSource(func(context.Context, string) (map[string]StatRecord, bool) { return source, source != nil })
	p.WithStatSink(func(_ context.Context, _ string, recs []StatRecord) error { recorded = recs; return nil })
	svc := core.ServiceRef{ID: "ep", Path: dir}
	ctx := context.Background()
	if _, err := p.Ingest(ctx, svc); err != nil {
		t.Fatal(err)
	}
	conn.take()
	if len(recorded) != 2 {
		t.Fatalf("sink got %d records, want 2", len(recorded))
	}
	source = map[string]StatRecord{}
	for _, r := range recorded {
		source[r.Path] = r
	}
	if _, err := p.Ingest(ctx, svc); err != nil {
		t.Fatal(err)
	}
	if got := conn.take(); len(got) != 0 {
		t.Fatalf("current-epoch records: read %v, want nothing", got)
	}
	for k, r := range source {
		r.Epoch = "old"
		source[k] = r
	}
	if _, err := p.Ingest(ctx, svc); err != nil {
		t.Fatal(err)
	}
	if got := conn.take(); len(got) != 2 {
		t.Fatalf("records from another extractor epoch: read %v, want both files", got)
	}
}

// TestStatGateRacyClean (RT-102): a file whose mtime falls inside the racy
// window of its stat is never vouched for by that stat — a same-size write
// within one mtime tick would otherwise be skipped forever on a filesystem
// with coarse mtimes — so the next ingest reads it again.
func TestStatGateRacyClean(t *testing.T) {
	dir := t.TempDir()
	writeCorpus(t, dir, map[string]string{"old.go": "package a\n\nfunc O() {}\n"})
	backdate(t, dir)
	writeCorpus(t, dir, map[string]string{"fresh.go": "package a\n\nfunc F() {}\n"}) // mtime: now
	conn := newCountingConn()
	p := New(conn, store.NewMemoryStore(), Config{Concurrency: 2, DisableTypedEdges: true})
	svc := core.ServiceRef{ID: "racy", Path: dir}
	ctx := context.Background()
	if _, err := p.Ingest(ctx, svc); err != nil {
		t.Fatal(err)
	}
	conn.take()
	if _, err := p.Ingest(ctx, svc); err != nil {
		t.Fatal(err)
	}
	got := conn.take()
	if len(got) != 1 || got["fresh.go"] != 1 {
		t.Fatalf("second ingest read %v; want only the racily-clean fresh.go", got)
	}
}
