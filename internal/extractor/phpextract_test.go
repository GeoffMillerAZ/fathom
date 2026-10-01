package extractor

import (
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
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
		if n.EndLine <= 0 {
			t.Errorf("symbol %q: EndLine not set (got %d)", n.Name, n.EndLine)
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

// phpCallFixture has a top-level function caller() calling callee() via a plain
// function_call_expression, and a class method Svc.doWork() calling callee() the
// same way. callee() itself makes no calls.
const phpCallFixture = `<?php
function callee() {}
function helper() {}
function caller() {
    callee();
    helper();
}
class Svc {
    public function doWork() {
        callee();
    }
}
`

func TestPHPExtractor_CallEdges(t *testing.T) {
	e := NewPhpExtractor()
	nodes, err := e.Extract("svc", "calls.php", []byte(phpCallFixture))
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}

	byName := make(map[string]core.SymbolNode, len(nodes))
	for _, n := range nodes {
		byName[n.Name] = n
	}

	t.Run("caller calls callee and helper", func(t *testing.T) {
		n, ok := byName["caller"]
		if !ok {
			t.Fatal("expected symbol caller")
		}
		wantCalls := map[string]bool{"callee": true, "helper": true}
		for _, c := range n.Calls {
			if !wantCalls[c] {
				t.Errorf("unexpected call %q in caller.Calls", c)
			}
			delete(wantCalls, c)
		}
		for missing := range wantCalls {
			t.Errorf("caller.Calls missing %q", missing)
		}
	})

	t.Run("caller Calls is sorted", func(t *testing.T) {
		n := byName["caller"]
		for i := 1; i < len(n.Calls); i++ {
			if n.Calls[i] < n.Calls[i-1] {
				t.Errorf("Calls not sorted at [%d]: %q > %q", i, n.Calls[i-1], n.Calls[i])
			}
		}
	})

	t.Run("Svc.doWork calls callee", func(t *testing.T) {
		n, ok := byName["Svc.doWork"]
		if !ok {
			t.Fatal("expected symbol Svc.doWork")
		}
		found := false
		for _, c := range n.Calls {
			if c == "callee" {
				found = true
			}
		}
		if !found {
			t.Errorf("Svc.doWork.Calls = %v, want to contain %q", n.Calls, "callee")
		}
	})

	t.Run("callee has no calls", func(t *testing.T) {
		n := byName["callee"]
		if len(n.Calls) != 0 {
			t.Errorf("callee.Calls = %v, want empty", n.Calls)
		}
	})
}

// phpMembersFixture exercises every member kind scip-php defines beyond the
// callables: a plain and a multi-name property, a promoted constructor
// property, class and interface constants (single and grouped), and enum
// cases (pure and backed).
const phpMembersFixture = `<?php
namespace Acme;

interface Sized
{
    const SIDES = 4;
}

class Box
{
    public const KIND = 'box', ALT = 'b';
    private static int $count = 0;
    protected ?string $label = null, $note = null;

    public function __construct(
        private readonly int $width,
        protected int $height = 1,
        $plain = 0,
    ) {
    }

    public function area(): int { return $this->width * $this->height; }
}

trait Tagged
{
    public array $tags = [];
}

enum Suit: string
{
    case Hearts = 'H';
    case Spades = 'S';
    const Wild = self::Spades;

    public function label(): string { return ucfirst($this->name); }
}
`

// TestPhpExtractor_PropsConstsEnumCases: LB-F81 — properties (declared and
// promoted), class/interface constants and enum cases are definitions named
// Owner.name, a property with its `$` as scip-php writes it.
func TestPhpExtractor_PropsConstsEnumCases(t *testing.T) {
	e := NewPhpExtractor()
	nodes, err := e.Extract("svc", "src/Box.php", []byte(phpMembersFixture))
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	type key struct {
		name string
		kind core.SourceKind
	}
	seen := map[key]core.SymbolNode{}
	for _, n := range nodes {
		seen[key{n.Name, n.Kind}] = n
	}
	want := []struct {
		key
		line int
	}{
		{key{"Sized.SIDES", core.KindPhpConst}, 6},
		{key{"Box.KIND", core.KindPhpConst}, 11},
		{key{"Box.ALT", core.KindPhpConst}, 11},
		{key{"Box.$count", core.KindPhpProperty}, 12},
		{key{"Box.$label", core.KindPhpProperty}, 13},
		{key{"Box.$note", core.KindPhpProperty}, 13},
		{key{"Box.$width", core.KindPhpProperty}, 16},
		{key{"Box.$height", core.KindPhpProperty}, 17},
		{key{"Tagged.$tags", core.KindPhpProperty}, 27},
		{key{"Suit.Hearts", core.KindPhpEnumCase}, 32},
		{key{"Suit.Spades", core.KindPhpEnumCase}, 33},
		{key{"Suit.Wild", core.KindPhpConst}, 34},
		{key{"Suit.label", core.KindPhpMethod}, 36},
		{key{"Box.area", core.KindPhpMethod}, 22},
	}
	for _, w := range want {
		n, ok := seen[w.key]
		if !ok {
			t.Errorf("missing %+v", w.key)
			continue
		}
		if n.Line != w.line || n.EndLine < n.Line || n.Tier != core.TierVerified {
			t.Errorf("%+v: line %d end %d tier %q, want line %d", w.key, n.Line, n.EndLine, n.Tier, w.line)
		}
	}
	if _, ok := seen[key{"Box.$plain", core.KindPhpProperty}]; ok {
		t.Error("a plain (unpromoted) parameter is not a property")
	}
}
