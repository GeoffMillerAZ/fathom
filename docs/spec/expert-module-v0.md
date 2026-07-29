# Expert Module v0

*Status: v0, experimental-but-append-only (see `README.md`'s stability
policy). Normative on what a conforming "expert module" is and what a
conforming manifest, server, and portability story must provide. Builds
directly on `store-format-v0.md` — a module is a conforming store plus one
additional artifact (the manifest); this document does not repeat the store
format's requirements, it adds to them. Design context:
`docs/design/expert-modules.md` (the use-case spec this formalizes) and
`docs/design/semantic-layers.md` §5 (commitments D-EM1–D-EM5).*

**Correction inherited from `store-format-v0.md`:** every `.duckdb`
reference in `docs/design/semantic-layers.md` §5 and
`docs/design/expert-modules.md`'s original body predates ADR-025 (SS-8);
the module store is SQLite. `docs/design/expert-modules.md` already carries
a 2026-07-23 correction note to this effect — this spec is written against
the corrected (SQLite-only) reality throughout.

## 1. Definition

An **expert module** is: **one conforming store (`store-format-v0.md`) +
one module manifest.** Nothing else. There is no second engine, no second
schema, no "expert edition" build (D-EM2) — a module is fathom's ordinary
store format, packaged with a manifest that makes it self-describing to a
consumer who never talked to the machine it was compiled on.

```
module = one .sqlite store, conforming to store-format-v0.md, plus
         one module manifest (schema/module/schema.cue's #Module, §3)
         describing that store's corpus, watermark, tier profile, eval
         evidence, recompile rights, and provenance.
```

The manifest is metadata **about** the store, not a separate data format —
v0 does not specify where the manifest is persisted relative to the store
file (embedded as a row, shipped as a sidecar `.cue`/`.json` file, or
generated on demand from `service_meta` + `compile_runs` are all legitimate
v0 implementations); what is normative is the manifest's **field contract**
(§3), not its storage location. `internal/modspec` (the CUE schema's
companion Go package, `schema/module/schema.cue`) validates the shape; no
runtime loader ships in this repo yet (§7's honest gap).

## 2. What makes a module different from "a chatbot with a folder of PDFs"

Every claim a module serves carries (`store-format-v0.md` §3/§8, Server
conformance):

- **Why it should be trusted** — evidence tier (§7 of `store-format-v0.md`),
  citation to the corpus line (anchor rules, §5 of `store-format-v0.md`).
- **When it stopped being sure** — per-claim status: fresh / stale /
  contradicted / refuted, never silently dropped.
- **What it was compiled from** — corpus watermark, compile ledger
  (`compile_runs`), eval evidence (§3.4 below).

This is D-EM4 (`docs/design/semantic-layers.md` §5), restated as a
conformance requirement rather than a design goal: a module that cannot
answer these three questions for a served claim is not a conforming module,
regardless of how good its answers otherwise sound.

## 3. The module manifest

Normative shape: `schema/module/schema.cue`'s `#Module` definition, tested
by `internal/modspec/schema_test.go` (positive: `schema/module/testdata/
valid.cue`; negatives: `schema/module/testdata/invalid.cue`, asserted live by
the Go test per this repo's schema convention — CLAUDE.md "New CUE
schemas").

**Closedness is an authoring gate, not a reader-conformance instrument.**
`#Module` is a CUE *closed* struct — a manifest instance with an unrecognized
field name fails validation against it, which is deliberate: it catches a
typo'd or misspelled field at authoring time, the same way `#Layer` already
does for layer packs. That authoring-time strictness does **not** relax
`README.md`'s additive-within-v0 rule ("fields may be added without a
version bump"). To reconcile the two: **a conforming manifest READER MUST
ignore unknown fields it encounters in a manifest** — closedness is enforced
by the schema an AUTHOR validates a new manifest against (this repo's own
`#Module`, or a future v0.x revision of it with more fields added), never by
a downstream reader parsing an already-authored manifest. A third-party
module compiled under a slightly newer `#Module` revision than a given
reader knows about therefore still parses: the reader sees, and must
tolerate, fields it doesn't recognize — the same discipline
`store-format-v0.md` §1.4 already states for unknown store tables/columns,
restated here for the manifest.

### 3.1 Module identity

| Field | Type | Requirement |
|---|---|---|
| `name` | `string`, `^[a-z][a-z0-9-]*$` | **MUST** — stable identifier, mirrors `#Layer.name`'s convention (`schema/semantic/schema.cue`) |
| `version` | `string`, `^v[0-9]+$` | **MUST** — mirrors `#Layer.version`'s exact precedent: THIS MODULE DEFINITION's own version, part of its provenance (`schema/semantic/schema.cue`'s comment: "version of the layer definition; part of every claim's provenance") — bumped when the module's own corpus selection, prompt, or compile identity changes meaningfully. **Not** the `#Module` CUE schema's own shape version (there is no separate field for that in v0, matching `#Layer`, which likewise carries no schema-shape-version field of its own). Both `schema/module/testdata/valid.cue` examples set `version: "v1"` as each module's own first version — a third-party author copying either example starts their own module at `"v1"` too, not at whatever `#Module`'s current CUE-schema revision happens to be. |
| `description` | `string` | **MUST** — human-readable, no format constraint |

### 3.2 Corpus selectors

| Field | Type | Requirement |
|---|---|---|
| `corpus.include` | `[...string]`, non-empty | **MUST** — repo-relative gitignore-style globs, the exact shape `#Layer.corpus.include` already uses (`schema/semantic/schema.cue`) |
| `corpus.exclude` | `[...string]` | MAY |

A module with an empty `corpus.include` is invalid — the same rule
`#Layer` already enforces (`schema/semantic/testdata/invalid.cue` case 1),
reused here rather than reinvented.

### 3.3 Watermark at compile time

| Field | Type | Requirement |
|---|---|---|
| `watermark` | `string`, 40-hex git SHA or `manifest:`+64-hex | **MUST** |

**The freshness doctrine (the load-bearing rule of this whole document): a
module without a watermark is invalid.** This is not a soft default — the
CUE schema's `watermark` field has no `*default` value and is not marked
optional (`?`); a manifest that omits it fails validation
(`schema/module/testdata/invalid.cue`'s missing-watermark case). The format
is exactly `store-format-v0.md` §6's `Connector.Watermark` contract: bare
git HEAD SHA for a git-backed corpus, `manifest:`-prefixed sha256 hex
otherwise. A served claim from a module whose watermark cannot be
established has nothing to be honest *relative to* — the same reasoning
that makes `store-format-v0.md` §6's two staleness mechanisms mandatory at
the store level, restated at the packaging level.

### 3.4 Tier profile

| Field | Type | Requirement |
|---|---|---|
| `tier_profile` | `string`, default `"engineering"` | **MUST** be present (defaulted); MAY name a non-`"engineering"` profile, with the same "data now, machinery later" caveat as `store-format-v0.md` §7 — v0 defines no interpretation for a second profile's values |

### 3.5 Eval-evidence pointer

| Field | Type | Requirement |
|---|---|---|
| `eval_evidence.path` | `string`, repo-relative | **MUST** when `eval_evidence.evaled = true` |
| `eval_evidence.evaled` | `bool`, default `false` | **MUST** be present |

This is the pack-taxonomy D3 requirement (`docs/design/pack-taxonomy.md`,
"How it's trusted" under Platform stratum) applied to a module rather than a
layer pack: a module shipped *outside* the fathom binary cannot rely on
"we ran the gate before shipping and trust our own binary" the way an
embedded concern-stratum layer can (`internal/semantic/packs.go`'s
`loadUnion` self-declared-vs-live-checked distinction). A module's
`evaled: true` claim **MUST** be checkable against an on-disk
`eval_evidence.path` — a manifest that claims `evaled: true` with an empty
or absent `path` is invalid (`schema/module/testdata/invalid.cue`).

### 3.6 Recompile rights

| Field | Type | Requirement |
|---|---|---|
| `recompile.allowed` | `bool`, default `true` | MUST be present |
| `recompile.prompt_identity` | `string` | **MUST** when `recompile.allowed = true` |

Recompiling a module means re-running its layer's compile pass over a
refreshed corpus. Doing so under a *different* prompt than the one that
produced the shipped claims silently breaks claim identity/reproducibility
(`store-format-v0.md` §9: `claim_id` reproducibility assumes an identical
prompt version) — so a manifest that permits recompilation **MUST** name
the exact prompt identity a compliant recompile is expected to use.
`recompile.allowed: false` (the closed case — e.g. a regulated-domain
snapshot never meant to be refreshed by its recipient) requires no prompt
identity, since no recompile is sanctioned at all.

### 3.7 License and provenance

| Field | Type | Requirement |
|---|---|---|
| `license` | `string`, default `"unspecified"` | MUST be present |
| `provenance.compiled_by` | `string` | **MUST** — a model ID, or the literal `"deterministic"` for an ADR-018 oracle-class (Model-empty) compile |
| `provenance.compiled_at` | `string`, RFC3339 | **MUST** |

`provenance.compiled_by = "deterministic"` mirrors `store-format-v0.md` §7's
`verified`-tier admission rule (ADR-016's M1 amendment): a module compiled
entirely by a deterministic, Model-empty pass names that explicitly rather
than leaving the field empty in a way indistinguishable from "unknown."

## 4. Serving contract

A conforming module server (§10 of `store-format-v0.md`'s Server
conformance level, specialized) **MUST**, at minimum, answer:

1. **Enumerate** — list the module's services/corpora and, for each, its
   current watermark and `service_meta.indexed_commit` (drift-detectable
   per `store-format-v0.md` §6).
2. **Look up a claim by scope** — the `obligations`-shaped query pattern
   (`docs/design/expert-modules.md` §3's "Queries" rows are all instances of
   this one shape: rule-by-scope, filtered by kind) — every returned claim
   payload carrying citation + tier + non-fresh status
   (`store-format-v0.md` §3's Server level).
3. **Report staleness** — `store-format-v0.md` §6's drift-detection MUST be
   answerable without a live corpus connection (comparing stored watermark
   fields), matching L1 reader conformance.

A module server **MAY** additionally expose the full read surface
(`query_symbols`, `find_symbol`, `references`, `outline`, `pack_context` —
the "expert surface" `docs/design/expert-modules.md` §6 names at
`POST /rpc/expert`), but the three obligations above are the floor: a
minimal conforming server that only answers scope-lookups and staleness is
still a conforming module server. `staleness_report`'s `layers` section
(`internal/mcp/meta.go`, backed by `ClaimsSummary`) is the reference
implementation's concrete instance of requirement 3.

## 5. Portability contract

**Compile on machine A, serve on machine B with citations intact and
staleness honest, with zero recompile.** This is `store-format-v0.md` §5's
anchor rules and §6's watermark rules, restated as the module-level
acceptance test:

1. Every anchor in the store resolves relative to `service_meta.path` —
   moving the module is one `fathom remap` call (`cmd/fathom/remap.go`),
   never a re-ingest (`store-format-v0.md` §5.2).
2. The watermark computed at the new location **MUST** equal the watermark
   computed at the old location for an unchanged corpus — a pure move
   **MUST NOT** read as drift (`store-format-v0.md` §6).
3. Citations **MUST** resolve at the new location with identical content —
   source fetch, not just symbol lookup, is part of the contract (a claim's
   `cites` link is only as trustworthy as the file it points at actually
   being fetchable post-move).

**Enforcement:** `TestStorePortability_RoundTrip`
(`pkg/client/portability_test.go`) is this contract's exact test — build at
location A (non-git corpus, exercising the `manifest:` watermark path),
move to location B, remap, and assert symbol lookup, source fetch, and
watermark equality all hold, then assert a subsequent re-ingest at B is
fully hash-gated (`report.Extracted == 0`) and does not trigger the
legacy-anchor self-heal (`report.PurgedLegacyAnchors == false`) — i.e. a
module built under this spec never LOOKS like a pre-ADR-017 legacy store to
its own reference implementation.

## 6. Lifecycle

One flow for every module (`docs/design/expert-modules.md` §5, restated as
conformance checkpoints rather than a howto):

1. **Create** — ingest any corpus a `Connector` can walk (`store-format-v0.md`
   §1's file-container properties apply from this point on).
2. **Compile** — dry-run/estimate mandatory before spend
   (`docs/design/semantic-layers.md` §4.4); real usage recorded in
   `compile_runs` (a Writer-conformance-required table,
   `store-format-v0.md` §3).
3. **Verify** — compile-time citation/scope verification rejects claims
   whose citations don't resolve or whose scopes match nothing; eval
   fixtures gate the layer's inclusion in a *shipped* set (§3.5's
   `eval_evidence`, not a v0 module-level requirement — a module MAY ship
   an honestly `evaled: false` layer, same as `findings`
   does today, `docs/layers.md`).
4. **Refresh** — watermark drift detected (`store-format-v0.md` §6) →
   re-ingest → citation sweep (`store-format-v0.md` §5.4/§5.5) → recompile
   only what drifted. Cost scales with change size, never corpus size
   (`docs/design/semantic-layers.md` §6).
5. **Ship** — copy the store file (WAL caveat: `store-format-v0.md` §1.2).
   The manifest (§3) travels with it, however it is packaged.
6. **Serve** — §4's serving contract, minimum floor or full expert surface.
7. **Federate** — explicitly NOT v0 (§7).

## 7. v0 scope and honest gaps

**In v0 (proven in code today):**

- Store portability (§5) — `TestStorePortability_RoundTrip`.
- The claims schema, citation sweep, and refutation durability
  (`store-format-v0.md` §8) — the same mechanisms the coding-companion
  tracer bullet (`obligations`) already exercises in production on this
  repo's own corpus (`docs/layers.md`).
- A real non-git module compiled end-to-end
  (`docs/design/expert-modules.md` §7 item 3: a findings-style corpus,
  ingested as its own service, claims compiled, queried standalone, passing
  the portability round-trip) — this is the demo that a module is
  configuration, not a new code path.
  **Enforcement:** `TestExpertModule_NonGitCorpus_EndToEnd`
  (`pkg/client/expertmodule_test.go`) is this demo, executable: ingest a
  non-git findings corpus, compile a claim over it, query it standalone
  (citation + tier attached, §4), edit the finding and re-ingest (claim
  serves `stale`, flagged not dropped — §4 requirement 2), then move the
  corpus + remap and assert the sweep finds zero contradictions and a
  subsequent re-ingest is fully hash-gated (§5's portability contract, the
  same shape `TestStorePortability_RoundTrip` proves for a code corpus,
  proven here for a claims-bearing one).

**Honest gaps (v0 documents the right shape; the reference implementation
has not caught up on these):**

- **No manifest loader ships in this repo.** `schema/module/schema.cue`'s
  `#Module` is validated by `internal/modspec/schema_test.go` (CUE-level:
  the shape is enforceable), but there is no `fathom module` CLI verb or Go
  package that reads a manifest at runtime and acts on it — unlike
  `#Layer`, which `internal/semantic.LoadLayer` actually decodes and
  executes against. **UNENFORCED** beyond CUE-shape validation; tracked
  here rather than silently assumed to exist. This mirrors
  `docs/design/expert-modules.md` §7's own scoping: "the pack CLI, when it
  comes, is a thin serializer" is explicitly H2/entry-conditioned work, not
  v0.
- **No packaging format for "manifest + store as one shippable artifact."**
  §1 deliberately leaves this open; `fathom pack` (`product-vision.md` §4
  H2) is the future thin serializer this spec anticipates but does not
  specify.

**Entry-conditioned, NOT v0 (named triggers, from `docs/design/
product-vision.md` §4 H3 and `docs/design/expert-modules.md` §7):**

- **Federation.** N conforming modules attached behind one router
  (`pkg/platform`'s cross-service machinery, generalized). Entry: ≥2 real
  external module consumers, or ≥100 external installs + ≥60% internal
  fathom-share sustained 4 weeks.
- **Failure-mode playbook module** (the incident/runbook use case,
  `docs/design/expert-modules.md` §3.3). Entry: corpus reaches ≥15–20
  resolved findings (today: ~5) — compiling below that would fail the
  layer's own precision gate.
- **Vectors as an in-store column.** Entry: telemetry showing intent-search
  misses that joins can't answer (`store-format-v0.md` §11).
- **Regulated-domain / external-connector modules.** Standing non-goal
  (`docs/design/product-vision.md` §3's "Standing non-goals" row) unless an
  external party ships a conforming module on this published spec with zero
  connectors authored by fathom.

## 8. Non-goals

Inherits `store-format-v0.md` §11 in full (no federation, no vector
columns, no multi-tenant store, no network wire protocol) — a module is a
store plus a manifest, and adds no non-goal of its own beyond what §7 above
already scopes out.
