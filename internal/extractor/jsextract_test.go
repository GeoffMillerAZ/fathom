package extractor_test

import (
	"testing"

	"github.com/geoffmilleraz/sextant/internal/core"
	"github.com/geoffmilleraz/sextant/internal/extractor"
)

const jsFixture = `// A simple JS module

export function greet(name) {
  return "Hello " + name;
}

export const add = (a, b) => a + b;

class Calculator {
  add(a, b) {
    return a + b;
  }

  subtract(a, b) {
    return a - b;
  }
}

async function fetchData(url) {
  return fetch(url);
}
`

func TestJSExtractor_Basic(t *testing.T) {
	x := extractor.NewJSExtractor()
	nodes, err := x.Extract("svc", "calc.js", []byte(jsFixture))
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}

	byName := make(map[string]core.SymbolNode, len(nodes))
	for _, n := range nodes {
		byName[n.Name] = n
	}

	t.Run("function greet", func(t *testing.T) {
		n, ok := byName["greet"]
		if !ok {
			t.Fatal("expected symbol greet")
		}
		if n.Kind != core.KindJSFunc {
			t.Errorf("Kind: got %q, want KindJSFunc", n.Kind)
		}
		if n.Tier != core.TierVerified {
			t.Errorf("Tier: got %q, want TierVerified", n.Tier)
		}
	})

	t.Run("arrow const add", func(t *testing.T) {
		n, ok := byName["add"]
		if !ok {
			t.Fatal("expected symbol add")
		}
		if n.Kind != core.KindJSFunc {
			t.Errorf("Kind: got %q, want KindJSFunc", n.Kind)
		}
	})

	t.Run("class Calculator", func(t *testing.T) {
		n, ok := byName["Calculator"]
		if !ok {
			t.Fatal("expected symbol Calculator")
		}
		if n.Kind != core.KindJSClass {
			t.Errorf("Kind: got %q, want KindJSClass", n.Kind)
		}
	})

	t.Run("method Calculator.add", func(t *testing.T) {
		n, ok := byName["Calculator.add"]
		if !ok {
			t.Fatal("expected symbol Calculator.add")
		}
		if n.Kind != core.KindJSMethod {
			t.Errorf("Kind: got %q, want KindJSMethod", n.Kind)
		}
	})

	t.Run("method Calculator.subtract", func(t *testing.T) {
		_, ok := byName["Calculator.subtract"]
		if !ok {
			t.Fatal("expected symbol Calculator.subtract")
		}
	})

	t.Run("async function fetchData", func(t *testing.T) {
		n, ok := byName["fetchData"]
		if !ok {
			t.Fatal("expected symbol fetchData")
		}
		if n.Kind != core.KindJSFunc {
			t.Errorf("Kind: got %q, want KindJSFunc", n.Kind)
		}
	})
}

const tsFixture = `export class UserService {
  constructor(private db: Database) {}

  async findById(id: string): Promise<User> {
    return this.db.find(id);
  }

  delete(id: string): void {
    this.db.delete(id);
  }
}

export const parseToken = (token: string): Claims => {
  return JSON.parse(token);
};
`

func TestJSExtractor_TypeScript(t *testing.T) {
	x := extractor.NewJSExtractor()
	nodes, err := x.Extract("svc", "user.ts", []byte(tsFixture))
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}

	byName := make(map[string]core.SymbolNode, len(nodes))
	for _, n := range nodes {
		byName[n.Name] = n
	}

	if _, ok := byName["UserService"]; !ok {
		t.Error("expected symbol UserService")
	}
	if _, ok := byName["UserService.findById"]; !ok {
		t.Error("expected symbol UserService.findById")
	}
	if _, ok := byName["UserService.delete"]; !ok {
		t.Error("expected symbol UserService.delete")
	}
	if _, ok := byName["parseToken"]; !ok {
		t.Error("expected symbol parseToken")
	}
}

func TestJSExtractor_Determinism(t *testing.T) {
	x := extractor.NewJSExtractor()
	n1, err := x.Extract("svc", "calc.js", []byte(jsFixture))
	if err != nil {
		t.Fatalf("first Extract: %v", err)
	}
	n2, err := x.Extract("svc", "calc.js", []byte(jsFixture))
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
