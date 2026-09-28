package langbench

// xref.go — LB-F30 (owner decision D4, 2026-09-28): tool-format L3,
// xref_precision/xref_recall, from a tool oracle's cross_references
// against the doc engine's resolved cross-file references
// (fusion.ResolveDocXrefs — the edges ingest stores).

// Raw counts and the oracle family for cross-file references.
const (
	CntXrefTruth        = "xref_truth"
	CntXrefTruthFound   = "xref_truth_found"
	CntXrefFathom       = "xref_fathom"
	CntXrefFathomInTrue = "xref_fathom_in_truth"

	FamilyXref = "xref"
)

// XrefPair is one cross-file reference: the referring entity (file,
// name) and the referenced one. To is "" when compared at file
// granularity.
type XrefPair struct {
	Path, From, ToPath, To string
}

// ScoreXrefs counts truth against found into counts. Names compare whole
// (a tool format's names are identities). When any truth pair leaves To
// empty the oracle names files, not entities, and both sides are compared
// at file granularity (To dropped). Duplicates collapse.
func ScoreXrefs(truth, found []XrefPair, counts map[string]int) {
	fileGrain := false
	for _, p := range truth {
		if p.To == "" {
			fileGrain = true
		}
	}
	norm := func(ps []XrefPair) map[XrefPair]bool {
		out := map[XrefPair]bool{}
		for _, p := range ps {
			if fileGrain {
				p.To = ""
			}
			out[p] = true
		}
		return out
	}
	t, f := norm(truth), norm(found)
	counts[CntXrefTruth] += len(t)
	counts[CntXrefFathom] += len(f)
	for p := range t {
		if f[p] {
			counts[CntXrefTruthFound]++
		}
	}
	for p := range f {
		if t[p] {
			counts[CntXrefFathomInTrue]++
		}
	}
}
