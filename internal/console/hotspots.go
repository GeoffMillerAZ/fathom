package console

// hotspots.go — WI-12 (docs/design/work-intelligence.md §3 U13): a view
// over T1's stored risk factors (certify.RiskWeightedCoverage, TQ-4) plus
// size and bus factor. Size is the symbol's L1 line range, a named proxy
// for complexity (the cognitive analyzer stays retired); the bus factor
// is the number of distinct commit authors of the file in 365 days, shown
// as a count and never with names (I58) — a single-author file is flagged
// as a risk, not a person.

import (
	"context"
	"strings"
	"time"

	"github.com/geoffmilleraz/fathom/internal/certify"
	"github.com/geoffmilleraz/fathom/internal/gitro"
)

// HotspotSizeProxy names what the size column measures.
const HotspotSizeProxy = "L1 line range (a proxy for complexity; no complexity analyzer runs)"

// BusFactorWindow is the author-count horizon.
const BusFactorWindow = 365 * 24 * time.Hour

// SymbolSpan is a symbol's stored L1 line range.
type SymbolSpan struct {
	StartLine int
	EndLine   int
}

// HotspotInputs is what Hotspots projects: T1's report, the stored spans
// of its symbols, and distinct-author counts per file (a file missing
// from Authors was not read).
type HotspotInputs struct {
	Risk    certify.RiskReport
	Spans   map[string]SymbolSpan
	Authors map[string]int
}

// Hotspot is one symbol.
type Hotspot struct {
	Symbol        string             `json:"symbol"`
	File          string             `json:"file"`
	Risk          float64            `json:"risk"`
	Size          Value[int]         `json:"size_lines"`
	Churn         certify.RiskFactor `json:"churn"`
	Centrality    certify.RiskFactor `json:"centrality"`
	Uncovered     certify.RiskFactor `json:"uncovered_share"`
	Authors       Value[int]         `json:"distinct_authors_365d"`
	BusFactorRisk bool               `json:"bus_factor_risk"`
}

// HotspotsModel is U13.
type HotspotsModel struct {
	Rows             []Hotspot `json:"rows"`
	SizeProxy        string    `json:"size_proxy"`
	Considered       int       `json:"considered"`
	CoverageSupplied bool      `json:"coverage_supplied"`
	Watermark        string    `json:"watermark,omitempty"`
	Unread           []string  `json:"unread,omitempty"`
}

// Hotspots keeps T1's rank and adds size and the bus-factor count.
func Hotspots(in HotspotInputs) HotspotsModel {
	m := HotspotsModel{Rows: []Hotspot{}, SizeProxy: HotspotSizeProxy, Considered: in.Risk.Considered,
		CoverageSupplied: in.Risk.CoverageSupplied, Watermark: in.Risk.Watermark, Unread: in.Risk.Unread}
	for _, r := range in.Risk.Rows {
		h := Hotspot{Symbol: r.Symbol, File: r.File, Risk: r.Risk}
		for _, f := range r.Factors {
			switch f.Name {
			case "churn":
				h.Churn = f
			case "centrality":
				h.Centrality = f
			case "uncovered_share":
				h.Uncovered = f
			}
		}
		if sp, ok := in.Spans[r.Symbol]; ok && sp.EndLine >= sp.StartLine && sp.StartLine > 0 {
			h.Size = Present(sp.EndLine-sp.StartLine+1, "the stored L1 range")
		} else {
			h.Size = Absent[int]("no stored line range for this symbol")
		}
		if n, ok := in.Authors[r.File]; ok {
			h.Authors = Measured(n, "git log over 365 days").WithDenominator("distinct commit authors, counted, never named")
			h.BusFactorRisk = n == 1
		} else {
			h.Authors = Absent[int]("the file's commit authors were not read")
		}
		m.Rows = append(m.Rows, h)
	}
	return m
}

// DistinctAuthorCount counts file's distinct commit author emails over
// BusFactorWindow before now in root's working tree — one read-only
// `git log` (internal/gitro). The emails are counted in memory and
// discarded; ok is false when git cannot answer.
func DistinctAuthorCount(ctx context.Context, root, file string, now time.Time) (int, bool) {
	if root == "" || file == "" {
		return 0, false
	}
	since := now.Add(-BusFactorWindow).UTC().Format(time.RFC3339)
	out, err := gitro.Output(ctx, root, "log", "--since="+since, "--format=%ae", "--", file)
	if err != nil {
		return 0, false
	}
	seen := map[string]bool{}
	for _, line := range strings.Split(string(out), "\n") {
		if line = strings.ToLower(strings.TrimSpace(line)); line != "" {
			seen[line] = true
		}
	}
	return len(seen), true
}
