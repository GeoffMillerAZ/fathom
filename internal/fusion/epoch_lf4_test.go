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

// TestIngest_ExtractorEpoch_WaveLF4ForcesReExtraction pins wave lf4's one bump (59 to 60): a file
// fingerprinted under epoch 59 carried the old extraction, so each must be re-read. A .ts file's
// named import is stored with its binding flag and draws no module edge where its file already
// reaches the module (LB-F411); a .sql file is read by the sql tags dialect, which stores a table's
// columns (LB-F420); an extensionless script is read by the dialect its shebang names (LB-F417); a
// .cpp file owns a module symbol, CppModule (LB-F434); a .scala definition's package is its file's
// package clause (LB-F438). The seeds are salted with "59", the true prior epoch (wave lf3's bump),
// so the test fails if the bump reverts.
func TestIngest_ExtractorEpoch_WaveLF4ForcesReExtraction(t *testing.T) {
	dir := t.TempDir()
	files := map[string]string{
		"src/limits.ts":  "import { clamp } from \"./clamp\";\nexport const LIMIT = clamp(3);\n",
		"src/clamp.ts":   "export function clamp(n: number): number {\n  return n;\n}\n",
		"db/schema.sql":  "CREATE TABLE author (\n  id int PRIMARY KEY,\n  name text\n);\n",
		"bin/deploy":     "#!/bin/bash\nusage() {\n  echo deploy\n}\n",
		"src/clamp.cpp":  "int clamp(int n) {\n  return n < 3 ? n : 3;\n}\n",
		"src/Shop.scala": "package shop\n\nobject Shop {\n  val limit = 3\n}\n",
	}
	st := store.NewMemoryStore()
	ctx := context.Background()
	svc := core.ServiceRef{ID: "epoch-lf4", Path: dir}
	for name, src := range files {
		path := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
		old := sha256.Sum256(append([]byte(src), []byte("\x00fathom-extractor-epoch:59")...))
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
		t.Fatalf("%d file(s) skipped: an epoch-59 fingerprint was honored, ExtractorEpoch was not bumped for wave lf4", rep.Skipped)
	}
	kinds := crSourceKinds(t, st, svc.ID)
	for _, c := range []struct{ rel, want string }{
		{"src/limits.ts", "JSModule"},
		{"db/schema.sql", "SQLColumn"},
		{"bin/deploy", "BashFunction"},
		{"src/clamp.cpp", "CppModule"},
	} {
		if !crHasKind(kinds[c.rel], c.want) {
			t.Errorf("%s re-read as %q, want %s among them", c.rel, kinds[c.rel], c.want)
		}
	}
	obs, err := st.QueryObservations(ctx, svc.ID, "")
	if err != nil {
		t.Fatal(err)
	}
	var shop *core.Observation
	for i := range obs {
		if obs[i].SourceRef == "src/Shop.scala" && obs[i].Symbol == "Shop" {
			shop = &obs[i]
		}
	}
	if shop == nil || shop.Package != "shop" {
		t.Errorf("src/Shop.scala's Shop re-read as %+v, want package \"shop\" from its package clause", shop)
	}
}
