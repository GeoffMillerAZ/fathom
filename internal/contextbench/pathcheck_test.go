package contextbench

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// TestDetectPathLeak_FindsServedLiveCheckoutPath pins the O47 shape: a
// served tool_result payload citing the live checkout's absolute path
// (not the export the session ran against) marks the cell leaked and
// names the exact path served. sessionRoot is unrelated to liveCheckout
// here (the "new layout" shape, scripts/context-bench.sh's current
// `${TMPDIR:-/tmp}/fathom-context-bench/`), so it never suppresses the hit.
func TestDetectPathLeak_FindsServedLiveCheckoutPath(t *testing.T) {
	liveCheckout := "/srv/acme/repo"
	sessionRoot := "/tmp/fathom-context-bench/abc123"
	tr := []byte(
		`{"type":"assistant","message":{"content":[{"type":"tool_use","id":"1","name":"mcp__fathom__read_source","input":{"path":"CLAUDE.md"}}],"usage":{}}}` + "\n" +
			`{"type":"user","message":{"content":[{"type":"tool_result","tool_use_id":"1","content":"served from /srv/acme/repo/CLAUDE.md line 15"}]}}` + "\n",
	)
	got := DetectPathLeak(tr, liveCheckout, sessionRoot)
	if !got.Leaked {
		t.Fatalf("expected Leaked=true, got %+v", got)
	}
	want := []string{"/srv/acme/repo/CLAUDE.md"}
	if !reflect.DeepEqual(got.Paths, want) {
		t.Errorf("Paths = %v, want %v", got.Paths, want)
	}
}

// TestDetectPathLeak_NoLeakWhenAbsent: a transcript that never serves the
// live checkout's path reports no leak.
func TestDetectPathLeak_NoLeakWhenAbsent(t *testing.T) {
	liveCheckout := "/srv/acme/repo"
	sessionRoot := "/tmp/fathom-context-bench/abc123"
	tr := []byte(
		`{"type":"assistant","message":{"content":[{"type":"tool_use","id":"1","name":"mcp__fathom__read_source","input":{"path":"CLAUDE.md"}}],"usage":{}}}` + "\n" +
			`{"type":"user","message":{"content":[{"type":"tool_result","tool_use_id":"1","content":"served from /tmp/fathom-context-bench/abc123/CLAUDE.md line 15"}]}}` + "\n",
	)
	got := DetectPathLeak(tr, liveCheckout, sessionRoot)
	if got.Leaked || len(got.Paths) != 0 {
		t.Errorf("expected no leak, got %+v", got)
	}
}

// TestDetectPathLeak_EmptyLiveCheckoutNeverLeaks: with no live-checkout
// path to check against, the reading is "not checked", not a false leak —
// and, since the zero value already means "not checked", also not a false
// "clean" for a payload that would otherwise have matched something.
func TestDetectPathLeak_EmptyLiveCheckoutNeverLeaks(t *testing.T) {
	tr := []byte(`{"type":"user","message":{"content":[{"type":"tool_result","tool_use_id":"1","content":"served from /srv/acme/repo/CLAUDE.md"}]}}` + "\n")
	got := DetectPathLeak(tr, "", "/tmp/fathom-context-bench/abc123")
	if got.Leaked || len(got.Paths) != 0 {
		t.Errorf("empty liveCheckout must never report a leak, got %+v", got)
	}
}

// TestDetectPathLeak_NestedLayoutExcludesHonestExportRefs pins the false
// positive a first cut of this check had: when the session root NESTS
// under the live checkout (the layout this package's own first VB-36 pass
// used, `<liveCheckout>/workspace/tmp/bench-subject-<sha>`), a served path
// INSIDE the legitimate export still contains liveCheckout as a string
// prefix — that must not count as a leak. A served path under liveCheckout
// but OUTSIDE the nested session root still must.
func TestDetectPathLeak_NestedLayoutExcludesHonestExportRefs(t *testing.T) {
	liveCheckout := "/srv/acme/repo"
	sessionRoot := "/srv/acme/repo/workspace/tmp/bench-subject-abc123"

	honest := []byte(`{"type":"user","message":{"content":[{"type":"tool_result","tool_use_id":"1","content":"served from /srv/acme/repo/workspace/tmp/bench-subject-abc123/CLAUDE.md line 15"}]}}` + "\n")
	got := DetectPathLeak(honest, liveCheckout, sessionRoot)
	if got.Leaked {
		t.Errorf("a served path INSIDE the nested session root must not be a leak, got %+v", got)
	}

	leaked := []byte(`{"type":"user","message":{"content":[{"type":"tool_result","tool_use_id":"1","content":"served from /srv/acme/repo/CLAUDE.md line 15"}]}}` + "\n")
	got2 := DetectPathLeak(leaked, liveCheckout, sessionRoot)
	if !got2.Leaked {
		t.Fatalf("a served path under the live checkout but OUTSIDE the nested session root must still be a leak, got %+v", got2)
	}
	want := []string{"/srv/acme/repo/CLAUDE.md"}
	if !reflect.DeepEqual(got2.Paths, want) {
		t.Errorf("Paths = %v, want %v", got2.Paths, want)
	}
}

// TestDetectPathLeak_NewLayoutStillCatchesLeak pins the CURRENT layout
// (session root outside the live checkout entirely): a served path naming
// the live checkout is unconditionally a leak — there is no honest reason
// for it to appear, since the export never nests under it.
func TestDetectPathLeak_NewLayoutStillCatchesLeak(t *testing.T) {
	liveCheckout := "/srv/acme/repo"
	sessionRoot := "/tmp/fathom-context-bench/abc123"
	tr := []byte(`{"type":"user","message":{"content":[{"type":"tool_result","tool_use_id":"1","content":"served from /srv/acme/repo/CLAUDE.md"}]}}` + "\n")
	got := DetectPathLeak(tr, liveCheckout, sessionRoot)
	if !got.Leaked {
		t.Fatalf("expected a leak, got %+v", got)
	}
}

// TestDetectPathLeak_SymlinkedSessionRootEquivalence pins finding 6: the
// session root and the live checkout are canonicalized (realpath) before
// comparison, so a symlinked path (macOS's /tmp -> /private/tmp, or a
// symlinked checkout) is not a false miss. Built with a REAL symlink
// under t.TempDir() rather than asserted against /tmp itself, so the test
// does not depend on this machine's own /tmp being a symlink.
func TestDetectPathLeak_SymlinkedSessionRootEquivalence(t *testing.T) {
	real := filepath.Join(t.TempDir(), "real-root")
	if err := os.MkdirAll(real, 0o755); err != nil {
		t.Fatal(err)
	}
	// The file named in the served payload must actually exist for
	// canonicalizePath's EvalSymlinks to resolve it (a served path that
	// exists nowhere on this machine has nothing to canonicalize against
	// — see canonicalizePath's fallback) — exactly the production shape,
	// where the served file is a real one the daemon just read.
	if err := os.WriteFile(filepath.Join(real, "CLAUDE.md"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(t.TempDir(), "link-root")
	if err := os.Symlink(real, link); err != nil {
		t.Skipf("symlink not supported here: %v", err)
	}

	// The runner was TOLD about the session root via its symlinked
	// spelling (link), but the daemon serves the REAL (symlink-resolved)
	// path underneath it — exactly the /tmp vs /private/tmp gap. liveCheckout
	// is `real` itself, so a served path under it is a leak UNLESS the
	// symlinked session root is recognized as covering it too.
	honest := []byte(`{"type":"user","message":{"content":[{"type":"tool_result","tool_use_id":"1","content":"served from ` + real + `/CLAUDE.md"}]}}` + "\n")
	got := DetectPathLeak(honest, real, link)
	if got.Leaked {
		t.Errorf("a path under the REAL directory a SYMLINKED session root points at must not leak, got %+v", got)
	}
}

// TestDetectOutOfRootReads_BashEscapesWithAbsolutePath pins O47's exact
// repro: 'sed -n 1,120p <live checkout>/CLAUDE.md' run via Bash escapes
// --restricted's confinement (which only covers Read/Glob/Grep).
func TestDetectOutOfRootReads_BashEscapesWithAbsolutePath(t *testing.T) {
	sessionRoot := "/tmp/fathom-context-bench/abc123"
	tr := []byte(`{"type":"assistant","message":{"content":[{"type":"tool_use","id":"1","name":"Bash","input":{"command":"sed -n 1,120p /srv/acme/repo/CLAUDE.md"}}],"usage":{}}}` + "\n")
	got := DetectOutOfRootReads(tr, sessionRoot)
	want := []OutOfRootRead{{Tool: "Bash", Path: "/srv/acme/repo/CLAUDE.md"}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

// TestDetectOutOfRootReads_ReadOutsideRoot pins the Read-tool shape: an
// absolute file_path outside sessionRoot is recorded even though
// --restricted is meant to confine Read to the working directory.
func TestDetectOutOfRootReads_ReadOutsideRoot(t *testing.T) {
	sessionRoot := "/tmp/fathom-context-bench/abc123"
	tr := []byte(`{"type":"assistant","message":{"content":[{"type":"tool_use","id":"1","name":"Read","input":{"file_path":"/srv/acme/repo/CLAUDE.md"}}],"usage":{}}}` + "\n")
	got := DetectOutOfRootReads(tr, sessionRoot)
	want := []OutOfRootRead{{Tool: "Read", Path: "/srv/acme/repo/CLAUDE.md"}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

// TestDetectOutOfRootReads_WithinRootIsClean: a Bash command and a Read
// call that both stay under sessionRoot record nothing — confinement
// working as intended is not itself a finding.
func TestDetectOutOfRootReads_WithinRootIsClean(t *testing.T) {
	sessionRoot := "/tmp/fathom-context-bench/abc123"
	tr := []byte(
		`{"type":"assistant","message":{"content":[{"type":"tool_use","id":"1","name":"Bash","input":{"command":"grep -n foo /tmp/fathom-context-bench/abc123/CLAUDE.md"}}],"usage":{}}}` + "\n" +
			`{"type":"assistant","message":{"content":[{"type":"tool_use","id":"2","name":"Read","input":{"file_path":"/tmp/fathom-context-bench/abc123/internal/core/model.go"}}],"usage":{}}}` + "\n" +
			// A relative Read path (the common case) is not absolute and so is
			// never flagged, regardless of sessionRoot.
			`{"type":"assistant","message":{"content":[{"type":"tool_use","id":"3","name":"Read","input":{"file_path":"CLAUDE.md"}}],"usage":{}}}` + "\n",
	)
	got := DetectOutOfRootReads(tr, sessionRoot)
	if len(got) != 0 {
		t.Errorf("expected no out-of-root reads, got %+v", got)
	}
}

// TestDetectOutOfRootReads_EmptySessionRootReportsNothing: with no session
// root to check against, the reading is "not checked", never a false
// "clean" or a false positive.
func TestDetectOutOfRootReads_EmptySessionRootReportsNothing(t *testing.T) {
	tr := []byte(`{"type":"assistant","message":{"content":[{"type":"tool_use","id":"1","name":"Bash","input":{"command":"cat /srv/acme/repo/CLAUDE.md"}}],"usage":{}}}` + "\n")
	if got := DetectOutOfRootReads(tr, ""); got != nil {
		t.Errorf("empty sessionRoot must report nothing, got %+v", got)
	}
}

// TestDetectOutOfRootReads_OtherToolsIgnored: only Bash and Read are
// examined — a Grep/Glob call (already confined by --restricted) is not a
// source of this finding.
func TestDetectOutOfRootReads_OtherToolsIgnored(t *testing.T) {
	sessionRoot := "/tmp/fathom-context-bench/abc123"
	tr := []byte(`{"type":"assistant","message":{"content":[{"type":"tool_use","id":"1","name":"mcp__fathom__read_source","input":{"path":"/srv/acme/repo/CLAUDE.md"}}],"usage":{}}}` + "\n")
	if got := DetectOutOfRootReads(tr, sessionRoot); got != nil {
		t.Errorf("expected no out-of-root reads from a non-Bash/Read tool, got %+v", got)
	}
}

// TestDetectOutOfRootReads_SingleSegmentPath pins finding 3: a
// single-segment absolute path ("/tmp", no second "/"-separated segment)
// used to miss entirely (`cd /tmp && cat notes.txt` produced no finding).
func TestDetectOutOfRootReads_SingleSegmentPath(t *testing.T) {
	sessionRoot := "/tmp/fathom-context-bench/abc123"
	tr := []byte(`{"type":"assistant","message":{"content":[{"type":"tool_use","id":"1","name":"Bash","input":{"command":"cd /tmp && cat notes.txt"}}],"usage":{}}}` + "\n")
	got := DetectOutOfRootReads(tr, sessionRoot)
	want := []OutOfRootRead{{Tool: "Bash", Path: "/tmp"}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

// TestDetectOutOfRootReads_TwoSegmentPath: cat /etc/hosts — a plain
// multi-segment path, alongside the single-segment and lone-root cases
// above/below so all three shapes are pinned together.
func TestDetectOutOfRootReads_TwoSegmentPath(t *testing.T) {
	sessionRoot := "/tmp/fathom-context-bench/abc123"
	tr := []byte(`{"type":"assistant","message":{"content":[{"type":"tool_use","id":"1","name":"Bash","input":{"command":"cat /etc/hosts"}}],"usage":{}}}` + "\n")
	got := DetectOutOfRootReads(tr, sessionRoot)
	want := []OutOfRootRead{{Tool: "Bash", Path: "/etc/hosts"}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

// TestDetectOutOfRootReads_LoneRoot pins the other half of finding 3: the
// lone root "/" (zero segments) must still count as an absolute path
// outside the session root — `ls /` is exactly as much an escape as
// `ls /etc`.
func TestDetectOutOfRootReads_LoneRoot(t *testing.T) {
	sessionRoot := "/tmp/fathom-context-bench/abc123"
	tr := []byte(`{"type":"assistant","message":{"content":[{"type":"tool_use","id":"1","name":"Bash","input":{"command":"ls /"}}],"usage":{}}}` + "\n")
	got := DetectOutOfRootReads(tr, sessionRoot)
	want := []OutOfRootRead{{Tool: "Bash", Path: "/"}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

// TestDetectOutOfRootReads_URLNeverMatchesAsAbsolutePath pins the other
// half of the finding-3 fix: allowing single-segment/lone-root paths must
// not turn "https://host/path" into a false positive by matching the
// "//" (a zero-length segment) as if it were a root.
func TestDetectOutOfRootReads_URLNeverMatchesAsAbsolutePath(t *testing.T) {
	sessionRoot := "/tmp/fathom-context-bench/abc123"
	tr := []byte(`{"type":"assistant","message":{"content":[{"type":"tool_use","id":"1","name":"Bash","input":{"command":"curl https://example.com/foo/bar"}}],"usage":{}}}` + "\n")
	if got := DetectOutOfRootReads(tr, sessionRoot); got != nil {
		t.Errorf("a URL must never be recorded as an absolute path, got %+v", got)
	}
}

// TestDetectOutOfRootReads_TildeExpandsToRealHome pins the tilde nit: a
// `~/.claude/x` read must be recorded with the REAL expansion
// ($HOME/.claude/x), never the tilde-stripped "/.claude/x" the old
// absolute-path regex would have matched (the "/" right after a dropped
// "~" looks exactly like a root-relative path).
func TestDetectOutOfRootReads_TildeExpandsToRealHome(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		t.Skip("no resolvable home directory on this machine")
	}
	sessionRoot := "/tmp/fathom-context-bench/abc123"
	tr := []byte(`{"type":"assistant","message":{"content":[{"type":"tool_use","id":"1","name":"Bash","input":{"command":"cat ~/.claude/x"}}],"usage":{}}}` + "\n")
	got := DetectOutOfRootReads(tr, sessionRoot)
	want := []OutOfRootRead{{Tool: "Bash", Path: filepath.Clean(home) + "/.claude/x"}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v, want %+v (never \"/.claude/x\")", got, want)
	}
	for _, r := range got {
		if r.Path == "/.claude/x" {
			t.Fatalf("tilde was stripped instead of expanded: %+v", got)
		}
	}
}

// TestDetectOutOfRootReads_TildeReadFilePath: the same expansion applies
// to a Read tool's file_path, not just a Bash command string.
func TestDetectOutOfRootReads_TildeReadFilePath(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		t.Skip("no resolvable home directory on this machine")
	}
	sessionRoot := "/tmp/fathom-context-bench/abc123"
	tr := []byte(`{"type":"assistant","message":{"content":[{"type":"tool_use","id":"1","name":"Read","input":{"file_path":"~/.claude/x"}}],"usage":{}}}` + "\n")
	got := DetectOutOfRootReads(tr, sessionRoot)
	want := []OutOfRootRead{{Tool: "Read", Path: filepath.Clean(home) + "/.claude/x"}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

// TestSumOutOfRootReads_and_FlaggedCells pins the ledger-facing aggregates:
// a total count across every cell, and a sorted list naming every cell
// that carries either finding.
func TestSumOutOfRootReads_and_FlaggedCells(t *testing.T) {
	results := []ArmResult{
		{TaskID: "H1-T01", Arm: "full", Trial: 1},
		{TaskID: "H1-T02", Arm: "full", Trial: 1, OutOfRootReads: 2},
		{TaskID: "H1-T03", Arm: "fathom", Trial: 2, PathLeak: true},
		{TaskID: "H1-T04", Arm: "grep", OutOfRootReads: 1, PathLeak: true},
	}
	if got := SumOutOfRootReads(results); got != 3 {
		t.Errorf("SumOutOfRootReads = %d, want 3", got)
	}
	want := []string{"H1-T02/full/t1", "H1-T03/fathom/t2", "H1-T04/grep/t1"}
	if got := FlaggedCells(results); !reflect.DeepEqual(got, want) {
		t.Errorf("FlaggedCells = %v, want %v", got, want)
	}
}
