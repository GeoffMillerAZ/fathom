# ADR-0065: Pin selected capability metadata without unrelated catalog churn

- **Status:** Accepted
- **Type:** architecture
- **Date:** 2026-09-16
- **Extends:** ADR-0024; version-1 contracts remain unchanged

## Context

Read-only checks against Resume-as-Code and Fathom both fail only their whole-catalog
revision, while their selected artifacts, module declarations and routers pass.
Fathom already required a prospective catalog re-selection in the navigation upgrade.
This fires ADR-0024's repeated catalog-review trigger. Current failures do not establish
that historical selected records are identical; retain exact old evidence when available.

## Decision

Add explicit adoption manifest version2. Keep the existing selected capability IDs,
profile/layers, artifact/module/router requirements and bounds. Replace
`catalog_revision` with required `capability_revisions`, an array of `{id, sha256}`:
exactly one pin per selected ID, no duplicates or extras. Empty arrays are valid for
artifact/module/router-only adoption. Normalize the pin array by ID, not entry contents.
V1 still requires its exact whole-catalog revision and rejects the v2 field; v2 rejects
the whole-catalog pin. No automatic migration or fallback occurs.

Each digest is SHA-256 of `encoding/json.Marshal` on the ordered typed envelope:
`domain` = `app-kit/catalog-entry`, `version` =1, `catalog_schema_version`, then
`entry` containing the complete validated `catalog.Entry`. Include every metadata field
and preserve its ordered arrays. Changes to selected tasks, dependencies, source anchors,
states, proof/gaps or provenance require review; unrelated entries do not affect the pin.
This is metadata identity, not transitive source, binary, license or runtime assurance.

Add read-only `appkit adoption pins` using the same explicit root/manifest and bounded
reader. Its separate version1 command report contains current pins and complete selected
metadata for review; unknown IDs fail. It never writes a manifest or follows app imports.
Existing inspect/check v1 report bytes and normalized selection hashes remain compatible.
V2 check retains all existing fit, artifact, module, router and read-limit gates. No
rendering, source execution, filesystem writes, network or host registration is added.

## Compatibility and adoption

V2 requires a rebuilt checker; old checkers must refuse it. Upgrade the tool before a
separately authorized manifest migration. Keep v1 when whole-catalog review is desired.
Review the actual selected records and runtime evidence before accepting emitted pins;
if historical bytes are unavailable, call this a prospective selection, not a proven delta.
Do not update either real consumer in this kit-only change or claim its adoption passes.

## Alternatives considered

- Silently ignore v1 revision drift: hides a promised failure and destroys replay.
- One selected-set hash: compact but cannot identify which selected record drifted.
- Runtime-source/transitive dependency pins here: duplicates delivery verification and
  expands a bounded declaration reader; require a separately demonstrated need.

## Revisit when

- A selected metadata change does not invalidate its pin, or unrelated records do.
- Two consumers need transitive capability contracts not represented by explicit IDs.
- A pin is represented as source/runtime proof, or migration silently blesses drift.
- Catalog schema/canonical serialization changes: version the hash contract explicitly.
