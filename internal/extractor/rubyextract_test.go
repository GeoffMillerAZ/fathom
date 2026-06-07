package extractor_test

import (
	"testing"

	"github.com/geoffmilleraz/sextant/internal/core"
	"github.com/geoffmilleraz/sextant/internal/extractor"
)

const rubyFixture = `module Payments
  class Gateway
    def initialize(config)
      @config = config
    end

    def charge(amount)
      # process payment
    end
  end

  def self.version
    "1.0"
  end
end
`

func TestRubyExtractor_Basic(t *testing.T) {
	x := extractor.NewRubyExtractor()
	nodes, err := x.Extract("svc", "payments.rb", []byte(rubyFixture))
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}

	byName := make(map[string]core.SymbolNode, len(nodes))
	for _, n := range nodes {
		byName[n.Name] = n
	}

	t.Run("module Payments", func(t *testing.T) {
		n, ok := byName["Payments"]
		if !ok {
			t.Fatal("expected symbol Payments")
		}
		if n.Kind != core.KindRubyModule {
			t.Errorf("Kind: got %q, want KindRubyModule", n.Kind)
		}
		if n.Tier != core.TierVerified {
			t.Errorf("Tier: got %q, want TierVerified", n.Tier)
		}
	})

	t.Run("class Gateway", func(t *testing.T) {
		n, ok := byName["Payments::Gateway"]
		if !ok {
			t.Fatal("expected symbol Payments::Gateway")
		}
		if n.Kind != core.KindRubyClass {
			t.Errorf("Kind: got %q, want KindRubyClass", n.Kind)
		}
	})

	t.Run("method initialize", func(t *testing.T) {
		_, ok := byName["Payments::Gateway.initialize"]
		if !ok {
			t.Fatal("expected symbol Payments::Gateway.initialize")
		}
	})

	t.Run("method charge", func(t *testing.T) {
		_, ok := byName["Payments::Gateway.charge"]
		if !ok {
			t.Fatal("expected symbol Payments::Gateway.charge")
		}
	})
}

func TestRubyExtractor_Determinism(t *testing.T) {
	x := extractor.NewRubyExtractor()
	n1, err := x.Extract("svc", "payments.rb", []byte(rubyFixture))
	if err != nil {
		t.Fatalf("first Extract: %v", err)
	}
	n2, err := x.Extract("svc", "payments.rb", []byte(rubyFixture))
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
