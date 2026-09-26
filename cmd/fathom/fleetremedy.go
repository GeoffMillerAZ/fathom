package main

// fleetremedy.go — SH-9 (docs/plans/ambient-fathom.md; docs/design/
// self-healing-fleet.md §7): time-to-remedy, the instrument phase U was
// pre-registered against. For each repository, the interval between a
// wiring gap first appearing in a fleet report snapshot and the first later
// snapshot in which it is closed.
//
// The snapshots are the dated reports `fathom fleet report` writes
// (~/.fathom/fleet/report-<date>.json), graded with wiringGrade — the grade
// the fleet page shows — so a gap is exactly what a reader of that page
// would have seen: a grade below A. The reading is honest about its
// resolution: a gap already present in the first snapshot opened on or
// before it (the interval is a lower bound), a gap still open at the latest
// snapshot has no time-to-remedy yet, and a snapshot that could not read
// the call log grades that repository unknown rather than unused.
//
// `fathom fleet remedy [--dir D] [--json]` prints it; the console's Operate
// page renders it (console_operate.go).

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// remedyBaseline is the pre-registered 2026-08-08 measurement (design §7):
// 9 of 17 repositories below grade A, the remedy printed continuously since
// at least 2026-07-30, time-to-remedy unbounded for all 9.
type remedyBaseline struct {
	Date   string `json:"date"`
	BelowA int    `json:"below_a"`
	Of     int    `json:"of"`
	Note   string `json:"note"`
}

var remedyBaselineRecorded = remedyBaseline{
	Date: "2026-08-08", BelowA: 9, Of: 17,
	Note: "measured before phase U shipped: 9 of 17 repositories below grade A, the remedy printed continuously since at least 2026-07-30 — time-to-remedy unbounded for all 9 (docs/design/self-healing-fleet.md §7). Success: median under one week and more than 14 of 17 at A within 30 days of shipping.",
}

// remedySnapshot is one dated fleet report.
type remedySnapshot struct {
	At   time.Time
	Path string
	Rows []remedySnapshotRow
}

// remedySnapshotRow is a snapshot's service row; CallsTotal is nil when
// that report could not read the call log (the grade is then unknown).
type remedySnapshotRow struct {
	fleetServiceRow
	CallsTotal *int `json:"calls_total"`
}

// remedyEpisode is one wiring gap: from the first snapshot grading the
// repository below A to the first later snapshot grading it A again.
type remedyEpisode struct {
	OpenedAt string `json:"opened_at"`
	// OpenedOnOrBefore: the gap is in the first snapshot, so it opened then
	// or earlier and Hours is a lower bound.
	OpenedOnOrBefore bool     `json:"opened_on_or_before,omitempty"`
	ClosedAt         string   `json:"closed_at,omitempty"`
	Hours            *float64 `json:"hours_to_remedy,omitempty"`
	// OpenHours is how long a still-open gap had been open at the latest
	// snapshot (a lower bound on its time-to-remedy).
	OpenHours *float64 `json:"open_hours,omitempty"`
	Grades    []string `json:"grades"`
	Remedy    string   `json:"remedy,omitempty"`
}

// remedyRepo is one repository's gaps across the snapshots.
type remedyRepo struct {
	Service string `json:"service"`
	// Latest is the grade at the latest snapshot that graded it ("" when
	// none could).
	Latest   string          `json:"latest_grade"`
	Episodes []remedyEpisode `json:"episodes"`
}

// remedyReport is `fathom fleet remedy --json`.
type remedyReport struct {
	SchemaVersion int            `json:"schema_version"`
	SnapshotDir   string         `json:"snapshot_dir"`
	Snapshots     int            `json:"snapshots"`
	First         string         `json:"first_snapshot,omitempty"`
	Latest        string         `json:"latest_snapshot,omitempty"`
	Repos         []remedyRepo   `json:"repos"`
	Closed        int            `json:"closed_gaps"`
	Open          int            `json:"open_gaps"`
	MedianHours   *float64       `json:"median_hours_to_remedy,omitempty"`
	BelowA        int            `json:"below_a_latest"`
	Graded        int            `json:"graded_latest"`
	Baseline      remedyBaseline `json:"baseline"`
	Note          string         `json:"note"`
	Skipped       []string       `json:"skipped_snapshots,omitempty"`
}

const remedySchemaVersion = 1

// snapshotGrade is wiringGrade over a snapshot row; ok is false when the
// snapshot could not read the call log (the traffic signal is unknown, so
// no grade is claimed).
func snapshotGrade(r remedySnapshotRow) (grade, raise string, ok bool) {
	if r.CallsTotal == nil {
		return "", "", false
	}
	row := obsServiceRow{fleetServiceRow: r.fleetServiceRow}
	row.CallsTotal = *r.CallsTotal
	grade, raise = wiringGrade(row)
	return grade, raise, true
}

// timeToRemedy folds dated snapshots (any order) into per-repository gap
// episodes and the fleet's median time-to-remedy over closed gaps.
func timeToRemedy(snaps []remedySnapshot) remedyReport {
	rep := remedyReport{SchemaVersion: remedySchemaVersion, Baseline: remedyBaselineRecorded, Snapshots: len(snaps)}
	sort.SliceStable(snaps, func(i, j int) bool { return snaps[i].At.Before(snaps[j].At) })
	if len(snaps) == 0 {
		rep.Note = "no fleet report snapshot — time-to-remedy is not measured (`fathom fleet report` writes one per day)"
		return rep
	}
	rep.First = snaps[0].At.UTC().Format(time.RFC3339)
	rep.Latest = snaps[len(snaps)-1].At.UTC().Format(time.RFC3339)

	type obs struct {
		at           time.Time
		grade, raise string
	}
	perRepo := map[string][]obs{}
	firstSeen := map[string]time.Time{}
	for _, s := range snaps {
		for _, r := range s.Rows {
			g, raise, ok := snapshotGrade(r)
			if !ok || r.Service == "" {
				continue
			}
			if _, seen := firstSeen[r.Service]; !seen {
				firstSeen[r.Service] = s.At
			}
			perRepo[r.Service] = append(perRepo[r.Service], obs{at: s.At, grade: g, raise: raise})
		}
	}
	names := make([]string, 0, len(perRepo))
	for n := range perRepo {
		names = append(names, n)
	}
	sort.Strings(names)
	latest := snaps[len(snaps)-1].At
	var closed []float64
	for _, name := range names {
		repo := remedyRepo{Service: name}
		var cur *remedyEpisode
		for i, o := range perRepo[name] {
			switch {
			case o.grade != "A" && cur == nil:
				cur = &remedyEpisode{OpenedAt: o.at.UTC().Format(time.RFC3339), OpenedOnOrBefore: i == 0 && o.at.Equal(snaps[0].At)}
				cur.Grades = append(cur.Grades, o.grade)
				cur.Remedy = o.raise
			case o.grade != "A":
				if cur.Grades[len(cur.Grades)-1] != o.grade {
					cur.Grades = append(cur.Grades, o.grade)
				}
				cur.Remedy = o.raise
			case cur != nil:
				opened, _ := time.Parse(time.RFC3339, cur.OpenedAt)
				h := roundHours(o.at.Sub(opened))
				cur.ClosedAt, cur.Hours = o.at.UTC().Format(time.RFC3339), &h
				repo.Episodes = append(repo.Episodes, *cur)
				closed = append(closed, h)
				rep.Closed++
				cur = nil
			}
			repo.Latest = o.grade
		}
		if cur != nil {
			opened, _ := time.Parse(time.RFC3339, cur.OpenedAt)
			h := roundHours(latest.Sub(opened))
			cur.OpenHours = &h
			repo.Episodes = append(repo.Episodes, *cur)
			rep.Open++
		}
		if repo.Latest != "" {
			rep.Graded++
			if repo.Latest != "A" {
				rep.BelowA++
			}
		}
		rep.Repos = append(rep.Repos, repo)
	}
	if len(closed) > 0 {
		sort.Float64s(closed)
		med := closed[len(closed)/2]
		rep.MedianHours = &med
	}
	switch {
	case rep.Closed == 0 && rep.Open > 0:
		rep.Note = fmt.Sprintf("no wiring gap has closed across %d snapshot(s) — time-to-remedy is unbounded for all %d open gap(s)", len(snaps), rep.Open)
	case rep.Closed == 0:
		rep.Note = fmt.Sprintf("no wiring gap appeared across %d snapshot(s)", len(snaps))
	default:
		rep.Note = fmt.Sprintf("%d gap(s) closed, %d open, across %d snapshot(s); a gap in the first snapshot opened on or before it, so its interval is a lower bound", rep.Closed, rep.Open, len(snaps))
	}
	return rep
}

func roundHours(d time.Duration) float64 {
	return float64(int(d.Hours()*10+0.5)) / 10
}

// fleetSnapshotDir is where `fathom fleet report` writes its snapshots.
func fleetSnapshotDir() string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return ""
	}
	return filepath.Join(home, ".fathom", "fleet")
}

// loadFleetSnapshots reads every report-*.json under dir. A missing dir is
// no snapshots; an unreadable or malformed file is skipped and named.
func loadFleetSnapshots(dir string) (snaps []remedySnapshot, skipped []string, err error) {
	paths, gerr := filepath.Glob(filepath.Join(dir, "report-*.json"))
	if gerr != nil {
		return nil, nil, gerr
	}
	sort.Strings(paths)
	for _, p := range paths {
		raw, rerr := os.ReadFile(p)
		if rerr != nil {
			skipped = append(skipped, filepath.Base(p)+": "+rerr.Error())
			continue
		}
		var doc struct {
			GeneratedAt string              `json:"generated_at"`
			Services    []remedySnapshotRow `json:"services"`
		}
		if jerr := json.Unmarshal(raw, &doc); jerr != nil {
			skipped = append(skipped, filepath.Base(p)+": "+jerr.Error())
			continue
		}
		at, terr := time.Parse(time.RFC3339, doc.GeneratedAt)
		if terr != nil {
			skipped = append(skipped, filepath.Base(p)+": no generated_at")
			continue
		}
		snaps = append(snaps, remedySnapshot{At: at, Path: p, Rows: doc.Services})
	}
	return snaps, skipped, nil
}

// buildRemedyReport is `fathom fleet remedy --json`'s whole computation;
// the Operate page calls it for its section.
func buildRemedyReport(dir string) remedyReport {
	snaps, skipped, err := loadFleetSnapshots(dir)
	rep := timeToRemedy(snaps)
	rep.SnapshotDir, rep.Skipped = dir, skipped
	if err != nil {
		rep.Note = "snapshot directory unreadable: " + err.Error()
	}
	return rep
}

func cmdFleetRemedy(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("fleet remedy", stderr)
	dir := fs.String("dir", fleetSnapshotDir(), "the fleet report snapshot directory (`fathom fleet report` writes report-<date>.json there)")
	jsonOut := fs.Bool("json", false, "emit JSON")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	rep := buildRemedyReport(*dir)
	if *jsonOut {
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(rep)
		return 0
	}
	fmt.Fprintf(stdout, "fleet time-to-remedy — %d snapshot(s) in %s\n", rep.Snapshots, rep.SnapshotDir)
	if rep.MedianHours != nil {
		fmt.Fprintf(stdout, "median time-to-remedy: %.1f h over %d closed gap(s); %d still open\n", *rep.MedianHours, rep.Closed, rep.Open)
	}
	fmt.Fprintln(stdout, rep.Note)
	for _, r := range rep.Repos {
		for _, e := range r.Episodes {
			span := "open"
			if e.Hours != nil {
				span = fmt.Sprintf("closed after %.1f h", *e.Hours)
			} else if e.OpenHours != nil {
				span = fmt.Sprintf("open %.1f h at the latest snapshot", *e.OpenHours)
			}
			on := ""
			if e.OpenedOnOrBefore {
				on = "on or before "
			}
			fmt.Fprintf(stdout, "  %-24s grade %s from %s%s — %s\n", r.Service, strings.Join(e.Grades, "→"), on, e.OpenedAt[:10], span)
		}
	}
	fmt.Fprintf(stdout, "baseline %s: %d of %d below A, unbounded\n", rep.Baseline.Date, rep.Baseline.BelowA, rep.Baseline.Of)
	return 0
}
