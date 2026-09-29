package main

// refactor_status_burndown.go — CX-12w (design §4, U9): `fathom refactor
// status --migration` prints, per repository, the burn-down and the
// completion verdict from review.MigrationBurnDown (CX-12). The library
// folds resolved facts; this file resolves them:
//
//   - rows: the migration's `gone:` rows — the rows of the repository's own
//     docs/plans/*.md whose `gone:` contract names the migration's From (a
//     `callers` or `pattern` row for a symbol, `import` or `pattern` for an
//     import, `module` or `pattern` for a pkg, `pattern` for a route or a
//     config key). A row is proven when the verify ledger's newest reading
//     of it says claimed-proven, and unproven otherwise, a row the ledger
//     never read included.
//   - series: those rows' verdicts over time, from the same ledger through
//     console.ProofHistory (a projection of it, never a second ledger): at
//     each reading where one of them changed, how many are claimed and how
//     many proven.
//   - conservation: run per repository, over base..worktree, only when
//     --conserve-since names the base; without it the verdict is never
//     complete, and says so.

import (
	"context"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/conserve"
	"github.com/geoffmilleraz/fathom/internal/console"
	"github.com/geoffmilleraz/fathom/internal/review"
	"github.com/geoffmilleraz/fathom/internal/store"
	"github.com/geoffmilleraz/fathom/internal/verify"
)

// migrationGoneRow is one plan row whose gone: contract names the
// migration's From.
type migrationGoneRow struct {
	Plan    string // repo-relative, slash-separated
	ID      string
	Claimed bool
}

// migrationGoneRows reads root's docs/plans/*.md (non-recursive, the set the
// gate walks) and returns the rows whose gone: contract names m.From, sorted
// by plan then id. An absent docs/plans is no rows, not an error.
func migrationGoneRows(root string, m review.Migration) []migrationGoneRow {
	var out []migrationGoneRow
	for _, file := range discoverPlanPaths(root) {
		body, err := os.ReadFile(file)
		if err != nil {
			continue
		}
		rel := filepath.ToSlash(strings.TrimPrefix(file, filepath.Clean(root)+string(filepath.Separator)))
		for _, t := range verify.ParseMarkdownPlan(rel, body).Tasks {
			if t.Gone != "" && t.ID != "" && goneNamesFrom(t.Gone, m) {
				out = append(out, migrationGoneRow{Plan: rel, ID: t.ID, Claimed: t.Claimed})
			}
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Plan != out[j].Plan {
			return out[i].Plan < out[j].Plan
		}
		return out[i].ID < out[j].ID
	})
	return out
}

// goneNamesFrom reports whether a gone: contract (its raw value) names m's
// From. Each migration kind is retired by the contract forms that can say
// so: an exact name for callers, import and module, and for a pattern a
// regexp that finds From (or its quoted form, the way `fathom refactor plan`
// seeds import ratchets).
func goneNamesFrom(raw string, m review.Migration) bool {
	form, arg, ok := splitGoneContract(raw)
	if !ok {
		return false
	}
	switch form {
	case "callers":
		return m.Kind == review.MigrationSymbol && bareSymbolName(arg) == bareSymbolName(m.From)
	case "import":
		return m.Kind == review.MigrationImport && arg == m.From
	case "module":
		return m.Kind == review.MigrationPkg && (arg == m.From || arg == strings.SplitN(m.From, "@", 2)[0])
	case "pattern":
		re, err := regexp.Compile("(?m)" + arg)
		if err != nil {
			return false
		}
		candidates := []string{m.From, `"` + m.From + `"`}
		if m.Kind == review.MigrationSymbol {
			candidates = append(candidates, bareSymbolName(m.From))
		}
		for _, c := range candidates {
			if re.MatchString(c) {
				return true
			}
		}
	}
	return false
}

// splitGoneContract splits a gone: value into its form (import, module,
// pattern or callers) and argument; a pattern's is unquoted. It mirrors
// internal/verify's parser, which is unexported, at this size
// (goneRatchetPattern's precedent).
func splitGoneContract(raw string) (form, arg string, ok bool) {
	raw = strings.TrimSpace(raw)
	for _, f := range []string{"import", "module", "pattern", "callers"} {
		if rest, cut := strings.CutPrefix(raw, f+" "); cut {
			arg = strings.TrimSpace(rest)
			if f == "pattern" && len(arg) >= 2 && arg[0] == '"' && arg[len(arg)-1] == '"' {
				arg = arg[1 : len(arg)-1]
			}
			return f, arg, arg != ""
		}
	}
	return "", "", false
}

// migrationLedgerRows reads rows' verdicts and their series out of the proof
// history h for service svc. Each row's verdict is its newest reading
// (proven only when that reads claimed-proven); a row the ledger never read
// is unproven. The series has a point at every reading where any of the rows
// changed, counting the rows then claimed (proven, unproven or unknown) and
// the ones proven, in time order, a point equal to the last dropped.
func migrationLedgerRows(h console.History, svc string, rows []migrationGoneRow) ([]review.RowVerdict, []review.MigrationBurnDownPoint) {
	ids := map[string]int{}
	for _, r := range rows {
		ids[r.ID]++
	}
	type tracked struct {
		hist console.RowHistory
		seen bool
	}
	track := make([]tracked, len(rows))
	verdicts := make([]review.RowVerdict, len(rows))
	times := map[string]bool{}
	for i, r := range rows {
		for _, rh := range h.Rows {
			if strings.EqualFold(rh.Service, svc) && rh.ID == r.ID && rh.Plan == r.Plan {
				track[i] = tracked{hist: rh, seen: true}
				for _, rd := range rh.Readings {
					times[rd.At] = true
				}
				break
			}
		}
		id := r.ID
		if ids[r.ID] > 1 {
			id = path.Base(r.Plan) + "#" + r.ID
		}
		verdict := review.RowUnproven
		if track[i].seen && track[i].hist.Latest().State == console.HistProven {
			verdict = review.RowProven
		}
		verdicts[i] = review.RowVerdict{ID: id, Verdict: verdict}
	}

	ats := make([]string, 0, len(times))
	for at := range times {
		ats = append(ats, at)
	}
	sort.Strings(ats)
	var points []review.MigrationBurnDownPoint
	for _, at := range ats {
		p := review.MigrationBurnDownPoint{At: at}
		for _, t := range track {
			if !t.seen {
				continue
			}
			st, ok := t.hist.StateAt(at)
			if !ok {
				continue
			}
			switch st.State {
			case console.HistProven:
				p.Claimed++
				p.Proven++
			case console.HistUnproven, console.HistUnknown:
				p.Claimed++
			}
		}
		if n := len(points); n > 0 && points[n-1].Claimed == p.Claimed && points[n-1].Proven == p.Proven {
			continue
		}
		points = append(points, p)
	}
	return verdicts, points
}

// migrationConservationFacts folds a conservation report into RepoRows'
// facts: a plane whose verdict is losses is an unexcused loss (detail names
// its first finding), one whose verdict is unproven leaves the conservation
// verdict unestablished (I44: a missing input is never "conserved").
func migrationConservationFacts(rep conserve.Report) (unexcused bool, detail, unproven string) {
	var losses, missing []string
	for _, p := range rep.Planes {
		switch p.Verdict {
		case conserve.Losses:
			unexcused = true
			first := ""
			for _, f := range p.Findings {
				if f.Declared != conserve.DeclaredPrior {
					first = f.Detail
					break
				}
			}
			losses = append(losses, strings.TrimSuffix(p.Plane+": "+first, ": "))
		case conserve.Unproven:
			missing = append(missing, fmt.Sprintf("plane %s unproven (missing %s)", p.Plane, strings.Join(p.Missing, ", ")))
		}
	}
	return unexcused, strings.Join(losses, "; "), strings.Join(missing, "; ")
}

// migrationConservation runs `fathom conserve --since` for one repository,
// through the same conserve.NewInput and planes the verb runs. A run that
// cannot be made leaves the verdict unestablished, named.
func migrationConservation(ctx context.Context, st store.Store, svc, root, since string) (unexcused bool, detail, unproven string) {
	in, err := conserve.NewInput(ctx, root, since, st, svc, discoverPlanPaths(root))
	if err != nil {
		return false, "", fmt.Sprintf("conserve --since %s could not run: %v", since, err)
	}
	return migrationConservationFacts(conserve.Run(ctx, in, conserve.Planes()))
}
