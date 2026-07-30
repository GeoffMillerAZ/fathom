package metrics

// spend.go — Q-9 (docs/plans/expert-suite-q.md): `fathom metrics spend`'s
// pure aggregation core over `compile_runs` — runs, tokens, USD, cost per
// claim, budget headroom, and escape-hatch (--unsafe-skip-eval-gate /
// --force) usage flagged, grouped by layer, day, or model. Pure: no I/O —
// cmd/fathom's cmdMetricsSpend reads compile_runs via store.MountReadOnly
// and resolves each layer's budget via semantic.LoadLayerIn, then calls
// AggregateSpend with the results.

import (
	"sort"
	"time"

	"github.com/geoffmilleraz/fathom/internal/core"
)

// SpendGroupByLayer/Day/Model are AggregateSpend's valid --by values.
const (
	SpendGroupByLayer = "layer"
	SpendGroupByDay   = "day"
	SpendGroupByModel = "model"
)

// CompileRunBudgetLookup resolves a layer's max_usd_per_run budget ceiling.
// found is false when the layer's pack could not be resolved (no --root
// layer pack under that name) — AggregateSpend reports that honestly via
// SpendRow.BudgetNote rather than a fabricated zero ceiling.
type CompileRunBudgetLookup func(layer string) (maxUSD float64, found bool)

// SpendOptions configures AggregateSpend.
type SpendOptions struct {
	Since, Until time.Time
	// GroupBy is one of SpendGroupByLayer (default), SpendGroupByDay,
	// SpendGroupByModel.
	GroupBy string
}

// SpendRow is one group's aggregate spend.
type SpendRow struct {
	Group         string  `json:"group"`
	Runs          int     `json:"runs"`
	TokensIn      int     `json:"tokens_in"`
	TokensOut     int     `json:"tokens_out"`
	CostUSD       float64 `json:"cost_usd"`
	ClaimsEmitted int     `json:"claims_emitted"`

	// CostPerClaimUSD is CostUSD / ClaimsEmitted — the design's own
	// headline number ("17 claims / $0.256 = $0.015"). Nil with an
	// honest CostPerClaimNote when ClaimsEmitted == 0 (a run that spent
	// but emitted nothing — never a fabricated +Inf/NaN).
	CostPerClaimUSD  *float64 `json:"cost_per_claim_usd"`
	CostPerClaimNote string   `json:"cost_per_claim_note,omitempty"`

	// BudgetMaxUSD/BudgetHeadroomUSD are populated ONLY for GroupBy ==
	// SpendGroupByLayer — a per-run ceiling has no cross-layer meaning
	// under day/model grouping. BudgetNote always explains why, either
	// way (not applicable to this grouping, or the layer's pack could
	// not be resolved).
	BudgetMaxUSD      *float64 `json:"budget_max_usd"`
	BudgetHeadroomUSD *float64 `json:"budget_headroom_usd"`
	BudgetNote        string   `json:"budget_note,omitempty"`

	// EscapeHatchRuns counts runs whose Escape field is non-empty
	// (--force / --unsafe-skip-eval-gate) — the cheap-but-untrusted runs
	// the design explicitly asks to be surfaced, never buried in the
	// total.
	EscapeHatchRuns int `json:"escape_hatch_runs"`
}

// SpendReport is AggregateSpend's output.
type SpendReport struct {
	Since   string     `json:"since"`
	Until   string     `json:"until"`
	GroupBy string     `json:"group_by"`
	Rows    []SpendRow `json:"rows"`

	TotalRuns    int     `json:"total_runs"`
	TotalCostUSD float64 `json:"total_cost_usd"`
	Note         string  `json:"note"`
}

// AggregateSpend groups runs (already read from compile_runs, already
// scoped to service/module by the caller) by opts.GroupBy and computes
// each row's cost, cost-per-claim, budget headroom, and escape-hatch
// count.
func AggregateSpend(runs []core.CompileRun, lookup CompileRunBudgetLookup, opts SpendOptions) SpendReport {
	groupBy := opts.GroupBy
	if groupBy == "" {
		groupBy = SpendGroupByLayer
	}
	r := SpendReport{
		Since:   opts.Since.UTC().Format(time.RFC3339),
		Until:   opts.Until.UTC().Format(time.RFC3339),
		GroupBy: groupBy,
	}
	if len(runs) == 0 {
		r.Note = "no compile runs in window"
		return r
	}

	type acc struct {
		runs, tokensIn, tokensOut, claimsEmitted, escapeHatch int
		costUSD                                               float64
	}
	byGroup := map[string]*acc{}
	for _, run := range runs {
		if !run.RanAt.IsZero() && (run.RanAt.Before(opts.Since) || run.RanAt.After(opts.Until)) {
			continue
		}
		key := spendGroupKey(run, groupBy)
		a, ok := byGroup[key]
		if !ok {
			a = &acc{}
			byGroup[key] = a
		}
		a.runs++
		a.tokensIn += run.TokensIn
		a.tokensOut += run.TokensOut
		a.costUSD += run.CostUSD
		a.claimsEmitted += run.ClaimsEmitted
		if run.Escape != "" {
			a.escapeHatch++
		}
		r.TotalRuns++
		r.TotalCostUSD += run.CostUSD
	}

	keys := make([]string, 0, len(byGroup))
	for k := range byGroup {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		a := byGroup[k]
		row := SpendRow{
			Group: k, Runs: a.runs, TokensIn: a.tokensIn, TokensOut: a.tokensOut,
			CostUSD: a.costUSD, ClaimsEmitted: a.claimsEmitted, EscapeHatchRuns: a.escapeHatch,
		}
		// Zero-denominator guard (design §6): a group that spent but
		// emitted zero claims never divides by zero — an honest note,
		// never a fabricated +Inf/NaN.
		if a.claimsEmitted == 0 {
			row.CostPerClaimNote = "not measurable: zero claims emitted in this group"
		} else {
			perClaim := a.costUSD / float64(a.claimsEmitted)
			row.CostPerClaimUSD = &perClaim
		}
		if groupBy != SpendGroupByLayer {
			row.BudgetNote = "not applicable: budget is a per-layer ceiling; use --by layer"
		} else if lookup == nil {
			row.BudgetNote = "not measurable: no budget lookup configured"
		} else if maxUSD, found := lookup(k); !found {
			row.BudgetNote = "not measurable: layer pack not found under --root"
		} else {
			m := maxUSD
			row.BudgetMaxUSD = &m
			// Headroom is against the largest single run's cost in this
			// group — a per-run ceiling compared to the closest any run
			// in window came to it, not a sum (summing per-run costs
			// against a per-run ceiling would be a category error).
			maxRunCost := 0.0
			for _, run := range runs {
				if spendGroupKey(run, groupBy) != k {
					continue
				}
				if run.CostUSD > maxRunCost {
					maxRunCost = run.CostUSD
				}
			}
			headroom := maxUSD - maxRunCost
			row.BudgetHeadroomUSD = &headroom
		}
		r.Rows = append(r.Rows, row)
	}
	return r
}

// spendGroupKey resolves one run's group key for the given GroupBy value.
func spendGroupKey(run core.CompileRun, groupBy string) string {
	switch groupBy {
	case SpendGroupByDay:
		if run.RanAt.IsZero() {
			return "unknown"
		}
		return run.RanAt.UTC().Format("2006-01-02")
	case SpendGroupByModel:
		if run.Model == "" {
			return "(none)"
		}
		return run.Model
	default: // SpendGroupByLayer
		return run.Layer
	}
}
