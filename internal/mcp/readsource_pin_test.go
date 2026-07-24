// Internal (same-package) pin test for read_source's file-size ceiling.
package mcp

import (
	"testing"

	"github.com/geoffmilleraz/fathom/internal/fusion"
)

// TestReadSource_FileCeilingMatchesExtractCap pins sourceReadMaxFileBytes to
// internal/fusion.MaxExtractSize. The ceiling's honesty story ("a file this
// large isn't in the index either — the extraction cap skipped it") only
// holds while the two numbers are EQUAL; this test makes any future change
// to either constant a loud, deliberate decision instead of silent drift.
// The constant is duplicated rather than imported so internal/mcp's
// production build does not depend on the whole fusion package for one int.
func TestReadSource_FileCeilingMatchesExtractCap(t *testing.T) {
	if sourceReadMaxFileBytes != fusion.MaxExtractSize {
		t.Fatalf("sourceReadMaxFileBytes = %d, fusion.MaxExtractSize = %d — the read ceiling must equal the extraction cap (update both together, and the tool Description + docs/mcp-tools.md)", sourceReadMaxFileBytes, fusion.MaxExtractSize)
	}
}
