package query_test

import (
	"context"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/query"
)

// TestQuery_RankingGolden pins the exact output order of the keyword+tier
// ranking (scoreAndRank: keywordOverlap*10 + tierRank, ties broken by name
// then file) across a fixed, deliberately mixed input. ADR-041 decision 2
// deleted the vector/hybrid ranking path; this golden is the receipt that
// removing it left the surviving keyword+tier path byte-identical.
func TestQuery_RankingGolden(t *testing.T) {
	obs := []core.Observation{
		// Two keyword hits, verified tier — should rank first.
		{ServiceID: "svc", ConcernID: "c", Symbol: "ValidateToken", Claim: "validates auth token", EvidenceTier: core.TierVerified, SourceRef: "auth.go", IngestedAt: epoch},
		// One keyword hit, verified tier.
		{ServiceID: "svc", ConcernID: "c", Symbol: "RefreshToken", Claim: "refreshes token only", EvidenceTier: core.TierVerified, SourceRef: "auth.go", IngestedAt: epoch},
		// Two keyword hits, doc-claimed tier — beaten by verified single-match on tier? No:
		// keywordScore*10 dominates tierRank, so this still outranks a lower keyword count.
		{ServiceID: "svc", ConcernID: "c", Symbol: "CheckAuthToken", Claim: "auth token check", EvidenceTier: core.TierDocClaimed, SourceRef: "docs.md", IngestedAt: epoch},
		// Zero keyword hits, verified tier — ranks by tier alone among zero-matches.
		{ServiceID: "svc", ConcernID: "c", Symbol: "Unrelated", Claim: "unrelated content", EvidenceTier: core.TierVerified, SourceRef: "x.go", IngestedAt: epoch},
		// Zero keyword hits, pre-scan tier.
		{ServiceID: "svc", ConcernID: "c", Symbol: "AlsoUnrelated", Claim: "also unrelated", EvidenceTier: core.TierPreScan, SourceRef: "y.go", IngestedAt: epoch},
		// Same keyword score and tier as ValidateToken's family member, tie broken by name.
		{ServiceID: "svc", ConcernID: "c", Symbol: "AardvarkToken", Claim: "auth token handling", EvidenceTier: core.TierVerified, SourceRef: "z.go", IngestedAt: epoch},
	}
	ms := newStore(t, obs)
	e := query.New(ms)

	result, err := e.Query(context.Background(), core.Query{
		ServiceID: "svc",
		ConcernID: "c",
		Keywords:  []string{"auth", "token"},
		TopN:      10,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// keywordScore*10 + tierRank, ties by name asc then file asc:
	//   ValidateToken:   2*10 + verified   -> highest keyword count, verified
	//   AardvarkToken:   2*10 + verified   -> same score as ValidateToken, "Aardvark" < "Validate"
	//   CheckAuthToken:  2*10 + docClaimed -> same keyword count, lower tier
	//   RefreshToken:    1*10 + verified   -> single keyword hit
	//   Unrelated:       0*10 + verified   -> zero keyword hits, higher tier than AlsoUnrelated
	//   AlsoUnrelated:   0*10 + preScan
	want := []string{
		"AardvarkToken",
		"ValidateToken",
		"CheckAuthToken",
		"RefreshToken",
		"Unrelated",
		"AlsoUnrelated",
	}
	got := make([]string, len(result.Nodes))
	for i, n := range result.Nodes {
		got[i] = n.Name
	}
	if len(got) != len(want) {
		t.Fatalf("node count = %d, want %d; got order %v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("position %d: got %q, want %q (full order got=%v want=%v)", i, got[i], want[i], got, want)
		}
	}
}
