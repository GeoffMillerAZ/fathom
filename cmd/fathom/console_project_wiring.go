package main

// console_project_wiring.go — the workspace's Wiring tab at
// /projects/{svc}/wiring (vision 35 §3 "Wiring: doctor checks, drift, and
// receipts"; OC-40): doctor's repository-side rows (`fathom doctor
// --repo-only`, the checks that read only the repository's files — no
// daemon, no harness process), the drift those rows and the preview plan
// name, and the receipts every apply leaves — beside the existing
// preview → apply → undo, which this page offers exactly as /wiring does
// (POST /observatory/wiring/preview behind requireLocalMutation) and never
// performs itself. Rendering the tab only reads.

import (
	"fmt"
	"html"
	"strings"
	"sync"
	"time"

	"github.com/geoffmilleraz/app-kit/appkit/ui/form"
	"github.com/geoffmilleraz/app-kit/appkit/ui/stat"
	"github.com/geoffmilleraz/app-kit/appkit/ui/table"
	"github.com/geoffmilleraz/fathom/internal/console"
	"github.com/geoffmilleraz/fathom/internal/wiring"
)

const (
	verbDoctorRepoOnly = "fathom doctor --path {root} --service {svc} --repo-only --json"
	verbFleetReceipts  = "fathom fleet receipts --repo {root} --json"
)

// projectReceiptsLimit is `fleet receipts`' own default limit.
const projectReceiptsLimit = 20

func init() {
	declarePage(consolePage{
		ID: "project-wiring", Route: "/projects/{svc}/wiring", Group: "Fleet", Order: 18, NavParent: "fleet",
		Title:    "Project wiring",
		Question: "Is fathom wired into this repository as it recorded, what drifted, and what did it write?",
		Verbs:    []string{verbDoctorRepoOnly, verbFleetReceipts},
		Build:    buildProjectWiringPage,
	})
}

func buildProjectWiringPage(cr *consoleRequest) (consoleOut, error) {
	pc, err := loadProjectContext(cr)
	if err != nil {
		return consoleOut{}, err
	}
	out := pc.frame("Wiring")
	out.Verbs = []string{}
	w := projectWiringFor(cr, pc)
	facts := w.facts
	if pc.rootNote == "" {
		vals := map[string]string{"root": pc.root, "svc": pc.svc}
		doctorVerb, receiptsVerb := fillVerb(verbDoctorRepoOnly, vals), fillVerb(verbFleetReceipts, vals)
		out.Verbs = []string{doctorVerb, receiptsVerb}
		out.Sections = map[string]any{doctorVerb: doctorJSONPayload(w.harness, pc.svc, pc.root, w.checks)}
		if w.receiptsErr == nil {
			out.Sections[receiptsVerb] = fleetReceiptsPayload(pc.root, w.receipts, w.total)
		} else {
			out.Sections[receiptsVerb] = map[string]string{"error": w.receiptsErr.Error()}
		}
	}
	view := console.WiringView(facts)
	out.Model = view
	out.Body = func() string {
		return renderProjectTabs(pc.svc, "wiring") + projectRootCallout(pc) + projectBody(renderProjectWiring(pc, view, out.Verbs))
	}
	return out, nil
}

// projectWiringData is one read of a repository's doctor rows, preview
// plan and receipts.
type projectWiringData struct {
	facts       console.WiringFacts
	harness     string
	checks      []doctorCheck
	receipts    []wiring.Receipt
	total       int
	receiptsErr error
	at          time.Time
}

// projectWiringTTL: the preview plan walks the repository's wiring files
// (seconds on a large checkout), so one read per repository is reused for
// a minute; Preview recomputes the plan fresh before anything is applied.
const projectWiringTTL = time.Minute

var projectWiringMemo struct {
	sync.Mutex
	entries map[string]projectWiringData
}

func projectWiringFor(cr *consoleRequest, pc projectContext) projectWiringData {
	key := pc.svc + "\x00" + pc.root + "\x00" + pc.rootNote
	projectWiringMemo.Lock()
	defer projectWiringMemo.Unlock()
	if d, ok := projectWiringMemo.entries[key]; ok && cr.now.Sub(d.at) < projectWiringTTL && !cr.now.Before(d.at) {
		return d
	}
	d := projectWiringData{at: cr.now}
	d.facts = console.WiringFacts{Service: pc.svc, Root: pc.root, RootNote: pc.rootNote, Source: "fathom doctor --repo-only"}
	if pc.rootNote == "" {
		d.harness = resolveDoctorHarness("auto", pc.root)
		d.checks = doctorRepoChecks(pc.root, d.harness)
		for _, c := range d.checks {
			d.facts.Checks = append(d.facts.Checks, console.WiringCheck{Name: c.Name, Status: string(c.Status), Reason: c.Reason, Remedy: c.Remedy})
		}
		// The preview's own engine, read-only: the same plan Preview shows.
		if pv, _, perr := previewFor(mutationTarget{Service: pc.svc, Root: pc.root}); perr != nil {
			d.facts.PlanErr = perr.Error()
		} else {
			d.facts.PlanDigest = pv.PlanDigest
			for _, it := range pv.Plan.Items {
				d.facts.Plan = append(d.facts.Plan, console.WiringPlanItem{File: it.File, Target: it.Target, Verdict: string(it.Verdict), Reason: it.Reason, Cause: it.Cause})
			}
		}
		d.receipts, d.total, d.receiptsErr = loadReceiptsLimited(pc.root, projectReceiptsLimit)
		if d.receiptsErr != nil {
			d.facts.ReceiptsErr = d.receiptsErr.Error()
		} else {
			d.facts.ReceiptsTotal = d.total
			for _, r := range d.receipts {
				d.facts.Receipts = append(d.facts.Receipts, console.WiringReceipt{ID: r.ID, Time: r.Time, Fathom: r.Fathom, UndoOf: r.UndoOf, Files: len(r.Files)})
			}
		}
	}
	if projectWiringMemo.entries == nil || len(projectWiringMemo.entries) > 32 {
		projectWiringMemo.entries = map[string]projectWiringData{}
	}
	projectWiringMemo.entries[key] = d
	return d
}

func renderProjectWiring(pc projectContext, v console.WiringViewModel, verbs []string) string {
	doctorVerb, receiptsVerb := "", ""
	if len(verbs) == 2 {
		doctorVerb, receiptsVerb = verbs[0], verbs[1]
	}
	var b strings.Builder
	b.WriteString(stat.RenderBand([]stat.Stat{
		valueStat("doctor rows failing", v.Fail), valueStat("warning", v.Warn), valueStat("passing", v.Pass),
		valueStat("drift", v.DriftCount), valueStat("receipts", v.ReceiptsTotal),
	}, stat.BandOptions{AriaLabel: pc.svc + " wiring"}))

	// Preview → apply → undo: the existing consent flow, unchanged.
	var flow strings.Builder
	flow.WriteString(`<p class="max-w-3xl text-ink-muted">Preview shows exactly what fathom would write into ` + html.EscapeString(pc.svc) + ` — file by file, before and after — and only then offers Apply. A region fathom tracks that was edited by hand is BLOCKED, never merged (law 15). Every apply leaves a receipt; its result page offers Undo.</p>`)
	if pc.rootNote == "" {
		flow.WriteString(`<form method="post" action="/observatory/wiring/preview" class="mt-3 [&>button]:min-h-11" data-wiring-preview><input type="hidden" name="service" value="` + html.EscapeString(pc.svc) + `">` +
			form.RenderButton(form.Button{Label: "Preview changes", Variant: form.ButtonGhost}) + `</form>`)
		if v.PlanApply.Known() {
			flow.WriteString(`<p class="mt-2 text-xs text-ink-muted" data-wiring-plan>` + html.EscapeString(fmt.Sprintf("The plan today: %s to apply, %s current, %s BLOCKED.",
				plural(v.PlanApply.V, "item"), plural(v.PlanCurrent.V, "item"), plural(v.PlanBlocked.V, "item"))) + `</p>`)
		}
	}
	flowPanel := renderPanel(consolePanel{
		ID: "wiring-flow", Title: "Preview, apply, undo",
		Question:    "What would fathom write here, and can I take it back?",
		Source:      "wiring.PlanWiring — the engine `fathom init` and `fathom fleet reconcile --apply` use",
		Denominator: v.PlanApply.Denominator, State: v.PlanApply.State, Reason: v.PlanApply.Reason,
		SourceKey: "index:" + pc.svc, Body: flow.String(),
	})

	// Drift.
	var drift strings.Builder
	if v.DriftCount.Known() {
		if len(v.Drift) == 0 {
			drift.WriteString(`<p class="text-ink-muted">No drift: doctor's wiring rows pass and the preview plan has nothing to apply or block.</p>`)
		} else {
			drift.WriteString(`<ul class="grid gap-2">`)
			for _, d := range v.Drift {
				drift.WriteString(`<li class="min-w-0 rounded-md border border-line bg-surface px-3 py-2"><p class="font-medium text-ink [overflow-wrap:anywhere]">` + html.EscapeString(d.What) + ` <span class="text-xs font-normal text-ink-muted">(` + html.EscapeString(d.Source) + `)</span></p>` +
					`<p class="text-ink-muted [overflow-wrap:anywhere]">` + html.EscapeString(clipText(d.Detail, 280)) + `</p>`)
				if d.Remedy != "" {
					drift.WriteString(`<p class="mt-1 text-xs text-ink [overflow-wrap:anywhere]">remedy: <code class="font-mono">` + html.EscapeString(clipText(d.Remedy, 240)) + `</code></p>`)
				}
				drift.WriteString(`</li>`)
			}
			drift.WriteString(`</ul>`)
		}
	}
	driftPanel := renderPanel(consolePanel{
		ID: "wiring-drift", Title: "Drift",
		Question: "What differs from what fathom recorded or would write — and the fix for each?",
		Source:   "doctor's wiring rows and the preview plan", Denominator: v.DriftCount.Denominator,
		State: v.DriftCount.State, Reason: v.DriftCount.Reason, SourceKey: "index:" + pc.svc, Body: drift.String(),
	})

	// Doctor rows.
	var doc strings.Builder
	if len(v.Checks) > 0 {
		rows := make([]table.Row, 0, len(v.Checks))
		for _, c := range v.Checks {
			rows = append(rows, table.Row{ID: "check-" + c.Name, Cells: []string{c.Group, c.Name, strings.ToUpper(c.Status), clipText(c.Reason, 360), clipText(c.Remedy, 200)}})
		}
		doc.WriteString(renderKitTable(table.TableParams{
			TableID: "project-doctor", Caption: fmt.Sprintf("%d repository-side doctor rows, wiring first", len(v.Checks)),
			Columns: []table.Column{{Key: "group", Label: "Group"}, {Key: "check", Label: "Check"}, {Key: "status", Label: "Status"}, {Key: "reason", Label: "Reason"}, {Key: "remedy", Label: "Remedy"}},
			Rows:    rows,
		}))
		doc.WriteString(`<p class="mt-2 text-xs text-ink-muted">The full diagnosis — daemon, harness registration, hooks firing, serving — is <code class="font-mono">fathom doctor --path ` + html.EscapeString(shellWord(pc.root)) + `</code> (41 rows; it dials the daemon, so it is not run on a page load).</p>`)
	}
	doctorPanel := renderPanel(consolePanel{
		ID: "wiring-doctor", Title: "Doctor",
		Question: "Do the repository's hooks, gates, stanza, skills and .fathom files say what fathom expects?",
		Verb:     doctorVerb, Source: "doctor's own check functions over " + pc.root, Denominator: v.Fail.Denominator,
		State: v.Fail.State, Reason: v.Fail.Reason, SourceKey: "index:" + pc.svc, Body: doc.String(),
	})

	// Receipts.
	var rec strings.Builder
	if v.ReceiptsTotal.Known() {
		if len(v.Receipts) == 0 {
			rec.WriteString(`<p class="text-ink-muted">No receipts — fathom has written nothing into this repository through apply.</p>`)
		} else {
			rows := make([]table.Row, 0, len(v.Receipts))
			for _, r := range v.Receipts {
				kind := "apply"
				if r.UndoOf != "" {
					kind = "undo of " + r.UndoOf
				}
				rows = append(rows, table.Row{ID: "receipt-" + r.ID, Cells: []string{r.ID, r.Time, kind, fmtThousands(r.Files), r.Fathom}})
			}
			rec.WriteString(renderKitTable(table.TableParams{
				TableID: "project-receipts", Caption: v.ReceiptsTotal.Denominator,
				Columns: []table.Column{{Key: "id", Label: "Receipt"}, {Key: "time", Label: "Time"}, {Key: "kind", Label: "Kind"}, {Key: "files", Label: "Files", Align: "right"}, {Key: "fathom", Label: "fathom"}},
				Rows:    rows,
			}))
			rec.WriteString(`<p class="mt-2 text-xs text-ink-muted">Undo restores a receipt exactly and refuses if a file changed since: <code class="font-mono">fathom fleet undo --repo ` + html.EscapeString(shellWord(pc.root)) + ` --last</code>, or the Undo button on an apply's result.</p>`)
		}
	}
	receiptsPanel := renderPanel(consolePanel{
		ID: "wiring-receipts", Title: "Receipts",
		Question: "What has fathom written into this repository, and can each write be undone?",
		Verb:     receiptsVerb, Source: "the repository's receipt log (wiring.LoadReceipts)", Denominator: v.ReceiptsTotal.Denominator,
		State: v.ReceiptsTotal.State, Reason: v.ReceiptsTotal.Reason, SourceKey: "index:" + pc.svc, Body: rec.String(),
	})
	// The consent flow and what it left side by side; the drift and the
	// doctor rows, which run long, at full width.
	b.WriteString(`<div class="mt-4 grid gap-4 lg:grid-cols-2">` + flowPanel + receiptsPanel + `</div>`)
	b.WriteString(`<div class="mt-4 grid gap-4">` + driftPanel + doctorPanel + `</div>`)
	return b.String()
}
