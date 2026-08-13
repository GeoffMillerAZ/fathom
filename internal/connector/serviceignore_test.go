package connector

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
)

func writeTree(t *testing.T, root string, files map[string]string) {
	t.Helper()
	for rel, body := range files {
		p := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func discoverPaths(t *testing.T, root, serviceID string) []string {
	t.Helper()
	c := NewLocalConnector()
	refs, err := c.Discover(context.Background(), core.ServiceRef{ID: serviceID, Path: root})
	if err != nil {
		t.Fatalf("Discover(%s): %v", serviceID, err)
	}
	out := make([]string, 0, len(refs))
	for _, r := range refs {
		out = append(out, r.Path)
	}
	return out
}

func contains(list []string, want string) bool {
	for _, s := range list {
		if s == want {
			return true
		}
	}
	return false
}

// TestServiceScopedIgnoreAppliesOnlyToItsService is the V2/V4 rework's
// core proof: a pattern recorded for service A excludes A's files at
// Discover and leaves service B — with the SAME relative layout —
// completely untouched. The old fleet-wide file made "purge one service"
// mean "stop indexing this path everywhere"; the scoped file cannot.
func TestServiceScopedIgnoreAppliesOnlyToItsService(t *testing.T) {
	t.Setenv(ServiceIgnoreDirEnvVar, filepath.Join(t.TempDir(), "ignore.d"))
	t.Setenv("HOME", t.TempDir()) // isolate ~/.fathom/ignore too

	layout := map[string]string{
		"secrets/token.go": "package secrets\n",
		"internal/keep.go": "package internal\n",
	}
	rootA := t.TempDir()
	rootB := t.TempDir()
	writeTree(t, rootA, layout)
	writeTree(t, rootB, layout)

	wrote, path, err := AppendServiceIgnore("svc-a", []string{"secrets/**"}, "purged: leaked credential")
	if err != nil || !wrote {
		t.Fatalf("AppendServiceIgnore: wrote=%v err=%v", wrote, err)
	}
	if filepath.Base(path) != "svc-a" {
		t.Fatalf("exclusion landed in %q, want a file named for the service", path)
	}

	gotA := discoverPaths(t, rootA, "svc-a")
	if contains(gotA, "secrets/token.go") {
		t.Fatalf("svc-a still discovers the excluded path: %v", gotA)
	}
	if !contains(gotA, "internal/keep.go") {
		t.Fatalf("svc-a lost an unrelated path: %v", gotA)
	}

	gotB := discoverPaths(t, rootB, "svc-b")
	if !contains(gotB, "secrets/token.go") {
		t.Fatalf("svc-b lost secrets/token.go to svc-a's exclusion — the exclusion is NOT service-scoped (V2/V4): %v", gotB)
	}

	// The counts disclose the scoped patterns distinctly.
	_, counts, err := LoadIgnoreScopedFor(rootA, "svc-a")
	if err != nil {
		t.Fatal(err)
	}
	if counts.Service != 1 {
		t.Fatalf("counts.Service = %d, want 1", counts.Service)
	}
	if _, counts, err = LoadIgnoreScopedFor(rootB, "svc-b"); err != nil || counts.Service != 0 {
		t.Fatalf("svc-b sees %d service-scoped pattern(s), want 0 (err=%v)", counts.Service, err)
	}
}

// TestAppendServiceIgnoreRefusesFleetWide is V2's refusal half: a request
// that would apply fleet-wide — no service id — is refused loudly, never
// widened; and an id that could escape the scoped directory is refused too.
func TestAppendServiceIgnoreRefusesFleetWide(t *testing.T) {
	t.Setenv(ServiceIgnoreDirEnvVar, filepath.Join(t.TempDir(), "ignore.d"))

	if _, _, err := AppendServiceIgnore("", []string{"**"}, "x"); err == nil {
		t.Fatal("a fleet-wide exclusion (empty service) was accepted — this is the accident that wiped the index")
	} else if !strings.Contains(strings.ToLower(err.Error()), "fleet-wide") {
		t.Fatalf("the refusal does not name the fleet-wide hazard: %v", err)
	}
	for _, id := range []string{"../evil", "a/b", `a\b`, "..", "."} {
		if _, _, err := AppendServiceIgnore(id, []string{"x/**"}, "c"); err == nil {
			t.Errorf("unsafe service id %q was accepted", id)
		}
	}
	if _, _, err := AppendServiceIgnore("svc", []string{"!vendor/**"}, "c"); err == nil {
		t.Fatal("a re-include pattern was accepted into a scoped ignore file")
	}
	if _, _, err := AppendServiceIgnore("svc", nil, "c"); err == nil {
		t.Fatal("an empty pattern list was accepted")
	}
}

// TestAppendServiceIgnoreDeduplicates: purge runs repeatedly; the file must
// not fill with duplicates.
func TestAppendServiceIgnoreDeduplicates(t *testing.T) {
	t.Setenv(ServiceIgnoreDirEnvVar, filepath.Join(t.TempDir(), "ignore.d"))
	if wrote, _, err := AppendServiceIgnore("svc", []string{"secrets/**"}, "first"); err != nil || !wrote {
		t.Fatalf("first append: wrote=%v err=%v", wrote, err)
	}
	wrote, path, err := AppendServiceIgnore("svc", []string{"secrets/**"}, "second")
	if err != nil {
		t.Fatal(err)
	}
	if wrote {
		t.Fatal("the same pattern was appended twice")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if n := strings.Count(string(data), "secrets/**"); n != 1 {
		t.Fatalf("pattern appears %d times, want 1:\n%s", n, data)
	}
}

// TestServiceScopedReincludeIsRefusedAtDiscover: the scoped file obeys the
// same law-13 additive-only rule as ~/.fathom/ignore — a hand-edited
// re-include blocks ingest loudly rather than silently widening the index.
func TestServiceScopedReincludeIsRefusedAtDiscover(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "ignore.d")
	t.Setenv(ServiceIgnoreDirEnvVar, dir)
	t.Setenv("HOME", t.TempDir())
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "svc"), []byte("!vendor/**\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	writeTree(t, root, map[string]string{"a.go": "package a\n"})
	c := NewLocalConnector()
	if _, err := c.Discover(context.Background(), core.ServiceRef{ID: "svc", Path: root}); err == nil {
		t.Fatal("a re-include in the scoped ignore file did not block Discover")
	}
}
