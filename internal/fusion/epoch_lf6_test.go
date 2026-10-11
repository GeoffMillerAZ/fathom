package fusion

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/store"
)

// TestIngest_ExtractorEpoch_WaveLF6ForcesReExtraction pins wave lf6's one bump (61 to 62): a file
// fingerprinted under epoch 61 carried the old extraction, so each must be re-read. A .js file's
// module-level let and var bindings are JSConst definitions (owner ruling M1, LB-F470, O749); an
// extensionless script the shebang rule dispatches is its dialect's file in the file-resolve pass
// (O793), which on an unchanged service runs only once a re-read changes its inputs. The seeds are
// salted with "61", the true prior epoch (wave lf5's bump), so the test fails if the bump reverts.
func TestIngest_ExtractorEpoch_WaveLF6ForcesReExtraction(t *testing.T) {
	dir := t.TempDir()
	files := map[string]string{
		"web/app.js":      "let count = 0;\nvar server = make();\n",
		"scripts/report":  "#!/usr/bin/env python3\nfrom pkg.util import fmt_row\n\ndef render(rows):\n    return fmt_row(rows)\n",
		"pkg/__init__.py": "",
		"pkg/util.py":     "def fmt_row(r):\n    return r\n",
	}
	st := store.NewMemoryStore()
	ctx := context.Background()
	svc := core.ServiceRef{ID: "epoch-lf6", Path: dir}
	for name, src := range files {
		path := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
		old := sha256.Sum256(append([]byte(src), []byte("\x00fathom-extractor-epoch:61")...))
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
		t.Fatalf("%d file(s) skipped: an epoch-61 fingerprint was honored, ExtractorEpoch was not bumped for wave lf6", rep.Skipped)
	}
	kinds := crSourceKinds(t, st, svc.ID)
	if !crHasKind(kinds["web/app.js"], "JSConst") {
		t.Errorf("web/app.js re-read as %q, want JSConst among them (its module-level let and var)", kinds["web/app.js"])
	}
	edges, err := st.TypedEdges(ctx, svc.ID)
	if err != nil {
		t.Fatal(err)
	}
	want := "scripts/report|epoch-lf6::scripts/report.render -> epoch-lf6::pkg/util.fmt_row"
	found := false
	for _, e := range edges {
		if e.Kind == core.EdgeFileResolved && e.FromFile+"|"+e.FromSymbol+" -> "+e.ToSymbol == want {
			found = true
		}
	}
	if !found {
		t.Errorf("the extensionless python3 script drew no file-resolved edge to pkg/util.py's fmt_row (want %s)", want)
	}
}
