package main

// console_plan.go — one plan's page, /plans/{svc}/{plan} (OC-22): the proof
// bar, a board of its rows by the state the gate computed (the kit's
// kanban, read-only, each card naming its contract kind) with the same rows
// as a table, the evidence drawer the cards open, and the burn-up from the
// plan's git history cross-checked with the verify ledger (the proven trend
// gated on 14 days of ledger rows, SD-3).

import (
	"bytes"
	"context"
	"fmt"
	"html"
	"html/template"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/a-h/templ"
	"github.com/geoffmilleraz/app-kit/appkit/ui/chrome"
	"github.com/geoffmilleraz/app-kit/appkit/ui/kanban"
	"github.com/geoffmilleraz/app-kit/appkit/ui/overlay"
	"github.com/geoffmilleraz/app-kit/appkit/ui/table"
	"github.com/geoffmilleraz/app-kit/appkit/ui/viz"
	"github.com/geoffmilleraz/fathom/internal/console"
)

func init() {
	declarePage(consolePage{
		ID: "plan", Route: "/plans/{svc}/{plan}", Group: "Proof", Order: 1, NavParent: "plans",
		Title:    "Plan",
		Question: "Which of this plan's rows does the gate prove, which remain, and how did it get here?",
		Verbs: []string{
			"fathom plans status --service {svc} --plan {plan} --json",
			"fathom plans next --service {svc} --plan {plan} --json",
			"fathom plans history --service {svc} --plan {plan} --json",
		},
		Build: buildPlanPage,
	})
}

// planPageModel is /plans/{svc}/{plan}'s JSON model.
type planPageModel struct {
	console.PlanLedgerView
	Root string `json:"root"`
}

// resolvePlanRequest finds the request's service root and plan.
func resolvePlanRequest(cr *consoleRequest, budget time.Duration) (serviceRoot, console.PlanFacts, error) {
	m := cr.model()
	svc, slug := cr.r.PathValue("svc"), cr.r.PathValue("plan")
	if m.DegradedNote != "" {
		return serviceRoot{}, console.PlanFacts{}, unknownProjectError(svc, m, cr.deps.selfURL)
	}
	var sr serviceRoot
	for _, r := range modelServiceRoots(m) {
		if r.Service == svc {
			sr = r
		}
	}
	if sr.Service == "" {
		return sr, console.PlanFacts{}, unknownProjectError(svc, m, cr.deps.selfURL)
	}
	sps := snapshotPlans(cr.r.Context(), plansFor(cr.deps), []serviceRoot{sr}, slug, budget)
	if len(sps) > 0 && sps[0].Err != "" {
		// An unreadable plans directory is a read failure, not a missing
		// plan: 503 with the reason, never a 404 that reads as "no such plan".
		return sr, console.PlanFacts{}, &consoleError{
			Status: http.StatusServiceUnavailable, Title: "Plans unreadable",
			Reason: svc + "'s docs/plans could not be read: " + sps[0].Err,
			Remedy: "fathom doctor", RemedyHref: "/statusz", RemedyLabel: "Open the daemon status",
		}
	}
	if len(sps) == 0 || len(sps[0].Plans) == 0 {
		return sr, console.PlanFacts{}, &consoleError{
			Status: http.StatusNotFound, Title: "Unknown plan",
			Reason: "No plan named " + slug + " under " + svc + "'s docs/plans.",
			Remedy: "fathom plans status --service " + svc, RemedyHref: "/plans?svc=" + url.QueryEscape(svc), RemedyLabel: "See " + svc + "'s plans",
		}
	}
	return sr, sps[0].Plans[0], nil
}

// planRowsAs is a finished plan's rows with PlanFile set to the path the
// verbs were given (the rel the page's chips name).
func planRowsAs(pc *consolePlans, plan console.PlanFacts) ([]planRow, bool) {
	e, ok := pc.entry(plan.Path)
	if !ok || e.err != "" || !plan.Verified {
		return nil, false
	}
	rows := e.result.rows()
	for i := range rows {
		rows[i].PlanFile = plan.Rel
	}
	return rows, true
}

func buildPlanPage(cr *consoleRequest) (consoleOut, error) {
	sr, plan, err := resolvePlanRequest(cr, consolePlansPlanWait)
	if err != nil {
		return consoleOut{}, err
	}
	pc := plansFor(cr.deps)
	slug := plan.Slug()
	ledger := pc.ledgerRead()
	revs, older, note := pc.history(sr.Root, plan.Rel)
	burnup := planBurnupFrom(sr.Service, plan.Path, revs, older, note, &ledger)
	view := console.PlanLedger(console.PlanLedgerInput{
		Plan:    plan,
		RowHref: func(id string) string { return planRowHref(sr.Service, slug, id) },
	})
	view.Burnup = burnup
	verbs := []string{
		"fathom plans status --service " + sr.Service + " --plan " + plan.Rel + " --json",
		"fathom plans next --service " + sr.Service + " --plan " + plan.Rel + " --json",
		"fathom plans history --service " + sr.Service + " --plan " + plan.Rel + " --json",
	}
	sections := map[string]any{verbs[2]: burnup}
	if rows, ok := planRowsAs(pc, plan); ok {
		sections[verbs[0]] = []planRollup{console.RollupPlan(plan.Rel, plan.Rows, plan.DaemonStale)}
		leases, _ := readLeases(sr.Root)
		frontier := frontierForPlan(rows, leases, cr.now)
		if frontier == nil {
			frontier = []frontierRow{}
		}
		sections[verbs[1]] = frontier
	}
	model := planPageModel{PlanLedgerView: view, Root: sr.Root}
	meta := []string{plan.Rel, plural(len(plan.Rows), "row")}
	switch {
	case plan.Verified:
		meta = append(meta, "verified at index "+view.Stamp.StoreCommit)
	case plan.Err != "":
		meta = append(meta, "not verified: "+plan.Err)
	default:
		meta = append(meta, "verifying — the verdicts arrive over /events")
	}
	return consoleOut{
		Title:  plan.Title,
		Crumbs: []chrome.Crumb{{Label: "Plans", Href: "/plans"}, {Label: sr.Service, Href: "/plans?svc=" + url.QueryEscape(sr.Service)}},
		Meta:   meta, Model: model, Sections: sections, Verbs: verbs,
		Body: func() string { return renderPlanPage(view, sr.Service, verbs) },
	}, nil
}

func renderPlanPage(v console.PlanLedgerView, service string, verbs []string) string {
	live := "plans:" + service
	var b strings.Builder
	barBody := proofBarHTML(v.Bar, v.Title+" rows by proof state")
	b.WriteString(renderPanel(consolePanel{
		ID: "proof-bar", Title: "Proof bar", SourceKey: live,
		Question:    "Of this plan's rows, how many does the gate prove, and what holds the rest?",
		Verb:        verbs[0],
		Source:      "the verify engine at the current index; the working tree's lexical oracles",
		Denominator: plural(v.Bar.Total, "row"),
		Freshness:   planStamp(v.Stamp),
		State:       v.State,
		Reason:      v.Reason,
		Body:        barBody,
	}))

	b.WriteString(`<div class="mt-4">`)
	b.WriteString(renderPanel(consolePanel{
		ID: "board", Title: "Rows by state", SourceKey: live,
		Question:    "Which rows remain, which are claimed but not proven, and what does each one's contract check?",
		Verb:        verbs[1],
		Source:      "the same verdicts; each card names the contract the gate checks",
		Denominator: plural(len(v.Rows), "row"),
		Freshness:   planStamp(v.Stamp),
		Body:        renderPlanBoard(v) + `<div class="mt-3">` + chrome.RenderDisclosure(chrome.Disclosure{Summary: "Every row as a table", Body: template.HTML(renderPlanRowsTable(v))}) + `</div>`,
	}))
	b.WriteString(`</div>`)

	b.WriteString(`<div class="mt-4">`)
	b.WriteString(renderPanel(consolePanel{
		ID: "burnup", Title: "Burn-up", SourceKey: live,
		Question:    "How did the plan's scope and its claimed rows grow, and does the verify ledger agree?",
		Verb:        verbs[2],
		Source:      "the plan file's git history; the verify ledger's runs of this plan",
		Denominator: plural(v.Burnup.Commits, "commit") + " of the plan file",
		State:       v.Burnup.State,
		Reason:      v.Burnup.Reason,
		Body:        renderBurnup(v.Burnup),
	}))
	b.WriteString(`</div>`)
	b.WriteString(renderRowDrawer())
	return b.String()
}

func planStamp(s console.PlanStampFacts) string {
	if s.VerifiedAt == "" {
		return "verifying — the page is told over /events when the verdicts arrive"
	}
	out := "verified " + s.VerifiedAt + " at index " + s.StoreCommit
	if s.DaemonStale {
		out += " (the daemon binary was measured stale)"
	}
	return out
}

// stateBadge is a row state's card badge: its words and its role.
var stateBadge = map[string]kanban.Badge{
	console.RowProven:         {Label: "proven", Role: "positive-soft"},
	console.RowOpMeasured:     {Label: "measured", Role: "info-soft"},
	console.RowClaimedFailing: {Label: "claimed · failing", Role: "danger-soft"},
	console.RowClaimedUnknown: {Label: "claimed · undecidable", Role: "caution-soft"},
	console.RowMeasuredStale:  {Label: "claimed · daemon stale", Role: "caution-soft"},
	console.RowPending:        {Label: "claimed · not yet verified", Role: "surface-sunken"},
	console.RowOpen:           {Label: "open", Role: "surface-sunken"},
	console.RowInProgress:     {Label: "in progress", Role: "accent-soft"},
	console.RowOpWaiting:      {Label: "waiting", Role: "surface-sunken"},
	console.RowDeferred:       {Label: "deferred", Role: "surface-sunken"},
	console.RowSkipped:        {Label: "refused or skipped", Role: "surface-sunken"},
}

// boardLaneCards is how many cards one lane of the board shows; the lane's
// badge still counts every row, and the table under the board lists them.
const boardLaneCards = 12

// renderPlanBoard renders the board on the kit's kanban, read-only.
func renderPlanBoard(v console.PlanLedgerView) string {
	cols := make([]kanban.Column, 0, len(v.Columns))
	var cards []kanban.CardItem
	var empty []string
	cut := 0
	for _, c := range v.Columns {
		// A lane with no rows is named, not drawn — six fixed-width lanes
		// overflow a desktop screen and hid the last two behind a scroll —
		// except "claimed, not proven": its emptiness is the news.
		if len(c.Cards) == 0 && c.ID != "claimed" {
			empty = append(empty, strings.ToLower(c.Label))
			continue
		}
		cols = append(cols, kanban.Column{ID: c.ID, Label: c.Label, Count: len(c.Cards)})
		shown := c.Cards
		if len(shown) > boardLaneCards {
			cut += len(shown) - boardLaneCards
			shown = shown[:boardLaneCards]
		}
		for _, card := range shown {
			meta := card.Detail
			if len(meta) > 140 {
				meta = meta[:137] + "…"
			}
			cards = append(cards, kanban.CardItem{
				ID: card.ID, StageID: c.ID, Title: card.Title, Subtitle: card.ID,
				Badges: []kanban.Badge{{Label: card.Contract, Role: "surface-sunken"}, stateBadge[card.State]},
				Meta:   meta, Href: card.Href,
			})
		}
	}
	var buf bytes.Buffer
	_ = kanban.Board(cols, cards).Render(context.Background(), &buf)
	note := ""
	if cut > 0 {
		note = `<p class="mt-2 text-xs text-ink-muted" data-board-cut>` + fmt.Sprintf("Each lane shows its first %d cards; %s more are in the table below — every lane's badge counts all its rows.", boardLaneCards, plural(cut, "row")) + `</p>`
	}
	if len(empty) > 0 {
		note += `<p class="mt-2 text-xs text-ink-muted" data-board-empty>` + html.EscapeString("No rows are "+strings.Join(empty, ", ")+" — those lanes are not drawn.") + `</p>`
	}
	return `<div role="region" aria-label="Rows by state, one lane per state" data-plan-board>` + readOnlyBoard(buf.String()) + `</div>` + note
}

// readOnlyBoard takes the drag affordances off the kit's board: it has no
// read-only mode, so every card is draggable with a grab cursor, and its
// title link hovers to a token the base theme does not define. A read-only
// ledger must not look movable. Local workaround, returned as
// FATHOM-C-KIT-01 (docs/app-kit.md).
func readOnlyBoard(s string) string {
	return strings.NewReplacer(
		` draggable="true"`, "",
		`cursor-grab active:cursor-grabbing`, "",
		`hover:text-neon-pink`, "hover:text-accent",
	).Replace(s)
}

func renderPlanRowsTable(v console.PlanLedgerView) string {
	rows := make([]table.Row, 0, len(v.Rows))
	for _, r := range v.Rows {
		link := `<a href="` + html.EscapeString(r.Href) + `" class="inline-flex min-h-11 items-center font-mono text-xs text-accent underline-offset-4 hover:underline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-focus">` + html.EscapeString(r.ID) + `</a>`
		rows = append(rows, table.Row{
			ID:        "row-" + r.ID,
			Cells:     []string{r.ID, r.Title, stateBadge[r.State].Label, r.Contract},
			HTMLCells: []templ.Component{templ.Raw(link)},
		})
	}
	return renderKitTable(table.TableParams{
		TableID: "plan-rows", Caption: "Every row in document order",
		Columns: []table.Column{{Key: "id", Label: "Row"}, {Key: "title", Label: "Title"}, {Key: "state", Label: "State"}, {Key: "contract", Label: "Contract"}},
		Rows:    rows,
	})
}

// renderBurnup draws the plan's revisions as one bar per commit on one
// scale — claimed rows, then open rows in scope, then the room scope grew
// into later — with the numbers in the same row; then the ledger's proven
// trend (stat.Absent until 14 days, SD-3) and the cross-check.
func renderBurnup(b console.Burnup) string {
	var out strings.Builder
	if b.State == console.StatePresent {
		rows := make([]table.Row, 0, len(b.Points))
		for i := len(b.Points) - 1; i >= 0; i-- {
			p := b.Points[i]
			segs := []viz.Segment{
				{Value: float64(p.Claimed), Role: "positive", Label: fmt.Sprintf("claimed: %d", p.Claimed)},
				{Value: float64(p.Scope - p.Claimed), Role: "line-strong", Label: fmt.Sprintf("in scope, not claimed: %d", p.Scope-p.Claimed)},
				{Value: float64(b.MaxScope - p.Scope), Role: "surface-sunken", Label: "not yet in scope"},
			}
			bar := `<div class="w-48 max-w-full">` + viz.StackBar(segs, viz.StackBarOptions{W: 192, H: 10, Fluid: true, AriaLabel: fmt.Sprintf("%s: %d claimed of %d in scope", p.At, p.Claimed, p.Scope)}) + `</div>`
			rows = append(rows, table.Row{
				ID:        "rev-" + p.Commit,
				Cells:     []string{p.At, p.Commit, "", fmtThousands(p.Scope), fmtThousands(p.Claimed)},
				HTMLCells: []templ.Component{nil, templ.Raw(`<span class="font-mono text-xs">` + html.EscapeString(p.Commit) + `</span>`), templ.Raw(bar)},
			})
		}
		caption := "The plan at each commit, newest first — bars share one scale (the largest scope)"
		if b.Omitted > 0 {
			caption += fmt.Sprintf("; the oldest %d commits are not drawn", b.Omitted)
		}
		out.WriteString(renderKitTable(table.TableParams{
			TableID: "burnup", Caption: caption,
			Columns: []table.Column{
				{Key: "at", Label: "Committed"}, {Key: "commit", Label: "Commit"}, {Key: "bar", Label: "Claimed, then open, in scope"},
				{Key: "scope", Label: "Rows in scope", Align: "right"}, {Key: "claimed", Label: "Claimed", Align: "right"},
			},
			Rows: rows,
		}))
	}
	out.WriteString(`<h3 class="mt-4 text-sm font-semibold text-ink">Proven by the gate over time</h3>`)
	out.WriteString(`<div class="mt-2">` + renderTrend(b.Proven, "proven by the gate, per day", "this plan's gate runs, one row per day") + `</div>`)
	cc := b.CrossCheck
	out.WriteString(`<p class="mt-3 text-sm text-ink-muted" data-cross-check>Cross-check with the verify ledger: ` +
		html.EscapeString(fmt.Sprintf("%d of %d gate runs on a revision of this plan counted the rows that revision holds; %d ran on a commit that did not touch the plan.", cc.Agree, cc.Matched, cc.Unmatched)) + `</p>`)
	if len(cc.Disagree) > 0 {
		out.WriteString(`<ul class="mt-1 list-disc pl-5 text-sm text-caution">`)
		for _, d := range cc.Disagree {
			out.WriteString(`<li>` + html.EscapeString(d) + `</li>`)
		}
		out.WriteString(`</ul>`)
	}
	return out.String()
}

// rowDrawerID is the plan page's evidence drawer.
const rowDrawerID = "row-drawer"

// renderRowDrawer is the kit drawer the board's cards open (the script
// below fetches the row page's fragment into it); without JavaScript each
// card is a link to the row's own page.
func renderRowDrawer() string {
	return overlay.RenderDrawer(overlay.Drawer{
		ID: rowDrawerID, Title: "Row evidence", LightDismiss: true,
		Body:   template.HTML(`<div data-row-drawer-body aria-live="polite"><p class="text-ink-muted">Choose a row on the board.</p></div>`),
		Footer: template.HTML(`<a data-row-drawer-page href="/plans" class="inline-flex min-h-11 items-center font-medium text-accent underline-offset-4 hover:underline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-focus">Open as a page</a><button type="button" data-row-drawer-close class="inline-flex min-h-11 min-w-11 items-center justify-center rounded-md border border-line px-3 text-sm font-medium text-ink hover:bg-surface-sunken focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-focus">Close</button>`),
	}) + `<script>` + rowDrawerScript + `</script>`
}

// rowDrawerScript opens a card's evidence in the drawer: a plain click on a
// board card fetches the row page's fragment, checks it is that row's, and
// shows it; Escape or Close returns focus to the card. A modified click, a
// failed fetch or no dialog support leaves the link to do its job.
const rowDrawerScript = `(function(){
var board=document.querySelector("[data-plan-board]"),dlg=document.getElementById("` + rowDrawerID + `");
if(!board||!dlg||typeof dlg.showModal!=="function"||!window.fetch)return;
var body=dlg.querySelector("[data-row-drawer-body]"),title=dlg.querySelector("h2"),page=dlg.querySelector("[data-row-drawer-page]"),opener=null,seq=0;
function show(msg){var p=document.createElement("p");p.className="text-ink-muted";p.textContent=msg;body.replaceChildren(p);}
board.addEventListener("click",function(e){
var a=e.target.closest("a[href*='/rows/']");
if(!a||e.defaultPrevented||e.button!==0||e.metaKey||e.ctrlKey||e.shiftKey||e.altKey)return;
e.preventDefault();opener=a;var href=a.getAttribute("href"),id=decodeURIComponent(href.split("/rows/")[1]||""),mine=++seq;
title.textContent=id+" · "+a.textContent.trim();page.setAttribute("href",href);show("Loading the evidence…");
if(!dlg.open)dlg.showModal();
fetch(href+"?format=fragment",{headers:{"Accept":"text/html"},credentials:"same-origin"}).then(function(r){if(!r.ok)throw new Error("HTTP "+r.status);return r.text();}).then(function(t){
if(mine!==seq)return;var tpl=document.createElement("template");tpl.innerHTML=t;var f=tpl.content.querySelector("[data-row-evidence]");
if(!f||f.getAttribute("data-row-id")!==id)throw new Error("the answer was not this row's evidence");body.replaceChildren(f);
}).catch(function(err){if(mine===seq)show("The evidence could not be loaded here ("+err.message+") — open it as a page.");});
});
dlg.addEventListener("click",function(e){if(e.target.closest("[data-overlay-dismiss],[data-row-drawer-close]"))dlg.close();});
dlg.addEventListener("close",function(){if(opener&&document.contains(opener))opener.focus();});
})();`
