package guardspec

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
)

func TestCallDirectionLoadsAndEvaluates(t *testing.T) {
	p := filepath.Join(t.TempDir(), "guard.cue")
	src := `layer_rules: [
	{id: "a-not-b", kind: "call_direction", sentence: "a never calls b.", from: ["a*"], to: ["b"]},
	{id: "a-may-c", kind: "call_direction", sentence: "ok", from: ["a"], to: ["c"], allow: true},
]
`
	if err := os.WriteFile(p, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	set, err := LoadLayerRules(p)
	if err != nil {
		t.Fatal(err)
	}
	if len(set.Rules) != 2 || set.Rules[0].AllowCall || !set.Rules[1].AllowCall {
		t.Fatalf("rules = %+v", set.Rules)
	}
	nodes := []core.SymbolNode{
		{Package: "a", Name: "F", Kind: core.KindGoFunc, FilePath: "a/f.go", Calls: []string{"b.G", "c.H"}},
		{Package: "b", Name: "G", Kind: core.KindGoFunc, FilePath: "b/g.go"},
		{Package: "c", Name: "H", Kind: core.KindGoFunc, FilePath: "c/h.go"},
	}
	idx := BuildIndexFromSymbols(nodes, nil)
	if got := EvaluateLayerRule(set.Rules[0], idx); len(got) != 1 || got[0].From != "a.F" || got[0].To != "b.G" {
		t.Fatalf("deny rule violations = %+v", got)
	}
	if got := EvaluateLayerRule(set.Rules[1], idx); len(got) != 0 {
		t.Fatalf("allow rule violations = %+v", got)
	}
	if n := len(CallDirectionRules(set)); n != 2 {
		t.Fatalf("CallDirectionRules = %d, want 2", n)
	}
}

func TestCallDirectionRequiresFromAndTo(t *testing.T) {
	p := filepath.Join(t.TempDir(), "guard.cue")
	if err := os.WriteFile(p, []byte(`layer_rules: [{id: "x", kind: "call_direction", sentence: "s", from: ["a"]}]`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadLayerRules(p); err == nil {
		t.Fatal("want an error for a call_direction rule with no to")
	}
}
