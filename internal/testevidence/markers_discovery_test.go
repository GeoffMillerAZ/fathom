package testevidence

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/store"
)

const markerPolicyProof = "package proof\n\n// certifies: invariant I35 (asserts)\nfunc TestProof(t *T) {}\n"

func isolateMarkerPolicy(t *testing.T) {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("FATHOM_HOME", filepath.Join(home, ".fathom"))
	t.Setenv(connector.ServiceIgnoreDirEnvVar, filepath.Join(home, ".fathom", "ignore.d"))
}

func markerPolicyDeclarations(t *testing.T, st store.Store, service string) []string {
	t.Helper()
	declarations, err := st.CertifyDeclarationsForService(context.Background(), service)
	if err != nil {
		t.Fatal(err)
	}
	files := []string{}
	for _, d := range declarations {
		files = append(files, d.File)
	}
	sort.Strings(files)
	return files
}

func assertMarkerPolicyFiles(t *testing.T, root, service string, want []string) {
	t.Helper()
	st := store.NewMemoryStore()
	t.Cleanup(func() { _ = st.Close() })
	report, err := IngestCertifyMarkers(context.Background(), st, service, "policy", root)
	if err != nil {
		t.Fatal(err)
	}
	if got := markerPolicyDeclarations(t, st, service); !reflect.DeepEqual(got, want) {
		t.Fatalf("marker declaration files=%v want=%v", got, want)
	}
	if report.FilesScanned != len(want) || report.Declared != len(want) || len(report.Warnings) != 0 {
		t.Fatalf("marker report %+v want only %v", report, want)
	}
}

func TestIngestCertifyMarkers_DefaultCacheAndProjectExclusions(t *testing.T) {
	isolateMarkerPolicy(t)
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "src", "proof_test.go"), markerPolicyProof)
	for _, dir := range connector.DefaultIgnoredDirNames() {
		mustWrite(t, filepath.Join(root, dir, "dependency", "proof_test.go"), markerPolicyProof)
	}
	mustWrite(t, filepath.Join(root, "vendor", "proof_test.go"), markerPolicyProof)
	mustWrite(t, filepath.Join(root, "private-copy", "proof_test.go"), markerPolicyProof)
	mustWrite(t, filepath.Join(root, ".fathomignore"), "private-copy/\n")
	// An ignored malformed marker cannot add warning noise either.
	mustWrite(t, filepath.Join(root, ".cache", "broken_test.go"), "package proof\n// certifies: not-a-kind I99\nfunc TestBad(t *T) {}\n")
	assertMarkerPolicyFiles(t, root, "svc", []string{"src/proof_test.go"})
}

func TestIngestCertifyMarkers_ExplicitIncludeKeepsPermanentAndProjectExclusions(t *testing.T) {
	isolateMarkerPolicy(t)
	root := t.TempDir()
	for _, name := range []string{"src/proof_test.go", ".cache/source/proof_test.go", ".cache/blocked/proof_test.go", "vendor/proof_test.go"} {
		mustWrite(t, filepath.Join(root, filepath.FromSlash(name)), markerPolicyProof)
	}
	mustWrite(t, filepath.Join(root, ".fathom", "config.toml"), "[connector]\ninclude = [\".cache\", \"vendor\"]\n")
	mustWrite(t, filepath.Join(root, ".fathomignore"), ".cache/blocked/\n")
	assertMarkerPolicyFiles(t, root, "svc", []string{".cache/source/proof_test.go", "src/proof_test.go"})
}

func TestIngestCertifyMarkers_ScopedExclusionAndLoudRefusal(t *testing.T) {
	isolateMarkerPolicy(t)
	root := t.TempDir()
	for _, name := range []string{"src/proof_test.go", "machine/proof_test.go", "service/proof_test.go", "project/proof_test.go"} {
		mustWrite(t, filepath.Join(root, filepath.FromSlash(name)), markerPolicyProof)
	}
	mustWrite(t, connector.UserIgnorePath(), "machine/\n")
	serviceIgnore, err := connector.ServiceIgnorePath("svc")
	if err != nil {
		t.Fatal(err)
	}
	mustWrite(t, serviceIgnore, "service/\n")
	mustWrite(t, filepath.Join(root, ".fathomignore"), "project/\n")
	assertMarkerPolicyFiles(t, root, "svc", []string{"src/proof_test.go"})
	assertMarkerPolicyFiles(t, root, "other", []string{"service/proof_test.go", "src/proof_test.go"})
	st := store.NewMemoryStore()
	t.Cleanup(func() { _ = st.Close() })
	if _, err = IngestCertifyMarkers(context.Background(), st, "svc", "before", root); err != nil {
		t.Fatal(err)
	}
	before := markerPolicyDeclarations(t, st, "svc")
	mustWrite(t, serviceIgnore, "!src/\n")
	_, err = IngestCertifyMarkers(context.Background(), st, "svc", "refused", root)
	var refusal *connector.ErrUserScopeReinclude
	if !errors.As(err, &refusal) {
		t.Fatalf("invalid scoped policy not refused by name: %v", err)
	}
	if after := markerPolicyDeclarations(t, st, "svc"); !reflect.DeepEqual(after, before) {
		t.Fatalf("failed discovery replaced prior declarations: %v -> %v", before, after)
	}
}

func TestIngestCertifyMarkers_GitIgnoredUntrackedFiles(t *testing.T) {
	isolateMarkerPolicy(t)
	root := t.TempDir()
	for _, name := range []string{"src/proof_test.go", "ignored/tracked_test.go", "ignored/untracked_test.go", ".cache/dependency/proof_test.go"} {
		mustWrite(t, filepath.Join(root, filepath.FromSlash(name)), markerPolicyProof)
	}
	mustWrite(t, filepath.Join(root, ".gitignore"), "ignored/\n")
	mustWrite(t, filepath.Join(root, ".fathom", "config.toml"), "[connector]\nrespect_gitignore = true\n")
	for _, args := range [][]string{{"init", "-q"}, {"add", "-f", "src/proof_test.go", "ignored/tracked_test.go", ".cache/dependency/proof_test.go"}} {
		cmd := exec.Command("git", args...)
		cmd.Dir = root
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("fixture git %v: %v: %s", args, err, output)
		}
	}
	// Tracked ignored files remain in the Git corpus; untracked ignored files
	// do not. Default caches stay excluded even if tracked.
	assertMarkerPolicyFiles(t, root, "svc", []string{"ignored/tracked_test.go", "src/proof_test.go"})
}

func TestIngestCertifyMarkers_RegularFilesAndLinkedRoot(t *testing.T) {
	isolateMarkerPolicy(t)
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "src", "proof_test.go"), markerPolicyProof)
	external := filepath.Join(t.TempDir(), "outside_test.go")
	mustWrite(t, external, markerPolicyProof)
	if err := os.Symlink(external, filepath.Join(root, "alias_test.go")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	assertMarkerPolicyFiles(t, root, "svc", []string{"src/proof_test.go"})
	link := filepath.Join(t.TempDir(), "linked-root")
	if err := os.Symlink(root, link); err != nil {
		t.Fatal(err)
	}
	assertMarkerPolicyFiles(t, link, "linked", []string{"src/proof_test.go"})
}

func TestIngestCertifyMarkers_DiscoveryCancellationPreservesState(t *testing.T) {
	isolateMarkerPolicy(t)
	root := filepath.Join(t.TempDir(), "build")
	mustWrite(t, filepath.Join(root, "proof_test.go"), markerPolicyProof)
	st := store.NewMemoryStore()
	t.Cleanup(func() { _ = st.Close() })
	if _, err := IngestCertifyMarkers(context.Background(), st, "svc", "before", root); err != nil {
		t.Fatal(err)
	}
	before := markerPolicyDeclarations(t, st, "svc")
	if !reflect.DeepEqual(before, []string{"proof_test.go"}) {
		t.Fatal("a corpus root named build was excluded")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := IngestCertifyMarkers(ctx, st, "svc", "cancelled", root)
	if !errors.Is(err, context.Canceled) || !strings.Contains(err.Error(), "discover Go files") {
		t.Fatalf("discovery cancellation not propagated: %v", err)
	}
	if after := markerPolicyDeclarations(t, st, "svc"); !reflect.DeepEqual(after, before) {
		t.Fatalf("cancelled scan replaced current declarations %v -> %v", before, after)
	}
}
