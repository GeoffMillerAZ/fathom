package conserve

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/certify"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/store"
)

// TestCertificatesPlane_LostDeclarationAndNonLiveState exercises the
// registered "certificates" plane end to end: a certifies marker declared
// at base and gone at head is a lost_declaration finding, and a compiled
// certification whose test file is in the changed set and whose state is
// unproven is a certificate_unproven finding — a live certificate in the
// changed set never appears at all.
func TestCertificatesPlane_LostDeclarationAndNonLiveState(t *testing.T) {
	ctx := context.Background()
	st := store.NewMemoryStore()
	defer st.Close()

	base := map[string]string{
		"pkg/x_test.go": "package pkg\n\n// certifies: invariant I1\nfunc TestI1(t *testing.T) {}\n",
	}
	head := map[string]string{} // TestI1's marker is gone at head

	unprovenMeta, _ := json.Marshal(certify.Meta{
		Test: "TestUnproven", State: string(certify.StateUnproven),
		TestSymbolKey: "svc::pkg/pkg.TestUnproven", TargetKind: "invariant", TargetID: "I2",
		TestFile: "pkg/y_test.go",
	})
	liveMeta, _ := json.Marshal(certify.Meta{
		Test: "TestLive", State: string(certify.StateLive),
		TestSymbolKey: "svc::pkg/pkg.TestLive", TargetKind: "invariant", TargetID: "I3",
		TestFile: "pkg/z_test.go",
	})
	if err := st.PutClaims(ctx, []core.SemanticClaim{
		{
			ClaimID: "c1", ServiceID: "svc", Layer: certify.Layer, LayerVersion: certify.LayerVersion,
			Kind: certify.Kind, Subject: "invariant:I2", Rule: "TestUnproven certifies invariant I2",
			Scope: []string{"pkg/y_test.go"}, Tier: core.TierDocClaimed, Status: core.ClaimFresh, Meta: unprovenMeta,
		},
		{
			ClaimID: "c2", ServiceID: "svc", Layer: certify.Layer, LayerVersion: certify.LayerVersion,
			Kind: certify.Kind, Subject: "invariant:I3", Rule: "TestLive certifies invariant I3",
			Scope: []string{"pkg/z_test.go"}, Tier: core.TierVerified, Status: core.ClaimFresh, Meta: liveMeta,
		},
	}, nil); err != nil {
		t.Fatalf("PutClaims: %v", err)
	}

	in := Input{
		Service: "svc", Base: "base-sha", Head: HeadWorktree, Store: st,
		ChangedFiles: []string{"pkg/x_test.go", "pkg/y_test.go", "pkg/z_test.go"},
		BaseFile:     fileReader(base), HeadFile: fileReader(head),
		Declarations: NewDeclarations(nil, nil),
	}
	rep := Run(ctx, in, []Plane{certificatesPlane{}})
	pr := rep.Planes[0]
	if pr.Verdict != Losses {
		t.Fatalf("verdict = %q, want losses; findings=%+v", pr.Verdict, pr.Findings)
	}

	var sawLost, sawUnproven, sawLive bool
	for _, f := range pr.Findings {
		switch {
		case f.Kind == "lost_declaration" && f.Key == "invariant:I1":
			sawLost = true
		case f.Kind == "certificate_unproven" && f.Key == "invariant:I2":
			sawUnproven = true
		case f.Key == "invariant:I3":
			sawLive = true
		}
	}
	if !sawLost {
		t.Errorf("expected a lost_declaration finding for invariant:I1, got %+v", pr.Findings)
	}
	if !sawUnproven {
		t.Errorf("expected a certificate_unproven finding for invariant:I2, got %+v", pr.Findings)
	}
	if sawLive {
		t.Errorf("a live certificate must never appear as a finding: %+v", pr.Findings)
	}
}

// TestCertificatesPlane_NoStoreIsUnproven pins I44.
func TestCertificatesPlane_NoStoreIsUnproven(t *testing.T) {
	in := Input{
		Service: "svc", ChangedFiles: []string{"pkg/x_test.go"},
		BaseFile: fileReader(nil), HeadFile: fileReader(nil),
	}
	rep := Run(context.Background(), in, []Plane{certificatesPlane{}})
	if rep.Planes[0].Verdict != Unproven {
		t.Fatalf("verdict = %q, want unproven", rep.Planes[0].Verdict)
	}
}

// TestCertificatesPlane_OutOfRangeNonLiveStateIsIgnored pins that a
// non-live certification whose test/reach is NOT in the changed set is
// never reported — this plane only surfaces defects the change itself
// could plausibly have caused.
func TestCertificatesPlane_OutOfRangeNonLiveStateIsIgnored(t *testing.T) {
	ctx := context.Background()
	st := store.NewMemoryStore()
	defer st.Close()

	meta, _ := json.Marshal(certify.Meta{
		Test: "TestFarAway", State: string(certify.StateBroken), TestFile: "unrelated/far_test.go",
	})
	if err := st.PutClaims(ctx, []core.SemanticClaim{{
		ClaimID: "c1", ServiceID: "svc", Layer: certify.Layer, LayerVersion: certify.LayerVersion,
		Kind: certify.Kind, Subject: "invariant:I9", Scope: []string{"unrelated/far.go"},
		Tier: core.TierDocClaimed, Status: core.ClaimFresh, Meta: meta,
	}}, nil); err != nil {
		t.Fatalf("PutClaims: %v", err)
	}

	in := Input{
		Service: "svc", Store: st, ChangedFiles: []string{"pkg/x_test.go"},
		BaseFile: fileReader(nil), HeadFile: fileReader(nil),
		Declarations: NewDeclarations(nil, nil),
	}
	rep := Run(ctx, in, []Plane{certificatesPlane{}})
	if rep.Planes[0].Verdict != Conserved {
		t.Fatalf("verdict = %q, want conserved (out-of-range broken cert is never a finding); findings=%+v", rep.Planes[0].Verdict, rep.Planes[0].Findings)
	}
}
