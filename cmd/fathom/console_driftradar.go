package main

// console_driftradar.go — CX-22 (docs/plans/change-and-compliance.md; design
// docs/design/change-and-compliance.md §5 "Output"): the architecture drift
// radar on the workspace's Intent tab. Two lists, each from a recorded
// instrument, never a model's opinion:
//
//   - "decisions due for review": the revisit and promote conditions whose
//     computable trigger fired at this audit (CX-21) — evaluated because a
//     person opened the tab or ran `fathom intent audit`, never by the
//     daemon on its own (CX-X5);
//   - "architecture eroding here": the archconformance deny rules whose
//     ingest-time violation count (CX-20) rose across the kept readings,
//     with the commit ranges that added violations.
//
// No rule pack, no reading, or no computable trigger renders as an absence
// with its reason — never as 0 (the doctrine's law 6).

import (
	"fmt"
	"html"
	"sort"
	"strings"

	"github.com/a-h/templ"
	"github.com/geoffmilleraz/app-kit/appkit/ui/stat"
	"github.com/geoffmilleraz/app-kit/appkit/ui/table"
	"github.com/geoffmilleraz/app-kit/appkit/ui/viz"
	"github.com/geoffmilleraz/fathom/internal/console"
	"github.com/geoffmilleraz/fathom/internal/intent"
	"github.com/geoffmilleraz/fathom/internal/store"
)

// driftRadarWindow is how many of the newest readings the radar reads a
// rule's trend over.
const driftRadarWindow = 20

// driftRadarAdded is one reading that added violations to a rule.
type driftRadarAdded struct {
	From  string `json:"from,omitempty"` // the previous reading's commit
	To    string `json:"to"`             // this reading's commit
	Delta int    `json:"delta"`
}

// driftRadarRule is one deny rule's trend over the window.
type driftRadarRule struct {
	Rule   string            `json:"rule"`
	Latest int               `json:"latest"`
	Counts []int             `json:"counts"` // oldest first, one per reading
	Rising bool              `json:"rising"` // latest above the window's first
	Added  []driftRadarAdded `json:"added,omitempty"`
}

// driftRadarModel is the radar's JSON projection.
type driftRadarModel struct {
	Due       console.Value[int]      `json:"due"`
	Decisions []intent.TriggerVerdict `json:"decisions,omitempty"`
	// Unevaluated are computable triggers the audit could not read, with
	// the reason; Prose counts the human-judged ones.
	Unevaluated []intent.TriggerVerdict `json:"unevaluated,omitempty"`
	Prose       int                     `json:"prose"`
	Eroding     console.Value[int]      `json:"eroding"`
	Rules       []driftRadarRule        `json:"rules,omitempty"`
	Readings    int                     `json:"readings"`
}

// buildDriftRadar projects an audit's trigger verdicts and a service's
// violation readings (sorted by seq then rule) into the radar.
func buildDriftRadar(triggers []intent.TriggerVerdict, trend []store.ArchViolationReading, trendErr error) driftRadarModel {
	m := driftRadarModel{}
	computable := 0
	for _, t := range triggers {
		switch {
		case t.Instrument == "":
			m.Prose++
		case t.State == intent.TriggerFired:
			computable++
			m.Decisions = append(m.Decisions, t)
		case t.State == intent.TriggerUnevaluated:
			computable++
			m.Unevaluated = append(m.Unevaluated, t)
		default:
			computable++
		}
	}
	const trigSource = "fathom intent audit --read-only (computable revisit/promote triggers)"
	if computable == 0 {
		m.Due = console.Absent[int](fmt.Sprintf("no revisit or promote condition names an instrument and a threshold — %d prose trigger(s) are human-judged, never evaluated", m.Prose)).WithSource(trigSource)
	} else {
		m.Due = console.Measured(len(m.Decisions), trigSource).WithDenominator(fmt.Sprintf("of %d computable trigger(s), the ones that fired at this audit", computable))
	}

	const trendSource = "the store's arch_violation_trend (recorded at each ingest)"
	seqs := []int64{}
	for _, r := range trend {
		if len(seqs) == 0 || seqs[len(seqs)-1] != r.Seq {
			seqs = append(seqs, r.Seq)
		}
	}
	m.Readings = len(seqs)
	switch {
	case trendErr != nil:
		m.Eroding = console.Errored[int]("the violation trend could not be read: " + trendErr.Error())
		return m
	case len(seqs) == 0:
		m.Eroding = console.Absent[int]("no violation reading recorded — the repository has no .fathom/rules/archconformance.cue, or it has not been ingested since it gained one").WithSource(trendSource)
		return m
	}
	if len(seqs) > driftRadarWindow {
		seqs = seqs[len(seqs)-driftRadarWindow:]
	}
	first := seqs[0]
	index := map[string]int{}
	for _, r := range trend {
		if r.Seq < first {
			continue
		}
		i, ok := index[r.Rule]
		if !ok {
			i = len(m.Rules)
			index[r.Rule] = i
			m.Rules = append(m.Rules, driftRadarRule{Rule: r.Rule})
		}
		rule := &m.Rules[i]
		rule.Counts = append(rule.Counts, r.Count)
		rule.Latest = r.Count
		if r.Delta > 0 && r.Seq != first {
			rule.Added = append(rule.Added, driftRadarAdded{From: r.PrevCommit, To: r.Commit, Delta: r.Delta})
		}
	}
	eroding := 0
	for i := range m.Rules {
		r := &m.Rules[i]
		r.Rising = len(r.Counts) > 1 && r.Latest > r.Counts[0]
		if r.Rising {
			eroding++
		}
	}
	// Eroding rules first, then by rule.
	sortRadarRules(m.Rules)
	if len(seqs) < 2 {
		m.Eroding = console.Absent[int]("one reading so far — a trend needs a second ingest at a later commit").WithSource(trendSource)
		return m
	}
	m.Eroding = console.Measured(eroding, trendSource).WithDenominator(fmt.Sprintf("of %d deny rule(s) over the newest %d reading(s), the ones whose count rose", len(m.Rules), len(seqs)))
	return m
}

func sortRadarRules(rs []driftRadarRule) {
	sort.SliceStable(rs, func(i, j int) bool {
		if rs[i].Rising != rs[j].Rising {
			return rs[i].Rising
		}
		return rs[i].Rule < rs[j].Rule
	})
}

// renderDriftRadar renders the radar panel for svc.
func renderDriftRadar(svc string, m driftRadarModel) string {
	var b strings.Builder
	b.WriteString(stat.RenderBand([]stat.Stat{
		valueStat("decisions due for review", m.Due),
		valueStat("rules eroding", m.Eroding),
	}, stat.BandOptions{AriaLabel: svc + " drift radar"}))

	b.WriteString(`<h3 class="mt-4 text-sm font-semibold text-ink">Decisions due for review</h3>`)
	if len(m.Decisions) == 0 {
		msg := "No computable trigger fired at this audit."
		if !m.Due.Known() {
			msg = m.Due.Reason
		}
		b.WriteString(`<p class="text-ink-muted" data-radar-due>` + html.EscapeString(msg) + `</p>`)
	} else {
		rows := make([]table.Row, 0, len(m.Decisions))
		for _, d := range m.Decisions {
			rows = append(rows, table.Row{ID: "due-" + d.ID, Cells: []string{d.Record, d.Label, clipText(d.Text, 140), d.Instrument + " " + d.Threshold, d.Reading, fmt.Sprintf("%s:%d", d.File, d.Line)}})
		}
		b.WriteString(`<div data-radar-due>` + renderKitTable(table.TableParams{
			TableID: "radar-due", Caption: fmt.Sprintf("%d decision(s) whose trigger fired", len(rows)),
			Columns: []table.Column{{Key: "record", Label: "Record"}, {Key: "label", Label: "Condition"}, {Key: "text", Label: "When"}, {Key: "trigger", Label: "Trigger"}, {Key: "reading", Label: "Reading"}, {Key: "where", Label: "Where"}},
			Rows:    rows,
		}) + `</div>`)
	}
	for _, u := range m.Unevaluated {
		b.WriteString(`<p class="mt-2 text-xs text-caution">unevaluated — ` + html.EscapeString(u.Record+" "+u.Instrument+" "+u.Threshold+": "+u.Reason) + `</p>`)
	}
	if m.Prose > 0 && m.Due.Known() {
		b.WriteString(`<p class="mt-2 text-xs text-ink-muted">` + html.EscapeString(fmt.Sprintf("%d prose trigger(s) stay human-judged.", m.Prose)) + `</p>`)
	}

	b.WriteString(`<h3 class="mt-4 text-sm font-semibold text-ink">Architecture eroding here</h3>`)
	if len(m.Rules) == 0 {
		b.WriteString(`<p class="text-ink-muted" data-radar-eroding>` + html.EscapeString(m.Eroding.Reason) + `</p>`)
		return b.String()
	}
	if !m.Eroding.Known() {
		b.WriteString(`<p class="text-ink-muted">` + html.EscapeString(m.Eroding.Reason) + `</p>`)
	}
	rows := make([]table.Row, 0, len(m.Rules))
	for _, r := range m.Rules {
		state := "holding"
		if r.Rising {
			state = "eroding"
		}
		var added []string
		for _, a := range r.Added {
			from := shortCommit(a.From)
			if from == "" {
				from = "…"
			}
			added = append(added, fmt.Sprintf("+%d in %s..%s", a.Delta, from, shortCommit(a.To)))
		}
		trend := make([]string, len(r.Counts))
		vals := make([]float64, len(r.Counts))
		for i, n := range r.Counts {
			trend[i] = fmtThousands(n)
			vals[i] = float64(n)
		}
		trendCell := html.EscapeString(strings.Join(trend, " → "))
		if len(vals) > 1 {
			trendCell = viz.Sparkline(vals, viz.SparklineOptions{W: 120, H: 24, AriaLabel: r.Rule + " violations per reading, oldest first: " + strings.Join(trend, ", ")})
		}
		rows = append(rows, table.Row{
			ID:        "rule-" + r.Rule,
			Cells:     []string{r.Rule, fmtThousands(r.Latest), strings.Join(trend, " → "), state, strings.Join(added, "; ")},
			HTMLCells: []templ.Component{nil, nil, templ.Raw(trendCell), nil, nil},
		})
	}
	b.WriteString(`<div data-radar-eroding>` + renderKitTable(table.TableParams{
		TableID: "radar-rules", Caption: fmt.Sprintf("%d deny rule(s) over the newest %d reading(s); eroding first", len(m.Rules), min(m.Readings, driftRadarWindow)),
		Columns: []table.Column{{Key: "rule", Label: "Rule"}, {Key: "now", Label: "Violations", Align: "right"}, {Key: "trend", Label: "Trend"}, {Key: "state", Label: "State"}, {Key: "added", Label: "Added by"}},
		Rows:    rows,
	}) + `</div>`)
	return b.String()
}

// driftRadarPanel wraps the radar in its panel.
func driftRadarPanel(svc, verb, fresh string, m driftRadarModel) string {
	state, reason := console.StatePresent, ""
	switch {
	case m.Eroding.State == console.StateError:
		state, reason = console.StateError, m.Eroding.Reason
	case !m.Due.Known() && !m.Eroding.Known():
		state, reason = console.StateAbsent, "no computable trigger and no violation trend to read — both lists below say why"
	}
	return renderPanel(consolePanel{
		ID: "intent-radar", Title: "Drift radar",
		Question: "Which decisions are due for review, and where is the architecture eroding? Triggers are evaluated when this tab opens or `fathom intent audit` runs — never by the daemon on its own.",
		Verb:     verb, Source: "fathom intent audit --read-only (triggers) · the store's arch_violation_trend, recorded at each ingest",
		Denominator: m.Eroding.Denominator, Freshness: fresh,
		State: state, Reason: reason, SourceKey: "plans:" + svc + " index:" + svc,
		Body: renderDriftRadar(svc, m),
	})
}
