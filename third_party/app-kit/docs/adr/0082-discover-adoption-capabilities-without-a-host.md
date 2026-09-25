# ADR-0082: Discover adoption capabilities without an agent host

- **Status:** Accepted
- **Type:** architecture
- **Date:** 2026-09-23
- **Extends:** ADR-0020/0024/0065/0078; existing commands and schemas unchanged

## Context

The CLI can draft an explicit adoption selection but cannot discover its capability
IDs or review exact metadata without MCP or Gallery. Recent host trials include
incomplete workflows, while the embedded catalog is available independently.
Adopters need the same guarded discovery without a running server or model.

## Decision

Add JSON-only `appkit adoption search` and `appkit adoption capability`.
Search projects the existing catalog query, exact filters, ranking and revision-bound
pagination; it defaults to ten summaries and permits at most fifty per page.
Capability requires `--id` and accepts `--expected-catalog-revision`, matching the
draft command's guard spelling. Validate the guard before returning an entry/pin;
on mismatch require fresh discovery and review, never silently drop it.

Use the same embedded CUE catalog, `Search`, `Get` and canonical `PinCapability`
implementation as MCP/Gallery. Search's JSON is the catalog page; exact retrieval
contains `revision`, `entry` and `capability_revision` with unchanged meanings.
No secondary registry, ranking, hash format or third-party source search is added.
Rebuild the binary to refresh its snapshot. A matching metadata revision does not
prove implementation identity, installed host configuration or runtime quality.

Success emits one deterministic JSON value and means metadata retrieved, not an
adoption check passed. All retrieval/usage/cancellation/output errors use exit2;
valid empty searches succeed with an empty items array. Help succeeds separately.
Output is capped at128KiB before writing, including its newline. Prewrite failure
emits no JSON; writer failure may leave partial output and must not be accepted.
Existing inspect/check/pins/router/draft contracts and exit policies remain intact.

These commands read only embedded first-party metadata: no consumer root/manifest,
filesystem discovery, purchased corpus, network, model or project command is needed.
Review exact entries, then use the existing explicit draft/save/check workflow.
MCP remains the agent integration; this CLI path neither changes its protocol nor
supplies licensed sources, documentation bodies or quality-gate execution.

## Alternatives rejected

- Require MCP/Gallery: unnecessary availability dependency for static discovery.
- Copy metadata or implement new ranking: risks disagreement across transports.
- Auto-select and install: obscures consumer ownership and review of proof gaps.

## Revisit when

- CLI/MCP/Gallery queries or canonical pins disagree for identical catalog inputs.
- A named consumer cannot complete metadata discovery within these output bounds.
- Two adopters need another existing read-only catalog projection through the CLI.
