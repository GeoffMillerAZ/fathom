package extractor

// groovydialect_test.go — LB-Q-groovy and LB-F93. Groovy is refused (design §14.2,
// LB-Q-groovy-rest): no grammar parses real Groovy under the 5% line, so no dialect claims
// .groovy or .gvy and the files stay disclosed as unsupported. The grammar is still pinned,
// because the Gradle build scripts it does parse (LB-Q-gradle) are read with it.

import (
	"slices"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/extractor/tsengine"
)

// TestGroovyRefusedNoDialectClaimsItsFiles: no manifest claims a Groovy source, and no groovy
// dialect is registered; a re-plan that builds one flips this test with its own reading.
func TestGroovyRefusedNoDialectClaimsItsFiles(t *testing.T) {
	ds, err := LoadDialects()
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range ds {
		if d.ID == "groovy" {
			t.Fatalf("a groovy dialect is registered, but LB-Q-groovy is refused (LB-Q-groovy-rest)")
		}
		for _, x := range []string{".groovy", ".gvy"} {
			if slices.Contains(d.Match.Extensions, x) || (d.Match.Sniff != nil && slices.Contains(d.Match.Sniff.Extensions, x)) {
				t.Errorf("dialect %s claims %s, but LB-Q-groovy is refused", d.ID, x)
			}
		}
	}
}

// TestGroovyGrammarPinned (LB-F93): the lock pins murtaza64/tree-sitter-groovy by commit
// (MIT, read at the pin), and the blob it builds parses a Gradle build script.
func TestGroovyGrammarPinned(t *testing.T) {
	lock, err := tsengine.LoadGrammarLock()
	if err != nil {
		t.Fatal(err)
	}
	row, ok := lock.Row("groovy")
	if !ok {
		t.Fatal("no groovy row in grammars.lock.json")
	}
	if row.Repo != "murtaza64/tree-sitter-groovy" || row.Licence != "MIT" || !slices.Contains(row.EntryPoints, "tree_sitter_groovy") {
		t.Fatalf("groovy row = %+v", row)
	}
	e, err := NewTSExtractor("gradle")
	if err != nil {
		t.Fatal(err)
	}
	defer e.Close()
	nodes, err := e.Extract("svc", "build.gradle", []byte("plugins {\n    id 'java'\n}\n"))
	if err != nil {
		t.Fatal(err)
	}
	if len(nodes) != 1 || nodes[0].Name != "java" {
		t.Fatalf("nodes = %+v, want the one plugin", nodes)
	}
}
