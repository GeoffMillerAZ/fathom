package boundaries

import (
	"cmp"
	"slices"
	"strings"

	"github.com/geoffmilleraz/fathom/pkg/lang/deps"
)

// Verdict is the closed set of outcomes for a rule.
type Verdict string

const (
	VerdictViolated   Verdict = "violated"
	VerdictClean      Verdict = "clean"
	VerdictUnverified Verdict = "unverified"
)

// Result reports the rule's verdict and both kinds of evidence. A known
// violation sets the verdict even when other evidence is unverified.
type Result struct {
	ID           string           `json:"id"`
	Verdict      Verdict          `json:"verdict"`
	MatchedFiles int              `json:"matched_files"`
	Violations   []deps.Edge      `json:"violations"`
	Unverified   []UnverifiedFile `json:"unverified"`
}

// UnverifiedFile records the dependency reader's evidence for a governed file.
// Path is "***" when an invalid file identity prevents establishing scope.
type UnverifiedFile struct {
	Path     string         `json:"path"`
	Dialect  string         `json:"dialect"`
	Measured bool           `json:"measured"`
	Trusted  bool           `json:"trusted"`
	Unknowns []deps.Unknown `json:"unknowns"`
}

// EvaluateBoundaries evaluates direct imports from dependency reader results.
// Results are sorted by rule ID and evidence by file and source line. A rule
// with no governed files is unverified. Malformed file or import identities
// leave affected rules unverified unless a known violation decides the rule.
// Unsafe raw operands are withheld as "***".
func EvaluateBoundaries(rules Rules, files []deps.FileDeps) []Result {
	files, unknownScope := checkedFiles(files)
	results := make([]Result, 0, len(rules.rules))
	for _, r := range rules.rules {
		res := Result{ID: r.id, Verdict: VerdictClean, Violations: []deps.Edge{}, Unverified: []UnverifiedFile{}}
		if unknownScope {
			res.Unverified = append(res.Unverified, UnverifiedFile{Path: withheld, Unknowns: []deps.Unknown{}})
		}
		for _, file := range files {
			if !matches(r.from, "", file.Path) {
				continue
			}
			res.MatchedFiles++
			for _, edge := range file.Edges {
				if r.violates(file.Path, edge) {
					res.Violations = append(res.Violations, edge)
				}
			}
			if !file.Complete {
				res.Unverified = append(res.Unverified, UnverifiedFile{
					Path: file.Path, Dialect: file.Dialect, Measured: file.Measured, Trusted: file.Trusted,
					Unknowns: file.Unknowns,
				})
			}
		}
		switch {
		case len(res.Violations) > 0:
			res.Verdict = VerdictViolated
		case len(res.Unverified) > 0 || res.MatchedFiles == 0:
			res.Verdict = VerdictUnverified
		}
		slices.SortFunc(res.Violations, compareEdges)
		slices.SortFunc(res.Unverified, compareUnverified)
		results = append(results, res)
	}
	return results
}

func (r rule) violates(from string, edge deps.Edge) bool {
	ecosystem, target := "", edge.To
	if edge.Resolution == deps.ResolvedExternal {
		ecosystem, target, _ = strings.Cut(edge.To, ":")
	}
	if matches(r.targets, ecosystem, target) == r.allowOnly {
		return false
	}
	return !slices.ContainsFunc(r.exceptions, func(e exception) bool {
		return matches(e.from, "", from) && matches(e.to, ecosystem, target)
	})
}

func compareEdges(a, b deps.Edge) int {
	return cmp.Or(cmp.Compare(a.From, b.From), cmp.Compare(a.Line, b.Line), cmp.Compare(a.To, b.To),
		cmp.Compare(a.Resolution, b.Resolution), cmp.Compare(a.Raw, b.Raw), cmp.Compare(a.Version, b.Version),
		cmp.Compare(a.Provenance, b.Provenance))
}
