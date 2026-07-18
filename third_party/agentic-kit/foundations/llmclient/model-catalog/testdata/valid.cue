// Package modelcatalog — valid test data that must unify with schema.cue.
// Running `cue vet ./...` from the package root and `cue vet ./testdata/` both
// validate these against the schema.
//
// EXAMPLE DATA ONLY. Every model id, family name, and numeric value in this
// file is an illustrative placeholder chosen to exercise the schema — these are
// NOT real models, NOT real families, and must never be copied into production
// config. Real deployments supply their own catalog from current provider
// documentation.
package modelcatalog

import "github.com/geoffmilleraz/agentic-kit/foundations/llmclient/model-catalog:modelcatalog"

// validMinimalCatalog is the smallest valid catalog: one model with all
// required fields and cap defaults exercised.
validMinimalCatalog: modelcatalog.#ModelCatalog & {
	models: {
		"example-fast-1": {
			id:     "example-fast-1"
			family: "alpha"
			lineage: []
			caps: {
				thinking:           false
				effort:             false
				temperature:        true
				seed:               false
				structured_outputs: false
			}
			cost_tier: 0
			deprecated: false
		}
	}
}

// validFullCatalog exercises multiple families, lineage references, all cap
// flags, cost tiers, and a deprecated model.
validFullCatalog: modelcatalog.#ModelCatalog & {
	models: {
		// alpha family — two models, one deprecated.
		"example-fast-1": {
			id:     "example-fast-1"
			family: "alpha"
			lineage: []
			caps: {
				thinking:           false
				effort:             false
				temperature:        true
				seed:               true
				structured_outputs: true
			}
			cost_tier: 0
			deprecated: false
		}
		"example-fast-1-legacy": {
			id:     "example-fast-1-legacy"
			family: "alpha"
			lineage: []
			caps: {
				thinking:           false
				effort:             false
				temperature:        true
				seed:               false
				structured_outputs: false
			}
			cost_tier: 0
			deprecated: true
		}

		// beta family — a capable model that cites alpha lineage (distilled).
		"example-smart-1": {
			id:     "example-smart-1"
			family: "beta"
			lineage: ["alpha"]
			caps: {
				thinking:           true
				effort:             true
				temperature:        true
				seed:               false
				structured_outputs: true
			}
			cost_tier: 2
			deprecated: false
		}

		// gamma family — an independent model with no lineage.
		"example-judge-1": {
			id:     "example-judge-1"
			family: "gamma"
			lineage: []
			caps: {
				thinking:           false
				effort:             false
				temperature:        true
				seed:               true
				structured_outputs: true
			}
			cost_tier: 1
			deprecated: false
		}
	}
}

// validCostTierZero exercises cost_tier at the minimum boundary.
validCostTierZero: modelcatalog.#ModelCatalog & {
	models: {
		"example-micro-1": {
			id:     "example-micro-1"
			family: "delta"
			lineage: []
			caps: {
				thinking:           false
				effort:             false
				temperature:        true
				seed:               false
				structured_outputs: false
			}
			cost_tier: 0
			deprecated: false
		}
	}
}

// validDeprecatedOnly exercises a catalog whose sole model is deprecated (valid
// schema-wise; Validate will report an all_deprecated_family problem, but the
// catalog itself is structurally legal).
validDeprecatedOnly: modelcatalog.#ModelCatalog & {
	models: {
		"example-old-1": {
			id:     "example-old-1"
			family: "epsilon"
			lineage: []
			caps: {
				thinking:           false
				effort:             false
				temperature:        true
				seed:               false
				structured_outputs: false
			}
			cost_tier: 0
			deprecated: true
		}
	}
}

// validEmptyCatalog is the smallest valid catalog: no models. Every Entry
// lookup returns (zero, false).
validEmptyCatalog: modelcatalog.#ModelCatalog & {
	models: {}
}
