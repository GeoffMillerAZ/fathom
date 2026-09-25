# ADR-0020: One catalog connects discovery, demonstration and adoption

- **Status:** Accepted
- **Type:** architecture
- **Date:** 2026-09-13
- **Supersedes:** ADR-0012; supplements ADR-0011 and ADR-0015

## Context

The Gallery and Assistant already exist. Commercial retrieval passes known-item tests,
but first-party recipes are not equally discoverable, metadata is incomplete and
consumer guidance differs between harnesses. ADR-0012's common-registry goal remains
right; its requirement that every demonstration wait for the full platform is wrong
for portable and terminal targets (ADR-0017/0018).

## Decision

Keep one appkit-mcp interface. Use one versioned, CUE-validated capability/recipe
catalog for MCP, Gallery and CLI guidance. Each entry has stable identity, applicable
targets/layers, task and exclusions, state fixtures, typed integration/dependencies,
maturity/provenance, consumer evidence and proof references. Exact format is G1 work.

Serve first-party capabilities without any commercial corpus. Licensed libraries are
optional source adapters with explicit source/runtime/version filters. Rank compatible
proven recipes ahead of raw references for a task; show missing capability honestly.
Never disguise a React component as drop-in Go or hide required interaction runtimes.
Search returns bounded IDs/summaries; exact get returns one contract/source set.

Evolve apps/gallery into the living catalog and experience laboratory. Retain live
tokens, components, source, variants, state controls and deterministic isolated previews.
Use working recipes for task demonstrations; label static specimens. Visual regression,
behavior checks and applicable host tests are separate evidence. Generate discovery and
test manifests from the same records; CUE validation may precede elaborate codegen.

The Gallery stays private/local by default. It must work for shipped capabilities
without NATS, a desktop shell, licensed assets or services that the specimen does not use.
Runtime service exercisers are added when their capabilities exist. Preserve ADR-0012's
test/preview/source ambitions and scoped browser-test tooling exception, not its implied
claim that those gates already run. Package tests remain necessary.

Consumer setup uses one local adoption contract and identical short CLAUDE/AGENTS
routers, with host-specific registration verified in each host. CLI, skills and MCP
share facts; they do not hold competing policies. Profiles/evidence are versioned.
Missing required selected artifacts fail checks; unselected capabilities are explicit
not-applicable. Runtime applications do not call MCP.

## Consequences

[Adoption](../design/adoption.md) and [experience-system](../design/experience-system.md)
define the workflow. This change exposes those docs through existing MCP tools/resources;
catalog, audited setup, upgrade automation and browser gates remain implementation goals.
Do not auto-upgrade consumers or claim global host registration from a project file.

## Alternatives rejected

- Separate UI, analytics and adoption MCP servers: duplicate knowledge and discovery cost.
  Wins if a real access-control, deployment or execution boundary requires isolation.
- An independent Storybook/catalog metadata copy: creates competing facts.
- MCP as a mandatory runtime design engine: introduces an unnecessary availability boundary.

## Revisit when

- Catalog/MCP/Gallery identities or contracts diverge in a conformance test.
- An evaluation set repeatedly chooses incompatible/raw references over proven recipes.
- A named remote workflow requires authentication and licensed-source access isolation.
- Registry maintenance costs exceed the duplication it removes; simplify representation.
