package langbench

import (
	"testing"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/cuecontext"
)

// TestDocSpecSchema (LB-F8, extended by LB-Q-cloudformation, design §4): a
// doc spec with the cloudformation yaml_tags scheme, a content sniff, the
// `..key` and `[N]` selectors and a declared_only reference validates, and
// each negative schema/dialect/testdata/invalid.cue documents for #Doc is
// refused.
func TestDocSpecSchema(t *testing.T) {
	cctx := cuecontext.New()
	schema := dialectSchema(t, false)
	base := `{decode: "yaml", detect: {},
		entities: [{select: "$", kind: "package", source_kind: "X", name: "$file"}]}`
	ok := cctx.CompileString(schema + "\nok: #Doc & " + base + ` & {
		yaml_tags: "cloudformation"
		detect: {require_keys: ["Resources"], content_sniff: "AWSTemplateFormatVersion"}
		references: [{from: "package", select: "$..Fn::GetAtt[0]", value: "$", to_kind: "package", edge: "ref", declared_only: true}]
	}`)
	if err := ok.Validate(cue.Concrete(true)); err != nil {
		t.Fatalf("the extended spec must be valid: %v", err)
	}
	cases := map[string]string{
		"yaml_tags off list":   base + ` & {yaml_tags: "k8s"}`,
		"yaml_tags on json":    `{decode: "json", yaml_tags: "cloudformation", detect: {}, entities: [{select: "$", kind: "package", source_kind: "X", name: "$file"}]}`,
		"declared_only string": base + ` & {references: [{from: "package", select: "$..Ref", value: "$", to_kind: "package", edge: "ref", declared_only: "yes"}]}`,
		"empty content_sniff":  base + ` & {detect: {content_sniff: ""}}`,
		"selector not rooted":  `{decode: "yaml", detect: {}, entities: [{select: "Resources..Ref", kind: "package", source_kind: "X", name: "$file"}]}`,
	}
	for name, spec := range cases {
		v := cctx.CompileString(schema + "\nbad: #Doc & " + spec)
		if err := v.Validate(cue.Concrete(true)); err == nil {
			t.Errorf("%s: #Doc accepted %s", name, spec)
		}
	}
}
