package reachfix

import "testing"

// TestClosureOnly calls onlyInClosure exclusively from inside a t.Run
// subtest closure — never at this function's own top level.
func TestClosureOnly(t *testing.T) {
	t.Run("x", func(t *testing.T) {
		if onlyInClosure() != 42 {
			t.Fatal("onlyInClosure: unexpected result")
		}
	})
}
