package langbench

import (
	"strings"
	"testing"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/cuecontext"
)

// TestDialectModuleSymbolSchema (LB-F346, K18): a manifest's module_symbol
// names a kind of its own that is canonical "module", neither a container nor
// a callable and outside oracle_kinds, on a tags engine whose package is the
// file's own name; each documented negative (schema/dialect/testdata/
// invalid.cue 16c..16h) is refused.
func TestDialectModuleSymbolSchema(t *testing.T) {
	cctx := cuecontext.New()
	schema := dialectSchema(t, false)
	const kinds = `kinds: {function: {source_kind: "PyFunc", canonical: "function", callable: true}, module: {source_kind: "PyModule", canonical: "module"}}`
	base := func(engine, kinds, extra string) string {
		grammar := ""
		if engine == "tags" {
			grammar = `grammar: {blob: "python", entry_point: "tree_sitter_python"}, queries: {tags: "tags.scm"},`
		}
		return `{id: "py", display: "Py", kind: "language", engine: "` + engine + `", lang_tag: "py",
		match: {extensions: [".py"]}, ` + grammar + kinds + `,
		required_kinds: ["function"], target_level: "L4", queue_id: "py"` + extra + `}`
	}
	ok := cctx.CompileString(schema + "\nok: #Dialect & " + base("tags", kinds, `, oracle_kinds: {ctags: {function: "function"}}, module_symbol: "module"`))
	if err := ok.Validate(cue.Concrete(true)); err != nil {
		t.Fatalf("a tags dialect naming its module kind must validate: %v", err)
	}
	none := cctx.CompileString(schema + "\nok: #Dialect & " + base("tags", kinds, ""))
	if err := none.Validate(cue.Concrete(true)); err != nil {
		t.Fatalf("module_symbol is optional: %v", err)
	}
	cases := map[string]string{
		"names no kind":         base("tags", strings.Replace(kinds, `, module: {source_kind: "PyModule", canonical: "module"}`, "", 1), `, module_symbol: "module"`),
		"canonical not module":  base("tags", kinds, `, module_symbol: "function"`),
		"native engine":         base("native", kinds, `, module_symbol: "module"`),
		"a container":           base("tags", strings.Replace(kinds, `canonical: "module"}`, `canonical: "module", container: true}`, 1), `, module_symbol: "module"`),
		"a callable":            base("tags", strings.Replace(kinds, `canonical: "module"}`, `canonical: "module", callable: true}`, 1), `, module_symbol: "module"`),
		"inside oracle_kinds":   base("tags", kinds, `, oracle_kinds: {ctags: {module: "module"}}, module_symbol: "module"`),
		"not a kind key string": base("tags", kinds, `, module_symbol: true`),
		"package by capture":    base("tags", kinds, `, package: {strategy: "capture"}, module_symbol: "module"`),
	}
	for name, m := range cases {
		bad := cctx.CompileString(schema + "\nbad: #Dialect & " + m)
		if bad.Err() == nil && bad.Validate(cue.Concrete(true)) == nil {
			t.Errorf("%s: schema accepted %s", name, m)
		}
	}
}
