package fusion_test

// LB-Q-ruby (design §13.2 step 13): the ruby dialect's imports.scm parses
// require, require_relative, load and autoload with a literal path and its
// manifest's file-symbol strategy resolves them: require_relative against the
// requiring file's directory, require against the importer's directory and then
// the lib/ source root. A require brings every top-level name of its target into
// scope (Ruby has no per-name import), so a bare call resolves to the one file
// that defines it: a constant reference (User.new, Formatting.money — the call
// text is the constant's name, LB-Q-ruby) reaches the class or module, and a
// top-level method call (helper) reaches its def. A method reached through a
// receiver (Formatting.money's `money`) is not a top-level name and stays
// unresolved, as does anything defined in a file the caller never requires.

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/fusion"
)

func TestResolveFileEdges_RubyRequire(t *testing.T) {
	dir, err := filepath.Abs(filepath.Join("testdata", "rubyfixture"))
	if err != nil {
		t.Fatalf("abs: %v", err)
	}
	rels := []string{
		filepath.Join("lib", "app.rb"),
		filepath.Join("lib", "app", "helper.rb"),
		filepath.Join("lib", "app", "models", "user.rb"),
		filepath.Join("lib", "app", "unused.rb"),
	}
	files, symbols := loadFileResolveFixture(t, dir, "ruby", rels)

	edges, report := fusion.ResolveFileEdges(context.Background(), "ruby", dir, files, symbols)
	if report.Skipped {
		t.Fatalf("skipped: %s", report.SkipReason)
	}
	if report.FilesScanned != 4 || report.ImportsParsed != 3 {
		t.Errorf("FilesScanned = %d ImportsParsed = %d, want 4 and 3", report.FilesScanned, report.ImportsParsed)
	}
	for _, e := range edges {
		if e.Kind != core.EdgeFileResolved {
			t.Errorf("edge %+v: kind %q, want file-resolved", e, e.Kind)
		}
		if core.EdgeConfidenceOf(e) != core.EdgeConfidenceHeuristic {
			t.Errorf("edge %+v: EdgeConfidenceOf = %q, want heuristic", e, core.EdgeConfidenceOf(e))
		}
	}
	keyOf := map[string]string{}
	for _, s := range symbols {
		keyOf[s.FilePath+"|"+s.Name] = s.Key()
	}
	from := keyOf["lib/app.rb|App.run"]
	got := fileResolveEdgeSetAll(edges)
	want := []string{
		from + " -> " + keyOf["lib/app/models/user.rb|User"],
		from + " -> " + keyOf["lib/app/helper.rb|Formatting"],
		from + " -> " + keyOf["lib/app/helper.rb|helper"],
	}
	for _, w := range want {
		if !got[w] {
			t.Errorf("missing edge %s; got %v", w, got)
		}
	}
	if len(got) != len(want) {
		t.Errorf("got %d edges %v, want %d %v", len(got), got, len(want), want)
	}
}
