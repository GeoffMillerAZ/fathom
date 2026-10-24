// budgeted — a layer pack whose budgets name a hidden table of its own.
_spend: {
	tokens: 150000
	usd:    0.5
}

name:           "budgeted"
version:        "v1"
kind:           "adr-rule"
corpus: include: ["README.md", "docs/**"]
prompt_version: "budgeted/content-addressed"
prompt_file:    "prompt.md"
tier:           "doc-claimed"
budgets: {
	max_tokens_per_run: _spend.tokens
	max_usd_per_run:    _spend.usd
}
