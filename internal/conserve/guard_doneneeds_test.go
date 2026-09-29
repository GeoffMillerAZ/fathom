package conserve

// guard_doneneeds_test.go — PR-2 (ADR-057 decision 3): a guard's `done
// needs:` tests join the CV-7 guarded set under their own reason,
// guard_done_needs, read from .fathom/guard.cue AT BASE through guardspec.
// The analysis half (GuardedTests, namesExplicitly) is tested in
// internal/analysis/integrity; these tests cover the source the plane
// reads and the product path: fathom conserve's integrity plane.

import (
	"context"
	"reflect"
	"strings"
	"testing"
)

const guardCueWithDoneNeeds = `guards: [
	{
		scopes: ["internal/store/**"]
		note: "intent: governed by I2 [gate] · I3 [test]; done needs: TestStoreFormat, TestStorePortability_RoundTrip (fathom intent audit) @ bd6866a32795"
		max_rules: 2
	},
	{
		scopes: ["cmd/fathom/verify.go"]
		note: "intent: done needs: TestVerifyPlan_Verified (fathom intent audit) @ 0c40e5b22509"
	},
	{
		command_scopes: ["npm *"]
		note: "No Node toolchain here; Tests are Go only, TestNotNamedHere mentioned outside any done-needs segment"
	},
]
`

// TestGuardDoneNeedsAt_ReadsTestShapedNamesFromBase: names come from the
// `done needs:` segment of every guard note, through guardspec, and only
// Test-shaped ones — prose, invariant ids, rungs and the stamp are not
// tests, and a name outside the segment is not named by it.
func TestGuardDoneNeedsAt_ReadsTestShapedNamesFromBase(t *testing.T) {
	got := guardDoneNeedsAt(fr(map[string][]byte{guardFilePath: []byte(guardCueWithDoneNeeds)}))
	want := []string{"TestStoreFormat", "TestStorePortability_RoundTrip", "TestVerifyPlan_Verified"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("guardDoneNeedsAt = %v, want %v", got, want)
	}
}

// TestDoneNeedsTestsOf_Segment pins the note grammar: the segment ends at
// `;`, at the `(fathom intent audit)` provenance tag or at the ` @ ` stamp;
// hand-written notes with a different tail still parse; prose words that
// merely start with "Test" (Tests, Testing) and non-Test tokens are not
// names.
func TestDoneNeedsTestsOf_Segment(t *testing.T) {
	cases := []struct {
		name string
		note string
		want []string
	}{
		{"generated form", "intent: governed by I2 [gate]; done needs: TestA, TestB (fathom intent audit) @ 0187019537f0", []string{"TestA", "TestB"}},
		{"no done needs", "intent: governed by I1 [code] · I11 [test] (fathom intent audit) @ 61ca1e3edc15", nil},
		{"hand written, tail after the list", "Done needs: `TestA`, `Test_b`; run the gate after. TestC is not part of it", []string{"TestA", "Test_b"}},
		{"stamp ends the segment", "done needs: TestA @ 3eac5ce7941c TestB", []string{"TestA"}},
		{"prose that starts with Test", "done needs: Tests for the store, Testing TestReal_1 and Test", []string{"TestReal_1"}},
		{"subtest path keeps the top-level name", "done needs: TestA/sub_case, TestB", []string{"TestA", "TestB"}},
		{"two segments", "done needs: TestA; other; done needs: TestB, TestA", []string{"TestA", "TestB"}},
		{"empty", "", nil},
	}
	for _, c := range cases {
		if got := doneNeedsTestsOf(c.note); !reflect.DeepEqual(got, c.want) {
			t.Errorf("%s: doneNeedsTestsOf(%q) = %v, want %v", c.name, c.note, got, c.want)
		}
	}
}

// TestGuardDoneNeedsAt_DegradesToEmpty: an absent file, a nil reader, a
// read error and a guard file guardspec refuses each contribute nothing
// — the same honest-absence handling invariantTestsAt gives its file. A
// malformed guard file is not parsed ad hoc: guardspec's own refusal
// stands.
func TestGuardDoneNeedsAt_DegradesToEmpty(t *testing.T) {
	if got := guardDoneNeedsAt(nil); got != nil {
		t.Errorf("nil reader: got %v, want nil", got)
	}
	if got := guardDoneNeedsAt(fr(nil)); got != nil {
		t.Errorf("absent file: got %v, want nil", got)
	}
	bad := "guards: [{note: \"done needs: TestA\"}]\n" // governs nothing: guardspec refuses a dead guard
	if got := guardDoneNeedsAt(fr(map[string][]byte{guardFilePath: []byte(bad)})); got != nil {
		t.Errorf("guardspec-refused file: got %v, want nil", got)
	}
	if got := guardDoneNeedsAt(fr(map[string][]byte{guardFilePath: []byte("this is not cue {{{")})); got != nil {
		t.Errorf("unparseable file: got %v, want nil", got)
	}
	errReader := func(string) ([]byte, bool, error) { return nil, false, context.Canceled }
	if got := guardDoneNeedsAt(errReader); got != nil {
		t.Errorf("read error: got %v, want nil", got)
	}
}

// TestIntegrityPlane_GuardDoneNeedsRenameIsFlagged is PR-2's product path:
// through fathom conserve's integrity plane, a test named only by a
// guard's `done needs:` — no marker, manifest, invariant, surface or
// reach — is renamed, and the finding carries the new reason. It is a
// loss (the guard still names the old name), not the informational clean
// rename the same change would be without the guard.
func TestIntegrityPlane_GuardDoneNeedsRenameIsFlagged(t *testing.T) {
	const testPath = "internal/store/x_test.go"
	body := "\tif 1 != 2 {\n\t\tt.Fatalf(\"boom\")\n\t}\n}\n"
	base := map[string][]byte{
		testPath:      []byte("package store\n\nimport \"testing\"\n\nfunc TestStoreFormat(t *testing.T) {\n" + body),
		guardFilePath: []byte(guardCueWithDoneNeeds),
	}
	head := map[string][]byte{
		testPath:      []byte("package store\n\nimport \"testing\"\n\nfunc TestStoreFormatRenamed(t *testing.T) {\n" + body),
		guardFilePath: []byte(guardCueWithDoneNeeds),
	}
	in := Input{Service: "svc", ChangedFiles: []string{testPath}, BaseFile: fr(base), HeadFile: fr(head)}

	rep := Run(context.Background(), in, []Plane{integrityPlane{}})
	pr := rep.Planes[0]
	if len(pr.Findings) != 1 {
		t.Fatalf("got %d findings, want 1: %+v", len(pr.Findings), pr.Findings)
	}
	f := pr.Findings[0]
	if f.Kind != "renamed" || f.Key != "TestStoreFormat" {
		t.Errorf("got kind=%q key=%q, want renamed TestStoreFormat", f.Kind, f.Key)
	}
	if !strings.Contains(f.Detail, "guarded by: guard_done_needs") {
		t.Errorf("Detail = %q, want it to name the reason guard_done_needs", f.Detail)
	}
	if pr.Verdict != Losses {
		t.Errorf("verdict = %q, want losses (a done-needs-named rename is still named at base)", pr.Verdict)
	}
}

// TestIntegrityPlane_GuardDoneNeedsReadFromBase is the laundering repro
// for this source (design §1.2's "at base", the same shape as
// TestIntegrityPlane_InvariantGuardReadFromBase): the test is deleted and
// the SAME change drops the guard's note. Reading head's guard file would
// see no done-needs, guard the test on neither side and report nothing.
func TestIntegrityPlane_GuardDoneNeedsReadFromBase(t *testing.T) {
	const testPath = "internal/store/x_test.go"
	base := map[string][]byte{
		testPath:      []byte("package store\n\nimport \"testing\"\n\nfunc TestStoreFormat(t *testing.T) {\n\tif 1 != 2 {\n\t\tt.Fatalf(\"boom\")\n\t}\n}\n"),
		guardFilePath: []byte(guardCueWithDoneNeeds),
	}
	head := map[string][]byte{
		testPath:      []byte("package store\n\nimport \"testing\"\n"),
		guardFilePath: []byte("guards: [{scopes: [\"internal/store/**\"], note: \"store rules\"}]\n"),
	}
	findings, missing := integrityPlane{}.Check(context.Background(), Input{
		Service:      "svc",
		ChangedFiles: []string{testPath, guardFilePath},
		BaseFile:     fr(base),
		HeadFile:     fr(head),
	})
	if len(missing) != 0 {
		t.Fatalf("missing = %v, want none", missing)
	}
	if len(findings) != 1 || findings[0].Kind != "deleted" || findings[0].Key != "TestStoreFormat" {
		t.Fatalf("got %+v, want one deleted finding for TestStoreFormat (named by the guard at base)", findings)
	}
}

// TestIntegrityPlane_NoGuardFileNoDoneNeedsFindings: a repo with no
// .fathom/guard.cue guards nothing under this reason — a clean rename of
// an otherwise unnamed test stays informational.
func TestIntegrityPlane_NoGuardFileNoDoneNeedsFindings(t *testing.T) {
	const testPath = "internal/store/x_test.go"
	body := "\tif 1 != 2 {\n\t\tt.Fatalf(\"boom\")\n\t}\n}\n"
	base := map[string][]byte{testPath: []byte("package store\n\nimport \"testing\"\n\nfunc TestStoreFormat(t *testing.T) {\n" + body)}
	head := map[string][]byte{testPath: []byte("package store\n\nimport \"testing\"\n\nfunc TestStoreFormatRenamed(t *testing.T) {\n" + body)}
	findings, _ := integrityPlane{}.Check(context.Background(), Input{
		Service: "svc", ChangedFiles: []string{testPath}, BaseFile: fr(base), HeadFile: fr(head),
	})
	if len(findings) != 0 {
		t.Fatalf("got %+v, want no findings: nothing guards TestStoreFormat", findings)
	}
}
