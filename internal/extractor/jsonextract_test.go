package extractor_test

import (
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/extractor"
)

// jsonPackageFixture is a trimmed package.json-style fixture with several
// top-level scalar keys, an object value, and an array value.
const jsonPackageFixture = `{
  "name": "fauxtown-app",
  "version": "1.0.0",
  "description": "Fauxtown test application",
  "main": "index.js",
  "scripts": {
    "build": "tsc",
    "test": "jest"
  },
  "dependencies": {
    "express": "^4.18.0"
  },
  "keywords": ["test", "fauxtown"],
  "private": true
}`

func TestJSONExtractor_TopLevelKeys(t *testing.T) {
	x := extractor.NewJSONExtractor()
	nodes, err := x.Extract("svc", "package.json", []byte(jsonPackageFixture))
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}

	expected := []string{
		"dependencies",
		"description",
		"keywords",
		"main",
		"name",
		"private",
		"scripts",
		"version",
	}

	if len(nodes) != len(expected) {
		t.Fatalf("expected %d nodes, got %d: %v", len(expected), len(nodes), nodeNames(nodes))
	}

	// Nodes must be sorted by Name.
	for i, want := range expected {
		n := nodes[i]
		if n.Name != want {
			t.Errorf("nodes[%d]: got %q, want %q", i, n.Name, want)
		}
		if n.Kind != core.KindJSONKey {
			t.Errorf("nodes[%d] Kind: got %q, want KindJSONKey", i, n.Kind)
		}
		if n.Tier != core.TierVerified {
			t.Errorf("nodes[%d] Tier: got %q, want TierVerified", i, n.Tier)
		}
		if n.Service != "svc" {
			t.Errorf("nodes[%d] Service: got %q, want svc", i, n.Service)
		}
		if n.FilePath != "package.json" {
			t.Errorf("nodes[%d] FilePath: got %q, want package.json", i, n.FilePath)
		}
	}
}

func TestJSONExtractor_NoRecursion(t *testing.T) {
	// Nested keys (inside "scripts" and "dependencies") must NOT appear.
	x := extractor.NewJSONExtractor()
	nodes, err := x.Extract("svc", "package.json", []byte(jsonPackageFixture))
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}

	byName := make(map[string]struct{}, len(nodes))
	for _, n := range nodes {
		byName[n.Name] = struct{}{}
	}

	for _, unexpected := range []string{"build", "test", "express"} {
		if _, found := byName[unexpected]; found {
			t.Errorf("unexpected nested key %q should not appear as top-level node", unexpected)
		}
	}
}

func TestJSONExtractor_ArrayRoot(t *testing.T) {
	// A JSON array at root should yield zero nodes.
	src := `["alpha", "beta", "gamma"]`
	x := extractor.NewJSONExtractor()
	nodes, err := x.Extract("svc", "list.json", []byte(src))
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	if len(nodes) != 0 {
		t.Errorf("expected 0 nodes for array root, got %d: %v", len(nodes), nodeNames(nodes))
	}
}

func TestJSONExtractor_ScalarRoot(t *testing.T) {
	// A bare JSON scalar should yield zero nodes.
	for _, src := range []string{`42`, `"hello"`, `true`, `null`} {
		x := extractor.NewJSONExtractor()
		nodes, err := x.Extract("svc", "scalar.json", []byte(src))
		if err != nil {
			t.Fatalf("Extract(%q): %v", src, err)
		}
		if len(nodes) != 0 {
			t.Errorf("Extract(%q): expected 0 nodes, got %d", src, len(nodes))
		}
	}
}

func TestJSONExtractor_InvalidJSON(t *testing.T) {
	// Invalid JSON must return nil, nil (best-effort).
	src := `{ "broken": `
	x := extractor.NewJSONExtractor()
	nodes, err := x.Extract("svc", "broken.json", []byte(src))
	if err != nil {
		t.Fatalf("Extract on invalid JSON should not return error, got: %v", err)
	}
	if len(nodes) != 0 {
		t.Errorf("expected 0 nodes on invalid JSON, got %d", len(nodes))
	}
}

func TestJSONExtractor_EmptyObject(t *testing.T) {
	src := `{}`
	x := extractor.NewJSONExtractor()
	nodes, err := x.Extract("svc", "empty.json", []byte(src))
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	if len(nodes) != 0 {
		t.Errorf("expected 0 nodes for empty object, got %d", len(nodes))
	}
}

func TestJSONExtractor_Determinism(t *testing.T) {
	x := extractor.NewJSONExtractor()
	n1, err := x.Extract("svc", "package.json", []byte(jsonPackageFixture))
	if err != nil {
		t.Fatalf("first Extract: %v", err)
	}
	n2, err := x.Extract("svc", "package.json", []byte(jsonPackageFixture))
	if err != nil {
		t.Fatalf("second Extract: %v", err)
	}
	if len(n1) != len(n2) {
		t.Fatalf("non-deterministic length: %d vs %d", len(n1), len(n2))
	}
	for i := range n1 {
		if n1[i].Name != n2[i].Name || n1[i].Kind != n2[i].Kind {
			t.Errorf("non-deterministic at [%d]: %+v vs %+v", i, n1[i], n2[i])
		}
	}
}

func TestJSONExtractor_TierVerified(t *testing.T) {
	src := `{"alpha": 1, "beta": 2}`
	x := extractor.NewJSONExtractor()
	nodes, err := x.Extract("svc", "simple.json", []byte(src))
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	for _, n := range nodes {
		if n.Tier != core.TierVerified {
			t.Errorf("node %q: Tier = %q, want TierVerified", n.Name, n.Tier)
		}
	}
}
