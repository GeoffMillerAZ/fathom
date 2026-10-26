# AGENTS.md — agentic-kit contributor guide

Layered, org-agnostic platform for **governed agentic work**, developed almost entirely by AI agents.
Core idea: **CUE owns the *shape* of data; Go owns *behavior*** — drift between them is a bug.
Module `github.com/geoffmilleraz/agentic-kit`, Go 1.26. New to CUE? Read `docs/why-cue.md` first.

## The gate (non-negotiable)

- `task validate` = `fmt-check` + `vet` + `cue-vet` + `go test -race ./...`. **Must be green before any commit.**
- lefthook hooks: pre-commit = fast staged-file checks (gofmt, `cue vet` on schema dirs); pre-push = full `task validate`.
- **Never `git commit --no-verify` or `LEFTHOOK=0`.** Repair a broken hook through the authorized path (`docs/intent/autonomy.md`).
- `task downstream-check` exercises configured consumers against the kit — run it before tagging or when exported APIs change. Known consumers live in `docs/intent/scope.md`; inspect the current task definitions and report actual coverage. A skipped or unconfigured consumer is not a pass. Fathom is currently checked out at `../sextant`.
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
- `specs/` — docs-as-spec acceptance specs; `docs/` — explainers (`adopting.md`, `agentic.md`, `why-cue.md`, `governance.md`) + the generated `catalog.md`; `docs/project-intent/` — the Project Intent practice; procedures in `.claude/skills/intent-*/` (`.agents/skills` links there for Codex), templates and offline snapshot updater in `intent-init/`
- `AGENTS.md` — the only instruction file. A one-line `@AGENTS.md` `CLAUDE.md` shim exists only where a session type that reads CLAUDE files only must be served (Claude Code cloud until proven; Bedrock/Vertex/telemetry-disabled builds before 2.1.281). A root `CLAUDE.md` switches off native AGENTS.md loading for the whole tree, so a shim sits beside every nested `AGENTS.md` too (https://code.claude.com/docs/en/memory). Never a second copy of anything here (`docs/project-intent/integration.md` §2, §5)
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

The fleet index (consumer-a · agentic-kit · app-kit · fathom) lives in
`../sextant/workspace/db/fleet.sqlite` (that dir's on-disk rename to
`../fathom` is pending — see the sibling's docs/rename-migration.md; scripts
here resolve either name automatically), served to sessions via the
user-scoped `fathom` MCP server. The generated stanza below routes questions
to the current tool surface. Cross-repo: check downstream impact on consumer-a/fathom
symbols before changing exported kit APIs (complements `task downstream-check`).
The daemon owns the SQLite store. Use its extended `ingest` tool to refresh
heavy uncommitted edits; HEAD drift queues refresh automatically. Use extended
`staleness_report` before trusting stale answers. Do not write its store from
a shell CLI ingest.

<!-- fathom:begin v3 sha256:ee3cc97eae5403e2422d7b80ca29911d7150fdb164bc40fa0c6da763b548801a -->
## fathom code intelligence

This repo is indexed by fathom as service "agentic-kit". Route by
question shape, on this session's default eight-tool MCP registration:
where a symbol is defined is `find_symbol`; who-calls it, briefly
(definition, a capped caller/callee list, tests) is `symbol_context`;
what-breaks is `blast_radius`; a composed review verdict over a diff is
`review_diff`; what-governs this repository is `obligations` with no
path (the root conventions); is a plan's box proven is `verify_plan`;
the current text at a cited line is `read_source`. A literal string, a
config value, a log line, or a document-versus-document contradiction
is `search`'s job (grep with a receipt), not the other tools'.
Read files to *edit* them, not to discover them. The wider verb set
(the full resolved caller graph, staleness and what-was-known, and
more) lives one hop away at `/rpc/extended` (or its `/rpc/companion`
alias); register that surface instead of `/rpc/agent` for a session
that needs it.

Opening moves by task shape (first 2-3 calls, then edit):
- bug report / stack trace / failing test: search -> symbol_context -> read_source
- feature/change: search -> obligations -> symbol_context
- review/verify: review_diff -> obligations -> verify_plan
- refactor/delete: blast_radius -> review_diff -> verify_plan

The index refreshes itself: a read of a drifted service answers from
what it has and queues a re-ingest in the background: nothing to call.

Obligations — governing rules for the file you are about to touch — are
injected automatically on every edit via the PostToolUse hook. Treat them
as binding, not advisory.

Continuity: run `fathom prime` at session start if your harness runs no
SessionStart hook. It prints the same identity, freshness, live-example,
thread-brief, and custodian primer this hook injects on Claude Code.
File what the next session must know: `fathom thread note "…"`.

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
cut a new tag after a green gate and a rolled changelog; ask before moving or deleting a tag,
tier demotions, `go.mod` requires, gate edits; never `--no-verify`.
