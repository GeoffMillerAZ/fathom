// Package costtracker defines the CUE-first contract for LLM price tables.
// Go types in types.go mirror these definitions; any drift between the two
// is a bug. Run `cue vet ./...` from this directory (or the repo root) to
// validate testdata/valid.cue against these constraints.
//
// NO PRICES ARE SHIPPED in this package — not in Go, not in CUE. Provider
// prices change without notice, and a stale embedded table silently produces
// wrong costs. A PriceTable is always caller-supplied config validated
// against #PriceTable; the values in testdata/valid.cue are illustrative
// EXAMPLES only, never defaults.
package costtracker

// #ModelPrice holds the USD price per million tokens for one model, split by
// token class. All four fields are required — config authors state every
// rate explicitly (zero is valid and means "free / not billed", e.g. for
// models without prompt caching). There are no defaults.
#ModelPrice: {
	// InputPerMTok is USD per million regular (uncached) input tokens.
	// Per Anthropic semantics, Usage.InputTokens excludes cache reads and
	// cache writes, so this rate never double-prices cached prefixes.
	InputPerMTok: number & >=0

	// OutputPerMTok is USD per million output tokens.
	OutputPerMTok: number & >=0

	// CacheReadPerMTok is USD per million cache-read input tokens.
	CacheReadPerMTok: number & >=0

	// CacheWritePerMTok is USD per million cache-creation input tokens.
	CacheWritePerMTok: number & >=0
}

// #PriceTable maps a model identifier to its prices. Model names are opaque
// provider-namespaced strings and must be non-empty. An empty table is valid
// (every model is then "unknown" and costs are not computed).
#PriceTable: {
	[!=""]: #ModelPrice
}
