package extractor_test

import (
	"testing"

	"github.com/geoffmilleraz/fathom/internal/analysis/callgraph"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/extractor"
)

// pyFixture exercises: class, class method, module-level function, async def,
// @decorator-decorated function, nested function (NOT a method), and a
// module-level constant assignment.
const pyFixture = `# Module-level comment
MODULE_CONST = 42

class Animal:
    """Base class."""

    def __init__(self, name):
        self.name = name

    def speak(self):
        pass

class Dog(Animal):
    def speak(self):
        return "Woof"

def standalone_func(x, y):
    return x + y

async def async_helper():
    pass

@staticmethod
def decorated_func():
    pass

def outer():
    def inner():
        pass
    return inner
`

func TestPythonExtractor_Basic(t *testing.T) {
	x := extractor.NewPythonExtractor()
	nodes, err := x.Extract("svc", "animal.py", []byte(pyFixture))
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}

	byName := make(map[string]core.SymbolNode, len(nodes))
	for _, n := range nodes {
		byName[n.Name] = n
	}

	// classes
	t.Run("class Animal", func(t *testing.T) {
		n, ok := byName["Animal"]
		if !ok {
			t.Fatal("expected symbol Animal")
		}
		if n.Kind != core.KindPyClass {
			t.Errorf("Kind: got %q, want KindPyClass", n.Kind)
		}
		if n.Tier != core.TierVerified {
			t.Errorf("Tier: got %q, want TierVerified", n.Tier)
		}
	})

	t.Run("class Dog", func(t *testing.T) {
		n, ok := byName["Dog"]
		if !ok {
			t.Fatal("expected symbol Dog")
		}
		if n.Kind != core.KindPyClass {
			t.Errorf("Kind: got %q, want KindPyClass", n.Kind)
		}
		if n.Tier != core.TierVerified {
			t.Errorf("Tier: got %q, want TierVerified", n.Tier)
		}
	})

	// methods (class-qualified)
	t.Run("method Animal.__init__", func(t *testing.T) {
		n, ok := byName["Animal.__init__"]
		if !ok {
			t.Fatal("expected symbol Animal.__init__")
		}
		if n.Kind != core.KindPyFunc {
			t.Errorf("Kind: got %q, want KindPyFunc", n.Kind)
		}
		if n.Tier != core.TierVerified {
			t.Errorf("Tier: got %q, want TierVerified", n.Tier)
		}
	})

	t.Run("method Animal.speak", func(t *testing.T) {
		_, ok := byName["Animal.speak"]
		if !ok {
			t.Fatal("expected symbol Animal.speak")
		}
	})

	t.Run("method Dog.speak", func(t *testing.T) {
		_, ok := byName["Dog.speak"]
		if !ok {
			t.Fatal("expected symbol Dog.speak")
		}
	})

	// standalone func
	t.Run("standalone_func", func(t *testing.T) {
		n, ok := byName["standalone_func"]
		if !ok {
			t.Fatal("expected symbol standalone_func")
		}
		if n.Kind != core.KindPyFunc {
			t.Errorf("Kind: got %q, want KindPyFunc", n.Kind)
		}
		if n.Tier != core.TierVerified {
			t.Errorf("Tier: got %q, want TierVerified", n.Tier)
		}
	})

	// async func
	t.Run("async_helper", func(t *testing.T) {
		n, ok := byName["async_helper"]
		if !ok {
			t.Fatal("expected symbol async_helper")
		}
		if n.Kind != core.KindPyFunc {
			t.Errorf("Kind: got %q, want KindPyFunc", n.Kind)
		}
		if n.Tier != core.TierVerified {
			t.Errorf("Tier: got %q, want TierVerified", n.Tier)
		}
	})

	// @decorator-decorated function must be extracted
	t.Run("decorated_func", func(t *testing.T) {
		n, ok := byName["decorated_func"]
		if !ok {
			t.Fatal("expected symbol decorated_func (decorated with @staticmethod)")
		}
		if n.Kind != core.KindPyFunc {
			t.Errorf("Kind: got %q, want KindPyFunc", n.Kind)
		}
		if n.Tier != core.TierVerified {
			t.Errorf("Tier: got %q, want TierVerified", n.Tier)
		}
	})

	// nested function inside a function — NOT a method, no class qualification
	t.Run("nested inner function", func(t *testing.T) {
		// outer must exist as a plain func
		if _, ok := byName["outer"]; !ok {
			t.Fatal("expected symbol outer")
		}
		// inner must exist as a plain func, not as "outer.inner"
		if _, ok := byName["inner"]; !ok {
			t.Fatal("expected symbol inner (nested func, unqualified)")
		}
		if _, ok := byName["outer.inner"]; ok {
			t.Fatal("inner should NOT be qualified as outer.inner — only class methods get qualified")
		}
	})

	// module-level constant assignment → KindPyVar
	t.Run("MODULE_CONST", func(t *testing.T) {
		n, ok := byName["MODULE_CONST"]
		if !ok {
			t.Fatal("expected symbol MODULE_CONST (module-level assignment)")
		}
		if n.Kind != core.KindPyVar {
			t.Errorf("Kind: got %q, want KindPyVar", n.Kind)
		}
		if n.Tier != core.TierVerified {
			t.Errorf("Tier: got %q, want TierVerified", n.Tier)
		}
	})
}

func TestPythonExtractor_LineNumbers(t *testing.T) {
	x := extractor.NewPythonExtractor()
	nodes, err := x.Extract("svc", "animal.py", []byte(pyFixture))
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}

	byName := make(map[string]core.SymbolNode, len(nodes))
	for _, n := range nodes {
		byName[n.Name] = n
	}

	// MODULE_CONST is on line 2.
	if n, ok := byName["MODULE_CONST"]; ok {
		if n.Line != 2 {
			t.Errorf("MODULE_CONST line: got %d, want 2", n.Line)
		}
	}

	// Animal class is on line 4.
	if n, ok := byName["Animal"]; ok {
		if n.Line != 4 {
			t.Errorf("Animal line: got %d, want 4", n.Line)
		}
	}
}

func TestPythonExtractor_Determinism(t *testing.T) {
	x := extractor.NewPythonExtractor()
	n1, err := x.Extract("svc", "animal.py", []byte(pyFixture))
	if err != nil {
		t.Fatalf("first Extract: %v", err)
	}
	n2, err := x.Extract("svc", "animal.py", []byte(pyFixture))
	if err != nil {
		t.Fatalf("second Extract: %v", err)
	}
	if len(n1) != len(n2) {
		t.Fatalf("non-deterministic length: %d vs %d", len(n1), len(n2))
	}
	for i := range n1 {
		if n1[i].Name != n2[i].Name || n1[i].Line != n2[i].Line {
			t.Errorf("non-deterministic at [%d]: %+v vs %+v", i, n1[i], n2[i])
		}
	}
}

func TestPythonExtractor_TierVerified(t *testing.T) {
	x := extractor.NewPythonExtractor()
	nodes, err := x.Extract("svc", "animal.py", []byte(pyFixture))
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	for _, n := range nodes {
		if n.Tier != core.TierVerified {
			t.Errorf("symbol %q has Tier %q, want TierVerified", n.Name, n.Tier)
		}
	}
}

// pyCallFixture is a minimal Python file for verifying call-edge extraction.
// - caller() calls callee() and helper().
// - MyClass.method() calls callee() too (method-level call edge).
// - callee() and helper() make no calls (empty Calls).
const pyCallFixture = `def callee():
    pass

def helper():
    pass

def caller():
    callee()
    helper()

class MyClass:
    def method(self):
        callee()
`

func TestPythonExtractor_CallEdges(t *testing.T) {
	x := extractor.NewPythonExtractor()
	nodes, err := x.Extract("svc", "calls.py", []byte(pyCallFixture))
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

	t.Run("method calls callee", func(t *testing.T) {
		n, ok := byName["MyClass.method"]
		if !ok {
			t.Fatal("expected symbol MyClass.method")
		}
		found := false
		for _, c := range n.Calls {
			if c == "callee" {
				found = true
			}
		}
		if !found {
			t.Errorf("MyClass.method.Calls = %v, want to contain %q", n.Calls, "callee")
		}
	})

	t.Run("callee has no calls", func(t *testing.T) {
		n := byName["callee"]
		if len(n.Calls) != 0 {
			t.Errorf("callee.Calls = %v, want empty", n.Calls)
		}
	})

	t.Run("callgraph RenameImpact resolves bare-name edge", func(t *testing.T) {
		g := callgraph.Build(nodes)
		impact := callgraph.RenameImpact(g, "callee")
		// callerKey = "svc::calls.caller", methodKey = "svc::calls.MyClass.method"
		callerKey := "svc::calls.caller"
		methodKey := "svc::calls.MyClass.method"
		foundCaller := false
		foundMethod := false
		for _, k := range impact {
			if k == callerKey {
				foundCaller = true
			}
			if k == methodKey {
				foundMethod = true
			}
		}
		if !foundCaller {
			t.Errorf("RenameImpact(%q) = %v, want to include %q", "callee", impact, callerKey)
		}
		if !foundMethod {
			t.Errorf("RenameImpact(%q) = %v, want to include %q", "callee", impact, methodKey)
		}
	})
}
