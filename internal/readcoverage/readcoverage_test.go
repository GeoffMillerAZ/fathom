package readcoverage

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	readmanifest "github.com/geoffmilleraz/agentic-kit/foundations/telemetry/read-manifest"
)

// RC-1: the three facts a bare `if err != nil { continue }` collapses stay
// distinct — absent, unreadable, and skipped are different outcomes, and
// only "unreadable" makes a manifest incomplete.
//
// The accounting now belongs to the kit (foundations/telemetry/read-manifest,
// generalized from the deleted internal/readset). This test keeps its name
// and its job: it is fathom's contract against the package it depends on, so
// a kit change that quietly reinstated the conflation would fail HERE, where
// I38 is stated, rather than nowhere.
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

	var s readmanifest.Set
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
	got := map[readmanifest.Kind]int{}
	for _, e := range m.Entries {
		got[e.Outcome.Kind]++
	}
	if got[readmanifest.KindRead] != 1 || got[readmanifest.KindAbsent] != 1 ||
		got[readmanifest.KindUnreadable] != 1 || got[readmanifest.KindSkipped] != 1 {
		t.Fatalf("each outcome must be distinct, got %v", got)
	}

	// Only a FAILED read makes coverage incomplete. A genuinely missing file
	// and a deliberate skip are facts the report can state.
	if m.Complete() {
		t.Fatal("a failed read must make the manifest incomplete")
	}
	var clean readmanifest.Set
	_, _ = clean.ReadFile(good)
	_, _ = clean.ReadFile(filepath.Join(dir, "nope.md"))
	clean.Skip(filepath.Join(dir, "archive.md"), "out of gate scope")
	if !clean.Manifest().Complete() {
		t.Fatal("absent and skipped are accounted for, not blindness")
	}

	// The note names the path AND the operating system's own wording, which
	// is what tells a reader "TCC wedge" rather than "file missing".
	note := Note(m)
	if !strings.Contains(note, "bad.md") || !strings.Contains(note, "denied") {
		t.Fatalf("note must name path and errno, got %q", note)
	}
}

// RC-3's floor: a count over an incomplete manifest is a LOWER BOUND and can
// never be rendered as a bare total. This is the formatting rule that stops
// "0 invariants" being printed for a repo with 37 — and the rendering is the
// one thing fathom still owns after the deletion onto the kit package.
func TestRenderCountIsALowerBoundWhenBlind(t *testing.T) {
	var complete readmanifest.Set
	seen := complete.Manifest()
	if got := RenderCount(seen, 37, "invariant(s)"); got != "37 invariant(s)" {
		t.Fatalf("complete coverage renders a plain count, got %q", got)
	}

	dir := t.TempDir()
	bad := filepath.Join(dir, "invariants.md")
	if err := os.WriteFile(bad, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	_ = os.Chmod(bad, 0o000)
	t.Cleanup(func() { _ = os.Chmod(bad, 0o644) })

	var blind readmanifest.Set
	_, _ = blind.ReadFile(bad)
	unseen := blind.Manifest()
	got := RenderCount(unseen, 0, "invariant(s)")
	if !strings.HasPrefix(got, "≥ 0 invariant(s)") || !strings.Contains(got, "invariants.md") {
		t.Fatalf("an incomplete count must be a named lower bound, got %q", got)
	}
	// The wording ADR-027 and the CHANGELOG pin, kept byte-identical across
	// the deletion onto the kit package.
	if !strings.Contains(got, "1 source(s) unreadable: ") {
		t.Fatalf("the pinned lower-bound wording must survive the migration, got %q", got)
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
	var s readmanifest.Set
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

	var sub readmanifest.Set
	_, _ = sub.ReadFile(p)

	var top readmanifest.Set
	if !top.Manifest().Complete() {
		t.Fatal("an empty set has seen everything it looked at")
	}
	top.Merge(sub.Manifest())
	if top.Manifest().Complete() {
		t.Fatal("merging a blind sub-reader must make the whole report blind")
	}
}
