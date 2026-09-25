# ADR-0024: Check explicit adoption without inventing runtime evidence

- **Status:** Accepted
- **Type:** architecture
- **Date:** 2026-09-13
- **Extends:** ADR-0020; existing vendor and CLI contracts remain unchanged

## Context

Car Assistant and Growing Gardens BI use the sibling app-kit checkout, generated
app-owned CSS and imported interaction scripts. Neither has a machine-readable
selection record; their CLAUDE/AGENTS routing is uneven. A missing kit.lock cannot
mean failed adoption: neither uses the portable vendor channel for these scripts.
File hashes, module declarations and instruction text establish different facts
from browser behavior, effective module resolution, host invocation or a clean build.

## Decision

Add a consumer-owned appkit.adoption.json contract, schema version 1, validated by
an embedded CUE schema. It names the consumer, selected catalog profile/layers and
capability IDs, catalog revision, human contract document, required artifact hashes,
optional Go module requirement and optional paired agent routing. It contains no
executable commands, secrets, inferred selections or claimed performance results.
One contract describes one selected profile; separate explicitly selected contracts
can describe distinct app surfaces. No unselected runtime becomes a requirement.

Use a shared internal reader/checker and thin `appkit adoption` CLI. Inspection
reports declarations without selecting capabilities. Checking validates the explicit
contract against bounded, root-confined regular files and the embedded catalog.
Missing or drifted required files fail; unavailable reads remain distinguishable.
Module inspection parses declarations only, does not follow local replacements,
and never launches Go, downloads dependencies or resolves ambient workspaces.
Local replacements may be explicitly allowed for development, never called delivery
proof. Artifact hashes prove agreement with the authored contract, not kit origin,
regeneration, freshness or licensing. Existing kit.lock generation/checking stays intact.

An optional agent selection requires the same generated short, marked router in
CLAUDE.md and AGENTS.md, pointing at the consumer-owned human contract. The CLI can
print this block for review, never install it. Text parity is not host execution.
JSON reports are versioned and deterministic for identical inputs; human diagnostics
reuse cli-kit output. New commands use 0 = scoped check passed, 1 = checked mismatch,
2 = usage or unable to complete checking. Inspection success is not adoption success.

No filesystem writes, automatic upgrades, host registration, shell execution, fleet
scans or telemetry occur. Clean builds, propagation receipts, fresh host sessions and
runtime/visual/task evidence remain independent G4 requirements. New surfaces are
additive; existing commands, lock formats and consumer instruction files are unchanged.

## Alternatives considered

- Infer selected layers from imports: misses build tools, aliases and legitimate
  divergences. Observations inform review; consumer intent owns selection.
- Extend kit.lock into an adoption database: conflates portable generated provenance
  with imported Go and vocabulary-only use. Wins if its actual channel needs new metadata.
- Execute proof commands while inspecting: expands a read-only operation into arbitrary
  code and downloads. Explicit bounded build/host verification is a separate workflow.

## Revisit when

- Two real consumers cannot describe a selected capability without contradictory fields.
- A checker passes while a declared required artifact or router is absent or drifted.
- Catalog changes repeatedly force review of unrelated selections; evaluate per-entry pins.
- A clean build disagrees with module observations; preserve declarations versus resolution.
