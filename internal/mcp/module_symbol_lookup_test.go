package mcp

import (
	"context"
	"encoding/json"
	"slices"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/store"
	"github.com/geoffmilleraz/fathom/pkg/client"
)

// module_symbol_lookup_test.go — wave h2 (O741): a file's module symbol
// (K18, core.ModuleSymbolName) answers an exact `<module>` query and the
// file-scoped lookups, keeps every edge, and stays out of find_symbol's
// case-fold, substring and typo tiers and its did_you_mean. The fixture
// mints module symbols of three kinds, two of them fixture-only
// (JavaScriptModule, RubyModule), so the rule is pinned on the name and lf3's
// dialects inherit it.

// moduleFixture is a store with three files whose module symbols import
// lib/signals.rb's, whose module-level code calls lib/emitter.rb's emit.
func moduleFixture(t *testing.T) *Server {
	t.Helper()
	ctx := context.Background()
	obs := func(file, pkg, name, kind string, line, end int, calls ...string) core.Observation {
		return core.Observation{
			ServiceID: "svc", SourceRef: file, SourceKind: core.SourceKind(kind), EvidenceTier: core.TierVerified,
			Symbol: name, Package: pkg, Calls: calls, Line: line, EndLine: end,
		}
	}
	st := store.NewMemoryStore()
	if err := st.PutObservations(ctx, []core.Observation{
		obs("web/app.js", "app", core.ModuleSymbolName, "JavaScriptModule", 1, 20, "render"),
		obs("web/app.js", "app", "render", "JavaScriptFunction", 3, 5),
		obs("lib/signals.rb", "signals", core.ModuleSymbolName, "RubyModule", 1, 10, "emit"),
		obs("lib/signals.rb", "signals", "connect", "RubyMethod", 2, 4),
		obs("lib/emitter.rb", "emitter", core.ModuleSymbolName, "RubyModule", 1, 8),
		obs("lib/emitter.rb", "emitter", "emit", "RubyMethod", 2, 6),
		obs("lib/loader.py", "loader", core.ModuleSymbolName, "PythonModule", 1, 30),
		obs("lib/loader.py", "loader", "load_module", "PythonFunc", 3, 8),
		obs("lib/loader.py", "loader", "ModuleType", "PythonClass", 10, 30),
	}); err != nil {
		t.Fatal(err)
	}
	mod := func(dir string) string { return core.SymbolKey("svc", dir, core.ModuleSymbolName) }
	if err := st.PutTypedEdges(ctx, "svc", "file-resolved", []core.Edge{
		{FromFile: "web/app.js", FromSymbol: mod("web/app"), ToSymbol: mod("lib/signals"), Kind: core.EdgeFileResolved},
		{FromFile: "lib/loader.py", FromSymbol: mod("lib/loader"), ToSymbol: mod("lib/signals"), Kind: core.EdgeFileResolved},
		{FromFile: "lib/signals.rb", FromSymbol: mod("lib/signals"), ToSymbol: mod("lib/emitter"), Kind: core.EdgeFileResolved},
		{FromFile: "lib/signals.rb", FromSymbol: mod("lib/signals"), ToSymbol: "svc::lib/emitter.emit", Kind: core.EdgeFileResolved},
		{FromFile: "web/app.js", FromSymbol: mod("web/app"), ToSymbol: "svc::web/app.render", Kind: core.EdgeFileResolved},
	}); err != nil {
		t.Fatal(err)
	}
	c := client.New(connector.NewLocalConnector(), st)
	t.Cleanup(func() { c.Close() })
	return NewServer(c)
}

// payloadNames lists the names under key ("symbols" or "did_you_mean").
func payloadNames(payload map[string]any, key string) []string {
	raw, _ := payload[key].([]any)
	var names []string
	for _, r := range raw {
		if m, ok := r.(map[string]any); ok {
			n, _ := m["name"].(string)
			if n == "" {
				n, _ = m["symbol"].(string)
			}
			names = append(names, n)
		}
	}
	return names
}

func findSymbolIn(t *testing.T, srv *Server, name string) map[string]any {
	t.Helper()
	return callTool(t, srv, "find_symbol", map[string]any{"service": "svc", "name": name})
}

// jsonString is v re-encoded without HTML escaping (so `<module>` reads as
// itself), for a containment check over a whole payload.
func jsonString(t *testing.T, v any) string {
	t.Helper()
	var b strings.Builder
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		t.Fatal(err)
	}
	return b.String()
}

// noModuleRows fails when a tier answered a query that did not name
// `<module>` with a module symbol.
func noModuleRows(t *testing.T, query string, payload map[string]any) {
	t.Helper()
	for _, key := range []string{"symbols", "did_you_mean"} {
		if names := payloadNames(payload, key); slices.Contains(names, core.ModuleSymbolName) {
			t.Errorf("find_symbol(%q) via %v: %s = %v, want no %s row", query, payload["matched_via"], key, names, core.ModuleSymbolName)
		}
	}
}

// The case-insensitive tier: a query naming a package ("signals") returns
// the package's members, and the file's module symbol is not one of them
// (on flask 3.1.3 it led every package match: O741); a case-folded spelling
// of the module symbol's own name is not the exact name.
func TestFindSymbolCaseFoldTierSkipsModuleSymbols(t *testing.T) {
	srv := moduleFixture(t)
	p := findSymbolIn(t, srv, "signals")
	if p["matched_via"] != "case_insensitive" {
		t.Fatalf("find_symbol(signals) matched_via = %v, want case_insensitive", p["matched_via"])
	}
	if got := payloadNames(p, "symbols"); !slices.Equal(got, []string{"connect"}) {
		t.Errorf("find_symbol(signals) = %v, want [connect]", got)
	}
	noModuleRows(t, "<MODULE>", findSymbolIn(t, srv, "<MODULE>"))

	all := []core.SymbolNode{
		{Name: core.ModuleSymbolName, Package: "signals", Kind: "RubyModule"},
		{Name: "connect", Package: "signals", Kind: "RubyMethod"},
	}
	if got, total := caseFoldMatches(all, "Signals"); len(got) != 1 || total != 1 || got[0].Name != "connect" {
		t.Errorf("caseFoldMatches(Signals) = %v (total %d), want [connect] (total 1)", got, total)
	}
}

// The substring tier: "odule" names real definitions, and the 83 module
// symbols of a Python repository no longer fill its 20-row cap.
func TestFindSymbolSubstringTierSkipsModuleSymbols(t *testing.T) {
	srv := moduleFixture(t)
	for _, q := range []string{"odule", "modul", "<mod"} {
		noModuleRows(t, q, findSymbolIn(t, srv, q))
	}
	p := findSymbolIn(t, srv, "odule")
	if p["matched_via"] != "substring" {
		t.Fatalf("find_symbol(odule) matched_via = %v, want substring", p["matched_via"])
	}
	if got := payloadNames(p, "symbols"); !slices.Equal(got, []string{"ModuleType", "load_module"}) {
		t.Errorf("find_symbol(odule) = %v, want [ModuleType load_module]", got)
	}
	if p["truncated"] != nil {
		t.Errorf("find_symbol(odule) truncated = %v, want unset (only the two real matches count)", p["truncated"])
	}
}

// The typo tier: a near-miss of `<module>` suggests no module symbol, a
// near-miss of a real name still suggests it.
func TestFindSymbolFuzzyTierSkipsModuleSymbols(t *testing.T) {
	srv := moduleFixture(t)
	for _, q := range []string{"<modle>", "<modules>"} {
		noModuleRows(t, q, findSymbolIn(t, srv, q))
	}
	p := findSymbolIn(t, srv, "load_modle")
	if got := payloadNames(p, "did_you_mean"); !slices.Equal(got, []string{"load_module"}) {
		t.Errorf("find_symbol(load_modle) did_you_mean = %v, want [load_module]", got)
	}
}

// The last-resort did_you_mean (QuerySymbols over the query's name tokens)
// ranks real definitions: "module" is one token of "loadModuleThing", and
// every module symbol's name contains it.
func TestFindSymbolDidYouMeanSkipsModuleSymbols(t *testing.T) {
	srv := moduleFixture(t)
	p := findSymbolIn(t, srv, "loadModuleThing")
	noModuleRows(t, "loadModuleThing", p)
	if got := payloadNames(p, "did_you_mean"); !slices.Equal(got, []string{"load_module", "ModuleType"}) {
		t.Errorf("find_symbol(loadModuleThing) did_you_mean = %v, want [load_module ModuleType]", got)
	}
	for _, concern := range []string{"module", "Module"} {
		q := callTool(t, srv, "query_symbols", map[string]any{"service": "svc", "concern": concern})
		if raw := q["nodes"]; raw != nil && strings.Contains(strings.ToLower(jsonString(t, raw)), `"`+core.ModuleSymbolName+`"`) {
			t.Errorf("query_symbols(%s) ranked a module symbol: %v", concern, raw)
		}
	}
}

// The contract that stays: an exact `<module>` query answers every module
// symbol whatever its kind, outline lists the file's, and the import and
// call edges through one still carry blast_radius, find_callers and
// symbol_context to its importers and callees.
func TestModuleSymbolKeepsExactLookupAndEdges(t *testing.T) {
	srv := moduleFixture(t)
	p := findSymbolIn(t, srv, core.ModuleSymbolName)
	if p["matched_via"] != "exact" {
		t.Fatalf("find_symbol(<module>) matched_via = %v, want exact", p["matched_via"])
	}
	kinds := map[string]bool{}
	for _, r := range p["symbols"].([]any) {
		kinds[r.(map[string]any)["kind"].(string)] = true
	}
	for _, k := range []string{"JavaScriptModule", "RubyModule", "PythonModule"} {
		if !kinds[k] {
			t.Errorf("find_symbol(<module>) kinds = %v, want %s among them", kinds, k)
		}
	}

	o := callTool(t, srv, "outline", map[string]any{"service": "svc", "file": "lib/signals.rb"})
	if got := payloadNames(o, "symbols"); !slices.Contains(got, core.ModuleSymbolName) {
		t.Errorf("outline(lib/signals.rb) = %v, want the file's %s", got, core.ModuleSymbolName)
	}

	signals := core.SymbolKey("svc", "lib/signals", core.ModuleSymbolName)
	importers := []string{core.SymbolKey("svc", "web/app", core.ModuleSymbolName), core.SymbolKey("svc", "lib/loader", core.ModuleSymbolName)}

	fc := jsonString(t, callTool(t, srv, "find_callers", map[string]any{"service": "svc", "symbol": signals}))
	for _, k := range importers {
		if !strings.Contains(fc, k) {
			t.Errorf("find_callers(%s) lacks importer %s: %s", signals, k, fc)
		}
	}

	sc := jsonString(t, callTool(t, srv, "symbol_context", map[string]any{"service": "svc", "symbol": signals, "snippet": false}))
	for _, k := range append(importers, "svc::lib/emitter.emit") {
		if !strings.Contains(sc, k) {
			t.Errorf("symbol_context(%s) lacks %s: %s", signals, k, sc)
		}
	}

	// Changing emit's file reaches the module symbol that calls it, and
	// through it the files that import that module.
	br := jsonString(t, callTool(t, srv, "blast_radius", map[string]any{"service": "svc", "changed_files": []string{"lib/emitter.rb"}}))
	for _, k := range append(importers, signals) {
		if !strings.Contains(br, k) {
			t.Errorf("blast_radius(lib/emitter.rb) lacks %s: %s", k, br)
		}
	}
}
