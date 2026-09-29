package console

// support.go — the Support page's projections (SM-21..SM-24, design
// docs/design/support-matrix.md §5): the heatmap, the dialects view, the
// burn-up and what's next, each computed from already-loaded inputs — the
// `fathom support --json` matrix, the grammar lock, the readings ledger and
// the plan rows' checkboxes. No file is read and no clock is consulted here
// (internal/CLAUDE.md's determinism contract); cmd/fathom's console_support
// .go loads the inputs and renders the result.

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/geoffmilleraz/fathom/internal/support"
)

// SupportMark is how one computed state is drawn: a shape and a label that
// each tell every state apart on their own, and a role colour as the third
// channel — never colour alone (design §5 panel 1).
type SupportMark struct {
	Shape string `json:"shape"`
	Label string `json:"label"`
	Role  string `json:"role"`
}

// SupportStateMark is the fixed mark for a computed state; "" is a column
// the entry has no cell in (drawn unlike any state, never as the lowest).
func SupportStateMark(s support.State) SupportMark {
	switch s {
	case support.StateProvenExcellent:
		return SupportMark{Shape: "●", Label: "excellent", Role: "positive"}
	case support.StateProvenGood:
		return SupportMark{Shape: "◐", Label: "good", Role: "positive"}
	case support.StateBelowTarget:
		return SupportMark{Shape: "▼", Label: "below target", Role: "caution"}
	case support.StateRegressed:
		return SupportMark{Shape: "✕", Label: "regressed", Role: "danger"}
	case support.StateTolerated:
		return SupportMark{Shape: "○", Label: "tolerated", Role: "ink-muted"}
	case support.StateDeclared:
		return SupportMark{Shape: "◌", Label: "declared", Role: "accent"}
	case support.StateRefused:
		return SupportMark{Shape: "⊘", Label: "refused", Role: "ink-muted"}
	case support.StateDeferred:
		return SupportMark{Shape: "◷", Label: "deferred", Role: "ink-muted"}
	case support.StateNA:
		return SupportMark{Shape: "–", Label: "n/a", Role: "ink-muted"}
	}
	return SupportMark{Shape: "·", Label: "no cell", Role: "ink-muted"}
}

// SupportStates is the closed state vocabulary in legend order.
var SupportStates = []support.State{
	support.StateProvenExcellent, support.StateProvenGood, support.StateBelowTarget,
	support.StateRegressed, support.StateTolerated, support.StateDeclared,
	support.StateRefused, support.StateDeferred, support.StateNA,
}

// SupportHeatCell is one drawn cell and the evidence its hover or click
// opens: the reading, the plan row and that row's verdict.
type SupportHeatCell struct {
	Capability string          `json:"capability"`
	State      support.State   `json:"state,omitempty"`
	Mark       SupportMark     `json:"mark"`
	Target     string          `json:"target,omitempty"`
	LevelMet   support.Level   `json:"level_met,omitempty"`
	Reading    string          `json:"reading,omitempty"`
	PlanRow    string          `json:"plan_row,omitempty"`
	Verdict    support.Verdict `json:"verdict,omitempty"`
	Title      string          `json:"title"`
}

// SupportHeatRow is one entry across its kind's columns.
type SupportHeatRow struct {
	Kind   support.Kind      `json:"kind"`
	ID     string            `json:"id"`
	Name   string            `json:"name"`
	Wave   int               `json:"wave,omitempty"`
	Engine string            `json:"engine,omitempty"`
	Cells  []SupportHeatCell `json:"cells"`
}

// SupportHeatGroup is one kind's block: its columns (levels for languages
// and tool formats, the capability list otherwise) and rows.
type SupportHeatGroup struct {
	Kind    support.Kind          `json:"kind"`
	Columns []string              `json:"columns"`
	Rows    []SupportHeatRow      `json:"rows"`
	Counts  map[support.State]int `json:"counts"`
}

// SupportHeatmap lays the matrix out per kind, in the matrix's own kind
// order (Project sorts by kind).
func SupportHeatmap(m support.Matrix) []SupportHeatGroup {
	var groups []SupportHeatGroup
	idx := map[support.Kind]int{}
	for _, e := range m.Entries {
		i, ok := idx[e.Kind]
		if !ok {
			i = len(groups)
			idx[e.Kind] = i
			groups = append(groups, SupportHeatGroup{Kind: e.Kind, Counts: map[support.State]int{}})
		}
		g := &groups[i]
		switch e.Kind {
		case support.KindLanguage:
			g.Columns = []string{"L1", "L2", "L3", "L4"}
		case support.KindTool:
			g.Columns = []string{"L1", "L2", "L3"}
		default:
			for _, c := range e.Cells {
				if !containsString(g.Columns, c.Capability) {
					g.Columns = append(g.Columns, c.Capability)
				}
			}
		}
	}
	for _, e := range m.Entries {
		g := &groups[idx[e.Kind]]
		byCap := make(map[string]support.Cell, len(e.Cells))
		for _, c := range e.Cells {
			byCap[c.Capability] = c
		}
		row := SupportHeatRow{Kind: e.Kind, ID: e.ID, Name: e.Name, Wave: e.Wave, Engine: e.Engine}
		for _, col := range g.Columns {
			c, ok := byCap[col]
			if !ok {
				row.Cells = append(row.Cells, SupportHeatCell{Capability: col, Mark: SupportStateMark(""), Title: e.Name + " · " + col + ": no cell — not registered, targeted or read at this column"})
				continue
			}
			g.Counts[c.State]++
			row.Cells = append(row.Cells, heatCell(e, c))
		}
		g.Rows = append(g.Rows, row)
	}
	return groups
}

func heatCell(e support.Entry, c support.Cell) SupportHeatCell {
	hc := SupportHeatCell{Capability: c.Capability, State: c.State, Mark: SupportStateMark(c.State), LevelMet: c.LevelMet, Reading: c.Reading, PlanRow: c.PlanRow, Verdict: c.Verdict}
	switch t := c.Target.(type) {
	case bool:
		if t {
			hc.Target = "proven"
		}
	case support.Level:
		hc.Target = string(t)
	case string:
		hc.Target = t
	}
	parts := []string{e.Name + " · " + c.Capability + ": " + hc.Mark.Label}
	if hc.Target != "" {
		parts = append(parts, "target "+hc.Target)
	}
	if c.Reading != "" {
		r := "reading " + c.Reading
		if c.LevelMet != "" {
			r += " (level met " + string(c.LevelMet) + ")"
		}
		parts = append(parts, r)
	} else {
		parts = append(parts, "no reading")
	}
	if c.PlanRow != "" {
		v := string(c.Verdict)
		if v == "" {
			v = "no verdict"
		}
		parts = append(parts, "plan row "+c.PlanRow+" ("+v+")")
	}
	hc.Title = strings.Join(parts, " — ")
	return hc
}

func containsString(xs []string, s string) bool {
	for _, x := range xs {
		if x == s {
			return true
		}
	}
	return false
}

// SupportEmbedCapBytes is the cap on the grammar blobs embedded in the
// binary (language-breadth §8; tsengine's lazyload test enforces it). The
// dialects view draws the running total against it: nearing it is the
// trigger for dialect packs (LB-F14/LB-F27), deferred until then.
const SupportEmbedCapBytes = 40 << 20

// supportNearCapShare is the share of the cap at which the packs trigger
// is raised.
const supportNearCapShare = 0.9

// SupportDialectInput is one dialect data directory as cmd/fathom read it:
// its manifest, its grammar's lock row (when the engine embeds one) and its
// reading.
type SupportDialectInput struct {
	ID, Name, Engine string
	// Blob is the grammar blob the manifest names; "" when the engine
	// embeds no grammar.
	Blob        string
	GrammarRepo string
	GrammarTag  string
	Licence     string
	// BlobBytes is the embedded (gzip) size from grammars.lock.json;
	// meaningful only when InLock.
	BlobBytes   int64
	InLock      bool
	LevelMet    string
	ReadingDate string
}

// SupportDialectRow is one drawn dialect row.
type SupportDialectRow struct {
	ID           string     `json:"id"`
	Name         string     `json:"name"`
	Engine       string     `json:"engine"`
	Blob         string     `json:"blob,omitempty"`
	Pin          string     `json:"pin,omitempty"`
	Licence      string     `json:"licence,omitempty"`
	LevelMet     string     `json:"level_met,omitempty"`
	ReadingDate  string     `json:"reading_date,omitempty"`
	Embedded     Value[int] `json:"embedded_bytes"`
	SharedBlob   bool       `json:"shared_blob,omitempty"`
	RunningTotal int64      `json:"running_total_bytes"`
}

// SupportDialectsView is the dialects panel.
type SupportDialectsView struct {
	Rows     []SupportDialectRow `json:"rows"`
	Total    int64               `json:"total_bytes"`
	Cap      int64               `json:"cap_bytes"`
	Share    float64             `json:"share"`
	Headroom int64               `json:"headroom_bytes"`
	NearCap  bool                `json:"near_cap"`
}

// SupportDialects sorts the dialects by id and keeps a running total of the
// embedded blob bytes, each blob counted once however many dialects share
// it. A dialect that embeds no grammar, or whose blob has no lock row, has
// no size: absent with its reason, never 0 bytes.
func SupportDialects(in []SupportDialectInput, capBytes int64) SupportDialectsView {
	rows := append([]SupportDialectInput(nil), in...)
	sort.Slice(rows, func(i, j int) bool { return rows[i].ID < rows[j].ID })
	users := map[string]int{}
	for _, d := range rows {
		if d.Blob != "" {
			users[d.Blob]++
		}
	}
	v := SupportDialectsView{Cap: capBytes}
	counted := map[string]bool{}
	for _, d := range rows {
		r := SupportDialectRow{ID: d.ID, Name: d.Name, Engine: d.Engine, Blob: d.Blob, Licence: d.Licence, LevelMet: d.LevelMet, ReadingDate: d.ReadingDate}
		if d.GrammarRepo != "" {
			r.Pin = d.GrammarRepo
			if d.GrammarTag != "" {
				r.Pin += "@" + d.GrammarTag
			}
		}
		switch {
		case d.Blob == "":
			r.Embedded = Absent[int]("the " + orWord(d.Engine, "unknown") + " engine embeds no grammar blob")
		case !d.InLock:
			r.Embedded = Absent[int]("blob " + d.Blob + " has no row in grammars.lock.json")
		default:
			r.Embedded = Measured(int(d.BlobBytes), "grammars.lock.json gz_bytes")
			r.SharedBlob = users[d.Blob] > 1
			if !counted[d.Blob] {
				counted[d.Blob] = true
				v.Total += d.BlobBytes
			}
		}
		r.RunningTotal = v.Total
		v.Rows = append(v.Rows, r)
	}
	if capBytes > 0 {
		v.Share = float64(v.Total) / float64(capBytes)
		v.Headroom = capBytes - v.Total
		v.NearCap = v.Share >= supportNearCapShare
	}
	return v
}

func orWord(s, fallback string) string {
	if s == "" {
		return fallback
	}
	return s
}

// SupportRequirement is one targeted capability an entry must meet to be at
// target: a level, or (Level "") a reading at any grade.
type SupportRequirement struct {
	Kind       support.Kind
	ID         string
	Capability string
	Level      support.Level
}

// SupportRequirements lists every targeted cell of the matrix.
func SupportRequirements(m support.Matrix) []SupportRequirement {
	var out []SupportRequirement
	for _, e := range m.Entries {
		for _, c := range e.Cells {
			if c.Target == nil {
				continue
			}
			r := SupportRequirement{Kind: e.Kind, ID: e.ID, Capability: c.Capability}
			switch t := c.Target.(type) {
			case support.Level:
				r.Level = t
			case string:
				r.Level = support.Level(t)
			}
			out = append(out, r)
		}
	}
	return out
}

// SupportMilestone is a line drawn on a kind's burn-up: Count entries at
// target by Label (the queue's waves, for languages and tool formats).
type SupportMilestone struct {
	Kind  support.Kind `json:"kind"`
	Label string       `json:"label"`
	Count int          `json:"count"`
}

// SupportBurnUpInput is everything the burn-up is computed from.
type SupportBurnUpInput struct {
	Ledger       []support.LedgerRow
	Requirements []SupportRequirement
	Milestones   []SupportMilestone
}

// SupportBurnDay is one calendar day of a kind's burn-up.
type SupportBurnDay struct {
	Date     string     `json:"date"`
	AtTarget Value[int] `json:"at_target"`
}

// SupportBurnSeries is one kind's burn-up over the ledger's window.
type SupportBurnSeries struct {
	Kind     support.Kind     `json:"kind"`
	Entries  int              `json:"entries"`
	Days     []SupportBurnDay `json:"days"`
	Measured int              `json:"measured_days"`
	Absent   int              `json:"absent_days"`
}

// SupportBurnUpView is the progress-over-time panel.
type SupportBurnUpView struct {
	From       string              `json:"from,omitempty"`
	To         string              `json:"to,omitempty"`
	Series     []SupportBurnSeries `json:"series"`
	Milestones []SupportMilestone  `json:"milestones"`
	Note       string              `json:"note,omitempty"`
}

// SupportBurnUp counts, per kind and calendar day between the ledger's
// first and last reading, the entries whose every requirement the latest
// reading so far meets. A day with no reading of that kind is absent — the
// ledger said nothing that day — never 0.
func SupportBurnUp(in SupportBurnUpInput) SupportBurnUpView {
	v := SupportBurnUpView{Milestones: in.Milestones}
	if v.Milestones == nil {
		v.Milestones = []SupportMilestone{}
	}
	ledger := append([]support.LedgerRow(nil), in.Ledger...)
	sort.SliceStable(ledger, func(i, j int) bool { return ledger[i].ReadingDate < ledger[j].ReadingDate })
	if len(ledger) == 0 {
		v.Series = []SupportBurnSeries{}
		v.Note = "the readings ledger (docs/bench/readings/support-ledger.jsonl) has no rows yet: no reading has been written since it started, so there is no history to draw"
		return v
	}
	from, err1 := time.Parse("2006-01-02", ledger[0].ReadingDate)
	to, err2 := time.Parse("2006-01-02", ledger[len(ledger)-1].ReadingDate)
	if err1 != nil || err2 != nil {
		v.Series = []SupportBurnSeries{}
		v.Note = "the readings ledger carries a reading_date that is not YYYY-MM-DD"
		return v
	}
	v.From, v.To = from.Format("2006-01-02"), to.Format("2006-01-02")

	reqByEntry := map[support.Kind]map[string][]SupportRequirement{}
	for _, r := range in.Requirements {
		if reqByEntry[r.Kind] == nil {
			reqByEntry[r.Kind] = map[string][]SupportRequirement{}
		}
		reqByEntry[r.Kind][r.ID] = append(reqByEntry[r.Kind][r.ID], r)
	}
	kindsRead := map[support.Kind]map[string]bool{} // kind → dates it has a reading on
	for _, r := range ledger {
		if kindsRead[r.Kind] == nil {
			kindsRead[r.Kind] = map[string]bool{}
		}
		kindsRead[r.Kind][r.ReadingDate] = true
	}
	var kinds []support.Kind
	for k := range kindsRead {
		kinds = append(kinds, k)
	}
	sort.Slice(kinds, func(i, j int) bool { return kinds[i] < kinds[j] })

	type key struct {
		kind    support.Kind
		id, cap string
	}
	for _, k := range kinds {
		s := SupportBurnSeries{Kind: k, Entries: len(reqByEntry[k])}
		latest := map[key]support.LedgerRow{}
		li := 0
		for d := from; !d.After(to); d = d.AddDate(0, 0, 1) {
			date := d.Format("2006-01-02")
			for ; li < len(ledger) && ledger[li].ReadingDate <= date; li++ {
				r := ledger[li]
				latest[key{r.Kind, r.ID, r.Capability}] = r
			}
			if !kindsRead[k][date] {
				s.Days = append(s.Days, SupportBurnDay{Date: date, AtTarget: Absent[int]("no " + string(k) + " reading on " + date)})
				s.Absent++
				continue
			}
			at := 0
			for id, reqs := range reqByEntry[k] {
				met := true
				for _, rq := range reqs {
					r, ok := latest[key{k, id, rq.Capability}]
					if !ok || r.Grade == support.GradeNone || (rq.Level != "" && levelOrder(r.LevelMet) < levelOrder(rq.Level)) {
						met = false
						break
					}
				}
				if met {
					at++
				}
			}
			s.Days = append(s.Days, SupportBurnDay{Date: date, AtTarget: Measured(at, "support-ledger.jsonl").WithDenominator(fmt.Sprintf("%d %s entries with a target", s.Entries, k))})
			s.Measured++
		}
		v.Series = append(v.Series, s)
	}
	return v
}

func levelOrder(l support.Level) int {
	switch l {
	case support.L1:
		return 1
	case support.L2:
		return 2
	case support.L3:
		return 3
	case support.L4:
		return 4
	}
	return 0
}

// SupportRowState is a plan row's checkbox as the plan file states it.
type SupportRowState string

const (
	SupportRowOpen          SupportRowState = "open"
	SupportRowClaimed       SupportRowState = "claimed"
	SupportRowDispositioned SupportRowState = "dispositioned" // [-] deferred (or no disposition)
	SupportRowRefused       SupportRowState = "refused"       // [-] refused: will not be built (G2-6)
)

// SupportNextItem is one target row that could be next: its entry, wave,
// owning plan row and that row's needs. Refused is a queue item the plan
// refuses (design §14.2) while its own row stays open — its -rest sibling is
// [-] refused, as langbench.RefusedItems reads it: it needs re-planning.
type SupportNextItem struct {
	Kind    support.Kind
	ID      string
	Name    string
	Wave    int
	PlanRow string
	Needs   []string
	Refused bool
}

// SupportNextRow is one eligible item.
type SupportNextRow struct {
	Kind    support.Kind `json:"kind"`
	ID      string       `json:"id"`
	Name    string       `json:"name"`
	PlanRow string       `json:"plan_row"`
	Needs   int          `json:"needs"`
}

// SupportNextWave is one wave's eligible items, the items a refused need
// leaves needing re-planning, and how many more wait.
type SupportNextWave struct {
	Wave    int              `json:"wave"`
	Items   []SupportNextRow `json:"items"`
	Replan  []SupportNextRow `json:"replan"`
	Blocked int              `json:"blocked"`
}

// SupportNextView is the what's-next panel.
type SupportNextView struct {
	Waves    []SupportNextWave `json:"waves"`
	Eligible int               `json:"eligible"`
	Blocked  int               `json:"blocked"`
	Replan   int               `json:"replan"`
	Done     int               `json:"done"`
}

// bareRowID strips a "plan.md#" prefix from a cross-plan row reference.
func bareRowID(ref string) string {
	if i := strings.LastIndex(ref, "#"); i >= 0 {
		return ref[i+1:]
	}
	return ref
}

// SupportNext is the language queue's dependency rule
// (docs/design/language-breadth.md §13.0 step 2) over the target rows, which
// since G2-6 is `plans next`'s rule too: an item whose own plan row is claimed
// or [-] is done; one whose every need is claimed is eligible; one with a
// refused need needs re-planning (the need will not be built, so waiting is
// wrong), as does an item refused itself (Refused); anything else — an open or
// deferred need, or a need or row the plans do not carry — is blocked. Grouped
// by wave, ascending; items by kind then id. D6's reading-at-L1 satisfaction of
// a queue-item need is lang-next's (langbench.QueueEligibility): this panel
// reads checkboxes only.
func SupportNext(items []SupportNextItem, rows map[string]SupportRowState) SupportNextView {
	sorted := append([]SupportNextItem(nil), items...)
	sort.Slice(sorted, func(i, j int) bool {
		a, b := sorted[i], sorted[j]
		if a.Wave != b.Wave {
			return a.Wave < b.Wave
		}
		if a.Kind != b.Kind {
			return a.Kind < b.Kind
		}
		return a.ID < b.ID
	})
	v := SupportNextView{Waves: []SupportNextWave{}}
	wave := func(w int) *SupportNextWave {
		if n := len(v.Waves); n > 0 && v.Waves[n-1].Wave == w {
			return &v.Waves[n-1]
		}
		v.Waves = append(v.Waves, SupportNextWave{Wave: w, Items: []SupportNextRow{}, Replan: []SupportNextRow{}})
		return &v.Waves[len(v.Waves)-1]
	}
	for _, it := range sorted {
		own, known := rows[bareRowID(it.PlanRow)]
		if known && own != SupportRowOpen {
			v.Done++
			continue
		}
		w := wave(it.Wave)
		row := SupportNextRow{Kind: it.Kind, ID: it.ID, Name: it.Name, PlanRow: it.PlanRow, Needs: len(it.Needs)}
		eligible, replan := known, it.Refused
		for _, n := range it.Needs {
			switch rows[bareRowID(n)] {
			case SupportRowClaimed:
			case SupportRowRefused:
				replan = true
			default:
				eligible = false
			}
		}
		switch {
		case replan:
			w.Replan = append(w.Replan, row)
			v.Replan++
		case !eligible:
			w.Blocked++
			v.Blocked++
		default:
			w.Items = append(w.Items, row)
			v.Eligible++
		}
	}
	return v
}
