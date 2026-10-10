package langbench

import (
	"testing"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/cuecontext"
)

// TestDialectVariantQueriesSchema (O732, LB-F385): a variant may name a tags
// query of its own (TypeScript's tsx: tags.tsx.scm), a .scm file; a variant
// query that is not a .scm file, or names a query kind other than tags, is
// refused (schema/dialect/testdata/invalid.cue 16i).
func TestDialectVariantQueriesSchema(t *testing.T) {
	cctx := cuecontext.New()
	schema := dialectSchema(t, false)
	base := func(variant string) string {
		return `{id: "ts", display: "TS", kind: "language", engine: "tags", lang_tag: "ts",
		match: {extensions: [".ts"]}, grammar: {blob: "typescript", entry_point: "tree_sitter_typescript"},
		variants: [{id: "tsx", entry_point: "tree_sitter_tsx", extensions: [".tsx"]` + variant + `}],
		queries: {tags: "tags.scm"}, kinds: {function: {source_kind: "TSF", canonical: "function", callable: true}},
		required_kinds: ["function"], target_level: "L4", queue_id: "ts"}`
	}
	for _, v := range []string{"", `, queries: {tags: "tags.tsx.scm"}`} {
		ok := cctx.CompileString(schema + "\nok: #Dialect & " + base(v))
		if err := ok.Validate(cue.Concrete(true)); err != nil {
			t.Fatalf("variant %q must validate: %v", v, err)
		}
	}
	for name, v := range map[string]string{
		"not a .scm file":   `, queries: {tags: "tags.tsx.json"}`,
		"an imports query":  `, queries: {imports: "imports.scm"}`,
		"queries no struct": `, queries: "tags.tsx.scm"`,
	} {
		bad := cctx.CompileString(schema + "\nbad: #Dialect & " + base(v))
		if bad.Err() == nil && bad.Validate(cue.Concrete(true)) == nil {
			t.Errorf("%s: schema accepted %s", name, v)
		}
	}
}
