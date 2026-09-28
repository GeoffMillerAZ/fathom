package review

import "testing"

// TestMigrationSitesPerRepo proves MigrationSites (CX-10, design §4)
// groups sites by repository, splits each repository's sites into
// Remaining (still on From) and Done (already on To), carries each site's
// tier, orders sites (File then Line) within a repo, and orders
// repositories by name — deterministically, regardless of input order.
func TestMigrationSitesPerRepo(t *testing.T) {
	m := Migration{Kind: MigrationSymbol, From: "OldWidget", To: "NewWidget"}

	sites := []MigrationSite{
		{Repository: "beta", File: "pkg/b.go", Line: 10, Tier: SiteTierLexical, OnTarget: false},
		{Repository: "alpha", File: "pkg/z.go", Line: 5, Tier: SiteTierTyped, OnTarget: false},
		{Repository: "alpha", File: "pkg/a.go", Line: 20, Tier: SiteTierTyped, OnTarget: false},
		{Repository: "alpha", File: "pkg/a.go", Line: 3, Tier: SiteTierLexical, OnTarget: true},
	}

	got := MigrationSites(m, sites)

	if len(got) != 2 {
		t.Fatalf("MigrationSites returned %d repos, want 2 (alpha, beta)", len(got))
	}
	if got[0].Repository != "alpha" || got[1].Repository != "beta" {
		t.Fatalf("repos = [%s, %s], want alphabetical [alpha, beta]", got[0].Repository, got[1].Repository)
	}

	alpha := got[0]
	if len(alpha.Remaining) != 2 {
		t.Fatalf("alpha.Remaining = %d, want 2", len(alpha.Remaining))
	}
	if alpha.Remaining[0].File != "pkg/a.go" || alpha.Remaining[0].Line != 20 {
		t.Errorf("alpha.Remaining[0] = %+v, want pkg/a.go:20 (File then Line order)", alpha.Remaining[0])
	}
	if alpha.Remaining[1].File != "pkg/z.go" {
		t.Errorf("alpha.Remaining[1] = %+v, want pkg/z.go", alpha.Remaining[1])
	}
	if len(alpha.Done) != 1 || alpha.Done[0].Tier != SiteTierLexical {
		t.Fatalf("alpha.Done = %+v, want one lexical-tier done site", alpha.Done)
	}

	beta := got[1]
	if len(beta.Remaining) != 1 || beta.Remaining[0].Tier != SiteTierLexical {
		t.Fatalf("beta.Remaining = %+v, want one lexical-tier remaining site", beta.Remaining)
	}
	if len(beta.Done) != 0 {
		t.Fatalf("beta.Done = %+v, want none", beta.Done)
	}
}

// TestMigrationSitesEmptyRepoOmitted proves a repository with zero sites is
// never reported — there is nothing to show — and that MigrationSites
// never panics or fabricates a row for an empty input.
func TestMigrationSitesEmptyRepoOmitted(t *testing.T) {
	m := Migration{Kind: MigrationImport, From: "old/pkg", To: "new/pkg"}
	if got := MigrationSites(m, nil); len(got) != 0 {
		t.Fatalf("MigrationSites(nil) = %+v, want empty", got)
	}
}

// TestMigrationSitesDeterministic proves the determinism contract: the
// same sites in a different input order produce byte-identical results.
func TestMigrationSitesDeterministic(t *testing.T) {
	m := Migration{Kind: MigrationSymbol, From: "OldWidget", To: "NewWidget"}
	a := []MigrationSite{
		{Repository: "beta", File: "pkg/b.go", Line: 10, Tier: SiteTierLexical},
		{Repository: "alpha", File: "pkg/z.go", Line: 5, Tier: SiteTierTyped},
		{Repository: "alpha", File: "pkg/a.go", Line: 20, Tier: SiteTierTyped},
	}
	b := []MigrationSite{a[2], a[0], a[1]}

	gotA := MigrationSites(m, a)
	gotB := MigrationSites(m, b)

	if len(gotA) != len(gotB) {
		t.Fatalf("len(gotA)=%d len(gotB)=%d, want equal", len(gotA), len(gotB))
	}
	for i := range gotA {
		if gotA[i].Repository != gotB[i].Repository {
			t.Fatalf("repo[%d]: %q != %q", i, gotA[i].Repository, gotB[i].Repository)
		}
		if len(gotA[i].Remaining) != len(gotB[i].Remaining) {
			t.Fatalf("repo %s: Remaining lengths differ: %d vs %d", gotA[i].Repository, len(gotA[i].Remaining), len(gotB[i].Remaining))
		}
		for j := range gotA[i].Remaining {
			if gotA[i].Remaining[j] != gotB[i].Remaining[j] {
				t.Fatalf("repo %s Remaining[%d]: %+v != %+v", gotA[i].Repository, j, gotA[i].Remaining[j], gotB[i].Remaining[j])
			}
		}
	}
}
