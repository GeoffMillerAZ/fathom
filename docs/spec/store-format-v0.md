# Governed Store Format v0

*Status: v0, experimental-but-append-only (see `README.md`'s stability
policy). Normative on the on-disk format the fathom binary reads and writes;
descriptive on rationale. Reference implementation: `internal/store`
(SQLite backend, `internal/store/sqlite.go`), domain types `internal/core`.*

**Correction to `docs/design/product-vision.md` §1 and
`docs/design/semantic-layers.md` §5:** both documents describe the format as
a versioned `.duckdb` spec. That was accurate when written (2026-07-07) and
is not accurate now. ADR-020 made SQLite the default backend
(2026-07-22); ADR-025 deleted the DuckDB backend outright (2026-07-23, SS-8
executed). **This spec is SQLite-only.** There is no DuckDB code path left to
target — `internal/store/backend.go`'s `BackendForPath` recognizes a
`.duckdb` extension only to refuse it (`errDuckDBRemoved`), never to open it.

## 1. File container

1. **MUST** be a single SQLite database file (`modernc.org/sqlite`, pure Go —
   no CGO to *read* a conforming store; ADR-020). File extension `.sqlite` or
   `.db`; `.duckdb` **MUST** be refused, not silently opened
   (`internal/store/backend.go`'s `BackendForPath`).
2. **MUST** be opened in WAL mode by a writer (`journal_mode(WAL)`,
   `internal/store/sqlite.go`'s `sqlitePragmaDSN`). This is the normative
   requirement; the copying discipline it implies for anyone shipping a
   *live* store is a separate, descriptive concern — see 2a below, kept
   apart deliberately so a MUST and an operational SHOULD are never
   conflated under one bullet.
   - **2a (SHOULD, operational, UNENFORCED).** A WAL-mode store open in a
     writer keeps recent commits in a `-wal` sidecar. A store copied while a
     writer holds it open SHOULD be copied only after the writer closes
     (which checkpoints), or as the `.sqlite` + `-wal` + `-shm` triple
     together — a bare copy of an in-use store risks silently dropping
     un-checkpointed writes (`docs/design/expert-modules.md` §5, step 5).
     This is prose discipline, not a file-format property: nothing in the
     format itself is violated by an operator who copies mid-write, only
     the copy's completeness.
3. **MUST NOT** require a running server to read. A conforming reader opens
   the file directly (embedded) or through the daemon — both are the same
   engine (`docs/design/expert-modules.md` §1: "there is no second engine,
   no second schema").
   - **Honest tension with the reference implementation.** The reference
     implementation does not actually offer a pure-read code path today:
     `store.Open`/`NewSQLiteStore` always runs `createSchema` (idempotent
     `CREATE TABLE IF NOT EXISTS` DDL, plus the format-marker pragma writes
     in §2) on every open, including one whose caller only intends to read —
     so opening a conforming store with THIS binary currently assumes
     writable media. A store on read-only media (a mounted read-only
     filesystem, or a DSN opened `?mode=ro`) is legal per this spec's file
     container but is not yet openable by `internal/store`'s own
     implementation. Closing this gap is a reference-implementation
     improvement (a genuinely read-only open path), not a spec change —
     tracked here rather than silently assumed away.
4. A conforming reader **MUST tolerate unknown tables and unknown columns**
   (README.md's compatibility rules). This is what makes "additive within
   v0" a real guarantee instead of a documentation promise.

**Enforcement:** file-container properties 1 and 3 are exercised end-to-end
by `TestStorePortability_RoundTrip` (`pkg/client/portability_test.go`) — it
builds a store, closes nothing but the corpus root, and reads it back
through the public `pkg/client` API with no daemon involved. 2a (the WAL
copy caveat) is **UNENFORCED** — it is an operational discipline documented
in `docs/design/expert-modules.md`, not a property a Go test can assert
against a single process's own connection pool. The read/write-open tension
noted under 3 is likewise **UNENFORCED**: there is no test asserting a
read-only open path, because no such path exists yet to test.

## 2. File-identity and format-version markers

A conforming store **MUST** carry two SQLite-native pragma markers, stamped
by `internal/store.SQLiteStore`'s schema setup (`sqlite.go`,
`stampFormatMarkers`, called from `createSchema`) — SQLite's own format
reserves exactly these two pragmas for exactly this purpose, WHOSE file
(`application_id`) and WHICH VERSION of that file type (`user_version`):

- **`PRAGMA application_id` MUST equal `0x46415448`** ("FATH" — the ASCII
  bytes F/A/T/H read big-endian, a stable, human-verifiable mnemonic rather
  than an opaque hash). This is the "is this even a fathom store?" check —
  fixed forever once published: changing it would break every existing
  reader's identification of a conforming store.
- **`PRAGMA user_version` MUST equal `1`** for this spec (v0's marker
  value). **Deliberately not `0`:** `user_version` defaults to `0` on every
  SQLite database ever created, including one no fathom writer has ever
  touched — mapping spec v0 to marker `0` would make the marker vacuous (a
  reader could never distinguish a conforming v0 store from an arbitrary
  SQLite file that merely happens to share the `.sqlite` extension). **`0`
  is reserved to mean "pre-spec/unversioned"** — a reader that sees
  `application_id = 0x46415448` but `user_version = 0` has found a fathom
  store written before this marker existed (or, in principle, one a future
  spec revision deliberately left unstamped) and **MUST** fall back to
  schema introspection (§3's required-table/column check) rather than
  trusting the version number.

**Write policy (both pragmas, additive-safe):**

- `application_id` is stamped only when it currently reads `0` (SQLite's own
  "never set" default) — a writer **MUST NOT** overwrite a non-zero, foreign
  `application_id` some other tool may already have stamped on a `.sqlite`
  file that merely happens to share fathom's store extension (§1's
  `BackendForPath` dispatches on extension alone, so this is a real
  possibility, not a theoretical one).
- `user_version` is bumped only when it is strictly lower than the writer's
  own `storeFormatVersion` — a writer **MUST NOT** decrease it. A store
  already stamped at or above the writer's own version (e.g. by a future
  v1+ writer) is left untouched, so an older binary opening a newer store
  cannot regress the marker a newer reader depends on. A fresh store, and a
  legacy store created before this marker existed, both read `0`
  (unversioned) and are correctly bumped to `1` on first open by a marker-
  aware writer — this bump is itself harmless: it asserts nothing the store
  didn't already structurally satisfy (§3's required tables/columns are
  unconditional on every SQLiteStore-created store regardless of marker
  presence), it only makes that fact legible without introspection.
- A reader **MUST** treat a `user_version` higher than the highest version
  it implements as "read with caution, expect unknown tables/columns," never
  as an error — the whole point of "additive within v0" is that new fields
  don't break old readers by design.

**Enforcement:** `TestStoreFormatV0_RequiredSchema`
(`internal/store/format_v0_test.go`) asserts a freshly-opened store's
`PRAGMA user_version` reads back as `1` and `PRAGMA application_id` reads
back as `0x46415448`. `TestStoreFormatV0_StampsUnversionedStore` (same file)
is the live-write-path proof this marker design depends on: it creates a
bare SQLite file with neither pragma set (both read `0`, verified as a
precondition), opens it via `store.Open`, and asserts both markers are
stamped on that first open — the scenario that would have nothing to assert
if the marker were mapped to `0` instead of `1`.
`TestStoreFormatV0_FormatVersionNeverRegresses` (same file) proves the
non-regression guarantee: a store pre-stamped at `user_version = 7`
(simulating a future v1+ writer) keeps reading `7` after being reopened by
this (v0-only) binary, never reset down to `1`.

## 3. Required tables and columns per conformance level

The tables below are the REQUIRED subset — a conforming store **MUST**
contain at least these tables with at least these columns (additional
tables/columns are always legal; column order and additional NOT
NULL/DEFAULT decoration are implementation detail, not part of the
contract). Transcribed from, and kept in lockstep with,
`internal/store/sqlite.go`'s `createSchema` DDL —
`TestStoreFormatV0_RequiredSchema` checks this exact table against the live
schema by introspecting `sqlite_master`/`PRAGMA table_info`, and
`TestStoreFormatV0_DocMatchesGoSource` (both `internal/store/format_v0_test.go`)
scrapes the L0 and Writer markdown tables below directly out of this file and
diffs them against the same Go-side map, so a drift on either side — the DDL
or this doc — is a failing test, never a silently stale doc.

### L0 — minimum readable store

A store conforming to no more than L0 can enumerate services, symbols, and
claims with citations intact, but makes no promises about staleness
presentation.

| Table | Required columns |
|---|---|
| `service_meta` | `service_id`, `path`, `indexed_commit`, `indexed_at` |
| `observations` | `service_id`, `concern_id`, `source_ref`, `source_kind`, `evidence_tier`, `symbol`, `package`, `claim`, `content_hash`, `calls`, `complexity`, `line`, `end_line`, `ingested_at`, `tool_kind`, `generated` |
| `semantic_claims` | `service_id`, `claim_id`, `layer`, `layer_version`, `kind`, `subject`, `rule`, `scope`, `tier`, `tier_profile`, `status`, `model`, `prompt_version`, `repro_key`, `input_digest`, `compiled_at_watermark`, `compiled_at` |
| `claim_links` | `service_id`, `claim_id`, `rel`, `target_ref`, `target_hash`, `anchor_text` |

`service_meta.path` is the **only** field that is honestly machine-local (a
filesystem path at the reading machine, rewritten by `fathom remap` on
move — §5); every other required column above is portable by construction
(§5).

### L1 — staleness- and tier-honest reader

L1 adds no new required table over L0 — it is a behavioral obligation over
the *same* schema: an L1-conforming reader **MUST** surface, for every served
claim, its `tier` (§7) and — when the claim is not fresh — its `status`
(never silently normalize `stale`/`contradicted` to absent). It **MUST**
also be able to detect watermark drift using only stored data: comparing a
claim's `compiled_at_watermark` against the owning service's current
`service_meta.indexed_commit` (a mismatch means the corpus has moved on
since this claim's evidence was last confirmed — see §6).

**Enforcement:** `TestObligationsFor_ScopedOrderedFlagged`
(`internal/semantic/serve_test.go`) is the concrete instance — it asserts a
fresh claim's payload always carries `Src`/`Tier`, a stale claim's payload
carries `Status` (never dropped), and a refuted claim never appears at all
(§8).

### Writer — append-only lineage discipline

A store conforming to Writer level, in addition to L0's tables, **MUST**
contain:

| Table | Required columns |
|---|---|
| `claim_events` | `claim_id`, `verdict`, `author`, `note`, `event_at` |
| `compile_runs` | `run_id`, `service_id`, `layer`, `corpus_watermark`, `model`, `prompt_version`, `dry_run`, `tokens_in`, `tokens_out`, `cost_usd`, `duration_ms`, `sections_total`, `sections_compiled`, `sections_failed`, `claims_emitted`, `claims_rejected`, `ran_at`, `refused`, `refusal_reason`, `budget_stopped`, `escape_note` |

A Writer **MUST NOT** delete rows from `semantic_claims` or `claim_events` —
superseding a claim is a status flip (`status` column), never a row
deletion; correcting a claim is a new `claim_events` row, never an edit or
delete of a prior one. See §8 for the exact lineage rules this backs.

### Server — minimum honest-serving contract

Server conformance adds no new table over L0/Writer — it is a behavioral
requirement on whatever process answers queries over the store (the daemon,
an embedded CLI invocation, or a third-party reader): every served claim
payload **MUST** carry its citation (`Src`) and evidence tier (`Tier`)
alongside the rule text, and **MUST** carry its `status` whenever that status
is not the fresh/default state — an agent that saw a rule yesterday must see
"that rule is stale" today, never silent absence
(`docs/design/semantic-layers.md` §4.3).

**Enforcement:** `TestObligationsFor_ScopedOrderedFlagged`
(`internal/semantic/serve_test.go`) asserts the citation/tier/status
presence rules directly at the library layer (seeds a fresh, a stale, and a
refuted claim; asserts `Src`/`Tier` are always populated, `Status` is
present on the stale one and empty on the fresh one, and the refuted claim
never appears). `TestObligationsTool_WithRules`
(`internal/mcp/semantic_tools_test.go`) is the MCP-layer companion: it
proves the tool wraps the same `semantic.Obligations` payload end-to-end
(rules, `compiled_at_watermark`, `index_stale` all survive the JSON-RPC
round-trip) rather than reshaping or dropping fields on the way out.

## 4. Non-required tables (present, not part of the required-schema contract)

`fingerprints`, `knowledge_cards`, `doc_summaries`, `commits`,
`typed_edges`, and `typed_edge_fingerprints` exist in the reference
implementation's schema (`internal/store/sqlite.go`) but are enrichment/
cache tables, not part of any conformance level's required set — a
conforming reader that never sees them (an older store, or a hand-built
conforming writer that never populates them) is still L0/L1/Writer/Server
conformant. They are documented here only so a schema diff against a real
store isn't mistaken for a spec violation.

## 5. Anchor rules (ADR-017)

1. Every `observations.source_ref` and every `claim_links` `target_ref` of
   `rel = "cites"`/`"scopes"`/`"scope-fp"` **MUST** be repo-relative,
   slash-separated (never a machine-local absolute path). A store **MUST
   NOT** record an absolute path at write time.
   **Enforcement:** `TestStorePortability_RoundTrip`
   (`pkg/client/portability_test.go`) asserts this directly
   (`filepath.IsAbs(o.SourceRef)` must be false for every observation) as a
   *birth* property, immediately after ingest, before any move.
2. **Root remap.** `service_meta.path` is the one machine-local field
   (§3); moving a corpus (or shipping a store to a new machine) is exactly
   one metadata update — `fathom remap` (`cmd/fathom/remap.go`) rewrites
   `service_meta.path`; every citation, source fetch, and watermark
   recomputation resolves through it immediately, with zero re-ingest.
   **Enforcement:** `TestStorePortability_RoundTrip`, steps 1–2 (symbol
   query and source-fetch resolve correctly at the new root after a
   `SetServiceMeta` remap and zero re-extraction).
3. **Legacy self-heal.** A store written before ADR-017 (absolute anchors)
   is still readable — `LocalConnector.Fetch` resolves an absolute
   `SourceRef.Path` as-is (back-compat fallback) rather than erroring — but
   the first write-path ingest against it purges that service's content
   wholesale (observations, fingerprints, doc summaries; identity rows kept)
   and rebuilds with relative anchors, reporting `PurgedLegacyAnchors: true`
   (`internal/fusion/pipeline.go`). **MUST NOT** silently mix absolute and
   relative anchors for the same service after that rebuild.
   **Enforcement:** `TestIngest_SelfHealsLegacyAbsoluteAnchors`
   (`internal/fusion/pipeline_test.go`) — seeds a service with absolute-path
   observations, ingests, and asserts `PurgedLegacyAnchors: true` plus a
   clean second re-ingest (`PurgedLegacyAnchors: false` — one-shot, not
   repeated every ingest).
4. **Anchor-text healing.** A `claim_links` row of `rel = "cites"` carries
   both `target_hash` (sha256 of the cited line range at compile time) and
   `anchor_text` (the literal cited text at compile time). On drift (hash
   mismatch), a reader/sweeper with `anchor_text` available **SHOULD**
   attempt to re-resolve the citation by searching the cited file's current
   content for that exact text: an unambiguous single match means the
   citation shifted (e.g. a prose insertion above it) and the citation
   **SHOULD** be re-anchored (`store.UpdateClaimLink`) in place, recorded as
   a `claim_events` row with `verdict: "re-anchored"` (status-preserving —
   the claim keeps serving fresh). A zero-match or ambiguous (more than one)
   result **MUST NOT** be treated as a re-anchor — it falls through to the
   stale/contradicted path (§8). `claim_links` rows written before
   `anchor_text` existed carry `""` and **MUST** skip the re-anchor attempt
   entirely (empty `anchor_text` means "cannot attempt," not "search for the
   empty string").
   **Enforcement:** `TestSweep_ReanchorsOnUpstreamLineShift` (the healing
   path), `TestSweep_OutOfRangeCitationContradicts` and
   `TestSweep_ContradictedRecordsContradictedEvent` (the never-silent-drop
   path) — all in `internal/semantic/sweep_test.go`.
5. **Broken citation ⇒ flagged, never silently served.** A citation that no
   longer resolves at all (cited file gone, or line range now out of
   bounds) **MUST** flip the owning claim's status to `contradicted` and the
   claim **MUST** continue to be served, with that status attached — never
   silently dropped, and never silently trusted as fresh.
   **Enforcement:** `TestSweep_OutOfRangeCitationContradicts`
   (`internal/semantic/sweep_test.go`); served-with-flag is additionally
   asserted at the payload layer by `TestObligationsFor_ScopedOrderedFlagged`
   (§3's Server-level citation).

## 6. Watermark formats and staleness semantics (ADR-017)

A watermark is an opaque freshness token for a service's corpus, computed by
`internal/connector.Connector.Watermark`:

- **Git-backed corpus:** the bare 40-character git HEAD SHA
  (`internal/connector/watermark.go`'s `gitHead`).
- **Non-git corpus:** a content-addressed hash, **MUST** be prefixed
  `manifest:` followed by 64 lowercase hex characters — sha256 over every
  discovered file's `(relative path, content hash)` pair in `Discover`
  order (`manifestWatermark`). Equal watermarks ⇔ identical corpus content,
  independent of machine, directory location, or mtimes.
- `""` (empty string) is a legal watermark meaning "freshness unknown" — an
  honest absence, never an error and never treated as "fresh."

**Two independent staleness mechanisms**, because they cover different
windows (`docs/design/semantic-layers.md` §4.3):

1. **Corpus-level:** `service_meta.indexed_commit` records the watermark at
   last ingest. A reader comparing a claim's `compiled_at_watermark`
   (§3, `semantic_claims`) against the CURRENT service watermark (recomputed
   live via `Connector.Watermark`, or approximated from
   `service_meta.indexed_commit` when only the store is available — §3's L1
   definition) detects drift since the claim was last confirmed.
2. **Citation-level:** the post-ingest sweep (§5.4/§5.5) re-hashes each
   claim's cited lines against the current corpus independent of the
   service-level watermark — a doc can drift a citation without the
   service's overall watermark changing meaning if only that one file
   moved, so citation-level and corpus-level staleness are deliberately not
   collapsed into one signal.

**Enforcement:** watermark format is asserted directly by
`TestLocalConnector_Watermark_ManifestStableAcrossMove` and
`TestLocalConnector_Watermark_GitRepoUsesHead`
(`internal/connector/watermark_test.go`, `internal/connector/local_test.go`).
Portability of watermark equality across a corpus move is asserted by
`TestStorePortability_RoundTrip` (watermark at location A equals watermark
at location B after a pure move, and the stored `IndexedCommit` matches the
recomputed value — no false staleness from relocation alone).

## 7. Evidence-tier vocabulary

`EvidenceTier` (`internal/core/model.go`) is a closed, five-value
vocabulary. A conforming store's `evidence_tier`/`tier` columns **MUST**
contain only these values:

| Tier | Meaning | Ceiling |
|---|---|---|
| `verified` | extracted directly from code, or produced by a deterministic Model-empty compile path (ADR-018 oracle classes: `index`, `cmd`, `lexical`) | 5 |
| `pre-scan` | pattern-matched, not confirmed | 4 |
| `doc-claimed` | stated in documentation, citation-grounded | 2 |
| `sme-stated` | said by a human expert | 2 |
| `assumed` | inferred, no direct evidence | 1 |

A code fact (`verified`) never outranks-by-tier a doc claim by accident —
ceilings are enforced by the query engine (agentic-kit's scoring
foundation), not by this store format directly; the format's obligation is
only that the stored value is one of the five above.

**`tier_profile` (ADR-017 §5).** Every `semantic_claims` row carries a
`tier_profile` string (`internal/core.SemanticClaim.TierProfile`, default
`"engineering"`) naming which tier *vocabulary* a claim's tier belongs to.
v0 ships exactly one profile (`"engineering"`, the table above) and defines
no machinery for interpreting a second profile differently — this is "data
now, machinery later": the column exists so a future non-engineering domain
(a legal-compliance module, say) can register its own tier semantics without
a schema migration, but v0 does not specify what a second profile's values
would mean. This is **documented as present-but-not-yet-normative**
(README.md's "v0 may still fail" clause) rather than omitted.

**Enforcement:** the closed vocabulary is asserted by `TestEvidenceTierValid`
(`internal/core/model_test.go`) at the domain-type level.

## 8. Claim lineage rules

Claims are identified by `claim_id = sha256(layer, kind, subject,
repro_key)` (`internal/core.ClaimID`) — content-addressed and stable across
recompiles of unchanged inputs. Lineage rules govern what a recompile is and
is not allowed to do to an existing claim_id's row.

1. **Claims are never deleted by compilation.** A recompile that re-derives
   an unchanged `(layer, kind, subject, repro_key)` tuple **MUST** produce
   the same `claim_id` and upsert the same row (`ON CONFLICT (service_id,
   claim_id) DO UPDATE`, `internal/store/sqlite.go`'s `PutClaims`) — never a
   duplicate row.
2. **Status is a status flip, never a delete.** `ClaimStatus` transitions
   (`fresh` → `stale` → `contradicted`, or → `superseded`, or → `refuted`)
   **MUST** be recorded by updating the `status` column on the existing row,
   never by deleting and reinserting.
3. **Refutation durability — the load-bearing rule.** A `claim_id` whose
   status is `refuted` **MUST NOT** be resurrected to a compiler-assigned
   status by a later recompile that re-emits the same `claim_id` (e.g.
   because the governing doc round-trips back to earlier wording). This is
   the property that makes "a human said no" durable across the corpus'
   normal churn — a refuted claim escaping refutation via a coincidental
   recompile would make the refute channel worthless.
   **Enforcement (code path):** the SQL itself —
   `internal/store/sqlite.go`'s `PutClaims` upsert:
   `status = CASE WHEN semantic_claims.status = 'refuted' THEN
   semantic_claims.status ELSE excluded.status END` — refutation cannot be
   overwritten by any `PutClaims` call, structurally, regardless of what the
   compiler passes in.
   **Enforcement (test):** `TestReview_FindingsBridge_RefutationDurability`
   (`cmd/fathom/review_test.go`) exercises exactly this: refute a claim,
   recompile, assert the claim is still refuted.
4. **`claim_events` is append-only.** `internal/store.Store` (`store.go`)
   defines exactly one write method over `claim_events` —
   `AppendClaimEvent`, a bare `INSERT` (`internal/store/sqlite.go`) — and no
   update or delete method over that table at all.
   **Enforcement (code path):** the absence of any such method on the
   `Store` interface is itself the enforcement — there is no code path in
   this codebase that can mutate or remove a `claim_events` row once
   written.
5. **`refuted` and `contradicted` are different judgments and MUST use
   different `claim_events.verdict` strings.** `contradicted` is the
   sweep's own machine-detected, anchor-dead state (§5.5) — the claim stays
   served, flagged. `refuted` is a deliberate human (or human-equivalent)
   judgment via `fathom claim refute` / the obligations tool's feedback
   action — the claim is never served again. A `claim_events` row reading
   `verdict: "refuted"` for a claim that is, in fact, still being served
   (because only the sweep produced the state) would make the audit log
   lie about whether the claim is live
   (`docs/design/semantic-layers.md` §4.3).
   **Enforcement:** `TestSweep_ContradictedRecordsContradictedEvent`
   (sweep only ever writes `"contradicted"`) and
   `TestClaim_Refute_FlipsStatusAndAppendsEvent` (`cmd/fathom/claim_test.go`,
   human path only ever writes `"refuted"`) — the two verdict strings are
   asserted from two independent write paths, never merged.
6. **`Served()` is closed-form and MUST exclude `superseded` and
   `refuted`.** `core.ClaimStatus.Served()` (`internal/core/claims.go`)
   returns true only for `fresh`, `stale`, `contradicted` — a store's
   serving layer **MUST NOT** present a `superseded` or `refuted` claim in
   any decision-moment payload.
   **Enforcement:** `TestObligationsFor_ScopedOrderedFlagged`
   (`internal/semantic/serve_test.go`) seeds one claim of each status and
   asserts the returned rule count and contents exclude the refuted one.

## 9. Claim identity and reproducibility

A claim's identity is a function of four inputs
(`internal/core.ClaimID(layer, kind, subject, repro_key)`), not of when it
was compiled or by whom. Provenance — separate from identity — is carried on
the row: `model` (empty for a deterministic/oracle-class compile), `prompt_
version`, `repro_key`, `input_digest` (sha256 of the exact source section
compiled), and `compiled_at`/`compiled_at_watermark`. A conforming writer
**MUST** ensure that recompiling the identical corpus section under the
identical prompt version reproduces the identical `claim_id` — this is what
makes the upsert-not-duplicate rule (§8.1) meaningful rather than
coincidental.

**UNENFORCED (gap, precise pointer):** there is no dedicated Go test in this
repo that recompiles the same section twice under a replayed/deterministic
LLM backend and asserts byte-identical `claim_id` end-to-end through the
compiler (as opposed to the store-layer upsert behavior, which §8.1 does
cover). `docs/design/semantic-layers.md` §4.4 references "deterministic
tests via the llmclient replay backend" for the compiler's incremental
digest-gating, but that suite predates this spec and was not written against
this specific MUST. Closing this gap is a compiler-test addition, not a
store-format change — tracked here rather than silently assumed.

## 10. Conformance levels — summary

| Level | Requires | Behavioral obligation |
|---|---|---|
| **L0 reader** | `service_meta`, `observations`, `semantic_claims`, `claim_links` (§3) | Can enumerate services/symbols/claims with citations intact (anchors resolve, §5) |
| **L1 reader** | Same tables as L0 | MUST surface tier + non-fresh status per claim; MUST be able to detect watermark drift from stored data alone (§3, §6) |
| **Writer** | L0 tables + `claim_events`, `compile_runs` (§3) | MUST honor append-only lineage discipline (§8) |
| **Server** | Same tables as L0/Writer | MUST attach citation + tier + non-fresh status to every served claim payload, never silently (§3, §5.5) |

A single process is typically L1 reader + Writer + Server all at once (the
fathom daemon); the levels are separated because a third-party conforming
*reader-only* implementation (§ "Expert Module v0," `expert-module-v0.md`)
is a legitimate, smaller thing to build.

## 11. Non-goals for v0

Explicitly out of scope, with the entry condition that would revisit each
(`docs/design/product-vision.md` §4 H3 unless noted):

- **No federation.** A store is single-corpus, single-writer (D-EM5,
  `docs/design/semantic-layers.md` §5). Multi-store federation is `ATTACH`
  over N conforming stores at a layer *above* this format, not a feature of
  the format itself. Entry: ≥2 real external module consumers, or ≥100
  external installs + ≥60% internal fathom-share sustained 4 weeks.
- **No embedding/vector columns.** Vectors are deferred with named revival
  evidence (`docs/design/semantic-layers.md` §4.7): "telemetry showing
  intent-search misses that joins can't answer." When they land, they are
  specified as an *additive in-store column* over existing claim/summary
  text (D-EM3) — never a second store — but that column does not exist in
  v0 and this spec makes no claim about its future shape beyond "additive."
- **No multi-tenant store.** One store = one corpus = one identity. An
  operator serving multiple tenants runs multiple stores; this format does
  not define row-level tenancy.
- **No network wire protocol.** This spec is a file format, not an API.
  Fathom's MCP surface (`docs/mcp-tools.md`) is a separate, versioned
  concern — a conforming store can be read by a completely different
  serving layer than fathom's own daemon, and this spec makes no claims
  about RPC framing, transport, or auth.
