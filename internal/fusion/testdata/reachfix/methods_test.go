package reachfix

import "testing"

// TestAExtract calls A.Extract only — B.Extract must never be reachable
// from it (VS-5).
func TestAExtract(t *testing.T) {
	a := NewA()
	if a.Extract() != "A" {
		t.Fatal("A.Extract: unexpected result")
	}
}
