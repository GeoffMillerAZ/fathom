package extractor

import (
	"reflect"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
)

// hostFixture is an HTML page whose module <script> imports two files, makes
// one top-level call (mount) and defines a function whose own call
// (util.format) stays the function's. The script's raw text starts on line 6,
// so its import lines are 7 and 8 in the page.
const hostFixture = `<!doctype html>
<html>
<body>
<label for="name">Name</label>
<input id="name">
<script type="module">
import { mount } from './app.js';
import * as util from "../lib/util.js";
mount(document.body);
function helper() {
  util.format();
}
</script>
</body>
</html>
`

// hostedHTMLExtractor is the html dialect with injection_host turned on (F2,
// wave lf7): the only manifest change from the embedded html dialect, compiled
// into an extractor of its own (its pool keys are its own).
func hostedHTMLExtractor(t *testing.T) *TSExtractor {
	t.Helper()
	if _, err := tsRegistry(); err != nil {
		t.Fatal(err)
	}
	ds, err := LoadDialects()
	if err != nil {
		t.Fatal(err)
	}
	var d Dialect
	for _, x := range ds {
		if x.ID == "html" {
			d = x
		}
	}
	d.InjectionHost = true
	if err := checkInjectionHost(d); err != nil {
		t.Fatalf("html with injection_host must load: %v", err)
	}
	rules, err := compileDialectRules(d)
	if err != nil {
		t.Fatal(err)
	}
	if !rules.injectionHost {
		t.Fatal("compileDialectRules dropped injection_host")
	}
	v := tsVariant{blob: d.Grammar.Blob, entryPoint: d.Grammar.EntryPoint, rules: rules}
	e := newTSExtractorFromSpec(&tsLangSpec{name: "html-host", variantForExt: map[string]tsVariant{".html": v}, defaultVariant: v})
	t.Cleanup(e.Close)
	return e
}

// TestInjectionHostOwnsImportsAndTopLevelCalls (F2, wave lf7): a host
// manifest's injection_host makes an injected range's import specs the host
// file's, at host lines, resolved under the host's own import strategy, and
// gives the range's top-level calls (no enclosing definition) to the host's
// module symbol. A call a definition of the range encloses stays the
// definition's, and the range mints no module symbol of its own.
func TestInjectionHostOwnsImportsAndTopLevelCalls(t *testing.T) {
	e := hostedHTMLExtractor(t)
	src := []byte(hostFixture)
	nodes, specs, ok, err := e.ExtractWithFileSpecs("svc", "web/index.html", src)
	if err != nil || !ok {
		t.Fatalf("ExtractWithFileSpecs: ok %v, err %v", ok, err)
	}
	m := moduleOf(t, nodes)
	if m.Kind != "HTMLDocument" {
		t.Fatalf("the module symbol is the host's (HTMLDocument), got %+v", m)
	}
	if want := []string{"mount", "name"}; !reflect.DeepEqual(m.Calls, want) {
		t.Errorf("host module calls = %q, want %q (the page's W1 reference and the script's top-level call)", m.Calls, want)
	}
	var helper *core.SymbolNode
	for i, n := range nodes {
		if n.Name == "helper" {
			helper = &nodes[i]
		}
		if n.IsModuleSymbol() && n.Kind != "HTMLDocument" {
			t.Errorf("an injected range minted a module symbol of its own: %+v", n)
		}
	}
	if helper == nil || helper.Line != 10 || !reflect.DeepEqual(helper.Calls, []string{"util.format"}) {
		t.Errorf("helper = %+v, want line 10 owning util.format", helper)
	}
	want := []ImportSpec{
		{LocalName: "mount", ExportedName: "mount", Module: "./app.js", Kind: ImportNamed, Line: 7, Binding: true},
		{LocalName: "util", Module: "../lib/util.js", Kind: ImportWhole, Line: 8},
	}
	if !reflect.DeepEqual(specs.Imports, want) {
		t.Errorf("host imports =\n %+v\nwant\n %+v", specs.Imports, want)
	}
	// The host file's default export is its module symbol (a component file's
	// default export is the component): an import of the file binds it.
	if wantEx := []ExportSpec{{Name: core.ModuleSymbolName, As: "default", Line: 1}}; !reflect.DeepEqual(specs.Exports, wantEx) {
		t.Errorf("host exports = %+v, want %+v", specs.Exports, wantEx)
	}

	// The file resolver's other two readers agree: ExtractFileSpecs (the
	// fallback when the memo has no specs) and ExtractImportSpans (the
	// harness's import_resolution), each at host lines.
	fs, err := e.ExtractFileSpecs("web/index.html", src)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(fs, specs) {
		t.Errorf("ExtractFileSpecs = %+v, ExtractWithFileSpecs = %+v", fs, specs)
	}
	imps, err := e.ExtractImports("web/index.html", src)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(imps, want) {
		t.Errorf("ExtractImports = %+v, want %+v", imps, want)
	}
	spans, err := e.ExtractImportSpans("web/index.html", src)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, s := range spans {
		got = append(got, s.Module+"@"+strconv.Itoa(s.StartLine)+"-"+strconv.Itoa(s.EndLine))
	}
	if wantSpans := []string{"./app.js@7-7", "../lib/util.js@8-8"}; !reflect.DeepEqual(got, wantSpans) {
		t.Errorf("import spans = %q, want %q", got, wantSpans)
	}
}

// TestInjectionHostOffLeavesTheHostUnchanged (F2): with the field off — every
// embedded host today, html — an injected range's imports stay out of the
// host's specs and its top-level calls stay unowned, as before F2.
func TestInjectionHostOffLeavesTheHostUnchanged(t *testing.T) {
	e, err := NewTSExtractor("html")
	if err != nil {
		t.Fatal(err)
	}
	defer e.Close()
	src := []byte(hostFixture)
	nodes, specs, ok, err := e.ExtractWithFileSpecs("svc", "web/index.html", src)
	if err != nil || !ok {
		t.Fatalf("ExtractWithFileSpecs: ok %v, err %v", ok, err)
	}
	if m := moduleOf(t, nodes); !reflect.DeepEqual(m.Calls, []string{"name"}) {
		t.Errorf("html module calls = %q, want only its W1 reference", m.Calls)
	}
	if len(specs.Imports) != 0 || len(specs.Exports) != 0 {
		t.Errorf("html imports %+v, exports %+v; want none (the page has no src or href)", specs.Imports, specs.Exports)
	}
	spans, err := e.ExtractImportSpans("web/index.html", src)
	if err != nil || len(spans) != 0 {
		t.Errorf("html import spans = %+v, %v; want none", spans, err)
	}
}

// TestCheckInjectionHostRules (F2): injection_host needs the injections query
// that finds the ranges, the module symbol that owns their top-level calls and
// the import strategy their specs resolve under.
func TestCheckInjectionHostRules(t *testing.T) {
	base := Dialect{ModuleSymbol: "document", Queries: DialectQueries{Injections: "injections.scm"},
		Imports: &DialectImports{Strategy: "relative-path"}, InjectionHost: true}
	if err := checkInjectionHost(base); err != nil {
		t.Fatalf("a full host must load: %v", err)
	}
	if err := checkInjectionHost(Dialect{}); err != nil {
		t.Fatalf("no host: %v", err)
	}
	for name, tc := range map[string]struct {
		edit func(*Dialect)
		want string
	}{
		"no injections query": {func(d *Dialect) { d.Queries.Injections = "" }, "queries.injections"},
		"no module symbol":    {func(d *Dialect) { d.ModuleSymbol = "" }, "module_symbol"},
		"no import strategy":  {func(d *Dialect) { d.Imports = nil }, "imports"},
	} {
		d := base
		tc.edit(&d)
		err := checkInjectionHost(d)
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s: err = %v, want one naming %q", name, err, tc.want)
		}
	}
}

// TestInjectionHostOnlyWhereTheManifestNamesIt (F2): the field is opt-in; the
// hosts that name it are listed here, so a new one is a reviewed change.
func TestInjectionHostOnlyWhereTheManifestNamesIt(t *testing.T) {
	ds, err := LoadDialects()
	if err != nil {
		t.Fatal(err)
	}
	var named []string
	for _, d := range ds {
		if d.InjectionHost {
			named = append(named, d.ID)
		}
	}
	slices.Sort(named)
	if want := injectionHostsWant; !reflect.DeepEqual(named, want) {
		t.Fatalf("dialects naming injection_host = %q, want %q", named, want)
	}
}

// injectionHostsWant are the embedded dialects that name injection_host.
var injectionHostsWant = []string{"svelte", "vue"} // wave lf7: LB-Q-svelte, LB-Q-vue

// TestFileResolvesInjectionHost (F2): an injection host takes part in the
// file resolver with no imports query of its own, since its injected ranges'
// specs are its imports; any other tags dialect still needs one.
func TestFileResolvesInjectionHost(t *testing.T) {
	host := Dialect{Engine: "tags", Imports: &DialectImports{Strategy: "relative-path"},
		Queries: DialectQueries{Tags: "tags.scm", Injections: "injections.scm"}, ModuleSymbol: "component", InjectionHost: true}
	if !fileResolves(host) {
		t.Fatal("an injection host with an import strategy must take part in the file resolver")
	}
	plain := host
	plain.InjectionHost = false
	if fileResolves(plain) {
		t.Fatal("a tags dialect with no imports query and no injection_host must not")
	}
	plain.Queries.Imports = "imports.scm"
	if !fileResolves(plain) {
		t.Fatal("a tags dialect with an imports query must")
	}
	goPkg := plain
	goPkg.Imports = &DialectImports{Strategy: "go-package"}
	if fileResolves(goPkg) {
		t.Fatal("go-package resolves natively")
	}
}
