package indexbench

// scip_oracle.go — the L0 typed-edge oracle for a language beyond Go
// (ADR-041 decision 5, SU-12, docs/plans/substrate.md): does fathom's own
// edge set for a subject (its native typed pass, an imported SCIP pass, or
// — when neither has run for this language yet — the syntax call graph)
// reproduce what an INDEPENDENT SCIP index says is there. Free tier
// (ADR-032): both sides are read once, nothing calls an LLM or a network.
//
// This is oracle.go's sibling, not its replacement: oracle.go's
// IndexOracleReport scores Go against a go/types truth this repo can build
// for itself; ScipOracle scores ANY language's edges against a SCIP truth
// (a real one from a calibration subject, or — offline — the SU-9 fixture)
// because fathom has no per-language type-checker of its own to build a
// truth from.

import (
	"sort"

	"github.com/geoffmilleraz/fathom/internal/core"
)

// ScipSubject is one scored unit for ScipOracle.
type ScipSubject struct {
	// Language names the subject for the report (e.g. "typescript") — not
	// consulted for scoring, purely a grouping/display key.
	Language string
	// FathomEdges is fathom's own edge set for this subject: typed edges
	// read from the store when a native or imported pass has already
	// populated them, or the syntax call graph
	// (callgraph.Build + CalleesDetail) when it has not. The oracle scores
	// whatever it is handed — deciding which source is fathom's current
	// answer for this language is the caller's job, not this package's.
	FathomEdges []core.Edge
	// Truth is the SCIP-derived edge set for the SAME subject — typically
	// internal/scip.Import.Packages flattened, from a real external
	// indexer's output (or, offline, the SU-9 fixture's hand-built one).
	// Truth edges carry a real Kind (core.EdgeDirect / core.EdgeUnresolved,
	// per internal/scip's own mapping); FathomEdges' Kind is read only when
	// present and is otherwise ignored — a syntax-only edge (Kind "") still
	// joins the DIRECT truth bucket by (FromSymbol, ToSymbol) alone.
	Truth []core.Edge
}

// ScipOracleRow scores one (language, truth edge kind) bucket. See the
// package doc comment above for what each field means; in short:
// Recall answers "how much of what SCIP claims (of this kind) did
// fathom's edges reproduce"; Precision is meaningful only on the "direct"
// row (an EdgeUnresolved truth edge names no real target fathom could
// ever predict, so it is never precision's numerator or denominator) and
// carries the WHOLE subject's Predicted count there, 0 elsewhere, so
// summing Predicted across a language's rows never double-counts.
// UnresolvedRate is language-level (repeated on every row for that
// language): the SCIP truth's own share of edges it could not resolve
// either — a property of the source, not a fathom defect.
type ScipOracleRow struct {
	Language       string  `json:"language"`
	EdgeKind       string  `json:"edge_kind"`
	Truth          int     `json:"truth"`
	Predicted      int     `json:"predicted,omitempty"`
	Correct        int     `json:"correct"`
	Precision      float64 `json:"precision"`
	Recall         float64 `json:"recall"`
	UnresolvedRate float64 `json:"unresolved_rate"`
}

// ScipOracleReport is ScipOracle's whole run — the same {Oracle, Corpus,
// rows} shape oracle.go's own Report commits as a golden (Services there,
// Rows here), so a reader who already knows how to read one free-tier
// oracle's report shape can read this one too.
type ScipOracleReport struct {
	Oracle string          `json:"oracle"`
	Corpus string          `json:"corpus"`
	Rows   []ScipOracleRow `json:"rows"`
}

// ScipOracle scores every subject and returns rows sorted by (Language,
// EdgeKind).
func ScipOracle(subjects []ScipSubject) ScipOracleReport {
	rep := ScipOracleReport{Oracle: "internal/indexbench/scip-oracle"}
	for _, sub := range subjects {
		rep.Rows = append(rep.Rows, scoreScipSubject(sub)...)
	}
	sort.Slice(rep.Rows, func(i, j int) bool {
		if rep.Rows[i].Language != rep.Rows[j].Language {
			return rep.Rows[i].Language < rep.Rows[j].Language
		}
		return rep.Rows[i].EdgeKind < rep.Rows[j].EdgeKind
	})
	return rep
}

// edgePairKey is the join key ScipOracle scores on: (FromSymbol, ToSymbol)
// alone, ignoring Kind — a syntax-only fathom edge (Kind "") must still be
// able to match a SCIP truth edge's core.EdgeDirect.
func edgePairKey(e core.Edge) string { return e.FromSymbol + "\x00" + e.ToSymbol }

func edgePairSet(edges []core.Edge) map[string]bool {
	out := make(map[string]bool, len(edges))
	for _, e := range edges {
		out[edgePairKey(e)] = true
	}
	return out
}

func scoreScipSubject(sub ScipSubject) []ScipOracleRow {
	predicted := edgePairSet(sub.FathomEdges)

	byKind := map[core.EdgeKind][]core.Edge{}
	for _, e := range sub.Truth {
		byKind[e.Kind] = append(byKind[e.Kind], e)
	}
	unresolvedRate := round4(rate(len(byKind[core.EdgeUnresolved]), len(sub.Truth)))

	kinds := make([]core.EdgeKind, 0, len(byKind))
	for k := range byKind {
		kinds = append(kinds, k)
	}
	sort.Slice(kinds, func(i, j int) bool { return kinds[i] < kinds[j] })

	rows := make([]ScipOracleRow, 0, len(kinds))
	for _, k := range kinds {
		edges := byKind[k]
		correct := 0
		for _, e := range edges {
			if predicted[edgePairKey(e)] {
				correct++
			}
		}
		label := string(k)
		if label == "" {
			label = "unlabeled"
		}
		row := ScipOracleRow{
			Language:       sub.Language,
			EdgeKind:       label,
			Truth:          len(edges),
			Correct:        correct,
			Recall:         round4(rate(correct, len(edges))),
			UnresolvedRate: unresolvedRate,
		}
		// Precision lives only on the direct row (see the type doc
		// comment): `correct` computed above, for k == EdgeDirect, is
		// exactly |directTruthSet ∩ predictedSet| — the same true-positive
		// count precision's numerator needs, just accumulated against a
		// different denominator (Truth count there, Predicted count here).
		if k == core.EdgeDirect {
			row.Predicted = len(sub.FathomEdges)
			row.Precision = round4(rate(correct, len(sub.FathomEdges)))
		}
		rows = append(rows, row)
	}
	return rows
}
