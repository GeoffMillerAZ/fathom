package console

// scorecard.go — WI-9, WI-10 (docs/design/work-intelligence.md §3 U5):
// the model and harness scorecard per agent identity (harness, model,
// effort setting). Descriptive only: every cell shows n and the
// confounders that differ between cells, cells below ScorecardMinN are
// greyed, and a causal sentence appears only when G2's admissibility test
// passes for the compared cells (law 12, admissibleComparison). Rows are
// agent identities, never people (I58).

import (
	"fmt"
	"sort"
	"strings"
)

// ScorecardMinN is the n below which a cell is greyed.
const ScorecardMinN = 10

// Scorecard modes.
const (
	ScorecardDescriptive = "descriptive"
	ScorecardCausal      = "causal"
)

// ZoneMix is one zone's share of a cell's changes.
type ZoneMix struct {
	Zone  string  `json:"zone"`
	Share float64 `json:"share"`
}

// ScorecardCell is one agent identity's figures.
type ScorecardCell struct {
	Agent        string         `json:"agent"`
	Harness      string         `json:"harness"`
	Model        string         `json:"model"`
	Effort       string         `json:"effort,omitempty"`
	N            int            `json:"n"`
	ProvenRows   int            `json:"proven_rows"`
	Retries      int            `json:"claimed_unproven"`
	Honesty      Value[float64] `json:"claimed_unproven_rate"`
	TokensPerRow Value[float64] `json:"tokens_per_proven_row"`
	USDPerRow    Value[float64] `json:"usd_per_proven_row"`
	Rework       Value[float64] `json:"rework_lines_per_change"`
	Greyed       bool           `json:"greyed"`
	ZoneMix      []ZoneMix      `json:"zone_mix"`
	MeanWeight   float64        `json:"mean_row_weight"`
	From         string         `json:"from"`
	To           string         `json:"to"`
}

// ScorecardModel is U5.
type ScorecardModel struct {
	Cells []ScorecardCell `json:"cells"`
	// Confounders names what differs between cells (zone mix, row weight
	// mix, time period): the reasons a difference is not an effect.
	Confounders []string `json:"confounders"`
	Mode        string   `json:"mode"`
	Causal      string   `json:"causal,omitempty"`
	Withheld    string   `json:"withheld,omitempty"`
	Unmeasured  int      `json:"unmeasured_changes"`
}

// ModelScorecard folds measured proven changes into one cell per agent
// identity, largest n first. Its mode is descriptive; scorecardCausalGate
// decides whether a causal sentence may be added.
func ModelScorecard(changes []ProvenChange) ScorecardModel {
	type acc struct {
		cell                   ScorecardCell
		tokens, usd, rework    float64
		priced, reworked, wsum int
		zones                  map[string]float64
	}
	cells := map[string]*acc{}
	m := ScorecardModel{Mode: ScorecardDescriptive, Cells: []ScorecardCell{}, Confounders: []string{}}
	for _, c := range changes {
		if !c.Known {
			m.Unmeasured++
			continue
		}
		a := cells[c.Agent]
		if a == nil {
			a = &acc{cell: ScorecardCell{Agent: c.Agent, Harness: c.Harness, Model: c.Model, Effort: c.Effort, From: c.At, To: c.At}, zones: map[string]float64{}}
			cells[c.Agent] = a
		}
		a.cell.N++
		a.cell.ProvenRows += len(c.Rows)
		a.cell.Retries += c.Retries
		a.tokens += c.Tokens
		a.wsum += c.Weight
		if c.USD.Known() {
			a.usd += c.USD.V
			a.priced++
		}
		if c.Rework.Known() {
			a.rework += float64(c.Rework.V)
			a.reworked++
		}
		for _, z := range c.Zones {
			a.zones[z.Zone] += z.Share
		}
		if c.At < a.cell.From {
			a.cell.From = c.At
		}
		if c.At > a.cell.To {
			a.cell.To = c.At
		}
	}
	for _, a := range cells {
		c := a.cell
		c.Greyed = c.N < ScorecardMinN
		rows := float64(c.ProvenRows)
		src := "the transcript fold joined to proof"
		if claimed := c.ProvenRows + c.Retries; claimed > 0 {
			c.Honesty = Measured(round2(float64(c.Retries)/float64(claimed)), "the proof history").
				WithDenominator(fmt.Sprintf("%d claims (%d proven rows + %d claimed-unproven)", claimed, c.ProvenRows, c.Retries))
		} else {
			c.Honesty = Absent[float64]("no claimed row")
		}
		c.TokensPerRow = Measured(round2(a.tokens/rows), src).WithDenominator(fmt.Sprintf("%d proven rows", c.ProvenRows))
		switch {
		case a.priced == 0:
			c.USDPerRow = Absent[float64](fmt.Sprintf("model %q is not in the price table", c.Model))
		case a.priced < c.N:
			c.USDPerRow = Partial(round2(a.usd/rows), fmt.Sprintf("%d of %d changes unpriced", c.N-a.priced, c.N))
		default:
			c.USDPerRow = Measured(round2(a.usd/rows), "the price table over the transcript fold")
		}
		if a.reworked == 0 {
			c.Rework = Absent[float64]("rework within 7 days not read for these commits")
		} else {
			c.Rework = Measured(round2(a.rework/float64(a.reworked)), "git log over the 7 days after each commit").
				WithDenominator(fmt.Sprintf("%d changes", a.reworked))
		}
		c.MeanWeight = round2(float64(a.wsum) / float64(c.N))
		for z, share := range a.zones {
			c.ZoneMix = append(c.ZoneMix, ZoneMix{Zone: z, Share: round2(share / float64(c.N))})
		}
		sort.Slice(c.ZoneMix, func(i, j int) bool {
			if c.ZoneMix[i].Share != c.ZoneMix[j].Share {
				return c.ZoneMix[i].Share > c.ZoneMix[j].Share
			}
			return c.ZoneMix[i].Zone < c.ZoneMix[j].Zone
		})
		m.Cells = append(m.Cells, c)
	}
	sort.Slice(m.Cells, func(i, j int) bool {
		if m.Cells[i].N != m.Cells[j].N {
			return m.Cells[i].N > m.Cells[j].N
		}
		return m.Cells[i].Agent < m.Cells[j].Agent
	})
	m.Confounders = scorecardConfounders(m.Cells)
	return m
}

// scorecardConfounders names each confounder that differs between cells.
func scorecardConfounders(cells []ScorecardCell) []string {
	out := []string{}
	if len(cells) < 2 {
		return out
	}
	mix := func(c ScorecardCell) string {
		var parts []string
		for _, z := range c.ZoneMix {
			parts = append(parts, fmt.Sprintf("%s=%.2f", z.Zone, z.Share))
		}
		return strings.Join(parts, ",")
	}
	zones, weight, period := false, false, false
	for _, c := range cells[1:] {
		zones = zones || mix(c) != mix(cells[0])
		weight = weight || c.MeanWeight != cells[0].MeanWeight
		period = period || c.From[:min(10, len(c.From))] != cells[0].From[:min(10, len(cells[0].From))] || c.To[:min(10, len(c.To))] != cells[0].To[:min(10, len(cells[0].To))]
	}
	if zones {
		out = append(out, "zone mix differs between cells")
	}
	if weight {
		out = append(out, "row weight mix differs between cells")
	}
	if period {
		out = append(out, "time period differs between cells")
	}
	return out
}

// scorecardCausalGate applies G2's admissibility test to the two largest
// cells: admissible, the page may say which is cheaper per proven row;
// otherwise it says the comparison is descriptive and why.
func scorecardCausalGate(m ScorecardModel) ScorecardModel {
	m.Mode, m.Causal, m.Withheld = ScorecardDescriptive, "", ""
	if len(m.Cells) < 2 {
		m.Withheld = "descriptive: fewer than two agent identities to compare"
		return m
	}
	a, b := m.Cells[0], m.Cells[1]
	adm := admissibleComparison(a.N, b.N)
	if !adm.Admissible {
		m.Withheld = "descriptive: " + adm.Withheld
		return m
	}
	metric, av, bv := "USD", a.USDPerRow, b.USDPerRow
	if !av.Known() || !bv.Known() {
		metric, av, bv = "tokens", a.TokensPerRow, b.TokensPerRow
	}
	if av.V == bv.V {
		m.Withheld = fmt.Sprintf("descriptive: %s and %s cost the same %s per proven row", a.Agent, b.Agent, metric)
		return m
	}
	cheap, dear, cv, dv := a, b, av.V, bv.V
	if bv.V < av.V {
		cheap, dear, cv, dv = b, a, bv.V, av.V
	}
	m.Mode = ScorecardCausal
	m.Causal = fmt.Sprintf("%s is cheaper per proven row than %s for this work (%s %.2f vs %.2f; n = %d and %d, G2 admissible)",
		cheap.Agent, dear.Agent, metric, cv, dv, cheap.N, dear.N)
	return m
}
