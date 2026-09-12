// bp-governs is a TEST-ONLY fixture pack (docs/plans/bench-pyramid.md
// BP-17..19): the governing half of O23's shape — a human-written rule
// about the file being edited. Nothing in internal/semantic knows its name.
name:    "bp-governs"
version: "v1"
kind:    "obligation"
corpus: include: ["CLAUDE.md"]
prompt_version: "bp-governs/fixture"
prompt_file:    "bp-governs/prompt.md"
tier:           "doc-claimed"
role:           "governs"
budgets: {
	max_tokens_per_run: 50000
	max_usd_per_run:    0.5
}
