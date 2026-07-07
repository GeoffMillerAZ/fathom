package extractor

import (
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
)

// luaFixture is a deterministic Lua source fragment exercising all function
// forms the LuaExtractor emits: global, local, module-dotted, and colon-method.
const luaFixture = `-- global function
function hello(name)
  print(name)
end

-- local function (also a function_declaration in the grammar)
local function localFn()
  return 42
end

-- module-style dotted function
function MyModule.method()
  return true
end

-- colon-style method
function MyClass:new()
  return {}
end
`

func TestLuaExtractor_BasicSymbols(t *testing.T) {
	e := NewLuaExtractor()
	nodes, err := e.Extract("svc", "src/init.lua", []byte(luaFixture))
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}

	byName := make(map[string]core.SymbolNode, len(nodes))
	for _, n := range nodes {
		byName[n.Name] = n
	}

	want := []string{"hello", "localFn", "MyModule.method", "MyClass:new"}
	for _, name := range want {
		n, ok := byName[name]
		if !ok {
			t.Errorf("missing symbol %q", name)
			continue
		}
		if n.Kind != core.KindLuaFunction {
			t.Errorf("symbol %q: want KindLuaFunction, got %q", name, n.Kind)
		}
		if n.Tier != core.TierVerified {
			t.Errorf("symbol %q: want TierVerified, got %q", name, n.Tier)
		}
		if n.Service != "svc" {
			t.Errorf("symbol %q: want Service %q, got %q", name, "svc", n.Service)
		}
		if n.FilePath != "src/init.lua" {
			t.Errorf("symbol %q: want FilePath %q, got %q", name, "src/init.lua", n.FilePath)
		}
	}
}

func TestLuaExtractor_TierVerified(t *testing.T) {
	e := NewLuaExtractor()
	nodes, err := e.Extract("svc", "main.lua", []byte(luaFixture))
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	for _, n := range nodes {
		if n.Tier != core.TierVerified {
			t.Errorf("symbol %q: expected TierVerified, got %q", n.Name, n.Tier)
		}
	}
}

func TestLuaExtractor_Determinism(t *testing.T) {
	e := NewLuaExtractor()
	a, err := e.Extract("svc", "lib.lua", []byte(luaFixture))
	if err != nil {
		t.Fatalf("first Extract: %v", err)
	}
	b, err := e.Extract("svc", "lib.lua", []byte(luaFixture))
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

func TestLuaExtractor_EmptySource(t *testing.T) {
	e := NewLuaExtractor()
	nodes, err := e.Extract("svc", "empty.lua", []byte{})
	if err != nil {
		t.Fatalf("Extract empty: %v", err)
	}
	if len(nodes) != 0 {
		t.Errorf("expected 0 nodes for empty source, got %d", len(nodes))
	}
}

// luaCallFixture has caller() calling callee() and helper() via bare
// function_call nodes. callee() and helper() make no calls.
const luaCallFixture = `
function callee() end
function helper() end
function caller()
    callee()
    helper()
end
`

func TestLuaExtractor_CallEdges(t *testing.T) {
	e := NewLuaExtractor()
	nodes, err := e.Extract("svc", "calls.lua", []byte(luaCallFixture))
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

	t.Run("callee has no calls", func(t *testing.T) {
		n := byName["callee"]
		if len(n.Calls) != 0 {
			t.Errorf("callee.Calls = %v, want empty", n.Calls)
		}
	})
}
