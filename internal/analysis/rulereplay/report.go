package rulereplay

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"
)

// WriteJSON writes r as indented JSON.
func (r Report) WriteJSON(w io.Writer) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(r)
}

// WriteTable writes r as a short header plus one row per violation,
// grouped in commit order — the CLI's default (non-JSON) rendering. A
// zero-violation replay says so in plain language rather than printing an
// empty table (VB-16's honest-reading requirement: "if nothing is caught
// anywhere, governance stays advisory and the row says so").
func (r Report) WriteTable(w io.Writer) {
	fmt.Fprintf(w, "fathom rules replay — since %s (%s)\n", r.Since, r.RangeSpec)
	fmt.Fprintf(w, "rules evaluated: %s\n", joinOrNone(r.RuleIDs))
	if len(r.SkippedRuleIDs) > 0 {
		fmt.Fprintf(w, "rules skipped (cross_service_direction — needs the fleet's typed edges, not derivable from one repo's git history): %s\n", strings.Join(r.SkippedRuleIDs, ", "))
	}
	fmt.Fprintf(w, "commits: %d   violations: %d   new: %d   elapsed: %dms (%.1fms/commit)\n",
		r.TotalCommits, r.TotalViolations, r.NewViolations, r.ElapsedMS, r.PerCommitAvgMS)

	if r.TotalViolations == 0 {
		fmt.Fprintf(w, "\nclean: no layer-rule violation found in any of the %d replayed commits — governance stays advisory here.\n", r.TotalCommits)
		return
	}

	fmt.Fprintln(w)
	tw := tabwriter.NewWriter(w, 0, 2, 2, ' ', 0)
	fmt.Fprintln(tw, "SHA\tDATE\tRULE\tFROM\tTO\tNEW\tSUBJECT")
	for _, c := range r.Commits {
		for _, v := range c.Violations {
			fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%v\t%s\n",
				shortSHA(c.SHA), c.Date.Format("2006-01-02"), v.RuleID, v.From, v.To, v.New, c.Subject)
		}
	}
	tw.Flush()
}

func joinOrNone(ss []string) string {
	if len(ss) == 0 {
		return "(none configured)"
	}
	return strings.Join(ss, ", ")
}

func shortSHA(sha string) string {
	if len(sha) > 12 {
		return sha[:12]
	}
	return sha
}
