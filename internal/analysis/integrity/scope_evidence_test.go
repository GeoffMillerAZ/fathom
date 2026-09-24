package integrity

import (
	"reflect"
	"testing"
)

// TestExtractScopeEvidenceTests_WrappedBulletAndSectionBounds mirrors this
// repository's own docs/intent/scope.md shape: a bullet that word-wraps
// across several lines, with "Evidence:" opening on one line and the test
// name it names landing on the next, plus a test named OUTSIDE the
// Compatibility surface section that must never be picked up.
func TestExtractScopeEvidenceTests_WrappedBulletAndSectionBounds(t *testing.T) {
	md := []byte(`# Scope

## Current goals

- Evidence: ` + "`TestOutsideSection`" + ` must never appear.

## Compatibility surface

A change to any item is breaking.

- Governed Store Format v0: additive within v0; a rename, retype, or removal is v1 and a
  ` + "`format_version`" + ` bump. Evidence: ` + "`docs/spec/store-format-v0.md`" + `,
  ` + "`internal/store/format_v0_test.go`" + ` (` + "`TestStoreFormatV0_RequiredSchema`" + `).
- Plan grammar v2: checkbox states. Evidence: ` + "`docs/plans/README.md`" + `,
  ` + "`internal/verify/markdown.go`" + `.
- No evidence clause at all on this bullet.

## Next section

- Evidence: ` + "`TestAlsoOutside`" + ` must never appear either.
`)

	got := ExtractScopeEvidenceTests(md)
	want := []string{"TestStoreFormatV0_RequiredSchema"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("ExtractScopeEvidenceTests = %v, want %v", got, want)
	}
}

// TestExtractScopeEvidenceTests_NoSection: an absent section is an honest
// nil, never an error or a panic.
func TestExtractScopeEvidenceTests_NoSection(t *testing.T) {
	if got := ExtractScopeEvidenceTests([]byte("# Scope\n\nnothing here\n")); got != nil {
		t.Errorf("got %v, want nil", got)
	}
	if got := ExtractScopeEvidenceTests(nil); got != nil {
		t.Errorf("got %v, want nil for empty input", got)
	}
}

// TestExtractScopeEvidenceTests_MultipleTestsOneBullet: a single Evidence
// clause naming more than one test names all of them, deduplicated and
// sorted.
func TestExtractScopeEvidenceTests_MultipleTestsOneBullet(t *testing.T) {
	md := []byte(`## Compatibility surface

- Two tests. Evidence: ` + "`TestZ`, `TestA`, `TestA`" + `.
`)
	got := ExtractScopeEvidenceTests(md)
	want := []string{"TestA", "TestZ"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}
