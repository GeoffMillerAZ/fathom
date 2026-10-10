package fusion

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/analysis/serviceref"
	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/store"
)

// TestIngest_ExtractorEpoch_WaveLF2ForcesReExtraction pins wave lf2's one bump (55 to 56): a file
// fingerprinted under epoch 55 carried the old extraction, so each must be re-read. A Python file
// gets its per-file module symbol (K18, LB-F346..LB-F348), and a Symfony config/routes YAML's
// routing.controllers resource is a recursive directory Mount (`/src/Controller/**`, LB-F351), so a
// controller in a subfolder composes under its prefix. The seeds are salted with "55", the true
// prior epoch, so the test fails if the bump reverts.
func TestIngest_ExtractorEpoch_WaveLF2ForcesReExtraction(t *testing.T) {
	dir := t.TempDir()
	files := map[string]string{
		"app.py":             "import os\n\nsetup(os.getcwd())\n\n\ndef setup(path):\n    return path\n",
		"config/routes.yaml": "controllers:\n    resource: routing.controllers\n    prefix: /{_locale}\n",
		"src/Controller/Admin/PostController.php": "<?php\n\nnamespace App\\Controller\\Admin;\n\nuse Symfony\\Component\\Routing\\Attribute\\Route;\n\n" +
			"#[Route('/admin/post')]\nfinal class PostController\n{\n    #[Route('/new', name: 'admin_post_new', methods: ['GET'])]\n    public function new(): void {}\n}\n",
	}
	st := store.NewMemoryStore()
	ctx := context.Background()
	svc := core.ServiceRef{ID: "epoch-lf2", Path: dir}
	for name, src := range files {
		path := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
		old := sha256.Sum256(append([]byte(src), []byte("\x00fathom-extractor-epoch:55")...))
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
		t.Fatalf("%d file(s) skipped: an epoch-55 fingerprint was honored, ExtractorEpoch was not bumped for wave lf2", rep.Skipped)
	}
	obs, err := st.QueryObservations(ctx, svc.ID, "")
	if err != nil {
		t.Fatal(err)
	}
	module := false
	var kinds []string
	for _, o := range obs {
		if o.SourceRef == "app.py" {
			kinds = append(kinds, string(o.SourceKind)+" "+o.Symbol)
			if o.SourceKind == "PythonModule" {
				module = true
			}
		}
	}
	if !module {
		slices.Sort(kinds)
		t.Errorf("re-extraction of app.py has no PythonModule symbol; app.py observations: %v", kinds)
	}
	refs, complete, err := st.ServiceRefsFor(ctx, svc.ID)
	if err != nil || !complete {
		t.Fatalf("ServiceRefsFor: complete=%v err=%v", complete, err)
	}
	routes := map[string]bool{}
	for _, r := range serviceref.RoutesFromRows(refs.Routes) {
		routes[r.Method+" "+r.Path] = true
	}
	if want := "GET /{_locale}/admin/post/new"; !routes[want] {
		keys := make([]string, 0, len(routes))
		for k := range routes {
			keys = append(keys, k)
		}
		slices.Sort(keys)
		t.Errorf("re-extraction has no composed route %q; routes: %v", want, keys)
	}
}
