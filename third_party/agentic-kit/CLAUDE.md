# CLAUDE.md — agentic-kit contributor guide

Layered, org-agnostic platform for **governed agentic work**, developed almost entirely by AI agents.
Core idea: **CUE owns the *shape* of data; Go owns *behavior*** — drift between them is a bug.
Module `github.com/geoffmilleraz/agentic-kit`, Go 1.26. New to CUE? Read `docs/why-cue.md` first.

## The gate (non-negotiable)

- `task validate` = `fmt-check` + `vet` + `cue-vet` + `go test -race ./...`. **Must be green before any commit.**
- lefthook hooks: pre-commit = fast staged-file checks (gofmt, `cue vet` on schema dirs); pre-push = full `task validate`.
- **Never `git commit --no-verify`.** Opt out only with `LEFTHOOK=0`, and only if the hook itself is broken.
- `task downstream-check` builds + tests both real consumers (fathom — currently checked out at `../sextant`, the on-disk dir rename to `../fathom` is still pending — and `../throughline`) against kit HEAD — run it before tagging or when exported APIs change; each check skips if the sibling isn't checked out, and resolves either dir name automatically.
- `task doctor` checks toolchain health. `task --list` shows everything.

## Hard rules

- **stdlib only.** Zero external Go dependencies, ever. `go.mod` has no `require` block — keep it that way.
- **Org-agnostic.** No company/product names, no hard-coded catalogs (sensitivity rules, taxonomies, gateways). Anything org-specific is CUE/config data validated against a schema.
- **Determinism.** Inject clocks (`now func() time.Time` — never bare `time.Now()` in library code) and seedable randomness. Every time-touching package exposes an injectable clock seam; a nil/unset clock MAY default to `time.Now` only when that default is documented on the seam (kits do this routinely; a handful of foundations do too). What is forbidden is a bare `time.Now()` with no seam. Table-driven tests, no sleeps, no network in tests (httptest only). All tests pass with `-race`.
- **Layering.** foundations import only stdlib + sibling foundations. kits compose foundations. runtimes/surfaces/toolchains build on both. Imports point downward only — nothing imports from kits/apps back into foundations.
- **Concurrency contract** stated in every package doc comment (e.g. "safe for concurrent use via sync.Mutex").
- Dirs are kebab-case; package names collapse the hyphen (`rate-limiter` → `package ratelimiter`).
- `foundations/governance/*` must stay WASM-safe (compiles to `GOOS=wasip1` / `js`): no file I/O, no goroutine leaks.

## Package shape (every foundation sub-package)

```
schema.cue    CUE contract (types, constraints, valid value space)
types.go      Go structs/interfaces mirroring the CUE contract
<impl>.go     implementation
mock.go       test double (where an interface exists)
*_test.go     contract + behavior tests
testdata/*.cue  positive fixtures (cue vet checks them); negative cases = commented docs + Go rejection tests
```

Skip `schema.cue` only for **behavioral-only** packages with no data contract (e.g. `foundations/llmclient`).

## Where things live

- `foundations/` — narrow stdlib-only packages, the durable nouns (contracts, evidence, review, security, governance, … — full category list in `docs/catalog.md`)
- `kits/` — high-order workflows composed from foundations (agent-harness, mcp-server, investigation, command-center, endpoint-guard, governed-prose-kit)
- `runtimes/` — execution, orchestration, policy, connector substrates
- `surfaces/` — human/developer-facing interfaces (forms/cueform, editor, demo)
- `toolchains/` — canonical CUE schemas, generators, materialization
- `cmd/` — runnable binaries (`endpoint-guard` sidecar, `kit-catalog` docs generator); `examples/` — e.g. governance-wasm; `scenarios/` — seeded end-to-end blueprints
- `specs/` — docs-as-spec acceptance specs; `docs/` — explainers (`adopting.md`, `agentic.md`, `why-cue.md`, `governance.md`) + the generated `catalog.md`; `docs/project-intent/` — the Project Intent practice (methodology, agent integration, examples); its skill and templates live in `.agents/skills/project-intent/`
- `workspace/` — runtime/state artifacts, not tracked; `tmp/` — gitignored design docs (`tmp/design_inspiration/files/`)
- `_reference/` dirs — reconstructed scanned sources next to the packages they inform. Go ignores them; **never import them**, only port from them.

## TRACKING.md discipline

- Single source of truth for build status. **Every capability gets a checkbox.**
- Mark `[x]` only when tests pass **and** the full gate is green. Legend: `[ ]` / `[~]` in progress / `[x]` done & tested / `[-]` deferred.
- Update it in the same change that lands the work.

## Git

- Conventional-ish subjects: `feat(foundations): …`, `docs: …`, `fix: …` — scope optional, body explains what + why.
- No co-author trailers, human or AI. Only trailers the harness adds itself.
- Commit only with the gate green; hooks enforce this — don't fight them.

## Working style for agents

- Scope or junk-gate question (add / defer / retire)? `VISION.md` is the arbiter — judge the change against its one-sentence test and junk gate.
- Read the sibling packages in a category before adding one — match their shape exactly.
- Mock where an interface exists; tests use injected clocks (`fixedClock`/`advancingClock` patterns are common in tests).
- Verify before reporting done: `gofmt -l` on touched files (must print nothing), `go vet ./...`, `go build ./...`, `go test -race` on your packages — or just `task validate`.

## fathom fleet index (code intelligence — use it first)

The fleet index (throughline · agentic-kit · app-kit · fathom) lives in
`../sextant/workspace/db/fleet.sqlite` (that dir's on-disk rename to
`../fathom` is pending — see the sibling's docs/rename-migration.md; scripts
here resolve either name automatically), served to sessions via the
user-scoped `fathom` MCP server. Resolve "where/what/who-calls" via its tools
(`find_symbol`, `query_symbols`, `find_callers`, `outline`, `repo_map`,
`pack_context`) before any Grep/Read sweep — read files to edit them, not to
discover them. Cross-repo: check downstream impact on throughline/fathom
symbols before changing exported kit APIs (complements `task downstream-check`).
Refresh via the MCP `ingest` tool (`{"service":"agentic-kit"}`) at session
start and after commits (hash-gated, ~0.2 s; the daemon owns the SQLite
store — refresh through it rather than a shell CLI ingest; a
best-effort post-commit lefthook covers no-session commits). After heavy
uncommitted edits run `staleness_report` before trusting results.

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
