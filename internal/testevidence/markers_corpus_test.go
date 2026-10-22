package testevidence

import (
	"context"
	"errors"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/store"
	"github.com/geoffmilleraz/fathom/internal/tenancy"
)

func TestCertifyDiscoveryHonorsCorpusExclusions(t *testing.T) {
	home := t.TempDir()
	t.Setenv("FATHOM_HOME", home)
	t.Setenv(connector.ServiceIgnoreDirEnvVar, filepath.Join(home, "ignore.d"))
	mustWrite(t, filepath.Join(home, "ignore"), "user-excluded/\n")
	mustWrite(t, filepath.Join(home, "ignore.d", "svc"), "service-excluded/\n")
	root := t.TempDir()
	cmd := exec.Command("git", "init", "-q", root)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git init: %v: %s", err, out)
	}
	mustWrite(t, filepath.Join(root, ".fathom", "config.toml"), "[connector]\nrespect_gitignore = true\n")
	mustWrite(t, filepath.Join(root, ".fathomignore"), "excluded/\n")
	mustWrite(t, filepath.Join(root, ".gitignore"), "gitignored/\n")
	for _, rel := range []string{"included_test.go", ".cache/cache_test.go", "excluded/hidden_test.go", "gitignored/hidden_test.go", "user-excluded/hidden_test.go", "service-excluded/hidden_test.go"} {
		mustWrite(t, filepath.Join(root, rel), "package cart\n// certifies: invariant I1\nfunc TestCart() {}\n")
	}
	mustWrite(t, filepath.Join(root, "plain.go"), "package cart\nfunc Cart() {}\n")
	ctx := context.Background()
	refs, err := connector.NewLocalConnector().Discover(ctx, core.ServiceRef{ID: "svc", Path: root})
	if err != nil {
		t.Fatal(err)
	}
	want := 0
	for _, ref := range refs {
		if strings.HasSuffix(ref.Path, ".go") {
			want++
		}
	}
	st := store.NewMemoryStore()
	t.Cleanup(func() { _ = st.Close() })
	r, err := IngestCertifyMarkers(ctx, st, "svc", "w1", root)
	if err != nil {
		t.Fatal(err)
	}
	if want != 2 || r.FilesScanned != want || r.FilesParsed != 1 || r.Declared != 1 {
		t.Errorf("discovered Go=%d report=%+v; want scanned=2 parsed=1 declared=1", want, r)
	}
	decls, err := st.CertifyDeclarationsForService(ctx, "svc")
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range decls {
		if d.File != "included_test.go" {
			t.Errorf("excluded declaration: %+v", d)
		}
	}
}

func TestCertifyScanStopsWithoutReplacingDeclarations(t *testing.T) {
	for _, mode := range []string{"cancelled", "refused"} {
		t.Run(mode, func(t *testing.T) {
			root := t.TempDir()
			mustWrite(t, filepath.Join(root, "x.go"), "package x\n")
			st := store.NewMemoryStore()
			t.Cleanup(func() { _ = st.Close() })
			if err := st.PutCertifyDeclarations(context.Background(), "svc", []store.CertifyDeclaration{{TestSymbolKey: "svc::x.TestPrior", Kind: "invariant", TargetID: "I1"}}); err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			want := context.Canceled
			if mode == "cancelled" {
				cancel()
			} else {
				previous := tenancy.CurrentLevel()
				tenancy.PublishLevel(tenancy.LevelRefuse)
				t.Cleanup(func() { tenancy.PublishLevel(previous) })
				want = tenancy.ErrOverBudget
			}
			_, err := IngestCertifyMarkers(ctx, st, "svc", "w1", root)
			if !errors.Is(err, want) {
				t.Errorf("error=%v, want %v", err, want)
			}
			decls, err := st.CertifyDeclarationsForService(context.Background(), "svc")
			if err != nil || len(decls) != 1 {
				t.Fatalf("prior declarations lost: %v %v", decls, err)
			}
		})
	}
}
