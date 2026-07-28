<!-- fathom:begin -->
## fathom code intelligence

This repo is indexed by fathom as service "fathom". Before any Grep/Read
sweep, resolve where/what/who-calls via the fathom MCP tools:
`find_symbol`, `references`, `symbol_context`, `outline`, `repo_map`,
`pack_context`, `verify_plan`. Read files to *edit* them, not to discover
them — the index replaces exploratory grep.

Refresh the index via the `ingest` MCP tool (`{"service":"fathom"}`) at
session start and after each commit; it is hash-gated, so a clean repo
refreshes in well under a second.

Obligations — governing rules for the file you are about to touch — are
injected automatically on every edit via the PostToolUse hook. Treat them
as binding, not advisory.

Done means proven: a plan task is only "done" when the index proves it, not
when it is claimed. Before checking off a `docs/plans/*.md` item, run:

    fathom verify --service fathom --plan docs/plans/<plan>.md

Exit 0 = every claimed task verifies. Exit 1 = a claimed task is unproven —
fix the code or uncheck the box. Exit 2 = operational (daemon down / index
stale) — fail-open, not a verification failure.
<!-- fathom:end -->
