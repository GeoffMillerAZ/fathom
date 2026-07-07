package extractor

import (
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
)

// csharpFixture is a deterministic C# source fragment exercising all symbol
// types the CSharpExtractor emits: namespace, class with method + constructor,
// interface, struct, and enum.
const csharpFixture = `namespace MyApp {
    class MyClass {
        public void DoThing() {}
        public MyClass() {}
    }
    interface IFoo {}
    struct MyStruct {}
    enum MyEnum { A, B }
}
`

func TestCSharpExtractor_BasicSymbols(t *testing.T) {
	e := NewCSharpExtractor()
	nodes, err := e.Extract("svc", "src/MyApp.cs", []byte(csharpFixture))
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}

	byName := make(map[string]core.SymbolNode, len(nodes))
	for _, n := range nodes {
		byName[n.Name] = n
	}

	want := []struct {
		name string
		kind core.SourceKind
	}{
		{"MyApp", core.KindCSharpNamespace},
		{"MyClass", core.KindCSharpClass},
		{"MyClass.DoThing", core.KindCSharpMethod},
		{"MyClass.MyClass", core.KindCSharpMethod}, // constructor
		{"IFoo", core.KindCSharpInterface},
		{"MyStruct", core.KindCSharpStruct},
		{"MyEnum", core.KindCSharpEnum},
	}

	for _, w := range want {
		n, ok := byName[w.name]
		if !ok {
			t.Errorf("missing symbol %q", w.name)
			continue
		}
		if n.Kind != w.kind {
			t.Errorf("symbol %q: want Kind %q, got %q", w.name, w.kind, n.Kind)
		}
		if n.Tier != core.TierVerified {
			t.Errorf("symbol %q: want TierVerified, got %q", w.name, n.Tier)
		}
		if n.Service != "svc" {
			t.Errorf("symbol %q: want Service %q, got %q", w.name, "svc", n.Service)
		}
		if n.FilePath != "src/MyApp.cs" {
			t.Errorf("symbol %q: want FilePath %q, got %q", w.name, "src/MyApp.cs", n.FilePath)
		}
	}
}

func TestCSharpExtractor_MethodQualification(t *testing.T) {
	src := []byte(`class Outer { class Inner { public void Act() {} } }`)
	e := NewCSharpExtractor()
	nodes, err := e.Extract("svc", "file.cs", src)
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	byName := make(map[string]core.SymbolNode, len(nodes))
	for _, n := range nodes {
		byName[n.Name] = n
	}
	if _, ok := byName["Outer"]; !ok {
		t.Error("expected Outer class")
	}
	if _, ok := byName["Outer.Inner"]; !ok {
		t.Error("expected Outer.Inner nested class")
	}
	if _, ok := byName["Outer.Inner.Act"]; !ok {
		t.Error("expected Outer.Inner.Act method")
	}
}

func TestCSharpExtractor_RecordDeclaration(t *testing.T) {
	src := []byte(`record MyRecord(int X, string Y);`)
	e := NewCSharpExtractor()
	nodes, err := e.Extract("svc", "records.cs", src)
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	found := false
	for _, n := range nodes {
		if n.Name == "MyRecord" && n.Kind == core.KindCSharpClass {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected MyRecord as KindCSharpClass (record_declaration)")
	}
}

func TestCSharpExtractor_TierVerified(t *testing.T) {
	e := NewCSharpExtractor()
	nodes, err := e.Extract("svc", "app.cs", []byte(csharpFixture))
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	for _, n := range nodes {
		if n.Tier != core.TierVerified {
			t.Errorf("symbol %q: expected TierVerified, got %q", n.Name, n.Tier)
		}
	}
}

func TestCSharpExtractor_Determinism(t *testing.T) {
	e := NewCSharpExtractor()
	a, err := e.Extract("svc", "app.cs", []byte(csharpFixture))
	if err != nil {
		t.Fatalf("first Extract: %v", err)
	}
	b, err := e.Extract("svc", "app.cs", []byte(csharpFixture))
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

func TestCSharpExtractor_EmptySource(t *testing.T) {
	e := NewCSharpExtractor()
	nodes, err := e.Extract("svc", "empty.cs", []byte{})
	if err != nil {
		t.Fatalf("Extract empty: %v", err)
	}
	if len(nodes) != 0 {
		t.Errorf("expected 0 nodes for empty source, got %d", len(nodes))
	}
}

// csharpCallFixture is a minimal C# file for verifying call-edge extraction.
// Caller() calls Helper() and Console.WriteLine — the invocation_expression
// "function" field captures "Helper" and "Console.WriteLine" respectively.
const csharpCallFixture = `class CallTest {
    public void Caller() {
        Helper();
        Console.WriteLine("hi");
    }
    private void Helper() {}
}
`

func TestCSharpExtractor_CallEdges(t *testing.T) {
	e := NewCSharpExtractor()
	nodes, err := e.Extract("svc", "CallTest.cs", []byte(csharpCallFixture))
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}

	byName := make(map[string]core.SymbolNode, len(nodes))
	for _, n := range nodes {
		byName[n.Name] = n
	}

	t.Run("Caller calls Helper", func(t *testing.T) {
		n, ok := byName["CallTest.Caller"]
		if !ok {
			t.Fatal("expected symbol CallTest.Caller")
		}
		found := false
		for _, c := range n.Calls {
			if c == "Helper" {
				found = true
			}
		}
		if !found {
			t.Errorf("CallTest.Caller.Calls = %v, want to contain %q", n.Calls, "Helper")
		}
	})

	t.Run("Caller Calls is sorted", func(t *testing.T) {
		n := byName["CallTest.Caller"]
		for i := 1; i < len(n.Calls); i++ {
			if n.Calls[i] < n.Calls[i-1] {
				t.Errorf("Calls not sorted at [%d]: %q > %q", i, n.Calls[i-1], n.Calls[i])
			}
		}
	})

	t.Run("Helper has no calls", func(t *testing.T) {
		n := byName["CallTest.Helper"]
		if len(n.Calls) != 0 {
			t.Errorf("Helper.Calls = %v, want empty", n.Calls)
		}
	})
}
