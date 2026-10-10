package langbench

import (
	"testing"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/cuecontext"
)

// TestDialectPartialPrefixSchema (LB-F465): imports.partial_prefix is Sass's
// partial prefix and nothing else (schema/dialect/testdata/invalid.cue 16j);
// the scss positive in valid.cue validates.
func TestDialectPartialPrefixSchema(t *testing.T) {
	cctx := cuecontext.New()
	schema := dialectSchema(t, false)
	base := func(prefix string) string {
		return `{id: "s", display: "S", kind: "language", engine: "tags", lang_tag: "s",
		match: {extensions: [".s1"]}, grammar: {blob: "scss", entry_point: "tree_sitter_scss"}, queries: {tags: "tags.scm"},
		kinds: {mixin: {source_kind: "SMixin", canonical: "function", callable: true}},
		required_kinds: ["mixin"], target_level: "L3", queue_id: "s",
		imports: {strategy: "relative-path", relative: true, partial_prefix: ` + prefix + `}}`
	}
	ok := cctx.CompileString(schema + "\nok: #Dialect & " + base(`"_"`))
	if err := ok.Validate(cue.Concrete(true)); err != nil {
		t.Fatalf(`partial_prefix "_" must validate: %v`, err)
	}
	for _, bad := range []string{`"-"`, `""`, `true`} {
		v := cctx.CompileString(schema + "\nbad: #Dialect & " + base(bad))
		if v.Err() == nil && v.Validate(cue.Concrete(true)) == nil {
			t.Errorf("schema accepted partial_prefix %s", bad)
		}
	}
	full := cctx.CompileString(dialectSchema(t, true))
	if err := full.LookupPath(cue.ParsePath("scss")).Validate(cue.Concrete(true)); err != nil {
		t.Errorf("valid.cue's scss manifest: %v", err)
	}
}
