package platform_test

import (
	"path/filepath"
	"runtime"
	"testing"

	"github.com/geoffmilleraz/fathom/pkg/platform"
)

func repoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	// file is .../pkg/platform/platform_test.go → go up three levels to repo root
	return filepath.Join(filepath.Dir(file), "..", "..")
}

func fixtureRepos(t *testing.T) map[string]string {
	t.Helper()
	root := repoRoot(t)
	return map[string]string{
		"service-a": filepath.Join(root, "testdata", "fleet", "service-a"),
		"service-b": filepath.Join(root, "testdata", "fleet", "service-b"),
	}
}

// TestBuild verifies the public Build wrapper produces a non-nil graph.
func TestBuild(t *testing.T) {
	repos := fixtureRepos(t)
	g, err := platform.Build(repos)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if g == nil {
		t.Fatal("Build returned nil graph")
	}
}

// TestPlatformCallers verifies callers of an exported symbol are returned.
func TestPlatformCallers(t *testing.T) {
	repos := fixtureRepos(t)
	g, err := platform.Build(repos)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	callers := platform.PlatformCallers(g, "Greet")
	if len(callers) == 0 {
		t.Fatal("expected callers of Greet, got none")
	}
	for _, e := range callers {
		if e.ToKey != "Greet" {
			t.Errorf("PlatformCallers(Greet): got edge with ToKey=%q", e.ToKey)
		}
	}
}

// TestCrossRepoBlastRadius verifies cross-repo blast radius.
func TestCrossRepoBlastRadius(t *testing.T) {
	repos := fixtureRepos(t)
	g, err := platform.Build(repos)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	deps := platform.CrossRepoBlastRadius(g, "service-a")
	if len(deps) == 0 {
		t.Fatal("expected service-b in blast radius of service-a, got none")
	}
	found := false
	for _, svc := range deps {
		if svc == "service-b" {
			found = true
		}
	}
	if !found {
		t.Errorf("service-b not in blast radius: %v", deps)
	}
}

// TestDeterminism ensures Build twice yields identical edges.
func TestDeterminism(t *testing.T) {
	repos := fixtureRepos(t)

	g1, err := platform.Build(repos)
	if err != nil {
		t.Fatalf("first Build: %v", err)
	}
	g2, err := platform.Build(repos)
	if err != nil {
		t.Fatalf("second Build: %v", err)
	}

	e1 := platform.PlatformCallers(g1, "Greet")
	e2 := platform.PlatformCallers(g2, "Greet")
	if len(e1) != len(e2) {
		t.Errorf("non-deterministic caller count: %d vs %d", len(e1), len(e2))
	}
	for i := range e1 {
		if e1[i] != e2[i] {
			t.Errorf("edge mismatch at index %d: %+v vs %+v", i, e1[i], e2[i])
		}
	}
}
