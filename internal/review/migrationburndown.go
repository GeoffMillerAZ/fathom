package review

// migrationburndown.go — CX-12 (design §4, U9): "Burn-down from proof
// history and completion proven by conservation (`gone:` rows proven, no
// unexcused loss)". The caller already ran PE-2's proof-history projection
// (internal/console.ProofHistory, docs/design/progress-and-economics.md
// §3) over the migration's own `gone:` rows (the ratchets `fathom
// refactor plan` writes, CX-9's From retired one package at a time) and
// `fathom conserve` for the conservation verdict — this package only
// folds those already-resolved, per-repository facts into a burn-down
// series and a completion verdict, reusing receipt.go's own RowVerdict/
// RowProven/RowUnproven vocabulary (the same "gone: rows proven" fact U7's
// rows-proven factor already speaks) rather than inventing a second one.
import "sort"

// MigrationBurnDownPoint is one reading of the migration's `gone:` rows'
// claimed/proven counts in one repository (PE-2's PlanReading.Claimed/
// Verified, scoped to just this migration's own rows) — the caller
// supplies these already in chronological order (oldest first); this
// package trusts that order rather than re-deriving a clock of its own
// (the no-I/O, no-clock discipline every file in this package follows).
type MigrationBurnDownPoint struct {
	At      string // RFC 3339 (PE-2's RowReading.At)
	Claimed int    // the migration's gone: rows claimed at this reading
	Proven  int    // ...of those, proven
}

// Remaining is Claimed rows not yet proven at this reading — mirrors
// internal/console.PlanReading.Unproven, never negative.
func (p MigrationBurnDownPoint) Remaining() int {
	if n := p.Claimed - p.Proven; n > 0 {
		return n
	}
	return 0
}

// RepoRows is one repository's `gone:` row verdicts for a migration
// (design §4's "gone: rows proven") plus that repository's `fathom
// conserve` verdict for the same change — mirrors receipt.go's Input
// fields (ClaimedRows/RowVerdict and ConservationUnexcused/
// ConservationDetail), repo-scoped for U9's fleet dimension.
type RepoRows struct {
	Repository string
	Rows       []RowVerdict
	Unexcused  bool
	Detail     string
}

// RepoMigrationBurnDown is one repository's burn-down series for a
// migration, plus whether the migration is COMPLETE there.
type RepoMigrationBurnDown struct {
	Repository string
	// Points is the repository's series (series[Repository] from
	// MigrationBurnDown's own argument), passed through in the caller's
	// order.
	Points []MigrationBurnDownPoint
	// Complete is true only when every one of Rows is RowProven AND
	// Unexcused is false (design §4: "gone: rows proven, no unexcused
	// loss" — both, never either alone).
	Complete bool
	// Reason names why Complete is false; always non-empty when Complete
	// is false, always empty when it is true.
	Reason string
}

// MigrationBurnDown builds the per-repository burn-down and completion
// verdict for m (CX-12). rows is authoritative for which repositories are
// reported (a repository absent from rows has nothing proving it is even
// tracked); series[repo] supplies that repository's burn-down points when
// present, nil otherwise. A repository with zero Rows is reported
// incomplete — nothing proves it finished, so it is never silently marked
// complete. Repositories are returned sorted by name.
func MigrationBurnDown(m Migration, series map[string][]MigrationBurnDownPoint, rows []RepoRows) []RepoMigrationBurnDown {
	_ = m // identifies which migration this burn-down belongs to (parallel to MigrationSites' own m parameter)

	sorted := append([]RepoRows(nil), rows...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Repository < sorted[j].Repository })

	out := make([]RepoMigrationBurnDown, 0, len(sorted))
	for _, r := range sorted {
		complete, reason := migrationRepoComplete(r)
		out = append(out, RepoMigrationBurnDown{
			Repository: r.Repository,
			Points:     series[r.Repository],
			Complete:   complete,
			Reason:     reason,
		})
	}
	return out
}

// migrationRepoComplete applies design §4's rule: every gone: row proven
// AND no unexcused conservation loss.
func migrationRepoComplete(r RepoRows) (complete bool, reason string) {
	if len(r.Rows) == 0 {
		return false, "no gone: rows declared for this migration in " + r.Repository + " — nothing proves it finished"
	}

	sortedRows := append([]RowVerdict(nil), r.Rows...)
	sort.Slice(sortedRows, func(i, j int) bool { return sortedRows[i].ID < sortedRows[j].ID })

	var unproven []string
	for _, rv := range sortedRows {
		if rv.Verdict != RowProven {
			unproven = append(unproven, rv.ID)
		}
	}
	if len(unproven) > 0 {
		reason = "unproven gone: row(s) in " + r.Repository + ": " + joinIDs(unproven)
		if r.Unexcused {
			reason += "; also an unexcused conservation loss"
			if r.Detail != "" {
				reason += " (" + r.Detail + ")"
			}
		}
		return false, reason
	}
	if r.Unexcused {
		reason = "an unexcused conservation loss in " + r.Repository
		if r.Detail != "" {
			reason += ": " + r.Detail
		}
		return false, reason
	}
	return true, ""
}

func joinIDs(ids []string) string {
	out := ids[0]
	for _, id := range ids[1:] {
		out += ", " + id
	}
	return out
}
