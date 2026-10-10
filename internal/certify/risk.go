package certify

// risk.go — TQ-4 (docs/plans/test-intelligence.md; design doc
// docs/design/test-intelligence.md §3 T1, §4): risk-weighted coverage.
// Per non-test symbol,
//
//	risk = uncovered_share x centrality x (1 + churn) x severity x (1 + incidents)
//
// with every factor normalized to [0, 1] within the service and returned
// separately, each with its citation, so a rank is explainable rather than
// a bare score. Every input is a stored fact (centrality and churn are
// computed at ingest, TQ-1/TQ-2) or one named file (the guard file and
// the incidents doc, TQ-3): the read is a sort over one service's stored
// rows with a LIMIT, never a graph walk or a tree read (I48).
//
// fathom asserts the factors; it never judges adequacy. Without a
// coverage profile every symbol is treated as uncovered and the citation
// says so — never a fabricated coverage number.

import (
	"context"
	"fmt"
	"path"
	"sort"
	"strings"

	readmanifest "github.com/geoffmilleraz/agentic-kit/foundations/telemetry/read-manifest"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/store"
)

// Risk list bounds.
const (
	DefaultRiskLimit = 20
	MaxRiskLimit     = 500
)

// RiskFactor is one factor of a risk row: its normalized value and the
// citation it came from.
type RiskFactor struct {
	Name  string  `json:"name"`
	Value float64 `json:"value"`
	Cite  string  `json:"cite"`
}

// RiskRow is one ranked symbol.
type RiskRow struct {
	Symbol  string       `json:"symbol"`
	File    string       `json:"file"`
	Risk    float64      `json:"risk"`
	Factors []RiskFactor `json:"factors"`
}

// RiskOptions tunes RiskWeightedCoverage.
type RiskOptions struct {
	// Limit caps the rows returned (DefaultRiskLimit when <= 0, at most
	// MaxRiskLimit).
	Limit int
	// Coverage maps a symbol key to its covered statement share in [0, 1],
	// from a coverage profile the caller parsed. nil means no profile was
	// supplied.
	Coverage map[string]float64
}

// RiskReport is RiskWeightedCoverage's answer.
type RiskReport struct {
	Rows []RiskRow `json:"rows"`
	// Considered counts the non-test symbols ranked before the limit (a
	// file's module symbol, K18, is never one).
	Considered       int    `json:"considered"`
	CoverageSupplied bool   `json:"coverage_supplied"`
	Watermark        string `json:"watermark,omitempty"`
	// Unread names an input file that exists and could not be read (I38).
	Unread []string `json:"unread,omitempty"`
}

// isTestSymbol reports whether a symbol is test code, which is never a
// risk target: a Test* function or anything declared in a test file.
func isTestSymbol(symbol, file string) bool {
	name := symbol
	if i := strings.LastIndexAny(name, ".:"); i >= 0 {
		name = name[i+1:]
	}
	if strings.HasPrefix(name, "Test") {
		return true
	}
	return isTestFile(file)
}

// isTestFile recognizes the common test-file conventions across the
// languages fathom indexes.
func isTestFile(file string) bool {
	base := path.Base(file)
	switch {
	case strings.HasSuffix(base, "_test.go"),
		strings.HasPrefix(base, "test_") && strings.HasSuffix(base, ".py"),
		strings.HasSuffix(base, "_test.py"),
		strings.Contains(base, ".test."), strings.Contains(base, ".spec."),
		strings.HasSuffix(base, "Test.java"), strings.HasSuffix(base, "Tests.cs"):
		return true
	}
	return false
}

// RiskWeightedCoverage ranks service's non-test symbols by T1 risk.
// corpusRoot is the service's indexed path; "" degrades severity to claims
// alone and incidents to findings alone.
func RiskWeightedCoverage(ctx context.Context, st store.Store, service, corpusRoot string, opts RiskOptions) (RiskReport, error) {
	limit := opts.Limit
	if limit <= 0 {
		limit = DefaultRiskLimit
	}
	if limit > MaxRiskLimit {
		limit = MaxRiskLimit
	}
	watermark, cent, err := st.SymbolCentrality(ctx, service)
	if err != nil {
		return RiskReport{}, fmt.Errorf("RiskWeightedCoverage: centrality: %w", err)
	}
	churn, err := st.FileChurn(ctx, service)
	if err != nil {
		return RiskReport{}, fmt.Errorf("RiskWeightedCoverage: churn: %w", err)
	}
	claims, err := st.ClaimsForService(ctx, service, "")
	if err != nil {
		return RiskReport{}, fmt.Errorf("RiskWeightedCoverage: claims: %w", err)
	}
	reads := readmanifest.NewSet()
	guards := loadGuards(corpusRoot)
	incidents := loadIncidentRecords(corpusRoot, reads)

	churnBy := make(map[string]int, len(churn))
	maxChurn := 0
	for _, c := range churn {
		churnBy[c.Path] = c.Commits180d
		if c.Commits180d > maxChurn {
			maxChurn = c.Commits180d
		}
	}
	var targets []store.SymbolCentrality
	var files []string
	maxCallers := 0
	for _, r := range cent {
		if r.Callers > maxCallers {
			maxCallers = r.Callers
		}
		// A file's module symbol (K18) is every import's target, so its
		// stored callers are its importers': 13 of flask 3.1.3's top 20 risk
		// rows were `<module>` rows (wave h2, O741). Its row stays stored and
		// in the service max (the stored Centrality is normalized by it);
		// only the ranking reads past it.
		if isTestSymbol(r.Symbol, r.File) || core.IsModuleSymbolKey(r.Symbol) {
			continue
		}
		targets = append(targets, r)
		files = append(files, r.File)
	}
	gov := riskInputs(files, guards, claims, incidents)
	maxInc := 0
	for _, g := range gov {
		if g.Incidents > maxInc {
			maxInc = g.Incidents
		}
	}

	rows := make([]RiskRow, 0, len(targets))
	for _, r := range targets {
		unc, uncCite := 1.0, "no coverage supplied: treated as uncovered"
		if opts.Coverage != nil {
			if cov, ok := opts.Coverage[r.Symbol]; ok {
				unc, uncCite = clamp01(1-cov), fmt.Sprintf("coverage profile: %.0f%% of statements covered", clamp01(cov)*100)
			} else {
				uncCite = "not in the supplied coverage profile: treated as uncovered"
			}
		}
		ch := churnBy[r.File]
		chN := 0.0
		if maxChurn > 0 {
			chN = float64(ch) / float64(maxChurn)
		}
		g := gov[r.File]
		incN := 0.0
		if maxInc > 0 {
			incN = float64(g.Incidents) / float64(maxInc)
		}
		incCite := "no incident record or finding names this file"
		if len(g.IncidentCites) > 0 {
			incCite = strings.Join(g.IncidentCites, "; ")
		}
		factors := []RiskFactor{
			{Name: "uncovered_share", Value: unc, Cite: uncCite},
			{Name: "centrality", Value: r.Centrality, Cite: fmt.Sprintf("%d transitive callers (service max %d), stored at ingest %s", r.Callers, maxCallers, watermarkLabel(watermark))},
			{Name: "churn", Value: chN, Cite: fmt.Sprintf("%d commits touching %s in 180 days (service max %d)", ch, r.File, maxChurn)},
			{Name: "severity", Value: g.Severity, Cite: g.SeverityCite},
			{Name: "incidents", Value: incN, Cite: incCite},
		}
		risk := unc * r.Centrality * (1 + chN) * g.Severity * (1 + incN)
		rows = append(rows, RiskRow{Symbol: r.Symbol, File: r.File, Risk: risk, Factors: factors})
	}
	sort.SliceStable(rows, func(i, j int) bool {
		if rows[i].Risk != rows[j].Risk {
			return rows[i].Risk > rows[j].Risk
		}
		return rows[i].Symbol < rows[j].Symbol
	})
	rep := RiskReport{Considered: len(rows), CoverageSupplied: opts.Coverage != nil, Watermark: watermark}
	for _, e := range reads.Manifest().Unreadable() {
		rep.Unread = append(rep.Unread, e.Path+": "+e.Outcome.Detail)
	}
	if len(rows) > limit {
		rows = rows[:limit]
	}
	rep.Rows = rows
	return rep, nil
}

func watermarkLabel(w string) string {
	if w == "" {
		return "(no watermark)"
	}
	if len(w) > 12 {
		w = w[:12]
	}
	return "@" + w
}

func clamp01(v float64) float64 {
	switch {
	case v < 0:
		return 0
	case v > 1:
		return 1
	}
	return v
}
