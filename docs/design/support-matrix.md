# The support matrix — what fathom supports, computed, never kept

Status: accepted 2026-09-27 (ADR-054). Plan: `docs/plans/support-matrix.md` (prefix `SM`).
Schema: `schema/support/`.
Builds on ADR-048 (a level is a reading, not a registration; I54), ADR-029 (harness
adapters are data behind one seam), ADR-018 (CUE is schema and validator, never an
authoring or execution surface), ADR-045 (every console panel renders a verb; every number
carries its instrument), law 3 (absent is never zero), law 4 (no net MCP tool growth).

## 1. The question and the decision

The owner asked: should fathom have a catalog that tracks implementation quality and
progress for languages and tools? **Yes, as a projection — never a hand-maintained status
list** (owner decision, binding).

Every supported thing already has, or gets, a data registry the code loads. The matrix
joins three things per entry:

1. **Declared target** — from the registry's target list (the language queue for languages
   and tool formats; a target file for the other kinds).
2. **Computed state** — from committed readings and from `fathom verify` of the plan row
   that owns the entry.
3. **Evidence link** — the reading path and commit, and the owning plan row id.

A level or status with no reading renders `declared` or `tolerated`. It is never a claimed
level.

### 1.1 Why not a hand-kept catalog

- **It rots.** Fathom's own records show it: the compatibility surface in
  `docs/intent/scope.md` still says "the 19-tool agent surface" while 25 tools are served
  (`docs/design/surface-reckoning.md` §1), and the roadmap memo still lists `package_deps`
  a month after it was deleted. A table that people type drifts from the code by the next
  commit.
- **It is the failure fathom exists to catch.** A catalog cell that says "L3" with no
  reading is a claimed-unproven row, the thing `verify` refuses in plans. ADR-048 decision
  2 already settled the rule for languages: a level counts only with a reading, and a test
  refuses a tracker level above its reading. The matrix extends that rule to every kind.
- **The data already exists.** Dialect manifests, the queue, harness descriptors and the
  plan rows are all machine-read. A hand-kept list would be a second, weaker copy of them.

### 1.2 Why not CUE-authored data

CUE is fathom's schema and validator (`schema/**`, `task cue-vet`); ADR-018 rejected it as
an authoring surface or execution engine after the Dagger post-mortem (CUE as authoring
UX and CUE as engine both failed with a funded team). The entries stay JSON (target lists,
readings) and Go data (registries), authored where their owners already work. CUE checks
their shape: `schema/support/schema.cue` validates the target file, the non-language
readings and the `--json` output, with valid/invalid testdata pairs (`schema/AGENTS.md`).
Nobody writes a matrix row in CUE.

## 2. Kinds, registries and targets

| Kind | Registry (what fathom loads) | Target list (declared) | Readings (computed) | Capability columns | Owning plan |
|---|---|---|---|---|---|
| `language` | dialect manifests `internal/extractor/dialects/<id>/dialect.json` (LB-F1); until LB-F1 lands, the extractor registry's language table | `docs/plans/language-breadth.queue.json` (`kind: language`) | `docs/bench/readings/lang/<id>.json` (`level_met`, `grade`) | L1, L2, L3, L4 | `language-breadth.md` (`LB-Q-<id>`) |
| `tool` | the same manifests (tool formats) | the queue (`kind: tooling`) | the same readings | L1, L2, L3 (L4 is `n/a`) | `language-breadth.md` |
| `client` | harness descriptors (`internal/harness`, `All()`) | `docs/plans/support.targets.json` | `docs/bench/readings/support/client/<id>.json`, written by a live check | the harness `Capability` list (`mcp-tools`, `stanza`, `skills`, `prime`, `obligations-post-edit`, `guard-pre-edit`, `stop-verify`, …) | this plan (SM) and the multi-client hook rows (vision 38) |
| `spec` | spec-dialect adapters (N4) | `support.targets.json` | `…/support/spec/<id>.json` | read, verify, converge, anchor-infer | spec-interop plan (vision 38) |
| `rules` | rules-file readers (N8.3) | `support.targets.json` | `…/support/rules/<id>.json` | read, levels, served-at-edit | governance plan (vision 38) |
| `coverage` | coverage-format parsers (T3; Go cover exists today) | `support.targets.json` | `…/support/coverage/<id>.json` | parse, block-join, per-layer | test-intelligence plan (TQ) |
| `contract` | contract-key kinds (N6: `route`, `rpc`, `gql`, `topic`, `table`, `env`, `config`, `pkg`, `image`, `deploy`, `cli`) | `support.targets.json` | `…/support/contract/<id>.json` | provider-extract, consumer-extract, link-precision, link-recall, drift | system-graph plan |
| `ecosystem` | dependency ecosystems (N1: Go, npm, PyPI, Maven, Cargo, NuGet) | `support.targets.json` | `…/support/ecosystem/<id>.json` | cache-read, lockfile-graph, api-index, upgrade-impact, docs, vuln-reach | dependency-plane plan |

The language queue stays the target list for languages and tool formats; LB-F22
(`TestLanguageQueueMatchesPlan`) keeps governing it. The other six kinds share one file,
`docs/plans/support.targets.json`, an array of
`{kind, id, name, target: {<capability>: <level-or-true>}, plan_row, wave}`. A registry
entry names its kind and id in code (a descriptor's `ID`, an adapter's `ID()`), so the join
key is `(kind, id)` everywhere.

A registry in a later plan (spec adapters, rules readers, parsers, contract kinds,
ecosystems) is a Go slice or table the code already needs to dispatch; the matrix only
requires that each exposes `(kind, id, capabilities)` through one small interface,
`support.Registry`, so the projection never imports the engines.

## 3. Computed state

For each entry and capability the state is one of a closed list. The computation is a pure
function of (registry, targets, readings, plan verdicts) with an injected clock; output is
sorted by kind, then id, then capability order.

| State | Rule | Shape and label |
|---|---|---|
| `proven-excellent` | a committed reading meets the capability's excellence bar, and the owning plan row verifies (or the capability has no row) | ● excellent |
| `proven-good` | the reading meets the good bar only | ◐ good |
| `below-target` | a reading exists and proves a level below the declared target | ▽ below target (with the level met) |
| `tolerated` | a registry entry exists, no reading (I54 for languages) | ○ tolerated |
| `declared` | a target row exists, no registry entry yet (planned, not built) | ◇ declared |
| `regressed` | the readings ledger shows the target was met once and the latest reading misses it (the U12 idea for support) | ✕ regressed |
| `refused` / `deferred` | the owning plan row is `[-]` with that disposition | ⊘ refused / ◌ deferred (with the trigger) |
| `n/a` | the capability does not apply to the kind (L4 for a tool format) | — |

A cell is never colour alone: the shape and the label carry the state, and colour (role
utilities only) repeats it. No reading means no level: a cell cannot say `L3` unless a
reading computed it.

**Client readings.** A harness descriptor's `VerifiedAgainst` is authored today. It becomes
an input the reading must corroborate: `fathom harness verify --harness <id>` (a live
check on the machine, never a network call) writes the client reading. Claude Code and
Codex get their first readings from the live tests that already exist (SM-12), so they do
not fall to `tolerated` on day one; Cursor, Copilot and Gemini stay `tolerated` until
verified (F-59).

### 3.1 The closure test

`TestSupportRegistriesMatchTargets` fails when:

- a registry entry has no target row (support nobody planned or measured);
- a target row names no registry entry **and** its `plan_row` is claimed `[x]` (claimed
  built, not registered) — or its `plan_row` names a row that does not exist;
- a reading names an `(kind, id)` in neither the registry nor the targets.

A target row whose plan row is still open is `declared`, not a failure: that is how planned
support is visible before it exists. This is the owner's "a target row names no registry
entry" rule, read so that planning ahead stays possible.

### 3.2 History

Readings are overwritten per entry, so progress needs a ledger. Every reading write also
appends one line to `docs/bench/readings/support-ledger.jsonl`:
`{kind, id, capability, level_met, grade, reading_date, commit}` (committed, append-only,
sorted on write by date then key). The burn-up and `regressed` are computed from it; a gap
renders as absent, never as zero.

## 4. Surfaces

All surfaces render one projection, `internal/support` (`Project(Input) Matrix`), with a
`fathom.support/v1` JSON shape. No MCP tool is added (law 4); agents that need it read the
CLI or the generated doc.

- **CLI.** `fathom support [--kind language|tool|client|spec|rules|coverage|contract|ecosystem] [--json]`.
  The text form is a table per kind with a one-line summary per kind (entries at target,
  below, tolerated, declared).
- **Generated doc.** `docs/support.md`, rendered by `fathom support --render-docs`;
  `TestSupportDocIsGenerated` fails when the committed file differs. LB-F17's tracker
  becomes the `language` and `tool` sections of this one generator, not a second one:
  `RenderTracker` is called by the support renderer, and LB-F17 and the LB-M6 milestone
  rows are repointed from `docs/languages.md` to `docs/support.md` in the commit that
  lands the generator (SM-9). `docs/languages.md` is then removed.
- **Console.** A Support page (§5).
- **Artifact data.** The roadmap artifact reads `fathom support --json`.

## 5. The console Support page

Route `/support`, group System, declared through `declarePage` in
`cmd/fathom/console_support.go`; the projection lives in `internal/console` and renders
`fathom support --json` (ADR-045: every panel renders a verb, and its footer names the
verb, the sources, the denominator and the freshness). App-kit doctrine
(`cmd/fathom/AGENTS.md`): search the kit and the appkit MCP before any markup; role
utilities only; `stat.Absent` for anything uninstrumented; CSS through `task ui:css`; a
screenshot of both themes before the row is claimed. If the kit has no heatmap, it is built
and promoted upward (D13), not hand-rolled in this repo.

Panels:

1. **Heatmap.** Rows are entries (languages, tool formats, clients, spec formats, rules
   files, coverage formats, contract kinds, ecosystems); columns are levels or capabilities
   (L1–L4 for languages, the kind's capability list otherwise); each cell is the computed
   state from §3 as shape, label and colour. Hover or click opens the evidence: the reading
   (path, commit, date, the metrics that decided it) and the plan row with its verify
   verdict.
2. **Dialects.** One row per dialect data directory (manifest, queries or selectors,
   fixtures): engine, grammar pin (repo@tag) and licence from `grammars.lock.json`, level
   reached, last reading date, and the embedded blob size with the running total against
   the 40 MiB cap. The cap line is the trigger for dialect packs: packs (the out-of-binary
   install and trust path, LB-F14/LB-F27) are deferred by owner decision until embedded
   grammars near the cap, so the page shows how near they are and plans no pack UI.
3. **Progress over time.** Burn-up of entries at target per kind, from the readings
   ledger (§3.2), with the milestone lines (M1–M3 for languages) drawn from the queue.
4. **What's next.** The next eligible queue items by wave: target rows whose
   `depends_on`/`needs` are all claimed or dispositioned, the same rule as
   `plans next --eligible-only`.

Filters: kind, wave, engine, state. The filter state is in the URL, so an answer can be
linked.

**UX measurement (SM-M2).** "Which languages are below target in wave 1?" is answered in
three clicks or fewer from Now, and the answer equals `fathom support --kind language --json`
filtered the same way (the UX-M1 method, `docs/bench/readings/ux-m1-m2-console-2026-09-26.md`).

## 6. What is not built

- No status field anyone sets by hand, no drag, no write-back (the no-tracker non-goal).
- No network: a client reading is a local live check; an ecosystem reading reads caches
  the toolchain already filled (N1).
- No pack UI (deferred with packs).
- No MCP tool.
