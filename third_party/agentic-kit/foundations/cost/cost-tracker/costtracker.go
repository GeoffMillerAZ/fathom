package costtracker

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"unicode/utf8"

	"github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/llmclient"
)

// mTok is the pricing denominator: ModelPrice rates are USD per million tokens.
const mTok = 1e6

// EstimateTokens estimates the token count of s with the chars/4 heuristic:
// the number of Unicode characters (runes) divided by four, rounded up, with
// the empty string estimating zero.
//
// Accuracy: for typical English prose the estimate is within roughly ±30% of
// a real tokenizer's count. Code, dense punctuation, and non-Latin scripts
// can fall outside that band (CJK text in particular tokenizes closer to one
// token per character). Use this for budgeting and rough pre-flight checks,
// never for billing — bill from llmclient.Usage, which is provider ground
// truth. Callers with a better estimator should inject it as an Estimator
// (see EstimatorOrDefault).
func EstimateTokens(s string) int {
	if s == "" {
		return 0
	}
	return (utf8.RuneCountInString(s) + 3) / 4
}

// Estimator is an injectable token-count estimator. EstimateTokens is the
// default implementation; callers with access to a real tokenizer supply
// their own.
type Estimator func(s string) int

// EstimatorOrDefault returns e when non-nil and EstimateTokens otherwise.
// Components that accept an optional Estimator call this once at construction
// so a nil estimator never has to be checked on the hot path.
func EstimatorOrDefault(e Estimator) Estimator {
	if e == nil {
		return EstimateTokens
	}
	return e
}

// Cost prices one call's Usage against the table. It returns (0, false) when
// model has no entry — the caller decides whether that is an error or a
// zero-cost record (foundations/llmclient/instrument.WithCost records tokens
// with zero cost).
//
// Each token class is priced independently at its own rate:
//
//	USD = InputTokens·InputPerMTok/1e6 + OutputTokens·OutputPerMTok/1e6
//	    + CacheReadInputTokens·CacheReadPerMTok/1e6
//	    + CacheCreationInputTokens·CacheWritePerMTok/1e6
//
// Assumption (Anthropic semantics): Usage.InputTokens EXCLUDES cache reads
// and cache writes — the provider reports regular input, cache-read input,
// and cache-creation input as disjoint counts. Cache-read tokens are
// therefore priced once at CacheReadPerMTok and never additionally as plain
// input. A provider that reports cache reads inside InputTokens would be
// double-billed by this formula; adapt the Usage at the adapter layer, not
// here.
func (p PriceTable) Cost(model string, u llmclient.Usage) (float64, bool) {
	mp, ok := p[model]
	if !ok {
		return 0, false
	}
	cost := float64(u.InputTokens)*mp.InputPerMTok/mTok +
		float64(u.OutputTokens)*mp.OutputPerMTok/mTok +
		float64(u.CacheReadInputTokens)*mp.CacheReadPerMTok/mTok +
		float64(u.CacheCreationInputTokens)*mp.CacheWritePerMTok/mTok
	return cost, true
}

// add accumulates one call into the totals.
func (t *Totals) add(u llmclient.Usage, usd float64) {
	t.Calls++
	t.InputTokens += int64(u.InputTokens)
	t.OutputTokens += int64(u.OutputTokens)
	t.CacheReadTokens += int64(u.CacheReadInputTokens)
	t.CacheWriteTokens += int64(u.CacheCreationInputTokens)
	t.USD += usd
}

// String renders the totals in the fixed, golden-testable form used by
// Tracker.Report:
//
//	calls=N input=N output=N cache_read=N cache_write=N usd=N.NNNNNN
func (t Totals) String() string {
	return fmt.Sprintf("calls=%d input=%d output=%d cache_read=%d cache_write=%d usd=%.6f",
		t.Calls, t.InputTokens, t.OutputTokens, t.CacheReadTokens, t.CacheWriteTokens, t.USD)
}

// Tracker aggregates recorded LLM calls: an overall total plus per-label and
// per-model breakdowns. Labels are caller-chosen logical buckets (a workflow
// step, an agent name); models are whatever the caller passed to Record.
//
// Tracker is safe for concurrent use.
type Tracker struct {
	mu      sync.Mutex
	total   Totals
	byLabel map[string]Totals
	byModel map[string]Totals
}

// New returns an empty Tracker ready for concurrent Record calls.
func New() *Tracker {
	return &Tracker{
		byLabel: make(map[string]Totals),
		byModel: make(map[string]Totals),
	}
}

// Record adds one call's usage and cost under the given label and model.
// costUSD is the caller-computed cost (typically PriceTable.Cost); pass 0
// when the model is unpriced — the tokens are still counted. Empty label or
// model strings are valid keys and aggregate under "".
func (t *Tracker) Record(label, model string, u llmclient.Usage, costUSD float64) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.total.add(u, costUSD)

	lt := t.byLabel[label]
	lt.add(u, costUSD)
	t.byLabel[label] = lt

	mt := t.byModel[model]
	mt.add(u, costUSD)
	t.byModel[model] = mt
}

// Totals returns the overall aggregate across every recorded call.
func (t *Tracker) Totals() Totals {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.total
}

// ByLabel returns a copy of the per-label aggregates. The returned map is
// independent of the Tracker's internal state. Go map iteration is
// randomized; for deterministic output use Report, which sorts keys.
func (t *Tracker) ByLabel() map[string]Totals {
	t.mu.Lock()
	defer t.mu.Unlock()
	return copyTotals(t.byLabel)
}

// ByModel returns a copy of the per-model aggregates. Same determinism note
// as ByLabel.
func (t *Tracker) ByModel() map[string]Totals {
	t.mu.Lock()
	defer t.mu.Unlock()
	return copyTotals(t.byModel)
}

// copyTotals shallow-copies a totals map (Totals is a value type, so a
// shallow copy is a deep copy).
func copyTotals(m map[string]Totals) map[string]Totals {
	out := make(map[string]Totals, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}

// Report renders a deterministic, golden-testable plain-text summary: the
// per-label section, the per-model section (each sorted by key, keys quoted
// with %q so empty keys stay visible), then the overall total.
//
//	cost report
//	by label:
//	  "alpha": calls=2 input=150 output=30 cache_read=5 cache_write=3 usd=0.750000
//	by model:
//	  "example-model": calls=2 input=150 output=30 cache_read=5 cache_write=3 usd=0.750000
//	total: calls=2 input=150 output=30 cache_read=5 cache_write=3 usd=0.750000
func (t *Tracker) Report() string {
	t.mu.Lock()
	defer t.mu.Unlock()

	var b strings.Builder
	b.WriteString("cost report\n")
	writeSection(&b, "by label:", t.byLabel)
	writeSection(&b, "by model:", t.byModel)
	fmt.Fprintf(&b, "total: %s\n", t.total)
	return b.String()
}

// writeSection appends one sorted "by X:" section to the report.
func writeSection(b *strings.Builder, header string, m map[string]Totals) {
	b.WriteString(header)
	b.WriteByte('\n')
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		fmt.Fprintf(b, "  %q: %s\n", k, m[k])
	}
}
