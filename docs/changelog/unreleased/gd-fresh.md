### Changed
- Index-sourced symbol, obligation and review citations carry typed freshness statuses and qualified rule summaries and hook tags. Each call streams each cited file once, keeps only snippet windows, and leaves byte-identical files clean during refresh; `read_source` reports its content identity and known index match.
- Existing indexes report `unknown` (not `stale`) for a changed file whose last full ingest saw it within 2 s of a write, until the next full ingest.
