---
name: catalog-adopt
description: Consolidate an existing CUE catalog into Catalog Kit. Use when adopting shared catalog composition, replacing duplicate merge glue, or planning a catalog migration.
---

# Adopt Catalog Kit

Read ../../README.md and the repository intent router. Inventory the existing domain schema, stable IDs, import/override semantics, consumers and gates before changing code. Use Fathom first when available; record its absence and use local source otherwise.

1. Preserve the domain owner. Reuse foundations/knowledge/catalog for hierarchy/search/drift and foundations/decision for questions. Keep domain payloads and semantic floors in their existing kit.
2. Capture a baseline from the consumer's actual catalog. Record source revision, effective entries, hidden entries, provenance and one refusal case. A successful demonstration is not production adoption.
3. Author a #Spec using selected local layers and explicit requires edges. Require stable keys and sources for new entries. Validate CUE, export it, then run catalogctl resolve. Keep receiver policy in a separate reviewed input.
4. Compare the old and proposed effective catalogs, including hidden state, order and synopsis. Use catalogctl explain for every override. Do not replace a domain resolver until its stricter floors pass against the new composition.
5. Test cycle, duplicate identity, stale patch, missing source and cap failures through the entrypoint the consumer will run. Keep network synchronization outside resolution and pin reviewed snapshots.
6. Run local CUE/Go checks, task validate and affected consumer checks. Report each recorded revision, pass, failure and skip. An absent consumer is untested.

Produce a reviewable migration with explicit remaining gaps. Do not enable model providers, change global agent settings, publish a release or remove the old path merely because the catalog resolves.
