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

// TestGuardedTestsIncludesGuardDoneNeeds is PR-2's anchor (ADR-057
// decision 3): a test a guard's `done needs:` names is guarded under its
// own reason, alone or beside another one, and a test no source names
// stays unguarded. The caller flattens the field, so this package reads
// no guard file.
func TestGuardedTestsIncludesGuardDoneNeeds(t *testing.T) {
	src := []byte(`package pkg

import "testing"

func TestDoneNeedsOnly(t *testing.T) {
	t.Log("ok")
}

func TestDoneNeedsAndInvariant(t *testing.T) {
	t.Log("ok")
}

func TestNamedByNobody(t *testing.T) {
	t.Log("ok")
}
`)
	got := GuardedTests(GuardedInput{
		Sources:             []Source{{Path: "pkg/x_test.go", Content: src}},
		InvariantTests:      []string{"TestDoneNeedsAndInvariant"},
		GuardDoneNeedsTests: []string{"TestDoneNeedsOnly", "TestDoneNeedsAndInvariant", "TestNotInAnyFile"},
	})
	// Reasons are sorted by value: "guard_done_needs" < "invariant_enforcement".
	want := map[string][]GuardReason{
		"TestDoneNeedsOnly":         {ReasonGuardDoneNeeds},
		"TestDoneNeedsAndInvariant": {ReasonGuardDoneNeeds, ReasonInvariant},
	}
	if len(got) != len(want) {
		t.Fatalf("got %d guarded tests %v, want %d: %v", len(got), names(got), len(want), keysOf(want))
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
	if string(ReasonGuardDoneNeeds) != "guard_done_needs" {
		t.Errorf("ReasonGuardDoneNeeds = %q, want the additive wire value guard_done_needs", ReasonGuardDoneNeeds)
	}
}

// TestCheck_RenamedGuardDoneNeedsTestIsStillNamed: a guard's `done needs:`
// names its test in a durable place, so renaming it is a loss exactly as
// renaming an invariant-named test is (ADR-057 decision 3) — the new
// reason counts as an explicit naming source for StillNamedElsewhere.
// Deleting it is a deleted finding carrying the new reason.
func TestCheck_RenamedGuardDoneNeedsTestIsStillNamed(t *testing.T) {
	const path = "pkg/x_test.go"
	baseSrc := "package pkg\n\nimport \"testing\"\n\nfunc TestDoneNeeded(t *testing.T) {\n\tif 1 != 2 {\n\t\tt.Fatalf(\"boom\")\n\t}\n}\n"
	guard := GuardedInput{GuardDoneNeedsTests: []string{"TestDoneNeeded"}}

	renamed := Check(Input{
		ChangedFiles: []string{path},
		BaseFile:     fileReader(map[string][]byte{path: []byte(baseSrc)}),
		HeadFile: fileReader(map[string][]byte{path: []byte(
			"package pkg\n\nimport \"testing\"\n\nfunc TestDoneNeededRenamed(t *testing.T) {\n\tif 1 != 2 {\n\t\tt.Fatalf(\"boom\")\n\t}\n}\n")}),
		Guard: guard,
	})
	if len(renamed) != 1 || renamed[0].Kind != KindRenamed {
		t.Fatalf("got %+v, want exactly one renamed finding", renamed)
	}
	if !renamed[0].StillNamedElsewhere {
		t.Errorf("StillNamedElsewhere = false, want true: a guard done-needs names the old test")
	}
	if !reflect.DeepEqual(renamed[0].Reasons, []GuardReason{ReasonGuardDoneNeeds}) {
		t.Errorf("Reasons = %v, want [guard_done_needs]", renamed[0].Reasons)
	}

	deleted := Check(Input{
		ChangedFiles: []string{path},
		BaseFile:     fileReader(map[string][]byte{path: []byte(baseSrc)}),
		HeadFile:     fileReader(map[string][]byte{path: []byte("package pkg\n\nimport \"testing\"\n")}),
		Guard:        guard,
	})
	if len(deleted) != 1 || deleted[0].Kind != KindDeleted || deleted[0].Key != "TestDoneNeeded" {
		t.Fatalf("got %+v, want one deleted finding for TestDoneNeeded", deleted)
	}
}
