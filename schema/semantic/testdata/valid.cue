package semantic

// A well-formed second layer definition (what layer 2 would look like).
failureMode: #Layer & {
	name:           "failure-mode"
	version:        "v1"
	kind:           "failure-mode"
	corpus: include: ["docs/dogfood-observations.md", "docs/postmortems/**"]
	prompt_version: "failure-mode/v1"
	prompt_file:    "schema/semantic/layers/failure-mode/prompt.md"
	tier:           "sme-stated"
	budgets: {
		max_tokens_per_run: 100000
		max_usd_per_run:    0.5
	}
}
