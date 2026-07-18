// Package evalrunner — CUE mirrors of the non-scenario runtime fixtures
// (testdata/modelcatalog.json and testdata/pricetable.json), validated
// against the owning foundations' schemas so the JSON the runner loads at
// `task eval` time and the shape CUE owns cannot drift silently.
//
// EXAMPLE DATA ONLY. Every model id, family name, and price in this file is
// an illustrative placeholder: these are NOT real models, NOT real families,
// NOT real prices, and must never be copied into production config. The kit
// ships no authoritative catalog rows and no authoritative prices (see the
// model-catalog and cost-tracker package docs); real deployments supply their
// own caller data. The families here exist purely to exercise
// modelcatalog.ValidateIndependence in replay-mode CI:
//
//   - example-judge-1 (family "beta") vs example-producer-1 (family "alpha"):
//     the independence-passing pair `task eval` runs with.
//   - example-judge-same-family (family "alpha"): exists ONLY so the
//     same-family construction canary can prove New is construction-fatal on
//     a shared_family violation.
package evalrunner

import (
	"github.com/geoffmilleraz/agentic-kit/foundations/llmclient/model-catalog:modelcatalog"
	"github.com/geoffmilleraz/agentic-kit/foundations/cost/cost-tracker:costtracker"
)

// fixtureCatalog mirrors testdata/modelcatalog.json.
fixtureCatalog: modelcatalog.#ModelCatalog & {
	models: {
		"example-judge-1": {
			id:     "example-judge-1"
			family: "beta"
			lineage: []
			caps: {
				thinking:           false
				effort:             false
				temperature:        true
				seed:               false
				structured_outputs: true
			}
			cost_tier:  1
			deprecated: false
		}
		"example-producer-1": {
			id:     "example-producer-1"
			family: "alpha"
			lineage: []
			caps: {
				thinking:           false
				effort:             false
				temperature:        true
				seed:               false
				structured_outputs: false
			}
			cost_tier:  0
			deprecated: false
		}
		"example-judge-same-family": {
			id:     "example-judge-same-family"
			family: "alpha"
			lineage: []
			caps: {
				thinking:           false
				effort:             false
				temperature:        true
				seed:               false
				structured_outputs: true
			}
			cost_tier:  1
			deprecated: false
		}
	}
}

// fixturePriceTable mirrors testdata/pricetable.json.
fixturePriceTable: costtracker.#PriceTable & {
	"example-judge-1": {
		InputPerMTok:      1.0
		OutputPerMTok:     5.0
		CacheReadPerMTok:  0.1
		CacheWritePerMTok: 1.25
	}
	"example-producer-1": {
		InputPerMTok:      0.25
		OutputPerMTok:     1.25
		CacheReadPerMTok:  0.025
		CacheWritePerMTok: 0.3
	}
}
