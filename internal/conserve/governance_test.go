package conserve

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/store"
)

// TestGovernanceOrphans_ScopeMatchedAtBaseNoneAtHead pins CV-23: a claim
// whose scope glob matched a file at base and matches nothing at head
// (the file was deleted, uncommitted) is orphaned; a claim whose scoped
// file still exists at head is not.
func TestGovernanceOrphans_ScopeMatchedAtBaseNoneAtHead(t *testing.T) {
	repo := newInputTestRepo(t)
	repo.write("pkg/deploy.go", "package pkg\n")
	repo.write("pkg/other.go", "package pkg\n")
	repo.commitAll("base commit")
	base := repo.headSHA()

	// Uncommitted deletion of pkg/deploy.go — HeadWorktree must see it as
	// gone even though it is still in the git index (governance.go's own
	// tracked-minus-`ls-files --deleted` reconstruction).
	if err := os.Remove(filepath.Join(repo.root, "pkg", "deploy.go")); err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	st := store.NewMemoryStore()
	defer st.Close()
	if err := st.PutClaims(ctx, []core.SemanticClaim{
		{
			ClaimID: "c1", ServiceID: "svc", Layer: "obligations", LayerVersion: "v1", Kind: "obligation",
			Subject: "CLAUDE.md#deploy", Rule: "deploy = task deploy", Scope: []string{"pkg/deploy.go"},
			Tier: core.TierDocClaimed, Status: core.ClaimFresh,
		},
		{
			ClaimID: "c2", ServiceID: "svc", Layer: "obligations", LayerVersion: "v1", Kind: "obligation",
			Subject: "CLAUDE.md#other", Rule: "other = task other", Scope: []string{"pkg/other.go"},
			Tier: core.TierDocClaimed, Status: core.ClaimFresh,
		},
		{
			// A layer NOT in governanceLayers — must never be considered.
			ClaimID: "c3", ServiceID: "svc", Layer: "certifications", LayerVersion: "v1", Kind: "certification",
			Subject: "invariant:I1", Scope: []string{"pkg/deploy.go"}, Tier: core.TierDocClaimed, Status: core.ClaimFresh,
		},
	}, nil); err != nil {
		t.Fatalf("PutClaims: %v", err)
	}
	if err := st.SetServiceMeta(ctx, store.ServiceMeta{ServiceID: "svc", Path: repo.root}); err != nil {
		t.Fatalf("SetServiceMeta: %v", err)
	}

	in := Input{
		Service: "svc", Root: repo.root, Base: base, Head: HeadWorktree, Store: st,
		Declarations: NewDeclarations(nil, nil),
	}
	findings, missing := GovernanceOrphans(ctx, in)
	if len(missing) != 0 {
		t.Fatalf("missing = %v, want none", missing)
	}
	if len(findings) != 1 {
		t.Fatalf("Findings = %+v, want exactly one (obligations:CLAUDE.md#deploy)", findings)
	}
	f := findings[0]
	if f.Kind != "orphaned" || f.Key != "obligations:CLAUDE.md#deploy" {
		t.Errorf("finding = %+v, want {orphaned obligations:CLAUDE.md#deploy}", f)
	}

	// The registered "governance" plane wraps GovernanceOrphans byte-for-
	// byte via Run — proven separately so this test's own symbol stays
	// GovernanceOrphans itself (CV-23's contract), not the plane wrapper.
	rep := Run(ctx, in, []Plane{governancePlane{}})
	if rep.Planes[0].Verdict != Losses {
		t.Fatalf("plane verdict = %q, want losses; findings=%+v", rep.Planes[0].Verdict, rep.Planes[0].Findings)
	}
}

// TestGovernanceOrphans_MissingInputs pins I44: no store, no root, and no
// base each independently make the plane unproven.
func TestGovernanceOrphans_MissingInputs(t *testing.T) {
	cases := []struct {
		name string
		in   Input
	}{
		{"no store", Input{Root: "/tmp/x", Base: "abc"}},
		{"no root", Input{Store: store.NewMemoryStore(), Base: "abc"}},
		{"no base", Input{Store: store.NewMemoryStore(), Root: "/tmp/x"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rep := Run(context.Background(), tc.in, []Plane{governancePlane{}})
			if rep.Planes[0].Verdict != Unproven {
				t.Errorf("verdict = %q, want unproven", rep.Planes[0].Verdict)
			}
		})
	}
}

// TestGovernanceOrphans_DeclaredExcusesIt pins I45's own excusing rule
// applied via this plane: a prior retires:/surface_change: declaration
// for the orphaned key excuses it (conserved), a self-declared one does
// not (losses).
func TestGovernanceOrphans_DeclaredExcusesIt(t *testing.T) {
	repo := newInputTestRepo(t)
	repo.write("pkg/deploy.go", "package pkg\n")
	repo.commitAll("base commit")
	base := repo.headSHA()
	if err := os.Remove(filepath.Join(repo.root, "pkg", "deploy.go")); err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	st := store.NewMemoryStore()
	defer st.Close()
	if err := st.PutClaims(ctx, []core.SemanticClaim{{
		ClaimID: "c1", ServiceID: "svc", Layer: "obligations", LayerVersion: "v1", Kind: "obligation",
		Subject: "CLAUDE.md#deploy", Scope: []string{"pkg/deploy.go"},
		Tier: core.TierDocClaimed, Status: core.ClaimFresh,
	}}, nil); err != nil {
		t.Fatalf("PutClaims: %v", err)
	}

	in := Input{
		Service: "svc", Root: repo.root, Base: base, Head: HeadWorktree, Store: st,
		Declarations: declaredPrior("obligations:CLAUDE.md#deploy"),
	}
	rep := Run(ctx, in, []Plane{governancePlane{}})
	if rep.Planes[0].Verdict != Conserved {
		t.Fatalf("verdict = %q, want conserved; findings=%+v", rep.Planes[0].Verdict, rep.Planes[0].Findings)
	}
}

func declaredPrior(key string) Declarations {
	return Declarations{base: []string{key}}
}
