# ADR-0043: Persist historical evidence with one shared private catalog

- **Status:** Partly superseded by 0055 (Gallery image access only)
- **Type:** architecture
- **Date:** 2026-09-14
- **Extends:** ADR-0020, ADR-0023 and ADR-0042; existing source/translation IDs remain unchanged

## Context

The integrity checker and P06 semantic task have retained evidence, but temporary
directories and prose cannot supply durable exact records to Gallery and MCP. The
existing private design-assets root already carries originals and saved translations.
A hash-valid receipt must not acquire certification simply because it is saved.

## Decision

Add an internal historical evidence store under the selected private root's `evidence/`.
An immutable canonical bundle retains contract, receipt, declared source member and
all bounded supplied artifact bytes. Its exact digest determines its `evidence@` ID.
A bounded CUE-authored index supplies detached, deterministic paginated metadata.

Explicit CLI saving requires independently selected contract/source/artifact hashes.
Before writing, run ADR-0042 integrity checks and verify the source archive/member.
Use real anchored directories, bounded regular-file reads, private file modes,
exclusive writer admission, no-clobber immutable publication and index replacement
last. Reuse the saved-translation store's publication pattern, not its semantic schema.
An abandoned lock requires operator review. No automatic repair, overwrite or deletion.

Exact reads recheck bundle identity, canonical metadata, declared artifact bytes,
index agreement and retained original membership. Stored expected hashes are historical
pins, not independently selected current revisions. Reads do not execute a report,
replay today's transformer, authenticate review or establish latest-vendor freshness.
Metadata search records prior integrity results; it does not freshly verify bundles.

CLI, read-only MCP tools and local Gallery routes share the same store engine and
return the same IDs, revision, records and incomplete gates. No bundle/source byte
download or save operation is added to MCP/Gallery. Existing private origin restrictions
apply. Missing optional evidence is empty; damaged configured state remains an error.
Original retrieval and source-free first-party discovery stay independently usable.

All records remain `not-verified`, even with nine claimed passes. Show recorded gate
claims separately from fresh historical-integrity checks. Semantic findings, full-gate
admission and eventual promotion require separately reviewed evidence integration;
the P06 fact checker is not implicitly executed or certified by this store.

## Consequences and alternatives

Retaining bytes costs private disk space and requires backups. Bounded base64 JSON
bundles are simple to inspect and hash but are not an efficient large-media database.
The owner controls the root; hostile same-user races, ACL audits and power-loss
durability are not claimed. No public runtime API, dependency or consumer migration.
Temporary-path-only references lose evidence when cleaned; a database/blob service
would add operational cost before the bounded private workflow demonstrates need.

## Revisit when

- Two candidates exceed bundle/index limits or measured read costs impede inspection.
- A stale or forged record is advertised as current target compatibility.
- A real consumer completes all applicable gates and needs a promotion history.
- Concurrent multi-user access, authenticated review or crash-durable storage is required.
