# Pack taxonomy — the four strata

*Status: adopted 2026-07-23 (ADR-024). Vocabulary, not machinery: this
document names and organizes distinctions the codebase already draws
(embedded vs `.fathom/` packs, `evaled` self-declaration vs live
`eval/RESULTS.md`, dialect detection vs layer compilation) — it does not
introduce a new subsystem. See `docs/design/semantic-layers.md` (the layer
compiler + serving design) and `docs/layers.md` (the shipped catalog) for
the machinery this taxonomy is vocabulary *for*.*

## Why a taxonomy

Fathom has, until now, used "pack" loosely for anything data-shaped and
pluggable: a semantic layer (`obligations`), a language extractor, a
project's own `.fathom/rules/*.cue`. That looseness was fine while there was
one pack *kind* (semantic layers) and one pack *axis* (embedded vs
project-installed). It stops being fine once a design conversation needs to
ask "who is allowed to author this, and what makes it trustworthy?" for
four genuinely different answers. The taxonomy below names those four
answers so future design docs, ADRs, and code comments can say "concern
pack" or "platform pack" instead of re-deriving the distinction every time.

## The four strata

| Stratum | Fathom realization today | Authored by | Trusted via |
|---|---|---|---|
| **dialect** | extractors + tool identity (data run by fathom's engines since ADR-048; not a pack) | fathom | golden tests, deterministic, a calibration reading |
| **provider** | does not exist — parked pack type (TX-3) | fathom or vendor | eval-gated, staleness vs. an UPSTREAM |
| **platform** | `.fathom/` project packs (an org's own obligations/rules/bindings) | the org | compiled from an org corpus; on-disk eval evidence required (D3) |
| **concern** | the layer catalog + rule packs + analyzers | fathom + org extensions | eval-gated embedded; entry-conditioned growth |

### Dialect — what fathom already understands as code

**What it is.** The knowledge of how to parse a language or tool format at
all: Go via `go/parser`, Python/JS/Java/Ruby/HCL/k8s/CUE/Markdown/go.mod via
deterministic heuristics (CLAUDE.md's "Architecture" section,
`internal/extractor`), tree-sitter grammars where wired (ADR-013/ADR-021).

**Who authors it.** Fathom. A dialect is not installable data — it is a Go
package implementing `extractor.Extractor`, registered by file extension in
`fusion.New()` (CLAUDE.md, "Adding things → Extractor").

*Refined by ADR-048 (2026-09-27).* A dialect is now a directory of data — a manifest,
tree-sitter queries or document selectors, provenance and goldens — run by one of two
engines (`docs/design/language-breadth.md` §2–§4). It is still authored by fathom and
trusted by goldens plus a calibration reading, and it is still not a pack in this
taxonomy's sense. The one installable form is a checksum-pinned **dialect pack** at user
scope (the same directory plus its grammar blob), for the long tail past the embedded
budget and for an org's private formats (ADR-011: org-specific is configuration).

**How it's trusted.** Golden-fixture tests against `testdata/fauxtown/`
(ADR-012) and the calibration harness (`calibration/`, ADR-023) — recall
measured per language against a proxy oracle. Deterministic: the same bytes
always extract to the same symbols.

**Why it is not a "pack."** A dialect has no corpus to compile, no LLM pass,
no eval-gate-before-ship story distinct from "the extractor's tests pass."
It is the substrate every other stratum's corpus selection (`corpus.include`
globs) assumes already works. Naming it here is deliberately for contrast:
when someone asks "can I add a pack for Rust," the honest answer is "you
add a dialect (an extractor), not a pack" — the taxonomy exists partly to
make that redirect fast.

### Provider — parked

*Promoted 2026-10-09 (owner decision D11, ADR-061 proposed; `docs/design/fathom-dir.md` §7).* The trigger below fired: fathom is the named consumer, app-kit and env-kit the providers. The stratum is renamed **tech**. A tech pack is deterministic data (`guards`, `generated`, `formats`, fixtures) read from the repository's own pinned copy of the kit, so it needs no eval gate and no upstream watermark: freshness is the pin. The text below is the parked record.

**What it would be.** A pack type explaining an *external technology's*
resource semantics to fathom — e.g. what an infrastructure-as-code
provider's resource graph means, or how a specific cloud API's rate limits
shape blast-radius analysis — content that is neither "how to parse this
file" (dialect) nor
"what this org's own rules are" (platform) but "what this third-party
system means," authored either by fathom or by a vendor.

**Trusted via.** Eval-gated like a concern pack, but with a second axis
no other stratum needs: **staleness against an UPSTREAM**. A provider pack
describes something that changes on someone else's schedule (a cloud API
version, a tool's config schema) — the pack's corpus doesn't just risk
going stale relative to *this repo's* churn (the watermark/staleness
machinery every layer already has, ADR-016 §4.3), it risks going stale
relative to the *provider's own release cadence*, which fathom's git-HEAD
watermark cannot see. This is the first pack type whose freshness story
needs an upstream version marker, not just a `compiled_at_commit`.

**Status: does not exist (TX-3, deferred).** No consumer has named a
provider whose semantics fathom must explain. Building the eval-gate and
staleness machinery ahead of a named need is the kit's own documented
failure mode (`docs/vision/`'s admission-rule reasoning, and
`docs/design/semantic-layers.md`'s citation of coverage-first ontologies
dying before delivering). **Trigger:** a named consumer plus one concrete
provider whose resource semantics an indexed service actually needs.

### Platform — an org's own governance, compiled

**What it is.** A project's own `.fathom/layers/<name>/` pack (`docs/layers.md`,
LC-3/LC-7): a directory an org authors and installs — its own obligations
corpus, its own architectural-constraint rules, its own spec-binding
extraction — compiled by the same engine that compiles fathom's shipped
packs, but never shipped in the fathom binary.

**Who authors it.** The org. Not fathom, not a vendor — whoever owns the
adopting repo's `.fathom/` directory.

**How it's trusted.** This is where the taxonomy names a distinction the
code already enforces mechanically (`internal/semantic/packs.go`'s
`loadUnion`, "Eval-gate honesty for PROJECT packs"): an **embedded** pack's
`evaled: true` is self-declared data, trusted because fathom's own
authoring process ran the precision/recall gate before shipping and
`eval/` is deliberately excluded from the binary (there is nothing on disk
to re-check at runtime). A **project** pack's `evaled: true` is checked
LIVE against a readable `eval/RESULTS.md` on disk next to its `layer.cue` —
a one-line lie in the CUE file is overridden back to `false` the moment
there's no eval evidence to back it. This is D3's trust seam
(`docs/plans/expert-depth.md`), and it is exactly what makes a project pack
"platform" stratum rather than "concern" stratum: the eval evidence has to
be *on disk, in the org's own repo*, not baked into a vendor's binary.

**Formalizes ADR-011.** ADR-011 says organization-specific content is
configuration, never product, and fathom's source tree carries zero
org-specific names as a hard requirement. The platform stratum is that
principle given a stable name and a load-bearing trust mechanism: an org's
governance content lives in `.fathom/`, is authored and eval-gated by the
org, and never enters the fathom repo or binary.

### Concern — the shipped catalog

**What it is.** The layer catalog fathom ships embedded in its own binary —
`obligations`, `findings`, `adr-rules`, `spec-bindings` today
(`docs/layers.md`) — plus the deterministic analyzers (`internal/analysis/*`)
and rule packs (`.fathom/rules/*.cue`, `internal/analysis/archconformance`)
that ship the same way. "Concern" names the thing these packs are *about*:
a cross-cutting engineering concern (what am I obliged to do here, what
architectural rule governs this package, what does this finding still owe)
rather than a technology (dialect) or an org's private governance
(platform).

**Who authors it.** Fathom, primarily — with room for "+ org extensions":
an org can contribute a *generically useful* concern pack back (e.g. a
well-architected review pack any repo could use), which is still concern
stratum because it is eval-gated and admitted the same way fathom's own
packs are, not because of who typed the prompt.

**How it's trusted.** Eval-gated before it enters the embedded set
(`docs/layers.md` "The gate": precision ≥ 0.90, recall ≥ 0.75 against
labeled fixtures with deliberate scope-traps, scored against a real model)
— with one honest exception on the record: `findings` shipped embedded
**not-gated** (no `eval/` yet; its `layer.cue` declares `evaled: false`,
the catalog prints `not-gated`, and `fathom compile` refuses it without
`--unsafe-skip-eval-gate`), admitted entry-conditioned on its own
`KILL.md` kill criterion instead of an eval pass. So the precise trust
statement is: an embedded concern pack is either eval-gated or visibly,
mechanically marked as not — never silently presumed gated. After
shipping, **entry-conditioned growth** — the 3-week generalize-or-kill
telemetry window (ADR-016 §4.6), and for the catalog overall, the playbook
pack's documented "≥15 findings" kill criterion is the model every future
concern pack's admission should cite (see TX-4 below).

### The obligations call — a documented judgment, not a default

The plan that adopted this taxonomy asked, explicitly, whether `obligations`
— whose corpus is `CLAUDE.md`, `ADR.md`, `docs/plans/README.md` — reads as
**platform** rather than **concern**, since those documents are about as
"this org's own governance" as content gets. The call: **`obligations`
stays concern stratum, the same as `findings`, `adr-rules`, and
`spec-bindings`.**

**Reasoning.** Stratum answers "who authored and eval-gated *this pack
instance* (its `layer.cue`, `prompt.md`, corpus selector, and
`eval/RESULTS.md`)?" — not "whose content does the compiled corpus happen
to contain?" Every one of the four shipped packs compiles per-repo content
by construction (`corpus.include` globs are always repo-relative) —
`adr-rules` compiles *this* repo's own `ADR.md` and `docs/design/*.md` just
as directly as `obligations` compiles *this* repo's `CLAUDE.md`. If
"the corpus is org-specific content" were sufficient to make a pack
platform-stratum, all four shipped packs would qualify, and the stratum
field would collapse into a synonym for "layer" — which is exactly the
"vocabulary, not machinery" design constraint this document opened with
warning against. What actually distinguishes platform from concern is
*authorship and eval-gate location*: `obligations`'s prompt, corpus-selector
pattern, and eval fixtures are authored once by fathom, ship in the binary
identical for every adopting repo, and were gated before shipping — the
same as the other three. An org that wants its OWN obligations-shaped pack,
compiled from a corpus fathom's shipped selector doesn't reach and
eval-gated on-disk under D3's live-check seam, installs a *project* pack
under `.fathom/layers/obligations/` (overriding the embedded one, per the
existing override rule) — and *that* pack instance is platform stratum,
correctly, because now the org authored the prompt and owns the eval
evidence. The stratum field is set per pack *instance*, and the embedded
`obligations` instance and a hypothetical project override of the same name
can legitimately carry different stratum values.

## Three key insights

1. **The premade-vs-custom axis IS the embedded-vs-`.fathom/` union, with
   the D3 trust seam riding on top of it.** Fathom already has exactly one
   pack-resolution mechanism (`internal/semantic.LoadLayerIn`'s union of
   embedded packs and `<root>/.fathom/layers/`, a project pack overriding an
   embedded one of the same name). Concern-vs-platform is not a second
   mechanism — it is a name for which side of that existing union a given
   pack instance resolved from, plus the trust-seam consequence
   (self-declared `evaled` vs. live-checked `evaled`) that side already
   carries.
2. **Provider packs are the first pack type whose corpus drifts against an
   UPSTREAM, not just against this repo.** Every mechanism fathom has today
   for pack freshness — the post-ingest citation sweep, the
   `compiled_at_commit` watermark, `staleness_report`'s layers section
   (ADR-016 §4.3) — answers "has *this repo* changed since the pack last
   compiled?" A provider pack's truth can go stale even when this repo's
   git HEAD hasn't moved at all, because the thing it describes (a vendor's
   API, a tool's schema) changed on its own release schedule. This is why
   provider packs are parked rather than half-built: the freshness
   machinery they need is a genuinely new watermark kind, not a
   configuration of the existing one, and building it before a named
   provider exists would be speculative.
3. **The platform stratum formalizes ADR-011.** "No organization-specific
   names anywhere; anything org-specific is configuration" was, until now,
   an org-agnostics rule about the fathom repo's own source tree. Naming
   the platform stratum makes it a rule about the *product*: fathom's
   extensibility story for an org's private governance is not "fathom adds
   a special case for you," it is "you author a platform pack under
   `.fathom/`," which is the existing project-pack mechanism, now named for
   what it structurally is.

## The two parked pack types (recorded, not built)

Both are recorded here and in ADR-024 with explicit triggers — the point of
naming them now is so the next time either need arises, the taxonomy
already has a slot for it, rather than reopening the "what kind of pack is
this?" design question from scratch.

- **Provider pack type (TX-3).** See "Provider — parked" above.
  **Trigger:** a named consumer plus one concrete provider whose resource
  semantics an indexed service actually needs.
- **Practice/concern pack library growth beyond the shipped catalog
  (TX-4).** Growing the concern-stratum catalog past `obligations` /
  `findings` / `adr-rules` / `spec-bindings` — e.g. a well-architected
  review pack, a compliance pack — one pack at a time, each gated the same
  way the shipped four were. **Trigger, per pack:** a real consumer with a
  real corpus, eval fixtures, and a kill criterion — the playbook pack's
  documented "≥15 findings" condition (`docs/layers.md`'s gate section, the
  entry-conditioned-growth clause of ADR-016 §4.6) is the model every new
  concern pack's admission case should cite. `adr-rules` sat eval-perfect
  with zero production claims for weeks before this taxonomy was written —
  a pack built ahead of activation is theater, not progress, and TX-4 exists
  precisely to name that failure mode so it isn't repeated.

## What this taxonomy does not change

No new Go subsystem, no new CUE definition beyond the single additive
`#Layer.stratum` field (`schema/semantic/schema.cue`, TX-2), no change to
`LoadLayerIn`'s resolution order, no new trust mechanism beyond the one
`internal/semantic/packs.go` already implements for the D3 seam. This
document is the vocabulary that makes those existing mechanisms nameable in
design conversations — nothing here should ever be cited as justifying a
new runtime.
