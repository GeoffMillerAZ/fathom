package certify

import "testing"

// fileMap turns a plain map into the (path)->(content, ok, err) closure
// LostDeclarations reads through — the same minimal shape conserve's own
// input_test.go tests build for BaseFile/HeadFile.
func fileMap(m map[string]string) func(path string) ([]byte, bool, error) {
	return func(path string) ([]byte, bool, error) {
		c, ok := m[path]
		if !ok {
			return nil, false, nil
		}
		return []byte(c), true, nil
	}
}

const lostDeclTestSrc = `package pkg

// certifies: invariant I1
func TestI1(t *testing.T) {}
`

// TestLostDeclarations_RemovedNotRetired pins CV-21 (ADR-044 §2.4): a
// certifies target declared at base and gone at head is reported; one
// still declared at head (even under a different test) is not; a target
// never declared at either point is not.
func TestLostDeclarations_RemovedNotRetired(t *testing.T) {
	base := fileMap(map[string]string{
		"pkg/x_test.go": lostDeclTestSrc,
		"pkg/y_test.go": "package pkg\n\n// certifies: invariant I2\nfunc TestI2(t *testing.T) {}\n",
	})
	head := fileMap(map[string]string{
		// pkg/x_test.go deleted entirely at head — I1's only declaration is gone.
		"pkg/y_test.go": "package pkg\n\n// certifies: invariant I2 (golden)\nfunc TestI2Renamed(t *testing.T) {}\n",
	})

	got := LostDeclarations("svc", []string{"pkg/x_test.go", "pkg/y_test.go"}, base, head)

	if len(got) != 1 {
		t.Fatalf("LostDeclarations = %+v, want exactly one lost declaration (I1)", got)
	}
	if got[0].Kind != "invariant" || got[0].ID != "I1" {
		t.Errorf("lost declaration = %+v, want {invariant I1}", got[0])
	}
}

// TestLostDeclarations_ScopedToChangedFiles pins "base facts come from
// the changed files only": a certifies marker in a file NOT in
// changedFiles never contributes, even if it would otherwise look lost.
func TestLostDeclarations_ScopedToChangedFiles(t *testing.T) {
	base := fileMap(map[string]string{"pkg/untouched_test.go": lostDeclTestSrc})
	head := fileMap(map[string]string{}) // as if I1's marker vanished

	got := LostDeclarations("svc", []string{"pkg/other.go"}, base, head)
	if len(got) != 0 {
		t.Errorf("LostDeclarations = %+v, want none (untouched_test.go was never in changedFiles)", got)
	}
}

// TestLostDeclarations_NoLossWhenPresentAtBoth pins the negative case.
func TestLostDeclarations_NoLossWhenPresentAtBoth(t *testing.T) {
	base := fileMap(map[string]string{"pkg/x_test.go": lostDeclTestSrc})
	head := fileMap(map[string]string{"pkg/x_test.go": lostDeclTestSrc})
	got := LostDeclarations("svc", []string{"pkg/x_test.go"}, base, head)
	if len(got) != 0 {
		t.Errorf("LostDeclarations = %+v, want none", got)
	}
}

// TestLostDeclarations_NilReadsAreSafe pins the nil-safe contract (an
// Input built without BaseFile/HeadFile must never panic).
func TestLostDeclarations_NilReadsAreSafe(t *testing.T) {
	got := LostDeclarations("svc", []string{"pkg/x_test.go"}, nil, nil)
	if len(got) != 0 {
		t.Errorf("LostDeclarations with nil reads = %+v, want none", got)
	}
}
