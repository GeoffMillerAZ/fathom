package fusion

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/store"
)

// countingConn is a LocalConnector that counts reads per path (Stat passes
// through, so the stat gate is live).
type countingConn struct {
	*connector.LocalConnector
	mu    sync.Mutex
	reads map[string]int
}

func newCountingConn() *countingConn {
	return &countingConn{LocalConnector: connector.NewLocalConnector(), reads: map[string]int{}}
}

func (c *countingConn) Fetch(ctx context.Context, ref core.SourceRef) ([]byte, error) {
	c.mu.Lock()
	c.reads[ref.Path]++
	c.mu.Unlock()
	return c.LocalConnector.Fetch(ctx, ref)
}

func (c *countingConn) take() map[string]int {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := c.reads
	c.reads = map[string]int{}
	return out
}

// TestIngest_ReadsOnlyStatChangedFiles is RT-102 (design §7.2): the second
// ingest of an unchanged tree reads no file; after one file's content (and
// so its size and mtime) changes, and another's mtime alone moves, exactly
// those two are read; a file whose stored fingerprint no longer matches the
// recorded hash is read though its stat did not move; Config.Rehash reads
// every file. The stored index is the same as a full read's.
func TestIngest_ReadsOnlyStatChangedFiles(t *testing.T) {
	dir := t.TempDir()
	files := map[string]string{}
	for i := 0; i < 12; i++ {
		files[fmt.Sprintf("pkg/f%02d.go", i)] = fmt.Sprintf("package pkg\n\nfunc F%02d() int { return %d }\n", i, i)
	}
	files["README.md"] = "# r\n"
	writeCorpus(t, dir, files)
	conn := newCountingConn()
	st := store.NewMemoryStore()
	p := New(conn, st, Config{Concurrency: 4, DisableTypedEdges: true})
	svc := core.ServiceRef{ID: "sg", Path: dir}
	ctx := context.Background()
	ingest := func() Report {
		t.Helper()
		r, err := p.Ingest(ctx, svc)
		if err != nil {
			t.Fatal(err)
		}
		return r
	}

	if r := ingest(); len(conn.take()) != len(files) || r.StatSkipped != 0 {
		t.Fatalf("first ingest must read every file (stat-skipped %d)", r.StatSkipped)
	}
	r := ingest()
	if got := conn.take(); len(got) != 0 {
		t.Fatalf("an unchanged tree read %v; want nothing", got)
	}
	if r.StatSkipped != len(files) || r.Skipped != len(files) {
		t.Fatalf("unchanged: StatSkipped %d Skipped %d, want %d", r.StatSkipped, r.Skipped, len(files))
	}

	writeCorpus(t, dir, map[string]string{"pkg/f03.go": "package pkg\n\nfunc F03() int { return 333 }\n"})
	later := time.Now().Add(time.Hour)
	if err := os.Chtimes(filepath.Join(dir, "pkg/f07.go"), later, later); err != nil {
		t.Fatal(err)
	}
	r = ingest()
	got := conn.take()
	keys := make([]string, 0, len(got))
	for k := range got {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	if strings.Join(keys, ",") != "pkg/f03.go,pkg/f07.go" {
		t.Fatalf("read %v; want exactly the changed and the touched file", keys)
	}
	if r.Extracted != 1 {
		t.Fatalf("Extracted %d; want 1 (the touched file's bytes are unchanged)", r.Extracted)
	}
	syms, _ := st.SymbolNamesForSource(ctx, "sg", "pkg/f03.go")
	if len(syms) != 1 || syms[0] != "F03" {
		t.Fatalf("the changed file's symbols: %v", syms)
	}

	// The stored fingerprint moved (a /refresh-file, a purge): the stat
	// alone no longer vouches for the file.
	if err := st.DeleteFingerprint(ctx, "sg", "pkg/f05.go"); err != nil {
		t.Fatal(err)
	}
	ingest()
	if got := conn.take(); len(got) != 1 || got["pkg/f05.go"] != 1 {
		t.Fatalf("after its fingerprint moved: read %v; want pkg/f05.go only", got)
	}

	// --rehash reads everything.
	rp := New(conn, st, Config{Concurrency: 4, DisableTypedEdges: true, Rehash: true})
	rp.statMemo = p.statMemo
	if _, err := rp.Ingest(ctx, svc); err != nil {
		t.Fatal(err)
	}
	if got := conn.take(); len(got) != len(files) {
		t.Fatalf("rehash read %d files; want %d", len(got), len(files))
	}

	// A stat source (the store's file_stats once wired) replaces the memo.
	fresh := New(conn, st, Config{Concurrency: 4, DisableTypedEdges: true})
	var sunk []StatRecord
	fresh.WithStatSink(func(_ context.Context, _ string, recs []StatRecord) error { sunk = recs; return nil })
	if _, err := fresh.Ingest(ctx, svc); err != nil {
		t.Fatal(err)
	}
	conn.take()
	if len(sunk) != len(files) {
		t.Fatalf("stat sink got %d records; want %d", len(sunk), len(files))
	}
	bySrc := map[string]StatRecord{}
	for _, r := range sunk {
		bySrc[r.Path] = r
	}
	other := New(conn, st, Config{Concurrency: 4, DisableTypedEdges: true})
	other.WithStatSource(func(context.Context, string) (map[string]StatRecord, bool) { return bySrc, true })
	if _, err := other.Ingest(ctx, svc); err != nil {
		t.Fatal(err)
	}
	if got := conn.take(); len(got) != 0 {
		t.Fatalf("with a stat source, an unchanged tree read %v", got)
	}
}

// putCounter counts PutObservations calls.
type putCounter struct {
	store.Store
	mu    sync.Mutex
	calls int
	obs   int
}

func (c *putCounter) PutObservations(ctx context.Context, obs []core.Observation) error {
	c.mu.Lock()
	c.calls++
	c.obs += len(obs)
	c.mu.Unlock()
	return c.Store.PutObservations(ctx, obs)
}

// TestIngestPeakContentWithinInflightBytes is RT-103 (design §7.2): pass 2
// reads through the in-flight byte window — the most content held at once
// never exceeds Config.InflightBytes although the corpus is several times
// larger — writes observations in batches rather than one write per file,
// and stores exactly what an unbounded read stores.
func TestIngestPeakContentWithinInflightBytes(t *testing.T) {
	dir := t.TempDir()
	files := map[string]string{}
	pad := strings.Repeat("// padding line to give the file some weight\n", 2500) // ~110 KiB
	for i := 0; i < 60; i++ {
		files[fmt.Sprintf("p/f%02d.go", i)] = fmt.Sprintf("package p\n\nfunc F%02d() int { return %d }\n%s", i, i, pad)
	}
	writeCorpus(t, dir, files)
	var corpus int64
	for _, b := range files {
		corpus += int64(len(b))
	}
	const window = 3 << 20
	if corpus < 2*window {
		t.Fatalf("corpus %d bytes is not larger than the window; the test would prove nothing", corpus)
	}

	ctx := context.Background()
	counted := &putCounter{Store: store.NewMemoryStore()}
	p := New(connector.NewLocalConnector(), counted, Config{Concurrency: 4, InflightBytes: window, DisableTypedEdges: true})
	rep, err := p.Ingest(ctx, core.ServiceRef{ID: "w", Path: dir})
	if err != nil {
		t.Fatal(err)
	}
	if rep.PeakInflightBytes <= 0 || rep.PeakInflightBytes > window {
		t.Fatalf("peak content in flight %d bytes; want 0 < peak ≤ %d (corpus %d)", rep.PeakInflightBytes, window, corpus)
	}
	if rep.Extracted != len(files) {
		t.Fatalf("Extracted %d, want %d", rep.Extracted, len(files))
	}
	if counted.calls*4 > len(files) {
		t.Fatalf("%d PutObservations calls for %d files (%d observations); want batched writes", counted.calls, len(files), counted.obs)
	}

	// Same index as an unbounded (window larger than the corpus) read.
	ref := store.NewMemoryStore()
	q := New(connector.NewLocalConnector(), ref, Config{Concurrency: 1, InflightBytes: 1 << 30, DisableTypedEdges: true})
	if _, err := q.Ingest(ctx, core.ServiceRef{ID: "w", Path: dir}); err != nil {
		t.Fatal(err)
	}
	a, _ := counted.QueryObservations(ctx, "w", "")
	b, _ := ref.QueryObservations(ctx, "w", "")
	if len(a) != len(b) || len(a) != len(files) {
		t.Fatalf("windowed stored %d observations, unbounded %d; want %d each", len(a), len(b), len(files))
	}
	for i := range a {
		if a[i].SourceRef != b[i].SourceRef || a[i].Symbol != b[i].Symbol || a[i].ContentHash != b[i].ContentHash {
			t.Fatalf("observation %d differs: %+v vs %+v", i, a[i], b[i])
		}
	}
}
