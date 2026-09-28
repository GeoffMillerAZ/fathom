package review

import "testing"

// TestMigrationBlockers proves migrationBlockers (CX-11, design §4) finds
// every remaining site whose file is governed against the change or is
// generated code, ignores done sites entirely (they cannot block
// anything, design §4: blockers apply to REMAINING sites of `from`), and
// returns its result sorted (Repository, File, Line) for determinism.
func TestMigrationBlockers(t *testing.T) {
	sites := []MigrationSite{
		{Repository: "alpha", File: "pkg/a.go", Line: 10, Tier: SiteTierTyped, OnTarget: false},   // governed
		{Repository: "alpha", File: "gen/zz.go", Line: 3, Tier: SiteTierTyped, OnTarget: false},   // generated
		{Repository: "alpha", File: "pkg/free.go", Line: 1, Tier: SiteTierTyped, OnTarget: false}, // neither: not a blocker
		{Repository: "beta", File: "pkg/b.go", Line: 5, Tier: SiteTierLexical, OnTarget: true},    // done: never a blocker, even though governed below
	}
	governed := []GovernedSite{
		{Repository: "alpha", File: "pkg/a.go", Rule: "no-touch-frozen-api"},
		{Repository: "beta", File: "pkg/b.go", Rule: "no-touch-frozen-api"},
	}
	generated := []GeneratedFile{
		{Repository: "alpha", File: "gen/zz.go"},
	}

	got := migrationBlockers(sites, governed, generated)

	if len(got) != 2 {
		t.Fatalf("migrationBlockers returned %d blockers, want 2 (got %+v)", len(got), got)
	}
	if got[0].Site.File != "gen/zz.go" || got[0].Reason != BlockedGenerated {
		t.Errorf("got[0] = %+v, want gen/zz.go BlockedGenerated (sorted by File before pkg/a.go)", got[0])
	}
	if got[1].Site.File != "pkg/a.go" || got[1].Reason != BlockedGoverned || got[1].Rule != "no-touch-frozen-api" {
		t.Errorf("got[1] = %+v, want pkg/a.go BlockedGoverned citing no-touch-frozen-api", got[1])
	}
}

// TestMigrationBlockersGovernedWinsOverGenerated proves a site named in
// BOTH governed and generated reports exactly one blocker row, with the
// more specific, cited BlockedGoverned reason — never two rows for one
// site.
func TestMigrationBlockersGovernedWinsOverGenerated(t *testing.T) {
	sites := []MigrationSite{
		{Repository: "alpha", File: "gen/both.go", Line: 1, OnTarget: false},
	}
	governed := []GovernedSite{{Repository: "alpha", File: "gen/both.go", Rule: "frozen"}}
	generated := []GeneratedFile{{Repository: "alpha", File: "gen/both.go"}}

	got := migrationBlockers(sites, governed, generated)
	if len(got) != 1 {
		t.Fatalf("migrationBlockers returned %d blockers, want exactly 1", len(got))
	}
	if got[0].Reason != BlockedGoverned || got[0].Rule != "frozen" {
		t.Fatalf("got[0] = %+v, want BlockedGoverned citing frozen", got[0])
	}
}

// TestMigrationBlockersNone proves no false positives: a site whose file
// is neither governed nor generated is never reported.
func TestMigrationBlockersNone(t *testing.T) {
	sites := []MigrationSite{{Repository: "alpha", File: "pkg/free.go", Line: 1, OnTarget: false}}
	if got := migrationBlockers(sites, nil, nil); len(got) != 0 {
		t.Fatalf("migrationBlockers = %+v, want none", got)
	}
}
