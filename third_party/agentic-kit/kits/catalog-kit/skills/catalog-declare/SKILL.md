---
name: catalog-declare
description: Author CUE catalog entries, team extensions and atomic decision bindings. Use when adding a catalog item, extending a hierarchy or preparing decision-model review criteria.
---

# Declare Catalog Entries

Read ../../README.md and schema.cue at the kit root. Resolve links relative to this file. Read the affected domain schema before authoring payloads.

1. Assign a stable key independent of the display title. Choose one canonical path placement; use facets for independent axes. Include source and revision/watermark where available. Never copy an inherited ID to a sibling layer without declaring its dependency.
2. Use ordinary nodes for new entries and compatible additions. Conflicting scalars require an explicit override with a fresh NodeDigest, a reason and a receiver-owned grant. Use explicit patch pointers to clear an order or hidden flag; do not assume zero means omitted.
3. For model use, split broad review rules into atomic claims. Bind one decision.Question to the exact node digest, definition version and SHA-256 definition_digest of the complete current domain definition bytes. Supply those exact bytes separately with prepare -definition. Supply authored option criteria and unknown for choice questions. Keep arithmetic, dates, IDs and exact checks deterministic.
4. Declare the required evidence IDs. The collector must capture allowed text with source, revision and exact text hash. A complete-looking packet is not proof of semantic sufficiency. Missing evidence must stop preparation, not become an empty prompt.
5. Keep applicability, evidence, observation and policy outcome distinct. Score is a rubric position, not probability of correctness. Provider thresholds require a measured, provider-specific evaluation; do not invent one.
6. Validate CUE, export JSON, resolve and prepare using catalogctl. Include positive, benign-lookalike, missing-context and malformed-input cases. Keep the parent rule open until every required criterion is established.

Return the authored changes and their verification scope. A prepared request does not authorize a provider call, external action or review closure.
