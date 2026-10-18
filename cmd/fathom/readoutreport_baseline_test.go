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
