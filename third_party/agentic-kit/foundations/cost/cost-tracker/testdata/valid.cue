// Package costtracker — valid test data that must unify with schema.cue.
//
// EXAMPLE PRICES ONLY. Every number in this file is an illustrative
// placeholder chosen to exercise the schema — these are NOT real provider
// prices, are NOT shipped as defaults, and must never be copied into
// production config. Real deployments supply their own price table from
// current provider pricing.
package costtracker

import "github.com/geoffmilleraz/agentic-kit/foundations/cost/cost-tracker:costtracker"

// validExampleTable exercises a multi-model table with cache pricing.
validExampleTable: costtracker.#PriceTable & {
	// EXAMPLE: a small, cheap model with prompt caching.
	"example-model-small": {
		InputPerMTok:      0.80
		OutputPerMTok:     4.0
		CacheReadPerMTok:  0.08
		CacheWritePerMTok: 1.0
	}

	// EXAMPLE: a large model with prompt caching.
	"example-model-large": {
		InputPerMTok:      3.0
		OutputPerMTok:     15.0
		CacheReadPerMTok:  0.30
		CacheWritePerMTok: 3.75
	}
}

// validNoCaching exercises explicit zero rates: a model without prompt
// caching states its cache rates as 0 (free / not billed) rather than
// omitting them — all four fields are required.
validNoCaching: costtracker.#PriceTable & {
	"example-model-no-cache": {
		InputPerMTok:      1.0
		OutputPerMTok:     2.0
		CacheReadPerMTok:  0
		CacheWritePerMTok: 0
	}
}

// validEmpty is the smallest valid table: no models priced. Every cost
// lookup then returns (0, false) and callers record zero-cost usage.
validEmpty: costtracker.#PriceTable & {}
