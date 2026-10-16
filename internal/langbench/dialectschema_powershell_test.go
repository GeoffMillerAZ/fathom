package langbench

import (
	"testing"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/cuecontext"
)

// TestDialectDirVarsSchema (LB-Q-powershell, wave lf6): imports.dir_vars
// names variables, each `$` and an identifier (schema/dialect/testdata/
// invalid.cue 16o); the powershell manifest's `["$PSScriptRoot"]` validates.
func TestDialectDirVarsSchema(t *testing.T) {
	cctx := cuecontext.New()
	schema := dialectSchema(t, false)
	base := func(vars string) string {
		return `{id: "p", display: "P", kind: "language", engine: "tags", lang_tag: "p",
		match: {extensions: [".p1"]}, grammar: {blob: "powershell", entry_point: "tree_sitter_powershell"}, queries: {tags: "tags.scm"},
		kinds: {function: {source_kind: "PFunction", canonical: "function", callable: true}},
		required_kinds: ["function"], target_level: "L3", queue_id: "p",
		imports: {strategy: "file-symbol", relative: true, separator: "\\", dir_vars: ` + vars + `}}`
	}
	ok := cctx.CompileString(schema + "\nok: #Dialect & " + base(`["$PSScriptRoot"]`))
	if err := ok.Validate(cue.Concrete(true)); err != nil {
		t.Fatalf(`dir_vars ["$PSScriptRoot"] must validate: %v`, err)
	}
	for _, bad := range []string{`["PSScriptRoot"]`, `[""]`, `["$PS ScriptRoot"]`, `"$PSScriptRoot"`} {
		v := cctx.CompileString(schema + "\nbad: #Dialect & " + base(bad))
		if v.Err() == nil && v.Validate(cue.Concrete(true)) == nil {
			t.Errorf("schema accepted dir_vars %s", bad)
		}
	}
}
