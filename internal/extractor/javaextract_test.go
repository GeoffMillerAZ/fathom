package extractor_test

import (
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/extractor"
)

// javaFixture exercises all constructs the new tree-sitter extractor must handle:
//   - a top-level class with two methods and a constructor
//   - a nested interface
//   - an enum
//   - a record
const javaFixture = `package com.example;

public class PaymentService {

    public PaymentService(String config) {
        this.config = config;
    }

    public void processPayment(String amount) {
        validate(amount);
    }

    private boolean validate(String amount) {
        return amount != null;
    }

    public interface Processor {
        void process(String amount);
    }

    public enum Status {
        ACTIVE, INACTIVE
    }

    public record Receipt(String amount, String id) {}
}
`

func TestJavaExtractor_Basic(t *testing.T) {
	x := extractor.NewJavaExtractor()
	nodes, err := x.Extract("svc", "PaymentService.java", []byte(javaFixture))
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

	t.Run("class PaymentService", func(t *testing.T) {
		n, ok := byName["PaymentService"]
		if !ok {
			t.Fatal("expected symbol PaymentService")
		}
		if n.Kind != core.KindJavaClass {
			t.Errorf("Kind: got %q, want KindJavaClass", n.Kind)
		}
	})

	t.Run("constructor PaymentService", func(t *testing.T) {
		n, ok := byName["PaymentService.PaymentService"]
		if !ok {
			t.Fatal("expected constructor symbol PaymentService.PaymentService (regex extractor missed constructors)")
		}
		if n.Kind != core.KindJavaMethod {
			t.Errorf("Kind: got %q, want KindJavaMethod", n.Kind)
		}
	})

	t.Run("method processPayment", func(t *testing.T) {
		n, ok := byName["PaymentService.processPayment"]
		if !ok {
			t.Fatal("expected symbol PaymentService.processPayment")
		}
		if n.Kind != core.KindJavaMethod {
			t.Errorf("Kind: got %q, want KindJavaMethod", n.Kind)
		}
	})

	t.Run("method validate", func(t *testing.T) {
		_, ok := byName["PaymentService.validate"]
		if !ok {
			t.Fatal("expected symbol PaymentService.validate")
		}
	})

	t.Run("interface Processor", func(t *testing.T) {
		n, ok := byName["PaymentService.Processor"]
		if !ok {
			t.Fatal("expected inner interface PaymentService.Processor")
		}
		if n.Kind != core.KindJavaInterface {
			t.Errorf("Kind: got %q, want KindJavaInterface", n.Kind)
		}
	})

	t.Run("interface method process", func(t *testing.T) {
		n, ok := byName["PaymentService.Processor.process"]
		if !ok {
			t.Fatal("expected interface method PaymentService.Processor.process")
		}
		if n.Kind != core.KindJavaMethod {
			t.Errorf("Kind: got %q, want KindJavaMethod", n.Kind)
		}
	})

	t.Run("enum Status", func(t *testing.T) {
		n, ok := byName["PaymentService.Status"]
		if !ok {
			t.Fatal("expected enum symbol PaymentService.Status (regex extractor mislabeled enums)")
		}
		if n.Kind != core.KindJavaEnum {
			t.Errorf("Kind: got %q, want KindJavaEnum", n.Kind)
		}
	})

	t.Run("record Receipt", func(t *testing.T) {
		n, ok := byName["PaymentService.Receipt"]
		if !ok {
			t.Fatal("expected record symbol PaymentService.Receipt (regex extractor mislabeled records)")
		}
		if n.Kind != core.KindJavaRecord {
			t.Errorf("Kind: got %q, want KindJavaRecord", n.Kind)
		}
	})
}

func TestJavaExtractor_Determinism(t *testing.T) {
	x := extractor.NewJavaExtractor()
	n1, err := x.Extract("svc", "P.java", []byte(javaFixture))
	if err != nil {
		t.Fatalf("first Extract: %v", err)
	}
	n2, err := x.Extract("svc", "P.java", []byte(javaFixture))
	if err != nil {
		t.Fatalf("second Extract: %v", err)
	}
	if len(n1) != len(n2) {
		t.Fatalf("non-deterministic length: %d vs %d", len(n1), len(n2))
	}
	for i := range n1 {
		if n1[i].Name != n2[i].Name {
			t.Errorf("non-deterministic at [%d]: %q vs %q", i, n1[i].Name, n2[i].Name)
		}
	}
}

// javaCallFixture is a minimal Java file for verifying call-edge extraction.
// caller() calls callee() and helper(); callee() and helper() make no calls.
const javaCallFixture = `public class CallTest {
    public void caller() {
        callee();
        helper();
    }
    public void callee() {}
    public void helper() {}
}
`

func TestJavaExtractor_CallEdges(t *testing.T) {
	x := extractor.NewJavaExtractor()
	nodes, err := x.Extract("svc", "CallTest.java", []byte(javaCallFixture))
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}

	byName := make(map[string]core.SymbolNode, len(nodes))
	for _, n := range nodes {
		byName[n.Name] = n
	}

	t.Run("caller calls callee and helper", func(t *testing.T) {
		n, ok := byName["CallTest.caller"]
		if !ok {
			t.Fatal("expected symbol CallTest.caller")
		}
		wantCalls := map[string]bool{"callee": true, "helper": true}
		for _, c := range n.Calls {
			if !wantCalls[c] {
				t.Errorf("unexpected call %q in CallTest.caller.Calls", c)
			}
			delete(wantCalls, c)
		}
		for missing := range wantCalls {
			t.Errorf("CallTest.caller.Calls missing %q", missing)
		}
	})

	t.Run("caller Calls is sorted", func(t *testing.T) {
		n := byName["CallTest.caller"]
		for i := 1; i < len(n.Calls); i++ {
			if n.Calls[i] < n.Calls[i-1] {
				t.Errorf("Calls not sorted at [%d]: %q > %q", i, n.Calls[i-1], n.Calls[i])
			}
		}
	})

	t.Run("callee has no calls", func(t *testing.T) {
		n := byName["CallTest.callee"]
		if len(n.Calls) != 0 {
			t.Errorf("callee.Calls = %v, want empty", n.Calls)
		}
	})
}
