package platformlink_test

import (
	"path/filepath"
	"reflect"
	"runtime"
	"testing"

	"github.com/geoffmilleraz/sextant/internal/core"
	"github.com/geoffmilleraz/sextant/internal/platformlink"
)

// repoRoot returns the absolute path to the repository root so tests can
// reference testdata regardless of where go test is invoked.
func repoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	// file is .../internal/platformlink/platformlink_test.go
	// repo root is two levels up
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

// TestBuildPlatformGraph_EdgeDetected verifies that a cross-repo call from
// service-b into service-a produces the expected CrossRepoEdge.
func TestBuildPlatformGraph_EdgeDetected(t *testing.T) {
	repos := fixtureRepos(t)
	g, err := platformlink.BuildPlatformGraph(repos)
	if err != nil {
		t.Fatalf("BuildPlatformGraph: %v", err)
	}

	edges := g.Edges()
	if len(edges) == 0 {
		t.Fatal("expected at least one cross-repo edge, got none")
	}

	// Find the edge from service-b calling service-a's Greet.
	var found *core.CrossRepoEdge
	for i := range edges {
		e := &edges[i]
		if e.FromService == "service-b" && e.ToService == "service-a" && e.ToKey == "Greet" {
			found = e
			break
		}
	}
	if found == nil {
		t.Fatalf("expected edge FromService=service-b ToService=service-a ToKey=Greet; edges=%v", edges)
	}
	if found.FromFile == "" {
		t.Error("FromFile must not be empty")
	}
	if found.ToFile == "" {
		t.Error("ToFile must not be empty")
	}
}

// TestCallers returns edges that call a given symbol key.
func TestCallers(t *testing.T) {
	repos := fixtureRepos(t)
	g, err := platformlink.BuildPlatformGraph(repos)
	if err != nil {
		t.Fatalf("BuildPlatformGraph: %v", err)
	}

	callers := g.Callers("Greet")
	if len(callers) == 0 {
		t.Fatal("expected callers of Greet, got none")
	}
	for _, e := range callers {
		if e.ToKey != "Greet" {
			t.Errorf("Callers(Greet) returned edge with ToKey=%q", e.ToKey)
		}
		if e.FromService != "service-b" {
			t.Errorf("unexpected caller service %q", e.FromService)
		}
	}
}

// TestBlastRadius returns services that depend on a given service.
func TestBlastRadius(t *testing.T) {
	repos := fixtureRepos(t)
	g, err := platformlink.BuildPlatformGraph(repos)
	if err != nil {
		t.Fatalf("BuildPlatformGraph: %v", err)
	}

	dependents := g.BlastRadius("service-a")
	if len(dependents) == 0 {
		t.Fatal("expected service-b in blast radius of service-a, got none")
	}
	found := false
	for _, svc := range dependents {
		if svc == "service-b" {
			found = true
		}
	}
	if !found {
		t.Errorf("service-b not in blast radius: %v", dependents)
	}

	// service-b has no known dependents in the fixture.
	deps := g.BlastRadius("service-b")
	if len(deps) != 0 {
		t.Errorf("blast radius of service-b should be empty, got %v", deps)
	}
}

// TestDeterminism ensures that building the graph twice yields identical results.
func TestDeterminism(t *testing.T) {
	repos := fixtureRepos(t)

	g1, err := platformlink.BuildPlatformGraph(repos)
	if err != nil {
		t.Fatalf("first build: %v", err)
	}
	g2, err := platformlink.BuildPlatformGraph(repos)
	if err != nil {
		t.Fatalf("second build: %v", err)
	}

	e1, e2 := g1.Edges(), g2.Edges()
	if !reflect.DeepEqual(e1, e2) {
		t.Errorf("non-deterministic edges:\nrun1: %v\nrun2: %v", e1, e2)
	}
}

// TestEmptyRepos returns an empty graph without error.
func TestEmptyRepos(t *testing.T) {
	g, err := platformlink.BuildPlatformGraph(map[string]string{})
	if err != nil {
		t.Fatalf("BuildPlatformGraph(empty): %v", err)
	}
	if edges := g.Edges(); len(edges) != 0 {
		t.Errorf("expected no edges for empty repos, got %v", edges)
	}
}

// TestNoIntraServiceEdges ensures that calls within the same service are not
// reported as cross-repo edges.
func TestNoIntraServiceEdges(t *testing.T) {
	repos := fixtureRepos(t)
	g, err := platformlink.BuildPlatformGraph(repos)
	if err != nil {
		t.Fatalf("BuildPlatformGraph: %v", err)
	}
	for _, e := range g.Edges() {
		if e.FromService == e.ToService {
			t.Errorf("intra-service edge found: %+v", e)
		}
	}
}
