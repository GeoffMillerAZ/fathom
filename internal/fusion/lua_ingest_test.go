package fusion_test

// lua_ingest_test.go — LB-Q-lua: a .lua file reaches the lua dialect through discovery and dispatch (the
// registry is built from the dialect manifests) once the CGO walker is retired. What only the dialect
// does — a nested declaration, a function assigned to a table field, a signature, a callee named by its
// last identifier — proves the dispatch is the dialect's, not the walker's.

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/fusion"
	"github.com/geoffmilleraz/fathom/internal/store"
)

func TestIngest_LuaReachesTheLuaDialect(t *testing.T) {
	dir := t.TempDir()
	src := `local M = {}

function M.run(opts)
  local function step(n)
    return n + 1
  end
  return step(opts.n) + util.count(opts)
end

local handlers = {
  on_open = function(ev) return ev end,
}

function M:close() end

return M
`
	p := filepath.Join(dir, "lua", "mod", "init.lua")
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	st := store.NewMemoryStore()
	svc := core.ServiceRef{ID: "lua-svc", Path: dir}
	pl := fusion.New(connector.NewLocalConnector(), st, fusion.Config{Concurrency: 1})
	ctx := context.Background()
	if _, err := pl.Ingest(ctx, svc); err != nil {
		t.Fatalf("Ingest: %v", err)
	}
	obs, err := st.QueryObservations(ctx, svc.ID, "")
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]core.Observation{}
	for _, o := range obs {
		got[o.Symbol] = o
	}
	for symbol, sig := range map[string]string{
		"M.run":   "function M.run(opts)",
		"step":    "local function step(n)",
		"on_open": "on_open = function(ev)",
		"M:close": "function M:close()",
	} {
		o, ok := got[symbol]
		if !ok {
			t.Errorf("no observation for %q (got %v)", symbol, keysOf(got))
			continue
		}
		if o.SourceKind != core.KindLuaFunction || o.Claim != sig || o.EndLine < o.Line {
			t.Errorf("%s: kind %q claim %q lines %d-%d, want LuaFunction %q", symbol, o.SourceKind, o.Claim, o.Line, o.EndLine, sig)
		}
	}
	if calls := strings.Join(got["M.run"].Calls, ","); calls != "count,step" {
		t.Errorf("M.run calls = %q, want count,step (a callee is its last identifier, a nested definition owns its own calls)", calls)
	}
}

func keysOf(m map[string]core.Observation) []string {
	var out []string
	for k := range m {
		out = append(out, k)
	}
	return out
}
