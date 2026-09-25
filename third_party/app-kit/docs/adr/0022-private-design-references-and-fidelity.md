# ADR-0022: Preserve complete private design references and adaptation fidelity

- **Status:** Accepted
- **Type:** data
- **Date:** 2026-09-13
- **Extends:** ADR-0010 and ADR-0020

## Context

The owner's [purchase audit](../assessments/2026-09-13-paid-design-assets.md) found
faithful snippet indexing but missing whole projects, documentation and preview
dependencies. Mechanical token retargeting can discard hierarchy, contrast and
interaction-state differences. The owner supplied 17 official ZIPs and confirmed
app-kit is private personal infrastructure, not a public catalog product.

## Decision

Keep official source archives byte-for-byte in an ignored private asset store with
SHA-256 identity. Import validates ZIP structure, CRCs, resource bounds and a CUE-owned
manifest before publishing a new catalog snapshot. Preserve old revisions and original
downloads. Read bounded source members on demand; importing never extracts, executes,
installs dependencies or uploads source. Git is not the backup for ignored assets.

Keep full-project identity, source URL, file classifications, manifests/dependencies,
README/license pointers and content hashes discoverable through the existing Assistant
and Gallery. Native capabilities and licensed references retain separate identities
and maturity claims. No source is embedded into a distributable kit by this import.
The existing snippet pipeline stays intact; current ZIPs do not replace its older
Tailwind and appearance variants. Their coverage differences must be explicit.

Separate visual-reference selection from renderer selection. Before material UI work,
choose relevant complete-page examples and state what composition and interaction
qualities to preserve. A native primitive can implement a paid reference's design;
finding that primitive must not bypass the reference review. Do not infer semantic
meaning from a palette family or automatically erase dark/hover/contrast distinctions.
Retargeting is advisory until contextual mapping and rendered checks justify changes.

Use official documentation formats before custom scraping. Add targeted, revisioned
visual/interaction capture where it supplies missing evidence; avoid duplicate broad
source scrapes. Runtime/assets/fonts must be pinned and rights considered before a
preview is described as reproducible or offline. Source presence proves neither.

## Consequences

Complete paid designs become usable references without forcing every app to React or
porting every template to Go. Source-aware discovery and visual comparisons still need
implementation and proof. Local assets need a separate backup and explicit refresh;
private data may be absent without disabling first-party guidance.

The public-catalog future is dropped because its Drop if condition fired. This does
not weaken provenance controls or grant redistribution rights. No new remote server,
public hosting, automatic downloader, dependency installation or host registration is
authorized by this record. Existing profile/security decisions continue to apply.

## Alternatives rejected

- Another MCP per source: duplicates routing and metadata. Wins if a real isolated
  access or deployment boundary appears (ADR-0020).
- Extract every ZIP into the source tree: duplicates source and increases accidental
  execution/export risk. Materialize only for a selected runnable reference workspace.
- More snippets as the quality metric: does not prove preservation of design value.

## Revisit when

- An archive/source schema changes or import rejects an official package.
- A selected reference cannot be reconstructed from its recorded source/dependencies.
- Two audited UI tasks skip reference review despite the discovery guidance.
- The owner changes the private audience or asks for distribution of licensed material.
