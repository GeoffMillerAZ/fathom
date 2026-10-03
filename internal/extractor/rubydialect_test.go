package extractor

// rubydialect_test.go — LB-Q-ruby. The ruby dialect left TestTagsDialectParity's
// legacy projection (see migratedTagsDialects) because it emits more than the
// pre-LB-Q-ruby query did (constants, attr_* properties, aliases, define_method
// methods, Class.new/Module.new bindings, a constant reference as a call) and its
// goldens are now the full, hand-written ones TestDialectGoldens checks. This
// test keeps the guarantee that mattered: every symbol the pre-LB-Q-ruby goldens
// listed (name, kind, first line) is still emitted. The table is those goldens'
// symbol lists as they were before LB-Q-ruby (written by the pre-manifest engine,
// PROVENANCE.md); a symbol may be added to the fixtures' goldens, never removed
// from here.

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"testing/fstest"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/cuecontext"
)

type legacyRubySymbol struct {
	name, kind string
	line       int
}

var legacyRubySymbols = map[string][]legacyRubySymbol{
	"shapes.rb": {
		{"Outer", "RubyModule", 4},
		{"Outer::Inner", "RubyModule", 7},
		{"Outer::Inner::Deep", "RubyClass", 8},
		{"Outer::Inner::Deep.call", "RubyMethod", 9},
		{"Outer::Service", "RubyClass", 16},
		{"Outer::Service.initialize", "RubyMethod", 20},
		{"Outer::Service.build", "RubyMethod", 25},
		{"Outer::Service.each", "RubyMethod", 29},
		{"Outer::Service.singleton_helper", "RubyMethod", 34},
		{"Outer::Service.secret?", "RubyMethod", 41},
		{"Outer::Service.value=", "RubyMethod", 45},
		{"Outer::Reopened", "RubyClass", 51},
		{"Outer::Reopened.method_missing", "RubyMethod", 52},
		{"top_level", "RubyMethod", 57},
	},
	"broken.rb": {
		{"good", "RubyMethod", 1},
		{"Survivor", "RubyClass", 8},
		{"Survivor.ok", "RubyMethod", 9},
	},
}

func TestRubyDialectKeepsLegacySymbols(t *testing.T) {
	e, err := NewTSExtractor("ruby")
	if err != nil {
		t.Fatalf("NewTSExtractor(ruby): %v", err)
	}
	defer e.Close()
	for fixture, want := range legacyRubySymbols {
		t.Run(fixture, func(t *testing.T) {
			got := extractGolden(t, e, filepath.Join("dialects", "ruby", "testdata", fixture))
			have := map[legacyRubySymbol]bool{}
			for _, s := range got.Symbols {
				have[legacyRubySymbol{s.Name, s.Kind, s.Line}] = true
			}
			for _, w := range want {
				if !have[w] {
					t.Errorf("%s: legacy symbol %s %s at line %d is no longer emitted", fixture, w.kind, w.name, w.line)
				}
			}
		})
	}
}

// TestRubyLocalsRule (LB-F254): a bare identifier (`helper`, no receiver and no
// arguments) is a call unless a local of that name was defined at or before it
// in a scope it can see (queries.locals, tsextract_locals.go): an assignment's
// left side, a parameter of any kind, a rescue variable, a for target or a
// pattern-matching binding, in the same def, class, module or top level, or an
// enclosing one through blocks and lambdas. A name another pattern already
// takes (a method's or setter's own name, a call's method name) is never also
// read as one. Reached through NewTSExtractor("ruby"), which ingest dispatches
// for .rb.
func TestRubyLocalsRule(t *testing.T) {
	e, err := NewTSExtractor("ruby")
	if err != nil {
		t.Fatalf("NewTSExtractor(ruby): %v", err)
	}
	defer e.Close()
	got := extractGolden(t, e, filepath.Join("dialects", "ruby", "testdata", "locals.rb"))
	calls := map[string]map[string]bool{}
	for _, s := range got.Symbols {
		calls[s.Name] = map[string]bool{}
		for _, c := range s.Calls {
			calls[s.Name][c] = true
		}
	}
	cases := []struct {
		sym       string
		want, not []string
	}{
		{"Widget.tag", []string{"suffix"}, []string{"prefix"}},
		{"Widget.initialize", []string{"helper", "finish", "ready?", "go"}, []string{"name", "size", "rest", "key", "opt", "extra", "blk", "total"}},
		{"Widget.render", []string{"name", "upcase", "format_label", "each", "show", "counter", "risky", "StandardError", "message", "rows", "draw", "shape", "fit", "factor", "call"},
			[]string{"label", "item", "err", "row", "first", "others", "width", "tall", "x", "scaled"}},
		{"Widget.name=", nil, []string{"name", "value"}},
		{"Widget.other", []string{"label"}, nil},
		{"uses_top", []string{"top"}, nil},
	}
	for _, c := range cases {
		have, ok := calls[c.sym]
		if !ok {
			t.Errorf("%s: no such symbol", c.sym)
			continue
		}
		for _, w := range c.want {
			if !have[w] {
				t.Errorf("%s: call %q missing (calls %v)", c.sym, w, sortedKeys(have))
			}
		}
		for _, n := range c.not {
			if have[n] {
				t.Errorf("%s: %q is a local or a name, not a call (calls %v)", c.sym, n, sortedKeys(have))
			}
		}
	}
}

func sortedKeys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// TestRubyLocalsManifest (LB-F254): queries.locals is a manifest field the CUE
// schema accepts only as a .scm file (schema/dialect/testdata, negative 16b),
// and the engine refuses a manifest whose tags query and locals query do not
// go together: a pattern set is-not "local" with no locals query to decide it,
// or a locals query no pattern uses.
func TestRubyLocalsManifest(t *testing.T) {
	var parts []string
	for _, p := range []string{"../../schema/dialect/dialect.cue", "../../schema/dialect/doc.cue", "../../schema/dialect/testdata/valid.cue"} {
		b, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		s := strings.Replace(string(b), "package dialect\n", "", 1)
		parts = append(parts, strings.Replace(s, "import \"struct\"\n", "", 1))
	}
	schema := "import \"struct\"\n" + strings.Join(parts, "\n")
	cctx := cuecontext.New()
	if err := cctx.CompileString(schema).Validate(cue.Concrete(true)); err != nil {
		t.Fatalf("valid.cue (rubyLocals) should validate: %v", err)
	}
	base := `{id: "ruby", display: "Ruby", kind: "language", engine: "tags", lang_tag: "ruby",
		match: {extensions: [".rb"]}, grammar: {blob: "ruby", entry_point: "tree_sitter_ruby"},
		kinds: {method: {source_kind: "RubyMethod", canonical: "method", callable: true}},
		required_kinds: ["method"], target_level: "L3", queue_id: "ruby"}`
	if err := cctx.CompileString(schema + "\nok: #Dialect & " + base + ` & {queries: {tags: "tags.scm", locals: "locals.scm"}}`).Validate(cue.Concrete(true)); err != nil {
		t.Fatalf("queries.locals .scm refused: %v", err)
	}
	bad := cctx.CompileString(schema + "\nbad: #Dialect & " + base + ` & {queries: {tags: "tags.scm", locals: "locals.json"}}`)
	if bad.Err() == nil && bad.Validate(cue.Concrete(true)) == nil {
		t.Error("schema accepted queries.locals that is not a .scm file")
	}

	const marked = "((identifier) @reference.call (#set! is-not \"local\"))\n"
	fsys := fstest.MapFS{"dialects/x/locals.scm": {Data: []byte("(method) @local.scope\n")}}
	for name, c := range map[string]struct {
		query, locals string
		ok            bool
	}{
		"marked, no locals query": {marked, "", false},
		"locals query, unmarked":  {"(call) @reference.call\n", "locals.scm", false},
		"marked with locals":      {marked, "locals.scm", true},
		"neither":                 {"(call) @reference.call\n", "", true},
	} {
		r := &tsDialectRules{id: "x", query: c.query}
		err := r.loadLocalsQuery(fsys, Dialect{ID: "x", Queries: DialectQueries{Locals: c.locals}})
		if (err == nil) != c.ok {
			t.Errorf("%s: err %v, want ok=%v", name, err, c.ok)
		}
		if c.ok && c.locals != "" && !strings.Contains(r.query, "@local.scope") {
			t.Errorf("%s: the locals query is not appended to the tags query", name)
		}
	}
}
