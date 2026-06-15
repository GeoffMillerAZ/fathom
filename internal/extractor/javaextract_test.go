package extractor_test

import (
	"testing"

	"github.com/geoffmilleraz/sextant/internal/core"
	"github.com/geoffmilleraz/sextant/internal/extractor"
)

const javaFixture = `package com.example;

public class PaymentService {

    public void processPayment(String amount) {
        validate(amount);
    }

    private boolean validate(String amount) {
        return amount != null;
    }

    public interface Processor {
        void process(String amount);
    }
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

	t.Run("class PaymentService", func(t *testing.T) {
		n, ok := byName["PaymentService"]
		if !ok {
			t.Fatal("expected symbol PaymentService")
		}
		if n.Kind != core.KindJavaClass {
			t.Errorf("Kind: got %q, want KindJavaClass", n.Kind)
		}
		if n.Tier != core.TierPreScan {
			t.Errorf("Tier: got %q, want TierPreScan", n.Tier)
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
