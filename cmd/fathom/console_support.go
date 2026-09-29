package main

// console_support.go — /support (SM-20..SM-25, design
// docs/design/support-matrix.md §5): the support matrix as a console page.
// Its JSON section is `fathom support --root <checkout> --json` exactly
// (the same join, support.go's buildSupportMatrix, over the same files);
// the panels are internal/console's projections — the heatmap (states as
// shape, label and colour), the dialects against the 40 MiB embed cap, the
// burn-up from the readings ledger and what's next by wave. The filter
// state (kind, wave, engine, state) is held in the URL so an answer can be
// linked. Rendering only reads committed files under the checkout.

import (
	"context"
	"fmt"
	"html"
	"html/template"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/a-h/templ"
	"github.com/geoffmilleraz/app-kit/appkit/ui/form"
	"github.com/geoffmilleraz/app-kit/appkit/ui/stat"
	"github.com/geoffmilleraz/app-kit/appkit/ui/table"
	"github.com/geoffmilleraz/app-kit/appkit/ui/viz"
	"github.com/geoffmilleraz/fathom/internal/console"
	"github.com/geoffmilleraz/fathom/internal/langbench"
	"github.com/geoffmilleraz/fathom/internal/support"
)

const (
	verbSupport     = "fathom support --root {root} --json"
	verbSupportKind = "fathom support --root {root} --kind {kind} --json"
)

// supportTargetsRel marks a checkout the page can project: the shared
// target file the matrix reads beside the language queue.
var supportTargetsRel = filepath.Join("docs", "plans", "support.targets.json")

func init() {
	declarePage(consolePage{
		ID: "support", Route: "/support", Href: "/support", Group: "System", Order: 30, Nav: true,
		Title:    "Support",
		Question: "What does fathom support — every language, tool format, client and integration — at which level, proven by which reading, and what is next?",
		Verbs:    []string{verbSupport, verbSupportKind},
		Build:    buildSupportPage,
	})
}

// supportFilter is the page's URL-held filter state (SM-25): zero values
// mean "every".
type supportFilter struct {
	Kind   string `json:"kind,omitempty"`
	Wave   int    `json:"wave,omitempty"`
	Engine string `json:"engine,omitempty"`
	State  string `json:"state,omitempty"`
}

// supportFilters parses the filter state from a request's query. An unknown
// kind or state, or a wave that is not a positive number, is refused — a
// link naming a filter the page would ignore must not render an unfiltered
// answer under it.
func supportFilters(q url.Values) (supportFilter, error) {
	f := supportFilter{Kind: q.Get("kind"), Engine: q.Get("engine"), State: q.Get("state")}
	if f.Kind != "" && !validSupportKind(f.Kind) {
		return supportFilter{}, fmt.Errorf("unknown kind %q (want %s)", f.Kind, strings.Join(supportKindNames(), ", "))
	}
	if f.State != "" && !validSupportState(f.State) {
		return supportFilter{}, fmt.Errorf("unknown state %q (want %s)", f.State, strings.Join(supportStateNames(), ", "))
	}
	if w := q.Get("wave"); w != "" {
		n, err := strconv.Atoi(w)
		if err != nil || n < 1 {
			return supportFilter{}, fmt.Errorf("wave %q is not a positive number", w)
		}
		f.Wave = n
	}
	return f, nil
}

// query renders the filter back into its URL query, in a fixed key order,
// so a filtered page's links reproduce it exactly.
func (f supportFilter) query() string {
	var parts []string
	add := func(k, v string) {
		if v != "" {
			parts = append(parts, k+"="+url.QueryEscape(v))
		}
	}
	add("kind", f.Kind)
	if f.Wave > 0 {
		add("wave", strconv.Itoa(f.Wave))
	}
	add("engine", f.Engine)
	add("state", f.State)
	return strings.Join(parts, "&")
}

// href is /support with this filter.
func (f supportFilter) href() string {
	if q := f.query(); q != "" {
		return "/support?" + q
	}
	return "/support"
}

// active reports whether any filter is set.
func (f supportFilter) active() bool { return f != supportFilter{} }

// keep reports whether an entry passes the filter: its kind, wave and
// engine match, and (with a state filter) one of its cells is in it.
func (f supportFilter) keep(e support.Entry) bool {
	if f.Kind != "" && string(e.Kind) != f.Kind {
		return false
	}
	if f.Wave > 0 && e.Wave != f.Wave {
		return false
	}
	if f.Engine != "" && e.Engine != f.Engine {
		return false
	}
	if f.State != "" {
		for _, c := range e.Cells {
			if string(c.State) == f.State {
				return true
			}
		}
		return false
	}
	return true
}

func validSupportState(s string) bool {
	for _, st := range console.SupportStates {
		if string(st) == s {
			return true
		}
	}
	return false
}

func supportStateNames() []string {
	out := make([]string, len(console.SupportStates))
	for i, s := range console.SupportStates {
		out[i] = string(s)
	}
	return out
}

// supportRootFor picks the checkout the page projects; tests swap it.
var supportRootFor = resolveSupportRoot

// resolveSupportRoot is ?svc='s registered root when named, else the first
// registered service (by name) whose root carries the support target file;
// svc is that service (its ingest is the panels' live source). note says
// why there is none.
func resolveSupportRoot(cr *consoleRequest) (svc, root, note string) {
	m := cr.model()
	want := cr.r.URL.Query().Get("svc")
	rows := append([]obsServiceRow(nil), m.Services...)
	sort.Slice(rows, func(i, j int) bool { return rows[i].Service < rows[j].Service })
	for _, s := range rows {
		if want != "" && s.Service != want {
			continue
		}
		if s.Path == "" {
			if want != "" {
				return "", "", "no local root is recorded for " + want
			}
			continue
		}
		if _, err := os.Stat(filepath.Join(s.Path, supportTargetsRel)); err != nil {
			if want != "" {
				return "", "", want + "'s root " + s.Path + " carries no " + supportTargetsRel + ": it is not a fathom checkout"
			}
			continue
		}
		return s.Service, s.Path, ""
	}
	if want != "" {
		return "", "", "no registered project is named " + want
	}
	return "", "", "no registered project's root carries " + supportTargetsRel + " — the matrix is projected from a fathom checkout, and none is registered with this daemon"
}

// supportData is one read of a checkout's support inputs.
type supportData struct {
	src      supportSources
	dialects []console.SupportDialectInput
	rows     map[string]console.SupportRowState
	err      error
}

const supportTTL = 30 * time.Second

var supportMemo = consoleMemo[supportData]{max: 8}

// supportKey is the read's input watermarks: every file the join reads
// whose change must re-read it before the TTL.
func supportKey(root string) string {
	key := root
	for _, rel := range []string{
		supportTargetsRel,
		filepath.Join("docs", "plans", "language-breadth.queue.json"),
		filepath.Join("docs", "plans", "language-breadth.md"),
		filepath.Join("docs", "plans", "support-matrix.md"),
		filepath.Join("docs", "bench", "readings", "support-ledger.jsonl"),
		filepath.Join("docs", "bench", "readings", "lang"),
		filepath.Join("internal", "extractor", "tsengine", "grammars", "grammars.lock.json"),
	} {
		key += "\x00" + fileWatermark(filepath.Join(root, rel))
	}
	return key
}

func supportDataFor(ctx context.Context, root string, now time.Time) supportData {
	d, _, _ := supportMemo.doCtx(ctx, supportKey(root), now, func() (supportData, time.Duration) {
		return readSupportData(ctx, root), supportTTL
	})
	return d
}

func readSupportData(ctx context.Context, root string) supportData {
	src, err := loadSupportSources(ctx, root)
	if err != nil {
		return supportData{err: err}
	}
	d := supportData{src: src}
	if d.dialects, err = loadSupportDialects(root, src); err != nil {
		return supportData{err: err}
	}
	if d.rows, err = loadSupportRowStates(root); err != nil {
		return supportData{err: err}
	}
	return d
}

// loadSupportDialects joins each dialect data directory's manifest to its
// grammar lock row and its reading (langbench reads the committed JSON).
func loadSupportDialects(root string, src supportSources) ([]console.SupportDialectInput, error) {
	ds, err := langbench.DialectDataFromRepo(root)
	if err != nil {
		return nil, err
	}
	pins, err := langbench.GrammarPinsFromRepo(root)
	if err != nil {
		return nil, err
	}
	out := make([]console.SupportDialectInput, 0, len(ds))
	for _, m := range ds {
		d := console.SupportDialectInput{ID: m.ID, Name: m.Display, Engine: m.Engine, Blob: m.Blob}
		if g, ok := pins[m.Blob]; ok && m.Blob != "" {
			d.InLock, d.BlobBytes = true, g.GzBytes
			d.GrammarRepo, d.GrammarTag, d.Licence = g.Repo, g.Tag, g.Licence
		}
		if r, ok := src.Readings[m.ID]; ok {
			d.LevelMet = r.LevelMet
			if len(r.MeasuredAt) >= 10 {
				d.ReadingDate = r.MeasuredAt[:10]
			}
		}
		out = append(out, d)
	}
	return out, nil
}

// loadSupportRowStates is the checkbox of every row in the two plans the
// target rows belong to (the language queue's and the support matrix's).
func loadSupportRowStates(root string) (map[string]console.SupportRowState, error) {
	marks, err := support.PlanRowMarks(root, "language-breadth.md", "support-matrix.md")
	if err != nil {
		return nil, err
	}
	rows := make(map[string]console.SupportRowState, len(marks))
	for id, m := range marks {
		st := console.SupportRowOpen
		switch m {
		case support.RowMarkClaimed:
			st = console.SupportRowClaimed
		case support.RowMarkDispositioned:
			st = console.SupportRowDispositioned
		case support.RowMarkRefused:
			st = console.SupportRowRefused
		}
		rows[id] = st
	}
	return rows, nil
}

// supportNextItems are the target rows What's next weighs: the queue's
// items (their needs are the plan row's depends_on) and the shared target
// rows.
func supportNextItems(src supportSources) []console.SupportNextItem {
	var out []console.SupportNextItem
	for _, q := range src.Queue {
		kind := support.KindLanguage
		if q.Kind == "tooling" {
			kind = support.KindTool
		}
		out = append(out, console.SupportNextItem{Kind: kind, ID: q.ID, Name: q.Name, Wave: q.Wave, PlanRow: q.PlanRow, Needs: q.DependsOn})
	}
	for _, t := range src.Targets {
		out = append(out, console.SupportNextItem{Kind: t.Kind, ID: t.ID, Name: t.Name, Wave: t.Wave, PlanRow: t.PlanRow})
	}
	return out
}

// supportMilestones are the queue's wave lines: after wave N, every
// language and tool format in waves 1..N at target.
func supportMilestones(src supportSources) []console.SupportMilestone {
	perWave := map[support.Kind]map[int]int{}
	maxWave := 0
	for _, q := range src.Queue {
		kind := support.KindLanguage
		if q.Kind == "tooling" {
			kind = support.KindTool
		}
		if perWave[kind] == nil {
			perWave[kind] = map[int]int{}
		}
		perWave[kind][q.Wave]++
		if q.Wave > maxWave {
			maxWave = q.Wave
		}
	}
	var out []console.SupportMilestone
	for _, kind := range []support.Kind{support.KindLanguage, support.KindTool} {
		total := 0
		for w := 1; w <= maxWave; w++ {
			total += perWave[kind][w]
			if perWave[kind][w] > 0 {
				out = append(out, console.SupportMilestone{Kind: kind, Label: fmt.Sprintf("after wave %d", w), Count: total})
			}
		}
	}
	return out
}

// supportMilestonesFor keeps the milestones of the filtered kind.
func supportMilestonesFor(ms []console.SupportMilestone, kind string) []console.SupportMilestone {
	if kind == "" {
		return ms
	}
	out := []console.SupportMilestone{}
	for _, m := range ms {
		if string(m.Kind) == kind {
			out = append(out, m)
		}
	}
	return out
}

// supportPageModel is the page's own projection (the JSON "model").
type supportPageModel struct {
	Root     string                      `json:"root,omitempty"`
	Note     string                      `json:"note,omitempty"`
	Filters  supportFilter               `json:"filters"`
	Entries  int                         `json:"entries"`
	Buckets  map[string]int              `json:"buckets"`
	Heatmap  []console.SupportHeatGroup  `json:"heatmap"`
	Dialects console.SupportDialectsView `json:"dialects"`
	BurnUp   console.SupportBurnUpView   `json:"burn_up"`
	Next     console.SupportNextView     `json:"next"`
	Engines  []string                    `json:"engines"`
	Waves    []int                       `json:"waves"`
}

func buildSupportPage(cr *consoleRequest) (consoleOut, error) {
	f, err := supportFilters(cr.r.URL.Query())
	if err != nil {
		return consoleOut{}, &consoleError{Status: http.StatusBadRequest, Title: "Bad filter", Reason: err.Error(), Remedy: "open /support and pick a filter from its form", RemedyHref: "/support", RemedyLabel: "Support"}
	}
	svc, root, note := supportRootFor(cr)
	model := supportPageModel{Filters: f, Note: note, Root: root, Buckets: map[string]int{}}
	out := consoleOut{Verbs: []string{}, Model: &model}
	if note != "" {
		out.Meta = []string{"no checkout to project"}
		out.Body = func() string { return renderSupportAbsent(note) }
		return out, nil
	}
	d := supportDataFor(cr.r.Context(), root, cr.now)
	if d.err != nil {
		return consoleOut{}, &consoleError{Status: http.StatusInternalServerError, Title: "Support inputs unreadable", Reason: d.err.Error(), Remedy: "fathom support --root " + root + " --json"}
	}
	matrix := buildSupportMatrix(d.src, func() time.Time { return cr.now })

	verb := fillVerb(verbSupport, map[string]string{"root": root})
	section := matrix
	if f.Kind != "" {
		verb = fillVerb(verbSupportKind, map[string]string{"root": root, "kind": f.Kind})
		section.Entries = filterEntriesByKind(matrix.Entries, support.Kind(f.Kind))
	}
	out.Verbs = []string{verb}
	out.Sections = map[string]any{verb: section}

	engines, waves := map[string]bool{}, map[int]bool{}
	var kept []support.Entry
	for _, e := range matrix.Entries {
		if e.Engine != "" {
			engines[e.Engine] = true
		}
		if e.Wave > 0 {
			waves[e.Wave] = true
		}
		if f.keep(e) {
			kept = append(kept, e)
			model.Buckets[entrySummaryBucket(e)]++
		}
	}
	for e := range engines {
		model.Engines = append(model.Engines, e)
	}
	sort.Strings(model.Engines)
	for w := range waves {
		model.Waves = append(model.Waves, w)
	}
	sort.Ints(model.Waves)
	model.Entries = len(kept)
	filtered := matrix
	filtered.Entries = kept
	model.Heatmap = console.SupportHeatmap(filtered)

	var dialects []console.SupportDialectInput
	for _, di := range d.dialects {
		if f.Engine == "" || di.Engine == f.Engine {
			dialects = append(dialects, di)
		}
	}
	model.Dialects = console.SupportDialects(dialects, console.SupportEmbedCapBytes)
	model.BurnUp = console.SupportBurnUp(console.SupportBurnUpInput{Ledger: d.src.Ledger, Requirements: console.SupportRequirements(matrix), Milestones: supportMilestonesFor(supportMilestones(d.src), f.Kind)})

	var next []console.SupportNextItem
	for _, it := range supportNextItems(d.src) {
		if (f.Kind == "" || string(it.Kind) == f.Kind) && (f.Wave == 0 || it.Wave == f.Wave) {
			next = append(next, it)
		}
	}
	model.Next = console.SupportNext(next, d.rows)

	out.Meta = []string{supportCount(model.Entries, "entry", "entries"), "checkout " + root}
	if matrix.Commit != "" {
		out.Meta = append(out.Meta, "at "+shortCommit(matrix.Commit))
	}
	out.Body = func() string { return renderSupportPage(model, verb, "index:"+svc+" plans:"+svc, cr.now) }
	return out, nil
}

// supportCount is "1 entry" / "3 entries".
func supportCount(n int, one, many string) string {
	if n == 1 {
		return fmtThousands(n) + " " + one
	}
	return fmtThousands(n) + " " + many
}

func renderSupportAbsent(note string) string {
	return renderPanel(consolePanel{
		ID: "support-matrix", SourceKey: "index", Title: "Support matrix",
		Question:    "Which checkout is this matrix projected from?",
		Verb:        "fathom support --json",
		Source:      "the registered projects' roots",
		Denominator: "every registered project",
		Freshness:   "as rendered",
		State:       console.StateAbsent, Reason: note,
		Body: `<p class="text-sm text-ink-muted">Run <code class="font-mono">fathom support --json</code> in a fathom checkout, or register one with this daemon (<code class="font-mono">fathom ingest --service fathom --path &lt;checkout&gt;</code>) and reload.</p>`,
	})
}

// supportRoleClass maps a mark's role to its text utility, written out in
// full so the Tailwind scan sees every class.
func supportRoleClass(role string) string {
	switch role {
	case "positive":
		return "text-positive"
	case "caution":
		return "text-caution"
	case "danger":
		return "text-danger"
	case "accent":
		return "text-accent"
	}
	return "text-ink-muted"
}

func renderSupportPage(m supportPageModel, verb, sourceKey string, now time.Time) string {
	var b strings.Builder
	b.WriteString(stat.RenderBand([]stat.Stat{
		{Label: "entries", Value: fmtThousands(m.Entries), Sub: supportFilterSub(m.Filters)},
		{Label: "at target", Value: fmtThousands(m.Buckets["at-target"]), ValueRole: "positive", Sub: "every targeted cell proven"},
		{Label: "below target", Value: fmtThousands(m.Buckets["below"]), ValueRole: "caution", Sub: "a reading, short of its target — or regressed"},
		{Label: "tolerated", Value: fmtThousands(m.Buckets["tolerated"]), Sub: "registered, no reading"},
		{Label: "declared", Value: fmtThousands(m.Buckets["declared"]), Sub: "planned, not built"},
	}, stat.BandOptions{AriaLabel: "support totals"}))
	b.WriteString(renderSupportFilterForm(m))
	freshness := "as of " + now.UTC().Format(time.RFC3339) + " (re-read when a committed input changes)"
	b.WriteString(`<div class="mt-4 flex flex-col gap-4">`)
	b.WriteString(renderPanel(consolePanel{
		ID: "support-heatmap", SourceKey: sourceKey, Title: "Heatmap",
		Question:    "Which level or capability has each entry reached, and on what evidence?",
		Verb:        verb,
		Source:      "the registries, docs/plans/support.targets.json, the language queue, docs/bench/readings/ and the plan rows",
		Denominator: supportCount(m.Entries, "entry", "entries") + " shown, each cell a computed state",
		Freshness:   freshness,
		Body:        renderSupportHeatmap(m),
	}))
	b.WriteString(renderPanel(consolePanel{
		ID: "support-dialects", SourceKey: sourceKey, Title: "Dialects",
		Question:    "Which dialect data directories exist, what grammar does each pin, and how near are the embedded grammars to the 40 MiB cap?",
		Verb:        verb,
		Source:      "internal/extractor/dialects/*/dialect.json, internal/extractor/tsengine/grammars/grammars.lock.json and docs/bench/readings/lang/",
		Denominator: supportCount(len(m.Dialects.Rows), "dialect directory", "dialect directories") + "; embedded bytes counted once per blob",
		Freshness:   freshness,
		Body:        renderSupportDialects(m.Dialects),
	}))
	burnState, burnReason := console.State(""), ""
	if len(m.BurnUp.Series) == 0 {
		burnState, burnReason = console.StateAbsent, m.BurnUp.Note
	}
	b.WriteString(renderPanel(consolePanel{
		ID: "support-burnup", SourceKey: sourceKey, Title: "Progress over time",
		Question:    "How many entries of each kind were at target on each day a reading was written?",
		Verb:        verb,
		Source:      "docs/bench/readings/support-ledger.jsonl and the language queue's waves",
		Denominator: "entries with a declared target, per kind",
		Freshness:   freshness,
		State:       burnState, Reason: burnReason,
		Body: renderSupportBurnUp(m.BurnUp),
	}))
	b.WriteString(renderPanel(consolePanel{
		ID: "support-next", SourceKey: sourceKey, Title: "What's next",
		Question:    "Which target rows can be worked now — every need claimed or dispositioned — by wave?",
		Verb:        verb,
		Source:      "the language queue's depends_on, docs/plans/support.targets.json and the checkboxes of docs/plans/language-breadth.md and support-matrix.md (the language queue's rule, docs/design/language-breadth.md §13.0 step 2: a claimed or dispositioned row meets a need)",
		Denominator: fmt.Sprintf("%d eligible, %d blocked, %d done", m.Next.Eligible, m.Next.Blocked, m.Next.Done),
		Freshness:   freshness,
		Body:        renderSupportNext(m.Next),
	}))
	b.WriteString(`</div>`)
	return b.String()
}

func supportFilterSub(f supportFilter) string {
	if !f.active() {
		return "every kind"
	}
	return "filtered: " + strings.ReplaceAll(f.query(), "&", ", ")
}

func renderSupportFilterForm(m supportPageModel) string {
	opts := func(every string, vals []string) []form.Option {
		o := []form.Option{{Value: "", Label: every}}
		for _, v := range vals {
			o = append(o, form.Option{Value: v, Label: v})
		}
		return o
	}
	waves := make([]string, len(m.Waves))
	for i, w := range m.Waves {
		waves[i] = strconv.Itoa(w)
	}
	wave := ""
	if m.Filters.Wave > 0 {
		wave = strconv.Itoa(m.Filters.Wave)
	}
	var b strings.Builder
	b.WriteString(`<form method="get" action="/support" class="mt-4 grid gap-3 sm:grid-cols-2 lg:grid-cols-5 [&_select]:min-h-11" aria-label="filter the support matrix" data-support-filters>`)
	b.WriteString(form.RenderField(form.Field{Label: "Kind", For: "support-kind", Control: template.HTML(form.RenderSelect(form.Select{ID: "support-kind", Name: "kind", Value: m.Filters.Kind, Options: opts("every kind", supportKindNames())}))}))
	b.WriteString(form.RenderField(form.Field{Label: "Wave", For: "support-wave", Control: template.HTML(form.RenderSelect(form.Select{ID: "support-wave", Name: "wave", Value: wave, Options: opts("every wave", waves)}))}))
	b.WriteString(form.RenderField(form.Field{Label: "Engine", For: "support-engine", Control: template.HTML(form.RenderSelect(form.Select{ID: "support-engine", Name: "engine", Value: m.Filters.Engine, Options: opts("every engine", m.Engines)}))}))
	b.WriteString(form.RenderField(form.Field{Label: "State", For: "support-state", Control: template.HTML(form.RenderSelect(form.Select{ID: "support-state", Name: "state", Value: m.Filters.State, Options: opts("every state", supportStateNames())}))}))
	b.WriteString(`<div class="flex items-end gap-2 [&>a]:min-h-11 [&>button]:min-h-11">` + form.RenderButton(form.Button{Label: "Filter", Variant: form.ButtonPrimary}) +
		form.RenderButton(form.Button{Label: "Clear", Href: "/support", Variant: form.ButtonGhost}) + `</div>`)
	b.WriteString(`</form>`)
	if m.Filters.active() {
		b.WriteString(`<p class="mt-2 text-xs text-ink-muted [overflow-wrap:anywhere]">This view's link: <a class="text-accent underline" href="` + html.EscapeString(m.Filters.href()) + `" data-support-permalink>` + html.EscapeString(m.Filters.href()) + `</a></p>`)
	}
	return b.String()
}

// renderSupportMark draws a state as its shape and label in its role — the
// shape and the word carry the state on their own; the colour only adds.
func renderSupportMark(mk console.SupportMark) string {
	return `<span class="inline-flex items-center gap-1 whitespace-nowrap ` + supportRoleClass(mk.Role) + `"><span aria-hidden="true" class="font-mono">` + html.EscapeString(mk.Shape) + `</span><span>` + html.EscapeString(mk.Label) + `</span></span>`
}

func renderSupportHeatmap(m supportPageModel) string {
	var b strings.Builder
	b.WriteString(`<ul class="mb-3 flex flex-wrap gap-x-4 gap-y-1 text-xs" aria-label="legend" data-support-legend>`)
	for _, s := range append(append([]support.State{}, console.SupportStates...), "") {
		b.WriteString(`<li>` + renderSupportMark(console.SupportStateMark(s)) + `</li>`)
	}
	b.WriteString(`</ul>`)
	if len(m.Heatmap) == 0 {
		b.WriteString(`<p class="italic text-ink-muted">No entry matches this filter.</p>`)
		return b.String()
	}
	for _, g := range m.Heatmap {
		cols := []table.Column{{Key: "entry", Label: "Entry"}}
		for _, c := range g.Columns {
			cols = append(cols, table.Column{Key: "cap-" + c, Label: c})
		}
		var rows []table.Row
		for _, r := range g.Rows {
			cells := []string{r.Name}
			sub := r.ID
			if r.Wave > 0 {
				sub += " · wave " + strconv.Itoa(r.Wave)
			}
			if r.Engine != "" {
				sub += " · " + r.Engine
			}
			htmlCells := []templ.Component{templ.Raw(`<span class="font-medium text-ink">` + html.EscapeString(r.Name) + `</span><span class="block font-mono text-xs text-ink-muted [overflow-wrap:anywhere]">` + html.EscapeString(sub) + `</span>`)}
			for _, c := range r.Cells {
				cells = append(cells, c.Mark.Label)
				htmlCells = append(htmlCells, templ.Raw(renderSupportCell(c)))
			}
			rows = append(rows, table.Row{ID: "support-" + string(r.Kind) + "-" + r.ID, Cells: cells, HTMLCells: htmlCells})
		}
		b.WriteString(`<h3 class="mt-4 text-sm font-semibold text-ink">` + html.EscapeString(string(g.Kind)) + ` <span class="font-normal text-ink-muted">(` + supportCount(len(g.Rows), "entry", "entries") + `)</span></h3>`)
		b.WriteString(renderKitTable(table.TableParams{
			TableID: "support-heatmap-" + string(g.Kind),
			Caption: string(g.Kind) + " entries × " + strings.Join(g.Columns, ", "),
			Columns: cols,
			Rows:    rows,
		}))
	}
	return b.String()
}

// renderSupportCell is one heatmap cell: the mark as a disclosure whose
// hover title and opened body name the reading and the plan row verdict.
func renderSupportCell(c console.SupportHeatCell) string {
	if c.State == "" {
		return `<span title="` + html.EscapeString(c.Title) + `" data-support-state="none">` + renderSupportMark(c.Mark) + `</span>`
	}
	var body []string
	if c.Target != "" {
		body = append(body, "target: "+c.Target)
	}
	if c.Reading != "" {
		r := "reading: " + c.Reading
		if c.LevelMet != "" {
			r += " (level met " + string(c.LevelMet) + ")"
		}
		body = append(body, r)
	} else {
		body = append(body, "no reading")
	}
	if c.PlanRow != "" {
		v := string(c.Verdict)
		if v == "" {
			v = "no verdict"
		}
		body = append(body, "plan row: "+c.PlanRow+" — "+v)
	}
	var lines strings.Builder
	for _, l := range body {
		lines.WriteString(`<span class="block">` + html.EscapeString(l) + `</span>`)
	}
	return `<details class="min-w-0" data-support-state="` + html.EscapeString(string(c.State)) + `"><summary class="inline-flex min-h-11 cursor-pointer items-center" title="` + html.EscapeString(c.Title) + `">` + renderSupportMark(c.Mark) + `</summary>` +
		`<span class="mt-1 block text-xs text-ink-muted [overflow-wrap:anywhere]" data-support-evidence>` + lines.String() + `</span></details>`
}

func fmtMiB(n int64) string { return fmt.Sprintf("%.1f MiB", float64(n)/(1<<20)) }

func renderSupportDialects(v console.SupportDialectsView) string {
	var b strings.Builder
	role := "accent"
	if v.NearCap {
		role = "caution"
	}
	b.WriteString(`<div class="mb-3 flex flex-wrap items-center gap-3" data-support-cap><span class="text-sm text-ink">` + html.EscapeString("Embedded grammars: "+fmtMiB(v.Total)+" of "+fmtMiB(v.Cap)) + `</span>` +
		viz.MeterRow(float64(v.Total), float64(v.Cap), viz.MeterOptions{W: 240, H: 16, FillRole: role, ShowPct: true, AriaLabel: "embedded grammar bytes against the 40 MiB cap"}) + `</div>`)
	trigger := fmt.Sprintf("%s headroom. Dialect packs (the out-of-binary install path, LB-F14/LB-F27) are deferred until the embedded grammars near the cap; this line is their trigger.", fmtMiB(v.Headroom))
	if v.NearCap {
		trigger = fmt.Sprintf("Near the cap: %s headroom. The dialect-packs trigger has fired (LB-F14/LB-F27).", fmtMiB(v.Headroom))
	}
	b.WriteString(`<p class="mb-3 text-xs ` + map[bool]string{true: "text-caution", false: "text-ink-muted"}[v.NearCap] + `" data-support-trigger>` + html.EscapeString(trigger) + `</p>`)
	var rows []table.Row
	for _, r := range v.Rows {
		size := `<span class="italic text-ink-muted" title="` + html.EscapeString(r.Embedded.Reason) + `">none — ` + html.EscapeString(r.Embedded.Reason) + `</span>`
		if r.Embedded.Known() {
			size = fmtMiB(int64(r.Embedded.V))
			if r.SharedBlob {
				size += ` <span class="block text-xs text-ink-muted">blob ` + html.EscapeString(r.Blob) + ` shared, counted once</span>`
			}
		}
		level := r.LevelMet
		if level == "" {
			level = "no reading"
		}
		rows = append(rows, table.Row{
			ID:    "dialect-" + r.ID,
			Cells: []string{r.Name, r.Engine, orDash(r.Pin), orDash(r.Licence), level, orDash(r.ReadingDate), "", fmtMiB(r.RunningTotal)},
			HTMLCells: []templ.Component{
				templ.Raw(`<span class="font-medium text-ink">` + html.EscapeString(orDash(r.Name)) + `</span><span class="block font-mono text-xs text-ink-muted">dialects/` + html.EscapeString(r.ID) + `</span>`),
				nil, nil, nil, nil, nil, templ.Raw(size), nil,
			},
		})
	}
	b.WriteString(renderKitTable(table.TableParams{
		TableID: "support-dialects",
		Caption: "Dialect data directories with their grammar pin and embedded size",
		Columns: []table.Column{
			{Key: "dialect", Label: "Dialect"}, {Key: "engine", Label: "Engine"}, {Key: "pin", Label: "Grammar pin"},
			{Key: "licence", Label: "Licence"}, {Key: "level", Label: "Level reached"}, {Key: "date", Label: "Last reading"},
			{Key: "size", Label: "Embedded", Align: "right"}, {Key: "total", Label: "Running total", Align: "right"},
		},
		Rows: rows,
	}))
	return b.String()
}

func renderSupportBurnUp(v console.SupportBurnUpView) string {
	var b strings.Builder
	if len(v.Series) > 0 {
		b.WriteString(`<p class="mb-2 text-xs text-ink-muted">` + html.EscapeString(v.From+" to "+v.To+"; a day with no reading of a kind is drawn as a gap, never as 0.") + `</p>`)
	}
	for _, s := range v.Series {
		b.WriteString(`<div class="mt-3 min-w-0" data-support-series="` + html.EscapeString(string(s.Kind)) + `"><h3 class="text-sm font-semibold text-ink">` + html.EscapeString(string(s.Kind)) + ` <span class="font-normal text-ink-muted">` + html.EscapeString(fmt.Sprintf("of %d with a target · %d measured, %d absent days", s.Entries, s.Measured, s.Absent)) + `</span></h3>`)
		b.WriteString(renderSupportStrip(s))
		b.WriteString(`</div>`)
	}
	if len(v.Milestones) > 0 {
		b.WriteString(`<h3 class="mt-4 text-sm font-semibold text-ink">Milestones from the queue</h3><ul class="mt-1 flex flex-col gap-1 text-sm" data-support-milestones>`)
		for _, ms := range v.Milestones {
			b.WriteString(`<li>` + html.EscapeString(fmt.Sprintf("%s %s: %d at target", ms.Kind, ms.Label, ms.Count)) + `</li>`)
		}
		b.WriteString(`</ul>`)
	}
	return b.String()
}

// renderSupportStrip draws one kind's days: a kit Sparkline per run of
// measured days and a dashed chip per run of absent days, in calendar
// order, so no stroke spans a day nothing was read (doctrine rule 6).
func renderSupportStrip(s console.SupportBurnSeries) string {
	var b strings.Builder
	b.WriteString(`<div class="mt-1 flex flex-wrap items-center gap-2">`)
	i := 0
	for i < len(s.Days) {
		j := i
		measured := s.Days[i].AtTarget.Known()
		for j < len(s.Days) && s.Days[j].AtTarget.Known() == measured {
			j++
		}
		run := dateRun{From: s.Days[i].Date, To: s.Days[j-1].Date, N: j - i}
		if !measured {
			b.WriteString(`<span class="shrink-0 rounded-md border border-dashed border-line px-2 py-1 text-xs italic text-ink-muted" title="` + html.EscapeString("no "+string(s.Kind)+" reading "+run.span()) + `">` + html.EscapeString(run.label()) + `</span>`)
		} else {
			vals := make([]float64, 0, j-i)
			last := 0
			for _, d := range s.Days[i:j] {
				vals = append(vals, float64(d.AtTarget.V))
				last = d.AtTarget.V
			}
			w := 20 + 12*(j-i)
			if w > 280 {
				w = 280
			}
			b.WriteString(`<span class="inline-flex shrink-0 items-center gap-1">` + viz.Sparkline(vals, viz.SparklineOptions{W: w, H: 32, FillRole: "accent-soft", AriaLabel: fmt.Sprintf("%s at target, %s", s.Kind, run.span())}) +
				`<span class="text-xs text-ink-muted">` + html.EscapeString(fmt.Sprintf("%d on %s", last, s.Days[j-1].Date)) + `</span></span>`)
		}
		i = j
	}
	b.WriteString(`</div>`)
	return b.String()
}

func renderSupportNext(v console.SupportNextView) string {
	if len(v.Waves) == 0 {
		return `<p class="italic text-ink-muted">Every target row in this view is claimed or dispositioned.</p>`
	}
	var b strings.Builder
	for _, w := range v.Waves {
		counts := fmt.Sprintf("%d eligible, %d blocked", len(w.Items), w.Blocked)
		if len(w.Replan) > 0 {
			counts += fmt.Sprintf(", %d needs re-planning", len(w.Replan))
		}
		b.WriteString(`<div class="mt-3 min-w-0" data-support-wave="` + strconv.Itoa(w.Wave) + `"><h3 class="text-sm font-semibold text-ink">` + html.EscapeString(fmt.Sprintf("Wave %d", w.Wave)) + ` <span class="font-normal text-ink-muted">` + html.EscapeString(counts) + `</span></h3>`)
		if len(w.Items) == 0 && len(w.Replan) == 0 {
			b.WriteString(`<p class="text-sm italic text-ink-muted">Nothing eligible: every open row here waits on an open or deferred need.</p></div>`)
			continue
		}
		if len(w.Items) > 0 {
			b.WriteString(`<ul class="mt-1 flex flex-col gap-1 text-sm">`)
			for _, it := range w.Items {
				b.WriteString(`<li class="min-w-0 [overflow-wrap:anywhere]"><span class="font-medium text-ink">` + html.EscapeString(it.Name) + `</span> <span class="text-ink-muted">` + html.EscapeString(fmt.Sprintf("%s · %s · %d needs met", it.Kind, it.PlanRow, it.Needs)) + `</span></li>`)
			}
			b.WriteString(`</ul>`)
		}
		if len(w.Replan) > 0 {
			// G2-6: a refused need will not be built; the item needs a new plan,
			// not a wait.
			b.WriteString(`<p class="text-sm italic text-ink-muted">Needs re-planning: a need was refused.</p><ul class="mt-1 flex flex-col gap-1 text-sm">`)
			for _, it := range w.Replan {
				b.WriteString(`<li class="min-w-0 [overflow-wrap:anywhere]"><span class="font-medium text-ink">` + html.EscapeString(it.Name) + `</span> <span class="text-ink-muted">` + html.EscapeString(fmt.Sprintf("%s · %s", it.Kind, it.PlanRow)) + `</span></li>`)
			}
			b.WriteString(`</ul>`)
		}
		b.WriteString(`</div>`)
	}
	return b.String()
}
