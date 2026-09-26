package main

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

// TestTimeToRemedy pins SH-9: a wiring gap is a snapshot grading a
// repository below A (wiringGrade, the grade the fleet page shows); its
// time-to-remedy runs from the first snapshot showing it to the first later
// snapshot grading the repository A again. A gap already in the first
// snapshot opened on or before it; a gap still open at the latest snapshot
// has no time-to-remedy, only how long it has been open; a snapshot that
// could not read the call log grades that repository unknown, never unused.
// The median is over closed gaps, and the 2026-08-08 baseline is recorded.
func TestTimeToRemedy(t *testing.T) {
	dir := t.TempDir()
	calls := func(n int) string { return `"calls_total":` + itoaTest(n) + `,` }
	row := func(svc, hooks, stanza, fresh, callsField string) string {
		return `{"service":"` + svc + `","path":"/r/` + svc + `",` + callsField + `"hooks_note":"` + hooks + `","stanza_note":"` + stanza + `","freshness":"` + fresh + `"}`
	}
	wired := func(svc string, n int) string {
		return row(svc, "3 fathom hook reference(s) in project settings", "present", "fresh", calls(n))
	}
	write := func(date, at string, rows ...string) {
		mustWrite(t, filepath.Join(dir, "report-"+date+".json"), `{"schema_version":2,"generated_at":"`+at+`","services":[`+strings.Join(rows, ",")+`]}`)
	}
	// alpha: wired throughout. beta: missing its stanza in the first
	// snapshot (a lower bound), fixed by the second. gamma: goes stale in the
	// second snapshot and is still stale at the last. delta: its call log
	// was unreadable in the middle snapshot (unknown, not a gap).
	write("2026-08-08", "2026-08-08T12:00:00Z",
		wired("alpha", 5),
		row("beta", "3 fathom hook reference(s) in project settings", "CLAUDE.md present, no fathom stanza", "fresh", calls(2)),
		wired("gamma", 9),
		wired("delta", 1))
	write("2026-08-10", "2026-08-10T00:00:00Z",
		wired("alpha", 6), wired("beta", 3),
		row("gamma", "3 fathom hook reference(s) in project settings", "present", "stale: indexed a vs HEAD b", calls(9)),
		row("delta", "3 fathom hook reference(s) in project settings", "present", "fresh", ""))
	write("2026-08-14", "2026-08-14T12:00:00Z",
		wired("alpha", 7), wired("beta", 4),
		row("gamma", "3 fathom hook reference(s) in project settings", "present", "stale: indexed a vs HEAD c", calls(9)),
		wired("delta", 2))
	mustWrite(t, filepath.Join(dir, "report-broken.json"), "{not json")

	rep := buildRemedyReport(dir)
	if rep.Snapshots != 3 || len(rep.Skipped) != 1 || !strings.Contains(rep.Skipped[0], "report-broken.json") {
		t.Fatalf("snapshots = %d, skipped %v; want 3 read and the broken one named", rep.Snapshots, rep.Skipped)
	}
	repos := map[string]remedyRepo{}
	for _, r := range rep.Repos {
		repos[r.Service] = r
	}
	if len(repos["alpha"].Episodes) != 0 || repos["alpha"].Latest != "A" {
		t.Errorf("alpha never had a gap: %+v", repos["alpha"])
	}
	beta := repos["beta"].Episodes
	if len(beta) != 1 || !beta[0].OpenedOnOrBefore || beta[0].ClosedAt != "2026-08-10T00:00:00Z" || beta[0].Hours == nil || *beta[0].Hours != 36 {
		t.Errorf("beta = %+v, want one gap opened on or before 08-08 12:00, closed 36 h later", beta)
	}
	if beta[0].Grades[0] != "B" || !strings.Contains(beta[0].Remedy, "fleet reconcile") {
		t.Errorf("beta's gap names its grade and the remedy printed: %+v", beta[0])
	}
	gamma := repos["gamma"].Episodes
	if len(gamma) != 1 || gamma[0].OpenedOnOrBefore || gamma[0].ClosedAt != "" || gamma[0].Hours != nil || gamma[0].OpenHours == nil || *gamma[0].OpenHours != 108 {
		t.Errorf("gamma = %+v, want an open gap from 08-10, 108 h open at the latest snapshot", gamma)
	}
	if len(repos["delta"].Episodes) != 0 {
		t.Errorf("an unreadable call log grades unknown, never a gap: %+v", repos["delta"])
	}
	if rep.Closed != 1 || rep.Open != 1 || rep.MedianHours == nil || *rep.MedianHours != 36 || rep.BelowA != 1 || rep.Graded != 4 {
		t.Errorf("report = closed %d open %d median %v below-A %d of %d", rep.Closed, rep.Open, rep.MedianHours, rep.BelowA, rep.Graded)
	}
	if rep.Baseline.Date != "2026-08-08" || rep.Baseline.BelowA != 9 || rep.Baseline.Of != 17 || !strings.Contains(rep.Baseline.Note, "unbounded") {
		t.Errorf("baseline = %+v", rep.Baseline)
	}
	// No snapshots: not measured, said so.
	if none := buildRemedyReport(t.TempDir()); none.Snapshots != 0 || none.MedianHours != nil || !strings.Contains(none.Note, "not measured") {
		t.Errorf("no snapshots = %+v", none)
	}
	// The verb prints the same report.
	var out, errb bytes.Buffer
	if code := run([]string{"fleet", "remedy", "--dir", dir, "--json"}, &out, &errb); code != 0 {
		t.Fatalf("fleet remedy: %d %s", code, errb.String())
	}
	var fromVerb remedyReport
	if err := json.Unmarshal(out.Bytes(), &fromVerb); err != nil || fromVerb.Closed != 1 || fromVerb.Open != 1 {
		t.Errorf("verb JSON = %s (%v)", out.String(), err)
	}
	out.Reset()
	if code := run([]string{"fleet", "remedy", "--dir", dir}, &out, &errb); code != 0 || !strings.Contains(out.String(), "median time-to-remedy: 36.0 h") || !strings.Contains(out.String(), "baseline 2026-08-08: 9 of 17 below A") {
		t.Errorf("verb text = %q", out.String())
	}
}

func itoaTest(n int) string {
	b, _ := json.Marshal(n)
	return string(b)
}
