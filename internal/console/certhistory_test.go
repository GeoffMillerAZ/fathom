package console

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/certify"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/store"
)

// WI-20/WI-21: the page's input is the store's own compiled claims plus
// each test's attested runs, replayed through the ladder.
func TestCertHistoriesFeedRegressions(t *testing.T) {
	ctx := context.Background()
	st := store.NewMemoryStore()
	mk := func(kind, target, test string) core.SemanticClaim {
		meta, _ := json.Marshal(certify.Meta{Test: test, TestSymbolKey: "k:" + test, TargetKind: kind, TargetID: target, State: "broken"})
		return core.SemanticClaim{ClaimID: core.ClaimID(certify.Layer, certify.Kind, kind+":"+target, "k:"+test), ServiceID: "svc",
			Layer: certify.Layer, Kind: certify.Kind, Subject: kind + ":" + target, Tier: core.TierPreScan, Status: core.ClaimFresh, Meta: meta}
	}
	if err := st.PutClaims(ctx, []core.SemanticClaim{mk("spec", "S1", "TestS"), mk("invariant", "I1", "TestI")}, nil); err != nil {
		t.Fatal(err)
	}
	run := func(digest, at, commit, status, test string) store.TestRun {
		return store.TestRun{ServiceID: "svc", RunDigest: digest, Component: "p", Suite: "unit", StartedAt: at, Verdict: status, Commit: commit,
			Cases: []store.TestCase{{CaseID: "1", Name: test, Status: status, Attempts: 1, TestSymbolKey: "k:" + test}}}
	}
	var runs []store.TestRun
	for _, test := range []string{"TestS", "TestI"} {
		runs = append(runs, run("a"+test, "2026-09-01T00:00:00Z", "c1", "pass", test), run("b"+test, "2026-09-02T00:00:00Z", "c2", "fail", test))
	}
	if err := st.PutTestRuns(ctx, "svc", runs); err != nil {
		t.Fatal(err)
	}
	certs, err := CertHistoriesFromStore(ctx, st, "svc")
	if err != nil || len(certs) != 2 || certs[0].Kind != "invariant" || !certs[0].Reached || len(certs[0].Runs) != 2 {
		t.Fatalf("histories wrong: %v %+v", err, certs)
	}
	all := AllProofRegressions(History{}, HistoryFilter{}, certs, nil)
	if len(all) != 2 || all[0].ChangedCommit != "c2" {
		t.Fatalf("want a certificate and an invariant regression at c2: %+v", all)
	}
	kinds := all[0].Kind + "," + all[1].Kind
	if kinds != "certificate,invariant" && kinds != "invariant,certificate" {
		t.Fatalf("kinds wrong: %s", kinds)
	}
}
