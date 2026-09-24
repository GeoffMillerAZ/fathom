package integrity

import (
	"reflect"
	"sort"
	"testing"
)

// TestGuardedTests_EveryReason is CV-7's own anchor: every one of design
// §1.2's five reasons guards a test, an unguarded test is omitted, and a
// test guarded by more than one reason carries every one of them (sorted).
func TestGuardedTests_EveryReason(t *testing.T) {
	src := []byte(`package pkg

import "testing"

// certifies: invariant I99 (asserts)
func TestMarkerGuarded(t *testing.T) {
	t.Log("ok")
}

func TestManifestGuarded(t *testing.T) {
	t.Log("ok")
}

func TestInvariantGuarded(t *testing.T) {
	t.Log("ok")
}

func TestSurfaceGuarded(t *testing.T) {
	t.Log("ok")
}

func TestReachGuarded(t *testing.T) {
	t.Log("ok")
}

// certifies: invariant I100 (asserts)
func TestDoublyGuarded(t *testing.T) {
	t.Log("ok")
}

func TestUnguarded(t *testing.T) {
	t.Log("ok")
}
`)

	in := GuardedInput{
		Sources:              []Source{{Path: "pkg/x_test.go", Content: src}},
		ManifestCertifies:    []string{"TestManifestGuarded"},
		InvariantTests:       []string{"TestInvariantGuarded", "TestDoublyGuarded"},
		SurfaceEvidenceTests: []string{"TestSurfaceGuarded"},
		ChangedSymbols:       []string{"pkg.Foo"},
		Reaches: func(test, symbol string) bool {
			return test == "TestReachGuarded" && symbol == "pkg.Foo"
		},
	}

	got := GuardedTests(in)
	want := map[string][]GuardReason{
		"TestMarkerGuarded":    {ReasonMarker},
		"TestManifestGuarded":  {ReasonManifest},
		"TestInvariantGuarded": {ReasonInvariant},
		"TestSurfaceGuarded":   {ReasonSurface},
		"TestReachGuarded":     {ReasonReach},
		"TestDoublyGuarded":    {ReasonMarker, ReasonInvariant},
	}
	if len(got) != len(want) {
		names := make([]string, len(got))
		for i, g := range got {
			names[i] = g.Name
		}
		t.Fatalf("got %d guarded tests %v, want %d: %v", len(got), names, len(want), keysOf(want))
	}
	for _, g := range got {
		wr, ok := want[g.Name]
		if !ok {
			t.Errorf("unexpected guarded test %q (reasons %v)", g.Name, g.Reasons)
			continue
		}
		if !reflect.DeepEqual(g.Reasons, wr) {
			t.Errorf("%s reasons = %v, want %v", g.Name, g.Reasons, wr)
		}
	}
}

// TestGuardedTests_NilReachesContributesNothing: design §1.2's own
// parenthetical — "(when an index is available)" — a nil Reaches must
// never fabricate ReasonReach.
func TestGuardedTests_NilReachesContributesNothing(t *testing.T) {
	src := []byte(`package pkg

func TestPlain(t *testing.T) {}
`)
	got := GuardedTests(GuardedInput{
		Sources:        []Source{{Path: "x_test.go", Content: src}},
		ChangedSymbols: []string{"pkg.Foo"},
		Reaches:        nil,
	})
	if len(got) != 0 {
		t.Errorf("GuardedTests with nil Reaches guarded %d test(s), want 0: %+v", len(got), got)
	}
}

// TestGuardedTests_Deterministic: sorted by (File, Line, Name), stable
// across repeated calls.
func TestGuardedTests_Deterministic(t *testing.T) {
	src := []byte(`package pkg

func TestB(t *testing.T) {}

func TestA(t *testing.T) {}
`)
	in := GuardedInput{
		Sources:        []Source{{Path: "x_test.go", Content: src}},
		InvariantTests: []string{"TestA", "TestB"},
	}
	got1 := GuardedTests(in)
	got2 := GuardedTests(in)
	if !reflect.DeepEqual(got1, got2) {
		t.Fatalf("GuardedTests is not deterministic:\n%+v\n%+v", got1, got2)
	}
	if len(got1) != 2 || got1[0].Name != "TestB" || got1[1].Name != "TestA" {
		t.Errorf("GuardedTests order = %v, want file-order (TestB then TestA, by Line)", names(got1))
	}
	if !sort.SliceIsSorted(got1, func(i, j int) bool { return got1[i].Line < got1[j].Line }) {
		t.Errorf("GuardedTests not sorted by Line: %+v", got1)
	}
}

func names(gs []GuardedTest) []string {
	out := make([]string, len(gs))
	for i, g := range gs {
		out[i] = g.Name
	}
	return out
}

func keysOf(m map[string][]GuardReason) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
