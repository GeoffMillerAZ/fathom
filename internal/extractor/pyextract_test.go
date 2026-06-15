package extractor_test

import (
	"testing"

	"github.com/geoffmilleraz/sextant/internal/core"
	"github.com/geoffmilleraz/sextant/internal/extractor"
)

const pyFixture = `# Module-level comment
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
		if n.Tier != core.TierPreScan {
			t.Errorf("Tier: got %q, want TierPreScan", n.Tier)
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

	// Animal class is on line 2.
	if n, ok := byName["Animal"]; ok {
		if n.Line != 2 {
			t.Errorf("Animal line: got %d, want 2", n.Line)
		}
	}

	// standalone_func is on line 15.
	if n, ok := byName["standalone_func"]; ok {
		if n.Line != 15 {
			t.Errorf("standalone_func line: got %d, want 15", n.Line)
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
