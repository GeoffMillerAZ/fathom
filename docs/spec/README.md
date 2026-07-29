# Fathom specs — index and stability policy

This directory is fathom's publishable-asset boundary: the durable format the
binary implements rather than the binary itself. The category precedent is
explicit (`docs/design/product-vision.md` §1): SQLite's file format outlived
every app that wrote it; Terraform's moat is the provider protocol, not the
`terraform` binary. Fathom's bet is that **the governed-claim store format**
plays the same role — a versioned, self-describing artifact a third party can
target without us in the loop. Publishing it is the H2 move
(`docs/design/product-vision.md` §4 H2); this directory is where "governed
knowledge engine" stops being a phrase in a roadmap doc and becomes something
with a conformance table and a failing test if violated.

## Documents

| Document | Scope |
|---|---|
| [store-format-v0.md](store-format-v0.md) | The Governed Store Format v0 — file container, required schema, anchors, watermarks, evidence tiers, claim lineage, conformance levels (L0/L1/writer/server), non-goals |
| [expert-module-v0.md](expert-module-v0.md) | The Expert Module v0 — a module = a conforming store + a manifest; manifest fields, serving contract, portability contract, the freshness doctrine |

Companion CUE schema: `schema/module/schema.cue` (`#Module`, the manifest
shape `expert-module-v0.md` §3 describes), tested by
`internal/modspec/schema_test.go`. Companion Go falsifiability test:
`internal/store/format_v0_test.go`'s `TestStoreFormatV0_RequiredSchema`
(cited throughout `store-format-v0.md`'s conformance tables).

## What "v0" means

**v0 is experimental, not a promise of a stable v1.** It is published because
the portability contract it depends on already holds in code
(`pkg/client/portability_test.go`'s `TestStorePortability_RoundTrip`), not
because the shape is expected to be final. Concretely:

- **Additive within v0.** A field or table may be *added* to the store or the
  manifest at any time without a version bump — that is what "v0" already
  buys a conforming reader (see "Compatibility rules" below). A field or table
  is never renamed, retyped, or removed within v0; that always requires a new
  spec version (v1+) and a `format_version` bump.
- **v0 may still fail.** Nothing here is a commitment that v0's shape survives
  contact with a second real consumer. The falsifiability instruments below
  exist so a violation is a red test, not a silent drift, whichever direction
  the spec ends up moving.
- **Experimental fields say so.** A field that exists in code today but has
  not earned its own conformance requirement (e.g. `tier_profile`'s
  reinterpretation machinery — ADR-017 §5, "data now, machinery later") is
  documented as present-but-not-yet-normative rather than omitted; omitting
  known-shaky things from a spec is how specs end up re-litigated instead of
  amended.

## Normative language

RFC 2119-style, used deliberately narrowly:

- **MUST** — a conformance requirement. Every MUST below is either enforced by
  a named test or code path in the reference implementation (this repo), or
  explicitly marked **UNENFORCED** with a pointer to what would close the gap.
  A MUST with neither is a spec bug — file it as one.
- **SHOULD** — a strong recommendation with a known, named exception. Deviating
  is legal but the deviation should be visible (e.g. in a served payload's
  flags), not silent.
- **MAY** — explicitly optional; a conforming implementation can ignore it
  with no compatibility consequence.

Everything else in these documents (rationale, examples, prose) is
descriptive, not normative — it explains why a MUST exists, it does not add
one.

## The falsifiability principle

A spec that cannot be checked against the thing it describes is marketing.
Every MUST in `store-format-v0.md` and `expert-module-v0.md` carries one of:

1. **A named Go test** in this repo that fails if the MUST is violated (e.g.
   `TestStoreFormatV0_RequiredSchema`, `TestStorePortability_RoundTrip`,
   `TestReview_FindingsBridge_RefutationDurability`).
2. **A named code path** whose absence of a contradicting capability is
   itself the enforcement (e.g. "`claim_events` is append-only" is enforced
   by `internal/store.Store` defining no update/delete method over that
   table, not by a test asserting a negative).
3. **UNENFORCED**, explicitly, with a one-line pointer to what's missing and
   why it's still documented as a MUST rather than downgraded to SHOULD (the
   spec should describe the right shape even where the reference
   implementation hasn't caught up, per this repo's own convention of
   preferring the honest gap over the accidental shape).

This is why the spec documents are dense with test names: a reader who
doesn't trust a claim in these documents has a `go test -run <Name>` to run,
not a design doc to re-litigate.

## Compatibility rules (what makes a reader "conforming")

- A conforming reader **MUST tolerate unknown tables and unknown columns**.
  The store is SQLite; a reader that does `SELECT *` or fails on an
  unexpected column is not reading the format, it is reading one snapshot of
  it. `store-format-v0.md`'s required-schema tables list the columns a
  reader may *depend on*, never the columns a store is limited to.
  (`TestStoreFormatV0_RequiredSchema` asserts required columns as a subset
  check against the live schema for exactly this reason — it does not assert
  an exact column set.)
- A conforming writer **MUST NOT** remove or retype a column or table this
  spec documents as required, within v0. Adding one is always legal.
- The store carries two identity/version markers: **`PRAGMA
  application_id`** (a fixed magic number identifying the file as a fathom
  store at all) and **`PRAGMA user_version`** (currently `1` for this spec —
  `0` is deliberately reserved to mean "pre-spec/unversioned," so the marker
  can't be confused with an arbitrary `.sqlite` file that never went through
  a fathom writer) — precisely so a future v1 reader can tell, without
  guessing from column presence, both whether a file is a fathom store and
  which spec version wrote it. `store-format-v0.md`'s "File-identity and
  format-version markers" section is normative on the exact mechanism.

## Out of scope for the index (see each doc's own Non-goals)

Federation, embeddings/vector columns, multi-tenancy, and a network wire
protocol are explicitly out of v0 — see `store-format-v0.md`'s Non-goals
section for the reasoning and the entry conditions that would revisit each
(`docs/design/product-vision.md` §4 H3).
