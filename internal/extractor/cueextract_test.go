package extractor_test

import (
	"testing"

	"github.com/geoffmilleraz/sextant/internal/core"
	"github.com/geoffmilleraz/sextant/internal/extractor"
)

const cueFixture = `package config

// #Service defines a Kubernetes service.
#Service: {
  name: string
  port: int & >0
}

// #Deployment defines a deployment spec.
#Deployment: {
  replicas: int | *1
  image: string
}

// Not a definition — should be ignored.
someValue: "hello"
`

func TestCUEExtractor_Basic(t *testing.T) {
	x := extractor.NewCUEExtractor()
	nodes, err := x.Extract("svc", "schema.cue", []byte(cueFixture))
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}

	byName := make(map[string]core.SymbolNode, len(nodes))
	for _, n := range nodes {
		byName[n.Name] = n
	}

	t.Run("#Service", func(t *testing.T) {
		n, ok := byName["#Service"]
		if !ok {
			t.Fatal("expected symbol #Service")
		}
		if n.Kind != core.KindCUEDefinition {
			t.Errorf("Kind: got %q, want KindCUEDefinition", n.Kind)
		}
		if n.Tier != core.TierVerified {
			t.Errorf("Tier: got %q, want TierVerified", n.Tier)
		}
	})

	t.Run("#Deployment", func(t *testing.T) {
		_, ok := byName["#Deployment"]
		if !ok {
			t.Fatal("expected symbol #Deployment")
		}
	})

	// Non-definition values should not appear.
	if _, ok := byName["someValue"]; ok {
		t.Error("unexpected symbol someValue (not a CUE definition)")
	}

	if len(nodes) != 2 {
		t.Errorf("expected 2 nodes, got %d", len(nodes))
	}
}

func TestCUEExtractor_LineNumbers(t *testing.T) {
	x := extractor.NewCUEExtractor()
	nodes, err := x.Extract("svc", "schema.cue", []byte(cueFixture))
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}

	byName := make(map[string]core.SymbolNode, len(nodes))
	for _, n := range nodes {
		byName[n.Name] = n
	}

	// #Service is on line 4 (after package line, blank, comment).
	if n, ok := byName["#Service"]; ok {
		if n.Line != 4 {
			t.Errorf("#Service line: got %d, want 4", n.Line)
		}
	}
}

func TestCUEExtractor_Determinism(t *testing.T) {
	x := extractor.NewCUEExtractor()
	n1, err := x.Extract("svc", "schema.cue", []byte(cueFixture))
	if err != nil {
		t.Fatalf("first Extract: %v", err)
	}
	n2, err := x.Extract("svc", "schema.cue", []byte(cueFixture))
	if err != nil {
		t.Fatalf("second Extract: %v", err)
	}
	if len(n1) != len(n2) {
		t.Fatalf("non-deterministic length: %d vs %d", len(n1), len(n2))
	}
}
