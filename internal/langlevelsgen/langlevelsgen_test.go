package langlevelsgen

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func put(t *testing.T, dir, name, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestGenerateSortsAndTakesTheGradeAtTheLevelMet(t *testing.T) {
	dir := t.TempDir()
	put(t, dir, "zeta.json", `{"dialect":"zeta","level_met":"L2","grade":{"L1":"excellent","L2":"good","L3":"unmet"}}`)
	put(t, dir, "alpha.json", `{"dialect":"alpha","level_met":"none","grade":{"L1":"unmet"}}`)
	put(t, dir, "alpha.md", "not a reading")
	got, err := Generate(dir)
	if err != nil {
		t.Fatal(err)
	}
	want := `[
  {
    "id": "alpha",
    "level": "none"
  },
  {
    "id": "zeta",
    "level": "L2",
    "grade": "good"
  }
]
`
	if string(got) != want {
		t.Fatalf("got\n%s", got)
	}
}

func TestGenerateRefusesAMisnamedOrIncompleteReading(t *testing.T) {
	for name, body := range map[string]string{
		"a.json": `{"dialect":"b","level_met":"L1"}`,
		"b.json": `{"dialect":"b"}`,
		"c.json": `not json`,
	} {
		dir := t.TempDir()
		put(t, dir, name, body)
		if _, err := Generate(dir); err == nil || !strings.Contains(err.Error(), name) {
			t.Errorf("%s: %v", name, err)
		}
	}
}

// The table is a function of the readings alone: the same readings written in
// another order, or generated twice, give the same bytes, so a merge of two
// branches that each regraded a dialect is settled by regenerating.
func TestGenerateIsDeterministic(t *testing.T) {
	readings := map[string]string{
		"a.json": `{"dialect":"a","level_met":"L1","grade":{"L1":"good"}}`,
		"b.json": `{"dialect":"b","level_met":"L3","grade":{"L3":"excellent"}}`,
		"c.json": `{"dialect":"c","level_met":"none","grade":{}}`,
	}
	var outs []string
	for _, order := range [][]string{{"a.json", "b.json", "c.json"}, {"c.json", "a.json", "b.json"}, {"b.json", "c.json", "a.json"}} {
		dir := t.TempDir()
		for _, name := range order {
			put(t, dir, name, readings[name])
		}
		for i := 0; i < 2; i++ {
			got, err := Generate(dir)
			if err != nil {
				t.Fatal(err)
			}
			outs = append(outs, string(got))
		}
	}
	for _, o := range outs[1:] {
		if o != outs[0] {
			t.Fatalf("generation depends on order or run:\n%s\n---\n%s", outs[0], o)
		}
	}
}
