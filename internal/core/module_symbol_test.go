package core

import "testing"

// TestModuleSymbolPredicatesAreKindGeneric pins K18's lookup rule on the
// name alone (wave h2): a module symbol is one whatever kind its dialect's
// manifest names, so JavaScript, Ruby and Lua (lf3) inherit every query-layer
// exclusion with no edit there. The key-level twin agrees with the node
// predicate on every node, because a centrality row carries only the key.
func TestModuleSymbolPredicatesAreKindGeneric(t *testing.T) {
	nodes := []struct {
		n    SymbolNode
		want bool
	}{
		{SymbolNode{Service: "svc", Package: "app", Name: ModuleSymbolName, Kind: "PythonModule", FilePath: "src/app.py"}, true},
		{SymbolNode{Service: "svc", Package: "app", Name: ModuleSymbolName, Kind: "JavaScriptModule", FilePath: "web/app.js"}, true},
		{SymbolNode{Service: "svc", Package: "signals", Name: ModuleSymbolName, Kind: "RubyModule", FilePath: "lib/signals.rb"}, true},
		{SymbolNode{Service: "svc", Name: ModuleSymbolName, Kind: "LuaModule"}, true},
		{SymbolNode{Service: "svc", Package: "loader", Name: "module", Kind: "PythonVar", FilePath: "lib/loader.py"}, false},
		{SymbolNode{Service: "svc", Package: "loader", Name: "Module", Kind: "PythonClass", FilePath: "lib/loader.py"}, false},
		{SymbolNode{Service: "svc", Package: "loader", Name: "load_module", Kind: "PythonFunc", FilePath: "lib/loader.py"}, false},
		{SymbolNode{Service: "svc", Package: "mod", Name: "Type.module", Kind: "GoMethod", FilePath: "mod/a.go"}, false},
	}
	for _, tc := range nodes {
		if got := tc.n.IsModuleSymbol(); got != tc.want {
			t.Errorf("%s %q IsModuleSymbol = %v, want %v", tc.n.Kind, tc.n.Name, got, tc.want)
		}
		if got := IsModuleSymbolKey(tc.n.Key()); got != tc.want {
			t.Errorf("IsModuleSymbolKey(%q) = %v, want %v (the node predicate's twin)", tc.n.Key(), got, tc.want)
		}
	}
	for _, key := range []string{"", "svc::", "svc::app.module", "svc::app.<module>x", "svc::<module>.run"} {
		if IsModuleSymbolKey(key) {
			t.Errorf("IsModuleSymbolKey(%q) = true, want false", key)
		}
	}
}
