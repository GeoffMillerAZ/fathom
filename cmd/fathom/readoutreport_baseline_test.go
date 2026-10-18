package main

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const roBaselineFixture = "testdata/readoutbaseline"

func TestReadoutReportBaselineCountsTouchedWithinThreeCalls(t *testing.T) {
	rep := readoutBaselineReport(filepath.Join(roBaselineFixture, "hooks.jsonl"), filepath.Join(roBaselineFixture, "projects"), time.Time{})
	if rep.Rows != 6 || rep.Located != 3 || rep.Touched != 2 {
		t.Fatalf("rows/located/touched = %d/%d/%d, want 6/3/2: %+v", rep.Rows, rep.Located, rep.Touched, rep)
	}
	nc := rep.NotComputable
	if nc.Total != 3 || nc.TranscriptGone != 1 || nc.IDAbsent != 1 || nc.NamesNothing != 1 {
		t.Errorf("not computable = %+v, want 3 (gone 1, id absent 1, names nothing 1)", nc)
	}
	if rep.Share != 0.6667 || rep.Lower <= 0.2 || rep.Lower >= rep.Share || rep.Upper <= rep.Share || rep.Upper > 1 {
		t.Errorf("share %v interval [%v, %v]", rep.Share, rep.Lower, rep.Upper)
	}
	if rep.Window.From != "2026-10-01T10:00:00Z" || rep.Window.To != "2026-10-03T14:00:00Z" {
		t.Errorf("window = %+v", rep.Window)
	}
}

func TestReadoutReportBaselineSinceBoundsRows(t *testing.T) {
	since := time.Date(2026, 10, 2, 10, 30, 0, 0, time.UTC)
	rep := readoutBaselineReport(filepath.Join(roBaselineFixture, "hooks.jsonl"), filepath.Join(roBaselineFixture, "projects"), since)
	if rep.Rows != 4 || rep.Located != 1 || rep.Touched != 1 {
		t.Errorf("rows/located/touched = %d/%d/%d, want 4/1/1", rep.Rows, rep.Located, rep.Touched)
	}
}

func TestCmdReadoutReportBaselineFlag(t *testing.T) {
	t.Setenv("FATHOM_HOOKS_LOG", filepath.Join(roBaselineFixture, "hooks.jsonl"))
	oldRoot := readoutProjectsDir
	readoutProjectsDir = func() string { return filepath.Join(roBaselineFixture, "projects") }
	t.Cleanup(func() { readoutProjectsDir = oldRoot })

	var out, errb bytes.Buffer
	if code := cmdReadoutReport([]string{"--baseline", "--json"}, &out, &errb); code != 0 {
		t.Fatalf("exit %d: %s", code, errb.String())
	}
	var rep roBaseline
	if err := json.Unmarshal(out.Bytes(), &rep); err != nil || rep.Rows != 6 || rep.Touched != 2 {
		t.Fatalf("json = %v, %+v", err, rep)
	}
	for _, leak := range []string{"sess-", "toolu_", "/work", "go test", "alpha", "TestBeta"} {
		if strings.Contains(out.String(), leak) {
			t.Errorf("output leaks %q", leak)
		}
	}
	out.Reset()
	if code := cmdReadoutReport([]string{"--baseline"}, &out, &errb); code != 0 || !strings.Contains(out.String(), "touched 2 of 3 located") {
		t.Errorf("text form (exit %d): %s", code, out.String())
	}
}

// TestTestRunRowCarriesToolUseIDAndBaselinePrefersIt pins O1210's fix: the
// test-run hook's row carries the input's tool_use_id on both events, and
// the baseline locates a row that has one by it exactly, falls back to the
// session, invocation, scope and time match only for a row without one,
// and counts the rows located each way.
func TestTestRunRowCarriesToolUseIDAndBaselinePrefersIt(t *testing.T) {
	srv, _ := countingDaemon(t)
	const tailFail = "--- FAIL: TestX (0.00s)\n    x_test.go:12: boom\nFAIL\nFAIL\tgithub.com/o/r/x\t0.2s\n"
	for _, c := range []struct{ event, wantID string }{{"test-run", "t1"}, {"failed-test-run", "t2"}} {
		repo := testRunRepo(t, true)
		payload := testRunPostPayload(repo, "go test ./x 2>&1 | tail -30", tailFail)
		if c.event == "failed-test-run" {
			payload = testRunFailurePayload(repo, "go test ./x", tailFail)
		}
		runTestRunHook(t, c.event, repo, srv.URL, payload)
		rows := testRunBeats(t)
		if len(rows) != 1 || rows[0]["tool_use_id"] != c.wantID {
			t.Errorf("%s: rows = %v, want one row with tool_use_id %q", c.event, rows, c.wantID)
		}
	}

	rep := readoutBaselineReport(filepath.Join(roBaselineFixture, "hooks.jsonl"), filepath.Join(roBaselineFixture, "projects"), time.Time{})
	// sess-run is located by its id; sess-none and sess-edit carry none and
	// are matched by time; sess-gone has an id its transcript lacks and is
	// never matched by time instead.
	if rep.LocatedBy.ToolUseID != 1 || rep.LocatedBy.Time != 2 || rep.Located != 3 || rep.NotComputable.TranscriptGone != 1 {
		t.Errorf("located by = %+v (located %d, gone %d), want 1 by tool-use id, 2 by time, 1 gone", rep.LocatedBy, rep.Located, rep.NotComputable.TranscriptGone)
	}
	b, err := json.Marshal(rep)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(b, []byte(`"located_by":{"tool_use_id":1,"time":2}`)) {
		t.Errorf("json lacks the located_by counts: %s", b)
	}
	var out bytes.Buffer
	printReadoutBaseline(&out, rep)
	if !strings.Contains(out.String(), "located 3 (by tool-use id 1, by time 2)") {
		t.Errorf("text form lacks the split:\n%s", out.String())
	}
}
