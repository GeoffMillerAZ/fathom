package extractor_test

import (
	"reflect"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/extractor"
)

// fixture is a small Go source file that exercises all symbol kinds and a call edge.
const fixture = `package mypkg

import "fmt"

// Greeter is an exported type.
type Greeter struct {
	Name string
}

// Speaker is an interface.
type Speaker interface {
	Speak() string
}

// DefaultTimeout is an exported const.
const DefaultTimeout = 30

// Version is an exported var.
var Version = "1.0"

// Hello is a top-level function that calls Goodbye.
func Hello(name string) string {
	return Goodbye(name)
}

// Goodbye is a top-level function that calls fmt.Println.
func Goodbye(name string) string {
	fmt.Println(name)
	return "bye " + name
}

// Greet is a method on Greeter.
func (g *Greeter) Greet() string {
	return Hello(g.Name)
}
`

func TestGoExtractor_Symbols(t *testing.T) {
	x := extractor.NewGoExtractor()
	nodes, err := x.Extract("mysvc", "pkg/mypkg/mypkg.go", []byte(fixture))
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}

	// Build a map by Name for easy lookup.
	byName := make(map[string]core.SymbolNode, len(nodes))
	for _, n := range nodes {
		byName[n.Name] = n
	}

	// --- type ---
	t.Run("GoType", func(t *testing.T) {
		n, ok := byName["Greeter"]
		if !ok {
			t.Fatal("expected symbol Greeter")
		}
		if n.Kind != core.KindGoType {
			t.Errorf("Kind: got %q, want %q", n.Kind, core.KindGoType)
		}
		if n.Package != "mypkg" {
			t.Errorf("Package: got %q, want %q", n.Package, "mypkg")
		}
		if n.FilePath != "pkg/mypkg/mypkg.go" {
			t.Errorf("FilePath: got %q, want %q", n.FilePath, "pkg/mypkg/mypkg.go")
		}
		if n.Line < 1 {
			t.Errorf("Line: got %d, want >= 1", n.Line)
		}
		if n.Tier != core.TierVerified {
			t.Errorf("Tier: got %q, want %q", n.Tier, core.TierVerified)
		}
		if n.Service != "mysvc" {
			t.Errorf("Service: got %q, want %q", n.Service, "mysvc")
		}
	})

	// --- interface ---
	t.Run("GoInterface", func(t *testing.T) {
		n, ok := byName["Speaker"]
		if !ok {
			t.Fatal("expected symbol Speaker")
		}
		if n.Kind != core.KindGoInterface {
			t.Errorf("Kind: got %q, want %q", n.Kind, core.KindGoInterface)
		}
	})

	// --- const ---
	t.Run("GoConst", func(t *testing.T) {
		n, ok := byName["DefaultTimeout"]
		if !ok {
			t.Fatal("expected symbol DefaultTimeout")
		}
		if n.Kind != core.KindGoConst {
			t.Errorf("Kind: got %q, want %q", n.Kind, core.KindGoConst)
		}
	})

	// --- var ---
	t.Run("GoVar", func(t *testing.T) {
		n, ok := byName["Version"]
		if !ok {
			t.Fatal("expected symbol Version")
		}
		if n.Kind != core.KindGoVar {
			t.Errorf("Kind: got %q, want %q", n.Kind, core.KindGoVar)
		}
	})

	// --- func ---
	t.Run("GoFunc", func(t *testing.T) {
		n, ok := byName["Hello"]
		if !ok {
			t.Fatal("expected symbol Hello")
		}
		if n.Kind != core.KindGoFunc {
			t.Errorf("Kind: got %q, want %q", n.Kind, core.KindGoFunc)
		}
		if n.Signature == "" {
			t.Error("Signature: want non-empty")
		}
	})

	// --- method ---
	t.Run("GoMethod", func(t *testing.T) {
		n, ok := byName["Greeter.Greet"]
		if !ok {
			t.Fatal("expected symbol Greeter.Greet")
		}
		if n.Kind != core.KindGoMethod {
			t.Errorf("Kind: got %q, want %q", n.Kind, core.KindGoMethod)
		}
		if n.Package != "Greeter" {
			t.Errorf("Package (receiver qualifier): got %q, want %q", n.Package, "Greeter")
		}
	})
}

func TestGoExtractor_CallEdges(t *testing.T) {
	x := extractor.NewGoExtractor()
	nodes, err := x.Extract("mysvc", "pkg/mypkg/mypkg.go", []byte(fixture))
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}

	byName := make(map[string]core.SymbolNode, len(nodes))
	for _, n := range nodes {
		byName[n.Name] = n
	}

	// Hello calls Goodbye → expect Goodbye's key in Hello.Calls.
	hello, ok := byName["Hello"]
	if !ok {
		t.Fatal("expected symbol Hello")
	}
	goodbyeKey := core.SymbolKey("mysvc", "mypkg", "Goodbye")
	found := false
	for _, c := range hello.Calls {
		if c == goodbyeKey {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Hello.Calls: want %q in %v", goodbyeKey, hello.Calls)
	}

	// Calls must be sorted (determinism).
	sorted := make([]string, len(hello.Calls))
	copy(sorted, hello.Calls)
	sortStrings(sorted)
	if !reflect.DeepEqual(hello.Calls, sorted) {
		t.Errorf("Hello.Calls not sorted: %v", hello.Calls)
	}
}

func TestGoExtractor_MethodCallEdge(t *testing.T) {
	x := extractor.NewGoExtractor()
	nodes, err := x.Extract("mysvc", "pkg/mypkg/mypkg.go", []byte(fixture))
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}

	byName := make(map[string]core.SymbolNode, len(nodes))
	for _, n := range nodes {
		byName[n.Name] = n
	}

	// Greeter.Greet calls Hello.
	greet, ok := byName["Greeter.Greet"]
	if !ok {
		t.Fatal("expected symbol Greeter.Greet")
	}
	helloKey := core.SymbolKey("mysvc", "mypkg", "Hello")
	found := false
	for _, c := range greet.Calls {
		if c == helloKey {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Greeter.Greet.Calls: want %q in %v", helloKey, greet.Calls)
	}
}

func TestGoExtractor_Determinism(t *testing.T) {
	x := extractor.NewGoExtractor()
	nodes1, err := x.Extract("mysvc", "pkg/mypkg/mypkg.go", []byte(fixture))
	if err != nil {
		t.Fatalf("first Extract: %v", err)
	}
	nodes2, err := x.Extract("mysvc", "pkg/mypkg/mypkg.go", []byte(fixture))
	if err != nil {
		t.Fatalf("second Extract: %v", err)
	}
	if !reflect.DeepEqual(nodes1, nodes2) {
		t.Errorf("non-deterministic: first=%v second=%v", nodes1, nodes2)
	}
}

func TestGoExtractor_LineNumbers(t *testing.T) {
	x := extractor.NewGoExtractor()
	nodes, err := x.Extract("mysvc", "pkg/mypkg/mypkg.go", []byte(fixture))
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}

	byName := make(map[string]core.SymbolNode, len(nodes))
	for _, n := range nodes {
		byName[n.Name] = n
	}

	// Greeter struct is declared on line 6 (1-based).
	if n, ok := byName["Greeter"]; ok {
		if n.Line != 6 {
			t.Errorf("Greeter line: got %d, want 6", n.Line)
		}
	} else {
		t.Fatal("expected symbol Greeter")
	}

	// Hello func is declared on line 22.
	if n, ok := byName["Hello"]; ok {
		if n.Line != 22 {
			t.Errorf("Hello line: got %d, want 22", n.Line)
		}
	} else {
		t.Fatal("expected symbol Hello")
	}
}

func TestGoExtractor_Signature(t *testing.T) {
	x := extractor.NewGoExtractor()
	nodes, err := x.Extract("mysvc", "pkg/mypkg/mypkg.go", []byte(fixture))
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}

	byName := make(map[string]core.SymbolNode, len(nodes))
	for _, n := range nodes {
		byName[n.Name] = n
	}

	hello := byName["Hello"]
	if hello.Signature == "" {
		t.Error("Hello.Signature: want non-empty")
	}
	// Signature should contain parameter and result info.
	wantSig := "func Hello(name string) string"
	if hello.Signature != wantSig {
		t.Errorf("Hello.Signature: got %q, want %q", hello.Signature, wantSig)
	}
}

func TestGoExtractor_UnresolvedCallsRecorded(t *testing.T) {
	// Goodbye calls fmt.Println — unresolved (external package).
	// We expect it to appear as the bare callee name or qualified.
	x := extractor.NewGoExtractor()
	nodes, err := x.Extract("mysvc", "pkg/mypkg/mypkg.go", []byte(fixture))
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}

	byName := make(map[string]core.SymbolNode, len(nodes))
	for _, n := range nodes {
		byName[n.Name] = n
	}

	goodbye, ok := byName["Goodbye"]
	if !ok {
		t.Fatal("expected symbol Goodbye")
	}
	// fmt.Println is an external call — should appear in Calls in some form.
	found := false
	for _, c := range goodbye.Calls {
		if c == "fmt.Println" || c == "Println" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Goodbye.Calls: want fmt.Println or Println in %v", goodbye.Calls)
	}
}

// sortStrings sorts a string slice in place (stdlib only).
func sortStrings(s []string) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j] < s[j-1]; j-- {
			s[j], s[j-1] = s[j-1], s[j]
		}
	}
}

// complexityFixture has known cyclomatic complexity values for assertions.
// complexFunc: 1 (base) + 1 (if) + 1 (for) + 1 (case) + 1 (case) + 1 (&&) + 1 (||) = 7
const complexityFixture = `package cpkg

func complexFunc(x, y int) int {
	if x > 0 && y > 0 || x < -1 {
		for i := 0; i < x; i++ {
			_ = i
		}
	}
	switch x {
	case 1:
		return 1
	case 2:
		return 2
	}
	return 0
}

func simpleFunc() {}
`

func TestGoExtractor_Complexity(t *testing.T) {
	x := extractor.NewGoExtractor()
	nodes, err := x.Extract("svc", "pkg/cpkg/cpkg.go", []byte(complexityFixture))
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}

	byName := make(map[string]core.SymbolNode, len(nodes))
	for _, n := range nodes {
		byName[n.Name] = n
	}

	// complexFunc: base=1, if=1, &&=1, ||=1, for=1, case1=1, case2=1 → 7
	cf, ok := byName["complexFunc"]
	if !ok {
		t.Fatal("expected symbol complexFunc")
	}
	const wantComplex = 7
	if cf.Complexity != wantComplex {
		t.Errorf("complexFunc.Complexity = %d, want %d", cf.Complexity, wantComplex)
	}

	// simpleFunc: no branches → complexity = 1
	sf, ok := byName["simpleFunc"]
	if !ok {
		t.Fatal("expected symbol simpleFunc")
	}
	if sf.Complexity != 1 {
		t.Errorf("simpleFunc.Complexity = %d, want 1", sf.Complexity)
	}

	// Non-func symbols (types, vars, consts) should have complexity 0.
	// (complexityFixture has no types; the assertion is about the func nodes being ≥ 1.)
	for _, n := range nodes {
		if n.Kind == core.KindGoFunc || n.Kind == core.KindGoMethod {
			if n.Complexity < 1 {
				t.Errorf("func/method %q has Complexity %d, want >= 1", n.Name, n.Complexity)
			}
		}
	}
}
