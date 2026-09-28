package langbench

import (
	"path/filepath"
	"testing"
)

// TestDialectDataAndGrammarPinsFromRepo reads this checkout's dialect
// directories and grammar lock: every directory's blob has a lock row with
// its pin, licence and embedded size.
func TestDialectDataAndGrammarPinsFromRepo(t *testing.T) {
	root := filepath.Join("..", "..")
	ds, err := DialectDataFromRepo(root)
	if err != nil {
		t.Fatal(err)
	}
	pins, err := GrammarPinsFromRepo(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(ds) == 0 || len(pins) == 0 {
		t.Fatalf("dialects %d, pins %d: want both", len(ds), len(pins))
	}
	for i, d := range ds {
		if i > 0 && ds[i-1].ID >= d.ID {
			t.Errorf("not sorted at %s", d.ID)
		}
		if d.Engine == "" || d.Display == "" {
			t.Errorf("%s: engine %q display %q", d.ID, d.Engine, d.Display)
		}
		if d.Blob == "" {
			continue
		}
		p, ok := pins[d.Blob]
		if !ok || p.Repo == "" || p.Tag == "" || p.Licence == "" || p.GzBytes <= 0 {
			t.Errorf("%s: pin %+v", d.ID, p)
		}
	}
	if none, err := GrammarPinsFromRepo(t.TempDir()); err != nil || len(none) != 0 {
		t.Errorf("no lock = %v, %v; want an empty map", none, err)
	}
}
