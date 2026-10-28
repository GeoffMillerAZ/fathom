### Added

- `symbol_context` serves an entity's stored facts under the additive `facts` field (anchored rows plus coverage): a Kubernetes workload's `K8sFact` rows, and the `EntityFact` rows ingest now stores for a Prometheus rule or Alertmanager route, as `docs/mcp-tools.md` details. The extractor epoch changes, so the first ingest after upgrading re-extracts every file once.
