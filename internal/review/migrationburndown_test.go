package review

import "testing"

// TestMigrationCompletionByConservation proves MigrationBurnDown (CX-12,
// design §4) marks a repository COMPLETE only when every one of the
// migration's `gone:` rows there is proven AND conservation found no
// unexcused loss — never on proof alone, never on conservation alone, and
// never for a repository with no rows to prove at all.
func TestMigrationCompletionByConservation(t *testing.T) {
	m := Migration{Kind: MigrationSymbol, From: "OldWidget", To: "NewWidget"}

	cases := []struct {
		name         string
		rows         RepoRows
		wantComplete bool
	}{
		{
			name: "all proven, no unexcused loss: complete",
			rows: RepoRows{
				Repository: "alpha",
				Rows:       []RowVerdict{{ID: "cx9-g1", Verdict: RowProven}, {ID: "cx9-gone", Verdict: RowProven}},
				Unexcused:  false,
			},
			wantComplete: true,
		},
		{
			name: "a claimed row unproven: incomplete",
			rows: RepoRows{
				Repository: "beta",
				Rows:       []RowVerdict{{ID: "cx9-g1", Verdict: RowProven}, {ID: "cx9-gone", Verdict: RowUnproven}},
				Unexcused:  false,
			},
			wantComplete: false,
		},
		{
			name: "all proven but an unexcused conservation loss: incomplete",
			rows: RepoRows{
				Repository: "gamma",
				Rows:       []RowVerdict{{ID: "cx9-gone", Verdict: RowProven}},
				Unexcused:  true,
				Detail:     "gamma: OldWidget's test coverage vanished without a surface_change: excuse",
			},
			wantComplete: false,
		},
		{
			name:         "no rows at all: incomplete, never silently complete",
			rows:         RepoRows{Repository: "delta"},
			wantComplete: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := MigrationBurnDown(m, nil, []RepoRows{tc.rows})
			if len(got) != 1 {
				t.Fatalf("MigrationBurnDown returned %d repos, want 1", len(got))
			}
			if got[0].Complete != tc.wantComplete {
				t.Fatalf("Complete = %v, want %v (reason: %q)", got[0].Complete, tc.wantComplete, got[0].Reason)
			}
			if !tc.wantComplete && got[0].Reason == "" {
				t.Errorf("Complete=false must always name a Reason")
			}
			if tc.wantComplete && got[0].Reason != "" {
				t.Errorf("Complete=true carried a non-empty Reason %q", got[0].Reason)
			}
		})
	}
}

// TestMigrationBurnDownSeriesOrderedByRepo proves the burn-down points a
// caller supplies per repository pass through untouched (this package
// trusts the caller's chronological order, matching its own no-clock
// discipline) and that repositories are returned sorted by name.
func TestMigrationBurnDownSeriesOrderedByRepo(t *testing.T) {
	m := Migration{Kind: MigrationImport, From: "old/pkg", To: "new/pkg"}
	series := map[string][]MigrationBurnDownPoint{
		"beta":  {{At: "2026-09-01T00:00:00Z", Claimed: 4, Proven: 1}, {At: "2026-09-08T00:00:00Z", Claimed: 4, Proven: 2}},
		"alpha": {{At: "2026-09-01T00:00:00Z", Claimed: 2, Proven: 2}},
	}
	rows := []RepoRows{
		{Repository: "alpha", Rows: []RowVerdict{{ID: "g1", Verdict: RowProven}}},
		{Repository: "beta", Rows: []RowVerdict{{ID: "g1", Verdict: RowUnproven}}},
	}

	got := MigrationBurnDown(m, series, rows)
	if len(got) != 2 || got[0].Repository != "alpha" || got[1].Repository != "beta" {
		t.Fatalf("got repos = %+v, want [alpha, beta] sorted", got)
	}
	if len(got[1].Points) != 2 || got[1].Points[1].Remaining() != 2 {
		t.Fatalf("beta.Points = %+v, want 2 points, second Remaining()==2 (4 claimed - 2 proven)", got[1].Points)
	}
	if got[0].Points[0].Remaining() != 0 {
		t.Errorf("alpha.Points[0].Remaining() = %d, want 0 (2 claimed, 2 proven)", got[0].Points[0].Remaining())
	}
}
