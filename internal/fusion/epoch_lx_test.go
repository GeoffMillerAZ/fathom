package fusion

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/store"
)

// TestIngest_ExtractorEpoch_WaveLXForcesReExtraction pins wave lx's one bump (50 to 51):
// a file fingerprinted under epoch 50 carried the old extraction, so each must be re-read.
// Python's class attributes are Owner.name symbols (LB-F241, LB-F242), a Dart callable with
// a comment before its => body is kept (LB-F247), a Rust macro_rules! item is a RustMacro
// (LB-F251), and a Ruby bare identifier no local binds is a call (LB-F254). The seeds are
// salted with "50", the true prior epoch, so the test fails if the bump reverts.
func TestIngest_ExtractorEpoch_WaveLXForcesReExtraction(t *testing.T) {
	dir := t.TempDir()
	files := map[string]string{
		"m.py":   "class C:\n    attr = 1\n\n    def run(self):\n        return self.attr\n",
		"c.dart": "class Channel {\n  int size()\n      // One comment.\n      => count();\n  int count() => 1;\n}\n",
		"m.rs":   "macro_rules! twice {\n    ($x:expr) => { $x * 2 };\n}\nfn f() -> i32 { twice!(1) }\n",
		"s.rb":   "class S\n  def helper\n  end\n\n  def run\n    helper\n  end\nend\n",
	}
	st := store.NewMemoryStore()
	ctx := context.Background()
	svc := core.ServiceRef{ID: "epoch-lx", Path: dir}
	for name, src := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
		old := sha256.Sum256(append([]byte(src), []byte("\x00fathom-extractor-epoch:50")...))
		if err := st.SetFingerprint(ctx, svc.ID, name, hex.EncodeToString(old[:])); err != nil {
			t.Fatal(err)
		}
	}
	p := New(connector.NewLocalConnector(), st, Config{Concurrency: 1})
	t.Cleanup(p.Close)
	rep, err := p.Ingest(ctx, svc)
	if err != nil {
		t.Fatal(err)
	}
	if rep.Skipped != 0 {
		t.Fatalf("%d file(s) skipped: an epoch-50 fingerprint was honored, ExtractorEpoch was not bumped for wave lx", rep.Skipped)
	}
	obs, err := st.QueryObservations(ctx, svc.ID, "")
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]core.Observation{}
	for _, o := range obs {
		got[o.SourceRef+" "+o.Symbol] = o
	}
	for _, want := range []string{"m.py C.attr", "c.dart Channel.size", "m.rs twice"} {
		if _, ok := got[want]; !ok {
			t.Errorf("re-extraction has no %q", want)
		}
	}
	if o := got["m.rs twice"]; o.SourceKind != "RustMacro" {
		t.Errorf("m.rs twice: kind %q, want RustMacro", o.SourceKind)
	}
	if run, ok := got["s.rb S.run"]; !ok || !slices.Contains(run.Calls, "helper") {
		t.Errorf("s.rb S.run calls = %v (found %v), want helper (a bare identifier no local binds is a call)", run.Calls, ok)
	}
	if t.Failed() {
		keys := make([]string, 0, len(got))
		for k := range got {
			keys = append(keys, k)
		}
		slices.Sort(keys)
		t.Logf("observations: %v", keys)
	}
}
