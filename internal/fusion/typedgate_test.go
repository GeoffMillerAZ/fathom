package fusion

import (
	"context"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/store"
)

// gateFixture is a Go module with a CUE module beside it (the cuefixture
// copied in) and a README.
func gateFixture(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("no go toolchain on PATH")
	}
	dir := t.TempDir()
	write := func(rel, body string) {
		t.Helper()
		p := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("go.mod", "module example.com/gate\n\ngo 1.22\n")
	write("a.go", "package gate\n\nfunc A() int { return B() }\n\nfunc B() int { return 1 }\n")
	write("README.md", "# gate\n")
	src, _ := filepath.Abs(filepath.Join("testdata", "cuefixture"))
	err := filepath.WalkDir(src, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel, _ := filepath.Rel(src, p)
		b, rerr := os.ReadFile(p)
		if rerr != nil {
			return rerr
		}
		write(rel, string(b))
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return dir
}

func gateIngest(t *testing.T, p *Pipeline, dir string) Report {
	t.Helper()
	rep, err := p.Ingest(context.Background(), core.ServiceRef{ID: "gate", Path: dir})
	if err != nil {
		t.Fatalf("ingest: %v", err)
	}
	return rep
}

func appendTo(t *testing.T, path, s string) {
	t.Helper()
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString(s); err != nil {
		t.Fatal(err)
	}
	_ = f.Close()
}

func cueEdgeCount(t *testing.T, st store.Store) int {
	t.Helper()
	edges, err := st.TypedEdges(context.Background(), "gate")
	if err != nil {
		t.Fatal(err)
	}
	n := 0
	for _, e := range edges {
		if strings.HasPrefix(e.FromFile, "pkg") && strings.HasSuffix(e.FromFile, ".cue") {
			n++
		}
	}
	return n
}

// TestDocOnlyChangeRunsNoTypedPass is RT-104 (design §7.2): the typed Go
// pass runs on the first ingest, not again when only a document changed
// (the report says why and the stored typed edges stand), and again when a
// .go file, go.mod or go.sum changes or a Go file is deleted. A pass that
// did not complete leaves no success record, so the next ingest runs it.
func TestDocOnlyChangeRunsNoTypedPass(t *testing.T) {
	dir := gateFixture(t)
	st := store.NewMemoryStore()
	p := New(connector.NewLocalConnector(), st, Config{})

	gateIngest(t, p, dir)
	if n := p.TypedPassTotal(); n != 1 {
		t.Fatalf("first ingest ran %d typed passes; want 1", n)
	}
	before, _ := st.TypedEdges(context.Background(), "gate")

	appendTo(t, filepath.Join(dir, "README.md"), "more words\n")
	rep := gateIngest(t, p, dir)
	if n := p.TypedPassTotal(); n != 1 {
		t.Fatalf("a README-only change ran the typed pass (total %d)", n)
	}
	if rep.TypedSkipped == "" {
		t.Fatal("a skipped typed pass must be named on the report")
	}
	after, _ := st.TypedEdges(context.Background(), "gate")
	if len(after) != len(before) {
		t.Fatalf("typed edges %d -> %d across a skipped pass; the stored edges must stand", len(before), len(after))
	}

	for i, step := range []func(){
		func() { appendTo(t, filepath.Join(dir, "a.go"), "\nfunc C() int { return A() }\n") },
		func() { appendTo(t, filepath.Join(dir, "go.mod"), "\n// touched\n") },
		func() { _ = os.WriteFile(filepath.Join(dir, "go.sum"), []byte(""), 0o644) },
		func() {
			_ = os.WriteFile(filepath.Join(dir, "b.go"), []byte("package gate\n\nfunc D() {}\n"), 0o644)
		},
		func() { _ = os.Remove(filepath.Join(dir, "b.go")) },
	} {
		want := p.TypedPassTotal() + 1
		step()
		rep := gateIngest(t, p, dir)
		if got := p.TypedPassTotal(); got != want || rep.TypedSkipped != "" {
			t.Fatalf("Go input change %d: typed passes %d (skipped %q), want %d", i, got, rep.TypedSkipped, want)
		}
	}

	// No success record (a halted or failed pass): the next ingest runs it
	// even with nothing changed.
	_ = st.DeleteTypedEdgePackage(context.Background(), "gate", goPassStateKey)
	want := p.TypedPassTotal() + 1
	gateIngest(t, p, dir)
	if got := p.TypedPassTotal(); got != want {
		t.Fatalf("without a success record the pass must run: total %d, want %d", got, want)
	}
	if !typedPassNeeded(nil, false) || typedPassNeeded([]string{"docs/x.md", "a.cue"}, true) || !typedPassNeeded([]string{"sub/go.work"}, true) {
		t.Fatal("typedPassNeeded's input rule is wrong")
	}
}

// TestCUEPassGatedAndSerialized is RT-105 (design §7.2): the CUE pass runs
// only when a .cue input changed (or none succeeded yet), waits for the
// process-wide heavy toolchain slot the typed Go pass holds, and a Go pass
// that runs without it never purges the CUE pass's stored edges.
func TestCUEPassGatedAndSerialized(t *testing.T) {
	dir := gateFixture(t)
	st := store.NewMemoryStore()
	p := New(connector.NewLocalConnector(), st, Config{})

	gateIngest(t, p, dir)
	if n := p.CUEPassTotal(); n != 1 {
		t.Fatalf("first ingest ran %d CUE passes; want 1", n)
	}
	cueEdges := cueEdgeCount(t, st)
	if cueEdges == 0 {
		t.Fatal("the fixture produced no CUE edges")
	}

	appendTo(t, filepath.Join(dir, "README.md"), "x\n")
	rep := gateIngest(t, p, dir)
	if n := p.CUEPassTotal(); n != 1 || rep.CUESkipped == "" {
		t.Fatalf("README-only change: CUE passes %d, skipped %q; want 1 and named", n, rep.CUESkipped)
	}
	// A Go-only change runs the Go pass, whose reconcile must leave the
	// CUE pass's rows alone.
	appendTo(t, filepath.Join(dir, "a.go"), "\nfunc E() {}\n")
	gateIngest(t, p, dir)
	if n := p.CUEPassTotal(); n != 1 {
		t.Fatalf("Go-only change ran the CUE pass (total %d)", n)
	}
	if got := cueEdgeCount(t, st); got != cueEdges {
		t.Fatalf("CUE edges %d -> %d after a Go pass; the Go reconcile purged another pass's rows", cueEdges, got)
	}

	// A .cue change runs it — but not while the heavy slot is held.
	appendTo(t, filepath.Join(dir, "pkgone", "base.cue"), "\n#Later: #Base\n")
	release, err := acquireTypedPassSlot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	var done int32
	finished := make(chan Report, 1)
	go func() {
		r := gateIngest(t, p, dir)
		atomic.StoreInt32(&done, 1)
		finished <- r
	}()
	time.Sleep(300 * time.Millisecond)
	if atomic.LoadInt32(&done) != 0 || p.CUEPassTotal() != 1 {
		release()
		t.Fatal("the CUE pass ran while the heavy toolchain slot was held")
	}
	release()
	select {
	case r := <-finished:
		if p.CUEPassTotal() != 2 || r.CUESkipped != "" {
			t.Fatalf("after the slot freed: CUE passes %d, skipped %q; want 2", p.CUEPassTotal(), r.CUESkipped)
		}
	case <-time.After(60 * time.Second):
		t.Fatal("the CUE pass never ran after the slot was released")
	}
	if !cuePassNeeded([]string{"cue.mod/module.cue"}, true) || cuePassNeeded([]string{"a.go"}, true) || !cuePassNeeded(nil, false) {
		t.Fatal("cuePassNeeded's input rule is wrong")
	}
}
