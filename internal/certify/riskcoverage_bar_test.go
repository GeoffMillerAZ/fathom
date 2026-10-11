package certify_test

import (
	"testing"

	"github.com/geoffmilleraz/fathom/internal/certify"
	"github.com/geoffmilleraz/fathom/internal/conserve"
)

// TestCoverageMetaBarMatchesConserve pins the risk rank's bar for the
// attested Go pair (TQ-26) to internal/conserve's CoverageMetaUsable: one
// rule (bound, clean, passed), checked in two packages that cannot share
// the code (conserve imports certify), so the copies can never drift.
func TestCoverageMetaBarMatchesConserve(t *testing.T) {
	for _, commit := range []string{"", "0123456789abcdef0123456789abcdef01234567"} {
		for _, dirty := range []bool{false, true} {
			for _, passed := range []bool{false, true} {
				ok, why := certify.CoverageMetaUsable(commit, dirty, passed)
				wantOK, wantWhy := conserve.CoverageMetaUsable(conserve.CoverageMeta{Commit: commit, Dirty: dirty, Passed: passed})
				if ok != wantOK || why != wantWhy {
					t.Errorf("commit=%q dirty=%v passed=%v: certify (%v, %q), conserve (%v, %q)", commit, dirty, passed, ok, why, wantOK, wantWhy)
				}
			}
		}
	}
}
