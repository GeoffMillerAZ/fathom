# Knowledge retrieval: full-text search over the words, linked to the code

Status: accepted 2026-09-27 (ADR-049 decision 8). Vision: `docs/vision/38-proof-layer-roadmap.md`.
Plan: `docs/plans/knowledge-retrieval.md` (KR-). Goal G20. Constraints carried in: ADR-041 and
vision 31 (embeddings removed; §5's kill criterion is the only way back), ADR-047 and I48 (no
request proportional to repository size), I38 (an unreadable input is never a healthy zero),
invariant I56 (no downloads), store purity (I2: pure Go, no CGO in `internal/store`).

## 1. Problem

Fathom measured badly on documentation questions ("what did we decide about X", "where is
the tenancy budget described", "what does this plan say about Y"): the tools rank code, and
`search` is grep with a receipt. The words that govern the code (ADRs, intent, plans, design
docs, memories, comments) have no ranked retrieval, and nothing notices when a doc describes a
symbol that has since changed.

## 2. Decisions

1. SQLite FTS5 in the existing store. **Verified:** the pure-Go driver in `go.mod`
   (`modernc.org/sqlite` v1.52.0, SQLite 3.53.2) creates `fts5` virtual tables with the
   `porter unicode61` tokenizer and answers `bm25()` and `snippet()` (probe run 2026-09-27 in
   this repository's module). No CGO, no second store, no service.
2. Section-aware: the unit is a section, and its heading path is a column.
3. BM25 fused with the existing concern ranking by reciprocal rank fusion; no learned weights.
4. Doc spans that name a symbol, a path or a contract key are linked; a changed target flags
   the span possibly stale, and `staleness_report` reports doc drift.
5. Dependency docs are served version-exact from the local caches the dependency plane reads.
6. No embeddings. Vision 31's kill criterion stands; KR-M1 is a lexical reading and promotes
   nothing in F13.

## 3. Corpus and units

| Source | Files | Unit |
|---|---|---|
| Decisions | `ADR.md` sections | an `## ADR-NNN` section, split further at bold paragraph labels (Context, Decision, Revisit when) |
| Intent | `docs/intent/*.md` | a `##` section or a numbered record (G, F, Q, I, P ids) |
| Plans | `docs/plans/*.md` | a `###` section, and each row as its own unit carrying its id |
| Design and vision | `docs/design/*.md`, `docs/vision/*.md` | a heading section (the deepest heading with body text) |
| Other docs | `*.md` under the service root, excluding generated and vendored paths | a heading section |
| Memories | the store's memory rows | one memory |
| Comments | doc comments on indexed symbols (already extracted with the symbol) | one comment, keyed to its symbol |

Documents inside `docs/plans/archive/` and `tmp/` are indexed with a `stale_by_location`
flag so they rank below live docs, never silently above them.

## 4. Store (additive)

```
docs_fts USING fts5(service_id UNINDEXED, path UNINDEXED, unit_id UNINDEXED,
                    heading_path, body, kind UNINDEXED,
                    tokenize = 'porter unicode61')
doc_units(service_id, unit_id, path, line_start, line_end, heading_path, kind,
          content_hash, indexed_at)
doc_links(service_id, unit_id, target_kind, target, line, link_tier, target_hash_at_link,
          valid_from, valid_to)            -- target_kind: symbol|path|key
```

Units are written at ingest when a doc file's content hash changes (hash-gated, P19); a
deleted file's units are removed in the same transaction. `doc_links.valid_to` is set when a
link stops holding (the governance-learning validity-window rule), not deleted.

## 5. Ranking

A query runs `docs_fts MATCH ?` bounded to the service set, `ORDER BY bm25(docs_fts, 3.0,
1.0)` (heading weighted three to one), `LIMIT 50`. The concern ranker scores the same units by
their links (a unit linked to symbols in the concern ranks higher). Reciprocal rank fusion
(k = 60) merges the two lists; results carry `snippet()` excerpts, the heading path, file and
line range, and the receipt's byte count. The surface is the existing `search` verb with
`kind: docs` (law 4: no new tool); `pack_context` may include the top doc units under its
existing budget.

## 6. Doc-to-code links and doc drift

A span is linked when it names, inside backticks or a code block:

| Target | Rule | Tier |
|---|---|---|
| a symbol | `Name` or `pkg.Name` resolving to exactly one indexed symbol in the service | `exact` |
| a path | a repo-relative path that exists | `exact` |
| a contract key | a key that parses under the system-graph grammar and exists in `contract_keys` | `exact` |
| a bare word | a symbol name outside backticks | not linked (too noisy) |

At link time the target's content hash is stored. When a later ingest changes the target (the
symbol's body hash, the file's hash, the key's provider), the span is **possibly stale**.
`staleness_report` gains a `doc_drift` section: span, target, the commit that changed it, and
how long ago. It never says a doc is wrong; it says the thing it describes moved.

## 7. Dependency docs, version-exact

For a `pkg:` key the dependency plane has indexed (`docs/design/system-graph.md` §8), the doc
units are the package's own README and doc comments at the pinned version, read from the same
cache directory. Go doc comments, `.d.ts` JSDoc, Python docstrings in `.pyi` or the module,
Rust `///` comments. Never fetched (I56). Wave 4.

Built (KR-11, KR-12): `depDocUnits` reads a directory a `DepCacheLocator` names (the dependency
plane's cache locator is the production implementation), public surface only, within the 8 MiB
per package and 2 MiB per file bounds; `IngestDepDocs` stores the units under the package's own
`pkg:<ecosystem>/<name>@<version>` key as the service id in the existing tables, so a version's
docs are a separate scope by construction; `search kind: docs pkg: K` answers from that scope
alone. An unindexed version answers `not_indexed` with the versions that are indexed, and an
unpinned key answers `unpinned` (never a guessed version). Maven and NuGet are refused by name.

## 8. Tenancy and cost

| Cost | Bound |
|---|---|
| Index size | FTS5 over prose; expected below 15% of the text indexed; capped per service by the byte budget, with a named `elided` when a service exceeds it |
| Ingest | hash-gated per file; a docs-only change re-indexes only changed units |
| Query | one FTS query with `LIMIT`, one concern pass over at most 50 units; no scan of `doc_units` |
| Link check | on ingest of a changed target only, via an index on `(service_id, target)` |

## 9. Measurement

| Reading | Threshold |
|---|---|
| KR-M1 | a doc-question slice of the bench (at least 30 questions whose answer lives in docs, graded by a registered criterion before the run): fathom's pass rate at least grep's, at no more than 1.0x grep's bytes |
| KR-M2 | doc drift on seeded changes: rename or change 20 symbols named in docs; recall at least 0.9, and no drift flagged for 20 unchanged symbols |
| KR-M3 | index size and ingest time on this repository: FTS below 15% of indexed text, docs ingest below 2 s incremental |

## 10. Jev, weighed

- **Where it could help:** query reformulation for vague questions.
- **Where it cannot:** ranking or linking; both must be deterministic and re-derivable.
- **Cost:** a model call per query on the hottest read path.
- **The measurement that would open it:** KR-M1 failing on a class of questions where a
  reformulated query passes; then an A/B at n at least 3. Until then, no role.

## 11. Refused

- Embeddings or a vector index (vision 31).
- A separate search service or index file outside the store.
- Linking bare words outside code spans.
- Marking a doc wrong: drift means "the target moved", nothing more.
- Fetching dependency docs from the network.
