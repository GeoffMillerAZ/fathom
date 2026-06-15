package extractor

import (
	"testing"

	"github.com/geoffmilleraz/sextant/internal/core"
)

// phpFixture is a deterministic PHP source fragment exercising all symbol types
// the PHPExtractor emits: class (with method), interface, trait, enum, and
// a top-level function.
const phpFixture = `<?php

class MyClass {
    public function myMethod(int $x): string {
        return (string) $x;
    }
}

interface MyInterface {
    public function doThing(): void;
}

trait MyTrait {
    public function helper(): int { return 1; }
}

enum Status {
    case Active;
    case Inactive;
}

function freeFunction(int $a): int {
    return $a + 1;
}
`

func TestPHPExtractor_BasicSymbols(t *testing.T) {
	e := NewPhpExtractor()
	nodes, err := e.Extract("svc", "src/App.php", []byte(phpFixture))
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}

	type key struct {
		name string
		kind core.SourceKind
	}
	seen := make(map[key]core.SymbolNode, len(nodes))
	for _, n := range nodes {
		seen[key{n.Name, n.Kind}] = n
	}

	want := []key{
		{"MyClass", core.KindPhpClass},
		{"MyClass.myMethod", core.KindPhpMethod},
		{"MyInterface", core.KindPhpInterface},
		{"MyInterface.doThing", core.KindPhpMethod},
		{"MyTrait", core.KindPhpTrait},
		{"MyTrait.helper", core.KindPhpMethod},
		{"Status", core.KindPhpEnum},
		{"freeFunction", core.KindPhpFunction},
	}

	for _, w := range want {
		n, ok := seen[w]
		if !ok {
			t.Errorf("missing symbol name=%q kind=%q", w.name, w.kind)
			continue
		}
		if n.Tier != core.TierVerified {
			t.Errorf("symbol %q: want TierVerified, got %q", w.name, n.Tier)
		}
		if n.Service != "svc" {
			t.Errorf("symbol %q: want Service %q, got %q", w.name, "svc", n.Service)
		}
		if n.FilePath != "src/App.php" {
			t.Errorf("symbol %q: want FilePath %q, got %q", w.name, "src/App.php", n.FilePath)
		}
	}
}

func TestPHPExtractor_MethodQualification(t *testing.T) {
	src := []byte(`<?php
class Calculator {
    public function add(int $a, int $b): int { return $a + $b; }
    public function sub(int $a, int $b): int { return $a - $b; }
}`)
	e := NewPhpExtractor()
	nodes, err := e.Extract("svc", "Calculator.php", src)
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	byName := make(map[string]core.SymbolNode, len(nodes))
	for _, n := range nodes {
		byName[n.Name] = n
	}
	if _, ok := byName["Calculator.add"]; !ok {
		t.Error("expected qualified symbol Calculator.add")
	}
	if _, ok := byName["Calculator.sub"]; !ok {
		t.Error("expected qualified symbol Calculator.sub")
	}
	if _, ok := byName["add"]; ok {
		t.Error("unqualified add should not appear")
	}
}

func TestPHPExtractor_TierVerified(t *testing.T) {
	e := NewPhpExtractor()
	nodes, err := e.Extract("svc", "App.php", []byte(phpFixture))
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	for _, n := range nodes {
		if n.Tier != core.TierVerified {
			t.Errorf("symbol %q: expected TierVerified, got %q", n.Name, n.Tier)
		}
	}
}

func TestPHPExtractor_Determinism(t *testing.T) {
	e := NewPhpExtractor()
	a, err := e.Extract("svc", "App.php", []byte(phpFixture))
	if err != nil {
		t.Fatalf("first Extract: %v", err)
	}
	b, err := e.Extract("svc", "App.php", []byte(phpFixture))
	if err != nil {
		t.Fatalf("second Extract: %v", err)
	}
	if len(a) != len(b) {
		t.Fatalf("non-deterministic: first run %d symbols, second %d", len(a), len(b))
	}
	for i := range a {
		if a[i].Name != b[i].Name || a[i].Kind != b[i].Kind || a[i].Line != b[i].Line {
			t.Errorf("index %d differs: %+v vs %+v", i, a[i], b[i])
		}
	}
}

func TestPHPExtractor_EmptySource(t *testing.T) {
	e := NewPhpExtractor()
	nodes, err := e.Extract("svc", "empty.php", []byte{})
	if err != nil {
		t.Fatalf("Extract empty: %v", err)
	}
	if len(nodes) != 0 {
		t.Errorf("expected 0 nodes for empty source, got %d", len(nodes))
	}
}
