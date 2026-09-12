// bp-claims is a TEST-ONLY fixture pack (docs/plans/bench-pyramid.md
// BP-17..19): the claiming half of O23's shape — plan rows over a
// docs/plans/** corpus, whose scope must come from each row's own
// {package, symbol} contract and never from its prose.
name:    "bp-claims"
version: "v1"
kind:    "spec-binding"
corpus: include: ["docs/plans/**"]
prompt_version: "bp-claims/fixture"
prompt_file:    "bp-claims/prompt.md"
tier:           "doc-claimed"
role:           "claims"
budgets: {
	max_tokens_per_run: 50000
	max_usd_per_run:    0.5
}
