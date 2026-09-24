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
| `obligations` | `obligation` | `doc-claimed` | `concern` | `CLAUDE.md`, `ADR.md`, `docs/plans/README.md`, `**/CLAUDE.md` | no | pass — ADR-016 baseline (see `eval/RESULTS.md`; a later production-model recheck found recall unstable around the bar, 0.71/0.86 — escalated as a pre-OP-6 input, not a gate failure) | `fathom layers list --store PATH --service NAME` |
| `findings` | `finding` | `sme-stated` | `concern` | `findings/**` | **yes** | **not-gated** — `eval/{labels.json,corpus.md}` now exist (docs/design/findings-as-evidence.md §6 item 5: 6 labeled findings across 6 classes + 2 traps), but `TestFindingsEval_Precision` has not yet been run against a real model (see `eval/RESULTS.md`); `fathom compile` still refuses this layer without `--unsafe-skip-eval-gate`, and no derived pattern (`internal/analysis/findings.PromoteTier`) may serve at a promoted tier until it does | `fathom layers list --store PATH --service NAME` |
| `adr-rules` | `adr-rule` | `doc-claimed` | `concern` | `ADR.md`, `docs/design/*.md` | no | pass — precision 1.00 / recall 1.00, 7 rules + 2 traps, production model pinned | `fathom layers list --store PATH --service NAME` |
| `spec-bindings` | `spec-binding` | `doc-claimed` | `concern` | `docs/plans/**` excl. `README.md` | no | pass — precision 1.00 / recall 1.00, 6 rules + 2 traps, production model pinned | `fathom layers list --store PATH --service NAME` |
| `certifications` | `certification` | `doc-claimed` → `pre-scan` → `verified` | `concern` | `// certifies:` markers, `tests/*.cue` manifests, attested test-kit runs | swept on every ingest | n/a — compiled deterministically, no model (ADR-043) | `fathom certify status\|gaps\|manifest` |

### `obligations` — every edit

**When to use:** always on, every fleet repo. The v1 tracer bullet (ADR-016
§4.8 #1): specs/standards/decisions governing the file about to be edited,
injected by the PostToolUse edit-hook.

**Corpus:** binding documents only — `CLAUDE.md`, `ADR.md`,
`docs/plans/README.md`, and **every nested instruction file** (`**/CLAUDE.md`,
excluding `third_party/**`, `.claude/worktrees/**`, `workspace/**`,
`node_modules/**`). Design essays are deliberately excluded (they produced
noisy aspirational rules on the first fleet compile). `AGENTS.md` is excluded
too: fathom writes the same stanza into both files, so including it compiles a
byte-duplicate of text `CLAUDE.md` already contributes — a repo whose
`AGENTS.md` carries rules its `CLAUDE.md` does not wants a project-local pack
override, not a default.

**Directory scope floor (BP-32).** Nested instruction files joined the corpus
because of O23: `internal/CLAUDE.md`'s "adding an extractor: register it in
`fusion.New` and add a golden test" was never compiled, so an edit to
`internal/extractor/goextract.go` was served six broad root-corpus rules with
the precise one absent. A nested file's rules only bind its own subtree, so
`internal/semantic`'s `scopeFloor` enforces that deterministically at compile
time, with no second LLM call: for a claim compiled from `<dir>/CLAUDE.md`,
a glob already under `<dir>/` passes through; a wildcard-leading or bare-file
glob (`**/*.go`, `goextract.go`) is prefixed with `<dir>/`; and a glob naming
a different literal directory (`cmd/**`) is a **compile error named per
claim** (`scope-out-of-subtree`) — never silently widened, never silently
re-homed. Root `CLAUDE.md`/`ADR.md` claims are unaffected.

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

**Findings as evidence (docs/design/findings-as-evidence.md):** every
finding-kind claim carries a closed-vocabulary `class` (correctness,
contract-violation, resource-leak, error-handling, concurrency, security,
config-drift, test-gap, performance, conformance) assigned at emit time —
by the reviewer prompt for the LLM-compiled corpus, deterministically by
`internal/analysis/review`'s dimension functions for the review-verb bridge
(`arch_conformance` always `conformance`, `spec_coverage` always
`test-gap`). `internal/analysis/findings.DerivePatterns` groups findings
sharing (class, normalized rule shape) across ≥2 sites into a computed,
never-persisted **pattern**; a pattern with ≥3 members carrying independent,
recorded code-change resolutions may rise exactly one tier
(`PromoteTier`) — but ONLY once this layer passes its eval gate (still not
today; see the catalog row above). `fathom findings near <symbol|file>` and
`fathom findings status` are the read-side verbs — see
`docs/use-cases/incident-investigation.md`.

**External producers (docs/vision/30-findings-loop.md, ADR-040).** A finding
on this layer need not originate from `fathom compile`: `fathom findings
import` projects a review kit's own portable journal into the same store,
under a distinct `PromptVersion` — `external-import/v1` — that keeps native
review reconciliation (`StaleReviewClaims`, `review-verb/v1`) and the
importer's own reconciliation from ever touching each other's claims.
Every producer (`fathom.review`, `reviewkit.system-review`,
`reviewkit.security-review`, `human.review`, …) is registered in a catalog
(`schema/semantic/producers.cue`) with a declared `tier_cap`: the served
tier is `min(event tier, producer cap)` — an imported claim's tier can
never RISE on import, whatever confidence the source event claims. A
`fathom`-origin producer named for import is refused outright (`circular:
native finding already exists as <claim_id>`) — authority never flows
backwards from a review-kit-imported journal entry to a native finding, or
from a native finding re-imported as if it were independent corroboration.
`docs/adoption/findings-loop.md` is the end-to-end guide. The cap, the
fathom-origin refusal and the independent-corroboration check all delegate
to agentic-kit's vendored `evidence-producer` catalog (`internal/semantic.
CatalogFromKit`, SU-25, ADR-041 decision 8) rather than reimplementing tier
arithmetic locally.

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

## Temporal validity and supersession (ADR-041 decision 7)

Every `SemanticClaim` carries a bi-temporal validity window beside its
freshness `status`: `valid_from` (stamped once, at first mint — a compiled
claim's `compiled_at`, an imported one's observation time) and `valid_to`
(the zero time while the claim is a live assertion, closed to the event
time by every path that ends a claim's validity — a native review/compile
reconciliation superseding it, an external closure/revocation, or a human
`fathom claim refute`). This answers a DIFFERENT question than `status`:
"was this rule true then", independent of what the claim's CURRENT status
says now.

`obligations`' `as_of` argument (and `findings near --as-of`) accepts an
RFC3339 timestamp or a commit sha (resolved against the service's own
indexed commits) and serves the claims whose window contains it —
including a claim since superseded or refuted, if `as_of` falls before that
event. The response echoes the resolved instant back as `as_of`; omitted,
serving is exactly the live path, unchanged.

Every claim a real successor replaces also carries an explicit `supersedes`
claim link (`rel:"supersedes"`, `TargetRef` = the predecessor's claim id) —
written wherever a native review reconciliation supersedes a prior finding
with a re-emitted one for the same subject. `SupersessionChain` walks that
link back through however many times the same subject has been replaced —
predecessor, its own predecessor, and so on — so a claim's full lineage is
one call away, not a guess from `claim_events` alone.

**Symbol-bound anchors (SU-21).** When a claim's citation resolves to
exactly one symbol definition in the index — the enclosing declaration, not
a guess across an ambiguous or absent match — it also gets an `anchors`
claim link (`TargetRef` = the symbol's `core.SymbolKey`, `TargetHash` = the
symbol's own current content fingerprint). `Sweep` reads it as a SECOND
healing path beside the literal-text re-anchor (T5-1/O13): when a cites
hash no longer matches and the literal text cannot be found (or the link
predates it), but the anchored symbol's own body hash is UNCHANGED at a new
range in the current index — a pure move, never a content edit passed off
as one — the citation is re-anchored there instead of flipping stale.
`rename_impact` lists any claim anchored to the renamed symbol
(`claims_anchored`/`claims_anchored_ids`), so a rename is never silent for
governance.

**Provenance (SU-22).** Every claim and event records the activity that
asserted it, `core.AssertedBy(kind, id)`'s canonical `"kind:id"` form: a
compile run's own `run.RunID` (`"compile:<run_id>"`), a review run's
`--idempotency-key` or its corpus watermark (`"review:<key-or-watermark>"`),
an import's `producer@version` (`"import:<producer>@<version>"`), or
`fathom claim add`'s harness hook session id (`"claim-add:<session>"`) when
one is resolvable. Served as `asserted_by` on each obligations rule
(omitted when the underlying claim never recorded one — every pre-SU-22
claim, and any writer this wave did not touch).

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
