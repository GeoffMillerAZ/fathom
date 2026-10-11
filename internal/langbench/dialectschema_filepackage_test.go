package langbench

import (
	"testing"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/cuecontext"
)

// TestDialectPackageFileSchema (LB-F519, wave lf7 F1): package strategy
// "file" (the file's base name with its extension) validates, alone and with
// module_symbol, beside "dir"; each documented negative (schema/dialect/
// testdata/invalid.cue 16m, 16n) is refused: a strategy off the list, and
// module_symbol on a package that is not the file's own (capture, none).
func TestDialectPackageFileSchema(t *testing.T) {
	cctx := cuecontext.New()
	schema := dialectSchema(t, false)
	const kinds = `kinds: {function: {source_kind: "PsFunc", canonical: "function", callable: true}, module: {source_kind: "PsModule", canonical: "module"}}`
	base := func(extra string) string {
		return `{id: "ps", display: "Ps", kind: "language", engine: "tags", lang_tag: "ps",
		match: {extensions: [".psm1", ".psd1"]}, grammar: {blob: "powershell", entry_point: "tree_sitter_powershell"}, queries: {tags: "tags.scm"}, ` + kinds + `,
		required_kinds: ["function"], target_level: "L3", queue_id: "ps"` + extra + `}`
	}
	for name, m := range map[string]string{
		"file":                    base(`, package: {strategy: "file"}`),
		"file with module_symbol": base(`, package: {strategy: "file"}, module_symbol: "module"`),
		"dir with module_symbol":  base(`, package: {strategy: "dir"}, module_symbol: "module"`),
		"default with module":     base(`, module_symbol: "module"`),
	} {
		v := cctx.CompileString(schema + "\nok: #Dialect & " + m)
		if err := v.Validate(cue.Concrete(true)); err != nil {
			t.Errorf("%s must validate: %v", name, err)
		}
	}
	for name, m := range map[string]string{
		"strategy off the list":        base(`, package: {strategy: "stem"}`),
		"module_symbol on capture":     base(`, package: {strategy: "capture"}, module_symbol: "module"`),
		"module_symbol on no package":  base(`, package: {strategy: "none"}, module_symbol: "module"`),
		"file spelt with an extension": base(`, package: {strategy: "file.ext"}`),
	} {
		bad := cctx.CompileString(schema + "\nbad: #Dialect & " + m)
		if bad.Err() == nil && bad.Validate(cue.Concrete(true)) == nil {
			t.Errorf("%s: schema accepted %s", name, m)
		}
	}
}
