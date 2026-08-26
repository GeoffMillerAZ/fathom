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

Continuity: run `fathom thread brief` at session start if your harness has
no SessionStart hook (Codex does not) — it prints the open threads, or
nothing. File what the next session must know: `fathom thread note "…"`.

Done means proven: a plan task is only "done" when the index proves it, not
when it is claimed. Before checking off a `docs/plans/*.md` item, run:

    fathom verify --service fathom --plan docs/plans/<plan>.md

Exit 0 = every claimed task verifies. Exit 1 = a claimed task is unproven —
fix the code or uncheck the box. Exit 2 = operational (daemon down / index
stale) — fail-open, not a verification failure.
<!-- fathom:end -->

## Repo rules (one line each — the pointer carries the full instructions)

- **workspace/**: ALL build artifacts, logs, temp files, local DBs go under
  gitignored `workspace/`, never elsewhere — `docs/usage.md` §workspace/ convention.
- **Gate**: `task validate` stays green; `lefthook install` once —
  `Taskfile.yml`, `lefthook.yml`.
- **Deploy = `task deploy`, NEVER a raw `cp`**: cp truncates a running
  executable and an unsigned binary loses the daemon's TCC grant —
  `Taskfile.yml` deploy, `.fathom/observations.md` O10.
- **Index-first**: fathom MCP tools before any Grep/Read sweep; refresh via
  the `ingest` MCP tool, never a shell `fathom ingest` against the shared
  store — `docs/guides/fleet-index.md`.
- **UI = app-kit doctrine**: never hand-write observatory markup —
  `cmd/fathom/CLAUDE.md`.
- **Org-agnostic (ADR-011)**: zero org-specific names/hosts/catalogs in code,
  docs, tests, fixtures — `ADR.md`.
- **Plan grammar + docs upkeep**: `docs/CLAUDE.md`, `docs/plans/README.md`.
- Full agent guidance (conventions, source-of-truth map): `CLAUDE.md`.

## Project intent

Durable intent lives at `docs/intent/README.md` (router). Vision: `docs/design/product-vision.md`.
Decisions: `ADR.md` (general decision records, not only architecture; index at
`docs/intent/decisions/README.md`). Consult the router before material product, design, domain, or
architecture work and load only what it routes you to. Record decisions that pass its
threshold; state fired revisit triggers before designing around them.

Change records: conventional commit with a `Verified:` line, one commit per reviewable
unit, no WIP on main. A `CHANGELOG.md` line in the same change only for observable
behavior (compatibility surface: `docs/intent/scope.md`); `### Breaking` first; collapse
`[Unreleased]` to the delta, never the journey. Rules: `agentic-kit/docs/project-intent/changes.md`.

Autonomy (`docs/intent/autonomy.md`): commit and push to `main` freely (the integration
branch from v0.5.0), and roll the changelog and tag after a green wave; ask before deleting,
purging, spending LLM budget, or touching global agent config; never force-push `main`,
raw-`cp` the daemon, skip a gate, or claim done without `Verified:`.
