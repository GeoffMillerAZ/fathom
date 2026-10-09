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

// TestIngest_ExtractorEpoch_WaveLF1ForcesReExtraction pins wave lf1's one bump (52 to 53): a file
// fingerprinted under epoch 52 carried the old extraction, so each must be re-read. A C# field is an
// Owner.name symbol (LB-F289, O594), `.java` is served by the Java dialect, whose `new X()` is a call
// to X (LB-F302, LB-F303, O611), a C struct field is a CField (O367, LB-F291), and C++ is served by
// its dialect, which names a data member after its class (LB-F117, LB-F295). The seeds are salted
// with "52", the true prior epoch, so the test fails if the bump reverts.
func TestIngest_ExtractorEpoch_WaveLF1ForcesReExtraction(t *testing.T) {
	dir := t.TempDir()
	files := map[string]string{
		"Order.cs":  "namespace App {\n  public class Order {\n    private int count;\n  }\n}\n",
		"Run.java":  "package app;\n\nclass Registry {}\n\nclass Run {\n  void run() {\n    Registry r = new Registry();\n  }\n}\n",
		"node.c":    "struct node {\n  int len;\n};\n\nint size(struct node *n) { return n->len; }\n",
		"shape.cpp": "namespace geo {\nclass Shape {\n public:\n  int sides;\n};\n}\n",
	}
	st := store.NewMemoryStore()
	ctx := context.Background()
	svc := core.ServiceRef{ID: "epoch-lf1", Path: dir}
	for name, src := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
		old := sha256.Sum256(append([]byte(src), []byte("\x00fathom-extractor-epoch:52")...))
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
		t.Fatalf("%d file(s) skipped: an epoch-52 fingerprint was honored, ExtractorEpoch was not bumped for wave lf1", rep.Skipped)
	}
	obs, err := st.QueryObservations(ctx, svc.ID, "")
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]core.Observation{}
	for _, o := range obs {
		got[o.SourceRef+"|"+string(o.SourceKind)+"|"+o.Symbol] = o
	}
	for _, want := range []string{
		"Order.cs|CSharpField|Order.count",
		"node.c|CField|node.len",
		"shape.cpp|CppField|geo.Shape.sides",
	} {
		if _, ok := got[want]; !ok {
			t.Errorf("re-extraction has no %q", want)
		}
	}
	run, ok := got["Run.java|JavaMethod|Run.run"]
	if !ok || !slices.Contains(run.Calls, "Registry") {
		t.Errorf("Run.java Run.run calls = %v (found %v), want Registry (a constructor call, the Java dialect's)", run.Calls, ok)
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
