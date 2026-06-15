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

// Not a definition — should be a plain CUEField.
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
		n, ok := byName["#Deployment"]
		if !ok {
			t.Fatal("expected symbol #Deployment")
		}
		if n.Kind != core.KindCUEDefinition {
			t.Errorf("Kind: got %q, want KindCUEDefinition", n.Kind)
		}
		if n.Tier != core.TierVerified {
			t.Errorf("Tier: got %q, want TierVerified", n.Tier)
		}
	})

	t.Run("someValue_is_KindCUEField", func(t *testing.T) {
		n, ok := byName["someValue"]
		if !ok {
			t.Fatal("expected symbol someValue (non-definition top-level field)")
		}
		if n.Kind != core.KindCUEField {
			t.Errorf("someValue Kind: got %q, want KindCUEField", n.Kind)
		}
		if n.Tier != core.TierVerified {
			t.Errorf("someValue Tier: got %q, want TierVerified", n.Tier)
		}
	})

	if len(nodes) != 3 {
		t.Errorf("expected 3 nodes (#Service, #Deployment, someValue), got %d", len(nodes))
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
	for i := range n1 {
		if n1[i].Name != n2[i].Name || n1[i].Line != n2[i].Line {
			t.Errorf("non-deterministic at index %d: %+v vs %+v", i, n1[i], n2[i])
		}
	}
}
