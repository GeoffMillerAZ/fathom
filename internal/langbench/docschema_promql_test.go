package langbench

import (
	"testing"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/cuecontext"
)

// TestDocSpecSchemaPromQL (LB-F317, wave c1): a reference's `promql` and a
// route-host cross-file reference's `promql` (with or without its own label
// set) validate, and each negative schema/dialect/testdata/invalid.cue
// documents (26-30) is refused: promql off route-host, with a pattern, an
// empty label, labels on an intra-file reference, promql with a scan.
func TestDocSpecSchemaPromQL(t *testing.T) {
	cctx := cuecontext.New()
	schema := dialectSchema(t, false)
	base := `{decode: "yaml", detect: {},
		entities: [{select: "$", kind: "package", source_kind: "X", name: "$file"}]}`
	ok := cctx.CompileString(schema + "\nok: #Doc & " + base + ` & {
		references: [{from: "package", select: "$.expr", value: "$", promql: {}, to_kind: "package", edge: "uses_record", declared_only: true}]
		cross_file: [{select: "$.expr", value: "$", promql: {}, resolve: "route-host", edge: "route_ref", scope: "$.metadata.namespace"},
			{select: "$.expr", value: "$", promql: {labels: ["job", "namespace"]}, resolve: "route-host", edge: "route_ref"}]
	}`)
	if err := ok.Validate(cue.Concrete(true)); err != nil {
		t.Fatalf("a spec with promql references must be valid: %v", err)
	}
	cases := map[string]string{
		"promql off route-host":      base + ` & {cross_file: [{select: "$", value: "$", resolve: "repo-path", edge: "e", promql: {}}]}`,
		"promql with a pattern":      base + ` & {cross_file: [{select: "$", value: "$", resolve: "route-host", edge: "e", promql: {}, pattern: "(a)"}]}`,
		"empty promql label":         base + ` & {cross_file: [{select: "$", value: "$", resolve: "route-host", edge: "e", promql: {labels: [""]}}]}`,
		"promql labels on reference": base + ` & {references: [{from: "package", select: "$", value: "$", to_kind: "package", edge: "e", promql: {labels: ["job"]}}]}`,
		"promql with a scan":         base + ` & {references: [{from: "package", select: "$", value: "$", to_kind: "package", edge: "e", promql: {}, scan: "(a)"}]}`,
	}
	for name, spec := range cases {
		v := cctx.CompileString(schema + "\nbad: #Doc & " + spec)
		if err := v.Validate(cue.Concrete(true)); err == nil {
			t.Errorf("%s: #Doc accepted %s", name, spec)
		}
	}
}
