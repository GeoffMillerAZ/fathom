package contextbench

import (
	"reflect"
	"testing"
)

// TestDetectPathLeak_FindsServedLiveCheckoutPath pins the O47 shape: a
// served tool_result payload citing the live checkout's absolute path
// (not the export the session ran against) marks the cell leaked and
// names the exact path served.
func TestDetectPathLeak_FindsServedLiveCheckoutPath(t *testing.T) {
	liveCheckout := "/Volumes/m2ext/code/geoffmilleraz/sextant"
	tr := []byte(
		`{"type":"assistant","message":{"content":[{"type":"tool_use","id":"1","name":"mcp__fathom__read_source","input":{"path":"CLAUDE.md"}}],"usage":{}}}` + "\n" +
			`{"type":"user","message":{"content":[{"type":"tool_result","tool_use_id":"1","content":"served from /Volumes/m2ext/code/geoffmilleraz/sextant/CLAUDE.md line 15"}]}}` + "\n",
	)
	got := DetectPathLeak(tr, liveCheckout)
	if !got.Leaked {
		t.Fatalf("expected Leaked=true, got %+v", got)
	}
	want := []string{"/Volumes/m2ext/code/geoffmilleraz/sextant/CLAUDE.md"}
	if !reflect.DeepEqual(got.Paths, want) {
		t.Errorf("Paths = %v, want %v", got.Paths, want)
	}
}

// TestDetectPathLeak_NoLeakWhenAbsent: a transcript that never serves the
// live checkout's path reports no leak.
func TestDetectPathLeak_NoLeakWhenAbsent(t *testing.T) {
	liveCheckout := "/Volumes/m2ext/code/geoffmilleraz/sextant"
	tr := []byte(
		`{"type":"assistant","message":{"content":[{"type":"tool_use","id":"1","name":"mcp__fathom__read_source","input":{"path":"CLAUDE.md"}}],"usage":{}}}` + "\n" +
			`{"type":"user","message":{"content":[{"type":"tool_result","tool_use_id":"1","content":"served from /tmp/bench-subject-abc123/CLAUDE.md line 15"}]}}` + "\n",
	)
	got := DetectPathLeak(tr, liveCheckout)
	if got.Leaked || len(got.Paths) != 0 {
		t.Errorf("expected no leak, got %+v", got)
	}
}

// TestDetectPathLeak_EmptyLiveCheckoutNeverLeaks: with no live-checkout
// path to check against, the reading is "not checked", not a false leak —
// and, since the zero value already means "not checked", also not a false
// "clean" for a payload that would otherwise have matched something.
func TestDetectPathLeak_EmptyLiveCheckoutNeverLeaks(t *testing.T) {
	tr := []byte(`{"type":"user","message":{"content":[{"type":"tool_result","tool_use_id":"1","content":"served from /Volumes/m2ext/code/geoffmilleraz/sextant/CLAUDE.md"}]}}` + "\n")
	got := DetectPathLeak(tr, "")
	if got.Leaked || len(got.Paths) != 0 {
		t.Errorf("empty liveCheckout must never report a leak, got %+v", got)
	}
}

// TestDetectOutOfRootReads_BashEscapesWithAbsolutePath pins O47's exact
// repro: 'sed -n 1,120p <live checkout>/CLAUDE.md' run via Bash escapes
// --restricted's confinement (which only covers Read/Glob/Grep).
func TestDetectOutOfRootReads_BashEscapesWithAbsolutePath(t *testing.T) {
	sessionRoot := "/tmp/bench-subject-abc123"
	tr := []byte(`{"type":"assistant","message":{"content":[{"type":"tool_use","id":"1","name":"Bash","input":{"command":"sed -n 1,120p /Volumes/m2ext/code/geoffmilleraz/sextant/CLAUDE.md"}}],"usage":{}}}` + "\n")
	got := DetectOutOfRootReads(tr, sessionRoot)
	want := []OutOfRootRead{{Tool: "Bash", Path: "/Volumes/m2ext/code/geoffmilleraz/sextant/CLAUDE.md"}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

// TestDetectOutOfRootReads_ReadOutsideRoot pins the Read-tool shape: an
// absolute file_path outside sessionRoot is recorded even though
// --restricted is meant to confine Read to the working directory.
func TestDetectOutOfRootReads_ReadOutsideRoot(t *testing.T) {
	sessionRoot := "/tmp/bench-subject-abc123"
	tr := []byte(`{"type":"assistant","message":{"content":[{"type":"tool_use","id":"1","name":"Read","input":{"file_path":"/Volumes/m2ext/code/geoffmilleraz/sextant/CLAUDE.md"}}],"usage":{}}}` + "\n")
	got := DetectOutOfRootReads(tr, sessionRoot)
	want := []OutOfRootRead{{Tool: "Read", Path: "/Volumes/m2ext/code/geoffmilleraz/sextant/CLAUDE.md"}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

// TestDetectOutOfRootReads_WithinRootIsClean: a Bash command and a Read
// call that both stay under sessionRoot record nothing — confinement
// working as intended is not itself a finding.
func TestDetectOutOfRootReads_WithinRootIsClean(t *testing.T) {
	sessionRoot := "/tmp/bench-subject-abc123"
	tr := []byte(
		`{"type":"assistant","message":{"content":[{"type":"tool_use","id":"1","name":"Bash","input":{"command":"grep -n foo /tmp/bench-subject-abc123/CLAUDE.md"}}],"usage":{}}}` + "\n" +
			`{"type":"assistant","message":{"content":[{"type":"tool_use","id":"2","name":"Read","input":{"file_path":"/tmp/bench-subject-abc123/internal/core/model.go"}}],"usage":{}}}` + "\n" +
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
	tr := []byte(`{"type":"assistant","message":{"content":[{"type":"tool_use","id":"1","name":"Bash","input":{"command":"cat /Volumes/m2ext/code/geoffmilleraz/sextant/CLAUDE.md"}}],"usage":{}}}` + "\n")
	if got := DetectOutOfRootReads(tr, ""); got != nil {
		t.Errorf("empty sessionRoot must report nothing, got %+v", got)
	}
}

// TestDetectOutOfRootReads_OtherToolsIgnored: only Bash and Read are
// examined — a Grep/Glob call (already confined by --restricted) is not a
// source of this finding.
func TestDetectOutOfRootReads_OtherToolsIgnored(t *testing.T) {
	sessionRoot := "/tmp/bench-subject-abc123"
	tr := []byte(`{"type":"assistant","message":{"content":[{"type":"tool_use","id":"1","name":"mcp__fathom__read_source","input":{"path":"/Volumes/m2ext/code/geoffmilleraz/sextant/CLAUDE.md"}}],"usage":{}}}` + "\n")
	if got := DetectOutOfRootReads(tr, sessionRoot); got != nil {
		t.Errorf("expected no out-of-root reads from a non-Bash/Read tool, got %+v", got)
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
