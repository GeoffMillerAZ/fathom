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
	"path/filepath"
	"testing"
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
