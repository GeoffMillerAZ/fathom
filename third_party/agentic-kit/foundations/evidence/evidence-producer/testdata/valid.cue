// Package evidenceproducer — valid test fixtures.
// Running `cue vet ./...` and `cue vet ./testdata/` both validate these against schema.cue.
package evidenceproducer

// engineeringCatalog: the fathom ADR-040 review-kit -> findings catalog,
// using the engineering ladder from foundations/review/scoring.
engineeringCatalog: #Catalog & {
	profile: "engineering"
	ladder: ["verified", "pre-scan", "doc-claimed", "sme-stated", "assumed"]
	producers: [
		{
			id:       "native-analyzer"
			kind:     "deterministic"
			origin:   "external"
			tier_cap: "pre-scan"
			requires: []
		},
		{
			id:       "review-kit-model"
			kind:     "model"
			origin:   "external"
			tier_cap: "sme-stated"
			requires: ["review-kit"]
			version:  "review-kit-v3"
		},
		{
			id:       "human-reviewer"
			kind:     "human"
			origin:   "external"
			tier_cap: "sme-stated"
			requires: []
		},
	]
}
