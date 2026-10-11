package langbench

import (
	"testing"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/cuecontext"
)

// TestDialectInjectionHostSchema (F2, wave lf7): injection_host is a boolean,
// and a host names the injections query that finds its ranges, the module
// symbol that owns their top-level calls and the import strategy their specs
// resolve under; each documented negative (schema/dialect/testdata/invalid.cue
// 16w..16z) is refused.
func TestDialectInjectionHostSchema(t *testing.T) {
	cctx := cuecontext.New()
	schema := dialectSchema(t, false)
	base := func(extra string) string {
		return `{id: "html", display: "HTML", kind: "language", engine: "tags", lang_tag: "html",
		match: {extensions: [".html"]}, grammar: {blob: "html", entry_point: "tree_sitter_html"},
		kinds: {label: {source_kind: "HTMLId", canonical: "label"}, document: {source_kind: "HTMLDocument", canonical: "module"}},
		required_kinds: ["label"], target_level: "L3", queue_id: "html"` + extra + `}`
	}
	const (
		injections = `, queries: {tags: "tags.scm", injections: "injections.scm"}`
		module     = `, module_symbol: "document"`
		imports    = `, imports: {strategy: "relative-path"}`
	)
	for name, m := range map[string]string{
		"a full host":       base(injections + module + imports + `, injection_host: true`),
		"host off":          base(injections + `, injection_host: false`),
		"no injection_host": base(injections),
	} {
		v := cctx.CompileString(schema + "\nok: #Dialect & " + m)
		if err := v.Validate(cue.Concrete(true)); err != nil {
			t.Errorf("%s must validate: %v", name, err)
		}
	}
	for name, m := range map[string]string{
		"16w no injections query": base(`, queries: {tags: "tags.scm"}` + module + imports + `, injection_host: true`),
		"16x no module_symbol":    base(injections + imports + `, injection_host: true`),
		"16y no imports":          base(injections + module + `, injection_host: true`),
		"16z not a boolean":       base(injections + module + imports + `, injection_host: "yes"`),
	} {
		bad := cctx.CompileString(schema + "\nbad: #Dialect & " + m)
		if bad.Err() == nil && bad.Validate(cue.Concrete(true)) == nil {
			t.Errorf("%s: schema accepted %s", name, m)
		}
	}
}
