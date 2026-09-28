package langbench

import (
	"fmt"
	"math"
	"slices"
	"sort"
)

// MetricDelta is one metric of a parity run: the incumbent's value, the
// dialect's, and new minus old (nil where either side did not measure it).
type MetricDelta struct {
	Metric    string   `json:"metric"`
	Incumbent *float64 `json:"incumbent"`
	Dialect   *float64 `json:"dialect"`
	Delta     *float64 `json:"delta"`
	// Gated marks the L1 metrics the §11 step-2 merge rule compares.
	Gated bool `json:"gated"`
}

// Parity is the §11 step-2 comparison of a dialect against the incumbent
// it retires, measured on the same subjects against the same oracles.
type Parity struct {
	Dialect        string        `json:"dialect"`
	Incumbent      string        `json:"incumbent"`
	Target         string        `json:"target_level"`
	IncumbentLevel string        `json:"incumbent_level_met"`
	DialectLevel   string        `json:"dialect_level_met"`
	IncumbentGrade Grade         `json:"incumbent_grade"`
	DialectGrade   Grade         `json:"dialect_grade"`
	Metrics        []MetricDelta `json:"metrics"`
	// KindsLost are SourceKinds the incumbent emitted and the dialect does
	// not: a regression unless the port row's reason names why (§11 step 3).
	KindsLost   []string `json:"kinds_lost"`
	KindsGained []string `json:"kinds_gained"`
	Merge       bool     `json:"merge"`
	Reasons     []string `json:"reasons"`
}

func levelRank(l string) int {
	if i := slices.Index(Levels, l); i >= 0 {
		return i + 1
	}
	return 0
}

// ComparePortParity applies §11 step 2: the port may merge when every L1
// metric the incumbent measured is measured for the dialect and is at
// least the incumbent's, and the dialect's reading meets the target level.
// incKinds and dialKinds are the SourceKinds each side emitted.
func ComparePortParity(inc, dial Reading, incumbent, target string, incKinds, dialKinds map[string]int) Parity {
	p := Parity{Dialect: dial.Dialect, Incumbent: incumbent, Target: target,
		IncumbentLevel: inc.LevelMet, DialectLevel: dial.LevelMet, IncumbentGrade: inc.Grade, DialectGrade: dial.Grade,
		KindsLost: []string{}, KindsGained: []string{}, Reasons: []string{}}
	gated := map[string]bool{}
	var order []string
	th := thresholdsFor(dial.Kind)
	for _, level := range Levels {
		for _, t := range th[level] {
			if level == "L1" {
				gated[t.metric] = true
			}
			if !slices.Contains(order, t.metric) {
				order = append(order, t.metric)
			}
		}
	}
	for _, m := range order {
		a, b := inc.Metrics[m], dial.Metrics[m]
		if a == nil && b == nil {
			continue
		}
		d := MetricDelta{Metric: m, Incumbent: a, Dialect: b, Gated: gated[m]}
		if a != nil && b != nil {
			v := math.Round((*b-*a)*1e4) / 1e4
			d.Delta = &v
		}
		p.Metrics = append(p.Metrics, d)
		if !gated[m] || a == nil {
			continue
		}
		switch {
		case b == nil:
			p.Reasons = append(p.Reasons, fmt.Sprintf("%s: the incumbent measured %.4f, the dialect is not measured", m, *a))
		case *b < *a:
			p.Reasons = append(p.Reasons, fmt.Sprintf("%s: dialect %.4f < incumbent %.4f", m, *b, *a))
		}
	}
	if levelRank(dial.LevelMet) < levelRank(target) {
		p.Reasons = append(p.Reasons, fmt.Sprintf("target %s not met: the dialect's level_met is %s", target, dial.LevelMet))
	}
	for k := range incKinds {
		if dialKinds[k] == 0 {
			p.KindsLost = append(p.KindsLost, k)
		}
	}
	for k := range dialKinds {
		if incKinds[k] == 0 {
			p.KindsGained = append(p.KindsGained, k)
		}
	}
	sort.Strings(p.KindsLost)
	sort.Strings(p.KindsGained)
	p.Merge = len(p.Reasons) == 0
	return p
}

// WithKindsOf returns m with other's kinds added for every SourceKind m
// does not already declare (keyed "incumbent:<key>"), so an incumbent's
// output is scored against the dialect's oracles without its own kinds
// falling out as excluded.
func (m Manifest) WithKindsOf(other Manifest) Manifest {
	out := m
	out.Kinds = map[string]KindSpec{}
	have := map[string]bool{}
	for k, v := range m.Kinds {
		out.Kinds[k] = v
		have[v.SourceKind] = true
	}
	for k, v := range other.Kinds {
		if !have[v.SourceKind] {
			out.Kinds["incumbent:"+k] = v
		}
	}
	return out
}
