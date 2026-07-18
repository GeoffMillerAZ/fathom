// Package costtracker provides config-driven LLM cost accounting: a chars/4
// token-estimation heuristic (EstimateTokens), a PriceTable mapping model
// names to per-million-token USD prices, and a Tracker that aggregates token
// usage and spend by label and by model.
//
// NO PRICES ARE SHIPPED. Provider prices change without notice, and a stale
// embedded price table silently produces wrong costs — which is worse than no
// costs at all. Every PriceTable is config data supplied by the caller and
// validated against schema.cue; the numbers in testdata/valid.cue are
// illustrative examples only, never defaults.
//
// Composition: imports the standard library plus the sibling foundation
// foundations/llmclient (for llmclient.Usage) only. CUE owns the shape of the
// price table (schema.cue, mirrored by ModelPrice/PriceTable here); Go owns
// behavior. The telemetry/cost middleware that feeds a Tracker lives in
// foundations/llmclient/instrument; this package knows nothing about it.
//
// Concurrency contract: Tracker is safe for concurrent use (guarded by a
// sync.Mutex). PriceTable is a plain map — safe for concurrent reads, not for
// concurrent mutation; treat it as immutable after construction.
// EstimateTokens, EstimatorOrDefault, and PriceTable.Cost are pure functions.
package costtracker

// ModelPrice holds the USD price per million tokens for one model, split by
// token class. A zero field means "free / not billed" — explicit zeros are
// expected for models without prompt caching.
//
// CUE mirror: #ModelPrice in schema.cue.
type ModelPrice struct {
	// InputPerMTok is USD per million regular (uncached) input tokens.
	InputPerMTok float64

	// OutputPerMTok is USD per million output tokens.
	OutputPerMTok float64

	// CacheReadPerMTok is USD per million cache-read input tokens
	// (llmclient.Usage.CacheReadInputTokens).
	CacheReadPerMTok float64

	// CacheWritePerMTok is USD per million cache-creation input tokens
	// (llmclient.Usage.CacheCreationInputTokens).
	CacheWritePerMTok float64
}

// PriceTable maps a model identifier to its prices. It is config data: load
// it from CUE/JSON validated against #PriceTable in schema.cue. This package
// ships NO prices (see the package doc for why).
//
// CUE mirror: #PriceTable in schema.cue.
type PriceTable map[string]ModelPrice

// Totals is an aggregate of recorded LLM calls: call count, token counts per
// class, and accumulated USD spend.
type Totals struct {
	// Calls is the number of recorded calls.
	Calls int64

	// InputTokens accumulates llmclient.Usage.InputTokens (regular input,
	// excluding cache reads and cache writes — Anthropic semantics).
	InputTokens int64

	// OutputTokens accumulates llmclient.Usage.OutputTokens.
	OutputTokens int64

	// CacheReadTokens accumulates llmclient.Usage.CacheReadInputTokens.
	CacheReadTokens int64

	// CacheWriteTokens accumulates llmclient.Usage.CacheCreationInputTokens.
	CacheWriteTokens int64

	// USD is the accumulated cost in US dollars.
	USD float64
}
