package integrity

import (
	"reflect"
	"sort"
	"testing"
)

// fileReader builds an Input.BaseFile/HeadFile-shaped closure over a plain
// map — the in-memory stand-in for conserve.Input's git-backed BaseFile/
// HeadFile in every test in this file.
func fileReader(files map[string][]byte) func(string) ([]byte, bool, error) {
	return func(path string) ([]byte, bool, error) {
		b, ok := files[path]
		return b, ok, nil
	}
}

// TestCheck_EveryKind is CV-9's own anchor: one guarded test per kind in
// design §1.3's table, all in a single changed file, each must produce
// exactly the finding its scenario calls for.
func TestCheck_EveryKind(t *testing.T) {
	const path = "pkg/sample_test.go"

	base := map[string][]byte{
		path: []byte(`package sample

import "testing"

func TestWillBeDeleted(t *testing.T) {
	if 1 != 2 {
		t.Fatalf("boom")
	}
}

func TestPureRename(t *testing.T) {
	x := compute()
	if x != 1 {
		t.Fatalf("got %d", x)
	}
}

func TestWillBeWeakened(t *testing.T) {
	if got := 1; got != 1 {
		t.Errorf("a")
	}
	if got := 2; got != 2 {
		t.Errorf("b")
	}
}

func TestWillBeSkipped(t *testing.T) {
	if 1 != 1 {
		t.Errorf("x")
	}
}

func TestWillGoldenChange(t *testing.T) {
	data := readFile("testdata/golden.txt")
	if data != "want" {
		t.Errorf("mismatch: %s", data)
	}
}

// certifies: invariant I92 (asserts)
func TestMarkerWillMove(t *testing.T) {
	if 1 != 1 {
		t.Errorf("y")
	}
}

func TestWillBeEdited(t *testing.T) {
	x := compute()
	if x != 1 {
		t.Errorf("z")
	}
}
`),
		"testdata/golden.txt": []byte("base content"),
	}

	head := map[string][]byte{
		path: []byte(`package sample

import "testing"

func TestPureRenameRenamed(t *testing.T) {
	x := compute()
	if x != 1 {
		t.Fatalf("got %d", x)
	}
}

func TestWillBeWeakened(t *testing.T) {
	if got := 1; got != 1 {
		t.Errorf("a")
	}
}

func TestWillBeSkipped(t *testing.T) {
	t.Skip("flaky")
	if 1 != 1 {
		t.Errorf("x")
	}
}

func TestWillGoldenChange(t *testing.T) {
	data := readFile("testdata/golden.txt")
	if data != "want" {
		t.Errorf("mismatch: %s", data)
	}
}

func TestMarkerWillMove(t *testing.T) {
	if 1 != 1 {
		t.Errorf("y")
	}
}

func TestWillBeEdited(t *testing.T) {
	x := compute()
	if x != 1 {
		t.Errorf("z: got %d", x)
	}
}
`),
		"testdata/golden.txt": []byte("head content — changed"),
	}

	guardedNames := []string{
		"TestWillBeDeleted", "TestPureRename", "TestWillBeWeakened",
		"TestWillBeSkipped", "TestWillGoldenChange", "TestMarkerWillMove",
		"TestWillBeEdited",
	}

	in := Input{
		ChangedFiles: []string{path},
		BaseFile:     fileReader(base),
		HeadFile:     fileReader(head),
		Guard: GuardedInput{
			InvariantTests: guardedNames, // guards everything uniformly; TestMarkerWillMove ALSO carries its own marker
		},
	}

	findings := Check(in)
	byKey := map[string][]Finding{}
	for _, f := range findings {
		byKey[f.Key] = append(byKey[f.Key], f)
	}

	assertOneKind(t, byKey, "TestWillBeDeleted", KindDeleted)
	assertOneKind(t, byKey, "TestPureRename", KindRenamed)
	assertOneKind(t, byKey, "TestWillBeWeakened", KindWeakened)
	assertOneKind(t, byKey, "TestWillBeSkipped", KindSkipped)
	assertOneKind(t, byKey, "TestWillGoldenChange", KindGoldenChanged)
	assertOneKind(t, byKey, "TestMarkerWillMove", KindMarkerMoved)
	assertOneKind(t, byKey, "TestWillBeEdited", KindEdited)

	if fs := byKey["TestPureRename"]; len(fs) == 1 {
		if fs[0].RenamedTo != "TestPureRenameRenamed" {
			t.Errorf("RenamedTo = %q, want TestPureRenameRenamed", fs[0].RenamedTo)
		}
	}

	// Nothing beyond the seven guarded names produced a finding.
	for key := range byKey {
		found := false
		for _, g := range guardedNames {
			if g == key {
				found = true
			}
		}
		if !found {
			t.Errorf("unexpected finding for un-scenario'd key %q: %+v", key, byKey[key])
		}
	}
}

func assertOneKind(t *testing.T, byKey map[string][]Finding, key string, want Kind) {
	t.Helper()
	fs := byKey[key]
	if len(fs) != 1 {
		t.Fatalf("%s: got %d finding(s) %+v, want exactly 1", key, len(fs), fs)
	}
	if fs[0].Kind != want {
		t.Errorf("%s: Kind = %q, want %q (detail: %s)", key, fs[0].Kind, want, fs[0].Detail)
	}
}

// TestCheck_I28Fixture reproduces the guards-pilot-8 i28 shape named in
// docs/vision/34-refactored-means-conserved.md §2: "an agent renamed and
// inverted the test that certifies invariant I28; `go test -run` then
// matched nothing and printed `ok`." Two certifying tests, each guarded by
// its own "// certifies: invariant I28" marker: one renamed with its
// assertion inverted, one simply deleted outright. BOTH must flag — the
// rename+invert case as `deleted` (its normalized body no longer matches
// anything at head, exactly because the inversion changed a comparison
// operator; see TestNormalizedBodyHash_OperatorChangeDiffers), never
// silently absorbed as an unremarkable "renamed."
func TestCheck_I28Fixture(t *testing.T) {
	const path = "internal/intent/i28_test.go"

	base := map[string][]byte{
		path: []byte(`package intent

import "testing"

// certifies: invariant I28 (asserts)
func TestI28InvariantHeld(t *testing.T) {
	violated := checkI28()
	if violated {
		t.Fatalf("I28 violated")
	}
}

// certifies: invariant I28 (asserts)
func TestI28SecondCell(t *testing.T) {
	if !invariantI28Holds() {
		t.Fatalf("I28 second cell failed")
	}
}
`),
	}
	head := map[string][]byte{
		path: []byte(`package intent

import "testing"

// TestI28InvariantHeld renamed to TestI28InvariantHeldRenamed, with its
// assertion inverted — the exact guards-pilot-8 failure: the check that
// used to fail now can't even run under its old name, AND would pass
// vacuously under its new one.
func TestI28InvariantHeldRenamed(t *testing.T) {
	violated := checkI28()
	if !violated {
		t.Fatalf("I28 violated")
	}
}

// TestI28SecondCell is gone entirely — no replacement anywhere.
`),
	}

	in := Input{
		ChangedFiles: []string{path},
		BaseFile:     fileReader(base),
		HeadFile:     fileReader(head),
	}
	findings := Check(in)

	byKey := map[string]Finding{}
	for _, f := range findings {
		byKey[f.Key] = f
	}

	renamedInverted, ok := byKey["TestI28InvariantHeld"]
	if !ok {
		t.Fatalf("TestI28InvariantHeld (renamed + inverted) produced NO finding — the i28 failure mode was not caught")
	}
	if renamedInverted.Kind != KindDeleted {
		t.Errorf("TestI28InvariantHeld: Kind = %q, want %q (inverting the assertion must break the normalized-body match, not merely relabel it renamed)", renamedInverted.Kind, KindDeleted)
	}

	deleted, ok := byKey["TestI28SecondCell"]
	if !ok {
		t.Fatalf("TestI28SecondCell (plain deletion) produced NO finding — a deleted certifying test was not caught")
	}
	if deleted.Kind != KindDeleted {
		t.Errorf("TestI28SecondCell: Kind = %q, want %q", deleted.Kind, KindDeleted)
	}
}

// TestCheck_MarkerMovedToAnotherFunction: the OTHER marker_moved sub-case
// design §1.3 names — the marker now sits on a different function, not
// merely removed.
func TestCheck_MarkerMovedToAnotherFunction(t *testing.T) {
	const path = "pkg/moved_test.go"
	base := map[string][]byte{
		path: []byte(`package pkg

import "testing"

// certifies: invariant I50 (asserts)
func TestOriginalHolder(t *testing.T) {
	if 1 != 1 {
		t.Errorf("x")
	}
}

func TestOtherFunction(t *testing.T) {
	if 2 != 2 {
		t.Errorf("y")
	}
}
`),
	}
	head := map[string][]byte{
		path: []byte(`package pkg

import "testing"

func TestOriginalHolder(t *testing.T) {
	if 1 != 1 {
		t.Errorf("x")
	}
}

// certifies: invariant I50 (asserts)
func TestOtherFunction(t *testing.T) {
	if 2 != 2 {
		t.Errorf("y")
	}
}
`),
	}
	in := Input{
		ChangedFiles: []string{path},
		BaseFile:     fileReader(base),
		HeadFile:     fileReader(head),
	}
	findings := Check(in)
	if len(findings) != 1 {
		t.Fatalf("got %d findings, want 1: %+v", len(findings), findings)
	}
	f := findings[0]
	if f.Kind != KindMarkerMoved || f.Key != "TestOriginalHolder" {
		t.Errorf("got %+v, want marker_moved on TestOriginalHolder", f)
	}
}

// TestCheck_NoGuardedTestsIsEmpty: a changed file with no guarded test
// produces no findings — Check never invents a finding for an ordinary,
// unguarded test.
func TestCheck_NoGuardedTestsIsEmpty(t *testing.T) {
	const path = "pkg/plain_test.go"
	base := map[string][]byte{path: []byte("package pkg\n\nfunc TestPlain(t *testing.T) {}\n")}
	head := map[string][]byte{path: []byte("package pkg\n\nfunc TestPlain(t *testing.T) {\n\t// entirely rewritten\n\tpanic(\"x\")\n}\n")}
	in := Input{ChangedFiles: []string{path}, BaseFile: fileReader(base), HeadFile: fileReader(head)}
	if got := Check(in); len(got) != 0 {
		t.Errorf("got %d findings for an unguarded test, want 0: %+v", len(got), got)
	}
}

// TestCheck_Deterministic: identical Input always produces identical,
// sorted output.
func TestCheck_Deterministic(t *testing.T) {
	const path = "pkg/det_test.go"
	base := map[string][]byte{path: []byte(`package pkg

import "testing"

func TestZ(t *testing.T) { t.Errorf("a") }
func TestA(t *testing.T) { t.Errorf("b") }
`)}
	head := map[string][]byte{path: []byte("package pkg\n\nimport \"testing\"\n")}
	in := Input{
		ChangedFiles: []string{path},
		BaseFile:     fileReader(base),
		HeadFile:     fileReader(head),
		Guard:        GuardedInput{InvariantTests: []string{"TestZ", "TestA"}},
	}
	f1 := Check(in)
	f2 := Check(in)
	if len(f1) != 2 {
		t.Fatalf("got %d findings, want 2", len(f1))
	}
	if !sort.SliceIsSorted(f1, func(i, j int) bool { return f1[i].Key < f1[j].Key }) {
		t.Errorf("findings not sorted by Key: %+v", f1)
	}
	for i := range f1 {
		if !reflect.DeepEqual(f1[i], f2[i]) {
			t.Fatalf("Check is not deterministic at index %d: %+v != %+v", i, f1[i], f2[i])
		}
	}
}
