package readset

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// RC-1: the three facts a bare `if err != nil { continue }` collapses stay
// distinct — absent, unreadable, and skipped are different outcomes, and
// only "unreadable" makes a manifest incomplete.
func TestReadSetRecordsAbsentUnreadableAndSkipped(t *testing.T) {
	dir := t.TempDir()
	good := filepath.Join(dir, "good.md")
	if err := os.WriteFile(good, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	bad := filepath.Join(dir, "bad.md")
	if err := os.WriteFile(bad, []byte("secret"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(bad, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(bad, 0o644) })

	var s Set
	if b, ok := s.ReadFile(good); !ok || string(b) != "hello" {
		t.Fatalf("readable file: ok=%v body=%q", ok, b)
	}
	if _, ok := s.ReadFile(filepath.Join(dir, "nope.md")); ok {
		t.Fatal("a missing file is not a successful read")
	}
	if _, ok := s.ReadFile(bad); ok {
		t.Fatal("an unreadable file is not a successful read")
	}
	s.Skip(filepath.Join(dir, "archive.md"), "out of gate scope")

	m := s.Manifest()
	got := map[Outcome]int{}
	for _, e := range m.Entries {
		got[e.Outcome]++
	}
	if got[Read] != 1 || got[Absent] != 1 || got[Unreadable] != 1 || got[Skipped] != 1 {
		t.Fatalf("each outcome must be distinct, got %v", got)
	}

	// Only a FAILED read makes coverage incomplete. A genuinely missing file
	// and a deliberate skip are facts the report can state.
	if m.Complete() {
		t.Fatal("a failed read must make the manifest incomplete")
	}
	var clean Set
	_, _ = clean.ReadFile(good)
	_, _ = clean.ReadFile(filepath.Join(dir, "nope.md"))
	clean.Skip(filepath.Join(dir, "archive.md"), "out of gate scope")
	if !clean.Manifest().Complete() {
		t.Fatal("absent and skipped are accounted for, not blindness")
	}

	// The note names the path AND the operating system's own wording, which
	// is what tells a reader "TCC wedge" rather than "file missing".
	note := m.Note()
	if !strings.Contains(note, "bad.md") || !strings.Contains(note, "denied") {
		t.Fatalf("note must name path and errno, got %q", note)
	}
}

// RC-3's floor: a count over an incomplete manifest is a LOWER BOUND and can
// never be rendered as a bare total. This is the formatting rule that stops
// "0 invariants" being printed for a repo with 37.
func TestRenderCountIsALowerBoundWhenBlind(t *testing.T) {
	var complete Set
	seen := complete.Manifest()
	if got := seen.RenderCount(37, "invariant(s)"); got != "37 invariant(s)" {
		t.Fatalf("complete coverage renders a plain count, got %q", got)
	}

	dir := t.TempDir()
	bad := filepath.Join(dir, "invariants.md")
	if err := os.WriteFile(bad, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	_ = os.Chmod(bad, 0o000)
	t.Cleanup(func() { _ = os.Chmod(bad, 0o644) })

	var blind Set
	_, _ = blind.ReadFile(bad)
	unseen := blind.Manifest()
	got := unseen.RenderCount(0, "invariant(s)")
	if !strings.HasPrefix(got, "≥ 0 invariant(s)") || !strings.Contains(got, "invariants.md") {
		t.Fatalf("an incomplete count must be a named lower bound, got %q", got)
	}
}

// A re-read must not upgrade a manifest to complete: first outcome wins.
func TestFirstOutcomeWinsSoBlindnessCannotBeUndone(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "f.md")
	if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	_ = os.Chmod(p, 0o000)
	var s Set
	_, _ = s.ReadFile(p)
	_ = os.Chmod(p, 0o644)
	_, _ = s.ReadFile(p)
	if s.Manifest().Complete() {
		t.Fatal("a later successful read must not erase a recorded failure")
	}
}

// Merge folds a sub-reader's manifest in, so a report assembled from several
// readers accounts for all of them.
func TestMergeAccumulatesBlindness(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "f.md")
	_ = os.WriteFile(p, []byte("x"), 0o644)
	_ = os.Chmod(p, 0o000)
	t.Cleanup(func() { _ = os.Chmod(p, 0o644) })

	var sub Set
	_, _ = sub.ReadFile(p)

	var top Set
	if !top.Manifest().Complete() {
		t.Fatal("an empty set has seen everything it looked at")
	}
	top.Merge(sub.Manifest())
	if top.Manifest().Complete() {
		t.Fatal("merging a blind sub-reader must make the whole report blind")
	}
}
