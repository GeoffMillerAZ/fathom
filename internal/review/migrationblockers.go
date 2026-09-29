package review

// migrationblockers.go — CX-11 (design §4, U9): "blockers: sites in files
// governed against the change, or in generated code". migrationBlockers
// is the same leaf-package shape as riskBand (band.go): the caller already
// resolved WHICH (repository, file) pairs are governed against this
// migration's change (an obligations rule scoped to that file, forbidding
// it — this package evaluates no rule itself) and which are generated
// code (whatever convention the repository marks generated files with —
// this package inspects no file content); migrationBlockers only matches
// those facts against MigrationSites' own remaining sites and orders the
// result.
import "sort"

// MigrationBlockReason names why migrationBlockers reported a site.
type MigrationBlockReason string

const (
	BlockedGoverned  MigrationBlockReason = "governed"
	BlockedGenerated MigrationBlockReason = "generated"
)

// GovernedSite is one caller-resolved fact: a rule forbids this
// migration's change from touching (Repository, File). Fleet-scoped
// (Repository), unlike receipt.go's single-diff RuleRef.
type GovernedSite struct {
	Repository string
	File       string
	Rule       string
}

// GeneratedFile marks (Repository, File) as generated code.
type GeneratedFile struct {
	Repository string
	File       string
}

// MigrationBlocker is one remaining site migrationBlockers found blocked.
type MigrationBlocker struct {
	Site   MigrationSite
	Reason MigrationBlockReason
	// Rule names the blocking rule when Reason is BlockedGoverned; empty
	// for BlockedGenerated (generated code is an unconditional block, no
	// rule to cite).
	Rule string
}

type repoFile struct{ repository, file string }

// MigrationBlockers is migrationBlockers' exported face — `fathom refactor
// status --migration` hands it the sites it found and the governed and
// generated facts it resolved per repository (CX-11w).
func MigrationBlockers(sites []MigrationSite, governed []GovernedSite, generated []GeneratedFile) []MigrationBlocker {
	return migrationBlockers(sites, governed, generated)
}

// migrationBlockers reports every REMAINING site (OnTarget == false; a
// Done site is already on To and can never block anything) whose
// (Repository, File) appears in governed or generated, sorted by
// Repository, then File, then Line (determinism contract,
// internal/AGENTS.md). A site named in both reports ONE row with the more
// specific, cited BlockedGoverned reason — never two rows for one site.
func migrationBlockers(sites []MigrationSite, governed []GovernedSite, generated []GeneratedFile) []MigrationBlocker {
	governedRule := make(map[repoFile]string, len(governed))
	for _, g := range governed {
		governedRule[repoFile{g.Repository, g.File}] = g.Rule
	}
	isGenerated := make(map[repoFile]bool, len(generated))
	for _, g := range generated {
		isGenerated[repoFile{g.Repository, g.File}] = true
	}

	var out []MigrationBlocker
	for _, s := range sites {
		if s.OnTarget {
			continue // already on To — cannot block a migration that already happened at this site
		}
		key := repoFile{s.Repository, s.File}
		if rule, ok := governedRule[key]; ok {
			// Governed wins over generated when a site is named in both —
			// the more specific, cited reason (this file's own doc comment).
			out = append(out, MigrationBlocker{Site: s, Reason: BlockedGoverned, Rule: rule})
			continue
		}
		if isGenerated[key] {
			out = append(out, MigrationBlocker{Site: s, Reason: BlockedGenerated})
		}
	}

	sort.Slice(out, func(i, j int) bool {
		if out[i].Site.Repository != out[j].Site.Repository {
			return out[i].Site.Repository < out[j].Site.Repository
		}
		if out[i].Site.File != out[j].Site.File {
			return out[i].Site.File < out[j].Site.File
		}
		return out[i].Site.Line < out[j].Site.Line
	})
	return out
}
