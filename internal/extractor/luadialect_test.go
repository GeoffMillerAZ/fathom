package extractor

// luadialect_test.go — LB-Q-lua (design §13.2 steps 10, 12 and 15). The per-fixture goldens run in the
// generic TestDialectGoldens (dialects/lua/testdata); this file holds a port's extra obligation: the
// contracts luaextract_test.go pinned for the CGO walker must hold for the dialect too, and what the
// dialect adds beyond the walker is pinned by name.

import (
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
)

// luaContractFixture and luaContractCalls are the walker's own fixtures (luaextract_test.go), kept
// here so the contract survives the walker's removal.
const luaContractFixture = `-- global function
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

const luaContractCalls = `
function callee() end
function helper() end
function caller()
    callee()
    helper()
end
`

func luaContractExtractors(t *testing.T) map[string]Extractor {
	t.Helper()
	d, err := NewTSExtractor("lua")
	if err != nil {
		t.Fatalf("NewTSExtractor(lua): %v", err)
	}
	t.Cleanup(d.Close)
	return map[string]Extractor{"incumbent": NewLuaExtractor(), "dialect": d}
}

func luaExtract(t *testing.T, e Extractor, path, src string) (map[string]core.SymbolNode, []core.SymbolNode) {
	t.Helper()
	nodes, err := e.Extract("svc", path, []byte(src))
	if err != nil {
		t.Fatalf("Extract %s: %v", path, err)
	}
	m := map[string]core.SymbolNode{}
	for _, n := range nodes {
		m[n.Name] = n
	}
	return m, nodes
}

// TestLuaDialectKeepsIncumbentContracts: the names (`MyModule.method`, `MyClass:new`, written as the
// declaration spells them), the one kind, tier, plumbing, order, determinism and the call edges the walker's
// tests pin. KindLuaFunction's string is byte-identical (§11 step 3).
func TestLuaDialectKeepsIncumbentContracts(t *testing.T) {
	for who, e := range luaContractExtractors(t) {
		t.Run(who+"/symbols", func(t *testing.T) {
			seen, nodes := luaExtract(t, e, "src/init.lua", luaContractFixture)
			for _, name := range []string{"hello", "localFn", "MyModule.method", "MyClass:new"} {
				n, ok := seen[name]
				if !ok {
					t.Errorf("missing symbol %q", name)
					continue
				}
				if n.Kind != core.KindLuaFunction || n.Tier != core.TierVerified || n.Service != "svc" || n.FilePath != "src/init.lua" || n.EndLine <= 0 {
					t.Errorf("%s: kind %q tier %q service %q file %q end %d", name, n.Kind, n.Tier, n.Service, n.FilePath, n.EndLine)
				}
			}
			for i := 1; i < len(nodes); i++ {
				if nodes[i].Line < nodes[i-1].Line {
					t.Errorf("not sorted by line at %d: %d after %d", i, nodes[i].Line, nodes[i-1].Line)
				}
			}
		})
		t.Run(who+"/empty and determinism", func(t *testing.T) {
			if _, nodes := luaExtract(t, e, "empty.lua", ""); len(nodes) != 0 {
				t.Errorf("empty source: %d nodes", len(nodes))
			}
			_, a := luaExtract(t, e, "lib.lua", luaContractFixture)
			_, b := luaExtract(t, e, "lib.lua", luaContractFixture)
			if len(a) != len(b) {
				t.Fatalf("non-deterministic: %d vs %d", len(a), len(b))
			}
			for i := range a {
				if a[i].Name != b[i].Name || a[i].Kind != b[i].Kind || a[i].Line != b[i].Line {
					t.Errorf("index %d differs: %+v vs %+v", i, a[i], b[i])
				}
			}
		})
		t.Run(who+"/call edges", func(t *testing.T) {
			seen, _ := luaExtract(t, e, "src/calls.lua", luaContractCalls)
			if c := seen["caller"]; strings.Join(c.Calls, ",") != "callee,helper" {
				t.Errorf("caller.Calls = %v, want [callee helper] sorted", c.Calls)
			}
			if c := seen["callee"]; len(c.Calls) != 0 {
				t.Errorf("callee.Calls = %v, want none", c.Calls)
			}
		})
	}
}

// TestLuaDialectBeyondIncumbent: what a port may add (§11 step 3) — nested declarations (the walker
// never entered a function body), functions assigned to a variable, a field or a string key, a Signature
// on every callable that stops at the parameters, a call named by its callee's last identifier, and no
// definition for a field keyed by an expression.
func TestLuaDialectBeyondIncumbent(t *testing.T) {
	e, err := NewTSExtractor("lua")
	if err != nil {
		t.Fatal(err)
	}
	defer e.Close()
	const src = `local M = {}

function M.outer(a)
  -- a comment opens the body
  local function inner(b)
    return b
  end
  local anon = function() return M.helper(a) end
  return inner(a) + anon()
end

M.assigned = function(x,
                      y)
  return obj:method(x) + string.rep("a", y)
end

local handlers = {
  on_open = function() end,
  ["on-close"] = function() end,
  [KEY] = function() end,
  plain = 1,
}
`
	seen, nodes := luaExtract(t, e, "src/m.lua", src)
	for name, sig := range map[string]string{
		"M.outer":    "function M.outer(a)",
		"inner":      "local function inner(b)",
		"anon":       "local anon = function()",
		"M.assigned": "M.assigned = function(x, y)",
		"on_open":    "on_open = function()",
		"on-close":   `["on-close"] = function()`,
	} {
		n, ok := seen[name]
		if !ok {
			t.Errorf("missing %q", name)
			continue
		}
		if n.Kind != core.KindLuaFunction || n.Signature != sig {
			t.Errorf("%s: kind %q signature %q, want %q", name, n.Kind, n.Signature, sig)
		}
	}
	for _, name := range []string{"KEY", "plain", "handlers", "M"} {
		if _, ok := seen[name]; ok {
			t.Errorf("%q is not a function definition", name)
		}
	}
	for _, n := range nodes {
		if n.Signature == "" || n.EndLine < n.Line {
			t.Errorf("%s: signature %q lines %d-%d", n.Name, n.Signature, n.Line, n.EndLine)
		}
	}
	if got := strings.Join(seen["M.outer"].Calls, ","); got != "anon,inner" {
		t.Errorf("M.outer.Calls = %v, want [anon inner] (a call inside a nested definition belongs to it)", got)
	}
	if got := strings.Join(seen["anon"].Calls, ","); got != "helper" {
		t.Errorf("anon.Calls = %v, want [helper] (M.helper is named by its last identifier)", got)
	}
	if got := strings.Join(seen["M.assigned"].Calls, ","); got != "method,rep" {
		t.Errorf("M.assigned.Calls = %v, want [method rep] (obj:method and string.rep by their last identifiers)", got)
	}
}
