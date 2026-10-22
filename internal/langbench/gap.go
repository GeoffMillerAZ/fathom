package langbench

import (
	"fmt"
	"math"
	"slices"
	"sort"
)

// Gap is a measured blocker, or a disclosed hole in the evidence. Examples
// are populated only from a separately recorded mismatch file, never guessed.
type Gap struct {
	Class    string   `json:"class"`
	Level    string   `json:"level"`
	Metric   string   `json:"metric"`
	Count    int      `json:"count"`
	Unit     string   `json:"unit"`
	Value    *float64 `json:"value,omitempty"`
	Bar      *float64 `json:"bar,omitempty"`
	Examples []string `json:"examples"`
	Next     string   `json:"next"`
}

type GapReport struct {
	ID       string `json:"id"`
	LevelMet string `json:"level_met"`
	Target   string `json:"target"`
	Gaps     []Gap  `json:"gaps"`
}

// ExplainGap uses the grading engine's own good floors. It does not change a
// reading or treat missing oracle evidence as a measured miss.
func ExplainGap(r Reading) GapReport {
	met, _ := LevelMet(r)
	out := GapReport{ID: r.Dialect, LevelMet: met, Target: r.TargetLevel, Gaps: []Gap{}}
	add := func(class, level, metric string, count int, unit string, value, bar *float64, next string) {
		out.Gaps = append(out.Gaps, Gap{class, level, metric, count, unit, value, bar, []string{}, next})
	}
	if n := r.Tenancy.FilesWithError; n != nil {
		files := 0
		for _, s := range r.Subjects {
			files += s.Files
		}
		if files > 0 && float64(*n)/float64(files) > .05 {
			add("parse error rate", "L1", "parse_error_rate", *n, "files", new(float64(*n)/float64(files)), new(.05), "§13.1 steps 5–8: try a newer pinned grammar; §13.2 step 12: probe the failing fixtures (refusal line §14.2)")
		}
	}
	for _, kind := range r.RequiredKinds {
		if !slices.Contains(r.KindIntersection, kind) {
			add("missing kind", "L1", kind, 1, "kinds", nil, nil, "§13.2 steps 9–12: add the required kind and a fixture, then measure with an oracle covering it")
		}
	}
	for _, level := range Levels {
		if slices.Index(Levels, level) > slices.Index(Levels, r.TargetLevel) {
			break
		}
		for _, t := range thresholdsFor(r.Kind)[level] {
			if slices.Contains(r.NotApplicable, t.metric) || slices.Contains(r.NotMeasured, t.metric) {
				continue
			}
			v := r.Metrics[t.metric]
			if v == nil {
				add("metric not measured", level, t.metric, 1, "metrics", nil, new(t.good), "§13.4 steps 18–20: run an independent oracle that measures this metric")
				continue
			}
			if *v < t.good {
				class, count, unit := gapCount(r, t.metric)
				add(class, level, t.metric, count, unit, new(*v), new(t.good), gapNext(level))
			}
			if tierCeiling(r.MetricTiers[t.metric], level) == GradeTolerated {
				add("oracle tier too weak", level, t.metric, 1, "metrics", nil, nil, "§13.4 steps 18–20: replace tier D or missing evidence with an independent tier A, B or C oracle")
			}
		}
	}
	files := 0
	for _, s := range r.Subjects {
		files += s.Files
	}
	if files > 0 && float64(r.Tenancy.Faults)/float64(files) > MaxFaultRatio {
		add("extraction faults", "L1", "fault_ratio", r.Tenancy.Faults, "files", new(float64(r.Tenancy.Faults)/float64(files)), new(MaxFaultRatio), "§13.4 steps 20–21: fix faulting fixtures and remeasure")
	}
	for _, t := range []struct {
		metric     string
		value, bar float64
		upper      bool
	}{{"throughput_mb_s", r.Tenancy.ThroughputMBs, MinThroughputMBs, false}, {"max_file_ms", float64(r.Tenancy.MaxFileMS), MaxFileMS, true}, {"peak_module_pages", float64(r.Tenancy.PeakModulePages), MaxPeakModulePages, true}} {
		if (!t.upper && t.value < t.bar) || (t.upper && t.value > t.bar) {
			add("tenancy floor", "L1", t.metric, 1, "metrics", new(t.value), new(t.bar), "§13.4 steps 20–21: fix extraction resource use and remeasure; keep the existing bar")
		}
	}
	sort.Slice(out.Gaps, func(i, j int) bool {
		a, b := out.Gaps[i], out.Gaps[j]
		if a.Level != b.Level {
			return a.Level < b.Level
		}
		if a.Class != b.Class {
			return a.Class < b.Class
		}
		return a.Metric < b.Metric
	})
	return out
}

func gapNext(level string) string {
	switch level {
	case "L1":
		return "§13.2 steps 9–12: add a failing fixture, fix kinds/ranges/signatures; §13.4 step 20: remeasure"
	case "L2":
		return "§13.2 steps 10–12: add a failing call fixture and fix the capture; §13.4 steps 19–20: remeasure"
	case "L3":
		return "§13.2 step 13: fix an import fixture and resolution; §13.4 steps 19–20: remeasure"
	default:
		return "§13.4 steps 19–21: inspect typed occurrences and definition mapping, then remeasure"
	}
}

func gapCount(r Reading, metric string) (string, int, string) {
	type pair struct{ class, den, num string }
	p, ok := map[string]pair{
		MSymRecall: {"missing definition", CntOracleDefs, CntMatched}, MSymPrecision: {"extra definition", CntFathomDefs, CntMatched},
		MKindAgreement: {"kind mismatch", CntMatched, CntKindEqual}, MRangeAgreement: {"range mismatch", CntRangeEligible, CntRangeOK}, MSignatureCoverage: {"missing signature", CntCallables, CntCallablesSigned},
		MCallRecall: {"missing call", CntCallTruth, CntCallTruthFound}, MCallPrecision: {"extra call", CntCallFathom, CntCallFathomInTrue},
		MImportResolution: {"unresolved import", CntImportsInRepo, CntImportsAgree}, MFileEdgeRecall: {"missing file edge", CntImportEdgesTruth, CntImportEdgesAgree}, MFileEdgePrecision: {"extra file edge", CntImportEdgesFathom, CntImportEdgesAgree},
		MRefRecall: {"missing reference", CntRefTruth, CntRefTruthFound}, MRefPrecision: {"extra reference", CntRefFathom, CntRefFathomInTrue},
		MDefMapping: {"unmapped definition", CntScipDefs, CntScipDefsMapped}, MTypedRecall: {"missing typed edge", CntScipEdges, CntScipEdgesTyped},
		MXrefRecall: {"missing cross reference", CntXrefTruth, CntXrefTruthFound}, MXrefPrecision: {"extra cross reference", CntXrefFathom, CntXrefFathomInTrue},
	}[metric]
	if !ok {
		return "metric below bar", 1, "metrics"
	}
	if metric == MImportResolution {
		if n := r.Counts[CntImportsUnresolved] + r.Counts[CntImportsElsewhere]; n > 0 {
			return p.class, n, "imports"
		}
	}
	if den, ok := r.Counts[p.den]; ok {
		return p.class, max(0, den-r.Counts[p.num]), "occurrences"
	}
	return p.class, 1, "metrics (raw counts unavailable)"
}

// NearMiss is a measured, first-blocking-level metric just short of its bar.
type NearMiss struct {
	ID            string  `json:"id"`
	Level         string  `json:"level"`
	Metric        string  `json:"metric"`
	Value         float64 `json:"value"`
	Bar           float64 `json:"bar"`
	GrammarCommit string  `json:"grammar_commit"`
	Source        string  `json:"source,omitempty"`
}

// NearMisses takes an absolute ratio margin (0.01 is one percentage point).
// Resource/time metrics are deliberately excluded: their units differ.
func NearMisses(r Reading, margin float64) []NearMiss {
	out := []NearMiss{}
	if margin < 0 || math.IsNaN(margin) || math.IsInf(margin, 0) {
		return out
	}
	report := ExplainGap(r)
	if len(report.Gaps) == 0 {
		return out
	}
	first := report.Gaps[0].Level
	commit := "unknown"
	if r.Grammar != nil && r.Grammar.Commit != "" {
		commit = r.Grammar.Commit
	}
	for _, g := range report.Gaps {
		if g.Level != first || g.Value == nil || g.Bar == nil {
			continue
		}
		if g.Metric != "parse_error_rate" && !knownMetric(g.Metric) {
			continue
		}
		d := math.Abs(*g.Value - *g.Bar)
		if d > 0 && d <= margin+1e-12 {
			out = append(out, NearMiss{ID: r.Dialect, Level: g.Level, Metric: g.Metric, Value: *g.Value, Bar: *g.Bar, GrammarCommit: commit})
		}
	}
	return out
}

// ReadingRegression compares every level, including grade regressions above
// the highest contiguous met level. Unknown/not-applicable are not proof.
func ReadingRegression(before, after Reading) []string {
	out := []string{}
	for _, l := range Levels {
		if gradeRank[after.Grade.get(l)] < gradeRank[before.Grade.get(l)] {
			out = append(out, fmt.Sprintf("%s %s -> %s", l, before.Grade.get(l), after.Grade.get(l)))
		}
	}
	if slices.Index(Levels, after.LevelMet) < slices.Index(Levels, before.LevelMet) {
		out = append(out, fmt.Sprintf("level_met %s -> %s", before.LevelMet, after.LevelMet))
	}
	return out
}

// MismatchClasses includes counted misses even when their level meets its
// good floor. This is the loop's diagnosis, not a change to the grading bars.
func MismatchClasses(r Reading) []Gap {
	out := ExplainGap(r).Gaps
	seen := map[string]bool{}
	for _, g := range out {
		seen[g.Metric] = true
	}
	for _, level := range Levels {
		if slices.Index(Levels, level) > slices.Index(Levels, r.TargetLevel) {
			break
		}
		for _, t := range thresholdsFor(r.Kind)[level] {
			if seen[t.metric] || r.Metrics[t.metric] == nil {
				continue
			}
			class, n, unit := gapCount(r, t.metric)
			if n > 0 && unit != "metrics (raw counts unavailable)" && unit != "metrics" {
				out = append(out, Gap{Class: class, Level: level, Metric: t.metric, Count: n, Unit: unit, Examples: []string{}, Next: gapNext(level)})
			}
		}
	}
	sort.Slice(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if a.Class != b.Class {
			return a.Class < b.Class
		}
		return a.Metric < b.Metric
	})
	return out
}
