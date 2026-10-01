package main

// claim_effectiveness.go — `fathom claim effectiveness` (GL-18, U4): the
// rule-effectiveness view as a verb. It prints exactly the projection
// /rules serves as its JSON section (console.ListRuleEffectiveness), so a
// page and the command that produces it cannot disagree (P10): the store
// the daemon reads, the rule-outcome log the edit hook writes, one clock.
// Read-only: it proposes and never applies — fathom edits no rules file and
// retires no rule from here (GL-X1, GL-X2).

import (
	"context"
	"fmt"
	"io"
	"slices"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/console"
)

// cmdClaimEffectiveness implements `fathom claim effectiveness`.
func cmdClaimEffectiveness(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("claim effectiveness", stderr)
	kf := addKnowledgeFlags(fs)
	service := fs.String("service", "", "only this project's rules (default: every project)")
	class := fs.String("class", console.RuleClassAll, "one of "+strings.Join(console.RuleClassFilters, ", "))
	page := fs.Int("page", 1, fmt.Sprintf("page of %d rules", console.RulesPerPage))
	if err := fs.Parse(reorderFlagsFirst(fs, args)); err != nil {
		return 1
	}
	if !console.ValidRuleClass(*class) {
		fmt.Fprintf(stderr, "error: --class must be one of %s, got %q\n", strings.Join(console.RuleClassFilters, ", "), *class)
		return 1
	}
	return withKnowledgeSource(kf, stderr, func(src console.KnowledgeSource, ks knowledgeStore) error {
		v, err := console.ListRuleEffectiveness(context.Background(), src, readRuleOutcomeSignal(ruleOutcomeLogPath()),
			console.RuleEffectivenessFilter{Service: *service, Class: *class}, *page, ruleEffectivenessNow())
		if err != nil {
			return err
		}
		if *service != "" && !slices.Contains(v.Services, *service) {
			return fmt.Errorf("no project %q in store %s", *service, ks)
		}
		if *kf.json {
			return writeKnowledgeJSON(stdout, v)
		}
		fmt.Fprintf(stdout, "store: %s\nrules: %s  signal: %s  reaches back: %s\n", ks, valueWords(v.Rules), valueWords(v.Signal), valueWords(v.SpanDays))
		if !v.Signal.Known() {
			fmt.Fprintf(stdout, "\nno rule is classified: %s\n", v.Signal.Reason)
		} else {
			for _, c := range v.Classes {
				fmt.Fprintf(stdout, "  %-16s %5d  %s\n", c.Class, c.Count, c.Means)
			}
			if len(v.Remedies) > 0 {
				fmt.Fprintf(stdout, "\nPROPOSALS (nothing is applied; %d in all)\n", v.RemediesTotal)
				for _, r := range v.Remedies {
					fmt.Fprintf(stdout, "%s  %-15s %s\n    %s — %s\n", r.RuleID, r.Class, r.Service, r.Remedy, r.Evidence)
					fmt.Fprintf(stdout, "    %s\n", r.Statement)
				}
			}
			fmt.Fprintf(stdout, "\nRULES — page %d of %d (%d match)\n", v.Page, v.Pages, v.Total)
			for _, r := range v.Rows {
				fmt.Fprintf(stdout, "%s  %-15s served %d top_changed %d still_served %d blocked %d refuted %d (90d served %d)  %s\n    %s\n",
					r.RuleID, r.Class, r.Served30, r.TopChanged30, r.StillServed30, r.Blocked30, r.Refuted30, r.Served90, r.Service, r.Statement)
			}
		}
		for _, n := range v.Notes {
			fmt.Fprintf(stdout, "note: %s\n", n)
		}
		return nil
	})
}
