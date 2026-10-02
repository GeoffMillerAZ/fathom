package langbench

import (
	"fmt"
	"math"
	"slices"
	"sort"
	"strconv"
	"strings"
)

// The excellence board (docs/languages.md's first generated section): the
// queue entries that carry a program tier (T1..T6), their level met, and the
// next gap, all computed from the queue, the readings and reading.go's
// thresholds. Tier membership is queue data alone; an entry without a tier is
// outside the target set and is only counted at the foot.

// Buckets an item falls in; a level met is one of Levels.
const (
	bucketNone        = "none"
	bucketRefused     = "refused"
	bucketTolerated   = "tolerated"
	bucketUnsupported = "unsupported"
)

// boardBucket says where one queue item stands: a level (L1..L4), "none"
// (measured, no level), "refused", "tolerated" (registered, no reading) or
// "unsupported". A refused item claims nothing whatever its reading says and
// an item with no reading claims no level (the CountMilestones rules).
func boardBucket(in TrackerInput, e QueueEntry) (string, Reading, bool) {
	if _, refused := in.Refused[e.ID]; refused {
		return bucketRefused, Reading{}, false
	}
	if r, ok := in.Readings[e.ID]; ok {
		if r.LevelMet == LevelNone {
			return bucketNone, r, true
		}
		return r.LevelMet, r, true
	}
	if _, ok := in.Registered[e.ID]; ok {
		return bucketTolerated, Reading{}, false
	}
	return bucketUnsupported, Reading{}, false
}

func kindLabel(kind string) string {
	if kind == "tooling" {
		return "tool format"
	}
	return "language"
}

// boardCounts is one summary row.
type boardCounts struct {
	items                       int
	byBucket                    map[string]int
	atTarget, atTargetExcellent int
}

func (c *boardCounts) add(in TrackerInput, e QueueEntry) {
	bucket, r, measured := boardBucket(in, e)
	c.items++
	if c.byBucket == nil {
		c.byBucket = map[string]int{}
	}
	c.byBucket[bucket]++
	if !measured || levelIndex(r.LevelMet) < levelIndex(e.TargetLevel) {
		return
	}
	c.atTarget++
	if allExcellentTo(r, e.TargetLevel) {
		c.atTargetExcellent++
	}
}

func allExcellentTo(r Reading, target string) bool {
	for _, l := range Levels[:levelIndex(target)+1] {
		if r.GradeAt(l) != GradeExcellent {
			return false
		}
	}
	return true
}

func (c boardCounts) row(label string) string {
	cols := []string{"L1", "L2", "L3", "L4", bucketNone, bucketRefused, bucketTolerated, bucketUnsupported}
	var b strings.Builder
	b.WriteString(label + " | " + strconv.Itoa(c.items))
	for _, k := range cols {
		b.WriteString(" | " + strconv.Itoa(c.byBucket[k]))
	}
	b.WriteString(" | " + strconv.Itoa(c.atTarget) + " | " + strconv.Itoa(c.atTargetExcellent))
	return "| " + b.String() + " |\n"
}

// RenderBoard renders the excellence board: a summary of tiered items by
// (tier, kind) with a total per kind, then one table per tier, then the count
// of untiered entries. It is deterministic: queue order, no clock.
func RenderBoard(in TrackerInput) string {
	var tierSet []string
	byTier := map[string][]QueueEntry{}
	var untiered []string
	for _, e := range in.Queue {
		if e.Tier == "" {
			untiered = append(untiered, e.ID)
			continue
		}
		if _, ok := byTier[e.Tier]; !ok {
			tierSet = append(tierSet, e.Tier)
		}
		byTier[e.Tier] = append(byTier[e.Tier], e)
	}
	sort.Strings(tierSet)
	sort.Strings(untiered)

	var b strings.Builder
	b.WriteString("## Excellence board (generated)\n\n")
	b.WriteString("Generated from the queue's `tier` field (T1..T6; an entry without one is outside the\n")
	b.WriteString("excellence target set), the committed readings and the thresholds of design §1.2; never\n")
	b.WriteString("hand-edit it. Level counts are the level met exactly; `none` is measured with no level met;\n")
	b.WriteString("`refused`, `tolerated` and `unsupported` claim no level. At target is a level met at or above the\n")
	b.WriteString("item's target; the last column also needs every level up to the target graded excellent.\n\n")
	b.WriteString("| Tier | Kind | Items | L1 | L2 | L3 | L4 | None | Refused | Tolerated | Unsupported | At target | At target, all excellent |\n")
	b.WriteString("|---|---|---|---|---|---|---|---|---|---|---|---|---|\n")
	totals := map[string]*boardCounts{"language": {}, "tooling": {}}
	for _, tier := range tierSet {
		for _, kind := range []string{"language", "tooling"} {
			var c boardCounts
			for _, e := range byTier[tier] {
				if e.Kind == kind {
					c.add(in, e)
					totals[kind].add(in, e)
				}
			}
			if c.items > 0 {
				b.WriteString(c.row(tier + " | " + kindLabel(kind)))
			}
		}
	}
	for _, kind := range []string{"language", "tooling"} {
		b.WriteString(totals[kind].row("**Total** | " + kindLabel(kind)))
	}
	for _, tier := range tierSet {
		fmt.Fprintf(&b, "\n### %s\n\n", tier)
		b.WriteString("| Item | Kind | Target | Level met | Next gap |\n|---|---|---|---|---|\n")
		for _, e := range byTier[tier] {
			level, gap := boardLevelAndGap(in, e)
			fmt.Fprintf(&b, "| %s | %s | %s | %s | %s |\n", cell(e.Name), kindLabel(e.Kind), e.TargetLevel, cell(level), cell(gap))
		}
	}
	if len(untiered) == 0 {
		b.WriteString("\nUntiered entries (outside the excellence target set): 0.\n")
	} else {
		fmt.Fprintf(&b, "\nUntiered entries (outside the excellence target set): %d (%s).\n", len(untiered), strings.Join(untiered, ", "))
	}
	return b.String()
}

func boardLevelAndGap(in TrackerInput, e QueueEntry) (level, gap string) {
	bucket, r, measured := boardBucket(in, e)
	switch bucket {
	case bucketRefused:
		row, _, _ := strings.Cut(in.Refused[e.ID], ":")
		return bucketRefused, "re-plan (" + row + " refused)"
	case bucketTolerated:
		return bucketTolerated, "no reading (" + e.PlanRow + ")"
	case bucketUnsupported:
		return bucketUnsupported, "no dialect (" + e.PlanRow + ")"
	}
	if !measured {
		return bucket, ""
	}
	return levelCell(r), nextGap(r, e.TargetLevel)
}

// nextGap names what stands between a measured item and excellent at its
// target: the first level up to the target that is not met, else the first
// met level graded good, else nothing.
func nextGap(r Reading, target string) string {
	met := levelIndex(r.LevelMet)
	for _, l := range Levels[:levelIndex(target)+1] {
		if levelIndex(l) > met {
			return l + " not met: " + unmetCause(r, l)
		}
	}
	for _, l := range Levels[:levelIndex(target)+1] {
		if r.GradeAt(l) == GradeGood {
			return l + " good: " + goodCause(r, l)
		}
	}
	return "excellent at target"
}

func boardNum(v float64) string {
	return strconv.FormatFloat(math.Round(v*10000)/10000, 'f', -1, 64)
}

// unmetCause is the first metric of the level below its good bar, else the
// reason the level still fails: a metric nobody measured, an oracle tier that
// proves nothing, or the tenancy floor.
func unmetCause(r Reading, level string) string {
	ts := thresholdsFor(r.Kind)[level]
	for _, t := range ts {
		if slices.Contains(r.NotApplicable, t.metric) {
			continue
		}
		v := r.Metrics[t.metric]
		switch {
		case v == nil && slices.Contains(r.NotMeasured, t.metric):
			continue
		case v == nil:
			return t.metric + " not measured"
		case *v < t.good:
			return fmt.Sprintf("%s %s < %s", t.metric, boardNum(*v), boardNum(t.good))
		}
	}
	for _, t := range ts {
		if v := r.Metrics[t.metric]; v != nil && tierCeiling(r.MetricTiers[t.metric], level) == GradeTolerated {
			return fmt.Sprintf("%s proven only by oracle tier %s", t.metric, r.MetricTiers[t.metric])
		}
	}
	if !tenancyHolds(r) {
		return "tenancy floor"
	}
	if level != Levels[0] {
		return "a level below is not met"
	}
	return "no metric measured"
}

// goodCause is the first metric of a good level that stops it being
// excellent: a value under the excellent bar, the oracle tier that caps the
// metric at good, or a metric the oracle cannot measure.
func goodCause(r Reading, level string) string {
	for _, t := range thresholdsFor(r.Kind)[level] {
		if slices.Contains(r.NotApplicable, t.metric) {
			continue
		}
		v := r.Metrics[t.metric]
		switch {
		case v == nil && slices.Contains(r.NotMeasured, t.metric):
			return t.metric + " not measured"
		case v == nil:
			continue
		case *v < t.excellent:
			return fmt.Sprintf("%s %s < %s", t.metric, boardNum(*v), boardNum(t.excellent))
		case tierCeiling(r.MetricTiers[t.metric], level) != GradeExcellent:
			return "oracle tier " + r.MetricTiers[t.metric] + " caps at good"
		}
	}
	return "graded good"
}
