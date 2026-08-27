# Project intent — router

Durable intent for fathom: why it exists, what must never break, what is in and out of
scope, what may come later, what has been decided and when to reopen it, who may do
what. Practice: `agentic-kit/docs/project-intent/`. Skills: `intent-design` before
design work, then `intent-decide`, `intent-doctor`, `intent-audit`, `intent-prune`,
`intent-release`, `intent-incident`, `intent-init`.

## Docs map

| Question | Read |
|---|---|
| Why this exists, what it should feel like, what it is not | `../design/product-vision.md` (adopted 2026-07-07); the feel: `../design/coding-agent-excellence.md` §0; the thesis: `../vision/01-thesis.md` (review, not binding) |
| How to resolve a tradeoff; laws 1–15; known tensions | `principles.md` |
| What must always / never be true; how each is enforced | `invariants.md` |
| Vocabulary, domain truths, platform facts, assumptions | `domain.md` |
| Current goals and exit tests, boundaries, compatibility surface, non-goals | `scope.md` |
| Plausible futures and how to keep them open for free | `futures.md` |
| Decisions, alternatives, consequences, revisit triggers | `decisions/README.md` (index); records are the `## ADR-NNN` sections of `../../ADR.md` |
| What an agent may do unasked, must ask first, never does | `autonomy.md` |
| What broke, what would have caught it, what it produced | `incidents.md`; catalogs: `../dogfood-observations.md`, `../../.fathom/observations.md` |
| What we do not know yet | `questions.md` |
| What changed for users since the last release | `../../CHANGELOG.md` (narrative per release: `../changelog/`) |
| How to cross a breaking change | `../rename-migration.md` (precedent); new notes: `../upgrades/<version>-<slug>.md` |
| Work orders and their proof | `../plans/` (grammar: `../plans/README.md`; progress: `fathom plans status`) |
| Design records and the vision series | `../design/`, `../vision/README.md` (status per doc) |
| Architecture, surfaces, configuration, usage | `../architecture.md`, `../mcp-tools.md`, `../configuration.md`, `../usage.md` |

## Routing: what to read for which task

| Task | Read first |
|---|---|
| Typo, test fix, refactor inside one package, no behavior change | nothing |
| Implementation touching a sensitive surface (below) | `invariants.md`, `decisions/README.md`, the surface's "Done needs" |
| Feature or behavior change | `scope.md`, `invariants.md`, `principles.md`, `decisions/README.md`, matching `futures.md` entries |
| New subsystem, architecture change, major refactor | the above plus `../design/product-vision.md`, `domain.md`, `futures.md`, the relevant ADR sections in full |
| Any push, tag, delete, purge, deploy, LLM spend, or global-config change | `autonomy.md` |
| Strategy or roadmap work | everything, plus `../vision/08-roadmap.md` and `../vision/24-work-frontier.md` §7 |

## Sensitive surfaces and the evidence "done" needs

| Surface | Why it is sensitive | Done needs |
|---|---|---|
| `internal/store/**` | store purity (I2), on-disk format v0 (I3), WAL pragmas | `go test -race ./internal/store` under `CGO_ENABLED=0`; `TestStoreFormatV0_RequiredSchema`; `TestStorePortability_RoundTrip` |
| `internal/verify/**`, `cmd/fathom/verify.go` | the gate's honesty: exit codes, static-only, tokenizer | `go test -race ./internal/verify ./cmd/fathom`; `fathom verify --strict` on this repo's own plans exits 0 |
| `internal/extractor/**`, `internal/fusion/**` | reachability and edge truth (I23, I24); silent-zero ingest (I25) | golden tests; `TestReachabilityFixtureEndToEnd`; per-language calibration when an extractor changes |
| `internal/mcp/**` | surface membership, budgets, doc sync (I11) | `TestMcpToolsDocMatchesRegistry`; `TestMaintainedLinesNameRegisteredToolsAndVerbs`; `docs/mcp-tools.md` and the use-case `## Maintained` line updated in the same change |
| `internal/semantic/**`, `schema/semantic/**` | claim governance: citations, scopes, tiers, sweep | eval fixtures with scope traps at precision ≥0.90; `task cue-vet` |
| `cmd/fathom/*hook*.go`, `cmd/fathom/brief.go`, `cmd/fathom/sessionhook.go` | injected into every session of every adopter; fail-open (I9, I12) | mutation-RED tests for the property changed; `TestBriefBudgetAndSourceGating`; `TestWholePrimerCap` |
| `cmd/fathom/observatory*.go`, `cmd/fathom/ui/**` | app-kit doctrine; generated CSS | `task ui:css`; a browser or Playwright screenshot in both themes |
| `docs/plans/*.md` | gated claims | `fathom verify --service fathom --plan <plan>` exit 0 before any `[x]` |
| `Taskfile.yml` (deploy), `lefthook.yml`, `scripts/*.sh` | the gates and the daemon install path | `task validate`; `bash -n`; `task deploy`'s serving assertion for deploy changes |
| `go.mod` (kit pin), `third_party/agentic-kit/**` | mirror drift (I15) | `task sync-kit` then `task vendor-check`; `TestPackFilesMatchSource` |
| `.fathom/*.cue`, `internal/custodian/**` | governs this repo's own agent config | `fathom config audit` exit 0; `fathom doctor`; `TestADRReferencesResolveConcern` |
| `internal/metrics/**`, `cmd/fathom/research_report.go` | the numbers the program steers by (law 12) | pre-registered criterion unchanged after data; `TestAbsenceNotesAreArmAware`; `TestAggregateChannels_NeverMeasurable_NotesVerbatim`; `TestBuildResearchReport_DiscoveryShare_ZeroBeats_NeverFabricates100Percent` |
| `CLAUDE.md`, `AGENTS.md` | the always-loaded router and the fleet stanza | `fathom config audit`; stanza paragraphs identical in both files (`intent-doctor` 15) |
| `docs/intent/**`, `ADR.md`, `.fathom/guard.cue` | the intent corpus itself (ADR-026): a wrong condition, rung, or guard note misleads every session | `intent-doctor` pass; `fathom config audit` 0 violated; `fathom intent audit` with 0 unparsed and COMPLETE coverage (ADR-027); `TestUnreadableInputNeverLooksHealthier` |

## Before designing

Say out loud, before proposing a design: which invariant, decision, non-goal, or future
applies, and whether any `Revisit when` / `Promote when` / `Drop if` condition has
fired. Build for now. Foreclose nothing documented when keeping it open is free.

## When to update

- A decision passed the threshold in `decisions/README.md` → append the ADR section, add the row.
- A goal met its `Done when` → remove it. A future met `Promote when` → move to
  `scope.md` and record the decision. A future met `Drop if` → delete it.
- An assumption, invariant, non-goal, or autonomy rule changed → edit in place, same change.
- Something broke → `intent-incident`; it ends as an invariant, a test, or a decision.
- A trigger fired → say so; update after the human decides.
- Trivial implementation work never touches this directory.

List every condition: `grep -rn -E "(Revisit when|Promote when|Drop if|Done when)" docs/intent/ ADR.md`

Verified against: Claude Code 2.1.245, Codex CLI 0.146.0 (2026-08-25).
