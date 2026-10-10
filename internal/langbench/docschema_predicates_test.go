package langbench

import (
	"testing"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/cuecontext"
)

// TestDocSpecSchemaPredicates (K20, wave c2): an entity's where.not_has_key and
// where.matches, its refuse_templates and unique, and a `kv:` value transform
// validate, and each negative schema/dialect/testdata/invalid.cue documents
// (31-35) is refused: not_has_key not a list, an empty matches pattern, a
// matches key that is no selector, refuse_templates or unique not a boolean.
// An RE2 that does not compile is CUE-valid and refused when the spec loads
// (internal/extractor TestDocWhereMatchesRefusedAtLoad).
func TestDocSpecSchemaPredicates(t *testing.T) {
	cctx := cuecontext.New()
	schema := dialectSchema(t, false)
	ok := cctx.CompileString(schema + "\nok: #Doc & " + `{decode: "yaml", detect: {}, entities: [
		{select: "$.steps[*]", where: {is_map: true, has_key: ["run"], not_has_key: ["id"]}, kind: "step", source_kind: "R", name: "fmt:{$path}#{$index}"},
		{select: "$.steps[*]", where: {matches: {".uses": "[a-z]+/[a-z]+@[0-9a-f]{40}", "$": ".*"}}, kind: "resource", source_kind: "P", name: "fmt:pin:{.uses}", unique: true},
		{select: "$.env[*]", kind: "key", source_kind: "I", name: "$ kv:OTEL_SERVICE_NAME cut:;", refuse_templates: true}]}`)
	if err := ok.Validate(cue.Concrete(true)); err != nil {
		t.Fatalf("a spec with the K20 predicates must be valid: %v", err)
	}
	cases := map[string]string{
		"not_has_key not a list":       `{decode: "yaml", detect: {}, entities: [{select: "$.steps[*]", where: {not_has_key: "id"}, kind: "step", source_kind: "X", name: "$index"}]}`,
		"empty matches pattern":        `{decode: "yaml", detect: {}, entities: [{select: "$.steps[*]", where: {matches: {".uses": ""}}, kind: "step", source_kind: "X", name: ".uses"}]}`,
		"matches key not a selector":   `{decode: "yaml", detect: {}, entities: [{select: "$.steps[*]", where: {matches: {uses: "a"}}, kind: "step", source_kind: "X", name: ".uses"}]}`,
		"refuse_templates not boolean": `{decode: "yaml", detect: {}, entities: [{select: "$", kind: "key", source_kind: "X", name: "$file", refuse_templates: "yes"}]}`,
		"unique not boolean":           `{decode: "yaml", detect: {}, entities: [{select: "$", kind: "key", source_kind: "X", name: "$file", unique: 1}]}`,
	}
	for name, spec := range cases {
		v := cctx.CompileString(schema + "\nbad: #Doc & " + spec)
		if err := v.Validate(cue.Concrete(true)); err == nil {
			t.Errorf("%s: #Doc accepted %s", name, spec)
		}
	}
}
