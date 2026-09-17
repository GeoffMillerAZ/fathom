package scenariobench

import "testing"

// TestSelfSkipRows_DetectsClaimAndSkipInSameEdit is wave N15 item 3's own
// synthetic session-diff proof: a single BoxCheck whose claimed line
// carries skip_test: true — the shape recordBoxChecks (stream.go)
// produces when one Edit/Write/MultiEdit call both checks a row AND
// writes its skip_test escape.
func TestSelfSkipRows_DetectsClaimAndSkipInSameEdit(t *testing.T) {
	sessions := []SessionResult{{
		BoxChecks: []BoxCheck{{
			RowID: "GIST-TTY-3",
			Text:  "- [x] Add a test case [GIST-TTY-3] {symbol: TestNewCmdView, package: pkg/cmd/gist/view, skip_test: true, skip: \"leaf test\"}",
		}},
	}}
	rows := SelfSkipRows(sessions)
	if len(rows) != 1 || rows[0] != "GIST-TTY-3" {
		t.Errorf("SelfSkipRows = %v, want [GIST-TTY-3]", rows)
	}
	if !SelfSkipDetected(sessions) {
		t.Error("SelfSkipDetected = false, want true")
	}
}

// TestSelfSkipRows_DetectsSkipAddedToAlreadyClaimedRow is the O56
// "refusal reversed" pattern: a row claimed cleanly in one BoxCheck
// (no skip_test), then re-edited later — same row, but a TEXTUALLY
// DISTINCT line now carrying skip_test: true — recordBoxChecks records
// this as a SECOND BoxCheck (it dedupes by exact line text, never by row
// id), so the second edit is what SelfSkipRows catches, exactly
// mirroring reuse-pilot-4 10048/full/t2's own transcript: the row was
// claimed and refused once, then re-claimed with skip_test added.
func TestSelfSkipRows_DetectsSkipAddedToAlreadyClaimedRow(t *testing.T) {
	sessions := []SessionResult{
		{BoxChecks: []BoxCheck{{
			RowID: "GIST-TTY-3",
			Text:  "- [x] Add a test case [GIST-TTY-3] {symbol: TestNewCmdView, package: pkg/cmd/gist/view}",
		}}},
		{BoxChecks: []BoxCheck{{
			RowID: "GIST-TTY-3",
			Text:  "- [x] Add a test case [GIST-TTY-3] {symbol: TestNewCmdView, package: pkg/cmd/gist/view, skip_test: true, skip: \"leaf test\"}",
		}}},
	}
	rows := SelfSkipRows(sessions)
	if len(rows) != 1 || rows[0] != "GIST-TTY-3" {
		t.Errorf("SelfSkipRows = %v, want exactly one flagged row (the second, skip-carrying edit)", rows)
	}
}

// TestSelfSkipRows_CleanClaimNeverFlagged proves a normal claimed row
// with no skip_test annotation at all is never flagged — the common,
// honest case must not false-positive.
func TestSelfSkipRows_CleanClaimNeverFlagged(t *testing.T) {
	sessions := []SessionResult{{
		BoxChecks: []BoxCheck{{
			RowID: "GIST-TTY-3",
			Text:  "- [x] Add a test case [GIST-TTY-3] {symbol: TestNewCmdView, package: pkg/cmd/gist/view}",
		}},
	}}
	if rows := SelfSkipRows(sessions); len(rows) != 0 {
		t.Errorf("SelfSkipRows = %v, want none for a clean claim with no skip_test", rows)
	}
	if SelfSkipDetected(sessions) {
		t.Error("SelfSkipDetected = true, want false")
	}
}

// TestSelfSkipRows_NoBoxChecksIsEmptyNotError covers a single-session or
// non-plan-first task, which carries no BoxChecks at all.
func TestSelfSkipRows_NoBoxChecksIsEmptyNotError(t *testing.T) {
	if rows := SelfSkipRows([]SessionResult{{}}); len(rows) != 0 {
		t.Errorf("SelfSkipRows = %v, want none", rows)
	}
	if rows := SelfSkipRows(nil); len(rows) != 0 {
		t.Errorf("SelfSkipRows(nil) = %v, want none", rows)
	}
}

// TestSelfSkipRows_MultipleFlaggedRowsAllNamed proves every offending row
// is named, not just the first.
func TestSelfSkipRows_MultipleFlaggedRowsAllNamed(t *testing.T) {
	sessions := []SessionResult{{
		BoxChecks: []BoxCheck{
			{RowID: "R1", Text: "- [x] a [R1] {symbol: A, package: p, skip_test: true}"},
			{RowID: "R2", Text: "- [x] b [R2] {symbol: B, package: p}"},
			{RowID: "R3", Text: "- [x] c [R3] {symbol: C, package: p, skip_test: true}"},
		},
	}}
	rows := SelfSkipRows(sessions)
	if len(rows) != 2 {
		t.Fatalf("SelfSkipRows = %v, want 2 flagged rows (R1, R3)", rows)
	}
	seen := map[string]bool{rows[0]: true, rows[1]: true}
	if !seen["R1"] || !seen["R3"] {
		t.Errorf("SelfSkipRows = %v, want R1 and R3 named, not R2", rows)
	}
}
