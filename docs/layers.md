# Semantic layer catalog

A **semantic layer** compiles a governing corpus into governed claims
(citation-grounded, tiered, staleness-tracked) and serves them at decision
moments — see `docs/design/semantic-layers.md` for the founding design and
`docs/vision/02-shape.md` §3 for the module-pack shape this catalog
implements.

A layer ships as a **module pack**: a directory of data, never Go code.

```
<name>/
  layer.cue    # plain data (no package clause, no "#Layer &" wrapper):
               # name, version, kind, corpus globs, prompt_file, tier,
               # budgets, serving caps — unified against schema/semantic/
               # schema.cue's #Layer definition by internal/semantic's loader
  prompt.md    # the compile system prompt; content-addressed
               # (PromptVersion = "<name>/" + sha256(prompt.md bytes)[:8])
  eval/        # labeled fixtures (governing docs + labels.json) for the
               # precision/recall shipping gate — see "The gate" below
  KILL.md      # the kill gate: metric, window, decision date
```

Built-in packs live at `schema/semantic/layers/<name>/` (authoritative) and
`internal/semantic/layers/<name>/` (a byte-identical `go:embed`'d mirror,
drift-tested by `TestPackFilesMatchSource`). A project adds its own packs at
`.fathom/layers/<name>/` — **presence = enabled, for compilation** (ADR-018
reconciler doctrine: there is no separate enable/disable state anywhere, and
deliberately no `fathom layers enable` verb). A project pack overrides an
embedded pack of the same name.

Two honesty notes on that doctrine:

- **Serving derives from the STORE, not from pack presence.** Removing a
  pack directory stops future *compiles*; claims it already compiled keep
  serving until retired — refuted per claim (`fathom claim refute`) or
  superseded by a recompile. `fathom layers list --store --service`
  surfaces such **orphaned layers** (claims in the store, no pack in the
  union) so an injected rule is always traceable; a `layers uninstall` verb
  that also retires claims is deferred (LC-13,
  `docs/plans/layer-catalog.md`).
- **A broken pack never takes the union down.** A half-authored
  `.fathom/layers/<name>/` (missing/undecodable `layer.cue`) is skipped
  from resolution — every other layer, including an embedded pack the
  broken directory would have overridden, keeps compiling and serving — and
  the breakage is warned once on stderr and shown by `fathom layers list`.
  Only requesting the broken pack by name errors.

## The catalog

**A4-1 (docs/plans/activation.md): two columns, two different truths.**
**Eval** is a gate ADMISSION fact: hand-authored (each pack's `layer.cue`
carries a self-declared `evaled: bool`, checked against that pack's own
`eval/RESULTS.md` at authoring time — see "The gate" below) and static once
recorded; it changes only when a pack is re-gated. **Production** is a LIVE
store fact — claim count + last compile date — and is **never
hand-maintained here**: run `fathom layers list --store PATH --service NAME`
for the current numbers (the CLI reads them straight from `compile_runs`/
`semantic_claims`, so this table can't drift stale the way a hardcoded count
would). `fathom compile` itself enforces the Eval column: a non-dry-run
compile of an `evaled: false` layer refuses unless `--unsafe-skip-eval-gate`
overrides it (D3-1, docs/plans/expert-depth.md).

**Stratum** (ADR-024, `docs/design/pack-taxonomy.md`) names which of the
four-stratum pack taxonomy's strata this pack instance belongs to — schema
data (`#Layer.stratum`), not a trust or enable/disable mechanism. All four
shipped packs are `concern`: fathom-authored, eval-gated before shipping,
identical for every adopting repo — including `obligations`, whose corpus
is org-specific content but whose pack (prompt, corpus selector, eval
fixtures) is not (see the design doc's "The obligations call" for the full
reasoning). `platform` is reserved for an org's own pack installed under
`.fathom/layers/`, authored and eval-gated on-disk under the D3 trust seam
(`internal/semantic/packs.go`'s `loadUnion`) — no shipped pack is platform
stratum today.

| Layer | Kind | Tier | Stratum | Corpus | Drift-tracked | Eval | Production |
|---|---|---|---|---|---|---|---|
| `obligations` | `obligation` | `doc-claimed` | `concern` | `CLAUDE.md`, `ADR.md`, `docs/plans/README.md` | no | pass — ADR-016 baseline (see `eval/RESULTS.md`; a later production-model recheck found recall unstable around the bar, 0.71/0.86 — escalated as a pre-OP-6 input, not a gate failure) | `fathom layers list --store PATH --service NAME` |
| `findings` | `finding` | `sme-stated` | `concern` | `findings/**` | **yes** | **not-gated** — shipped without eval/ yet, see its `KILL.md`; `fathom compile` refuses this layer without `--unsafe-skip-eval-gate` | `fathom layers list --store PATH --service NAME` |
| `adr-rules` | `adr-rule` | `doc-claimed` | `concern` | `ADR.md`, `docs/design/*.md` | no | pass — precision 1.00 / recall 1.00, 7 rules + 2 traps, production model pinned | `fathom layers list --store PATH --service NAME` |
| `spec-bindings` | `spec-binding` | `doc-claimed` | `concern` | `docs/plans/**` excl. `README.md` | no | pass — precision 1.00 / recall 1.00, 6 rules + 2 traps, production model pinned | `fathom layers list --store PATH --service NAME` |

### `obligations` — every edit

**When to use:** always on, every fleet repo. The v1 tracer bullet (ADR-016
§4.8 #1): specs/standards/decisions governing the file about to be edited,
injected by the PostToolUse edit-hook.

**Corpus:** binding documents only — `CLAUDE.md`, `ADR.md`,
`docs/plans/README.md`. Design essays are deliberately excluded (they
produced noisy aspirational rules on the first fleet compile).

**Cost:** single-digit cents per full compile of a repo this size; $0
marginal on code commits (claims anchor to slow-churn docs, joined to code
by scope glob at query time); pennies on doc edits (per-section digest
gating).

### `findings` — review/audit conclusions with remediation drift

**When to use:** a repo with a `findings/` directory of one-markdown-file-
per-finding review/audit conclusions (`docs/use-cases/change-review.md`).
The only layer with `serving.drift_tracked: true` — its claims carry
`code_drifted` when the SCOPED CODE (not the cited finding doc) changes
since compile, a live serving annotation never a status flip
(docs/vision/03-findings-and-experts.md §2).

**Corpus:** `findings/**`.

**Triage:** `fathom claim refute|corroborate` — see
`docs/use-cases/change-review.md`.

### `adr-rules` — architectural constraints (the layer-2 proof)

**When to use:** any repo with decision records / design docs whose
constraints should surface at edit time, the same way obligations does for
`CLAUDE.md`/`ADR.md`'s developer rules. This is the layer-2 proof
(docs/vision/02-shape.md, docs/vision/08-roadmap.md): the first layer no
code in this repo was written for by name — `fathom layers install` plus
this pack's own `layer.cue`/`prompt.md`/`eval/` is the whole delta.

**Corpus:** `ADR.md`, `docs/design/*.md`.

**Prompt discipline:** an explicit "Consequences paragraph that only
describes what a decision does NOT bind" scope-trap rule — a decision
naming what it excludes must not leak scope onto the excluded package.

**Corpus overlap with `obligations`:** `ADR.md` is in both packs' corpora,
deliberately — obligations extracts an ADR's *developer rules*, adr-rules
its *architectural constraints*. The same source line can therefore yield
one claim of each kind, and both can appear in the same served payload
(deduplication is by claim identity, not text); the 6-rule serving cap
bounds the cost. If that double-serving proves noisy in practice, narrow
one pack's corpus in a project-local override.

### `spec-bindings` — gated-plan task bindings (the SDD pack)

**When to use:** repos using the `docs/plans/*.md` gated-checklist grammar
(`docs/plans/README.md`) who want an open task's binding requirement served
at edit time for the package it names — feeding the same serving channel
the edit-hook already renders kind-aware (a `spec-binding` claim shows up
labeled, exactly like a `finding`).

**Corpus:** `docs/plans/**` excluding `README.md` (the grammar reference
itself carries no gated tasks).

**Extraction discipline:** only task lines whose annotation states a
`package:` value are extracted; a `disposition: deferred`/`refused` task or
an unannotated line is skipped, not guessed at — proven by two deliberate
traps in `eval/plan-fixture.md`.

**Completed tasks compile too — by design.** A `[x]` task's binding still
describes the package's contract (the work was done *to* that rule), so
spec-bindings serves it like an open task's. The honest caveat: long-lived
plan files accumulate, and a stale completed binding is historical noise at
edit time — retire individual claims via `fathom claim refute`, or prune
old plans out of the corpus with a project-local override's
`corpus.exclude`.

## The gate

Every layer earns its place in the embedded set the same way
(`docs/design/semantic-layers.md` §4.6): labeled fixtures under its own
`eval/` (governing-doc excerpts + `labels.json`: probes, and per-rule
`cite_substr`/`governs`/`trap_must_not_govern`), scored by
`internal/semantic/eval_test.go`'s shared `runLayerEval` harness against a
**real model** — spends tokens, so it only runs when asked:

```sh
# metered Anthropic API key:
FATHOM_EVAL_LLM=1 ANTHROPIC_API_KEY=... go test ./internal/semantic -run TestADRRulesEval -v

# subscription lane, no API key (a logged-in Claude Code CLI):
FATHOM_EVAL_LLM=1 FATHOM_LLM_PROVIDER=claude-cli go test ./internal/semantic -run TestSpecBindingsEval -v
```

The fixture set is EXPLICIT — `labels.json`'s `"docs"` list, never a glob —
so `RESULTS.md` and other non-fixture files in `eval/` can never leak into
the corpus. On the `claude-cli` lane the harness pins the model to the
**production compile default** (`claude-haiku-4-5-20251001`,
`compileModel()`'s default) unless `FATHOM_EVAL_MODEL` overrides it: an
eval gated under an unrecorded, floating model doesn't transfer to the
fleet-compile lane. Every `RESULTS.md` entry records model + lane + date +
command.

**Bar:** precision ≥ 0.90 on (claim, probe) application pairs (every-edit
exposure makes false positives trust-fatal — precision leads), recall ≥
0.75 on labeled rules. A pack that clears it records the run in its own
`eval/RESULTS.md`; a pack that doesn't, after honest prompt iteration,
records the failing metrics in `KILL.md` and stays OUT of the embedded set
— a rejected pack still exists as installable data (`fathom layers install
<dir>`), it just isn't shipped in the binary.

After shipping, every layer enters a 3-week generalize-or-kill telemetry
window (same §4.6, each pack's own `KILL.md` records the decision date):
call-rate from `mcp-calls.jsonl`/hook telemetry, plus a quality/outcome
signal specific to the layer.

## The `fathom layers` verb

```sh
fathom layers list    [--root DIR] [--store PATH --service NAME]
fathom layers show    <name> [--root DIR]
fathom layers install <dir>  [--root DIR] [--force]
```

Store-direct, like `fathom claim`'s v0 convention — no daemon round-trip.
`--root` defaults to the current directory. `list` unions embedded packs
with `<root>/.fathom/layers/`, marking an override with `*`, and always
shows an **EVAL** column (`pass` / `not-gated`, from each pack's own
`layer.cue` — schema data, never hand-maintained prose, see "The catalog"
above) and a **STRATUM** column (`concern` / `platform`, ADR-024 — see "The
catalog" above and `docs/design/pack-taxonomy.md`). Add `--store`/`--service`
for the **PRODUCTION** columns — CLAIMS
(compiled-claims-per-layer) and LAST_COMPILE (the layer's newest
`compile_runs.ran_at`, `never` if none) — both A4-1's live-from-the-store
truth (docs/plans/activation.md), plus the orphaned-layers section (claims
served from the store whose pack is gone — see the honesty notes above).
Broken project packs are listed with their load error, never silently
dropped. `show` prints the full resolved definition plus its stratum, eval
status, and prompt provenance (the content-addressed compile identity).
`install` validates a candidate pack directory against `#Layer` (a missing
`prompt.md` or empty/absent `eval/` is a warning, not a failure — the layer
just hasn't cleared its gate yet), stages the copy, and atomically renames
it into `<root>/.fathom/layers/<name>/` — a mid-copy failure leaves no
half-written pack behind. Reinstalling an existing pack is refused without
`--force`; `--force` replaces the whole directory (never merges, so stale
files from the previous version don't survive). Users never hand-author
CUE: install copies a directory; the shipped packs are authored once, under
the gate, here.

Compiling a project-local pack works exactly like an embedded one —
`fathom compile --service <svc> --layer <name>` resolves through the same
union, rooted at the service's own ingested path
(`pkg/client.CompileLayer`).
