package extractor_test

import (
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/extractor"
)

// rubyFixture exercises all constructs the new tree-sitter extractor must handle:
//   - a module containing a class
//   - instance methods inside the class
//   - a singleton method (def self.foo) inside the class
const rubyFixture = `module Payments
  class Gateway
    def initialize(config)
      @config = config
    end

    def charge(amount)
      # process payment
    end

    def self.supported_currencies
      ["USD", "EUR"]
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

	// All symbols must carry TierVerified (not TierPreScan as in the old extractor).
	for _, n := range nodes {
		if n.Tier != core.TierVerified {
			t.Errorf("symbol %q: Tier=%q, want TierVerified", n.Name, n.Tier)
		}
	}

	t.Run("module Payments", func(t *testing.T) {
		n, ok := byName["Payments"]
		if !ok {
			t.Fatal("expected symbol Payments")
		}
		if n.Kind != core.KindRubyModule {
			t.Errorf("Kind: got %q, want KindRubyModule", n.Kind)
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
		n, ok := byName["Payments::Gateway.initialize"]
		if !ok {
			t.Fatal("expected symbol Payments::Gateway.initialize")
		}
		if n.Kind != core.KindRubyMethod {
			t.Errorf("Kind: got %q, want KindRubyMethod", n.Kind)
		}
	})

	t.Run("method charge", func(t *testing.T) {
		_, ok := byName["Payments::Gateway.charge"]
		if !ok {
			t.Fatal("expected symbol Payments::Gateway.charge")
		}
	})

	t.Run("singleton method self.supported_currencies", func(t *testing.T) {
		n, ok := byName["Payments::Gateway.supported_currencies"]
		if !ok {
			t.Fatal("expected singleton method Payments::Gateway.supported_currencies")
		}
		if n.Kind != core.KindRubyMethod {
			t.Errorf("Kind: got %q, want KindRubyMethod", n.Kind)
		}
	})

	t.Run("module singleton method self.version", func(t *testing.T) {
		n, ok := byName["Payments.version"]
		if !ok {
			t.Fatal("expected module singleton method Payments.version")
		}
		if n.Kind != core.KindRubyMethod {
			t.Errorf("Kind: got %q, want KindRubyMethod", n.Kind)
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

// rubyCallFixture is a minimal Ruby file for verifying call-edge extraction.
// greet() calls helper("arg") and puts("hi") — both emit "call" nodes with
// a "method" field. helper() makes no calls.
const rubyCallFixture = `class Caller
  def greet
    helper("arg")
    puts("hi")
  end
  def helper(x)
  end
end
`

func TestRubyExtractor_CallEdges(t *testing.T) {
	x := extractor.NewRubyExtractor()
	nodes, err := x.Extract("svc", "caller.rb", []byte(rubyCallFixture))
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}

	byName := make(map[string]core.SymbolNode, len(nodes))
	for _, n := range nodes {
		byName[n.Name] = n
	}

	t.Run("greet calls helper and puts", func(t *testing.T) {
		n, ok := byName["Caller.greet"]
		if !ok {
			t.Fatal("expected symbol Caller.greet")
		}
		wantCalls := map[string]bool{"helper": true, "puts": true}
		for _, c := range n.Calls {
			if !wantCalls[c] {
				t.Errorf("unexpected call %q in Caller.greet.Calls", c)
			}
			delete(wantCalls, c)
		}
		for missing := range wantCalls {
			t.Errorf("Caller.greet.Calls missing %q", missing)
		}
	})

	t.Run("greet Calls is sorted", func(t *testing.T) {
		n := byName["Caller.greet"]
		for i := 1; i < len(n.Calls); i++ {
			if n.Calls[i] < n.Calls[i-1] {
				t.Errorf("Calls not sorted at [%d]: %q > %q", i, n.Calls[i-1], n.Calls[i])
			}
		}
	})

	t.Run("helper has no calls", func(t *testing.T) {
		n := byName["Caller.helper"]
		if len(n.Calls) != 0 {
			t.Errorf("helper.Calls = %v, want empty", n.Calls)
		}
	})
}
