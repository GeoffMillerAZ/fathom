package main

// console_rules.go — /rules, the rule-effectiveness page (GL-18, U4;
// docs/design/governance-learning.md §5): each governing rule with how often
// the edit hook served it, how often the same rule still topped the file on
// the next edit or another rule did (still_served, top_changed: scope
// persistence and churn, not compliance), how often it blocked one, and how
// often a human refuted it —
// counts with their n, the class each rule falls in (ignored, never
// triggered, effective, contested) and the change the evidence proposes.
// The page's JSON section is `fathom claim effectiveness --json` with the
// same filter.
//
// Every proposal is a proposal: fathom reads rules files and never edits
// one, and a retirement is the learning loop's own announced step, not this
// page's (GL-X1, GL-X2). A rule with no signal is unmeasured, never "never
// triggered"; a class that needs 90 days of signal waits for it.

import (
	"html"
	"html/template"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/a-h/templ"
	"github.com/geoffmilleraz/fathom/internal/console"
	"github.com/geoffmilleraz/fathom/third_party/app-kit/appkit/ui/callout"
	"github.com/geoffmilleraz/fathom/third_party/app-kit/appkit/ui/chrome"
	"github.com/geoffmilleraz/fathom/third_party/app-kit/appkit/ui/form"
	"github.com/geoffmilleraz/fathom/third_party/app-kit/appkit/ui/stat"
	"github.com/geoffmilleraz/fathom/third_party/app-kit/appkit/ui/table"
)

// ruleFreshness is the panels' freshness: the governing rules follow compile
// events (a recompile moves the set); the rule-outcome log has no event, so a
// new outcome shows when the page is read again.
const ruleFreshness = "read from the store and the rule-outcome log for this request; follows compile events, and new outcomes show on the next read"

// verbRuleEffectiveness is the rule-effectiveness verb; bracketed flags
// appear only when the request sets them (verbsFillTemplates reads the
// brackets as optional).
const verbRuleEffectiveness = "fathom claim effectiveness [--service {svc}] [--class {class}] [--page {page}] --json"

// ruleEffectivenessNow is the clock the verb reads; a test pins it beside
// the page's own.
var ruleEffectivenessNow = time.Now

func init() {
	declarePage(consolePage{
		ID: "rules", Route: "/rules", Href: "/rules", Group: "Evidence", Order: 12, Nav: true,
		Title:    "Rule effectiveness",
		Question: "Which governing rules do edits follow, ignore or never reach — and what change does that evidence propose?",
		Verbs:    []string{verbRuleEffectiveness},
		Build:    buildRulesPage,
	})
}

// rulesHref is the rule table for a filter; empty parts are every project
// and every class.
func rulesHref(service, class string) string {
	v := url.Values{}
	if service != "" {
		v.Set("svc", service)
	}
	if class != "" && class != console.RuleClassAll {
		v.Set("class", class)
	}
	if len(v) == 0 {
		return "/rules"
	}
	return "/rules?" + v.Encode()
}

// ruleEffectivenessVerb is the exact command for a filter and page.
func ruleEffectivenessVerb(f console.RuleEffectivenessFilter, page int) string {
	var b strings.Builder
	b.WriteString("fathom claim effectiveness")
	if f.Service != "" {
		b.WriteString(" --service " + shellWord(f.Service))
	}
	if f.Class != "" && f.Class != console.RuleClassAll {
		b.WriteString(" --class " + f.Class)
	}
	if page > 1 {
		b.WriteString(" --page " + strconv.Itoa(page))
	}
	return b.String() + " --json"
}

func buildRulesPage(cr *consoleRequest) (consoleOut, error) {
	src, err := knowledgeSourceFor(cr)
	if err != nil {
		return consoleOut{}, err
	}
	q := cr.r.URL.Query()
	f := console.RuleEffectivenessFilter{Service: strings.TrimSpace(q.Get("svc")), Class: strings.TrimSpace(q.Get("class"))}
	if f.Class == "" {
		f.Class = console.RuleClassAll
	}
	if !console.ValidRuleClass(f.Class) {
		return consoleOut{}, &consoleError{Status: http.StatusBadRequest, Title: "Unknown class",
			Reason: "class must be one of " + strings.Join(console.RuleClassFilters, ", ") + "; got " + f.Class + ".",
			Remedy: "fathom claim effectiveness --class all", RemedyHref: "/rules", RemedyLabel: "Show every rule"}
	}
	page := 1
	if p, perr := strconv.Atoi(q.Get("page")); perr == nil && p > 1 {
		page = p
	}
	v, err := console.ListRuleEffectiveness(cr.r.Context(), src, readRuleOutcomeSignal(ruleOutcomeLogPath()), f, page, cr.now)
	if err != nil {
		return consoleOut{}, err
	}
	if f.Service != "" && !containsString(v.Services, f.Service) {
		return consoleOut{}, &consoleError{Status: http.StatusNotFound, Title: "Unknown project",
			Reason: "No project in this store is named " + f.Service + ".",
			Remedy: "fathom claim effectiveness", RemedyHref: "/rules", RemedyLabel: "Show every project's rules"}
	}
	verb := ruleEffectivenessVerb(v.Filter, v.Page)
	return consoleOut{
		Meta:     []string{pluralValue(v.Rules, "governing rule"), "as of " + v.AsOf},
		Verbs:    []string{verb},
		Sections: map[string]any{verb: v},
		Body:     func() string { return renderRuleEffectiveness(v, verb) },
	}, nil
}

// ruleClassBadge names a class as the kit's badge: the word is the channel,
// the role only tints it.
func ruleClassBadge(class string) string {
	role := map[string]string{"ignored": "caution", "contested": "caution", "never_triggered": "info", "effective": "positive"}[class]
	return callout.Badge(strings.ReplaceAll(class, "_", " "), role)
}

// rulePercent renders a rate with the n it stands on, or the named floor.
func rulePercent(rate *float64, n int) string {
	if rate == nil {
		return `<span class="text-ink-muted">n = ` + strconv.Itoa(n) + ` (rate needs 20)</span>`
	}
	return strconv.Itoa(int(*rate*100+0.5)) + `% <span class="text-ink-muted">of ` + fmtThousands(n) + `</span>`
}

// renderRuleEffectiveness is the page body: the filter, the signal and the
// class counts at a glance, the proposed changes with their evidence, then
// every governing rule with its counts.
func renderRuleEffectiveness(v console.RuleEffectivenessView, verb string) string {
	var b strings.Builder
	b.WriteString(renderRulesFilter(v))

	tiles := []stat.Stat{
		valueStat("governing rules", v.Rules),
		valueStat("outcome events read", v.Signal),
		valueStat("days the signal reaches back", v.SpanDays),
	}
	for _, c := range v.Classes {
		if c.Class == console.RuleClassUnclassified {
			continue
		}
		label := strings.ReplaceAll(c.Class, "_", " ")
		if !v.Signal.Known() {
			s := stat.Absent(label, "no signal")
			s.ValueRole = "ink-muted"
			tiles = append(tiles, s)
			continue
		}
		tiles = append(tiles, stat.Stat{Label: label, Value: fmtThousands(c.Count), Sub: c.Means})
	}
	b.WriteString(`<div class="mb-4">` + stat.RenderBand(tiles, stat.BandOptions{AriaLabel: "rule effectiveness at a glance"}) + `</div>`)

	b.WriteString(renderPanel(consolePanel{
		ID: "rule-remedies", Title: "Proposed changes", SourceKey: "layers",
		Question:    "Which rules does the evidence say to move to a hook, rewrite or prune? These are proposals: fathom never edits a rules file or retires a rule from this page.",
		Verb:        verb,
		Source:      ruleSignalSource(v),
		Denominator: "ignored and never-triggered rules of " + ruleScope(v),
		Freshness:   ruleFreshness,
		State:       v.Signal.State, Reason: v.Signal.Reason,
		Body: renderRuleRemedies(v),
	}))
	b.WriteString(`<div class="mt-4">` + renderPanel(consolePanel{
		ID: "rule-table", Title: "Governing rules", SourceKey: "layers",
		Question:    "How is each rule doing? Counts are over 30 days (90 for served), each with its n; a rate needs 20 serves.",
		Verb:        verb,
		Source:      ruleSignalSource(v) + " · the daemon's store (semantic_claims)",
		Denominator: pluralValue(v.Rules, "governing rule") + " of " + ruleScope(v),
		Freshness:   ruleFreshness,
		State:       v.Signal.State, Reason: v.Signal.Reason,
		Body: renderRuleTable(v),
	}) + `</div>`)
	if len(v.Notes) > 0 {
		b.WriteString(`<ul class="mt-4 list-disc pl-5 text-sm text-ink-muted" data-rule-notes>`)
		for _, n := range v.Notes {
			b.WriteString(`<li class="[overflow-wrap:anywhere]">` + html.EscapeString(n) + `</li>`)
		}
		b.WriteString(`</ul>`)
	}
	return b.String()
}

func ruleSignalSource(v console.RuleEffectivenessView) string {
	if v.Signal.Source != "" {
		return "the rule-outcome log (" + v.Signal.Source + ")"
	}
	return "the rule-outcome log (~/.fathom/rule-outcomes.jsonl)"
}

func ruleScope(v console.RuleEffectivenessView) string {
	if v.Filter.Service != "" {
		return "project " + v.Filter.Service
	}
	return "every project"
}

// renderRulesFilter is the filter: a GET form, so every view is a URL.
func renderRulesFilter(v console.RuleEffectivenessView) string {
	projects := []form.Option{{Value: "", Label: "Every project"}}
	for _, s := range v.Services {
		projects = append(projects, form.Option{Value: s, Label: s})
	}
	classes := make([]form.Option, 0, len(console.RuleClassFilters))
	for _, c := range console.RuleClassFilters {
		label := strings.ReplaceAll(c, "_", " ")
		if c == console.RuleClassAll {
			label = "Every class"
		}
		classes = append(classes, form.Option{Value: c, Label: label})
	}
	field := func(label, id, name, value string, opts []form.Option) string {
		return form.RenderField(form.Field{Label: label, For: id, Control: template.HTML(form.RenderSelect(form.Select{ID: id, Name: name, Value: value, Options: opts}))})
	}
	return `<form method="get" action="/rules" class="mb-4 grid items-start gap-3 rounded-lg border border-line bg-surface-raised p-4 sm:grid-cols-[1fr_1fr_auto] [&_select]:min-h-11" aria-label="Filter rules" data-rules-form>` +
		field("Project", "rules-svc", "svc", v.Filter.Service, projects) +
		field("Class", "rules-class", "class", v.Filter.Class, classes) +
		`<div class="md:pt-6 [&>button]:min-h-11 [&>button]:w-full">` + form.RenderButton(form.Button{Label: "Show rules"}) + `</div></form>`
}

// ruleStatementLink is a rule's statement opening its claim page.
func ruleStatementLink(id, service, statement string) template.HTML {
	return template.HTML(`<a href="` + html.EscapeString(claimHref(id, service)) + `" class="inline-flex min-h-11 items-center rounded-sm text-accent underline-offset-4 hover:underline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-focus">` + html.EscapeString(statement) + `</a>`)
}

// renderRuleRemedies is the proposals table, or the honest empty state.
func renderRuleRemedies(v console.RuleEffectivenessView) string {
	if !v.Signal.Known() {
		return chrome.RenderEmptyState(chrome.EmptyState{Heading: "Nothing to propose without a signal",
			Body: "No rule is classified until the edit hook has recorded outcomes. " + v.Signal.Reason})
	}
	if len(v.Remedies) == 0 {
		return chrome.RenderEmptyState(chrome.EmptyState{Heading: "No change is proposed",
			Body: "No rule of " + ruleScope(v) + " is ignored, and none is old enough in the signal to be called never triggered."})
	}
	var rows []table.Row
	for _, r := range v.Remedies {
		rows = append(rows, table.Row{
			ID:    "remedy-" + r.Service + "-" + r.RuleID,
			Cells: []string{"", r.Service, "", r.Remedy, r.Evidence},
			HTMLCells: []templ.Component{
				templ.Raw(string(ruleStatementLink(r.RuleID, r.Service, r.Statement))),
				templ.Raw(knowledgeLink(projectHref(r.Service), r.Service)),
				templ.Raw(ruleClassBadge(r.Class)),
				nil, nil,
			},
		})
	}
	out := renderKitTable(table.TableParams{
		TableID: "rule-remedies-table",
		Caption: "Proposed changes for " + fmtThousands(v.RemediesTotal) + " rules, each with its evidence",
		Columns: []table.Column{
			{Key: "rule", Label: "Rule"}, {Key: "project", Label: "Project"}, {Key: "class", Label: "Class"},
			{Key: "proposal", Label: "Proposal"}, {Key: "evidence", Label: "Evidence"},
		},
		Rows: rows,
	})
	if v.RemediesTotal > len(v.Remedies) {
		out += `<p class="mt-2 text-sm text-ink-muted">Showing the first ` + strconv.Itoa(len(v.Remedies)) + ` of ` + fmtThousands(v.RemediesTotal) + `; the command lists the same.</p>`
	}
	return out
}

// renderRuleTable is the governing-rule table, paged.
func renderRuleTable(v console.RuleEffectivenessView) string {
	if !v.Signal.Known() {
		return chrome.RenderEmptyState(chrome.EmptyState{Heading: "No rule is classified yet",
			Body: v.Signal.Reason})
	}
	if len(v.Rows) == 0 {
		return chrome.RenderEmptyState(chrome.EmptyState{Heading: "No rule matches", Body: "No governing rule of " + ruleScope(v) + " is in this class."})
	}
	var rows []table.Row
	lo := (v.Page-1)*v.PerPage + 1
	for _, r := range v.Rows {
		rate := ""
		switch {
		case r.StillServedRate != nil && r.Class == "ignored":
			rate = rulePercent(r.StillServedRate, r.Served30) + " still served"
		case r.TopChangedRate != nil:
			rate = rulePercent(r.TopChangedRate, r.Served30) + " top changed"
		default:
			rate = rulePercent(nil, r.Served30)
		}
		if r.Note != "" {
			rate += `<br><span class="text-xs text-ink-muted">` + html.EscapeString(r.Note) + `</span>`
		}
		rows = append(rows, table.Row{
			ID:    "rule-" + r.Service + "-" + r.RuleID,
			Cells: []string{"", r.Service, "", strconv.Itoa(r.Served30), "", strconv.Itoa(r.Blocked30), strconv.Itoa(r.Refuted30), strconv.Itoa(r.Served90)},
			HTMLCells: []templ.Component{
				templ.Raw(string(ruleStatementLink(r.RuleID, r.Service, r.Statement))),
				templ.Raw(knowledgeLink(projectHref(r.Service), r.Service)),
				templ.Raw(ruleClassBadge(r.Class)),
				nil,
				templ.Raw(rate),
				nil, nil, nil,
			},
			Muted: r.Class == "unclassified",
		})
	}
	out := renderKitTable(table.TableParams{
		TableID: "rule-table-rows",
		Caption: "Rules " + fmtThousands(lo) + "–" + fmtThousands(lo+len(v.Rows)-1) + " of " + fmtThousands(v.Total) + ", ignored and contested first",
		Columns: []table.Column{
			{Key: "rule", Label: "Rule"}, {Key: "project", Label: "Project"}, {Key: "class", Label: "Class"},
			{Key: "served", Label: "Served 30d"}, {Key: "rate", Label: "Outcome"},
			{Key: "blocked", Label: "Blocked 30d"}, {Key: "refuted", Label: "Refuted 30d"}, {Key: "served90", Label: "Served 90d"},
		},
		Rows: rows,
	})
	out += consolePager(chrome.Pagination{
		Page: v.Page, TotalPages: v.Pages, AriaLabel: "rule pages",
		HrefFor: func(p int) string {
			h := rulesHref(v.Filter.Service, v.Filter.Class)
			sep := "?"
			if strings.Contains(h, "?") {
				sep = "&"
			}
			return h + sep + "page=" + strconv.Itoa(p) + "#rule-table"
		},
	})
	return out
}
