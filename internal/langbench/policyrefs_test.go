package langbench

import "testing"

// TestScorePolicyRefs (LB-F137): policy pairs add to the xref family (the
// union with cross_references already counted there) and to their own
// policy_* counts; duplicates collapse, a pair the oracle lacks lowers
// precision, a pair fathom misses lowers recall.
func TestScorePolicyRefs(t *testing.T) {
	counts := map[string]int{CntXrefTruth: 2, CntXrefFathom: 2, CntXrefTruthFound: 2, CntXrefFathomInTrue: 2}
	truth := []XrefPair{
		{Path: "p.yaml", From: "r", ToPath: "d.yaml", To: "Deployment/web"},
		{Path: "p.yaml", From: "r", ToPath: "d.yaml", To: "Deployment/api"},
		{Path: "p.yaml", From: "r", ToPath: "d.yaml", To: "Deployment/api"},
	}
	found := []XrefPair{
		{Path: "p.yaml", From: "r", ToPath: "d.yaml", To: "Deployment/web"},
		{Path: "p.yaml", From: "r", ToPath: "d.yaml", To: "Pod/stray"},
	}
	ScorePolicyRefs(truth, found, counts)
	for k, want := range map[string]int{
		CntPolicyTruth: 2, CntPolicyFathom: 2, CntPolicyTruthFound: 1, CntPolicyFathomInTrue: 1,
		CntXrefTruth: 4, CntXrefFathom: 4, CntXrefTruthFound: 3, CntXrefFathomInTrue: 3,
	} {
		if counts[k] != want {
			t.Errorf("%s = %d, want %d", k, counts[k], want)
		}
	}
}
