// demo — a layer pack in the shape fathom layers install reads
// (schema/semantic #Layer): a name, a corpus, a prompt and the budgets.
_spend: {
	tokens: 200000
	usd:    1.0
}

name:    "demo"
version: "v1"
kind:    "adr-rule"
corpus: include: ["README.md", "docs/**"]
tier:    "doc-claimed"
stratum: "project"
budgets: {
	max_tokens_per_run: _spend.tokens
	max_usd_per_run:    _spend.usd
}
