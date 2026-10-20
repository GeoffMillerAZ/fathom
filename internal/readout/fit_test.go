package readout

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"
)

// fitCorpus is a labelled corpus with a known shape: lean is right on a
// non-read output over 3,000 bytes and on a whole read over 400 lines, with
// a few labels against the grain so no cut point reads perfect.
func fitCorpus() []FitLabel {
	var ls []FitLabel
	for i := 0; i < 160; i++ {
		b := 200 + i*50
		right := b > 3000
		if i%23 == 0 {
			right = !right
		}
		ls = append(ls, FitLabel{ID: fmt.Sprintf("toolu_out%03d", i), LeanKnown: true, LeanRight: right,
			Features: Features{Event: EventSuccess, Class: StreamDiagnostic, Bytes: b, Lines: b / 40, RuleFloor: 100, Recovery: -1}})
	}
	for i := 0; i < 80; i++ {
		lines := 50 + i*10
		ls = append(ls, FitLabel{ID: fmt.Sprintf("toolu_read%03d", i), LeanKnown: true, LeanRight: lines > 400,
			Features: Features{Event: EventSuccess, Class: StreamContent, Bytes: 40 * lines, Lines: lines, WholeRead: true, RuleFloor: 100, Recovery: -1}})
	}
	for i := 0; i < 20; i++ {
		ls = append(ls, FitLabel{ID: fmt.Sprintf("toolu_fail%03d", i), EnrichKnown: true, EnrichRight: i%2 == 0,
			Features: Features{Event: EventFailure, Class: StreamDiagnostic, Bytes: 900, Lines: 20, Records: map[string]int{RecordCompileError: 2}, EditedOverlap: (i + 1) % 2, Recovery: -1}})
	}
	ls = append(ls, FitLabel{ID: "toolu_unlabelled", Features: Features{Event: EventSuccess}})
	return ls
}

// TestFitPolicyTableIsDeterministicOnASeededHalf: the fit splits the labels
// by a seed into two halves that never share a call, fits the byte floor
// and the long-read cut on one, scores both, and gives the same result for
// the same seed whatever order the labels arrive in. The written policy
// file reads back as the same cut points and moves Choose; an absent file
// leaves the built-ins and an invalid one is refused with them standing.
func TestFitPolicyTableIsDeterministicOnASeededHalf(t *testing.T) {
	labels := fitCorpus()
	res := fitPolicyTable(labels, 7)

	rev := append([]FitLabel(nil), labels...)
	sort.Slice(rev, func(i, j int) bool { return rev[i].ID > rev[j].ID })
	if again := fitPolicyTable(rev, 7); !reflect.DeepEqual(again, res) {
		t.Fatalf("label order moved the fit:\n%+v\n%+v", again, res)
	}
	if again := fitPolicyTable(labels, 7); !reflect.DeepEqual(again, res) {
		t.Fatalf("a second run moved the fit")
	}

	train, test := splitHalves(labels, 7)
	if len(train)+len(test) != len(labels) || len(train) != len(labels)/2 {
		t.Fatalf("halves %d + %d of %d", len(train), len(test), len(labels))
	}
	seen := map[string]bool{}
	for _, l := range train {
		seen[l.ID] = true
	}
	for _, l := range test {
		if seen[l.ID] {
			t.Fatalf("%s is in both halves", l.ID)
		}
	}
	other, _ := splitHalves(labels, 8)
	if reflect.DeepEqual(ids(other), ids(train)) {
		t.Fatalf("seeds 7 and 8 gave the same half")
	}

	if res.Seed != 7 || res.Labels != len(labels) || res.Unscored != 1 {
		t.Fatalf("seed %d labels %d unscored %d", res.Seed, res.Labels, res.Unscored)
	}
	if res.Fit.All.N+res.HeldOut.All.N != len(labels)-1 {
		t.Fatalf("scored %d + %d, want %d", res.Fit.All.N, res.HeldOut.All.N, len(labels)-1)
	}
	if f := res.Policy.LeanFloorBytes; f < 2500 || f > 3500 {
		t.Fatalf("fitted floor %d, want near 3,000", f)
	}
	if l := res.Policy.LongReadLines; l < 350 || l > 450 {
		t.Fatalf("fitted long-read cut %d, want near 400 (%+v)", l, res)
	}
	if res.Builtin != BuiltinPolicy() {
		t.Fatalf("builtin %+v", res.Builtin)
	}
	for _, h := range []FitHalf{res.Fit, res.HeldOut} {
		a := h.All
		if a.N == 0 || a.Accuracy < 0.85 || a.Lower > a.Accuracy || a.Upper < a.Accuracy || a.Right > a.N {
			t.Fatalf("half %+v: want accuracy at least 0.85 inside its Wilson bounds", a)
		}
		if h.Contested.N == 0 || h.Contested.N > a.N {
			t.Fatalf("contested %+v of %+v", h.Contested, a)
		}
	}

	if empty := fitPolicyTable(nil, 1); empty.Policy != BuiltinPolicy() || empty.Fit.All.N != 0 || empty.HeldOut.All.N != 0 {
		t.Fatalf("no labels: %+v, want the built-in policy and nothing scored", empty)
	}

	home := t.TempDir()
	if p, found, err := LoadPolicyFile(home); err != nil || found || p != BuiltinPolicy() {
		t.Fatalf("absent file: %+v %v %v, want the built-ins, not found, no error", p, found, err)
	}
	if err := WritePolicy(home, res); err != nil {
		t.Fatal(err)
	}
	p, found, err := LoadPolicyFile(home)
	if err != nil || !found || p != res.Policy {
		t.Fatalf("read back %+v %v, want %+v", p, err, res.Policy)
	}
	f := Features{Event: EventSuccess, Class: StreamDiagnostic, Bytes: 2000, Promoted: true, RuleFloor: 100, Recovery: -1}
	if a, b := Choose(f, BuiltinPolicy()), Choose(f, p); a.Mode != ChoiceLean || b.Mode != ChoiceRaw {
		t.Fatalf("built-in %+v, fitted %+v: the fitted floor must hold a 2,000-byte output raw", a, b)
	}
	if entries, _ := os.ReadDir(filepath.Dir(PolicyPath(home))); len(entries) != 1 {
		t.Fatalf("the atomic write left %d entries beside the policy file", len(entries))
	}
	for _, bad := range []string{"version = 1\nlong_read_lines = 0\n", "version = 1\nlong_read_lines = 200\nnonsense = 1\n", "version = 1\nlong_read_lines = 200\nlean_floor_bytes = -4\n", "version = 9\nlong_read_lines = 200\n", "long_read_lines = 200\n", "[[x\n"} {
		if err := os.WriteFile(PolicyPath(home), []byte(bad), 0o644); err != nil {
			t.Fatal(err)
		}
		if p, found, err := LoadPolicyFile(home); err == nil || found || p != BuiltinPolicy() {
			t.Fatalf("invalid %q: %+v %v, want the built-ins and an error", bad, p, err)
		}
	}
}

func ids(ls []FitLabel) []string {
	out := make([]string, len(ls))
	for i, l := range ls {
		out[i] = l.ID
	}
	return out
}
