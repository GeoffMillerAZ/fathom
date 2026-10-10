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

// TestIngest_ExtractorEpoch_WaveLF5ForcesReExtraction pins wave lf5's one bump (60 to 61): a file
// fingerprinted under epoch 60 carried the old extraction, so each must be re-read. An .html file
// owns a module symbol, HTMLDocument, which calls the ids the page references (LB-F441); a .css file
// owns a module symbol, CSSModule, with its var() references (LB-F459); a .scss file is read by the
// scss tags dialect, newly dispatched (LB-F465). The seeds are salted with "60", the true prior epoch
// (wave lf4's bump), so the test fails if the bump reverts.
func TestIngest_ExtractorEpoch_WaveLF5ForcesReExtraction(t *testing.T) {
	dir := t.TempDir()
	files := map[string]string{
		"web/index.html":    "<!doctype html>\n<html><body>\n<label for=\"q\">Search</label>\n<input id=\"q\">\n</body></html>\n",
		"web/site.css":      ":root {\n  --accent: #333;\n}\n.btn {\n  color: var(--accent);\n}\n",
		"web/_buttons.scss": "@mixin pill($r) {\n  border-radius: $r;\n}\n.btn {\n  @include pill(4px);\n}\n",
	}
	st := store.NewMemoryStore()
	ctx := context.Background()
	svc := core.ServiceRef{ID: "epoch-lf5", Path: dir}
	for name, src := range files {
		path := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
		old := sha256.Sum256(append([]byte(src), []byte("\x00fathom-extractor-epoch:60")...))
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
		t.Fatalf("%d file(s) skipped: an epoch-60 fingerprint was honored, ExtractorEpoch was not bumped for wave lf5", rep.Skipped)
	}
	kinds := crSourceKinds(t, st, svc.ID)
	for _, c := range []struct{ rel, want string }{
		{"web/index.html", "HTMLDocument"},
		{"web/site.css", "CSSModule"},
		{"web/_buttons.scss", "SCSSMixin"},
	} {
		if !crHasKind(kinds[c.rel], c.want) {
			t.Errorf("%s re-read as %q, want %s among them", c.rel, kinds[c.rel], c.want)
		}
	}
}
