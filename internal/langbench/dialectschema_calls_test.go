package langbench

import (
	"testing"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/cuecontext"
)

// TestDialectCallsOwnerSchema (wave lf5, design §1.8 W1): calls.owner is
// "module" and needs the module symbol it names, calls.split is "whitespace";
// each documented negative (schema/dialect/testdata/invalid.cue 16j..16l) is
// refused.
func TestDialectCallsOwnerSchema(t *testing.T) {
	cctx := cuecontext.New()
	schema := dialectSchema(t, false)
	base := func(extra string) string {
		return `{id: "html", display: "HTML", kind: "language", engine: "tags", lang_tag: "html",
		match: {extensions: [".html"]}, grammar: {blob: "html", entry_point: "tree_sitter_html"}, queries: {tags: "tags.scm"},
		kinds: {label: {source_kind: "HTMLId", canonical: "label"}, document: {source_kind: "HTMLDocument", canonical: "module"}},
		required_kinds: ["label"], target_level: "L3", queue_id: "html"` + extra + `}`
	}
	for name, m := range map[string]string{
		"owner and split": base(`, module_symbol: "document", calls: {owner: "module", split: "whitespace"}`),
		"scope owner":     base(`, module_symbol: "document", calls: {owner: "scope"}`),
		"split alone":     base(`, calls: {split: "whitespace"}`),
		"no calls":        base(``),
	} {
		v := cctx.CompileString(schema + "\nok: #Dialect & " + m)
		if err := v.Validate(cue.Concrete(true)); err != nil {
			t.Errorf("%s must validate: %v", name, err)
		}
	}
	for name, m := range map[string]string{
		"16j owner without module_symbol":  base(`, calls: {owner: "module"}`),
		"16j2 scope without module_symbol": base(`, calls: {owner: "scope"}`),
		"16k owner off the list":           base(`, module_symbol: "document", calls: {owner: "file"}`),
		"16l split off the list":           base(`, calls: {split: "comma"}`),
	} {
		bad := cctx.CompileString(schema + "\nbad: #Dialect & " + m)
		if bad.Err() == nil && bad.Validate(cue.Concrete(true)) == nil {
			t.Errorf("%s: schema accepted %s", name, m)
		}
	}
}
