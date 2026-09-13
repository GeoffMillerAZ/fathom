package indexbench

import (
	"sort"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/contextbench/publicloc"
)

// Edge kinds a comparable EdgeSet may carry (SB-28).
const (
	// EdgeKindCall is a caller->callee structural edge — a static call
	// graph's edge (fathom's own find_callers, or a competitor's).
	EdgeKindCall = "call"
	// EdgeKindReference is one occurrence site of a symbol — fathom's own
	// `references` tool's shape, and what an LSP's references/find-usages
	// call returns (SB-30).
	EdgeKindReference = "reference"
)

// Edge is one edge in a comparable edge set — a competitor's call or
// reference-occurrence edge, translated into the vocabulary CompareEdges
// scores. For EdgeKindCall, From is the caller's name and To the callee's,
// in whatever spelling the source tool used (CompareEdges resolves an
// unqualified spelling against the truth when it is unambiguous). For
// EdgeKindReference, From is the occurrence site as "file:line" (or a bare
// file when no line is known) and To is the referenced symbol's name.
type Edge struct {
	From string `json:"from"`
	To   string `json:"to"`
	Kind string `json:"kind"`
}

// EdgeSet is a loaded competitor edge collection plus a named count of what
// its source could not be turned into a comparable Edge — a count, never a
// silent drop (BP-30's rule, applied to loaders as well as scorers).
type EdgeSet struct {
	Edges        []Edge         `json:"edges"`
	DroppedKinds map[string]int `json:"dropped_kinds,omitempty"`
}

// Comparison is one edge set's precision/recall against the go/types
// truth — deliberately the same core fields as ToolScore, so a
// competitor's row sits beside fathom's own without translation.
type Comparison struct {
	Tool                 string  `json:"tool,omitempty"`
	Queries              int     `json:"queries"`
	Population           int     `json:"population"`
	Predicted            int     `json:"predicted"`
	Truth                int     `json:"truth"`
	Correct              int     `json:"correct"`
	Precision            float64 `json:"precision"`
	Recall               float64 `json:"recall"`
	Unadjudicated        int     `json:"unadjudicated,omitempty"`
	UnadjudicatedQueries int     `json:"unadjudicated_queries,omitempty"`
}

// callTargetIndex resolves a possibly-bare callee or caller spelling to the
// one truth-qualified name it can only mean, or reports that the truth
// cannot rule on it — refsubset.go's ambiguity rule (BP-30), extended from
// "receiverless functions mentioned in text" to every declared name, because
// a competitor's call edge may name a method with no receiver qualifier at
// all (a static graph's node label is typically bare).
type callTargetIndex struct {
	exact  map[string]bool
	byBare map[string][]string
}

func newCallTargetIndex(truth publicloc.GoTruth) callTargetIndex {
	idx := callTargetIndex{exact: map[string]bool{}, byBare: map[string][]string{}}
	for _, n := range declaredNames(truth) {
		idx.exact[n] = true
		b := bareName(n)
		idx.byBare[b] = append(idx.byBare[b], n)
	}
	return idx
}

// resolve returns the truth-qualified name that name refers to, or ("",
// false) when the truth is not entitled to rule on it: unknown to the
// truth, or a bare spelling shared by more than one declaration.
func (idx callTargetIndex) resolve(name string) (string, bool) {
	if idx.exact[name] {
		return name, true
	}
	cands := idx.byBare[bareName(name)]
	if len(cands) == 1 {
		return cands[0], true
	}
	return "", false
}

// CompareEdges scores theirs — an externally produced edge set (a static
// graph's call edges, or an LSP's reference occurrences) — against the
// go/types truth, with the SAME precision/recall accounting and the SAME
// adjudicability rule oracle.go applies to fathom's own answers (BP-30): an
// edge whose target the truth is not entitled to rule on (a name it does
// not declare, a bare spelling shared by more than one declaration, an
// occurrence in a file go/types never walked) is Unadjudicated — never
// counted wrong, never counted right, never in a rate.
//
// EdgeKindCall edges are grouped by callee (To) and scored against
// truth.CallersByName: each edge's From is resolved the same way To is,
// and a query is one distinct, truth-resolvable callee. EdgeKindReference
// edges are grouped by referenced name (To) and scored against
// truth.UsesByName through ReferencesOracleSubset — only the file half of
// From enters the comparison (a trailing ":line" is stripped), matching
// scoreReferences' own per-file truth.
//
// theirs may mix kinds; CompareEdges pools every scored query into one
// Comparison, the same pooling ToolScore already does per tool rather than
// per query shape.
func CompareEdges(theirs []Edge, truth publicloc.GoTruth) Comparison {
	calls := newCallTargetIndex(truth)
	refs := NewReferencesOracleSubset(truth, "")
	refAdjudicable := setOf(refs.Names)

	type groupKey struct{ kind, to string }
	groups := map[groupKey][]Edge{}
	var order []groupKey
	for _, e := range theirs {
		k := groupKey{e.Kind, e.To}
		if _, ok := groups[k]; !ok {
			order = append(order, k)
		}
		groups[k] = append(groups[k], e)
	}
	sort.Slice(order, func(i, j int) bool {
		if order[i].kind != order[j].kind {
			return order[i].kind < order[j].kind
		}
		return order[i].to < order[j].to
	})

	cmp := Comparison{Population: len(calls.exact) + len(refAdjudicable)}
	for _, k := range order {
		edges := groups[k]
		switch k.kind {
		case EdgeKindCall:
			target, ok := calls.resolve(k.to)
			if !ok {
				cmp.Unadjudicated += len(edges)
				cmp.UnadjudicatedQueries++
				continue
			}
			want := setOf(truth.CallersByName[target])
			got := map[string]bool{}
			for _, e := range edges {
				caller, ok := calls.resolve(e.From)
				if !ok {
					cmp.Unadjudicated++
					continue
				}
				got[caller] = true
			}
			cmp.Queries++
			accumulateComparison(&cmp, got, want)
		case EdgeKindReference:
			if !refAdjudicable[k.to] {
				cmp.Unadjudicated += len(edges)
				cmp.UnadjudicatedQueries++
				continue
			}
			want := setOf(truth.UsesByName[k.to])
			got := map[string]bool{}
			for _, e := range edges {
				f := fileOfSite(e.From)
				if refs.Adjudicable(f) {
					got[f] = true
				} else {
					cmp.Unadjudicated++
				}
			}
			cmp.Queries++
			accumulateComparison(&cmp, got, want)
		default:
			// A relation CompareEdges has no truth for at all (a loader
			// bug, or a caller that forgot to filter): count it, never
			// score it.
			cmp.Unadjudicated += len(edges)
		}
	}
	finishComparison(&cmp)
	return cmp
}

func accumulateComparison(cmp *Comparison, got, want map[string]bool) {
	cmp.Predicted += len(got)
	cmp.Truth += len(want)
	for g := range got {
		if want[g] {
			cmp.Correct++
		}
	}
}

func finishComparison(cmp *Comparison) {
	cmp.Precision = rate(cmp.Correct, cmp.Predicted)
	cmp.Recall = rate(cmp.Correct, cmp.Truth)
}

// fileOfSite strips a trailing ":line" from a "file:line" occurrence site.
// A site with no numeric suffix (a bare file path) is returned unchanged.
func fileOfSite(site string) string {
	i := strings.LastIndex(site, ":")
	if i <= 0 || !isAllDigits(site[i+1:]) {
		return site
	}
	return site[:i]
}

func isAllDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}
