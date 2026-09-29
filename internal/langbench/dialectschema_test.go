package langbench

import (
	"os"
	"strings"
	"testing"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/cuecontext"
)

// dialectSchema returns schema/dialect's definitions (#Dialect and #Doc) and
// positive testdata as one CUE source (package clauses and imports hoisted).
func dialectSchema(t *testing.T, withValid bool) string {
	t.Helper()
	var parts []string
	files := []string{"../../schema/dialect/dialect.cue", "../../schema/dialect/doc.cue"}
	if withValid {
		files = append(files, "../../schema/dialect/testdata/valid.cue")
	}
	for _, p := range files {
		b, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		s := strings.Replace(string(b), "package dialect\n", "", 1)
		s = strings.Replace(s, "import \"struct\"\n", "", 1)
		parts = append(parts, s)
	}
	return "import \"struct\"\n" + strings.Join(parts, "\n")
}

// TestDialectManifestSchema (LB-F29, design §2.2): the positive testdata
// validates concretely and every documented negative is refused.
func TestDialectManifestSchema(t *testing.T) {
	cctx := cuecontext.New()
	v := cctx.CompileString(dialectSchema(t, true))
	if err := v.Err(); err != nil {
		t.Fatalf("compile: %v", err)
	}
	if err := v.Validate(cue.Concrete(true)); err != nil {
		t.Fatalf("valid.cue should validate concretely: %v", err)
	}

	base := `{id: "go", display: "Go", kind: "language", engine: "native", lang_tag: "go",
		match: {extensions: [".go"]},
		kinds: {function: {source_kind: "GoFunc", canonical: "function", callable: true}},
		required_kinds: ["function"], target_level: "L4", queue_id: "go"}`
	schema := dialectSchema(t, false)
	ok := cctx.CompileString(schema + "\nok: #Dialect & " + base)
	if err := ok.Validate(cue.Concrete(true)); err != nil {
		t.Fatalf("the negatives' base must be valid: %v", err)
	}
	cases := map[string]string{
		"claimed level":          `{level_met: "L4"}`,
		"uppercase id":           `{id: "Go", queue_id: "Go"}`,
		"canonical off list":     `{kinds: {function: {canonical: "procedure"}}}`,
		"required not a kind":    `{required_kinds: ["function", "class"]}`,
		"queue_id differs":       `{queue_id: "golang"}`,
		"tags without grammar":   `{engine: "tags"}`,
		"doc with grammar":       `{engine: "doc", grammar: {blob: "go", entry_point: "tree_sitter_go"}}`,
		"strategy off list":      `{imports: {strategy: "stack-graphs"}}`,
		"unknown import param":   `{imports: {strategy: "file-symbol", roots: ["lib"]}}`,
		"import case off list":   `{imports: {strategy: "dotted-module", case: "kebab"}}`,
		"type_wildcard off list": `{imports: {strategy: "namespace-dir", type_wildcard: "fields"}}`,
		"signature too long":     `{signature: {max_chars: 300}}`,
		"extension no dot":       `{match: {extensions: ["go"]}}`,
		"level past L4":          `{target_level: "L5"}`,
		"name_match off list":    `{name_match: "exact"}`,
		"names unknown rule":     `{names: {trim: true}}`,
		"names empty prefix":     `{names: {strip_prefix: [""]}}`,
	}
	for name, patch := range cases {
		src := schema + "\nbad: #Dialect & " + base + " & " + patch
		if name == "required not a kind" || name == "canonical off list" {
			// list/struct replacement, not unification with the base's value
			src = schema + "\nbad: #Dialect & " + strings.Replace(base, `required_kinds: ["function"]`, `required_kinds: ["function", "class"]`, 1)
			if name == "canonical off list" {
				src = schema + "\nbad: #Dialect & " + strings.Replace(base, `canonical: "function"`, `canonical: "procedure"`, 1)
			}
		}
		bad := cctx.CompileString(src)
		if bad.Err() == nil && bad.Validate(cue.Concrete(true)) == nil {
			t.Errorf("%s: schema accepted %s", name, patch)
		}
	}
	empty := cctx.CompileString(schema + "\nbad: #Dialect & " + strings.Replace(base,
		`kinds: {function: {source_kind: "GoFunc", canonical: "function", callable: true}},
		required_kinds: ["function"]`, `kinds: {}, required_kinds: []`, 1))
	if empty.Err() == nil && empty.Validate(cue.Concrete(true)) == nil {
		t.Error("no kinds: schema accepted a manifest without kinds")
	}
}
