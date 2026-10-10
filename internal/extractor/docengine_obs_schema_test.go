package extractor

// docengine_obs_schema_test.go — schema/dialect's #Doc accepts wave c3's
// additions (LB-F361) and refuses each negative
// schema/dialect/testdata/invalid.cue documents for them (36-43). It sits
// beside the engine whose load-time checks mirror it (TestDocObsRefusedAtLoad).

import (
	"os"
	"strings"
	"testing"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/cuecontext"
)

func obsDocSchema(t *testing.T) string {
	t.Helper()
	var parts []string
	for _, p := range []string{"../../schema/dialect/dialect.cue", "../../schema/dialect/doc.cue"} {
		b, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		s := strings.Replace(string(b), "package dialect\n", "", 1)
		parts = append(parts, strings.Replace(s, "import \"struct\"\n", "", 1))
	}
	return "import \"struct\"\n" + strings.Join(parts, "\n")
}

// TestDocObsSchema (LB-F361): the alertmanager and grafana-dashboard doc
// specs validate, and each documented negative is refused.
func TestDocObsSchema(t *testing.T) {
	cctx := cuecontext.New()
	schema := obsDocSchema(t)
	for _, id := range []string{"alertmanager", "grafana-dashboard", "prometheus"} {
		b, err := os.ReadFile("dialects/" + id + "/doc.json")
		if err != nil {
			t.Fatal(err)
		}
		v := cctx.CompileString(schema + "\nok: #Doc & " + string(b))
		if err := v.Validate(cue.Concrete(true)); err != nil {
			t.Errorf("%s/doc.json is not a valid #Doc: %v", id, err)
		}
	}
	base := `{decode: "yaml", detect: {}, entities: [{select: "$", kind: "package", source_kind: "X", name: "$file"}]}`
	for name, spec := range map[string]string{
		"36 entity-name without to_kind":  base + ` & {cross_file: [{select: "$.a", value: "$", resolve: "entity-name", edge: "e"}]}`,
		"37 to_kind on repo-path":         base + ` & {cross_file: [{select: "$.a", value: "$", resolve: "repo-path", to_kind: "X", edge: "e"}]}`,
		"38 promql labels on entity-name": base + ` & {cross_file: [{select: "$.a", value: "$", resolve: "entity-name", to_kind: "X", edge: "e", promql: {labels: ["job"]}}]}`,
		"39 matchers with neither":        base + ` & {matchers: [{from: "package", select: "$.m", edge: "e"}]}`,
		"40 matchers with both":           base + ` & {matchers: [{from: "package", select: "$.m", syntax: "alertmanager", op: "=", edge: "e"}]}`,
		"41 matcher op off the list":      base + ` & {matchers: [{from: "package", select: "$.m", op: "==", edge: "e"}]}`,
		"42 fact without a key":           base + ` & {facts: [{from: "package", select: "$.m"}]}`,
		"43 empty signature field":        `{decode: "yaml", detect: {}, entities: [{select: "$", kind: "package", source_kind: "X", name: "$file", signature_fields: [""]}]}`,
	} {
		v := cctx.CompileString(schema + "\nbad: #Doc & " + spec)
		if err := v.Validate(cue.Concrete(true)); err == nil {
			t.Errorf("%s: #Doc accepted %s", name, spec)
		}
	}
}
