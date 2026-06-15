package extractor

import (
	"testing"

	"github.com/geoffmilleraz/sextant/internal/core"
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
