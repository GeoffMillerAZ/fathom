package labs

import (
	"fmt"
	"sort"
	"strings"
)

// Stage is a measured funnel stage: #Stage's value, its interval and the
// points it was read over. A stage that cannot be measured is a
// "not measured: <reason>" string instead, never a zero (section 4.6).
type Stage struct {
	Value    float64    `json:"value"`
	Interval [2]float64 `json:"interval"`
	N        int        `json:"n"`
}

// Stages holds the five funnel stages, each a Stage or a named
// "not measured" string.
type Stages struct {
	Eligible  any `json:"eligible"`
	Fired     any `json:"fired"`
	Delivered any `json:"delivered"`
	Acted     any `json:"acted"`
	Helped    any `json:"helped"`
}

// SubstituteCount is one substitute tool and how many eligible points it
// served instead of the surface.
type SubstituteCount struct {
	Tool  string `json:"tool"`
	Count int    `json:"count"`
}

const (
	// notMeasuredActed: AD-21 and RO-M4's next-three-calls rules are not
	// built, so no acted-on share exists to read (owner ruling R3).
	notMeasuredActed = "not measured: AD-21/RO-M4 not built"
	// notMeasuredHelped: helped needs a second arm, which replay has not
	// (law 12).
	notMeasuredHelped = "not measured: replay has no arm"
	// notMeasuredHook: hook seams are measured from AD-20's eligible log,
	// which is not built (owner ruling R3).
	notMeasuredHook = "not measured: AD-20 eligible log not built"
)

// FiringReadingRow is one lab's funnel over one of its surfaces. The
// counts are the margins the rates were read from, so a reader can
// recompute them; Stages is the same reading in #Result's shape.
type FiringReadingRow struct {
	Lab     string `json:"lab"`
	Feature string `json:"feature"`
	Surface string `json:"surface"`
	// Points is the corpus the row was read over, Eligible and ShouldNot
	// its two margins, and TreeUnrecovered the points whose session tree
	// could not be recovered (owner ruling R2: every one of them).
	Points          int               `json:"points"`
	Eligible        int               `json:"eligible"`
	ShouldNot       int               `json:"should_not"`
	Fired           int               `json:"fired"`
	FiredAtEligible int               `json:"fired_at_eligible"`
	FalseFires      int               `json:"false_fires"`
	Delivered       int               `json:"delivered"`
	DeliveredOf     int               `json:"delivered_of"`
	TreeUnrecovered int               `json:"tree_unrecovered"`
	Missed          []SubstituteCount `json:"missed,omitempty"`
	Rates           Rates             `json:"-"` // NaN where a rate has no data; Stages carries the same reading
	Stages          Stages            `json:"stages"`
}

// FiringReading reads the replay funnel for each lab surface, sorted by
// lab then surface. One row per surface: a lab measuring two surfaces is
// two readings, as the ledger key (run, surface) requires.
//
// A point belongs to a surface when the surface fired at it, when the
// rule that decided it names that surface, or when the point is a
// should-not for every surface (no fathom verb should fire there).
// Nothing is inferred from a surface family replay cannot see: a hook,
// stanza or decision-site surface reads "not measured" with its reason.
func FiringReading(points []Point, labs []Lab) []FiringReadingRow {
	var rows []FiringReadingRow
	for _, lab := range labs {
		surfaces := append([]string(nil), lab.Surface...)
		sort.Strings(surfaces)
		for _, surface := range surfaces {
			rows = append(rows, readSurface(points, lab, surface))
		}
	}
	sort.SliceStable(rows, func(i, j int) bool {
		if rows[i].Lab != rows[j].Lab {
			return rows[i].Lab < rows[j].Lab
		}
		return rows[i].Surface < rows[j].Surface
	})
	return rows
}

func readSurface(points []Point, lab Lab, surface string) FiringReadingRow {
	row := FiringReadingRow{Lab: lab.ID, Feature: lab.ID, Surface: surface, Points: len(points), TreeUnrecovered: len(points)}
	if len(lab.Features) > 0 {
		row.Feature = lab.Features[0]
	}
	missed := map[string]int{}
	for _, p := range points {
		label, reason := ClassifyDecisionPoint(p)
		fired := SurfaceKey(p.Call) == surface
		concerns := fired || RuleSurface(reason) == surface || label == ShouldNot && UniversalShouldNot(reason)
		if fired {
			row.Fired++
			if p.Call.Result.Present && !p.Call.Result.IsError {
				row.Delivered++
			}
		}
		if !concerns {
			continue
		}
		switch label {
		case ShouldFire:
			row.Eligible++
			if fired {
				row.FiredAtEligible++
			} else if tool := SubstituteTool(p.Call); tool != "" {
				missed[tool]++
			}
		case ShouldNot:
			row.ShouldNot++
			if fired {
				row.FalseFires++
			}
		}
	}
	row.DeliveredOf = row.Fired
	for tool, count := range missed {
		row.Missed = append(row.Missed, SubstituteCount{Tool: tool, Count: count})
	}
	sort.Slice(row.Missed, func(i, j int) bool { return row.Missed[i].Tool < row.Missed[j].Tool })
	row.Rates = BinaryRates(ConfusionTable{
		TP: row.FiredAtEligible,
		FP: row.FalseFires,
		TN: row.ShouldNot - row.FalseFires,
		FN: row.Eligible - row.FiredAtEligible,
	})
	row.Stages = row.stages(lab, surface)
	return row
}

// stages renders the funnel in #Result's shape. Acted and helped are
// never read here; eligible and fired are read only where replay can see
// the surface at all.
func (row FiringReadingRow) stages(lab Lab, surface string) Stages {
	stages := Stages{Acted: notMeasuredActed, Helped: notMeasuredHelped}
	unreadable := ""
	switch {
	case lab.Firing.Mode == "n/a":
		unreadable = fmt.Sprintf("not measured: firing is n/a: %s", lab.Firing.Reason)
	case strings.HasPrefix(surface, "hook:"):
		unreadable = notMeasuredHook
	case !strings.HasPrefix(surface, "mcp:") && !strings.HasPrefix(surface, "skill:"):
		unreadable = fmt.Sprintf("not measured: replay reads calls, not %s surfaces", surfaceFamily(surface))
	case row.Eligible == 0 && row.Fired == 0:
		unreadable = fmt.Sprintf("not measured: no point names %s", surface)
	}
	if unreadable != "" {
		stages.Eligible, stages.Fired, stages.Delivered = unreadable, unreadable, unreadable
		return stages
	}
	if row.Eligible == 0 {
		// The surface fired but v0 classifies none of its points
		// should-fire (a verb that needs the call's argument): there is no
		// eligible margin, so neither stage is a measured zero.
		reason := fmt.Sprintf("not measured: v0 classifies no point should-fire for %s", surface)
		stages.Eligible, stages.Fired = reason, reason
	} else {
		stages.Eligible = share(row.Eligible, row.Points)
		stages.Fired = rateStage(row.Rates.TPR, row.Eligible)
	}
	if row.DeliveredOf == 0 {
		stages.Delivered = fmt.Sprintf("not measured: %s never fired", surface)
	} else {
		stages.Delivered = share(row.Delivered, row.DeliveredOf)
	}
	return stages
}

// surfaceFamily is the key family a surface belongs to, for a reason line.
func surfaceFamily(surface string) string {
	if i := strings.Index(surface, ":"); i > 0 {
		return surface[:i]
	}
	return surface
}

// share is k of n with its Wilson interval, read through the same
// estimator the rates use.
func share(k, n int) any {
	if n <= 0 {
		return "not measured: no points"
	}
	rates := BinaryRates(ConfusionTable{TP: k, FN: n - k})
	return rateStage(rates.TPR, n)
}

func rateStage(rate Rate, n int) Stage {
	return Stage{Value: rate.Value, Interval: [2]float64{rate.Lo, rate.Hi}, N: n}
}

// ReplayProvenance names what a replay row can prove in place of the arm
// pins it cannot know: the corpus it decoded and the fathom commit that
// read it (owner ruling R4).
type ReplayProvenance struct {
	CorpusSHA    string `json:"corpus_sha"`
	FathomCommit string `json:"fathom_commit"`
}

// ResultRow renders this reading as a #Result row for AppendLabResult:
// one row per (run, surface), arm "replay", spend 0 and the verdict
// indeterminate, because a replay funnel counts and never decides. The
// criterion the run was preregistered to decide is the caller's.
func (row FiringReadingRow) ResultRow(runID, criterionID, preregSHA string, provenance ReplayProvenance) map[string]any {
	return map[string]any{
		"run_id":  runID,
		"task_id": row.Surface,
		"arm":     "replay",
		"trials":  0,
		"evaluation": map[string]any{
			"criterion_id": criterionID,
			"verdict":      "indeterminate",
			"reason":       "replay counts the funnel; it decides no criterion",
		},
		"lab":     row.Lab,
		"feature": row.Feature,
		"stages": map[string]any{
			"eligible":  row.Stages.Eligible,
			"fired":     row.Stages.Fired,
			"delivered": row.Stages.Delivered,
			"acted":     row.Stages.Acted,
			"helped":    row.Stages.Helped,
		},
		"quality":     []QualityReading{},
		"pass_k":      0,
		"replay":      provenance,
		"prereg_sha":  preregSHA,
		"spend_usd":   0.0,
		"spend_label": "replay",
		"attributes":  map[string]any{"gen_ai.tool.name": strings.TrimPrefix(row.Surface, "mcp:")},
	}
}
