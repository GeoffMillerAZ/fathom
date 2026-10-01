### `migrate-store` (removed, SS-8)

**No longer exists in this or any later build.** It ported an existing
DuckDB store's eight non-derivable tables (`semantic_claims`, `claim_links`,
`claim_events`, `compile_runs`, `service_meta`, `commits`, `knowledge_cards`,
`doc_summaries`) into a freshly created SQLite store (ADR-020). Once the
fleet had migrated and the daemon ran a full day green on SQLite, SS-8
(`docs/plans/store-sqlite.md`; ADR-025) deleted the command along with the
DuckDB driver it depended on — the trigger fired 2026-07-23. The historical
runbook (what it did, in order) is kept as a historical note in
[configuration.md](../configuration.md); a genuine need to read an old
`.duckdb` file requires a pre-SS-8 build of fathom.

---
