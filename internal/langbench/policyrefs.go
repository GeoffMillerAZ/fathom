package langbench

// policyrefs.go — LB-F137 (wave pc): policy references scored at a tool
// format's L3. A policy pair is a cross-file reference (a policy, rule,
// exception or constraint entity to the Kubernetes object or declared
// resource its target names), so it is scored into the xref family — the
// union with any cross_references the same subject's oracle scored there —
// and, for disclosure, into its own policy_* counts (routes.go's shape).

// The policy counts a subject carries beside the xref family's.
const (
	CntPolicyTruth        = "policy_truth"
	CntPolicyFathom       = "policy_fathom"
	CntPolicyTruthFound   = "policy_truth_found"
	CntPolicyFathomInTrue = "policy_fathom_in_truth"
	// CntPolicyUnresolvedPrefix + reason counts fathom's unresolved policy
	// references; CntPolicyOracleUnresolvedPrefix + reason the oracle's.
	CntPolicyUnresolvedPrefix       = "policy_unresolved_"
	CntPolicyOracleUnresolvedPrefix = "policy_oracle_unresolved_"
)

// ScorePolicyRefs counts policy truth against found (entity granularity,
// names compared whole, duplicates collapsed) into counts' xref family and
// its policy_* counts.
func ScorePolicyRefs(truth, found []XrefPair, counts map[string]int) {
	own := map[string]int{}
	ScoreXrefs(truth, found, own)
	for xref, policy := range map[string]string{
		CntXrefTruth: CntPolicyTruth, CntXrefFathom: CntPolicyFathom,
		CntXrefTruthFound: CntPolicyTruthFound, CntXrefFathomInTrue: CntPolicyFathomInTrue,
	} {
		counts[xref] += own[xref]
		counts[policy] += own[xref]
	}
}
