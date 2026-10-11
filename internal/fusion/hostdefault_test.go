package fusion

import (
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/extractor"
)

// TestHostDefaultExportBindsModuleSymbol (F2, wave lf7): an injection host's
// default export names its module symbol (extractor.hostDefaultExport), which
// no name of the file's scope reaches (K18), so the export index binds it
// through the resolver's module symbols: an import of a component file used
// bare binds the component, never the one definition its <script> holds
// alone. Without a module lookup, or for a file with no module symbol, the
// export binds nothing, as before.
func TestHostDefaultExportBindsModuleSymbol(t *testing.T) {
	const file = "src/Button.vue"
	specs := map[string]extractor.FileSpecs{file: {Exports: []extractor.ExportSpec{{Name: core.ModuleSymbolName, As: "default", Line: 1}}}}
	top := map[string]map[string]string{file: {"props": "web::src/Button.vue.props"}}
	r := &importResolver{strategy: "relative-path"}
	x := newExportIndex(r, specs, top)
	if key, ok := x.lookup(file, "default"); ok {
		t.Fatalf("with no module lookup the default export bound %q", key)
	}
	x = newExportIndex(r, specs, top)
	x.moduleOf = func(f string) (string, bool) {
		if f == file {
			return "web::src/Button.vue.<module>", true
		}
		return "", false
	}
	if key, ok := x.lookup(file, "default"); !ok || key != "web::src/Button.vue.<module>" {
		t.Fatalf("lookup(default) = %q, %v; want the component's module symbol", key, ok)
	}
	if key, ok := x.lookup("src/Other.vue", "default"); ok {
		t.Fatalf("a file with no default export bound %q", key)
	}
}
