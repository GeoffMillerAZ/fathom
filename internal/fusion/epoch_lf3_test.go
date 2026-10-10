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

// TestIngest_ExtractorEpoch_WaveLF3ForcesReExtraction pins wave lf3's one bump (58 to 59): a file
// fingerprinted under epoch 58 carried the old extraction, so each must be re-read. A .ts file owns
// its top-level code in `<module>` (JSModule) and a module-level const is a JSConst (LB-F381,
// LB-F382); a .rs file is read by the Rust tags dialect, which names a const a RustConst (LB-F388);
// a .sh file is read by the bash tags dialect, which names an exported variable a BashVariable
// (LB-F391); a .scala file is read by the Scala tags dialect, which names a member val a ScalaVal
// (LB-F399). The seeds are salted with "58", the true prior epoch (wave h3's bump), so the test
// fails if the bump reverts.
func TestIngest_ExtractorEpoch_WaveLF3ForcesReExtraction(t *testing.T) {
	dir := t.TempDir()
	files := map[string]string{
		"src/limits.ts":  "export const LIMIT = 3;\nexport function clamp(n: number): number {\n  return Math.min(n, LIMIT);\n}\n",
		"src/lib.rs":     "pub const MAX: u32 = 3;\npub fn clamp(n: u32) -> u32 {\n    n.min(MAX)\n}\n",
		"scripts/run.sh": "export APP_ENV=prod\nrun() {\n  echo \"$APP_ENV\"\n}\n",
		"src/Shop.scala": "object Shop {\n  val limit = 3\n  def clamp(n: Int): Int = math.min(n, limit)\n}\n",
	}
	st := store.NewMemoryStore()
	ctx := context.Background()
	svc := core.ServiceRef{ID: "epoch-lf3", Path: dir}
	for name, src := range files {
		path := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
		old := sha256.Sum256(append([]byte(src), []byte("\x00fathom-extractor-epoch:58")...))
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
		t.Fatalf("%d file(s) skipped: an epoch-58 fingerprint was honored, ExtractorEpoch was not bumped for wave lf3", rep.Skipped)
	}
	kinds := crSourceKinds(t, st, svc.ID)
	for _, c := range []struct{ rel, want string }{
		{"src/limits.ts", "JSModule"},
		{"src/limits.ts", "JSConst"},
		{"src/lib.rs", "RustConst"},
		{"scripts/run.sh", "BashVariable"},
		{"src/Shop.scala", "ScalaVal"},
	} {
		if !crHasKind(kinds[c.rel], c.want) {
			t.Errorf("%s re-read as %q, want %s among them", c.rel, kinds[c.rel], c.want)
		}
	}
}
