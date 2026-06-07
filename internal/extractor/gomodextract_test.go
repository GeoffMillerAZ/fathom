package extractor_test

import (
	"testing"

	"github.com/geoffmilleraz/sextant/internal/core"
	"github.com/geoffmilleraz/sextant/internal/extractor"
)

const gomodFixture = `module github.com/example/myapp

go 1.21

require (
	github.com/pkg/errors v0.9.1
	golang.org/x/sync v0.5.0 // indirect
)

require github.com/stretchr/testify v1.8.4
`

func TestGoModExtractor_Basic(t *testing.T) {
	x := extractor.NewGoModExtractor()
	nodes, err := x.Extract("svc", "go.mod", []byte(gomodFixture))
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}

	byName := make(map[string]core.SymbolNode, len(nodes))
	for _, n := range nodes {
		byName[n.Name] = n
	}

	deps := []struct {
		name    string
		version string
	}{
		{"github.com/pkg/errors", "v0.9.1"},
		{"golang.org/x/sync", "v0.5.0"},
		{"github.com/stretchr/testify", "v1.8.4"},
	}

	for _, dep := range deps {
		n, ok := byName[dep.name]
		if !ok {
			t.Errorf("expected module dep %q", dep.name)
			continue
		}
		if n.Kind != core.KindModuleDep {
			t.Errorf("%q Kind: got %q, want KindModuleDep", dep.name, n.Kind)
		}
		if n.Tier != core.TierVerified {
			t.Errorf("%q Tier: got %q, want TierVerified", dep.name, n.Tier)
		}
		if n.Signature != dep.version {
			t.Errorf("%q Signature: got %q, want %q", dep.name, n.Signature, dep.version)
		}
	}

	if len(nodes) != 3 {
		t.Errorf("expected 3 nodes, got %d", len(nodes))
	}
}

func TestGoModExtractor_Determinism(t *testing.T) {
	x := extractor.NewGoModExtractor()
	n1, err := x.Extract("svc", "go.mod", []byte(gomodFixture))
	if err != nil {
		t.Fatalf("first Extract: %v", err)
	}
	n2, err := x.Extract("svc", "go.mod", []byte(gomodFixture))
	if err != nil {
		t.Fatalf("second Extract: %v", err)
	}
	if len(n1) != len(n2) {
		t.Fatalf("non-deterministic length: %d vs %d", len(n1), len(n2))
	}
	for i := range n1 {
		if n1[i].Name != n2[i].Name {
			t.Errorf("non-deterministic at [%d]", i)
		}
	}
}
