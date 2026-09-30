package main

// console_project_intent.go — the workspace's Intent tab at
// /projects/{svc}/intent (vision 35 §3 "Intent"; design §3; OC-38): the
// repository's intent corpus through `fathom intent audit --read-only` —
// invariants with their enforcement rung and audit state, decisions with
// their revisit conditions (fired or not), futures and questions, the
// unproven claims, and the intent ledger's debt over time.
//
// Every tab body is built from source-keyed panels (console_live.go), so
// each panel marks itself updated on its own sources; the page declares no
// page-level sources. The tab runs the audit read-only (runIntentAudit, the verb's own code):
// rendering a page never appends a clock, baseline, debt or cache row to the
// user's intent ledger, and the index oracle's requests carry the console's
// origin, so looking at the tab is never usage. An audit reads the whole
// corpus, the tree and the verify ledger, so one run is kept while its
// inputs are unchanged (intentAuditKey), five minutes at most, and its time
// is stated.

import (
	"fmt"
	"html"
	"html/template"
	"strconv"
	"strings"
	"time"

	"github.com/geoffmilleraz/app-kit/appkit/ui/chrome"
	"github.com/geoffmilleraz/app-kit/appkit/ui/stat"
	"github.com/geoffmilleraz/app-kit/appkit/ui/table"
	"github.com/geoffmilleraz/app-kit/appkit/ui/viz"
	"github.com/geoffmilleraz/fathom/internal/console"
	"github.com/geoffmilleraz/fathom/internal/intent"
	"github.com/geoffmilleraz/fathom/internal/store"
)

const verbIntentAuditReadOnly = "fathom intent audit --read-only --json --root {root} --service {svc} --url {url}"

func init() {
	declarePage(consolePage{
		ID: "project-intent", Route: "/projects/{svc}/intent", Group: "Fleet", Order: 16, NavParent: "fleet",
		Title:    "Project intent",
		Question: "What does this repository promise — invariants, decisions, futures — and does each still hold?",
		Verbs:    []string{verbIntentAuditReadOnly},
		Build:    buildProjectIntentPage,
	})
}

// projectIntentModel is the intent tab's JSON projection.
type projectIntentModel struct {
	RootNote string                   `json:"root_note,omitempty"`
	Error    string                   `json:"error,omitempty"`
	AuditAt  string                   `json:"audit_at,omitempty"`
	View     *console.IntentViewModel `json:"view,omitempty"`
	// Radar is the drift radar (CX-22): fired computable triggers and the
	// deny rules whose ingest-time violation count rose.
	Radar *driftRadarModel `json:"radar,omitempty"`
}

// intentAuditResult is one read-only audit run, as the memo keeps it.
type intentAuditResult struct {
	run intentAuditRun
	err error
}

// intentAuditMemo keeps recent audits (console_memo.go: the lock guards the
// map, never an audit, so one project's cold audit never holds up another
// project's kept one).
var intentAuditMemo = consoleMemo[intentAuditResult]{max: 16}

// intentAuditTTL bounds how stale a shown audit may be. An audit of a large
// repository walks its whole tree and asks the index about every named test
// (18–50 s on this repository's own checkout), so a run is kept while its
// inputs are unchanged — for five minutes at most — and its time is stated
// on every panel.
const intentAuditTTL = 5 * time.Minute

// intentAuditRetryTTL is how long a run that failed, or could not read
// every source, is kept: the EPERM wedge behind it clears in seconds, and
// the full audit must then replace the partial one soon.
const intentAuditRetryTTL = 30 * time.Second

// intentAuditKey is the audit's input watermarks (design §1 "Caching"): the
// root and service, the commit (the corpus and the tree), the generation of
// the panels' own live source plans:{svc} — a verify-ledger row for the
// project, which the audit reads for U1/U2 and which makes every intent
// panel say "updated — refresh this panel" — and the intent ledger's size
// and mtime (the debt over time and the fired conditions). A refresh the
// affordance offers therefore re-runs the audit; nothing else does.
func intentAuditKey(root, svc string, plansGeneration uint64) string {
	return strings.Join([]string{root, svc, repoHeadNoFork(root), strconv.FormatUint(plansGeneration, 10), fileWatermark(intent.LedgerPath())}, "\x00")
}

// readOnlyIntentAudit runs (or recalls) the read-only audit for root.
func readOnlyIntentAudit(root, svc, selfURL string, st store.Store, plansGeneration uint64, now time.Time) (intentAuditRun, time.Time, error) {
	r, at := intentAuditMemo.do(intentAuditKey(root, svc, plansGeneration), now, func() (intentAuditResult, time.Duration) {
		// The daemon's own store answers a computable trigger's
		// arch_violations instrument (CX-21) — read because this tab was
		// opened, never on a schedule.
		run, err := runIntentAudit(intentAuditOptions{Root: root, Service: svc, URL: selfURL, ReadOnly: true, HTTP: consoleRPCClient(30 * time.Second), Store: st})
		keep := intentAuditTTL
		if err != nil || !run.Audit.Coverage.Complete() {
			keep = intentAuditRetryTTL
		}
		return intentAuditResult{run: run, err: err}, keep
	})
	return r.run, at, r.err
}

func buildProjectIntentPage(cr *consoleRequest) (consoleOut, error) {
	pc, err := loadProjectContext(cr)
	if err != nil {
		return consoleOut{}, err
	}
	out := pc.frame("Intent")
	model := projectIntentModel{}
	// A root this daemon cannot read has no corpus to audit: no verb is
	// offered (it would fail), and the page says why.
	out.Verbs = []string{}
	if pc.rootNote != "" {
		model.RootNote = pc.rootNote
		out.Model = model
		out.Body = func() string {
			return renderProjectTabs(pc.svc, "intent") + renderPanel(consolePanel{
				ID: "intent", Title: "Intent", Question: "What does this repository promise?",
				State: console.StateAbsent, Reason: "not audited — " + pc.rootNote, Source: "fathom intent audit --read-only",
				SourceKey: "plans:" + pc.svc,
			})
		}
		return out, nil
	}
	var st store.Store
	if cr.deps.c != nil {
		st = cr.deps.c.Store()
	}
	run, at, aerr := readOnlyIntentAudit(pc.root, pc.svc, cr.deps.selfURL, st, cr.deps.sourceGeneration(cr.r.Context(), "plans:"+pc.svc), cr.now)
	if aerr != nil {
		model.Error = aerr.Error()
		out.Model = model
		out.Body = func() string {
			return renderProjectTabs(pc.svc, "intent") + renderPanel(consolePanel{ID: "intent", Title: "Intent", State: console.StateError, Reason: aerr.Error(), Question: "What does this repository promise?", Source: "fathom intent audit --read-only", SourceKey: "plans:" + pc.svc})
		}
		return out, nil
	}
	verb := fillVerb(verbIntentAuditReadOnly, map[string]string{"root": pc.root, "svc": pc.svc, "url": cr.deps.selfURL})
	out.Verbs = []string{verb}
	view := console.IntentView(run.Corpus, run.Audit, run.Ledger, at)
	model.View, model.AuditAt = &view, at.UTC().Format(time.RFC3339)
	// CX-22: the drift radar — the audit's trigger verdicts and the
	// service's ingest-time violation readings (one small per-service read).
	var trend []store.ArchViolationReading
	terr := fmt.Errorf("no store")
	if st != nil {
		trend, terr = st.ArchViolationTrend(cr.r.Context(), pc.svc)
	}
	radar := buildDriftRadar(run.Audit.Triggers, trend, terr)
	model.Radar = &radar
	out.Model = model
	out.Sections = map[string]any{verb: withLedgerIgnored(intentAuditPayload(run.Corpus, run.Audit, run.Ledger, at), run.LedgerIgnored)}
	out.Body = func() string {
		return renderProjectTabs(pc.svc, "intent") + projectBody(renderProjectIntent(pc, view, verb, at, &radar))
	}
	return out, nil
}

func renderProjectIntent(pc projectContext, v console.IntentViewModel, verb string, at time.Time, radar *driftRadarModel) string {
	var b strings.Builder
	// No corpus to show: absent when the repository has none (a choice),
	// an error naming the files when one exists and cannot be read (I38) —
	// the projection decides which, from the corpus's own reads.
	if !v.Corpus.Known() {
		b.WriteString(renderPanel(consolePanel{
			ID: "intent", Title: "Intent", Verb: verb,
			Question: "What does this repository promise?",
			State:    v.Corpus.State, Reason: v.Corpus.Reason,
			Source: "fathom intent audit --read-only", Freshness: "audited " + at.UTC().Format("15:04:05 UTC"), SourceKey: "plans:" + pc.svc,
		}))
		return b.String()
	}
	enforced := 0
	for _, inv := range v.Invariants {
		if inv.Enforced {
			enforced++
		}
	}
	b.WriteString(stat.RenderBand([]stat.Stat{
		{Label: "invariants", Value: fmtThousands(len(v.Invariants)), Sub: fmt.Sprintf("%d claim an enforcing rung", enforced)},
		valueStat("unproven claims", v.UnprovenCount),
		valueStat("conditions fired", v.Fired),
		valueStat("recorded audits", v.DebtState),
	}, stat.BandOptions{AriaLabel: pc.svc + " intent"}))
	fresh := "audited " + at.UTC().Format("2006-01-02 15:04:05 UTC") + " (read-only; re-run when the commit, a verify row for " + pc.svc + " or the intent ledger changes, else kept up to five minutes)"
	if radar != nil {
		b.WriteString(`<div class="mt-4">` + driftRadarPanel(pc.svc, verb, fresh, *radar) + `</div>`)
	}

	// Invariants.
	var inv strings.Builder
	rows := make([]table.Row, 0, len(v.Invariants))
	for _, r := range v.Invariants {
		state := r.State
		if len(r.Detail) > 0 {
			state += " — " + strings.Join(r.Detail, "; ")
		}
		rows = append(rows, table.Row{ID: "inv-" + r.ID, Cells: []string{r.ID, clipText(r.Statement, 180), r.Rung, clipText(state, 240)}})
	}
	if len(rows) == 0 {
		inv.WriteString(`<p class="text-ink-muted">The corpus declares no invariants.</p>`)
	} else {
		inv.WriteString(renderKitTable(table.TableParams{
			TableID: "project-invariants", Caption: fmt.Sprintf("%d invariants, with the rung each claims and what the audit found", len(rows)),
			Columns: []table.Column{{Key: "id", Label: "ID"}, {Key: "statement", Label: "Invariant"}, {Key: "rung", Label: "Enforced by"}, {Key: "state", Label: "Audit"}},
			Rows:    rows,
		}))
	}
	b.WriteString(`<div class="mt-4 grid gap-4">`)
	b.WriteString(renderPanel(consolePanel{
		ID: "intent-invariants", Title: "Invariants",
		Question: "Which promises are enforced by a test or a gate, which are honestly only stated, and which does no instrument show?",
		Verb:     verb, Source: v.Router, Denominator: "every invariant row in invariants.md", Freshness: fresh,
		State: v.UnprovenCount.State, Reason: v.UnprovenCount.Reason, SourceKey: "plans:" + pc.svc, Body: inv.String(),
	}))

	// Conditions: one panel, a disclosure per group — the corpus of a
	// large repository holds a hundred of them, and the groups' counts by
	// state are what a reader scans first.
	var conds strings.Builder
	conds.WriteString(`<div class="grid gap-3">`)
	for _, g := range []struct {
		id, title string
		list      []console.IntentCondition
	}{{"decisions", "Decisions — revisit conditions", v.Decisions}, {"futures", "Futures — promote and drop conditions", v.Futures}, {"questions", "Questions", v.Questions}, {"other", "Goals and assumptions", v.Other}} {
		byState := map[string]int{}
		for _, c := range g.list {
			byState[c.State]++
		}
		summary := fmt.Sprintf("%s — %s", g.title, plural(len(g.list), "condition"))
		for _, st := range []string{"fired", "not fired", "human-judged", "terminal"} {
			if byState[st] > 0 {
				summary += fmt.Sprintf(", %d %s", byState[st], st)
			}
		}
		if len(g.list) == 0 {
			conds.WriteString(`<p class="text-ink-muted" data-intent-group="` + g.id + `">` + html.EscapeString(summary) + `</p>`)
			continue
		}
		rows := make([]table.Row, 0, len(g.list))
		for _, r := range g.list {
			state := r.State
			if r.FiredAt != "" {
				state += " " + r.FiredAt
			}
			text := strings.TrimSpace(strings.Trim(strings.TrimSpace(r.Text), "*"))
			if text == "" {
				// The label's line carries no text (a bulleted list follows
				// it); the corpus line is the place to read it.
				text = fmt.Sprintf("see %s:%d", r.File, r.Line)
			}
			rows = append(rows, table.Row{ID: g.id + "-" + r.ID, Cells: []string{r.ID, r.Label, clipText(text, 180), state}})
		}
		tbl := renderKitTable(table.TableParams{
			TableID: "project-" + g.id, Caption: fmt.Sprintf("%d conditions", len(g.list)),
			Columns: []table.Column{{Key: "id", Label: "Record"}, {Key: "label", Label: "Condition"}, {Key: "text", Label: "When"}, {Key: "state", Label: "State"}},
			Rows:    rows,
		})
		conds.WriteString(`<div data-intent-group="` + g.id + `">` + chrome.RenderDisclosure(chrome.Disclosure{Summary: summary, Body: template.HTML(tbl), Open: len(g.list) <= 6}) + `</div>`)
	}
	conds.WriteString(`</div>`)
	b.WriteString(renderPanel(consolePanel{
		ID: "intent-conditions", Title: "Decisions, futures and questions",
		Question: "Which conditions would change a decision, promote a future or answer a question — and has any fired? Prose is human-judged; only an annotated condition can be shown to fire.",
		Source:   "the intent corpus under " + pc.root, Denominator: v.Fired.Denominator, Freshness: fresh,
		State: v.Fired.State, Reason: v.Fired.Reason, SourceKey: "plans:" + pc.svc, Body: conds.String(),
	}))

	// Unproven claims.
	var un strings.Builder
	if len(v.Unproven) == 0 {
		un.WriteString(`<p class="text-ink-muted">None from the sources that could be computed.</p>`)
	} else {
		rows := make([]table.Row, 0, len(v.Unproven))
		for i, cl := range v.Unproven {
			rows = append(rows, table.Row{ID: fmt.Sprintf("claim-%d", i), Cells: []string{cl.Kind, cl.Ref, clipText(cl.Detail, 200), clipText(cl.Remedy, 200)}})
		}
		un.WriteString(renderKitTable(table.TableParams{
			TableID: "project-unproven", Caption: fmt.Sprintf("%d claims no instrument shows", len(v.Unproven)),
			Columns: []table.Column{{Key: "kind", Label: "Kind"}, {Key: "ref", Label: "Where"}, {Key: "detail", Label: "Claim"}, {Key: "remedy", Label: "Remedy"}},
			Rows:    rows,
		}))
	}
	for _, u := range v.Uncomputable {
		un.WriteString(`<p class="mt-2 text-xs text-caution">not computed — ` + html.EscapeString(u) + `</p>`)
	}
	b.WriteString(renderPanel(consolePanel{
		ID: "intent-unproven", Title: "Unproven claims",
		Question: "What does the corpus claim that nothing proves — and the command or edit that fixes each?",
		Verb:     verb, Source: "fathom intent audit --read-only", Denominator: v.UnprovenCount.Denominator, Freshness: fresh,
		State: v.UnprovenCount.State, Reason: v.UnprovenCount.Reason, SourceKey: "plans:" + pc.svc, Body: un.String(),
	}))

	// Debt over time.
	var debt strings.Builder
	if len(v.Debt) > 0 {
		vals := make([]float64, 0, len(v.Debt))
		rows := make([]table.Row, 0, len(v.Debt))
		for i, d := range v.Debt {
			vals = append(vals, float64(d.Total))
			rows = append(rows, table.Row{ID: fmt.Sprintf("debt-%d", i), Cells: []string{d.At, fmtThousands(d.Total)}})
		}
		if len(vals) > 1 {
			debt.WriteString(`<div class="max-w-xl" data-intent-debt>` + viz.Sparkline(vals, viz.SparklineOptions{W: 480, H: 48, Fluid: true, AriaLabel: "unproven claims per recorded audit"}) + `</div>`)
		}
		tbl := renderKitTable(table.TableParams{
			TableID: "project-intent-debt", Caption: "unproven claims per recorded audit, oldest first",
			Columns: []table.Column{{Key: "at", Label: "Audited (UTC)"}, {Key: "total", Label: "Unproven", Align: "right"}},
			Rows:    rows,
		})
		debt.WriteString(`<div class="mt-3">` + chrome.RenderDisclosure(chrome.Disclosure{Summary: "Debt over time as a table", Body: template.HTML(tbl)}) + `</div>`)
	}
	clock := "no kill clock started (no sibling consumer repository seen yet)"
	if v.ClockStarted != "" {
		clock = "kill clock started " + v.ClockStarted
	}
	b.WriteString(renderPanel(consolePanel{
		ID: "intent-debt", Title: "Debt over time",
		Question: "Is the list of unproven claims shrinking? Each point is one recorded audit (read-only runs record none).",
		Source:   intent.LedgerPath() + " — " + clock, Denominator: v.DebtState.Denominator,
		State: v.DebtState.State, Reason: v.DebtState.Reason, SourceKey: "plans:" + pc.svc, Body: debt.String(),
	}))
	b.WriteString(`</div>`)
	return b.String()
}

// clipText shortens s to at most n runes, marking the cut with "…" — the
// full text stays in the page's JSON and the corpus it cites.
func clipText(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return strings.TrimSpace(string(r[:n-1])) + "…"
}
