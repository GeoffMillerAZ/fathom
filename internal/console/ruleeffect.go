package console

// ruleeffect.go — GL-18 (docs/plans/governance-learning.md, design doc
// docs/design/governance-learning.md §5, U4): the rule-effectiveness view.
// A rule served a hundred times and violated on the hundred and first edit
// carries a signal the edit hook records (GL-10) and semantic.
// RuleEffectiveness classifies (GL-11): ignored, never triggered, effective
// or contested, each with a proposed remedy (GL-19) that nothing here — or
// anywhere — applies (GL-X1/GL-X2). `fathom claim effectiveness --json`
// prints exactly this; /rules renders it.
//
// The view does not read the signal log itself: the caller hands it a
// RuleSignal, so the log's path, its bounded read and its failure are the
// caller's (cmd/fathom owns the writer and the reader) and this package
// stays free of file layout. What it owns is honesty about the signal:
//
//   - no log, or one that could not be read, means no rule is classified —
//     an unmeasured rule is not a rule that never triggered (I38), so the
//     rows are empty and the signal value names why;
//   - "never triggered" is a claim about 90 days, so it is not judged until
//     the log reaches back that far; the rules that would qualify are
//     counted and named, and none is proposed for pruning early;
//   - a ratio needs at least semantic.RuleEffectivenessMinServed served
//     events in the window, and below it the row shows its n and no rate.

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/semantic"
)

// RuleSignal is the rule-outcome log as the projection reads it.
type RuleSignal struct {
	// State is present (the log was read), absent (there is no log yet) or
	// error (it exists and could not be read); Reason names why for the last
	// two.
	State  State
	Reason string
	// Source names where it was read from.
	Source string
	Events []semantic.RuleOutcomeEvent
	// Skipped counts log lines the reader could not parse; the view names
	// them beside the counts they are missing from.
	Skipped int
}

// Rule class filters: one of design doc §5's classes, the rules no class
// applies to yet, or every rule.
const (
	RuleClassAll          = "all"
	RuleClassUnclassified = "unclassified"
)

// RuleClassFilters is every class filter, most urgent first.
var RuleClassFilters = []string{
	RuleClassAll,
	string(semantic.RuleClassIgnored),
	string(semantic.RuleClassContested),
	string(semantic.RuleClassNeverTriggered),
	RuleClassUnclassified,
	string(semantic.RuleClassEffective),
}

// ValidRuleClass reports whether s is a class filter ("" included).
func ValidRuleClass(s string) bool {
	if s == "" {
		return true
	}
	for _, f := range RuleClassFilters {
		if f == s {
			return true
		}
	}
	return false
}

// RulesPerPage pages the rule table.
const RulesPerPage = 50

// ruleRemediesShown caps the listed proposals; RemediesTotal stays the true
// count.
const ruleRemediesShown = 50

// RuleEffectivenessFilter selects rules. Empty Service is every project;
// empty Class is every class.
type RuleEffectivenessFilter struct {
	Service string `json:"service,omitempty"`
	Class   string `json:"class"`
}

// RuleClassTally counts the rules in one class.
type RuleClassTally struct {
	Class string `json:"class"`
	Count int    `json:"count"`
	// Means is the class's condition in words.
	Means string `json:"means"`
}

// RuleEffectivenessRow is one governing rule with its counts over the
// windows. Rates are absent (nil) below the served floor: n is Served30.
type RuleEffectivenessRow struct {
	RuleID    string `json:"rule_id"`
	Service   string `json:"service"`
	Layer     string `json:"layer"`
	Status    string `json:"status"`
	Statement string `json:"statement"`
	// Class is "unclassified" when no class's condition is met (or, for
	// never triggered, the log does not yet cover its window).
	Class          string   `json:"class"`
	Served30       int      `json:"served_30d"`
	Complied30     int      `json:"complied_30d"`
	Violated30     int      `json:"violated_30d"`
	Blocked30      int      `json:"blocked_30d"`
	Refuted30      int      `json:"refuted_30d"`
	Corroborated30 int      `json:"corroborated_30d"`
	Served90       int      `json:"served_90d"`
	ComplianceRate *float64 `json:"compliance_rate,omitempty"`
	ViolationRate  *float64 `json:"violation_rate,omitempty"`
	// Remedy is the proposal for the class ("" when none); a proposal only.
	Remedy string `json:"remedy,omitempty"`
	// Note says why a row is not classified when the reason is the signal's
	// reach rather than the rule's behaviour.
	Note string `json:"note,omitempty"`
}

// RuleRemedyRow is one proposed change with the evidence for it.
type RuleRemedyRow struct {
	RuleID    string `json:"rule_id"`
	Service   string `json:"service"`
	Statement string `json:"statement"`
	Class     string `json:"class"`
	Remedy    string `json:"remedy"`
	Evidence  string `json:"evidence"`
}

// RuleEffectivenessView is the page's projection.
type RuleEffectivenessView struct {
	Filter RuleEffectivenessFilter `json:"filter"`
	AsOf   string                  `json:"as_of"`
	// Signal is how many outcome events the log holds; absent when there is
	// no log, an error when it could not be read.
	Signal Value[int] `json:"signal"`
	// SpanDays is how far back the log reaches; absent when it holds no
	// event. "Never triggered" is judged only once it reaches 90 days.
	SpanDays Value[int] `json:"span_days"`
	// Rules is how many governing rules the store holds (current claims in
	// governing layers, not retired) — a lower bound naming any project
	// whose claims could not be read.
	Rules Value[int] `json:"rules"`
	// Classes tallies every governing rule of the filtered projects, before
	// the class filter and paging.
	Classes []RuleClassTally `json:"classes"`
	// Total is how many rows match the filter; Rows is one page of them.
	Total   int                    `json:"total"`
	Page    int                    `json:"page"`
	Pages   int                    `json:"pages"`
	PerPage int                    `json:"per_page"`
	Rows    []RuleEffectivenessRow `json:"rows"`
	// Remedies are the proposals for the ignored and never-triggered rules
	// of the filtered projects, each with its evidence, capped at
	// ruleRemediesShown; RemediesTotal is the true count.
	Remedies      []RuleRemedyRow `json:"remedies"`
	RemediesTotal int             `json:"remedies_total"`
	Services      []string        `json:"services"`
	Notes         []string        `json:"notes"`
}

// ruleClassOrder ranks classes for display: what needs a human first.
var ruleClassOrder = map[string]int{
	string(semantic.RuleClassIgnored): 0, string(semantic.RuleClassContested): 1,
	string(semantic.RuleClassNeverTriggered): 2, RuleClassUnclassified: 3, string(semantic.RuleClassEffective): 4,
}

// ruleClassMeans states each class's condition (design doc §5's table).
var ruleClassMeans = map[string]string{
	string(semantic.RuleClassIgnored):        "violated on at least half of 20 or more serves in 30 days, and never blocked",
	string(semantic.RuleClassContested):      "refuted at least 3 times in 30 days",
	string(semantic.RuleClassNeverTriggered): "not served in 90 days",
	RuleClassUnclassified:                    "no class applies yet: fewer than 20 serves in 30 days, or a signal too short to judge",
	string(semantic.RuleClassEffective):      "complied on at least 90% of 20 or more serves in 30 days",
}

// ruleUniverse is one rule the view classifies.
type ruleUniverse struct {
	service string
	claim   core.SemanticClaim
}

// ListRuleEffectiveness projects the store's governing rules against the
// signal at now: counts and rates per rule with n, the class each falls in,
// and the remedies proposed for the ignored and never-triggered ones. page <
// 1 is the first page; a page past the last is the last.
func ListRuleEffectiveness(ctx context.Context, src KnowledgeSource, sig RuleSignal, f RuleEffectivenessFilter, page int, now time.Time) (RuleEffectivenessView, error) {
	if f.Class == "" {
		f.Class = RuleClassAll
	}
	if !ValidRuleClass(f.Class) {
		return RuleEffectivenessView{}, fmt.Errorf("unknown class %q (want one of %s)", f.Class, strings.Join(RuleClassFilters, ", "))
	}
	k, err := readKnowledge(ctx, src)
	if err != nil {
		return RuleEffectivenessView{}, err
	}
	now = now.UTC()
	out := RuleEffectivenessView{
		Filter: f, AsOf: rfc3339(now), PerPage: RulesPerPage,
		Rows: []RuleEffectivenessRow{}, Remedies: []RuleRemedyRow{}, Services: []string{}, Notes: []string{},
		Classes: []RuleClassTally{},
	}

	var universe []ruleUniverse
	all := map[string]ruleUniverse{} // any claim, for a rule id the log names that is no longer governing
	var unreadable []string
	for _, s := range k.Services {
		out.Services = append(out.Services, s.Service)
		if f.Service != "" && s.Service != f.Service {
			continue
		}
		if s.Err != "" {
			unreadable = append(unreadable, s.Service)
			continue
		}
		roles := semantic.LayerRolesIn(s.Root)
		for _, cl := range s.Claims {
			u := ruleUniverse{service: s.Service, claim: cl}
			if _, seen := all[cl.ClaimID]; !seen {
				all[cl.ClaimID] = u
			}
			if cl.Status.Served() && cl.Lifecycle.EffectiveLifecycle() != core.LifecycleRetired && roles[cl.Layer] != semantic.RoleClaims {
				universe = append(universe, u)
			}
		}
	}
	denom := "governing rules: current claims in governing layers, not retired"
	if f.Service != "" {
		denom += " · project " + f.Service
	}
	if len(unreadable) > 0 {
		out.Rules = Partial(len(universe), "lower bound; claims unreadable for: "+strings.Join(unreadable, ", ")).WithSource(storeSource).WithDenominator(denom)
	} else {
		out.Rules = Measured(len(universe), storeSource).WithDenominator(denom)
	}

	// The signal's own truth state, and how far back it reaches.
	var oldest time.Time
	switch {
	case sig.State == StateError:
		out.Signal = Errored[int](sig.Reason)
		out.SpanDays = Absent[int]("the rule-outcome log could not be read")
	case sig.State != StatePresent || len(sig.Events) == 0:
		reason := sig.Reason
		if reason == "" {
			reason = "no rule outcome has been recorded yet: the edit hook records one on each served edit"
		}
		out.Signal = Absent[int](reason)
		out.SpanDays = Absent[int]("no rule outcome has been recorded")
	default:
		for _, e := range sig.Events {
			if e.At.After(now) {
				continue
			}
			if oldest.IsZero() || e.At.Before(oldest) {
				oldest = e.At
			}
		}
		out.Signal = Measured(len(sig.Events), sig.Source).WithDenominator("rule outcome events in the log")
		if oldest.IsZero() {
			out.SpanDays = Absent[int]("every event is dated after this view's instant")
		} else {
			out.SpanDays = Measured(int(now.Sub(oldest)/(24*time.Hour)), sig.Source).WithDenominator("days from the oldest recorded event")
		}
	}
	if sig.Skipped > 0 {
		out.Notes = append(out.Notes, fmt.Sprintf("%d lines of the rule-outcome log could not be parsed and are not counted", sig.Skipped))
	}
	if !out.Signal.Known() || oldest.IsZero() {
		// Nothing is classified: an unmeasured rule is not a rule that never
		// triggered. The empty tally still names every class.
		for _, c := range []string{string(semantic.RuleClassIgnored), string(semantic.RuleClassContested), string(semantic.RuleClassNeverTriggered), RuleClassUnclassified, string(semantic.RuleClassEffective)} {
			out.Classes = append(out.Classes, RuleClassTally{Class: c, Means: ruleClassMeans[c]})
		}
		out.Pages, out.Page = 1, 1
		out.Notes = append(out.Notes, "no rule is classified without a rule-outcome signal; "+out.Signal.Reason)
		return out, nil
	}

	ids := make([]string, 0, len(universe))
	for _, u := range universe {
		ids = append(ids, u.claim.ClaimID)
	}
	results := semantic.RuleEffectiveness(ids, sig.Events, now)
	byID := make(map[string]semantic.RuleEffectivenessResult, len(results))
	for _, r := range results {
		byID[r.RuleID] = r
	}

	// "Never triggered" is a claim about 90 days: judged only when the log
	// reaches back that far.
	span := now.Sub(oldest)
	neverJudged := span >= semantic.RuleNeverTriggeredWindow
	held := 0
	for id, r := range byID {
		if r.Class == semantic.RuleClassNeverTriggered && !neverJudged {
			held++
			r.Class, r.Remedy = semantic.RuleClassNone, ""
			byID[id] = r
		}
	}
	if held > 0 {
		out.Notes = append(out.Notes, fmt.Sprintf("never triggered is not judged yet: the signal reaches back %d of %d days, so %d rules that were not served in it are left unclassified rather than proposed for pruning",
			int(span/(24*time.Hour)), int(semantic.RuleNeverTriggeredWindow/(24*time.Hour)), held))
	}

	// Rows: every governing rule, plus a rule the log names that is no
	// longer governing but is still a claim in the store.
	seenRow := map[string]bool{}
	governing := make(map[string]bool, len(universe))
	rowOf := func(u ruleUniverse, r semantic.RuleEffectivenessResult) RuleEffectivenessRow {
		class := string(r.Class)
		if r.Class == semantic.RuleClassNone {
			class = RuleClassUnclassified
		}
		row := RuleEffectivenessRow{
			RuleID: u.claim.ClaimID, Service: u.service, Layer: u.claim.Layer, Status: string(u.claim.Status), Statement: u.claim.Rule,
			Class: class, Served30: r.Served30, Complied30: r.Complied30, Violated30: r.Violated30, Blocked30: r.Blocked30,
			Refuted30: r.Refuted30, Corroborated30: r.Corroborated30, Served90: r.Served90, Remedy: r.Remedy,
		}
		if r.Served30 >= semantic.RuleEffectivenessMinServed {
			c, v := float64(r.Complied30)/float64(r.Served30), float64(r.Violated30)/float64(r.Served30)
			row.ComplianceRate, row.ViolationRate = &c, &v
		}
		if r.Class == semantic.RuleClassNone && r.Served90 == 0 && !neverJudged {
			row.Note = "not served in the signal so far; never triggered needs 90 days of it"
		}
		return row
	}
	var rows []RuleEffectivenessRow
	for _, u := range universe {
		key := u.service + "\x00" + u.claim.ClaimID
		if seenRow[key] {
			continue
		}
		seenRow[key] = true
		governing[u.claim.ClaimID] = true
		rows = append(rows, rowOf(u, byID[u.claim.ClaimID]))
	}
	unknown := 0
	for _, r := range results {
		if governing[r.RuleID] {
			continue
		}
		u, ok := all[r.RuleID]
		if !ok {
			unknown++
			continue
		}
		rows = append(rows, rowOf(u, r))
	}
	if unknown > 0 && f.Service == "" {
		out.Notes = append(out.Notes, fmt.Sprintf("%d rule ids in the signal log are not claims in this store (a removed project or a compiled-away rule); they are counted nowhere above", unknown))
	}

	tally := map[string]int{}
	for _, r := range rows {
		tally[r.Class]++
	}
	for _, c := range []string{string(semantic.RuleClassIgnored), string(semantic.RuleClassContested), string(semantic.RuleClassNeverTriggered), RuleClassUnclassified, string(semantic.RuleClassEffective)} {
		out.Classes = append(out.Classes, RuleClassTally{Class: c, Count: tally[c], Means: ruleClassMeans[c]})
	}

	// Remedies: GL-19's filter over the same results, for the rules shown.
	var remedyInput []semantic.RuleEffectivenessResult
	for _, r := range rows {
		remedyInput = append(remedyInput, byID[r.RuleID])
	}
	stmt := map[string]RuleEffectivenessRow{}
	for _, r := range rows {
		if _, ok := stmt[r.RuleID]; !ok {
			stmt[r.RuleID] = r
		}
	}
	sort.SliceStable(remedyInput, func(i, j int) bool { return remedyInput[i].RuleID < remedyInput[j].RuleID })
	remSeen := map[string]bool{}
	for _, rm := range semantic.RuleRemedies(remedyInput) {
		if remSeen[rm.RuleID] {
			continue
		}
		remSeen[rm.RuleID] = true
		out.RemediesTotal++
		if len(out.Remedies) < ruleRemediesShown {
			r := stmt[rm.RuleID]
			out.Remedies = append(out.Remedies, RuleRemedyRow{
				RuleID: rm.RuleID, Service: r.Service, Statement: r.Statement, Class: string(rm.Class), Remedy: rm.Remedy, Evidence: rm.Evidence,
			})
		}
	}

	// The filter, the order, the page.
	var shown []RuleEffectivenessRow
	for _, r := range rows {
		if f.Class == RuleClassAll || r.Class == f.Class {
			shown = append(shown, r)
		}
	}
	sort.SliceStable(shown, func(i, j int) bool {
		a, b := shown[i], shown[j]
		if ra, rb := ruleClassOrder[a.Class], ruleClassOrder[b.Class]; ra != rb {
			return ra < rb
		}
		if a.Served30 != b.Served30 {
			return a.Served30 > b.Served30
		}
		if a.Service != b.Service {
			return a.Service < b.Service
		}
		return a.RuleID < b.RuleID
	})
	out.Total = len(shown)
	out.Pages = max(1, (len(shown)+RulesPerPage-1)/RulesPerPage)
	out.Page = min(max(page, 1), out.Pages)
	lo := (out.Page - 1) * RulesPerPage
	hi := min(lo+RulesPerPage, len(shown))
	out.Rows = append(out.Rows, shown[lo:hi]...)
	return out, nil
}
