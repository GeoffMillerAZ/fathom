# agentic-kit — project intent (router)

Durable intent for agentic-kit: why it exists, what must never break, what is in and out
of scope, what may come later, what has been decided and when to reopen it, who may do
what. Practice: `../project-intent/` (this repo is the canonical home of the practice and
its first self-adopter). Skills: `intent-design` before design work, then
`intent-decide`, `intent-doctor`, `intent-audit`, `intent-prune`, `intent-release`,
`intent-incident`. This directory routes; good existing documents are pointed to.

## Docs map

| Question | Read |
|---|---|
| Mission, one-sentence test, core beliefs, incubation boundary | [`VISION.md`](../../VISION.md) |
| Why one repository, what belongs, the component map with split/retire triggers, dependency direction | [`repo.md`](repo.md) |
| How to resolve a tradeoff; the tensions that pull both ways | [`principles.md`](principles.md) |
| What must always or never be true, and which lint, task, or guard enforces it | [`invariants.md`](invariants.md) |
| What a governed workflow must prove; implementation vs enforcement vs adoption | [`assurance.md`](assurance.md) |
| Vocabulary, domain truths, assumptions, the named consumers | [`domain.md`](domain.md) |
| Current goals with exit tests, boundaries, compatibility surface, non-goals | [`scope.md`](scope.md) |
| Plausible futures, each with its subscribe-when trigger | [`futures.md`](futures.md) |
| Decisions (D1–D10, D13, D35-*, designs) and revisit triggers | [`decisions/README.md`](decisions/README.md) |
| What an agent may do unasked, must ask first, never does; the branch model | [`autonomy.md`](autonomy.md) |
| What broke, what would have caught it, what it produced | [`incidents.md`](incidents.md) |
| What we do not know yet | [`questions.md`](questions.md) |
| What changed for consumers since the last release | [`CHANGELOG.md`](../../CHANGELOG.md) |
| Package classification and the compatibility promise | [`docs/compat.md`](../compat.md) |
| Anti-pattern ledger and practice guides ([fast-judgment models](../fast-judgment.md) and its [catalog](../fast-judgment-catalog.md); [orchestrating agents](../orchestrating-agents.md); [incident readiness](../incident-readiness.md); [serving rules](../serving-rules.md); [measuring tools](../measuring-tools.md)) | [`docs/practices.md`](../practices.md) |
| Capability backlog, admission triggers and what graduated | [`docs/roadmap.md`](../roadmap.md) |
| Build status, every capability with a checkbox | [`TRACKING.md`](../../TRACKING.md) |
| Work orders and their proof | [`docs/plans/`](../plans/), [`docs/designs/`](../designs/) |
| Current delivery, baseline assessment and composed-workflow design | [`delivery`](../assessments/2026-09-13-governed-workflows.md), [`baseline`](../assessments/2026-09-12-agentic-kit.md), [`design`](../designs/governed-workflows.md) |
| Evidence-based selection, profiles, screening and operational delivery | [`benchmark design`](../designs/benchmark-evidence.md), [`assessment design`](../designs/engineering-assessments.md), [operational ledger](../plans/2026-09-operational-evidence.md), decisions 0011–0014 (live-study deferral) |
| Threat model: what is and is not defended | [`docs/threat-model.md`](../threat-model.md) |

## Routing: what to read for which task

| Task | Read first |
|---|---|
| Typo, test fix, refactor inside one package, no exported change | nothing |
| Implementation touching a sensitive surface (below) | `invariants.md`, decisions index, the surface's "Done needs" |
| New exported API, new package, classification change | `scope.md` (compatibility surface), `invariants.md`, `principles.md`, decisions index, matching `futures.md` entries |
| New kit, runtime, host adapter, or cross-package redesign | the above plus `VISION.md`, `domain.md`, `assurance.md`, `futures.md`, relevant decisions in full |
| Any merge to `main`, tag, deletion, `go.mod` change, or gate edit | `autonomy.md` |
| Roadmap, public-readiness, 1.0 | everything |

## Sensitive surfaces and the evidence "done" needs

| Surface | Why | Done needs |
|---|---|---|
| `go.mod` | stdlib-only is constitution (I1) | `lint empty-require`; a `require` line is a decision |
| `foundations/**`, `kits/**`, `runtimes/**`, `surfaces/**`, `toolchains/**` | the compatibility surface (`compat.md`) | `task validate`; `task downstream-check` for any exported change; `compat.md` row |
| `cmd/constitution-lint/**`, `lefthook.yml`, `Taskfile.yml` | the gate itself | reviewed change; every check's own test green; never loosened to pass |
| `*/schema.cue`, `testdata/*.cue` | CUE owns shape (I5) | `task cue-vet`; schema change to a `stable` package is breaking |
| `foundations/governance/**` | must stay WASM-safe (I9) | `task wasm-build` |
| `foundations/llmclient/**` | encodes external CLI/API behavior (tripwires) | adapter `Ping` green; `Verified against` line updated |
| `TRACKING.md`, `docs/compat.md`, `docs/catalog.md`, `README.md` count | lint-coupled docs | `lint tracking-coverage`, `compat-coverage`, `catalog-fresh`, `readme-count` |
| `fathom-pack/**` | pre-edit guards consumers inherit | cite the incident; ≤ 4 rules per entry; measure or retire |
| Governed workflow/profile claims | optional wiring or skipped checks can look healthy | `assurance.md`; entry-point failure scenarios and a versioned, scoped coverage receipt |

## Before designing

Say out loud, before proposing a design: which invariant, decision, non-goal, or future
applies, whether a `Revisit when` / `Promote when` / `Drop if` condition has fired, and
which consumer or bounded incubation pulls for the change (decision 0004). Build the
present hypothesis; preserve documented options when free. Do not pre-build other futures.

## When to update

- A decision passed the threshold → `intent-decide` (`decisions/NNNN-slug.md`).
- A goal met its `Done when` → remove it. A future met `Promote when` (subscriber or
  admitted incubation) → `scope.md` plus a record. `Drop if` met → delete.
- An assumption, invariant, non-goal, or autonomy rule changed → same change.
- Something broke → `intent-incident`; it ends as a lint check, a guard, a test, or a decision.
- Trivial work never touches this directory.

List every condition: `rg -n "Revisit when|Promote when|Drop if|Done when" docs/intent/`

Verified against: Claude Code 2.1.270, Codex CLI 0.154.0 (2026-09-13), within the
[recorded scopes](../evidence/native-hosts/2026-09-13/README.md); Codex live coverage is unknown.
