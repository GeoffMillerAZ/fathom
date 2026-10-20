package langbench

import (
	"testing"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/cuecontext"
)

// TestDialectGlobalNameSchema (LB-F580): the global-name strategy takes a
// scope, "service" or "folder", and defines, keys of the manifest's kinds
// (schema/dialect/testdata/invalid.cue 16p, 16q).
func TestDialectGlobalNameSchema(t *testing.T) {
	cctx := cuecontext.New()
	schema := dialectSchema(t, false)
	base := func(imports string) string {
		return `{id: "g", display: "G", kind: "tooling", engine: "tags", lang_tag: "g",
		match: {extensions: [".g1"]}, grammar: {blob: "graphql", entry_point: "tree_sitter_graphql"}, queries: {tags: "tags.scm", imports: "imports.scm"},
		kinds: {type: {source_kind: "GType", canonical: "type_alias", container: true}, extension: {source_kind: "GExt", canonical: "type_alias", container: true}},
		required_kinds: ["type"], target_level: "L3", queue_id: "g",
		imports: ` + imports + `}`
	}
	for _, good := range []string{
		`{strategy: "global-name"}`,
		`{strategy: "global-name", scope: "service", defines: ["type"]}`,
		`{strategy: "global-name", scope: "folder", defines: ["type", "extension"]}`,
	} {
		v := cctx.CompileString(schema + "\nok: #Dialect & " + base(good))
		if err := v.Validate(cue.Concrete(true)); err != nil {
			t.Errorf("imports %s must validate: %v", good, err)
		}
	}
	for _, bad := range []string{
		`{strategy: "global-names"}`,
		`{strategy: "global-name", scope: "repo"}`,
		`{strategy: "global-name", defines: ["class"]}`,
		`{strategy: "global-name", defines: "type"}`,
	} {
		v := cctx.CompileString(schema + "\nbad: #Dialect & " + base(bad))
		if v.Err() == nil && v.Validate(cue.Concrete(true)) == nil {
			t.Errorf("schema accepted imports %s", bad)
		}
	}
}
