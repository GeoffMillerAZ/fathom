package extractor

import (
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
)

// TestTSInjections (LB-F6, design §3.6): an injections query's
// @injection.content with a static `#set! injection.language` re-runs that
// dialect over the byte range — lines offset into the host file, one level
// deep, nothing qualified across the boundary; an unknown language is
// skipped.
func TestTSInjections(t *testing.T) {
	hostQuery := `
(class_definition name: (identifier) @name) @definition.class
(function_definition name: (identifier) @name body: (_) @body) @definition.function
`
	injections := `
((assignment left: (identifier) @_var right: (string (string_content) @injection.content))
  (#eq? @_var "SCRIPT")
  (#set! injection.language "javascript"))
((assignment left: (identifier) @_var right: (string (string_content) @injection.content))
  (#eq? @_var "OTHER")
  (#set! injection.language "no-such-dialect"))
`
	e := testTSExtractor("python", "tree_sitter_python", hostQuery, map[string]tsDefKind{
		"class":    {Kind: core.KindPyClass, Container: true},
		"function": {Kind: core.KindPyFunc, Callable: true},
	}, func(r *tsDialectRules) { r.injectionsQuery = injections })
	defer e.Close()

	src := `class Host:
    SCRIPT = """
function injected(a) {
  return helper(a);
}
class Widget { render() { draw(); } }
"""
    OTHER = """function ignored() {}"""

    def method(self):
        pass
`
	nodes, err := e.Extract("svc", "app/m.py", []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	type want struct {
		kind       core.SourceKind
		line, end  int
		calls, pkg string
	}
	wants := map[string]want{
		"Host":          {core.KindPyClass, 1, 11, "", "m"},
		"Host.method":   {core.KindPyFunc, 10, 11, "", "m"},
		"injected":      {core.KindJSFunc, 3, 5, "helper", "m"},
		"Widget":        {core.KindJSClass, 6, 6, "", "m"},
		"Widget.render": {core.KindJSMethod, 6, 6, "draw", "m"},
	}
	got := map[string]core.SymbolNode{}
	for _, n := range nodes {
		got[n.Name] = n
	}
	if len(got) != len(wants) {
		t.Errorf("symbols = %v, want exactly %d", symStrings(nodes), len(wants))
	}
	for name, w := range wants {
		n, ok := got[name]
		if !ok {
			t.Errorf("%s missing (got %v)", name, symStrings(nodes))
			continue
		}
		if n.Kind != w.kind || n.Line != w.line || n.EndLine != w.end || strings.Join(n.Calls, ",") != w.calls || n.Package != w.pkg || n.FilePath != "app/m.py" {
			t.Errorf("%s = %+v, want %+v", name, n, w)
		}
	}
	if _, ok := got["Host.injected"]; ok {
		t.Error("an injected definition was qualified across the boundary")
	}
	for i := 1; i < len(nodes); i++ {
		if nodes[i-1].Line > nodes[i].Line {
			t.Errorf("output not sorted by line: %v", symStrings(nodes))
		}
	}
}
