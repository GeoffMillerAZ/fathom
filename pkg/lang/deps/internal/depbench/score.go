// Package depbench scores dependency observations in tests. Dialect oracles
// live in their respective tests and adapt independent evidence to TruthFile.
package depbench

import (
	"bytes"
	"encoding/json"

	"github.com/geoffmilleraz/fathom/pkg/lang/deps"
)

// TruthEdge is a native direct dependency. B1 counts only repository targets;
// external bindings can also refute local targets under B2. Raw and Kind
// identify the originating construct. Use Line 0 if the oracle has no line.
type TruthEdge struct {
	Kind       deps.Kind
	Raw        string
	Line       int
	Resolution deps.Resolution
	To         string
}

// TruthFile is present only when the oracle covered the file's direct edges.
// An empty Edges list is evidence that the oracle found no such edges.
type TruthFile struct {
	Path  string
	Edges []TruthEdge
}

// Observation holds independently collected truth and three observations of the
// same population. Repeat runs the reader again, Relocated uses a second root.
type Observation struct {
	Files, Repeat, Relocated []deps.FileDeps
	Truth                    []TruthFile
}

// Reading carries counts and their denominators. B1/B2 apply to complete files
// with oracle coverage. B4Extra / B4ComparableEdges covers all files with
// oracle coverage, including incomplete files. UnknownByCode counts findings,
// and Evidence is the denominator for their share of all reported evidence.
// B2Inconclusive counts edges in complete files whose operands could not be
// uniquely identified against the oracle.
type Reading struct {
	Files, OracleFiles, Complete, CompleteWithOracle, CompleteWithoutOracle int
	Expected, CompleteEdges, Missing, Wrong, Extra, B2Inconclusive          int
	B4Extra, B4ComparableEdges, Edges, Unknowns, Evidence                   int
	UnknownByCode                                                           map[deps.UnknownCode]int
	B1, B2, B3                                                              bool
}

type edgeCounts struct {
	observed, expected, missing, wrong, extra, inconclusive int
}

// Score checks fixture and corpus observations without invoking an oracle. An
// empty complete population and a complete file without unique truth both make
// the soundness bars inconclusive. Apply the population minimum separately.
func Score(o Observation) Reading {
	truth := make(map[string][]TruthFile)
	for _, f := range o.Truth {
		truth[f.Path] = append(truth[f.Path], f)
	}
	seen := make(map[string]int)
	for _, f := range o.Files {
		seen[f.Path]++
	}
	out := Reading{Files: len(o.Files), UnknownByCode: make(map[deps.UnknownCode]int)}
	for _, f := range o.Files {
		out.Edges += len(f.Edges)
		out.Unknowns += len(f.Unknowns)
		for _, u := range f.Unknowns {
			out.UnknownByCode[u.Code]++
		}
		if f.Complete {
			out.Complete++
		}
		rows := truth[f.Path]
		if len(rows) != 1 || seen[f.Path] != 1 {
			if f.Complete {
				out.CompleteWithoutOracle++
			}
			continue
		}
		out.OracleFiles++
		c := compare(f.Edges, rows[0].Edges)
		out.B4ComparableEdges += c.observed
		out.B4Extra += c.extra
		if f.Complete {
			out.CompleteWithOracle++
			out.Expected += c.expected
			out.CompleteEdges += c.observed
			out.Missing += c.missing
			out.Wrong += c.wrong
			out.B2Inconclusive += c.inconclusive
			out.Extra += c.extra
		}
	}
	out.Evidence = out.Edges + out.Unknowns
	covered := out.CompleteWithOracle > 0 && out.CompleteWithoutOracle == 0
	out.B1 = covered && out.Missing == 0
	out.B2 = covered && out.Wrong == 0 && out.B2Inconclusive == 0
	out.B3 = sameOutput(o)
	return out
}

func compare(edges []deps.Edge, truth []TruthEdge) edgeCounts {
	// B1 compares dependency targets, independently of indexer line or operand
	// formatting. B2 uses the construct identity to distinguish wrong targets
	// from independent extra edges. The oracle adapter defines that identity.
	type target struct {
		kind       deps.Kind
		resolution deps.Resolution
		to         string
	}
	wanted := make(map[target]bool)
	for _, u := range truth {
		if isRepositoryTarget(u.Resolution) {
			wanted[target{u.Kind, u.Resolution, u.To}] = true
		}
	}
	have := make(map[target]bool)
	var out edgeCounts
	for _, e := range edges {
		if !isRepositoryTarget(e.Resolution) {
			continue
		}
		out.observed++
		k := target{e.Kind, e.Resolution, e.To}
		have[k] = true
		paired, matched, inconclusive := compareBinding(e, truth)
		switch {
		case inconclusive:
			out.inconclusive++
		case paired && !matched:
			out.wrong++
		case !wanted[k]:
			out.extra++
		}
	}
	for _, u := range truth {
		if !isRepositoryTarget(u.Resolution) {
			continue
		}
		out.expected++
		if !have[target{u.Kind, u.Resolution, u.To}] {
			out.missing++
		}
	}
	return out
}

func isRepositoryTarget(resolution deps.Resolution) bool {
	return resolution == deps.File || resolution == deps.Directory
}

func compareBinding(e deps.Edge, truth []TruthEdge) (paired, matched, inconclusive bool) {
	knownRaw := hasOperand(e.Raw)
	var exactTarget, lineTarget, ambiguous, conflictingLine bool
	var differentExactTargets, exactMissingLine bool
	var exactOperand, lineOperand *TruthEdge
	for i := range truth {
		u := &truth[i]
		if e.Kind != u.Kind {
			continue
		}
		sameOperand := knownRaw && hasOperand(u.Raw) && e.Raw == u.Raw
		if u.Line > 0 && e.Line > 0 && e.Line != u.Line {
			conflictingLine = conflictingLine || sameOperand
			continue
		}
		sameTarget := e.To == u.To && e.Resolution == u.Resolution
		if knownRaw && hasOperand(u.Raw) {
			if sameOperand {
				if exactOperand == nil {
					exactOperand = u
				}
				differentExactTargets = differentExactTargets || u.To != exactOperand.To || u.Resolution != exactOperand.Resolution
				exactMissingLine = exactMissingLine || u.Line <= 0 || e.Line <= 0
				exactTarget = exactTarget || sameTarget
			}
			continue
		}
		// A withheld operand needs a physical line on both sides. Several
		// targets may come from one native operand, but two operands on the
		// same line cannot be distinguished without their raw values.
		if e.Line <= 0 || u.Line <= 0 {
			ambiguous = true
			continue
		}
		if lineOperand == nil {
			lineOperand = u
		} else if !hasOperand(u.Raw) || u.Raw != lineOperand.Raw {
			ambiguous = true
		}
		lineTarget = lineTarget || sameTarget
	}
	if exactOperand != nil {
		// Multiple targets can come from one located operand. Without a line
		// on both sides they can also be distinct occurrences of the same raw
		// operand, so target membership alone cannot prove the binding.
		return true, exactTarget, differentExactTargets && exactMissingLine || !exactTarget && (ambiguous || lineOperand != nil)
	}
	if conflictingLine || ambiguous || (!knownRaw && lineOperand == nil) {
		return false, false, true
	}
	return lineOperand != nil, lineTarget, false
}

func hasOperand(raw string) bool { return raw != "" && raw != "***" }

func sameOutput(o Observation) bool {
	if len(o.Files) == 0 || len(o.Repeat) != len(o.Files) || len(o.Relocated) != len(o.Files) {
		return false
	}
	one, err := json.Marshal(o.Files)
	if err != nil {
		return false
	}
	two, err := json.Marshal(o.Repeat)
	if err != nil {
		return false
	}
	three, err := json.Marshal(o.Relocated)
	return err == nil && bytes.Equal(one, two) && bytes.Equal(one, three)
}
