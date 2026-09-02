package readmanifest_test

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"testing"

	readmanifest "github.com/geoffmilleraz/agentic-kit/foundations/telemetry/read-manifest"
)

// TestFirstOutcomeWins pins the rule that makes blindness sticky: a retry
// that succeeds cannot erase a recorded failure — and, symmetrically, a
// later failure does not overwrite a recorded success (one pass, one
// record per path).
func TestFirstOutcomeWins(t *testing.T) {
	s := readmanifest.NewSet()
	s.Record("a.md", readmanifest.Unreadable(errors.New("operation not permitted")))
	s.Record("a.md", readmanifest.Read(12))
	if s.Manifest().Complete() {
		t.Fatal("a later successful read must not erase a recorded failure")
	}
	if got := s.Manifest().Entries[0].Outcome; got.Kind != readmanifest.KindUnreadable || got.Detail != "operation not permitted" {
		t.Fatalf("first outcome must win, got %+v", got)
	}

	s2 := readmanifest.NewSet()
	s2.Record("b.md", readmanifest.Read(3))
	s2.Record("b.md", readmanifest.Unreadable(errors.New("permission denied")))
	if !s2.Manifest().Complete() {
		t.Fatal("first outcome wins in both directions: a recorded success stays recorded")
	}
}

// TestCompleteFalseOnOneUnreadable pins that only a FAILED read makes the
// manifest incomplete: absence and a stated skip are facts, not blindness.
func TestCompleteFalseOnOneUnreadable(t *testing.T) {
	s := readmanifest.NewSet()
	if !s.Manifest().Complete() {
		t.Fatal("an empty set has seen everything it looked at")
	}
	s.Record("read.md", readmanifest.Read(5))
	s.Record("gone.md", readmanifest.Absent())
	s.Skip("archive.md", "out of scope")
	if !s.Manifest().Complete() {
		t.Fatalf("read + absent + skipped is complete, got %+v", s.Manifest())
	}
	s.Record("locked.md", readmanifest.Unreadable(errors.New("permission denied")))
	m := s.Manifest()
	if m.Complete() {
		t.Fatal("one unreadable path must make the manifest incomplete")
	}
	if bad := m.Unreadable(); len(bad) != 1 || bad[0].Path != "locked.md" {
		t.Fatalf("Unreadable = %+v, want exactly locked.md", bad)
	}
	if got, want := m.Note(), "1 unreadable: locked.md: permission denied"; got != want {
		t.Fatalf("Note = %q, want %q", got, want)
	}
}

// TestLowerBoundByteStable pins the exact text every count over an
// incomplete manifest must carry — the formatting rule that stops "0
// invariants" being printed for a repository with 37.
func TestLowerBoundByteStable(t *testing.T) {
	complete := readmanifest.NewSet()
	complete.Record("x.md", readmanifest.Read(1))
	if got := complete.Manifest().LowerBound(37); got != "37" {
		t.Fatalf("complete coverage renders a plain count, got %q", got)
	}
	if got := (readmanifest.Manifest{}).LowerBound(0); got != "0" {
		t.Fatalf("an empty manifest is complete, got %q", got)
	}

	two := readmanifest.NewSet()
	two.Record("b.md", readmanifest.Unreadable(errors.New("permission denied")))
	two.Record("a.md", readmanifest.Unreadable(errors.New("operation not permitted")))
	two.Record("c.md", readmanifest.Read(9))
	if got, want := two.Manifest().LowerBound(0), "≥ 0 (2 unreadable: a.md, b.md)"; got != want {
		t.Fatalf("LowerBound = %q, want %q", got, want)
	}
	if got, want := two.Manifest().Note(), "2 unreadable: a.md: operation not permitted; b.md: permission denied"; got != want {
		t.Fatalf("Note = %q, want %q", got, want)
	}

	many := readmanifest.NewSet()
	for _, p := range []string{"p7", "p6", "p5", "p4", "p3", "p2", "p1"} {
		many.Record(p, readmanifest.Unreadable(errors.New("permission denied")))
	}
	if got, want := many.Manifest().LowerBound(3), "≥ 3 (7 unreadable: p1, p2, p3, p4, p5, …)"; got != want {
		t.Fatalf("LowerBound caps the inline list, got %q, want %q", got, want)
	}
	// Stability across calls.
	if a, b := many.Manifest().LowerBound(3), many.Manifest().LowerBound(3); a != b {
		t.Fatalf("LowerBound is not byte-stable: %q vs %q", a, b)
	}
}

// TestSortedOutput pins deterministic, path-sorted manifests regardless of
// recording order, and byte-stable JSON.
func TestSortedOutput(t *testing.T) {
	s := readmanifest.NewSet()
	s.Record("zeta", readmanifest.Read(1))
	s.Record("alpha", readmanifest.Absent())
	s.Skip("mid", "binary")
	m := s.Manifest()
	want := []readmanifest.Entry{
		{Path: "alpha", Outcome: readmanifest.Absent()},
		{Path: "mid", Outcome: readmanifest.Skipped("binary")},
		{Path: "zeta", Outcome: readmanifest.Read(1)},
	}
	if !reflect.DeepEqual(m.Entries, want) {
		t.Fatalf("Entries = %+v, want %+v", m.Entries, want)
	}
	if again := s.Manifest(); !reflect.DeepEqual(m, again) {
		t.Fatal("Manifest is not deterministic")
	}
	b, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	const golden = `{"entries":[{"path":"alpha","outcome":{"kind":"absent"}},{"path":"mid","outcome":{"kind":"skipped","detail":"binary"}},{"path":"zeta","outcome":{"kind":"read","bytes":1}}]}`
	if string(b) != golden {
		t.Fatalf("JSON drifted from the schema fixtures:\n got %s\nwant %s", b, golden)
	}
}

// TestReadFileClassifies pins the three facts a bare `continue` collapses:
// a readable file, a missing file, and a file that exists but cannot be
// read are three different outcomes, and only the last is blindness.
func TestReadFileClassifies(t *testing.T) {
	dir := t.TempDir()
	good := filepath.Join(dir, "good.md")
	if err := os.WriteFile(good, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	s := readmanifest.NewSet()
	s.RelativeTo(dir)
	if b, ok := s.ReadFile(good); !ok || string(b) != "hello" {
		t.Fatalf("readable file: ok=%v body=%q", ok, b)
	}
	if _, ok := s.ReadFile(filepath.Join(dir, "nope.md")); ok {
		t.Fatal("a missing file is not a successful read")
	}
	m := s.Manifest()
	if !m.Complete() {
		t.Fatalf("absent is a fact, not blindness: %+v", m)
	}
	if m.Entries[0].Path != "good.md" || m.Entries[1].Path != "nope.md" {
		t.Fatalf("paths must be root-relative, got %+v", m.Entries)
	}
	if m.Entries[0].Outcome != readmanifest.Read(5) || m.Entries[1].Outcome != readmanifest.Absent() {
		t.Fatalf("outcomes = %+v", m.Entries)
	}
	// A path outside the root is rendered verbatim, never as a ../ chain.
	s.Record(filepath.Join(filepath.Dir(dir), "elsewhere.md"), readmanifest.Absent())
	if got := s.Manifest().Entries[0].Path; got != filepath.Join(filepath.Dir(dir), "elsewhere.md") {
		t.Fatalf("outside-root path must be verbatim, got %q", got)
	}

	if os.Geteuid() == 0 {
		t.Skip("running as root: permission bits cannot make a file unreadable")
	}
	bad := filepath.Join(dir, "bad.md")
	if err := os.WriteFile(bad, []byte("secret"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(bad, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(bad, 0o644) })
	if _, ok := s.ReadFile(bad); ok {
		t.Fatal("an unreadable file is not a successful read")
	}
	m = s.Manifest()
	if m.Complete() {
		t.Fatal("a failed read must make the manifest incomplete")
	}
	if got := m.Unreadable(); len(got) != 1 || got[0].Path != "bad.md" || got[0].Outcome.Detail != "permission denied" {
		t.Fatalf("unreadable entry must name the path and the OS wording, got %+v", got)
	}
}

// TestUnreadableStripsPathError pins that the errno text is the OS's own
// wording with the *fs.PathError wrapper removed, and that a nil error is
// still recorded as blindness.
func TestUnreadableStripsPathError(t *testing.T) {
	pe := &os.PathError{Op: "open", Path: "/x/y", Err: errors.New("operation not permitted")}
	if got := readmanifest.Unreadable(pe).Detail; got != "operation not permitted" {
		t.Fatalf("Detail = %q", got)
	}
	if got := readmanifest.Unreadable(nil).Detail; got != "unknown error" {
		t.Fatalf("nil err must still be blindness, got %q", got)
	}
	if got := readmanifest.Skipped("").Detail; got != "unspecified" {
		t.Fatalf("empty category must still account for the path, got %q", got)
	}
}

// TestMergeAccumulatesBlindness pins that a report assembled from several
// readers is blind if any of them was.
func TestMergeAccumulatesBlindness(t *testing.T) {
	sub := readmanifest.NewSet()
	sub.Record("sub/locked.md", readmanifest.Unreadable(errors.New("permission denied")))
	top := readmanifest.NewSet()
	top.Record("top.md", readmanifest.Read(1))
	top.Merge(sub.Manifest())
	m := top.Manifest()
	if m.Complete() {
		t.Fatal("merging a blind sub-reader must make the whole report blind")
	}
	if len(m.Entries) != 2 || m.Entries[0].Path != "sub/locked.md" {
		t.Fatalf("merged entries = %+v", m.Entries)
	}
	// Merge is first-outcome-wins too.
	top.Merge(readmanifest.Manifest{Entries: []readmanifest.Entry{{Path: "top.md", Outcome: readmanifest.Absent()}}})
	if got := top.Manifest().Entries[1].Outcome; got != readmanifest.Read(1) {
		t.Fatalf("merge must not overwrite, got %+v", got)
	}
}

// TestConcurrentRecord pins the concurrency contract under -race.
func TestConcurrentRecord(t *testing.T) {
	s := readmanifest.NewSet()
	s.RelativeTo("/root")
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			for j := 0; j < 50; j++ {
				p := filepath.Join("/root", "f", string(rune('a'+i)), string(rune('a'+j%26)))
				if j%7 == 0 {
					s.Record(p, readmanifest.Unreadable(errors.New("permission denied")))
				} else {
					s.Record(p, readmanifest.Read(j))
				}
				_ = s.Manifest()
			}
		}(i)
	}
	wg.Wait()
	m := s.Manifest()
	if len(m.Entries) != 8*26 {
		t.Fatalf("entries = %d, want %d", len(m.Entries), 8*26)
	}
	if m.Complete() {
		t.Fatal("concurrent failures must still be recorded")
	}
}
