package testevidence

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/store"
)

func markerFixture(t *testing.T, root, rel, target string) {
	t.Helper()
	mustWrite(t, filepath.Join(root, filepath.FromSlash(rel)), "package fixture\n\n// certifies: invariant "+target+" (asserts)\nfunc TestMarker(t *T) {}\n")
}

func TestIngestCertifyMarkers_UsesCorpusScopeAndReplacesExcludedDeclarations(t *testing.T) {
	ctx := context.Background()
	st := store.NewMemoryStore()
	t.Cleanup(func() { _ = st.Close() })
	root := t.TempDir()
	markerFixture(t, root, "src/maintained_test.go", "I-MAINTAINED")
	markerFixture(t, root, ".cache/worklanes/ignored_test.go", "I-CACHE")
	markerFixture(t, root, ".artifacts/ignored_test.go", "I-ARTIFACT")
	mustWrite(t, filepath.Join(root, ".fathomignore"), ".artifacts/\n")

	check := func(want int, targets ...string) {
		t.Helper()
		report, err := IngestCertifyMarkers(ctx, st, "svc", "wm", root)
		if err != nil {
			t.Fatal(err)
		}
		if report.Declared != want {
			t.Fatalf("Declared = %d, want %d", report.Declared, want)
		}
		decls, err := st.CertifyDeclarationsForService(ctx, "svc")
		if err != nil {
			t.Fatal(err)
		}
		if len(decls) != want {
			t.Fatalf("stored = %d, want %d", len(decls), want)
		}
		got := map[string]bool{}
		for _, d := range decls {
			got[d.TargetID] = true
		}
		for _, target := range targets {
			if !got[target] {
				t.Fatalf("target %q absent from %+v", target, decls)
			}
		}
		if got["I-ARTIFACT"] {
			t.Fatalf("ignored artifact declared: %+v", decls)
		}
	}
	check(1, "I-MAINTAINED")
	mustWrite(t, filepath.Join(root, ".fathom", "config.toml"), "[connector]\ninclude = [\".cache\"]\n")
	check(2, "I-MAINTAINED", "I-CACHE")
	mustWrite(t, filepath.Join(root, ".fathom", "config.toml"), "[connector]\ninclude = []\n")
	check(1, "I-MAINTAINED") // Whole-service replacement prunes the old cache declaration.
}

func TestIngestCertifyMarkers_RespectsConfiguredGitCorpus(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git unavailable")
	}
	ctx := context.Background()
	st := store.NewMemoryStore()
	t.Cleanup(func() { _ = st.Close() })
	root := t.TempDir()
	if out, err := exec.Command("git", "-C", root, "init", "-q").CombinedOutput(); err != nil {
		t.Fatalf("git init: %v: %s", err, out)
	}
	mustWrite(t, filepath.Join(root, ".gitignore"), "ignored/\n")
	mustWrite(t, filepath.Join(root, ".fathom", "config.toml"), "[connector]\nrespect_gitignore = true\n")
	markerFixture(t, root, "src/maintained_test.go", "I-GIT-MAINTAINED")
	markerFixture(t, root, "ignored/ignored_test.go", "I-GIT-IGNORED")
	report, err := IngestCertifyMarkers(ctx, st, "svc", "wm", root)
	if err != nil {
		t.Fatal(err)
	}
	if report.Declared != 1 {
		t.Fatalf("Git-mode declarations = %d, want 1", report.Declared)
	}
	decls, err := st.CertifyDeclarationsForService(ctx, "svc")
	if err != nil {
		t.Fatal(err)
	}
	if len(decls) != 1 || decls[0].TargetID != "I-GIT-MAINTAINED" {
		t.Fatalf("Git-mode declarations = %+v", decls)
	}
	if _, err := os.Stat(filepath.Join(root, "ignored", "ignored_test.go")); err != nil {
		t.Fatal(err)
	}
}
