<!-- fathom:begin v3 sha256:e2e089f74ee8c0a8457d2e2b23ac8b4e548b3180a0b52cca5e52aa8232841f55 -->
## fathom code intelligence

This repo is indexed by fathom as service "fathom". Route by
question shape (SB-7, docs/vision/27-strengths-bench.md §8 — the home
bench found "index before any grep" over-broad), on this session's
default eight-tool MCP registration (surface reckoning F-86): where a
symbol is defined is `find_symbol`; who-calls it, briefly (definition,
a capped caller/callee list, tests) is `symbol_context`; what-breaks
is `blast_radius`; a composed review verdict over a diff is
`review_diff`; what-governs this repository is `obligations` with no
path (the root conventions); is a plan's box proven is `verify_plan`;
the current text at a cited line is `read_source`. A literal string, a
config value, a log line, or a document-versus-document contradiction
is `search`'s job (grep with a receipt, RC-24), not the other tools'.
Read files to *edit* them, not to discover them. The wider verb set
— the full resolved caller graph, staleness and what-was-known, and
more — lives one hop away at `/rpc/extended` (or its `/rpc/companion`
alias); register that surface instead of `/rpc/agent` for a session
that needs it.

The index refreshes itself: a read of a drifted service answers from
what it has and queues a re-ingest in the background (RT-99) —
nothing to call.

Obligations — governing rules for the file you are about to touch, and
where `docs/intent/` exists its matched invariants — are injected on every
edit via the PostToolUse hook. Treat them as binding, not advisory.

Continuity: run `fathom prime` at session start if your harness has no
SessionStart hook (Codex does not) — it prints the same identity,
freshness, live-example, thread-brief, and custodian primer this hook
injects on Claude Code. File what the next session must know:
`fathom thread note "…"`.

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
- **Index-first**: fathom MCP tools before any Grep/Read sweep; the index
  refreshes itself on a read (RT-99), the `ingest` tool is served only at
  `/rpc/extended`, and never a shell `fathom ingest` against the shared
  store — `docs/guides/fleet-index.md`.
- **UI = app-kit doctrine**: never hand-write observatory markup —
  `cmd/fathom/CLAUDE.md`.
- **Org-agnostic (ADR-011)**: zero org-specific names/hosts/catalogs in code,
  docs, tests, fixtures — `ADR.md`.
- **Plan grammar + docs upkeep**: `docs/CLAUDE.md`, `docs/plans/README.md`.
- Full agent guidance (conventions, source-of-truth map): `CLAUDE.md`.

<!-- appkit:begin -->
## App-kit

App-kit adoption lives at docs/app-kit.md: selected layers/profile, design context,
revision, proof commands and divergences. Before material UI/CLI work, consult it and
the kit's intent router through appkit MCP or the pinned local docs. Select a fitting
capability/recipe; verify the actual task and states in the target UI. Record reusable
improvements or a reasoned divergence and return applicable fixes upstream.
<!-- appkit:end -->

Project-specific contribution routing and disposition evidence live in
`docs/app-kit.md#selected-metadata-and-reusable-contributions-2026-09-16`; retrieve the
shared contract with app-kit MCP `get_kit_decision("contribution-loop")`.

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
