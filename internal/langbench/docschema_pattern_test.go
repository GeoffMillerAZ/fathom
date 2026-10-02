package langbench

import (
	"testing"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/cuecontext"
)

// TestDocSpecSchemaPattern (LB-F145, wave iac): a reference's and a
// cross-file reference's `pattern` validates, and each pattern negative
// schema/dialect/testdata/invalid.cue documents (22-24) is refused: an empty
// pattern, one with no group, one on a label-selector.
func TestDocSpecSchemaPattern(t *testing.T) {
	cctx := cuecontext.New()
	schema := dialectSchema(t, false)
	base := `{decode: "json", detect: {},
		entities: [{select: "$", kind: "package", source_kind: "X", name: "$file"}]}`
	ok := cctx.CompileString(schema + "\nok: #Doc & " + base + ` & {
		references: [{from: "package", select: "$", value: "fmt:[{$}]", pattern: "parameters\\('([^']+)'\\)", to_kind: "package", edge: "ref", declared_only: true}]
		cross_file: [{select: "$..uri", value: "$", pattern: "'([^']+\\.json)'", resolve: "repo-path", edge: "linked"}]
	}`)
	if err := ok.Validate(cue.Concrete(true)); err != nil {
		t.Fatalf("a spec with patterns must be valid: %v", err)
	}
	cases := map[string]string{
		"empty pattern":       base + ` & {references: [{from: "package", select: "$", value: "$", to_kind: "package", edge: "ref", pattern: ""}]}`,
		"pattern no group":    base + ` & {references: [{from: "package", select: "$", value: "$", to_kind: "package", edge: "ref", pattern: "parameters"}]}`,
		"pattern not string":  base + ` & {cross_file: [{select: "$", value: "$", resolve: "repo-path", edge: "e", pattern: 3}]}`,
		"pattern on selector": base + ` & {cross_file: [{select: "$.selector", value: "$", resolve: "label-selector", edge: "selects", pattern: "(a)"}]}`,
	}
	for name, spec := range cases {
		v := cctx.CompileString(schema + "\nbad: #Doc & " + spec)
		if err := v.Validate(cue.Concrete(true)); err == nil {
			t.Errorf("%s: #Doc accepted %s", name, spec)
		}
	}
}
