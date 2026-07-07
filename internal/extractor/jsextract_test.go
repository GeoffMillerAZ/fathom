package extractor_test

import (
	"testing"

	"github.com/geoffmilleraz/fathom/internal/analysis/callgraph"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/extractor"
)

// jsFixture is a plain JavaScript module covering function declarations, class
// declarations with methods, and exported arrow-function consts.
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
		if n.Tier != core.TierVerified {
			t.Errorf("Tier: got %q, want TierVerified", n.Tier)
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
		if n.Tier != core.TierVerified {
			t.Errorf("Tier: got %q, want TierVerified", n.Tier)
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
		if n.Tier != core.TierVerified {
			t.Errorf("Tier: got %q, want TierVerified", n.Tier)
		}
	})

	t.Run("method Calculator.subtract", func(t *testing.T) {
		n, ok := byName["Calculator.subtract"]
		if !ok {
			t.Fatal("expected symbol Calculator.subtract")
		}
		if n.Tier != core.TierVerified {
			t.Errorf("Tier: got %q, want TierVerified", n.Tier)
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
		if n.Tier != core.TierVerified {
			t.Errorf("Tier: got %q, want TierVerified", n.Tier)
		}
	})
}

// tsFixture exercises TypeScript-specific declarations: interface, type alias,
// enum, as well as class methods and exported arrow consts.
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

interface Repository<T> {
  findById(id: string): Promise<T>;
  save(item: T): void;
}

type UserID = string;

enum Status {
  Active = "active",
  Inactive = "inactive",
}
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

	t.Run("class UserService", func(t *testing.T) {
		n, ok := byName["UserService"]
		if !ok {
			t.Fatal("expected symbol UserService")
		}
		if n.Kind != core.KindJSClass {
			t.Errorf("Kind: got %q, want KindJSClass", n.Kind)
		}
		if n.Tier != core.TierVerified {
			t.Errorf("Tier: got %q, want TierVerified", n.Tier)
		}
	})

	t.Run("method UserService.findById", func(t *testing.T) {
		if _, ok := byName["UserService.findById"]; !ok {
			t.Error("expected symbol UserService.findById")
		}
	})

	t.Run("method UserService.delete", func(t *testing.T) {
		if _, ok := byName["UserService.delete"]; !ok {
			t.Error("expected symbol UserService.delete")
		}
	})

	t.Run("arrow const parseToken", func(t *testing.T) {
		n, ok := byName["parseToken"]
		if !ok {
			t.Fatal("expected symbol parseToken")
		}
		if n.Kind != core.KindJSFunc {
			t.Errorf("Kind: got %q, want KindJSFunc", n.Kind)
		}
		if n.Tier != core.TierVerified {
			t.Errorf("Tier: got %q, want TierVerified", n.Tier)
		}
	})

	t.Run("interface Repository", func(t *testing.T) {
		n, ok := byName["Repository"]
		if !ok {
			t.Fatal("expected symbol Repository")
		}
		if n.Kind != core.KindTSInterface {
			t.Errorf("Kind: got %q, want KindTSInterface", n.Kind)
		}
		if n.Tier != core.TierVerified {
			t.Errorf("Tier: got %q, want TierVerified", n.Tier)
		}
	})

	t.Run("type alias UserID", func(t *testing.T) {
		n, ok := byName["UserID"]
		if !ok {
			t.Fatal("expected symbol UserID")
		}
		if n.Kind != core.KindTSType {
			t.Errorf("Kind: got %q, want KindTSType", n.Kind)
		}
		if n.Tier != core.TierVerified {
			t.Errorf("Tier: got %q, want TierVerified", n.Tier)
		}
	})

	t.Run("enum Status", func(t *testing.T) {
		n, ok := byName["Status"]
		if !ok {
			t.Fatal("expected symbol Status")
		}
		if n.Kind != core.KindTSEnum {
			t.Errorf("Kind: got %q, want KindTSEnum", n.Kind)
		}
		if n.Tier != core.TierVerified {
			t.Errorf("Tier: got %q, want TierVerified", n.Tier)
		}
	})
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
			t.Errorf("non-deterministic at [%d]: %q vs %q", i, n1[i].Name, n2[i].Name)
		}
	}
}

// jsCallFixture exercises call-edge extraction for plain JS.
// - callee() is called by caller() and by the arrow const arrowCaller.
// - callee itself makes no calls.
const jsCallFixture = `function callee() {}

function helper() {}

function caller() {
  callee();
  helper();
}

const arrowCaller = () => {
  callee();
};

class MyService {
  doWork() {
    callee();
  }
}
`

func TestJSExtractor_CallEdges(t *testing.T) {
	x := extractor.NewJSExtractor()
	nodes, err := x.Extract("svc", "edges.js", []byte(jsCallFixture))
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

	t.Run("arrowCaller calls callee", func(t *testing.T) {
		n, ok := byName["arrowCaller"]
		if !ok {
			t.Fatal("expected symbol arrowCaller")
		}
		found := false
		for _, c := range n.Calls {
			if c == "callee" {
				found = true
			}
		}
		if !found {
			t.Errorf("arrowCaller.Calls = %v, want to contain %q", n.Calls, "callee")
		}
	})

	t.Run("method MyService.doWork calls callee", func(t *testing.T) {
		n, ok := byName["MyService.doWork"]
		if !ok {
			t.Fatal("expected symbol MyService.doWork")
		}
		found := false
		for _, c := range n.Calls {
			if c == "callee" {
				found = true
			}
		}
		if !found {
			t.Errorf("MyService.doWork.Calls = %v, want to contain %q", n.Calls, "callee")
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
		// expected callers: svc::edges.caller, svc::edges.arrowCaller, svc::edges.MyService.doWork
		callerKey := "svc::edges.caller"
		arrowKey := "svc::edges.arrowCaller"
		methodKey := "svc::edges.MyService.doWork"
		impactSet := make(map[string]bool, len(impact))
		for _, k := range impact {
			impactSet[k] = true
		}
		for _, want := range []string{callerKey, arrowKey, methodKey} {
			if !impactSet[want] {
				t.Errorf("RenameImpact(%q) = %v, want to include %q", "callee", impact, want)
			}
		}
	})
}
