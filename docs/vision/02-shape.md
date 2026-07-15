# 02 — Product shape: kernel, module packs, satellites

**Verdict.** One repo, one binary, forever. Well-architected reviews, code
reviews, investigations, and compliance are **not** separate tools and
**not** permanent Go packages — they are **expert-module packs**:
directories of data (CUE layer manifest, versioned prompt, corpus globs,
eval fixtures, kill gate) compiled by the one fathom engine into the
existing governed-claims stratum. The extensibility mechanism is
**CUE-manifest data-defined layers**; the "CUE cataloging companion" idea
collapses into making the manifest you already have load-bearing. Reject
buildmode=plugin outright; defer Go SPI promotion and sidecar protocols
until a named external consumer exists; satellites (chat apps, hosted
console) stay entry-conditioned per H3.

## 1. The boundary, concretely

**Kernel — in the fathom repo forever:**
- `internal/{connector,fusion,extractor,filter,store,query,vector}` — ingest → index → query.
- `internal/semantic` — compiler, sweep, serving; generalized to run any *loaded* layer manifest.
- `internal/verify` + the gate CLI — the headline product.
- `internal/core` — the Observation + SemanticClaim domain model.
- `internal/mcp` + `cmd/fathom` — the delivery surfaces, plus hooks.
- `internal/analysis/*` (pruned) — deterministic producers, callable by tools and by module compiles.
- The **portable store format spec** (H2) — the artifact third parties eventually target.

**Module pack — data, versioned, installable:**

```
layers/<name>/
  layer.cue        # #Layer manifest: kind, corpus globs, tier, budgets, serving caps
  prompt.md        # the compile prompt; content-addressed (see §3 M1)
  producers.cue    # optional: deterministic analyzers to run + their policy/rules
  eval/            # seeded fixtures + scope traps; precision/recall thresholds
  KILL.md          # the kill gate: metric, window, decision date
```

In-tree packs: `obligations` first (migrate the hardcoded layer here), then
layer 2 (see [06 §2](06-use-cases.md) for the candidate decision). Beyond
those two, `change-review`, `well-architected`, and `compliance` are
*candidates*, each subject to the §5 admission rule and its own entry
condition (per [06 §1](06-use-cases.md), compliance is a bounded probe and
change-review is the layer-2 fallback — neither is a committed build). A
pack needing private/domain corpora lives outside the repo and installs by
path — same shape.

**Satellite apps (separate repos, H3 entry-conditioned):** incident command
center, learning platform, hosted connected-experts console. All consume
`pkg/client` or `POST /rpc/{surface}` + portable stores. None starts before
the H3 entry conditions (≥2 external module consumers, or ≥100 installs +
≥60% internal share for 4 weeks).

Why this is nearly forced rather than a judgment call:
- `TestExpertModule_NonGitCorpus_EndToEnd` already proved "a module is
  configuration of the existing engine" — the only missing piece is a
  product surface for that configuration.
- Separate repos are pure overhead for a solo dev with zero external users
  (the agentic-kit go.work decoupling is itself an unstarted H1 item).
- Baking each review in as a permanent Go package repeats the analysis
  layer's measured mistake: 37 packages, 6 fully dead, ~50 MCP tools with
  zero calls in 8 days.

## 2. What happens to the 37 analysis packages

Keep them as pure `internal/analysis` libraries; their productization path
is **through packs**: a manifest's `producers.cue` declares deterministic
analyzers whose outputs persist as `tier: verified`, `Kind:"finding"`
claims beside the LLM-compiled `doc-claimed` ones. That single move fixes
four measured gaps at once — no persistence, no baselines, no
suppression/triage, no stored policy (arch rules and license allow-lists
move from per-call MCP args into the pack manifest).

Prune now (evidence in): delete the 6 never-wired packages (`apichangelog`,
`crossrepocycles`, `dataflow`, `ineffassign`, `packagedeps`; wire `churn`
into `hotspots` first — the tool's own description apologizes for churn's
absence while the package sits unused). Hold `internal/adr` and
`internal/ownership` as explicit pack dependencies for the well-architected
and compliance packs — or delete them too if those packs miss their entry
windows. Tombstone dormant `internal/llm` enrichment (see [01 §2](01-thesis.md)).

## 3. The extensibility mechanism (your plugin-system question)

Five candidates, evaluated honestly; **CUE-manifest data-defined layers
chosen** as the one primary mechanism:

1. **CUE manifests (data) — CHOSEN.** `schema/semantic/schema.cue #Layer`
   already encodes exactly what varied between obligations and the
   expert-module test: name, kind, corpus globs, prompt_version, tier,
   budgets, serving caps. Third parties extend fathom by writing a
   directory, not linking code — which matches who the extenders actually
   are, and keeps the compile-time verification gates (citation hash,
   scope-trap defense, word budgets) as *engine* guarantees no extension
   can bypass. Stated limits: a manifest cannot define a new deterministic
   producer or language extractor — those need Go, and that's acceptable.
2. **In-process Go SPI (promote `internal/` → `pkg/spi`) — evidence-gated,
   not yet.** Every SPI is `internal/` today, so "third parties extend
   fathom" is literally false at the Go level — but promoting now freezes
   interfaces with zero external users. Gate: promote the first interface
   when the first named external Go consumer appears; start with
   `extractor.Extractor` (smallest, most demanded), never the monolithic
   24-method `store.Store` (split it first).
3. **Subprocess/JSON sidecar — the designated escape hatch, not now.**
   Right if a non-Go party ever needs a custom deterministic producer;
   until then it's speculative protocol surface.
4. **MCP composition — rejected as an extension mechanism.** Composing MCP
   servers aggregates tools; it cannot put claims into the governed store
   with provenance, sweep, and portability — the whole point. MCP stays the
   consumption surface.
5. **Go buildmode=plugin — rejected flatly.** Exact-toolchain matching,
   hostile to CGO+DuckDB on macOS, no Windows, breaks single-binary
   distribution.

**Migration path (each step shippable alone):**
- **M1 (soon — it removes a live defect class):** `go:embed` and actually
  *load* `schema/semantic/schema.cue`; delete the hand-synced Go mirrors
  (`ObligationsInclude`, `systemPrompt`, the `PromptVersion` constant).
  Move the prompt to a file the manifest references and compute
  `PromptVersion` from a hash of the **full effective prompt** (system
  scaffolding + section template + prompt file — hashing only the prompt
  file would recreate the stale-identity bug when the Go scaffolding
  changes). A prompt edit then automatically becomes a new compile
  identity, killing the v1→v2 digest-gate bug class.
  **Be honest about the cost: this is a flag-day recompile.** The digest
  gate checks `PromptVersion` per claim, so the identity change invalidates
  every claim compiled under `obligations/v2` — the next compile redoes the
  whole fleet. Sequence it so the fleet pays once: fold the identity change
  into the serve-time precision pass ([04 Gap 3](04-coding-agents.md)),
  which bumps the prompt anyway. Do **not** land M1 between a fleet compile
  and its measurement window.
  **Enforce the CUE run-level budgets** — `max_tokens_per_run`,
  `max_usd_per_run`, `max_claims_per_file` are enforced by nothing today
  (the per-rule word cap and serving caps *are* enforced, but as hardcoded
  Go constants rather than loaded CUE values). Define abort semantics:
  a budget abort is resumable, because already-compiled sections pass the
  digest gate on the next run.
  **Widen `#Layer.tier` in the same change**: the schema currently allows
  only `doc-claimed | sme-stated`, which forbids the `verified` tier that
  deterministic producers need ([03 §3](03-findings-and-experts.md)).
  Admitting `verified` for `Model:""` deterministic compile paths amends
  ADR-016's tier-gating rationale — deterministic verification is exactly
  the revival evidence it asked for. Do it with M1 so `cue vet` and the
  loaded manifest never disagree.
  Unlock `compile_layer`/`fathom compile` to accept any layer *present in
  the loaded manifest* — with one definition shipped this changes no
  behavior beyond the recompile above, but layer 2 becomes configuration.
- **M2 (H2, with layer 2):** serving-template vocabulary (obligation-style
  rules vs finding-style entries); `fathom layer install <dir>`
  (copy + validate + record); project-local layers via `.fathom/layers/`.
  Layer 2 ships as the proof that a new layer needs no Go changes —
  ADR-016's revival evidence, met by construction.
- **M3 (H2/H3, demand-triggered):** `producers.cue` deterministic
  producers writing `Kind:"finding"` claims; `#Module` schema +
  `fathom pack`/`unpack` (tarball of the layer dir + optional compiled
  `.duckdb`); `pkg/spi` promotion on first external Go consumer.

## 4. The CUE catalog question

**Don't build a companion project.** What a catalog would actually catalog
is layer definitions and module packs — everything else is premature
(findings taxonomies, policies: blocked on `Kind:"finding"` existing) or
counterproductive to formalize (tool surfaces need pruning, not a manifest
that ossifies them).

What the ecosystems teach: OPA won by defining a dead-simple **bundle
format** early and letting registries grow around it — it never built a
catalog service. Timoni proves CUE-native packaging mechanics work and
also that a CUE-native ecosystem is a niche within a niche — CUE should
*validate*, directories/tarballs should *distribute*. Successful
config-artifact ecosystems ship in this order: (1) schema + local loading,
(2) a pack format that is "a directory you can tar," (3) OCI/registry
distribution, (4) catalog/search. Fathom is at step 0.5 — schema, no
loading. The "cataloging system" is step 4; skipping to it inverts the
order that works. M1 is step 1; M3's `fathom pack` is step 2; stop there
until H3 conditions fire, and even then start with "a git repo of packs."

Kill criterion, to stay honest: if layer 2 dies at its own kill gate, the
manifest loader stays (it's now just how obligations is configured, and it
removed a bug class), but all `#Module`/pack work stops.

## 5. Bloat discipline — the admission rule

> **A capability enters the kernel only with a wired delivery channel
> (gate, hook, or documented use-case surface) AND survives a 30-day
> call-log kill gate. Everything else enters as a module pack. Module
> packs carry their own KILL.md.**

And the sequencing rule from [03 §5](03-findings-and-experts.md), restated
because it governs this doc too: nothing graduates from contract to code
without a named consumer or a passed gate — spec → library → CLI/MCP verb →
surface → service, one rung at a time.
