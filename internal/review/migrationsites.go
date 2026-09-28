package review

// migrationsites.go — CX-10 (design §4, U9): "remaining and done sites per
// repository in the fleet, each with its tier (typed or lexical)". A
// MigrationSite is one already-resolved reference to a Migration's From or
// To value — the caller does the fleet-wide lookup (a typed callgraph/
// references query where a typed pass exists, a lexical grep otherwise,
// same "typed where available, lexical otherwise" rule receipt.go's own
// LinkRef.Tier follows) and hands this package the flattened facts; this
// package only groups and orders them (the leaf-package discipline every
// other file in this package already follows).
import "sort"

// SiteTier is the evidence tier a caller resolved a MigrationSite at —
// deliberately just these two words (design §4), distinct from
// LinkTierSpec/LinkTierTyped (a different join, U7's cross-boundary links,
// not U9's migration sites).
type SiteTier string

const (
	SiteTierTyped   SiteTier = "typed"
	SiteTierLexical SiteTier = "lexical"
)

// MigrationSite is one caller-resolved reference to a Migration's From or
// To value, in one repository.
type MigrationSite struct {
	Repository string
	File       string
	Line       int
	Tier       SiteTier
	// OnTarget is true when this site already reads the migration's To
	// value (a "done" site, design §4's "sites already on to"); false
	// means it still reads From (a "remaining" site, "remaining sites of
	// from"). The caller decides this — this package never inspects
	// source text itself.
	OnTarget bool
}

// RepoMigrationSites is one repository's site tally for a Migration:
// Remaining sites still on From and Done sites already on To, each sorted
// by File then Line (determinism contract, internal/CLAUDE.md).
type RepoMigrationSites struct {
	Repository string
	Remaining  []MigrationSite
	Done       []MigrationSite
}

// MigrationSites groups sites by repository and splits each repository's
// sites into Remaining and Done by OnTarget (CX-10). Repositories are
// returned sorted by name; a repository with zero sites is never included
// — there is nothing to report for it. m is accepted (not merely sites)
// so a caller's contract mirrors BuildRiskReceipt's own Input-plus-facts
// shape, and so a future caller can cite which migration these sites
// belong to without re-threading it through every MigrationSite.
func MigrationSites(m Migration, sites []MigrationSite) []RepoMigrationSites {
	_ = m // identifies which migration sites belong to; grouping itself needs only OnTarget

	byRepo := make(map[string][]MigrationSite, len(sites))
	for _, s := range sites {
		byRepo[s.Repository] = append(byRepo[s.Repository], s)
	}

	repos := make([]string, 0, len(byRepo))
	for r := range byRepo {
		repos = append(repos, r)
	}
	sort.Strings(repos)

	out := make([]RepoMigrationSites, 0, len(repos))
	for _, r := range repos {
		var remaining, done []MigrationSite
		for _, s := range byRepo[r] {
			if s.OnTarget {
				done = append(done, s)
			} else {
				remaining = append(remaining, s)
			}
		}
		sortMigrationSites(remaining)
		sortMigrationSites(done)
		out = append(out, RepoMigrationSites{Repository: r, Remaining: remaining, Done: done})
	}
	return out
}

func sortMigrationSites(sites []MigrationSite) {
	sort.Slice(sites, func(i, j int) bool {
		if sites[i].File != sites[j].File {
			return sites[i].File < sites[j].File
		}
		return sites[i].Line < sites[j].Line
	})
}
