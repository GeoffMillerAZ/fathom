package intent

import (
	"reflect"
	"testing"
)

// TestExtractInvariantEnforcementTests_ContentOnly pins the fix behind
// ADR-044 §1.2's "at base" guardedness requirement (plane finding [0] of
// the refactor-conservation branch review): the enforcement-column test
// names must be computable from invariants.md's raw bytes at ONE point in
// history alone, without Load's directory walk over docs/intent/ — the
// shape a caller reading BASE content via `git show <base>:path` actually
// has (it cannot glob a historical tree the way Load globs the live
// filesystem).
func TestExtractInvariantEnforcementTests_ContentOnly(t *testing.T) {
	body := []byte("# Invariants\n\n" +
		"| # | Invariant | Enforced by | Source |\n" +
		"|---|---|---|---|\n" +
		"| I1 | Something always true | test `TestGuarded` | ADR-1 |\n" +
		"| I2 | Something else, unenforced | code (no test) | ADR-2 |\n")

	got := ExtractInvariantEnforcementTests(body)
	want := []string{"TestGuarded"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

// TestExtractInvariantEnforcementTests_NoTableIsNil: a document with no
// invariants table at all names nothing — never a fabricated claim.
func TestExtractInvariantEnforcementTests_NoTableIsNil(t *testing.T) {
	if got := ExtractInvariantEnforcementTests([]byte("# empty doc\n")); got != nil {
		t.Errorf("got %v, want nil for a file with no invariants table", got)
	}
}

// TestExtractInvariantEnforcementTests_MultipleTestsOneRow: a row naming
// more than one test in its enforcement cell contributes every one of
// them.
func TestExtractInvariantEnforcementTests_MultipleTestsOneRow(t *testing.T) {
	body := []byte("| # | Invariant | Enforced by | Source |\n" +
		"|---|---|---|---|\n" +
		"| I9 | Two tests enforce this | test `TestOne`, `TestTwo` | ADR-9 |\n")

	got := ExtractInvariantEnforcementTests(body)
	want := []string{"TestOne", "TestTwo"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}
