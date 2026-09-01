<!-- fathom:begin -->
## fathom code intelligence

This repo is indexed by fathom as service "agentic-kit". Before any Grep/Read
sweep, resolve where/what/who-calls via the fathom MCP tools:
`find_symbol`, `references`, `symbol_context`, `outline`, `repo_map`,
`pack_context`, `verify_plan`. Read files to *edit* them, not to discover
them — the index replaces exploratory grep.

Refresh the index via the `ingest` MCP tool (`{"service":"agentic-kit"}`) at
session start and after each commit; it is hash-gated, so a clean repo
refreshes in well under a second.

Obligations — governing rules for the file you are about to touch — are
injected automatically on every edit via the PostToolUse hook. Treat them
as binding, not advisory.

Continuity: run `fathom thread brief` at session start if your harness has
no SessionStart hook (Codex does not) — it prints the open threads, or
nothing. File what the next session must know: `fathom thread note "…"`.

Done means proven: a plan task is only "done" when the index proves it, not
when it is claimed. Before checking off a `docs/plans/*.md` item, run:

    fathom verify --service agentic-kit --plan docs/plans/<plan>.md

Exit 0 = every claimed task verifies. Exit 1 = a claimed task is unproven —
fix the code or uncheck the box. Exit 2 = operational (daemon down / index
stale) — fail-open, not a verification failure.
<!-- fathom:end -->

## Project intent

Durable intent lives at `docs/intent/README.md` (router). Vision: `VISION.md`.
Decisions: `docs/intent/decisions/` (index at `docs/intent/decisions/README.md`; D1–D10
point at the uplift plan). Consult the router before material product, design, domain,
or architecture work and load only what it routes you to. Record decisions that pass its
threshold; state fired revisit triggers before designing around them.

Change records: conventional commit with a `Verified:` line, one commit per reviewable
unit. A `CHANGELOG.md` line in the same change only for observable behavior
(compatibility surface: `docs/intent/scope.md`); `### Breaking` first; collapse
`[Unreleased]` to the delta, never the journey. Rules: `docs/project-intent/changes.md`.

Autonomy (`docs/intent/autonomy.md`): topic branches off `main` are free; fast-forward to
`main` only with `task validate` (and `downstream-check` for exported changes) green;
ask before tags, tier demotions, `go.mod` requires, gate edits; never `--no-verify`.
