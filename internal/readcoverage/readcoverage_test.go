package readcoverage

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	readmanifest "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/telemetry/read-manifest"
)

// unreadableStaging is one way to put a path on disk that a read fails on,
// with the operating system's own words for the failure. The kit's
// Set.ReadFile calls os.ReadFile itself, so these tests stage real OS
// failures rather than inject one. A chmod 000 stops a non-root reader
// only — root's DAC override reads a mode-000 file — so that staging runs
// when euid != 0; a directory where the file should be fails os.ReadFile
// ("is a directory") for every user, root included (CS-8: the cloud VMs
// run the suite as root), and always runs.
type unreadableStaging struct {
	name     string
	wording  string // what Note must carry: the OS's own wording
	needUser bool   // root reads through it
	stage    func(t *testing.T, path string)
	fix      func(t *testing.T, path string) // make path a readable file
}

var unreadableStagings = []unreadableStaging{
	{
		name: "mode 000", wording: "denied", needUser: true,
		stage: func(t *testing.T, path string) {
			if err := os.WriteFile(path, []byte("secret"), 0o644); err != nil {
				t.Fatal(err)
			}
			if err := os.Chmod(path, 0o000); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = os.Chmod(path, 0o644) })
		},
		fix: func(t *testing.T, path string) {
			if err := os.Chmod(path, 0o644); err != nil {
				t.Fatal(err)
			}
		},
	},
	{
		name: "a directory in its place", wording: "is a directory",
		stage: func(t *testing.T, path string) {
			if err := os.Mkdir(path, 0o755); err != nil {
				t.Fatal(err)
			}
		},
		fix: func(t *testing.T, path string) {
			if err := os.Remove(path); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
				t.Fatal(err)
			}
		},
	},
}

// eachUnreadableStaging runs fn once per staging as a named subtest.
func eachUnreadableStaging(t *testing.T, fn func(t *testing.T, st unreadableStaging)) {
	t.Helper()
	for _, st := range unreadableStagings {
		t.Run(st.name, func(t *testing.T) {
			if st.needUser && os.Geteuid() == 0 {
				t.Skip("root reads a mode-000 file; the directory staging covers this check for root")
			}
			fn(t, st)
		})
	}
}

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
	eachUnreadableStaging(t, func(t *testing.T, st unreadableStaging) {
		dir := t.TempDir()
		good := filepath.Join(dir, "good.md")
		if err := os.WriteFile(good, []byte("hello"), 0o644); err != nil {
			t.Fatal(err)
		}
		bad := filepath.Join(dir, "bad.md")
		st.stage(t, bad)

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
		if !strings.Contains(note, "bad.md") || !strings.Contains(note, st.wording) {
			t.Fatalf("note must name path and errno (%q), got %q", st.wording, note)
		}
	})
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

	eachUnreadableStaging(t, func(t *testing.T, st unreadableStaging) {
		bad := filepath.Join(t.TempDir(), "invariants.md")
		st.stage(t, bad)

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
	})
}

// A re-read must not upgrade a manifest to complete: first outcome wins.
func TestFirstOutcomeWinsSoBlindnessCannotBeUndone(t *testing.T) {
	eachUnreadableStaging(t, func(t *testing.T, st unreadableStaging) {
		p := filepath.Join(t.TempDir(), "f.md")
		st.stage(t, p)
		var s readmanifest.Set
		_, _ = s.ReadFile(p)
		st.fix(t, p)
		if _, ok := s.ReadFile(p); !ok {
			t.Fatal("the fixed file must read on its own")
		}
		if s.Manifest().Complete() {
			t.Fatal("a later successful read must not erase a recorded failure")
		}
	})
}

// Merge folds a sub-reader's manifest in, so a report assembled from several
// readers accounts for all of them.
func TestMergeAccumulatesBlindness(t *testing.T) {
	eachUnreadableStaging(t, func(t *testing.T, st unreadableStaging) {
		p := filepath.Join(t.TempDir(), "f.md")
		st.stage(t, p)

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
	})
}
