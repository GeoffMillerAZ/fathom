package langbench

import (
	"sort"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/core"
)

// Raw counts a subject records; metrics are ratios of their sums, so a
// reading merged over subjects is exact (never an average of ratios).
const (
	CntOracleDefs      = "oracle_defs"
	CntFathomDefs      = "fathom_defs"
	CntMatched         = "matched"
	CntKindEqual       = "kind_equal"
	CntRangeEligible   = "range_eligible"
	CntRangeOK         = "range_ok"
	CntCallables       = "callables"
	CntCallablesSigned = "callables_signed"

	CntCallTruth        = "call_truth"
	CntCallTruthFound   = "call_truth_found"
	CntCallFathom       = "call_fathom"
	CntCallFathomInTrue = "call_fathom_in_truth"

	CntScipDefs       = "scip_defs"
	CntScipDefsMapped = "scip_defs_mapped"
	CntScipEdges      = "scip_edges"
	CntScipEdgesTyped = "scip_edges_typed"

	CntFileEdgesTruth  = "file_edges_truth"
	CntFileEdgesFathom = "file_edges_fathom"
	CntFileEdgesAgree  = "file_edges_agree"

	CntRefTruth        = "ref_truth"
	CntRefTruthFound   = "ref_truth_found"
	CntRefFathom       = "ref_fathom"
	CntRefFathomInTrue = "ref_fathom_in_truth"
)

// Families group counts by the oracle that produced them; a subject
// records the tier each family was measured at.
const (
	FamilyL1        = "l1"
	FamilyCalls     = "calls"
	FamilyDefMap    = "defmap"
	FamilyTyped     = "typed"
	FamilyFileEdges = "file_edges"
	FamilyRefs      = "refs"
)

// BareName is a qualified name's last segment (§1.4: qualified names are
// compared on the last segment).
func BareName(q string) string {
	i := strings.LastIndexAny(q, ".:#/\\ )")
	return q[i+1:]
}

// L1Result is one subject's §1.4 definition matching.
type L1Result struct {
	Counts       map[string]int
	Excluded     map[string]int
	Intersection []string
	// DialectOnly are fathom's definitions inside the kind intersection
	// the oracle matched nothing to (name in the dialect's match form) —
	// the only candidates an adjudication may admit (D3).
	DialectOnly []OracleDef
}

type scoredDef struct {
	path, name, canonical string
	line, end             int
	callable, signed      bool
	matched               bool
}

// ScoreL1 matches fathom's definitions against an oracle's (§1.4): same
// file, same bare name, |Δline| ≤ 2, nearest first, one-to-one.
// Definitions whose kind maps outside the kind intersection on either side
// are excluded from both counts and tallied as "<side>:<kind>".
func ScoreL1(m Manifest, oracle string, nodes []core.SymbolNode, defs []OracleDef) L1Result {
	inter := m.Intersection(oracle)
	in := map[string]bool{}
	for _, c := range inter {
		in[c] = true
	}
	res := L1Result{Counts: map[string]int{}, Excluded: map[string]int{}, Intersection: inter}

	var mine []*scoredDef
	for _, n := range nodes {
		c, callable, ok := m.CanonicalForSource(string(n.Kind))
		if !ok || !in[c] {
			res.Excluded["fathom:"+string(n.Kind)]++
			continue
		}
		mine = append(mine, &scoredDef{path: n.FilePath, name: m.MatchName(n.Name), canonical: c, line: n.Line, end: n.EndLine,
			callable: callable, signed: strings.TrimSpace(n.Signature) != ""})
	}
	var theirs []*scoredDef
	for _, d := range defs {
		c, ok := m.OracleCanonical(oracle, d.Kind, d.Scoped)
		if !ok || !in[c] {
			res.Excluded[oracle+":"+d.Kind]++
			continue
		}
		theirs = append(theirs, &scoredDef{path: d.Path, name: d.Name, canonical: c, line: d.Line, end: d.End})
	}
	res.Counts[CntFathomDefs] = len(mine)
	res.Counts[CntOracleDefs] = len(theirs)
	for _, d := range mine {
		if d.callable {
			res.Counts[CntCallables]++
			if d.signed {
				res.Counts[CntCallablesSigned]++
			}
		}
	}

	type key struct{ path, name string }
	byKey := map[key][]*scoredDef{}
	for _, d := range theirs {
		byKey[key{d.path, d.name}] = append(byKey[key{d.path, d.name}], d)
	}
	type pair struct {
		f, o *scoredDef
		dist int
	}
	var pairs []pair
	for _, f := range mine {
		for _, o := range byKey[key{f.path, f.name}] {
			dist := f.line - o.line
			if dist < 0 {
				dist = -dist
			}
			if dist <= 2 {
				pairs = append(pairs, pair{f, o, dist})
			}
		}
	}
	sort.SliceStable(pairs, func(i, j int) bool {
		a, b := pairs[i], pairs[j]
		if a.dist != b.dist {
			return a.dist < b.dist
		}
		if a.f.path != b.f.path {
			return a.f.path < b.f.path
		}
		if a.o.line != b.o.line {
			return a.o.line < b.o.line
		}
		return a.f.line < b.f.line
	})
	for _, p := range pairs {
		if p.f.matched || p.o.matched {
			continue
		}
		p.f.matched, p.o.matched = true, true
		res.Counts[CntMatched]++
		if p.f.canonical == p.o.canonical {
			res.Counts[CntKindEqual]++
		}
		if p.o.end > 0 {
			res.Counts[CntRangeEligible]++
			d := p.f.end - p.o.end
			if p.f.end > 0 && d >= -1 && d <= 1 {
				res.Counts[CntRangeOK]++
			}
		}
	}
	for _, d := range mine {
		if !d.matched {
			res.DialectOnly = append(res.DialectOnly, OracleDef{Path: d.path, Name: d.name, Line: d.line, End: d.end, Kind: d.canonical})
		}
	}
	return res
}

// CallPair is (enclosing definition's bare name, callee bare name) in a
// file — the §1.4 unit of call truth.
type CallPair struct {
	Path, Caller, Callee string
}

// FathomCallPairs returns the extractor's call pairs over files, each
// callee reduced to its last dotted segment (§1.1 L2).
func FathomCallPairs(nodes []core.SymbolNode, files map[string]bool) map[CallPair]bool {
	return callPairs(nodes, files, BareName)
}

func callPairs(nodes []core.SymbolNode, files map[string]bool, name func(string) string) map[CallPair]bool {
	out := map[CallPair]bool{}
	for _, n := range nodes {
		if files != nil && !files[n.FilePath] {
			continue
		}
		for _, c := range n.Calls {
			callee := name(c)
			if callee == "" {
				continue
			}
			out[CallPair{n.FilePath, name(n.Name), callee}] = true
		}
	}
	return out
}

// ScorePairs counts truth against found for one pair family, writing the
// four counts under the given keys.
func ScorePairs(truth, found map[CallPair]bool, counts map[string]int, kTruth, kTruthFound, kFound, kFoundInTruth string) {
	counts[kTruth] += len(truth)
	counts[kFound] += len(found)
	for p := range truth {
		if found[p] {
			counts[kTruthFound]++
		}
	}
	for p := range found {
		if truth[p] {
			counts[kFoundInTruth]++
		}
	}
}
