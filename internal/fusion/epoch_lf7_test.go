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

// TestIngest_ExtractorEpoch_WaveLF7ForcesReExtraction pins wave lf7's one bump (62 to 63): a file
// fingerprinted under epoch 62 carried the old extraction, so each must be re-read. A .ts file's
// module-level let and var bindings are JSConst definitions (owner ruling S3a, LB-F486); a bash
// script's top level is its module symbol, its source and . statements imports (LB-Q-bash L3, owner
// ruling S1); .vue, .svelte and .astro files are component dialects (LB-F536, LB-F544) where each was
// fingerprinted as an unsupported, disclosed file. The seeds are salted with "62", the true prior
// epoch (wave lf6's bump), so the test fails if the bump reverts.
func TestIngest_ExtractorEpoch_WaveLF7ForcesReExtraction(t *testing.T) {
	dir := t.TempDir()
	files := map[string]string{
		"web/state.ts":        "let count = 0;\nvar server = make();\n",
		"bin/run.sh":          "#!/usr/bin/env bash\n. ./lib.sh\nhelper\n",
		"bin/lib.sh":          "helper() {\n  echo hi\n}\n",
		"ui/Button.vue":       "<template>\n  <button>ok</button>\n</template>\n<script setup lang=\"ts\">\nconst label = 'ok'\n</script>\n",
		"ui/Card.svelte":      "<script>\n  export let title = '';\n</script>\n<h1>{title}</h1>\n",
		"site/pages/a.astro":  "---\nconst title = 'a';\n---\n<h1>{title}</h1>\n",
		"site/pages/index.md": "# index\n",
	}
	st := store.NewMemoryStore()
	ctx := context.Background()
	svc := core.ServiceRef{ID: "epoch-lf7", Path: dir}
	for name, src := range files {
		path := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
		if filepath.Ext(name) == ".md" {
			continue // a control: never seeded, always read
		}
		old := sha256.Sum256(append([]byte(src), []byte("\x00fathom-extractor-epoch:62")...))
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
		t.Fatalf("%d file(s) skipped: an epoch-62 fingerprint was honored, ExtractorEpoch was not bumped for wave lf7", rep.Skipped)
	}
	kinds := crSourceKinds(t, st, svc.ID)
	for file, kind := range map[string]string{
		"web/state.ts":       "JSConst",         // S3a: a module-level let or var
		"bin/run.sh":         "BashModule",      // the script's top level
		"ui/Button.vue":      "VueComponent",    // the component a file is
		"ui/Card.svelte":     "SvelteComponent", // likewise
		"site/pages/a.astro": "AstroComponent",  // likewise
	} {
		if !crHasKind(kinds[file], kind) {
			t.Errorf("%s re-read as %q, want %s among them", file, kinds[file], kind)
		}
	}
}
