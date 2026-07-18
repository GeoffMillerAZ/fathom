# foundations/provenance

Authorship, journaling, and proof-of-existence for the evidence spine
(see `foundations/evidence/README.md` for the full pipeline).

| Package | Purpose | Concurrency contract |
|---|---|---|
| `ai-provenance` | `Author` record (human or AI) attached to every finding/decision/prose node. AI authors carry a full reproduction key (model + prompt + params + input digest) so claims can be invalidated or re-run. Embedded as `evidencespan.Span.Author` | Not documented — value types + pure constructors; assume single-goroutine |
| `anchor` | Merkle-tree proof-of-existence for batches of evidence content-hashes. Per-leaf inclusion proofs reveal only the path, never sibling content. Trust comes from an external authority (RFC 3161 / OpenTimestamps / CT log) binding the root to wall-clock time; submission is an adapter concern | `MerkleTree` documented safe for concurrent reads once constructed; `AnchorBuilder` is single-use (`AddLeaf` panics after `Build`), no concurrent-safety documented |
| `event-store` | Append-only, hash-chained `Store` — **the journal** / system-of-record for chain-of-custody. Each entry commits to its predecessor; `Verify` detects edits, reordering, middle-drops, and Seq gaps offline | `Store` documented safe for concurrent use (Append, Entries, Head, Len, Verify, Replay all lock) |

## How they compose

1. Evidence producers stamp every span/decision with an `aiprovenance.Author`.
2. Evidence lifecycle events are appended to an `event-store.Store` — the
   append-only journal that downstream projections must never be trusted over.
3. `anchor` batches content-hashes into a Merkle tree for external timestamping.
   The event-store's own threat model says it cannot detect authorized chain
   rewrites or tail truncation — anchoring `Head().Hash` plus the entry count
   in an external trusted store (which is exactly what `anchor` is for) closes
   that gap.

Determinism: `event-store` and `anchor` take injected clocks
(`NewStoreWithClock`, `NewAnchorBuilderWithClock`); `ai-provenance`
constructors take an explicit `at time.Time`.
