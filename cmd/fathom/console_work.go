package main

// console_work.go — /work (WI-8, WI-11, WI-13; docs/design/
// work-intelligence.md §3): the work-intelligence pages for one
// repository — the model and harness scorecard (U5), the interest rate per
// zone (U2), cost per zone with the first-attempt split (U1), the friction
// map with its proposed remedies (U3), and hotspots (U13). Every figure is
// a projection in internal/console over records that already exist: the
// verify ledger's proof history joined to the session ledger (PE-15), one
// read-only `git show` for the proving commits' files, the rule-outcome
// log (GL-10), and T1's stored risk factors. Read-only by construction;
// remedies are proposals (WI-X4); rows are agent identities, never people
// (I58). The computation is memoized per (repository, ledgers, HEAD).

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/a-h/templ"
	"github.com/geoffmilleraz/app-kit/appkit/ui/chrome"
	"github.com/geoffmilleraz/app-kit/appkit/ui/table"
	"github.com/geoffmilleraz/fathom/internal/certify"
	"github.com/geoffmilleraz/fathom/internal/config"
	"github.com/geoffmilleraz/fathom/internal/console"
	"github.com/geoffmilleraz/fathom/internal/gitro"
)

func init() {
	declarePage(consolePage{
		ID: "work", Route: "/work", Href: "/work", Group: "Proof", Order: 6, Nav: true,
		Title:    "Work intelligence",
		Question: "Where does agent effort go by zone, where do agents struggle, and which agent identity does which work here?",
		Build:    buildWorkPage,
	})
}

// Bounds on one /work computation (tenancy: no whole-tree read, one git
// exec per step, every read a bounded tail).
const (
	workMaxCommits     = 200     // proving commits whose files are read
	workMaxAuthorLooks = 200     // commit author-time lookups for the join
	workRuleTailBytes  = 4 << 20 // the rule-outcome log's newest bytes
	workHotspotRows    = 20
)

// workPageModel is /work's JSON model.
type workPageModel struct {
	Service      string                 `json:"service,omitempty"`
	Repositories []string               `json:"repositories"`
	History      console.SourceRead     `json:"history_read"`
	Unjoined     int                    `json:"unjoined_proven_rows"`
	Commits      string                 `json:"commits_read"`
	Work         console.WorkModel      `json:"work"`
	Friction     console.FrictionView   `json:"friction"`
	FrictionRead []string               `json:"friction_sources"`
	Hotspots     *console.HotspotsModel `json:"hotspots,omitempty"`
	HotspotsErr  string                 `json:"hotspots_error,omitempty"`
}

var workMemo = consoleMemo[workPageModel]{max: 16}

func buildWorkPage(cr *consoleRequest) (consoleOut, error) {
	m := cr.model()
	roots, scope, err := plansScope(cr, m)
	if err != nil {
		return consoleOut{}, err
	}
	repos := []string{}
	for _, sr := range modelServiceRoots(m) {
		repos = append(repos, sr.Service)
	}
	// Without a repository every section is its empty projection: a zero
	// Value carries no truth state and would not serialize.
	model := workPageModel{Service: scope, Repositories: repos, Work: console.WorkIntelligence(nil, nil),
		Friction: console.FrictionWithRemedies(console.FrictionInputs{}, console.RemedyInputs{})}
	meta := []string{"choose a repository", "generated " + m.GeneratedAt}
	if scope != "" {
		sr := roots[0]
		key := strings.Join([]string{sr.Service, sr.Root, fileWatermark(verifyLedgerPath()), fileWatermark(sessionLedgerPath()),
			fileWatermark(ruleOutcomeLogPath()), repoHeadNoFork(sr.Root)}, "\x00")
		ctx := cr.r.Context()
		model, _, _ = workMemo.doCtx(ctx, key, cr.now, func() (workPageModel, time.Duration) {
			return computeWork(ctx, m, sr, cr.now), 2 * time.Minute
		})
		model.Service, model.Repositories = scope, repos
		if r := readTestPortfolio(ctx, plansFor(cr.deps), sr); r.err != "" {
			model.HotspotsErr = r.err
		} else {
			hs := workHotspots(ctx, cr.deps, sr, r.risk, cr.now)
			model.Hotspots = &hs
		}
		meta = []string{scope, "generated " + m.GeneratedAt}
	}
	return consoleOut{Meta: meta, Model: model, Verbs: []string{}, Body: func() string { return renderWorkPage(model) }}, nil
}

// computeWork joins the proof history to the session ledger for one
// repository and projects U1, U2, U5 and U3.
func computeWork(ctx context.Context, m observatoryModel, sr serviceRoot, now time.Time) workPageModel {
	var model workPageModel
	h, read := console.ProofHistory(ctx, verifyLedgerPath(), console.DefaultHistoryTail)
	model.History = read
	f := console.HistoryFilter{Service: sr.Service}
	looks := 0
	authorAt := func(ctx context.Context, root, commit string) (string, bool) {
		if looks >= workMaxAuthorLooks {
			return "", false
		}
		looks++
		return console.CommitAuthorAt(ctx, root, commit)
	}
	join := console.JoinSessionProof(ctx, h, agentSessionsOf(m), f, authorAt)
	model.Unjoined = len(join.Unjoined)
	commits, note := workCommitFacts(ctx, sr.Root, join)
	model.Commits = note
	zones := console.NewZones(workZonesConfig(sr.Root), nil)
	changes := console.EffortPerProvenChange(join, h, sessionEffortsOf(m), commits, zones)
	model.Work = console.WorkIntelligence(changes, nil)

	rules, ruleNote := workRuleEvents(sr.Root)
	model.FrictionRead = []string{
		ruleNote,
		"search, lookup and revert events: not read by this page (the hook log's replay carries no symbol per call)",
		"claimed-unproven anchors: not read by this page",
	}
	model.Friction = console.FrictionWithRemedies(console.FrictionInputs{Rules: rules, Zones: zones}, console.RemedyInputs{})
	return model
}

// sessionEffortsOf reads each session's effort from its last ledger row.
// The ledger records neither harness nor model, so the agent identity is
// "unknown" and no model is priced — named, never guessed.
func sessionEffortsOf(m observatoryModel) []console.SessionEffort {
	if m.ledgerErr != nil || m.ledger == nil {
		return nil
	}
	out := make([]console.SessionEffort, 0, len(m.ledger.order))
	for _, k := range m.ledger.order {
		e := m.ledger.latest[k]
		se := console.SessionEffort{SessionID: e.SessionID}
		if e.TokensIn != nil {
			se.TokensIn = *e.TokensIn
		}
		if e.TokensOut != nil {
			se.TokensOut = *e.TokensOut
		}
		if e.TurnCount != nil {
			se.Turns = *e.TurnCount
		}
		if e.DurationMS != nil {
			se.WallSeconds = int(*e.DurationMS / 1000)
		}
		out = append(out, se)
	}
	return out
}

// workCommitFacts reads the files of the newest workMaxCommits proving
// commits in one read-only `git show`. Rework is not read here (it needs
// a log per commit): it stays absent, named.
func workCommitFacts(ctx context.Context, root string, join console.JoinResult) (map[string]console.CommitFacts, string) {
	type at struct{ commit, at string }
	seen := map[string]string{}
	for _, o := range join.Outcomes {
		for _, r := range o.Rows {
			if r.Commit != "" && gitro.IsObjectName(r.Commit) && r.At > seen[r.Commit] {
				seen[r.Commit] = r.At
			}
		}
	}
	if len(seen) == 0 {
		return nil, "no proving commit to read"
	}
	list := make([]at, 0, len(seen))
	for c, a := range seen {
		list = append(list, at{c, a})
	}
	sort.Slice(list, func(i, j int) bool {
		if list[i].at != list[j].at {
			return list[i].at > list[j].at
		}
		return list[i].commit < list[j].commit
	})
	note := fmt.Sprintf("files of %s read with one git show", plural(len(list), "proving commit"))
	if len(list) > workMaxCommits {
		note = fmt.Sprintf("files of the newest %d of %d proving commits read with one git show", workMaxCommits, len(list))
		list = list[:workMaxCommits]
	}
	args := []string{"show", "--no-renames", "--name-only", "--format=@%H"}
	for _, c := range list {
		args = append(args, c.commit)
	}
	out, err := gitro.Output(ctx, root, args...)
	if err != nil {
		return nil, "the proving commits' files could not be read: " + err.Error()
	}
	facts := map[string]console.CommitFacts{}
	var cur string
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		switch {
		case line == "":
		case strings.HasPrefix(line, "@"):
			cur = line[1:]
			facts[cur] = console.CommitFacts{Commit: cur}
		case cur != "":
			cf := facts[cur]
			cf.Files = append(cf.Files, line)
			facts[cur] = cf
		}
	}
	// The join may name an abbreviated commit; key both spellings.
	for _, c := range list {
		if _, ok := facts[c.commit]; ok {
			continue
		}
		for full, cf := range facts {
			if strings.HasPrefix(full, c.commit) {
				cf.Commit = c.commit
				facts[c.commit] = cf
			}
		}
	}
	return facts, note
}

// workZonesConfig reads the repository's own `[zones]` table.
func workZonesConfig(root string) map[string][]string {
	if root == "" {
		return nil
	}
	loaded, err := config.Load("", filepath.Join(root, ".fathom", "config.toml"))
	if err != nil {
		return nil
	}
	return loaded.Config.Zones
}

// workRuleEvents reads the rule-outcome log's newest workRuleTailBytes and
// keeps this repository's blocked, violated and served events, file paths
// made repository-relative.
func workRuleEvents(root string) ([]console.RuleEvent, string) {
	path := ruleOutcomeLogPath()
	if path == "" {
		return nil, "rule outcomes: no home directory to find the log in"
	}
	data, err := readFileTail(path, workRuleTailBytes)
	if os.IsNotExist(err) {
		return nil, "rule outcomes: no log at " + path + " — no hook has recorded one"
	} else if err != nil {
		return nil, "rule outcomes: " + path + " could not be read: " + err.Error()
	}
	var out []console.RuleEvent
	for _, line := range strings.Split(string(data), "\n") {
		var e ruleOutcomeEvent
		if json.Unmarshal([]byte(line), &e) != nil || e.RuleID == "" {
			continue
		}
		file := e.File
		if filepath.IsAbs(file) {
			rel, err := filepath.Rel(root, file)
			if err != nil || strings.HasPrefix(rel, "..") {
				continue
			}
			file = filepath.ToSlash(rel)
		}
		out = append(out, console.RuleEvent{Rule: e.RuleID, Event: e.Event, Session: e.Session, File: file, At: e.TS})
	}
	return out, fmt.Sprintf("rule outcomes: %s from %s (the newest %d MiB)", plural(len(out), "event"), path, workRuleTailBytes>>20)
}

// readFileTail reads at most the last n bytes of path, from the first
// whole line.
func readFileTail(path string, n int64) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	fi, err := f.Stat()
	if err != nil {
		return nil, err
	}
	off := int64(0)
	if fi.Size() > n {
		off = fi.Size() - n
	}
	if _, err := f.Seek(off, io.SeekStart); err != nil {
		return nil, err
	}
	data, err := io.ReadAll(io.LimitReader(f, n))
	if err != nil {
		return nil, err
	}
	if off > 0 {
		if i := strings.IndexByte(string(data), '\n'); i >= 0 {
			data = data[i+1:]
		}
	}
	return data, nil
}

// workHotspots projects T1's rank with the stored line ranges of its
// symbols and a distinct-author count per file (bounded to the rows
// shown).
func workHotspots(ctx context.Context, deps *consoleDeps, sr serviceRoot, risk certify.RiskReport, now time.Time) console.HotspotsModel {
	if len(risk.Rows) > workHotspotRows {
		risk.Rows = risk.Rows[:workHotspotRows]
	}
	spans := map[string]console.SymbolSpan{}
	authors := map[string]int{}
	for _, r := range risk.Rows {
		if deps != nil && deps.c != nil {
			if obs, err := deps.c.Store().ObservationsForSymbol(ctx, sr.Service, r.Symbol); err == nil {
				for _, o := range obs {
					ref, _, _ := strings.Cut(o.SourceRef, ":")
					if ref == r.File && o.Line > 0 && o.EndLine >= o.Line {
						spans[r.Symbol] = console.SymbolSpan{StartLine: o.Line, EndLine: o.EndLine}
						break
					}
				}
			}
		}
		if _, ok := authors[r.File]; !ok {
			if n, ok := console.DistinctAuthorCount(ctx, sr.Root, r.File, now); ok {
				authors[r.File] = n
			}
		}
	}
	return console.Hotspots(console.HotspotInputs{Risk: risk, Spans: spans, Authors: authors})
}

// ─── rendering ──────────────────────────────────────────────────────────────

func renderWorkPage(m workPageModel) string {
	var b strings.Builder
	b.WriteString(renderRepoScopeNav("/work", m.Repositories, m.Service))
	if m.Service == "" {
		b.WriteString(chrome.RenderEmptyState(chrome.EmptyState{
			Heading: "Choose a repository",
			Body:    "Work intelligence joins this repository's proof history to the sessions that produced it; pick one above.",
		}))
		return b.String()
	}
	b.WriteString(renderWorkIntelligence(m.Work, m.History, m.Unjoined, m.Commits))
	b.WriteString(`<div class="mt-4">` + renderPanel(consolePanel{ID: "friction", Title: "Friction map", SourceKey: "sessions",
		Question: "Where do agents struggle — blocked edits, rediscovery, search loops, reverts, unproven claims — and what might help?",
		Source:   strings.Join(m.FrictionRead, "; "), Denominator: fmt.Sprintf("%s, %s", plural(m.Friction.Map.RuleEvents, "rule event"), plural(m.Friction.Map.Events, "activity event")),
		Body: renderFrictionMap(m.Friction)}) + `</div>`)
	hsBody, hsState, hsReason := "", console.State(""), ""
	if m.Hotspots != nil {
		hsBody = renderHotspots(*m.Hotspots)
	} else {
		hsState, hsReason = console.StateError, m.HotspotsErr
		hsBody = `<p class="text-ink-muted">` + html.EscapeString(m.HotspotsErr) + `</p>`
	}
	b.WriteString(`<div class="mt-4">` + renderPanel(consolePanel{ID: "hotspots", Title: "Hotspots", SourceKey: "index",
		Question: "Which symbols are big, churning, central and uncovered — and which files have a single author?",
		Verb:     `test_impact rank:"risk"`,
		Source:   "T1's factors stored at ingest; the stored L1 range; git log over 365 days (author counts only)",
		State:    hsState, Reason: hsReason, Body: hsBody}) + `</div>`)
	return b.String()
}

// renderWorkIntelligence renders U5, U2 and U1 as three panels.
func renderWorkIntelligence(w console.WorkModel, hist console.SourceRead, unjoined int, commits string) string {
	var b strings.Builder
	src := "the verify ledger's proof history joined to the session ledger (" + hist.Path + ")"
	state, reason := hist.State, hist.Reason
	if state == console.StatePresent && w.Changes == 0 {
		state, reason = console.StateAbsent, fmt.Sprintf("no proven change joined to a session yet (%s unjoined, named on /progress)", plural(unjoined, "proven row"))
	}
	panel := func(p consolePanel) { b.WriteString(`<div class="mt-4">` + renderPanel(p) + `</div>`) }

	sc := w.Scorecard
	scBody := renderScorecard(sc)
	panel(consolePanel{ID: "scorecard", Title: "Model and harness scorecard", SourceKey: "plans sessions",
		Question: "Per agent identity (harness, model, effort): proven rows, honesty, cost per proven row, rework — descriptive unless G2 admits a comparison.",
		Source:   src, Denominator: plural(len(sc.Cells), "agent identity") + ", " + plural(w.Changes, "proven change"),
		State: state, Reason: reason, Body: scBody})

	panel(consolePanel{ID: "interest", Title: "Interest rate by zone", SourceKey: "plans sessions",
		Question: "Which zones cost more per proven change than the typical change here?",
		Source:   src + "; " + commits, Denominator: plural(len(w.Interest.Zones), "zone"),
		State: state, Reason: reason, Body: renderInterest(w.Interest)})

	panel(consolePanel{ID: "cost-per-zone", Title: "Cost per zone", SourceKey: "plans sessions",
		Question: "Where do tokens, cost and CI minutes go by zone, and how much of it followed a claimed-unproven retry?",
		Source:   src + "; " + commits, Denominator: plural(len(w.CostPerZone), "zone"),
		State: state, Reason: reason, Body: renderCostPerZone(w.CostPerZone, w.FirstAttempt)})
	return b.String()
}

func workValue(v console.Value[float64], format string) string {
	switch {
	case v.Known() && v.State == console.StatePartial:
		return "≥ " + fmt.Sprintf(format, v.V)
	case v.Known():
		return fmt.Sprintf(format, v.V)
	}
	return "not measured — " + v.Reason
}

func workMuted(s string) templ.Component {
	return templ.Raw(`<span class="text-ink-muted">` + html.EscapeString(s) + `</span>`)
}

func renderScorecard(sc console.ScorecardModel) string {
	var b strings.Builder
	if sc.Mode == console.ScorecardCausal {
		b.WriteString(`<p class="mb-2 text-sm text-ink" data-scorecard-causal>` + html.EscapeString(sc.Causal) + `</p>`)
	} else {
		b.WriteString(`<p class="mb-2 text-sm text-ink-muted" data-scorecard-descriptive>` + html.EscapeString("Descriptive only. "+strings.TrimPrefix(sc.Withheld, "descriptive: ")) + `</p>`)
	}
	if len(sc.Cells) == 0 {
		b.WriteString(`<p class="text-ink-muted" data-scorecard-empty>No proven change joined to a measured session.</p>`)
		return b.String()
	}
	rows := make([]table.Row, 0, len(sc.Cells))
	for i, c := range sc.Cells {
		var mix []string
		for _, z := range c.ZoneMix {
			mix = append(mix, fmt.Sprintf("%s %.0f%%", z.Zone, z.Share*100))
		}
		cells := []string{c.Harness, c.Model, c.Effort, fmt.Sprint(c.N), fmt.Sprint(c.ProvenRows), workValue(c.Honesty, "%.2f"),
			workValue(c.TokensPerRow, "%.0f"), workValue(c.USDPerRow, "$%.2f"), workValue(c.Rework, "%.1f"), strings.Join(mix, ", "), c.From[:min(10, len(c.From))] + " – " + c.To[:min(10, len(c.To))]}
		r := table.Row{ID: fmt.Sprintf("scorecard-%d", i), Cells: cells}
		if c.Greyed {
			r.HTMLCells = make([]templ.Component, len(cells))
			for j, s := range cells {
				r.HTMLCells[j] = workMuted(s)
			}
		}
		rows = append(rows, r)
	}
	b.WriteString(renderKitTable(table.TableParams{TableID: "work-scorecard", Caption: fmt.Sprintf("Cells with n below %d are greyed", console.ScorecardMinN),
		Columns: []table.Column{{Key: "harness", Label: "Harness"}, {Key: "model", Label: "Model"}, {Key: "effort", Label: "Effort"},
			{Key: "n", Label: "n", Align: "right"}, {Key: "rows", Label: "Proven rows", Align: "right"}, {Key: "honesty", Label: "Claimed-unproven rate", Align: "right"},
			{Key: "tokens", Label: "Tokens / row", Align: "right"}, {Key: "usd", Label: "Cost / row", Align: "right"}, {Key: "rework", Label: "Rework lines", Align: "right"},
			{Key: "zones", Label: "Zone mix"}, {Key: "period", Label: "Period"}},
		Rows: rows}))
	if len(sc.Confounders) > 0 {
		b.WriteString(`<p class="mt-2 text-xs text-ink-muted" data-scorecard-confounders>` + html.EscapeString("Confounders: "+strings.Join(sc.Confounders, "; ")+".") + `</p>`)
	}
	return b.String()
}

func renderInterest(im console.InterestModel) string {
	if len(im.Zones) == 0 {
		return `<p class="text-ink-muted" data-interest-empty>` + html.EscapeString("No zone to rate: "+im.FleetMedian.Reason) + `</p>`
	}
	rows := make([]table.Row, 0, len(im.Zones))
	for _, z := range im.Zones {
		var mix []string
		for _, a := range z.ModelMix {
			mix = append(mix, fmt.Sprintf("%s ×%d", a.Agent, a.N))
		}
		rows = append(rows, table.Row{ID: "interest-" + z.Zone, Cells: []string{z.Zone, workValue(z.Rate, "%.2f×"), fmt.Sprint(z.N),
			fmt.Sprintf("%.0f", z.Median), fmt.Sprintf("%.0f – %.0f", z.Q1, z.Q3), strings.Join(mix, ", ")}})
	}
	return renderKitTable(table.TableParams{TableID: "work-interest", Caption: fmt.Sprintf("Measure: %s; fleet median %s", im.Measure, workValue(im.FleetMedian, "%.0f")),
		Columns: []table.Column{{Key: "zone", Label: "Zone"}, {Key: "rate", Label: "Interest rate", Align: "right"}, {Key: "n", Label: "n", Align: "right"},
			{Key: "median", Label: "Median", Align: "right"}, {Key: "iqr", Label: "IQR", Align: "right"}, {Key: "mix", Label: "Agent identities"}},
		Rows: rows})
}

func renderCostPerZone(zc []console.ZoneCost, fa console.FirstAttemptModel) string {
	if len(zc) == 0 {
		return `<p class="text-ink-muted" data-cost-empty>No proven change with known files in view.</p>`
	}
	split := map[string]console.AttemptSplit{}
	for _, s := range fa.Zones {
		split[s.Zone] = s
	}
	rows := make([]table.Row, 0, len(zc))
	for _, z := range zc {
		s := split[z.Zone]
		rows = append(rows, table.Row{ID: "cost-" + z.Zone, Cells: []string{z.Zone, fmt.Sprint(z.N), fmt.Sprintf("%.1f", z.Changes), fmt.Sprintf("%.0f", z.Tokens),
			workValue(z.USD, "$%.2f"), workValue(z.CIMinutes, "%.1f"), fmt.Sprintf("%d / %d", s.First.N, s.Retried.N)}})
	}
	return renderKitTable(table.TableParams{TableID: "work-cost", Caption: fmt.Sprintf("Fleet: %d first-attempt proofs, %d after claimed-unproven retries", fa.Fleet.First.N, fa.Fleet.Retried.N),
		Columns: []table.Column{{Key: "zone", Label: "Zone"}, {Key: "n", Label: "Changes", Align: "right"}, {Key: "share", Label: "Share-weighted", Align: "right"},
			{Key: "tokens", Label: "Tokens", Align: "right"}, {Key: "usd", Label: "Cost", Align: "right"}, {Key: "ci", Label: "CI minutes", Align: "right"},
			{Key: "split", Label: "First attempt / after retries", Align: "right"}},
		Rows: rows})
}

// renderFrictionMap renders U3: the file hotspots with their signal
// counts, then each remedy as a proposal with its evidence (WI-X4: there
// is no apply control).
func renderFrictionMap(v console.FrictionView) string {
	var b strings.Builder
	if len(v.Map.Files) == 0 && len(v.Map.Symbols) == 0 {
		b.WriteString(`<p class="text-ink-muted" data-friction-empty>No friction signal recorded for this repository.</p>`)
		return b.String()
	}
	var list []console.FrictionHotspot
	list = append(append(append(list, v.Map.Zones...), v.Map.Files...), v.Map.Symbols...)
	rows := make([]table.Row, 0, len(list))
	for i, h := range list {
		c := h.Counts
		rows = append(rows, table.Row{ID: fmt.Sprintf("friction-%d", i), Cells: []string{h.Kind, h.Key, fmt.Sprint(h.Total), fmt.Sprint(c.Blocked),
			fmt.Sprint(c.Rediscovery), fmt.Sprint(c.SearchLoops), fmt.Sprint(c.Reverts), fmt.Sprint(c.ClaimedUnproven)}})
	}
	b.WriteString(`<div data-friction-map>` + renderKitTable(table.TableParams{TableID: "work-friction", Caption: "Signals per zone, file and symbol, highest first",
		Columns: []table.Column{{Key: "kind", Label: "Kind"}, {Key: "key", Label: "Where"}, {Key: "total", Label: "Total", Align: "right"},
			{Key: "blocked", Label: "Blocked edits", Align: "right"}, {Key: "rediscovery", Label: "Rediscovery", Align: "right"},
			{Key: "loops", Label: "Search loops", Align: "right"}, {Key: "reverts", Label: "Reverts", Align: "right"}, {Key: "unproven", Label: "Claimed-unproven", Align: "right"}},
		Rows: rows}) + `</div>`)
	if len(v.Remedies) == 0 {
		b.WriteString(`<p class="mt-2 text-sm text-ink-muted" data-remedies-empty>No pattern in the remedy table matches these hotspots.</p>`)
		return b.String()
	}
	rrows := make([]table.Row, 0, len(v.Remedies))
	for i, r := range v.Remedies {
		rrows = append(rrows, table.Row{ID: fmt.Sprintf("remedy-%d", i), Cells: []string{r.Target, r.Pattern, r.Proposal, strings.Join(r.Evidence, " · "), r.Status}})
	}
	b.WriteString(`<div class="mt-3" data-remedies>` + renderKitTable(table.TableParams{TableID: "work-remedies", Caption: "Proposed remedies with their evidence — proposals only, nothing is applied",
		Columns: []table.Column{{Key: "target", Label: "Where"}, {Key: "pattern", Label: "Pattern"}, {Key: "proposal", Label: "Proposal"}, {Key: "evidence", Label: "Evidence"}, {Key: "status", Label: "Status"}},
		Rows:    rrows}) + `</div>`)
	return b.String()
}

// renderHotspots renders U13: T1's factors with the size proxy named and
// the author count as a number, a single author flagged.
func renderHotspots(m console.HotspotsModel) string {
	if len(m.Rows) == 0 {
		return `<p class="text-ink-muted" data-hotspots-empty>No ranked symbol in this repository (centrality is stored at ingest).</p>`
	}
	rows := make([]table.Row, 0, len(m.Rows))
	for i, h := range m.Rows {
		size := "not measured — " + h.Size.Reason
		if h.Size.Known() {
			size = fmt.Sprint(h.Size.V)
		}
		authors := "not read"
		if h.Authors.Known() {
			authors = fmt.Sprint(h.Authors.V)
			if h.BusFactorRisk {
				authors += " — bus-factor risk"
			}
		}
		rows = append(rows, table.Row{ID: fmt.Sprintf("hotspot-%d", i), Cells: []string{h.Symbol, h.File, fmt.Sprintf("%.3f", h.Risk), size,
			fmt.Sprintf("%.2f", h.Churn.Value), fmt.Sprintf("%.2f", h.Centrality.Value), fmt.Sprintf("%.2f", h.Uncovered.Value), authors}})
	}
	return `<div data-hotspots>` + renderKitTable(table.TableParams{TableID: "work-hotspots",
		Caption: fmt.Sprintf("Size is the %s; authors are counted over 365 days, never named; top %d of %s", m.SizeProxy, len(m.Rows), plural(m.Considered, "symbol")),
		Columns: []table.Column{{Key: "symbol", Label: "Symbol"}, {Key: "file", Label: "File"}, {Key: "risk", Label: "Risk", Align: "right"},
			{Key: "size", Label: "Size (lines)", Align: "right"}, {Key: "churn", Label: "Churn", Align: "right"}, {Key: "centrality", Label: "Centrality", Align: "right"},
			{Key: "uncovered", Label: "Uncovered", Align: "right"}, {Key: "authors", Label: "Distinct authors", Align: "right"}},
		Rows: rows}) + `</div>`
}
