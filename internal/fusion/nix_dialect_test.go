package fusion_test

// nix_dialect_test.go — LB-Q-nix (design §6, §13): a Nix path literal (`import ./lib`, `callPackage ./x.nix`,
// an `imports` element) resolves through the file-symbol strategy (a directory to its default.nix), so a
// function a file applies that an imported file defines resolves to it; ingest dispatches every .nix file
// through the tags-engine loop (no hand-wired line: the manifest claims the extension). A name imported only
// by an attribute selection (`util.quadruple`) binds like any whole-file import: the file brings every
// top-level name into scope.

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/fusion"
	"github.com/geoffmilleraz/fathom/internal/store"
)

func TestResolveFileEdges_NixImports(t *testing.T) {
	dir, err := filepath.Abs(filepath.Join("testdata", "nixfixture"))
	if err != nil {
		t.Fatal(err)
	}
	files, symbols := loadFileResolveFixture(t, dir, "nix", fixtureFiles(t, dir, ".nix"))
	edges, report := fusion.ResolveFileEdges(context.Background(), "nix", dir, files, symbols)
	if report.Skipped {
		t.Fatalf("skipped: %s", report.SkipReason)
	}
	want := []string{
		"default.nix|nine -> util.nix|triple",
		"default.nix|six -> lib/default.nix|double",
		"default.nix|twelve -> util.nix|quadruple",
	}
	got := namedEdges(edges, symbols)
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("edges:\n  %s\nwant:\n  %s", strings.Join(got, "\n  "), strings.Join(want, "\n  "))
	}
}

func TestIngest_NixDialectIsDispatched(t *testing.T) {
	dir := t.TempDir()
	for rel, src := range map[string]string{
		"default.nix":   "{ pkgs }:\n{\n  app = pkgs.callPackage ./app.nix { };\n  twice = x: x * 2;\n}\n",
		"app.nix":       "{ stdenv }:\nstdenv.mkDerivation { pname = \"app\"; }\n",
		"flake.nix":     "{\n  description = \"d\";\n  outputs = { self }: { };\n}\n",
		"notes.nix.txt": "{ ignored = 1; }\n",
	} {
		p := filepath.Join(dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for _, p := range []string{"default.nix", "a/b/flake.nix", "shell.nix"} {
		if got := connector.LanguageForPath(p); got != "nix" {
			t.Errorf("LanguageForPath(%q) = %q, want nix", p, got)
		}
	}
	st := store.NewMemoryStore()
	svc := core.ServiceRef{ID: "nix-svc", Path: dir}
	p := fusion.New(connector.NewLocalConnector(), st, defaultCfg())
	t.Cleanup(p.Close)
	if _, err := p.Ingest(context.Background(), svc); err != nil {
		t.Fatalf("Ingest: %v", err)
	}
	obs, err := st.QueryObservations(context.Background(), svc.ID, "")
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, o := range obs {
		got = append(got, o.SourceRef+"|"+string(o.SourceKind)+"|"+o.Symbol)
	}
	slices.Sort(got)
	want := []string{
		"app.nix|NixParam|stdenv",
		"default.nix|NixFunction|twice",
		"default.nix|NixPackage|app",
		"default.nix|NixParam|pkgs",
		"flake.nix|NixAttribute|description",
		"flake.nix|NixFunction|outputs",
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("observations:\n  %s\nwant:\n  %s", strings.Join(got, "\n  "), strings.Join(want, "\n  "))
	}
}
