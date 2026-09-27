package client_test

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/store"
	"github.com/geoffmilleraz/fathom/pkg/client"
)

// TestObligationsIndexStaleSkipsOversize is RT-26 (design §4.3): the
// index-stale check read the whole edited file on every obligations call,
// whatever its size. It now stats first: a file over the 2 MiB hot-path
// cap is never read, is not reported stale (so no hook refreshes it), and
// the check names the skip — "not checked", never a silent "fresh" (I38).
// A file under the cap keeps the exact pre-RT-26 comparison.
func TestObligationsIndexStaleSkipsOversize(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	big := filepath.Join(dir, "big.md")
	small := filepath.Join(dir, "CLAUDE.md")
	if err := os.WriteFile(big, []byte("# big\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(small, []byte("# R\n\n- Keep tests green.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	c := newTestClient(t, dir)
	defer c.Close()
	if _, err := c.Ingest(ctx, client.ServiceRef{ID: "svc", Path: dir}); err != nil {
		t.Fatal("Ingest:", err)
	}

	// big.md grows past the cap after the ingest: its stored fingerprint
	// no longer matches, and the pre-RT-26 check read all of it to say so.
	if err := os.WriteFile(big, bytes.Repeat([]byte("x"), int(client.HotPathReadCap)+1), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(small, []byte("# R\n\n- Keep tests green.\n- New rule.\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	stop := client.ObserveReadsUnder(dir)
	stale, unchecked := c.ObligationsIndexCheck(ctx, "svc", "big.md")
	staleBool := c.ObligationsIndexStale(ctx, "svc", "big.md")
	smallStale, smallUnchecked := c.ObligationsIndexCheck(ctx, "svc", "CLAUDE.md")
	reads := stop()

	if stale || staleBool {
		t.Errorf("oversize file reported stale (check=%v, bool=%v); want not stale — it was never read, so nothing may be claimed or refreshed", stale, staleBool)
	}
	if !strings.Contains(unchecked, "read cap") || !strings.Contains(unchecked, "not refreshed") {
		t.Errorf("unchecked = %q, want it to name the read cap and that the file was not refreshed", unchecked)
	}
	for _, r := range reads {
		if r == big {
			t.Errorf("the oversize file was opened for reading: reads=%v", reads)
		}
	}
	if !smallStale || smallUnchecked != "" {
		t.Errorf("small edited file: stale=%v unchecked=%q, want stale with no skip note", smallStale, smallUnchecked)
	}
	if len(reads) != 1 || reads[0] != small {
		t.Errorf("reads = %v, want exactly the small file, read through the capped reader", reads)
	}
}

// countingConn wraps a connector and counts discovery walks and content
// fetches, so a test can prove what an obligations call touched.
type countingConn struct {
	connector.Connector
	mu       sync.Mutex
	discover int
	fetched  []string
}

func (cc *countingConn) Discover(ctx context.Context, svc core.ServiceRef) ([]core.SourceRef, error) {
	cc.mu.Lock()
	cc.discover++
	cc.mu.Unlock()
	return cc.Connector.Discover(ctx, svc)
}

func (cc *countingConn) Fetch(ctx context.Context, ref core.SourceRef) ([]byte, error) {
	cc.mu.Lock()
	cc.fetched = append(cc.fetched, ref.Path)
	cc.mu.Unlock()
	return cc.Connector.Fetch(ctx, ref)
}

func (cc *countingConn) reset() {
	cc.mu.Lock()
	cc.discover, cc.fetched = 0, nil
	cc.mu.Unlock()
}

func (cc *countingConn) counts() (int, []string) {
	cc.mu.Lock()
	defer cc.mu.Unlock()
	return cc.discover, append([]string(nil), cc.fetched...)
}

// writeTree writes files (repo-relative path -> content) under dir.
func writeTree(t testing.TB, dir string, files map[string]string) {
	t.Helper()
	for p, body := range files {
		abs := filepath.Join(dir, filepath.FromSlash(p))
		if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(abs, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

// TestLayerCoverageFromSourcePathsSingleFlightNeverReadsContents is RT-143
// (design §4.3): obligations' layer coverage and raw fallback each ran
// client.Sources — a discovery walk and a read of every file in the tree —
// on every call. Now the corpus paths come from one names-only walk per
// (service, store watermark, layer digest), shared by concurrent callers;
// only the layer's own guides (and, for the raw fallback, the edited file)
// are ever read; nothing goes through the connector's whole-tree Fetch;
// and a repeat call at the same watermark reads nothing at all. A guide
// edit is still seen at once (its stat moved); a new guide at the next
// ingest.
func TestLayerCoverageFromSourcePathsSingleFlightNeverReadsContents(t *testing.T) {
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

	st := store.NewMemoryStore()
	cc := &countingConn{Connector: connector.NewLocalConnector()}
	c := client.New(cc, noFileStatsStore{st}) // RT-143 is the pre-file_stats path (RT-82 covers the other)
	defer c.Close()
	if _, err := c.Ingest(ctx, client.ServiceRef{ID: "svc", Path: dir}); err != nil {
		t.Fatal("Ingest:", err)
	}
	if err := st.RecordCompileRun(ctx, core.CompileRun{
		RunID: "r1", ServiceID: "svc", Layer: "obligations",
		RanAt: time.Unix(1, 0).UTC(), SectionsTotal: 2, BudgetStopped: true,
	}); err != nil {
		t.Fatal(err)
	}
	cc.reset()

	edited := filepath.Join(dir, "internal", "pkg07", "file.go")
	stop := client.ObserveReadsUnder(dir)
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			cov, found, err := c.ObligationsLayerCoverage(ctx, "svc")
			if err != nil || !found || cov.SectionsTotal != 2 {
				t.Errorf("coverage: cov=%+v found=%v err=%v, want 2 live sections", cov, found, err)
			}
			if _, _, ok, err := c.RawFallbackObligations(ctx, "svc", "internal/pkg07/file.go", 2); err != nil || !ok {
				t.Errorf("raw fallback: ok=%v err=%v, want the uncompiled guides served", ok, err)
			}
		}()
	}
	wg.Wait()
	reads := stop()

	discovers, fetched := cc.counts()
	if discovers != 1 {
		t.Errorf("discovery walks = %d across 16 concurrent calls, want exactly 1", discovers)
	}
	if len(fetched) != 0 {
		head := fetched
		if len(head) > 5 {
			head = head[:5]
		}
		t.Errorf("connector fetched %d files (first: %v); want none — whole-tree content reads are gone", len(fetched), head)
	}
	allowed := map[string]bool{
		filepath.Join(dir, "CLAUDE.md"):             true,
		filepath.Join(dir, "internal", "CLAUDE.md"): true,
		edited: true,
	}
	for _, r := range reads {
		if !allowed[r] {
			t.Errorf("read %s: only the layer's guides and the edited file may be read", r)
		}
	}

	// A repeat call at the same watermark reads no guide again.
	stop = client.ObserveReadsUnder(dir)
	if _, _, err := c.ObligationsLayerCoverage(ctx, "svc"); err != nil {
		t.Fatal(err)
	}
	if again := stop(); len(again) != 0 {
		t.Errorf("repeat coverage call read %v, want nothing (guides memoized by stat)", again)
	}

	// A guide edit is seen on the next call, with no walk.
	writeTree(t, dir, map[string]string{"internal/CLAUDE.md": "# internal\n\n- Sort all outputs.\n\n## More\n\n- And more.\n"})
	future := time.Now().Add(2 * time.Second)
	_ = os.Chtimes(filepath.Join(dir, "internal", "CLAUDE.md"), future, future)
	cov, _, err := c.ObligationsLayerCoverage(ctx, "svc")
	if err != nil || cov.SectionsTotal != 3 {
		t.Errorf("after a guide edit: cov=%+v err=%v, want 3 sections", cov, err)
	}

	// A new guide arrives with the next ingest: one more walk, not per call.
	writeTree(t, dir, map[string]string{"cmd/CLAUDE.md": "# cmd\n\n- Flags are documented.\n"})
	if _, err := c.Ingest(ctx, client.ServiceRef{ID: "svc", Path: dir}); err != nil {
		t.Fatal(err)
	}
	cc.reset()
	for i := 0; i < 3; i++ {
		cov, _, err = c.ObligationsLayerCoverage(ctx, "svc")
	}
	if err != nil || cov.SectionsTotal != 4 {
		t.Errorf("after ingesting a new guide: cov=%+v err=%v, want 4 sections", cov, err)
	}
	if discovers, _ := cc.counts(); discovers != 1 {
		t.Errorf("walks after the new watermark = %d over 3 calls, want 1", discovers)
	}
}

// TestLayerCoverageNamesOversizeGuideAsLowerBound: a guide over the
// hot-path cap is never read; coverage names it in unread_corpus, so its
// sections_total is a lower bound, not a silent omission (I38).
func TestLayerCoverageNamesOversizeGuideAsLowerBound(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	writeTree(t, dir, map[string]string{
		"CLAUDE.md": "# Rules\n\n- Run tests before committing.\n",
		"ADR.md":    "# ADR\n",
		"main.go":   "package main\n",
	})
	st := store.NewMemoryStore()
	c := client.New(connector.NewLocalConnector(), st)
	defer c.Close()
	if _, err := c.Ingest(ctx, client.ServiceRef{ID: "svc", Path: dir}); err != nil {
		t.Fatal(err)
	}
	if err := st.RecordCompileRun(ctx, core.CompileRun{RunID: "r1", ServiceID: "svc", Layer: "obligations", RanAt: time.Unix(1, 0).UTC()}); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "ADR.md"), bytes.Repeat([]byte("# h\n"), int(client.HotPathReadCap)/4+1), 0o644); err != nil {
		t.Fatal(err)
	}
	stop := client.ObserveReadsUnder(dir)
	cov, found, err := c.ObligationsLayerCoverage(ctx, "svc")
	reads := stop()
	if err != nil || !found {
		t.Fatalf("found=%v err=%v", found, err)
	}
	if len(cov.UnreadCorpus) != 1 || !strings.HasPrefix(cov.UnreadCorpus[0], "ADR.md: ") || !strings.Contains(cov.UnreadCorpus[0], "read cap") {
		t.Errorf("UnreadCorpus = %v, want ADR.md named with the cap", cov.UnreadCorpus)
	}
	if cov.SectionsTotal != 1 {
		t.Errorf("SectionsTotal = %d, want 1 (CLAUDE.md only — a lower bound)", cov.SectionsTotal)
	}
	for _, r := range reads {
		if strings.HasSuffix(r, "ADR.md") {
			t.Errorf("oversize ADR.md was read")
		}
	}
}

// TestRawFallbackReadsOnlyEditedFile is RT-81 (design §4.3): the raw
// fallback fetched every file in the tree to find the edited one's content
// for its behaviour signal. It now reads the edited file alone (beside the
// layer's own guides), through the capped reader; an edited file over the
// cap is not read at all, the raw rows are still served from the guides,
// and why names the skip.
func TestRawFallbackReadsOnlyEditedFile(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	tree := map[string]string{
		"CLAUDE.md":   "# Rules\n\n## Non-negotiables — the workspace/ convention\n\nAll build artifacts belong under workspace/.\n",
		"cmd/emit.go": "package cmd\n\nimport \"os\"\n\nfunc emit(dir string) error {\n\treturn os.WriteFile(dir, nil, 0o644)\n}\n",
		"cmd/huge.go": "package cmd\n",
	}
	for i := 0; i < 30; i++ {
		tree[fmt.Sprintf("lib/l%02d.go", i)] = fmt.Sprintf("package lib\n\nfunc L%02d() {}\n", i)
	}
	writeTree(t, dir, tree)
	cc := &countingConn{Connector: connector.NewLocalConnector()}
	c := client.New(cc, noFileStatsStore{store.NewMemoryStore()})
	defer c.Close()
	if _, err := c.Ingest(ctx, client.ServiceRef{ID: "svc", Path: dir}); err != nil {
		t.Fatal(err)
	}
	cc.reset()

	stop := client.ObserveReadsUnder(dir)
	rows, _, ok, err := c.RawFallbackObligations(ctx, "svc", "cmd/emit.go", 1)
	reads := stop()
	if err != nil || !ok || len(rows) != 1 {
		t.Fatalf("rows=%+v ok=%v err=%v, want one raw row", rows, ok, err)
	}
	_, fetched := cc.counts()
	if len(fetched) != 0 {
		t.Errorf("connector fetched %d files; want none", len(fetched))
	}
	emit := filepath.Join(dir, "cmd", "emit.go")
	nEmit := 0
	for _, r := range reads {
		switch r {
		case emit:
			nEmit++
		case filepath.Join(dir, "CLAUDE.md"):
		default:
			t.Errorf("read %s; only the edited file and the layer's guides may be read", r)
		}
	}
	if nEmit != 1 {
		t.Errorf("edited file read %d times, want exactly once", nEmit)
	}

	// The edited file grows past the cap: never read, rows still served
	// from the guides, and why says the file was not read.
	if err := os.WriteFile(filepath.Join(dir, "cmd", "huge.go"), bytes.Repeat([]byte("// x\n"), int(client.HotPathReadCap)/5+1), 0o644); err != nil {
		t.Fatal(err)
	}
	stop = client.ObserveReadsUnder(dir)
	rows, why, ok, err := c.RawFallbackObligations(ctx, "svc", "cmd/huge.go", 1)
	reads = stop()
	if err != nil || !ok || len(rows) != 1 {
		t.Fatalf("oversize edited file: rows=%+v ok=%v err=%v, want the guide's raw row", rows, ok, err)
	}
	if !strings.Contains(why, "not read") || !strings.Contains(why, "read cap") {
		t.Errorf("why = %q, want it to name the unread edited file and the cap", why)
	}
	for _, r := range reads {
		if strings.HasSuffix(r, "huge.go") {
			t.Errorf("oversize edited file was read")
		}
	}
}

// TestRawFallbackNamesUnreadGuide is I38 on the raw fallback (RT-81 review):
// a guide layerCorpus could not read (over the cap, EPERM) must never
// vanish silently. (a) With no compile_runs row — the exact case the raw
// fallback exists for — the served rows' why names the unread guide and
// coverage carries it even though found is false. (b) With a compile run,
// an unread nested guide that governs the file must not let the fallback
// conclude "this file's guide is fully compiled" and go quiet.
func TestRawFallbackNamesUnreadGuide(t *testing.T) {
	ctx := context.Background()
	oversize := bytes.Repeat([]byte("# h\n"), int(client.HotPathReadCap)/4+1)
	adr := "# ADR\n\n## ADR-001: Keep builds in workspace\n\nAll build artifacts belong under workspace/.\n"

	t.Run("no compile run", func(t *testing.T) {
		dir := t.TempDir()
		writeTree(t, dir, map[string]string{
			"CLAUDE.md": "# Rules\n\n- Run tests before committing.\n",
			"ADR.md":    adr,
			"main.go":   "package main\n",
		})
		st := store.NewMemoryStore()
		c := client.New(connector.NewLocalConnector(), st)
		defer c.Close()
		if _, err := c.Ingest(ctx, client.ServiceRef{ID: "svc", Path: dir}); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "CLAUDE.md"), oversize, 0o644); err != nil {
			t.Fatal(err)
		}
		rows, why, ok, err := c.RawFallbackObligations(ctx, "svc", "main.go", 3)
		if err != nil || !ok || len(rows) == 0 {
			t.Fatalf("rows=%d ok=%v err=%v, want ADR.md's raw rows", len(rows), ok, err)
		}
		if !strings.Contains(why, "CLAUDE.md") || !strings.Contains(why, "lower bound") {
			t.Errorf("why = %q, want the unread CLAUDE.md named as a lower bound", why)
		}
		cov, found, err := c.ObligationsLayerCoverage(ctx, "svc")
		if err != nil || found {
			t.Fatalf("found=%v err=%v, want found=false (no compile run)", found, err)
		}
		if len(cov.UnreadCorpus) != 1 || !strings.HasPrefix(cov.UnreadCorpus[0], "CLAUDE.md: ") {
			t.Errorf("UnreadCorpus = %v, want CLAUDE.md named even with no compile run", cov.UnreadCorpus)
		}
	})

	t.Run("unread nested guide governs", func(t *testing.T) {
		dir := t.TempDir()
		writeTree(t, dir, map[string]string{
			"ADR.md":        adr,
			"sub/CLAUDE.md": "# Sub\n\n- Keep sub small.\n",
			"sub/x.go":      "package sub\n",
		})
		st := store.NewMemoryStore()
		c := client.New(connector.NewLocalConnector(), st)
		defer c.Close()
		if _, err := c.Ingest(ctx, client.ServiceRef{ID: "svc", Path: dir}); err != nil {
			t.Fatal(err)
		}
		if err := st.RecordCompileRun(ctx, core.CompileRun{RunID: "r1", ServiceID: "svc", Layer: "obligations", RanAt: time.Unix(1, 0).UTC()}); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "sub", "CLAUDE.md"), oversize, 0o644); err != nil {
			t.Fatal(err)
		}
		rows, why, ok, err := c.RawFallbackObligations(ctx, "svc", "sub/x.go", 3)
		if err != nil || !ok || len(rows) == 0 {
			t.Fatalf("rows=%d ok=%v err=%v, want the fallback to fire: the governing sub/CLAUDE.md was not read, so it is not known compiled", len(rows), ok, err)
		}
		if !strings.Contains(why, "sub/CLAUDE.md") {
			t.Errorf("why = %q, want the unread governing guide named", why)
		}
	})
}

// TestEditedFileReadOnlyWhenStatMoves is design §4.3's stat-first rule for
// the edited file (RT-26/RT-81 review): the freshness check hashed, and the
// raw fallback re-read, the edited file on every obligations call. Both
// now keep what they derived (the hash; the behaviour tokens) against the
// file's stat and read it again only when the stat moves — while a stored
// fingerprint that moved (a re-ingest) is still compared afresh.
func TestEditedFileReadOnlyWhenStatMoves(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	writeTree(t, dir, map[string]string{
		"CLAUDE.md":   "# Rules\n\n## Non-negotiables — the workspace/ convention\n\nAll build artifacts belong under workspace/.\n",
		"cmd/emit.go": "package cmd\n\nimport \"os\"\n\nfunc emit(dir string) error {\n\treturn os.WriteFile(dir, nil, 0o644)\n}\n",
	})
	c := client.New(connector.NewLocalConnector(), store.NewMemoryStore())
	defer c.Close()
	if _, err := c.Ingest(ctx, client.ServiceRef{ID: "svc", Path: dir}); err != nil {
		t.Fatal(err)
	}
	emit := filepath.Join(dir, "cmd", "emit.go")
	countEmit := func(reads []string) int {
		n := 0
		for _, r := range reads {
			if r == emit {
				n++
			}
		}
		return n
	}

	stop := client.ObserveReadsUnder(dir)
	for i := 0; i < 3; i++ {
		if stale, _ := c.ObligationsIndexCheck(ctx, "svc", "cmd/emit.go"); stale {
			t.Fatalf("call %d: unchanged file reported stale", i)
		}
		if _, _, ok, err := c.RawFallbackObligations(ctx, "svc", "cmd/emit.go", 1); err != nil || !ok {
			t.Fatalf("call %d: raw fallback ok=%v err=%v", i, ok, err)
		}
	}
	if n := countEmit(stop()); n != 2 {
		t.Errorf("edited file read %d times over 3 unchanged calls; want 2 (one hash, one token read), then none", n)
	}

	// The file changes: its stat moves, so each derivation reads it once.
	if err := os.WriteFile(emit, []byte("package cmd\n\nfunc emit() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	future := time.Now().Add(2 * time.Second)
	_ = os.Chtimes(emit, future, future)
	stop = client.ObserveReadsUnder(dir)
	stale, _ := c.ObligationsIndexCheck(ctx, "svc", "cmd/emit.go")
	stale2, _ := c.ObligationsIndexCheck(ctx, "svc", "cmd/emit.go")
	if n := countEmit(stop()); n != 1 {
		t.Errorf("after an edit the hash re-read %d times over two checks; want 1", n)
	}
	if !stale || !stale2 {
		t.Errorf("edited file stale=%v,%v; want stale on both checks (the memo keeps the comparison, not the verdict)", stale, stale2)
	}

	// A re-ingest moves the stored fingerprint; the unchanged stat's hash
	// is compared against it afresh, without reading.
	if _, err := c.Ingest(ctx, client.ServiceRef{ID: "svc", Path: dir}); err != nil {
		t.Fatal(err)
	}
	stop = client.ObserveReadsUnder(dir)
	stale, _ = c.ObligationsIndexCheck(ctx, "svc", "cmd/emit.go")
	if n := countEmit(stop()); n != 0 {
		t.Errorf("after a re-ingest with no edit the file was read %d times; want 0", n)
	}
	if stale {
		t.Error("after a re-ingest the file is still reported stale")
	}
}

// gatedConn blocks Discover, once armed, until release is closed, and
// counts the walks that entered it.
type gatedConn struct {
	connector.Connector
	armed   atomic.Bool
	release chan struct{}
	walks   atomic.Int64
}

func (g *gatedConn) Discover(ctx context.Context, svc core.ServiceRef) ([]core.SourceRef, error) {
	if g.armed.Load() {
		g.walks.Add(1)
		<-g.release
	}
	return g.Connector.Discover(ctx, svc)
}

// TestLayerCoveragePathsLeaderHonoursCancel (RT-143 review): the caller
// that started the corpus walk waited for it even after its own context
// was cancelled (a hook past its deadline), while joined callers could
// leave. The leader now leaves on cancel too; the walk finishes detached
// and is still memoized for the next caller.
func TestLayerCoveragePathsLeaderHonoursCancel(t *testing.T) {
	dir := t.TempDir()
	writeTree(t, dir, map[string]string{"CLAUDE.md": "# R\n\n- Keep tests green.\n", "main.go": "package main\n"})
	g := &gatedConn{Connector: connector.NewLocalConnector(), release: make(chan struct{})}
	c := client.New(g, noFileStatsStore{store.NewMemoryStore()})
	defer c.Close()
	if _, err := c.Ingest(context.Background(), client.ServiceRef{ID: "svc", Path: dir}); err != nil {
		t.Fatal(err)
	}
	g.armed.Store(true)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		_, _, _ = c.ObligationsLayerCoverage(ctx, "svc")
	}()
	for g.walks.Load() == 0 {
		runtime.Gosched()
	}
	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		close(g.release)
		t.Fatal("the leading caller kept waiting on the corpus walk after its context was cancelled")
	}
	close(g.release)
	// The detached walk completes and is memoized: the next call walks nothing.
	deadline := time.Now().Add(2 * time.Second)
	for {
		if _, _, err := c.ObligationsLayerCoverage(context.Background(), "svc"); err != nil {
			t.Fatal(err)
		}
		if g.walks.Load() == 1 || time.Now().After(deadline) {
			break
		}
	}
	if n := g.walks.Load(); n != 1 {
		t.Errorf("walks = %d, want the one detached walk reused", n)
	}
}

// TestLayerCoveragePathsSingleFlightWithoutWatermark (RT-143 review): a
// service with no store watermark (never ingested, no meta) cannot be
// memoized, but concurrent callers must still share one walk rather than
// each walking the tree.
func TestLayerCoveragePathsSingleFlightWithoutWatermark(t *testing.T) {
	dir := t.TempDir()
	writeTree(t, dir, map[string]string{"CLAUDE.md": "# R\n\n- Keep tests green.\n", "main.go": "package main\n"})
	g := &gatedConn{Connector: connector.NewLocalConnector(), release: make(chan struct{})}
	g.armed.Store(true)
	c := client.New(g, noFileStatsStore{store.NewMemoryStore()})
	defer c.Close()

	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _, _ = c.ObligationsLayerCoverage(context.Background(), dir)
		}()
	}
	for g.walks.Load() == 0 {
		runtime.Gosched()
	}
	time.Sleep(200 * time.Millisecond) // let the other callers arrive and join
	close(g.release)
	wg.Wait()
	if n := g.walks.Load(); n != 1 {
		t.Errorf("walks = %d across 8 concurrent calls on a service with no watermark, want 1", n)
	}
}

// TestObligationsHotPathReaderAgnosticTripwire strengthens the "reads only
// the guides and the edited file" proofs (RT-81/RT-143 review): those
// watch readFileCapped and the connector's Fetch, so a regression to a
// plain os.ReadFile tree walk would slip past them. A FIFO in the tree
// blocks ANY reader that opens it, whatever the API; an obligations call
// that finishes proves nothing opened a non-selected file's contents.
func TestObligationsHotPathReaderAgnosticTripwire(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	writeTree(t, dir, map[string]string{
		"CLAUDE.md":          "# Rules\n\n- Run tests before committing.\n",
		"internal/CLAUDE.md": "# internal\n\n- Sort all outputs.\n",
		"internal/a/a.go":    "package a\n\nfunc A() {}\n",
	})
	st := store.NewMemoryStore()
	c := client.New(connector.NewLocalConnector(), st)
	defer c.Close()
	if _, err := c.Ingest(ctx, client.ServiceRef{ID: "svc", Path: dir}); err != nil {
		t.Fatal(err)
	}
	if err := st.RecordCompileRun(ctx, core.CompileRun{RunID: "r1", ServiceID: "svc", Layer: "obligations", RanAt: time.Unix(1, 0).UTC(), SectionsTotal: 2, BudgetStopped: true}); err != nil {
		t.Fatal(err)
	}
	if err := syscall.Mkfifo(filepath.Join(dir, "internal", "a", "tripwire.go"), 0o644); err != nil {
		t.Skipf("mkfifo: %v", err)
	}
	// A new store watermark is not needed: the names-only walk runs on
	// the first call after ingest anyway.
	done := make(chan error, 1)
	go func() {
		if _, _, err := c.ObligationsLayerCoverage(ctx, "svc"); err != nil {
			done <- err
			return
		}
		c.ObligationsIndexCheck(ctx, "svc", "internal/a/a.go")
		_, _, _, err := c.RawFallbackObligations(ctx, "svc", "internal/a/a.go", 2)
		done <- err
	}()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		// Unblock the stuck reader so the test binary can exit.
		if f, err := os.OpenFile(filepath.Join(dir, "internal", "a", "tripwire.go"), os.O_WRONLY, 0); err == nil {
			f.Close()
		}
		t.Fatal("an obligations call opened a non-selected file's contents (blocked on the FIFO tripwire)")
	}
}

// TestConcurrentColdHotReadsShareOneRead (RT-95, I51): concurrent cold
// obligations callers share one read of each file — the edited file's
// hash and the layer corpus's guides — instead of each reading it before
// the first stores what it read.
func TestConcurrentColdHotReadsShareOneRead(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	writeTree(t, dir, map[string]string{
		"CLAUDE.md":   "# Rules\n\n## Non-negotiables — the workspace/ convention\n\nAll build artifacts belong under workspace/.\n",
		"cmd/emit.go": "package cmd\n\nimport \"os\"\n\nfunc emit(dir string) error {\n\treturn os.WriteFile(dir, nil, 0o644)\n}\n",
	})
	c := client.New(connector.NewLocalConnector(), store.NewMemoryStore())
	defer c.Close()
	if _, err := c.Ingest(ctx, client.ServiceRef{ID: "svc", Path: dir}); err != nil {
		t.Fatal(err)
	}
	stop := client.ObserveReadsUnderSlow(dir, 30*time.Millisecond)
	const callers = 16
	var wg sync.WaitGroup
	for i := 0; i < callers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = c.ObligationsIndexCheck(ctx, "svc", "cmd/emit.go")
			_, _ = c.Obligations(ctx, "svc", "cmd/emit.go", 5)
		}()
	}
	wg.Wait()
	per := map[string]int{}
	for _, r := range stop() {
		per[r]++
	}
	for path, n := range per {
		if n != 1 {
			t.Errorf("%s read %d times by %d concurrent cold callers; want 1", path, n, callers)
		}
	}
	if len(per) == 0 {
		t.Fatal("no read observed: the test exercises nothing")
	}
}
