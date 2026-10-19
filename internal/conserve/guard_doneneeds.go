package conserve

// guard_doneneeds.go — the source of the CV-7 guard_done_needs reason
// (ADR-057 decision 3, plan PR-2): the tests a guard note's `done needs:`
// segment names in .fathom/guard.cue. `fathom intent audit` renders that
// segment from the router's surface table (cmd/fathom/intentexport.go
// guardNote: `intent: governed by I2 [gate]; done needs: TestA, TestB
// (fathom intent audit) @ <stamp>`); the guard file is read, not the
// router, because it is what governs edits and an adopter may hand-write it.
//
// Read AT BASE through Input.BaseFile, the same discipline invariantTestsAt
// applies to invariants.md and for the same reason (design §1.2's "at
// base"): reading head would let a change delete or rename a test and drop
// the guard's citation of it in the same commit, guarding the test on
// neither side. The file is decoded by guardspec (the conforming reader of
// #Guards), never scraped as CUE text; only the free-text note is then
// parsed, and only for Test-shaped names.

import (
	"regexp"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/guardspec"
	"github.com/geoffmilleraz/fathom/internal/projectdir"
)

// guardFilePath is the repo-relative project guard file.
const guardFilePath = projectdir.Dir + "/" + string(projectdir.Guard)

// doneNeedsLabel opens the segment guardNote writes; matched
// case-insensitively so a hand-written "Done needs:" reads the same.
var doneNeedsLabel = regexp.MustCompile(`(?i)done needs:`)

// goTestName is a top-level Go test function name: "Test" then a rune that
// is not a lowercase letter (go test's own rule), so prose such as "Tests"
// or "Testing" is not a name, and a bare "Test" is not one either.
var goTestName = regexp.MustCompile(`\bTest[A-Z0-9_][A-Za-z0-9_]*`)

// segmentEnds are the delimiters that end a `done needs:` segment: the next
// note part (`;`), the provenance tag guardNote appends and its stamp.
var segmentEnds = []string{";", "(fathom intent audit)", " @ ", "\n"}

// guardDoneNeedsAt returns, sorted and deduplicated, every Test-shaped name
// a guard note's `done needs:` segment names in .fathom/guard.cue at base.
// An absent file, a nil reader, a read error or a file guardspec refuses
// (malformed, or a guard that governs nothing) contributes nothing — the
// honest-absence handling invariantTestsAt gives its own file; a guard file
// guardspec does not serve is not serving an edit either.
func guardDoneNeedsAt(baseFile func(string) ([]byte, bool, error)) []string {
	if baseFile == nil {
		return nil
	}
	b, ok, err := baseFile(guardFilePath)
	if err != nil || !ok {
		return nil
	}
	return guardDoneNeedsIn(b)
}

// guardDoneNeedsIn is guardDoneNeedsAt over the guard file's bytes: the
// sorted, deduplicated `done needs:` test names, nil for a file guardspec
// refuses. PinLosses also reads it at head, to tell a silent loss from a
// retired pin.
func guardDoneNeedsIn(b []byte) []string {
	set, err := guardspec.ParseGuards(guardFilePath, b)
	if err != nil {
		return nil
	}
	seen := map[string]bool{}
	for _, g := range set.Guards {
		for _, name := range doneNeedsTestsOf(g.Note) {
			seen[name] = true
		}
	}
	return setToSortedSlice(seen)
}

// doneNeedsTestsOf returns the sorted, deduplicated Test-shaped names in
// every `done needs:` segment of one guard note.
func doneNeedsTestsOf(note string) []string {
	seen := map[string]bool{}
	for _, loc := range doneNeedsLabel.FindAllStringIndex(note, -1) {
		segment := note[loc[1]:]
		for _, d := range segmentEnds {
			if j := strings.Index(segment, d); j >= 0 {
				segment = segment[:j]
			}
		}
		for _, name := range goTestName.FindAllString(segment, -1) {
			seen[name] = true
		}
	}
	return setToSortedSlice(seen)
}
