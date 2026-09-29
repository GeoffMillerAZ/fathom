package main

// console_sessions.go — the Sessions pages (vision 35 §3 Evidence, design
// §3 "Sessions"; OC-43): a table of sessions at /sessions — one row per
// session, never one per ledger row, filterable by arm, repo and text,
// sortable, fifty to a page — and one session's replay at /sessions/{id},
// windowed: never more than console.MaxWindowEvents events in a response
// (the survey's replay shipped 9.3 MB for one long session), paged and
// filterable by channel.
//
// The table renders `fathom metrics sessions --json`, the replay `fathom
// metrics sessions --id <id> --json` (metrics_evidence.go); the table reads
// the observatory model's one read of the ledger, the read the shared
// sessions metric comes from (OC-9).

import (
	"context"
	"fmt"
	"html"
	"html/template"
	"net/url"
	"sort"
	"strconv"
	"strings"

	"github.com/geoffmilleraz/app-kit/appkit/ui/chrome"
	"github.com/geoffmilleraz/app-kit/appkit/ui/form"
	"github.com/geoffmilleraz/app-kit/appkit/ui/stat"
	"github.com/geoffmilleraz/app-kit/appkit/ui/table"
	"github.com/geoffmilleraz/app-kit/appkit/ui/viz"
	"github.com/geoffmilleraz/fathom/internal/console"
)

func init() {
	declarePage(consolePage{
		ID: "sessions", Route: "/sessions", Href: "/sessions", Group: "Evidence", Order: 10, Nav: true,
		Title: "Sessions", Aliases: []string{"story"},
		Question: "What did agents do, with fathom and without?",
		Verbs:    []string{"fathom metrics sessions --json"},
		Shows:    []console.MetricID{console.MetricSessions, console.MetricObservations},
		Build:    buildSessionsPage,
	})
	declarePage(consolePage{
		ID: "session", Route: "/sessions/{id}", Group: "Evidence", Order: 11, NavParent: "sessions",
		Title:    "Session",
		Question: "What happened in one session, event by event?",
		// The exact command names the session and the window: the page's
		// JSON and heading carry it filled in (consoleOut.Verbs).
		Verbs: []string{"fathom metrics sessions --id {id} --per-page {per} --json"},
		Build: buildSessionPage,
	})
}

// sessionsPageModel is the table page's JSON projection.
type sessionsPageModel struct {
	Table console.SessionTableModel `json:"table"`
	// Feed is the live session feed (PE-19w): no session id, no
	// per-person figure (I58).
	Feed []sessionFeedRow `json:"feed"`
}

// sessionFilterFrom reads the table's filter from the query; an unknown
// sort key falls back to newest first.
func sessionFilterFrom(q url.Values) console.SessionFilter {
	f := console.SessionFilter{Arm: q.Get("arm"), Repo: q.Get("repo"), Query: strings.TrimSpace(q.Get("q")), Sort: q.Get("sort"), Asc: q.Get("dir") == "asc"}
	if !containsString(console.SessionSorts, f.Sort) {
		f.Sort = "at"
	}
	return f
}

// sessionsCommand is the exact `metrics sessions` command the page shows.
func sessionsCommand(f console.SessionFilter, page int) string {
	flags := []string{"--arm", f.Arm, "--repo", f.Repo, "--q", f.Query}
	if f.Sort != "at" {
		flags = append(flags, "--sort", f.Sort)
	}
	cmd := evCommand("fathom metrics sessions", flags...)
	extra := ""
	if f.Asc {
		extra += " --asc"
	}
	if page > 1 {
		extra += " --page " + strconv.Itoa(page)
	}
	return strings.TrimSuffix(cmd, " --json") + extra + " --json"
}

func buildSessionsPage(cr *consoleRequest) (consoleOut, error) {
	m := cr.model()
	q := cr.r.URL.Query()
	f := sessionFilterFrom(q)
	tbl := console.SessionTableFold(sessionLedgerPath(), m.sessionLedgerRead(), m.ledgerErr, f, evPage(q), 50)
	verb := sessionsCommand(f, tbl.Page)
	// The shared sessions metric is the model's one read of the ledger —
	// the read the table above is built from too (OC-9, ADR-045 decision 3).
	metrics := consoleModelMetrics(m)
	sessions := metrics[console.MetricSessions]
	feed, feedTotal := sessionFeedFrom(cr.r.Context(), m)
	if feed == nil {
		feed = []sessionFeedRow{}
	}
	meta := "sessions " + valuePlain(tbl.Sessions)
	if tbl.Sessions.Known() {
		meta = fmt.Sprintf("%s from %s", plural(tbl.Sessions.V, "session"), plural(tbl.LedgerRows.V, "ledger row"))
	}
	return consoleOut{
		Meta:     []string{"generated " + m.GeneratedAt, meta},
		Model:    sessionsPageModel{Table: tbl, Feed: feed},
		Metrics:  pageMetrics(cr.page, metrics),
		Sections: map[string]any{verb: tbl},
		Body: func() string {
			matching := valueStat("matching this filter", tbl.Matching)
			rows := valueStat("ledger rows", tbl.LedgerRows)
			denominator := ""
			if tbl.Matching.Known() {
				matching.Sub = "of " + plural(tbl.Sessions.V, "session")
				rows.Sub = "the Stop hook appends one per turn; " + plural(tbl.Anonymous.V, "row") + " with no session id count once each"
				denominator = plural(tbl.Matching.V, "matching session") + " of " + plural(tbl.Sessions.V, "session")
			}
			band := stat.RenderBand([]stat.Stat{valueStat("distinct sessions (all time)", sessions), matching, rows}, stat.BandOptions{AriaLabel: "sessions"})
			state, reason := console.StatePresent, ""
			switch {
			case !tbl.Sessions.Known():
				state, reason = tbl.Sessions.State, tbl.Sessions.Reason
			case !sessions.Known():
				state, reason = sessions.State, sessions.Reason
			}
			// PE-19w: the live feed, read-only, following the sessions
			// source over the SSE channel like every panel on this page.
			// Without a ledger read the feed is not measured: no count and
			// no empty state that would read as "no sessions".
			feedDenominator, feedBody := "", ""
			if state == console.StatePresent {
				feedDenominator = fmt.Sprintf("the %s of %s", plural(len(feed), "newest session"), plural(feedTotal, "session"))
				feedBody = renderSessionFeed(feed)
			}
			feedPanel := `<div class="mt-4">` + renderPanel(consolePanel{
				ID: "live-feed", Title: "Live session feed", SourceKey: "sessions",
				Question:    "Which sessions ran most recently, as which agent, and how many rows did each prove?",
				Source:      "the session ledger (" + sessionLedgerPath() + "), each session's last row, joined to the verify ledger's proof history (PE-15)",
				Denominator: feedDenominator,
				Freshness:   "as of the model read at " + m.GeneratedAt,
				State:       state, Reason: reason,
				Body: feedBody,
			}) + `</div>`
			return band + feedPanel + `<div class="mt-4">` + renderPanel(consolePanel{
				ID: "sessions-table", Title: "Sessions", SourceKey: "sessions",
				Question: "Which sessions ran, under which arm, and how far did each get?",
				Verb:     verb, Source: "the session ledger (" + sessionLedgerPath() + "), the last row per session id",
				Denominator: denominator,
				Freshness:   "as of the model read at " + m.GeneratedAt,
				State:       state, Reason: reason,
				Body: renderSessionFilter(tbl) + renderSessionTable(cr, tbl),
			}) + `</div>`
		},
	}, nil
}

// renderSessionFilter is the table's GET form: arm, repo and text, with a
// link that clears it. It works without JavaScript.
func renderSessionFilter(tbl console.SessionTableModel) string {
	f := tbl.Filter
	arms := []form.Option{{Value: "", Label: "every arm"}}
	for _, a := range tbl.Arms {
		arms = append(arms, form.Option{Value: a, Label: a})
	}
	repos := []form.Option{{Value: "", Label: "every repo"}}
	for _, r := range tbl.Repos {
		repos = append(repos, form.Option{Value: r, Label: r})
	}
	var b strings.Builder
	b.WriteString(`<form method="get" action="/sessions" class="mb-4 grid gap-3 sm:grid-cols-2 lg:grid-cols-4 [&_input]:min-h-11 [&_select]:min-h-11" aria-label="filter sessions">`)
	b.WriteString(form.RenderField(form.Field{Label: "Arm", For: "sessions-arm", Control: template.HTML(form.RenderSelect(form.Select{ID: "sessions-arm", Name: "arm", Value: f.Arm, Options: arms}))}))
	b.WriteString(form.RenderField(form.Field{Label: "Repo", For: "sessions-repo", Control: template.HTML(form.RenderSelect(form.Select{ID: "sessions-repo", Name: "repo", Value: f.Repo, Options: repos}))}))
	b.WriteString(form.RenderField(form.Field{Label: "Session or repo contains", For: "sessions-q", Control: template.HTML(form.RenderInput(form.Input{ID: "sessions-q", Name: "q", Value: f.Query, Placeholder: "8c867d31"}))}))
	if f.Sort != "at" {
		b.WriteString(`<input type="hidden" name="sort" value="` + html.EscapeString(f.Sort) + `">`)
	}
	if f.Asc {
		b.WriteString(`<input type="hidden" name="dir" value="asc">`)
	}
	b.WriteString(`<div class="flex items-end gap-2 [&>a]:min-h-11 [&>button]:min-h-11">` + form.RenderButton(form.Button{Label: "Filter", Variant: form.ButtonPrimary}) +
		form.RenderButton(form.Button{Label: "Clear", Href: "/sessions", Variant: form.ButtonGhost}) + `</div>`)
	b.WriteString(`</form>`)
	return b.String()
}

// sessionFigure is a table figure: the number, or "—" (unrecorded, never 0)
// with its reason for assistive tech.
func sessionFigure(v *int) string {
	if v == nil {
		return `<span class="text-ink-muted" data-truth-state="absent" title="not recorded for this session">—<span class="sr-only"> not recorded</span></span>`
	}
	return `<span class="tabular-nums">` + fmtThousands(*v) + `</span>`
}

// sessionSortHref is the table sorted by key: newest (descending) first, or
// the other direction when key is already the sort.
func sessionSortHref(cr *consoleRequest, tbl console.SessionTableModel, key string) string {
	dir := "desc"
	if tbl.Filter.Sort == key && !tbl.Filter.Asc {
		dir = "asc"
	}
	next := url.Values{}
	for k, vs := range cr.r.URL.Query() {
		if k != "page" && k != "format" {
			next[k] = vs
		}
	}
	next.Set("sort", key)
	next.Set("dir", dir)
	return "/sessions?" + next.Encode()
}

// sessionSortState is how the column key is sorted, in words and an arrow.
func sessionSortState(tbl console.SessionTableModel, key string) string {
	switch {
	case tbl.Filter.Sort != key:
		return ""
	case tbl.Filter.Asc:
		return ` <span aria-hidden="true">↑</span><span class="sr-only"> (sorted ascending)</span>`
	}
	return ` <span aria-hidden="true">↓</span><span class="sr-only"> (sorted descending)</span>`
}

// sessionSortHeader is a column header that sorts the table by key: a link
// that flips the direction when the column is already the sort.
func sessionSortHeader(cr *consoleRequest, tbl console.SessionTableModel, key, label string) templComponent {
	return evRaw(`<a href="` + html.EscapeString(sessionSortHref(cr, tbl, key)) + `" class="inline-flex min-h-11 items-center gap-1 text-ink hover:text-accent focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-focus">` + html.EscapeString(label) + sessionSortState(tbl, key) + `</a>`)
}

// sessionSortLabels names each sort key as the table's header does.
//
//fathom:tenancy-allow pkg-map read-only lookup table of constants, never written after init: not a cache
var sessionSortLabels = map[string]string{"at": "last row", "turns": "turns", "first_edit": "first edit", "tokens": "tokens", "fathom_calls": "fathom calls", "catches": "verify catches"}

// renderSessionSortChips is the sort below md, where the table (and its
// sortable headers) is hidden: one chip per key, the current one marked
// with its direction, each a plain link.
func renderSessionSortChips(cr *consoleRequest, tbl console.SessionTableModel) string {
	chips := make([]evChip, 0, len(console.SessionSorts))
	for _, key := range console.SessionSorts {
		label := sessionSortLabels[key]
		if tbl.Filter.Sort == key {
			if tbl.Filter.Asc {
				label += " ↑"
			} else {
				label += " ↓"
			}
		}
		chips = append(chips, evChip{Label: label, Href: sessionSortHref(cr, tbl, key), Selected: tbl.Filter.Sort == key})
	}
	return evChips("Sort by", chips)
}

// renderSessionTable is the table (from md up) and the same rows stacked
// (below md) with a sort control of their own, then the pages. With no
// readable ledger it renders nothing: the panel's truth line names why.
func renderSessionTable(cr *consoleRequest, tbl console.SessionTableModel) string {
	if !tbl.Matching.Known() {
		return ""
	}
	if tbl.Matching.V == 0 {
		return chrome.RenderEmptyState(chrome.EmptyState{Heading: "No session matches", Body: "Nothing in the ledger matches this filter — clear it to see every session."})
	}
	rows := make([]table.Row, 0, len(tbl.Rows))
	var list strings.Builder
	for _, r := range tbl.Rows {
		link := sessionLink(r.ID)
		arm := r.Arm
		if arm == "" {
			arm = "—"
		}
		rows = append(rows, table.Row{ID: "session-" + r.ID, Cells: []string{"", r.Repo, arm}, HTMLCells: []templComponent{
			evRaw(link), nil, nil, evRaw(`<span class="whitespace-nowrap tabular-nums">` + html.EscapeString(evShortTime(r.At)) + `</span>`),
			evRaw(sessionFigure(r.Turns)), evRaw(sessionFigure(r.FirstEditTurn)), evRaw(sessionFigure(r.Tokens)),
			evRaw(sessionFigure(r.FathomCalls)), evRaw(sessionFigure(r.VerifyCatches)),
		}})
		list.WriteString(`<li class="min-w-0 rounded-lg border border-line bg-surface-raised px-3 pb-3">` + link +
			chrome.RenderDescriptionList([]chrome.DescItem{
				{Term: "repo · arm", Detail: strings.TrimSpace(r.Repo + " · " + arm)},
				{Term: "last row", Detail: evShortTime(r.At)},
				{Term: "turns · first edit", DetailHTML: template.HTML(sessionFigure(r.Turns) + " · " + sessionFigure(r.FirstEditTurn))},
				{Term: "tokens (in + out)", DetailHTML: template.HTML(sessionFigure(r.Tokens))},
				{Term: "fathom calls · catches", DetailHTML: template.HTML(sessionFigure(r.FathomCalls) + " · " + sessionFigure(r.VerifyCatches))},
			}) + `</li>`)
	}
	cols := []table.Column{
		{Key: "session", Label: "Session"}, {Key: "repo", Label: "Repo"}, {Key: "arm", Label: "Arm"},
		{Key: "at", Label: "Last row (UTC)", HeaderHTML: sessionSortHeader(cr, tbl, "at", "Last row (UTC)")},
		{Key: "turns", Label: "Turns", Align: "right", HeaderHTML: sessionSortHeader(cr, tbl, "turns", "Turns")},
		{Key: "first_edit", Label: "First edit", Align: "right", HeaderHTML: sessionSortHeader(cr, tbl, "first_edit", "First edit")},
		{Key: "tokens", Label: "Tokens (in + out)", Align: "right", HeaderHTML: sessionSortHeader(cr, tbl, "tokens", "Tokens (in + out)")},
		{Key: "fathom_calls", Label: "Fathom calls", Align: "right", HeaderHTML: sessionSortHeader(cr, tbl, "fathom_calls", "Fathom calls")},
		{Key: "catches", Label: "Verify catches", Align: "right", HeaderHTML: sessionSortHeader(cr, tbl, "catches", "Verify catches")},
	}
	tb := renderKitTable(table.TableParams{TableID: "sessions", Columns: cols, Rows: rows,
		Caption: fmt.Sprintf("sessions %s–%s of %s, one row per session", fmtThousands((tbl.Page-1)*tbl.PerPage+1), fmtThousands((tbl.Page-1)*tbl.PerPage+len(tbl.Rows)), fmtThousands(tbl.Matching.V))})
	return `<div class="hidden md:block" data-sessions-table>` + tb + `</div>` +
		`<div class="md:hidden" data-sessions-sort>` + renderSessionSortChips(cr, tbl) + `</div>` +
		`<ul class="flex flex-col gap-3 md:hidden" aria-label="sessions" data-sessions-list>` + list.String() + `</ul>` +
		`<p class="mt-2 text-xs text-ink-muted">— = not recorded for that session (a row that predates the field), never zero.</p>` +
		evPagination(cr.r.URL.Path, cr.r.URL.Query(), tbl.Page, tbl.Pages, "session pages")
}

// sessionLink is a session's replay link, a 44 px target naming the id.
func sessionLink(id string) string {
	if id == "" {
		return `<span class="italic text-ink-muted">no session id</span>`
	}
	return `<a class="inline-flex min-h-11 items-center text-accent font-mono text-xs [overflow-wrap:anywhere] underline-offset-4 hover:underline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-focus" href="` + html.EscapeString(sessionHref(id)) + `">` + html.EscapeString(shortSessionID(id)) + `<span class="sr-only"> (session ` + html.EscapeString(id) + `)</span></a>`
}

// sessionPageWindow is the replay page's window size: small enough for the
// page's HTML budget, far under the response bound.
const sessionPageWindow = 200

// replayCommand is the exact `metrics sessions --id` command the replay
// shows, each value quoted where a shell would split it.
func replayCommand(id string, channels []string, page, perPage int) string {
	var b strings.Builder
	b.WriteString("fathom metrics sessions --id " + shellQuote(id))
	for _, c := range channels {
		b.WriteString(" --channel " + shellQuote(c))
	}
	if page > 1 {
		b.WriteString(" --page " + strconv.Itoa(page))
	}
	b.WriteString(" --per-page " + strconv.Itoa(perPage) + " --json")
	return b.String()
}

func buildSessionPage(cr *consoleRequest) (consoleOut, error) {
	id := cr.r.PathValue("id")
	q := cr.r.URL.Query()
	var channels []string
	for _, c := range q["channel"] {
		for _, part := range strings.Split(c, ",") {
			if part = strings.TrimSpace(part); part != "" {
				channels = append(channels, part)
			}
		}
	}
	// ?per= widens a JSON window up to the response bound; the HTML page
	// never draws more than its own window, the size its budget is set for.
	per := sessionPageWindow
	if n, err := strconv.Atoi(q.Get("per")); err == nil && n > 0 {
		per = min(n, console.MaxWindowEvents)
		if !wantsJSON(cr.r) {
			per = min(per, sessionPageWindow)
		}
	}
	m := cr.model()
	rp := buildSessionReplay(sessionLedgerPath(), m.ledger, m.ledgerErr, id, channels, evPage(q), per)
	verb := replayCommand(id, channels, rp.Window.Page, rp.Window.PerPage)
	// The top bar's h1 truncates, so it names the session by its short id;
	// the meta line carries the full id as selectable text. The law-14
	// caveat is the body's callout, stated once (F0 proof, item 7).
	return consoleOut{
		Title:    "Session " + shortSessionID(id),
		Meta:     []string{"session id " + id},
		Verbs:    []string{verb},
		Model:    rp,
		Sections: map[string]any{verb: rp},
		Body:     func() string { return renderSessionReplay(cr, rp, verb) },
	}, nil
}

// renderSessionReplay is the replay page body: the law-14 caveat, the
// windowed timeline with its channel chips, the window's events as a table,
// the pages, and the session's ledger row.
func renderSessionReplay(cr *consoleRequest, rp sessionReplay, verb string) string {
	w := rp.Window
	q := cr.r.URL.Query()
	path := cr.r.URL.Path
	var b strings.Builder
	b.WriteString(`<div class="mb-3 rounded-md border border-dashed border-caution px-3 py-2 text-ink-muted" data-law14>a session replay is a lens over mechanism-labeled events, never evidence (law 14)</div>`)

	var body strings.Builder
	if w.Events.Known() {
		chips := []evChip{{Label: "every channel", Href: evHref(path, q, "channel", ""), Selected: len(q["channel"]) == 0, Count: valueChipCount(w.Events)}}
		for _, c := range w.Channels {
			chips = append(chips, evChip{Label: c.Channel, Href: evHref(path, q, "channel", c.Channel), Selected: len(q["channel"]) > 0 && c.Selected, Count: fmtThousands(c.Events)})
		}
		body.WriteString(evChips("Channel", chips))
		body.WriteString(fmt.Sprintf(`<p class="mb-2 text-xs text-ink-muted" data-replay-window>window %d of %d · %s of %s · %s → %s UTC</p>`,
			w.Page, w.Pages, plural(len(w.Window), "event"), valueChipCount(w.Matching), html.EscapeString(evShortTime(w.From)), html.EscapeString(evShortTime(w.To))))
		if len(w.Window) > 0 {
			events := make([]viz.Event, 0, len(w.Window))
			for _, e := range w.Window {
				label := e.Channel
				if e.Kind != "" {
					label += ":" + e.Kind
				}
				events = append(events, viz.Event{At: e.At, Label: label, Role: beatRole(e.Channel, e.Kind), Lane: e.Channel})
			}
			tl := viz.Timeline(events, viz.TimelineOptions{W: 900, Fluid: true, AriaLabel: "session timeline"})
			// A text-dense chart: Fluid from md up (capped at its native
			// 900 px), its native width in its own keyboard-reachable
			// scrolling region below md (design §2, FATHOM-F0-KIT-04).
			body.WriteString(`<div class="overflow-x-auto" role="region" tabindex="0" aria-label="session timeline — scrolls sideways"><div class="min-w-[900px] max-w-[900px] md:min-w-0">` + tl + `</div></div>`)
			rows := make([]table.Row, 0, len(w.Window))
			for i, e := range w.Window {
				rows = append(rows, table.Row{ID: "event-" + strconv.Itoa(i), Cells: []string{e.At.UTC().Format("15:04:05"), e.Channel, e.Kind}, HTMLCells: []templComponent{nil, nil, nil,
					evRaw(`<span class="font-mono text-xs [overflow-wrap:anywhere]">` + html.EscapeString(e.Detail) + `</span>`)}})
			}
			tb := renderKitTable(table.TableParams{TableID: "session-events", Caption: fmt.Sprintf("this window's %s", plural(len(w.Window), "event")),
				Columns: []table.Column{{Key: "at", Label: "Time (UTC)"}, {Key: "channel", Label: "Channel"}, {Key: "kind", Label: "Kind"}, {Key: "detail", Label: "Labels"}}, Rows: rows})
			body.WriteString(`<div class="mt-3">` + chrome.RenderDisclosure(chrome.Disclosure{Summary: "this window's events as a table", Body: template.HTML(tb)}) + `</div>`)
		}
		body.WriteString(evPagination(path, q, w.Page, w.Pages, "replay windows"))
	}
	replayState, replayReason := console.StatePresent, ""
	if w.Events.State != console.StatePresent {
		replayState, replayReason = w.Events.State, w.Events.Reason
	}
	b.WriteString(renderPanel(consolePanel{
		ID: "replay", Title: "Replay", SourceKey: "sessions",
		Question: "What did the hooks record in this session, in order?",
		Verb:     verb, Source: "the hooks log (" + beatLogPath() + ", and its rotated generation), the beats naming this session",
		Denominator: fmt.Sprintf("at most %d events per window (%d per window here)", console.MaxWindowEvents, w.PerPage),
		Freshness:   "read at most 30 s ago (the model's TTL)",
		State:       replayState, Reason: replayReason,
		Body: body.String(),
	}))

	var card strings.Builder
	if e := rp.Ledger; e != nil {
		inOut := (*int)(nil)
		if e.TokensIn != nil && e.TokensOut != nil {
			n := *e.TokensIn + *e.TokensOut
			inOut = &n
		}
		card.WriteString(chrome.RenderDescriptionList([]chrome.DescItem{
			{Term: "repo", Detail: e.Repo},
			{Term: "arm", Detail: e.Arm},
			{Term: "last row", Detail: evShortTime(e.At)},
			{Term: "turns", DetailHTML: template.HTML(sessionFigure(e.TurnCount))},
			{Term: "first edit at turn", DetailHTML: template.HTML(sessionFigure(e.FirstEditTurn))},
			{Term: "tokens (in + out)", DetailHTML: template.HTML(sessionFigure(inOut))},
			{Term: "verify catches", DetailHTML: template.HTML(sessionFigure(e.VerifyCatches))},
			{Term: "discovery actions", DetailHTML: template.HTML(sessionFigure(e.DiscoveryActionsTotal))},
		}))
	}
	state, reason := rp.LedgerState, rp.LedgerNote
	if rp.Ledger != nil {
		state, reason = console.StatePresent, ""
	}
	// The ledger row is the "ledger" object of the replay verb's own JSON,
	// so the panel names that verb — the table verb prints no such row.
	b.WriteString(`<div class="mt-4">` + renderPanel(consolePanel{
		ID: "ledger-row", Title: "The session's ledger row", SourceKey: "sessions",
		Question: "What did the Stop hook record for this session at its last turn?",
		Verb:     verb, Source: "the session ledger (" + sessionLedgerPath() + "), the replay JSON's \"ledger\" object",
		Freshness: "as of the model read", State: state, Reason: reason,
		Body: card.String(),
	}) + `</div>`)

	// PE-18w: the same ledger row's tool-call mix and cache hit rate.
	mixBody, mixDenom := "", ""
	if e := rp.Ledger; e != nil {
		mix, counted := ledgerToolMix([]sessionLedgerEntry{*e})
		noMix := "no per-tool counts on this session's ledger row: the transcript fold records them from PE-18w on, and a Codex rollout carries none"
		if counted > 0 {
			noMix = ""
		}
		mixBody, mixDenom = renderToolMix("session-tool-mix", mix, noMix), plural(mix.TotalCalls, "tool call")
	}
	b.WriteString(`<div class="mt-4">` + renderPanel(consolePanel{
		ID: "tool-mix", Title: "Tool-call mix and cache hit rate", SourceKey: "sessions",
		Question:    "Which tools did this session call, and how much of its context came from cache?",
		Source:      "the session ledger row's tool_calls_by_name and token fields (the Stop hook's transcript fold), the replay JSON's \"ledger\" object",
		Denominator: mixDenom, Freshness: "as of the model read", State: state, Reason: reason,
		Body: mixBody,
	}) + `</div>`)
	return b.String()
}

// sessionJoinMaxAuthorLooks bounds the commit author-time lookups one
// page's session join makes (one read-only git process each); a commit past
// it joins by window only, and its row is named unjoined, never guessed.
const sessionJoinMaxAuthorLooks = 100

// sessionProofJoin is PE-15's join for a console page: the verify
// ledger's proof history (a bounded, cached tail read) joined to sessions
// (agentSessionsOf's windows over the model's ledger read), under f.
func sessionProofJoin(ctx context.Context, sessions []console.AgentSession, f console.HistoryFilter) (console.JoinResult, console.History, console.SourceRead) {
	h, read := console.ProofHistory(ctx, verifyLedgerPath(), console.DefaultHistoryTail)
	looks := 0
	authorAt := func(ctx context.Context, root, commit string) (string, bool) {
		if looks >= sessionJoinMaxAuthorLooks {
			return "", false
		}
		looks++
		return console.CommitAuthorAt(ctx, root, commit)
	}
	return console.JoinSessionProof(ctx, h, sessions, f, authorAt), h, read
}

// sessionFeedLimit is how many sessions the live feed lists, newest first.
const sessionFeedLimit = 25

// sessionFeedFrom builds the live session feed (PE-19w): the newest
// sessions the model's ledger read holds, each with its agent identity,
// repository and window, and the proven rows the join attributed to it.
// The join sees only these sessions and the days they span: a reading
// before the oldest listed session cannot join any of them. total is every
// session the read holds.
func sessionFeedFrom(ctx context.Context, m observatoryModel) (rows []sessionFeedRow, total int) {
	sessions := agentSessionsOf(m)
	total = len(sessions)
	sort.SliceStable(sessions, func(i, j int) bool { return sessions[i].End > sessions[j].End })
	sessions = sessions[:min(len(sessions), sessionFeedLimit)]
	if len(sessions) == 0 {
		return nil, total
	}
	from := sessions[0].Start
	for _, s := range sessions {
		from = min(from, s.Start)
	}
	f := console.HistoryFilter{}
	if len(from) >= len("2006-01-02") {
		f.From = from[:len("2006-01-02")]
	}
	join, _, read := sessionProofJoin(ctx, sessions, f)
	proven := make(map[string]int, len(join.Outcomes))
	for _, o := range join.Outcomes {
		proven[o.SessionID] = len(o.Rows)
	}
	rows = make([]sessionFeedRow, 0, len(sessions))
	for _, s := range sessions {
		r := sessionFeedRow{Harness: s.Harness, Model: s.Model, Repo: s.Repo, Start: s.Start, End: s.End}
		n, joined := proven[s.ID]
		switch {
		case read.State != console.StatePresent:
			r.Proven = console.Absent[int]("the proof history was not read: " + read.Reason)
		case joined:
			r.Proven = console.Measured(n, "the proof history joined to the session ledger")
		default:
			r.Proven = console.Absent[int]("no claimed-proven transition joined to this session yet")
		}
		rows = append(rows, r)
	}
	return rows, total
}

// ledgerRowsOf is the last ledger row of each session the join attributed
// rows to, in the join's order; a session the ledger no longer holds is
// skipped.
func ledgerRowsOf(m observatoryModel, join console.JoinResult) []sessionLedgerEntry {
	if m.ledgerErr != nil || m.ledger == nil {
		return nil
	}
	out := make([]sessionLedgerEntry, 0, len(join.Outcomes))
	for _, o := range join.Outcomes {
		if e, ok := m.ledger.latest[o.SessionID]; ok {
			out = append(out, e)
		}
	}
	return out
}

// ledgerToolMix sums ledger rows' per-name tool calls and cache tokens
// into console.ToolMix (PE-18w): cache-read tokens over cache-read plus
// fresh input, where fresh input is the uncached input and the
// cache-creation tokens (both were processed, not read from cache).
// counted is how many rows carried per-name counts; a row without them
// adds only its tokens.
func ledgerToolMix(rows []sessionLedgerEntry) (console.ToolMixModel, int) {
	calls := map[string]int{}
	cacheRead, fresh, counted := 0, 0, 0
	for _, e := range rows {
		if len(e.ToolCallsByName) > 0 {
			counted++
			for name, n := range e.ToolCallsByName {
				calls[name] += n
			}
		}
		if e.TokensCacheRead != nil {
			cacheRead += *e.TokensCacheRead
		}
		if e.TokensIn != nil {
			fresh += *e.TokensIn
		}
		if e.TokensCacheCreate != nil {
			fresh += *e.TokensCacheCreate
		}
	}
	return console.ToolMix(calls, cacheRead, fresh), counted
}

// renderToolMix renders the cache hit rate and call count as a band and
// the mix as a table. noMix, when set, replaces the table: an absent mix
// is named, never drawn as 0% for every tool.
func renderToolMix(tableID string, m console.ToolMixModel, noMix string) string {
	hit := stat.Absent("cache hit rate", m.CacheHit.Reason)
	hit.ValueRole = "ink-muted"
	if m.CacheHit.Known() {
		hit = stat.Stat{Label: "cache hit rate", Value: fmt.Sprintf("%.0f%%", m.CacheHit.V*100), Sub: m.CacheHit.Denominator}
	}
	calls := stat.Absent("tool calls", "no per-tool counts")
	calls.ValueRole = "ink-muted"
	if noMix == "" {
		calls = stat.Stat{Label: "tool calls", Value: fmtThousands(m.TotalCalls)}
	}
	var b strings.Builder
	b.WriteString(stat.RenderBand([]stat.Stat{hit, calls}, stat.BandOptions{AriaLabel: "cache hit rate and tool calls"}))
	if noMix != "" || len(m.Mix) == 0 {
		if noMix == "" {
			noMix = "no tool call recorded"
		}
		b.WriteString(`<p class="mt-3 text-ink-muted" data-tool-mix-empty>` + html.EscapeString(noMix) + `</p>`)
		return b.String()
	}
	rows := make([]table.Row, 0, len(m.Mix))
	for i, e := range m.Mix {
		rows = append(rows, table.Row{ID: tableID + "-" + strconv.Itoa(i), Cells: []string{e.Tool, fmtThousands(e.Calls), fmt.Sprintf("%.0f%%", e.Share*100)}})
	}
	b.WriteString(`<div class="mt-3">` + renderKitTable(table.TableParams{
		TableID: tableID, Caption: "Tool calls by tool name, most called first; other sums the names past the ledger row's top 20",
		Columns: []table.Column{{Key: "tool", Label: "Tool"}, {Key: "calls", Label: "Calls", Align: "right"}, {Key: "share", Label: "Share", Align: "right"}},
		Rows:    rows,
	}) + `</div>`)
	return b.String()
}

// shortSessionID is a session id's first eight characters (a UUID's first
// group), or the whole id when it is short.
func shortSessionID(id string) string {
	if r := []rune(id); len(r) > 8 {
		return string(r[:8])
	}
	return id
}
