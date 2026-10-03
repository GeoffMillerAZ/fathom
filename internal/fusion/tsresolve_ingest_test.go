package fusion

// LB-F223..LB-F228 through ingest's file-resolved pass: a TypeScript
// workspace's cross-package edges reach the store, and an edit to a
// tsconfig.json or a package.json alone re-resolves them on the next ingest.

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/store"
)

// copyFixture copies testdata/<name> into a fresh directory (ingest never
// discovers files under a directory called testdata).
func copyFixture(t *testing.T, name string) string {
	t.Helper()
	src, err := filepath.Abs(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	dst := t.TempDir()
	files := map[string]string{}
	err = filepath.WalkDir(src, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		b, rerr := os.ReadFile(p)
		if rerr != nil {
			return rerr
		}
		rel, _ := filepath.Rel(src, p)
		files[rel] = string(b)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	writeCorpus(t, dst, files)
	return dst
}

// fileEdgesTo is the stored file-resolved edges as "fromFile|fromName ->
// toFile|toName", the symbols' files and names looked up from the store.
func fileEdgesTo(t *testing.T, st store.Store, svc string) []string {
	t.Helper()
	ctx := context.Background()
	obs, err := st.QueryObservations(ctx, svc, "")
	if err != nil {
		t.Fatal(err)
	}
	where := map[string]string{}
	for _, o := range obs {
		n := o.ToSymbolNode()
		where[n.Key()] = n.FilePath + "|" + n.Name
	}
	all, err := st.TypedEdges(ctx, svc)
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, e := range all {
		if e.Kind == core.EdgeFileResolved {
			out = append(out, where[e.FromSymbol]+" -> "+where[e.ToSymbol])
		}
	}
	sort.Strings(out)
	return out
}

func hasEdge(edges []string, want string) bool {
	for _, e := range edges {
		if e == want {
			return true
		}
	}
	return false
}

func edit(t *testing.T, dir, rel, old, new string) {
	t.Helper()
	p := filepath.Join(dir, filepath.FromSlash(rel))
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), old) {
		t.Fatalf("%s: %q not found", rel, old)
	}
	if err := os.WriteFile(p, []byte(strings.Replace(string(b), old, new, 1)), 0o644); err != nil {
		t.Fatal(err)
	}
}

// TestIngest_TSWorkspaceEdgesReResolveOnManifestEdit: ingest stores the
// edges a workspace package's exports, an outDir's source, a `#` import and
// tsconfig paths resolve (LB-F223..LB-F227); an edit to a package.json alone,
// then to a tsconfig.json alone, re-runs the file-resolved pass and moves the
// edge (LB-F228), where the pass used to wait for a TS/JS file to change.
func TestIngest_TSWorkspaceEdgesReResolveOnManifestEdit(t *testing.T) {
	ctx := context.Background()

	t.Run("package.json", func(t *testing.T) {
		dir := copyFixture(t, "tsnpmws")
		writeCorpus(t, dir, map[string]string{"packages/core/alt/index.ts": "export function coreMain(): number {\n  return 0;\n}\n"})
		st := store.NewMemoryStore()
		p := New(connector.NewLocalConnector(), st, Config{})
		t.Cleanup(p.Close)
		svc := core.ServiceRef{ID: "tsws", Path: dir}
		if _, err := p.Ingest(ctx, svc); err != nil {
			t.Fatal(err)
		}
		edges := fileEdgesTo(t, st, svc.ID)
		for _, want := range []string{
			"packages/app/src/main.ts|main -> packages/core/src/index.ts|coreMain",
			"packages/app/src/main.ts|main -> packages/core/src/feature.ts|featureFn",
			"packages/app/src/main.ts|main -> packages/core/src/mode-esm.ts|mode",
			"packages/app/src/main.ts|main -> packages/core/src/locales/fr.ts|frLocale",
			"packages/app/src/main.ts|main -> packages/app/src/local.ts|local",
			"packages/app/src/main.ts|main -> packages/app/src/dir/index.ts|dirFn",
			"packages/app/src/legacy.cts|legacy -> packages/core/src/mode-cjs.ts|mode",
			// import localMod = require('./local'): a whole-module binding
			"packages/app/src/legacy.cts|legacy -> packages/app/src/local.ts|local",
			"packages/core/src/index.ts|coreMain -> packages/core/src/internal/secret.ts|secret",
			"packages/core/src/index.ts|coreMain -> packages/core/src/feature.ts|featureFn",
		} {
			if !hasEdge(edges, want) {
				t.Errorf("missing edge %s\nstored: %q", want, edges)
			}
		}
		if hasEdge(edges, "packages/app/src/main.ts|main -> packages/core/src/internal/secret.ts|secret") {
			t.Errorf("an export excluded by null resolved: %q", edges)
		}

		edit(t, dir, "packages/core/package.json", `"source": "./src/index.ts"`, `"source": "./alt/index.ts"`)
		rep, err := p.Ingest(ctx, svc)
		if err != nil {
			t.Fatal(err)
		}
		if rep.FileResolveSkipped != "" {
			t.Fatalf("an edit to package.json alone skipped the file-resolved pass: %s", rep.FileResolveSkipped)
		}
		edges = fileEdgesTo(t, st, svc.ID)
		if !hasEdge(edges, "packages/app/src/main.ts|main -> packages/core/alt/index.ts|coreMain") ||
			hasEdge(edges, "packages/app/src/main.ts|main -> packages/core/src/index.ts|coreMain") {
			t.Fatalf("after the package.json edit main should reach alt/index.ts's coreMain only: %q", edges)
		}
	})

	t.Run("tsconfig.json", func(t *testing.T) {
		dir := copyFixture(t, "tspaths")
		writeCorpus(t, dir, map[string]string{"packages/core/alt/index.ts": "export function coreFn(): number {\n  return 0;\n}\n"})
		st := store.NewMemoryStore()
		p := New(connector.NewLocalConnector(), st, Config{})
		t.Cleanup(p.Close)
		svc := core.ServiceRef{ID: "tspaths", Path: dir}
		if _, err := p.Ingest(ctx, svc); err != nil {
			t.Fatal(err)
		}
		edges := fileEdgesTo(t, st, svc.ID)
		for _, want := range []string{
			"packages/app/src/main.ts|main -> packages/core/src/index.ts|coreFn",
			"packages/app/src/main.ts|main -> packages/core/src/math.ts|add",
			"packages/app/src/main.ts|main -> packages/special/lib/entry.ts|special",
			"apps/web/src/page.ts|page -> shared/util.ts|util",
			"apps/web/src/page.ts|page -> apps/web/local/thing.ts|thing",
			"baseonly/src/main.ts|run -> baseonly/src/lib/math.ts|mul",
			"rootdirs/src/view.ts|view -> rootdirs/templates/template.ts|template",
			"solution/src/a.ts|a -> solution/src/lib/x.ts|libFn",
		} {
			if !hasEdge(edges, want) {
				t.Errorf("missing edge %s\nstored: %q", want, edges)
			}
		}

		edit(t, dir, "tsconfig.json", `"strict": true,`, `"strict": true, "paths": { "@app/*": ["packages/*/alt"] },`)
		rep, err := p.Ingest(ctx, svc)
		if err != nil {
			t.Fatal(err)
		}
		if rep.FileResolveSkipped != "" {
			t.Fatalf("an edit to tsconfig.json alone skipped the file-resolved pass: %s", rep.FileResolveSkipped)
		}
		edges = fileEdgesTo(t, st, svc.ID)
		if !hasEdge(edges, "packages/app/src/main.ts|main -> packages/core/alt/index.ts|coreFn") {
			t.Fatalf("after the tsconfig.json edit main should reach alt/index.ts's coreFn: %q", edges)
		}
	})
}

// TestIngest_ExtractorEpoch_TSResolutionForcesReResolution pins epoch 51
// (wave lh, item ts, LB-F223..LB-F228; the worker's 50 renumbered by the
// integrator to origin/main's + 1): a .ts or .js file fingerprinted at epoch
// 50 kept the edges the relative-only rule drew, and the file-resolved pass
// waits for an input to change, so the bump "50"->"51" must re-read every
// file and re-resolve it with the workspace in view. The seeded fingerprints
// are salted with "50", the TRUE prior epoch (the XT-5 rule in
// epoch_test.go), so this fails if the bump reverts.
func TestIngest_ExtractorEpoch_TSResolutionForcesReResolution(t *testing.T) {
	ctx := context.Background()
	dir := copyFixture(t, "tsnpmws")
	st := store.NewMemoryStore()
	svc := core.ServiceRef{ID: "epoch51-svc", Path: dir}
	err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		b, rerr := os.ReadFile(p)
		if rerr != nil {
			return rerr
		}
		rel, _ := filepath.Rel(dir, p)
		old := sha256.Sum256(append(append([]byte{}, b...), []byte("\x00fathom-extractor-epoch:50")...))
		return st.SetFingerprint(ctx, svc.ID, filepath.ToSlash(rel), hex.EncodeToString(old[:]))
	})
	if err != nil {
		t.Fatal(err)
	}
	p := New(connector.NewLocalConnector(), st, Config{})
	t.Cleanup(p.Close)
	rep, err := p.Ingest(ctx, svc)
	if err != nil {
		t.Fatal(err)
	}
	if rep.Skipped != 0 {
		t.Fatalf("%d file(s) skipped (a pre-epoch-51 fingerprint was honored): ExtractorEpoch was not bumped for the TS resolution", rep.Skipped)
	}
	edges := fileEdgesTo(t, st, svc.ID)
	for _, want := range []string{
		"packages/app/src/main.ts|main -> packages/core/src/index.ts|coreMain",
		"packages/core/src/index.ts|coreMain -> packages/core/src/internal/secret.ts|secret",
	} {
		if !hasEdge(edges, want) {
			t.Errorf("re-resolution did not surface %s: %q", want, edges)
		}
	}
}
