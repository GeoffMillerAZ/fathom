# MCP Tools Reference

Fathom exposes a JSON-RPC 2.0 MCP server (stdio or HTTP). This page documents every
registered tool, prompt, and resource.

## The surfaces at a glance

| Surface | Tools | Serves |
|---|---|---|
| `/rpc/agent` (the default `fathom init` prints) | 8 | `blast_radius`, `find_symbol`, `obligations`, `read_source`, `review_diff`, `search`, `symbol_context`, `verify_plan` |
| `/rpc/extended` (alias `/rpc/companion`) | 21 | the eight, plus `find_callers`, `findings_near`, `ingest`, `list_services`, `localize`, `outline`, `pack_context`, `query_symbols`, `references`, `repo_map`, `staleness_report`, `status`, `test_impact` |
| `/rpc` (the full registry) | 25 | the 21, plus `compile_layer`, `cross_repo_callers`, `remove_service`, `rename_service` |

A surface is an allowlist for ergonomics, not a security boundary: a call to a tool off
the registered surface still runs and is flagged. The lists come from
`internal/mcp/surface.go`; `TestMcpToolsDocMatchesRegistry` keeps the tables below in step
with the registry.

See [usage.md](usage.md) for how to connect a client and select a surface.
See [architecture.md](architecture.md) for how the tool groups map to the three-layer design.

---

## Where each part lives

This page is the index; the reference is split per group so parallel work adds
files instead of editing one. `TestMcpToolsDocMatchesRegistry` reads this page and every
file below, in this order.

| File | Holds |
|---|---|
| [surfaces](mcp-tools/surfaces.md) | Tool groups, profiles and surfaces: the profile table, the path-scoped HTTP surfaces and the plain HTTP endpoints |
| [protocol](mcp-tools/protocol.md) | The served protocol contract |
| [cost-classes](mcp-tools/cost-classes.md) | The cost-class table (mirrors the registry) |
| [shared](mcp-tools/shared.md) | Shared tools: the group heading and intro |
| [shared-lookup](mcp-tools/shared-lookup.md) | Shared tools: lookup (find_symbol, status, staleness_report) |
| [shared-obligations-a](mcp-tools/shared-obligations-a.md) | Shared tools: obligations, part a (and the `Finding` shape) |
| [shared-obligations-b](mcp-tools/shared-obligations-b.md) | Shared tools: obligations, part b |
| [shared-obligations-c](mcp-tools/shared-obligations-c.md) | Shared tools: obligations, part c |
| [shared-obligations-d](mcp-tools/shared-obligations-d.md) | Shared tools: obligations, part d |
| [shared-obligations-e](mcp-tools/shared-obligations-e.md) | Shared tools: obligations, part e |
| [shared-obligations-f](mcp-tools/shared-obligations-f.md) | Shared tools: obligations, part f |
| [shared-registry](mcp-tools/shared-registry.md) | Shared tools: registry (ingest, remove_service, rename_service) |
| [shared-context](mcp-tools/shared-context.md) | Shared tools: context (repo_map, pack_context) |
| [coding](mcp-tools/coding.md) | Coding tools |
| [investigation](mcp-tools/investigation.md) | Investigation tools |
| [platform](mcp-tools/platform.md) | Platform tools |
| [module-host](mcp-tools/module-host.md) | Module host (no MCP tools) |
| [symbolnode](mcp-tools/symbolnode.md) | The SymbolNode shape |
| [prompts](mcp-tools/prompts.md) | Prompts |
| [resources](mcp-tools/resources.md) | Resources |

---

## Secrets: what a read tool serves (SR-76)

No secret value is stored (LB-F367, ADR-065): a dialect selects no secret-typed field, and the
sentinel test finds the planted value in no row of any store table, full-text tables included. Every
tool that answers from the store therefore answers without one. Two answers also read repository
bytes, and withhold them from a file whose values may be secrets: one holding an entity of a kind a
dialect declares sensitive (every Alertmanager and Grafana provisioning entity, embedded in a
Kubernetes object or not), or a core Kubernetes Secret (`Secret/<name>`, apiVersion `v1`).

- `symbol_context` serves no `snippet` for a definition in such a file, its own kind sensitive or not
  (the Secret or ConfigMap that embeds the configuration too); `snippet_withheld` says why.
- `references` serves an occurrence in such a file with an empty `text`; `text_withheld` says why.
  Its location stays.

Two readers serve repository bytes by design, and are the declared exemptions: `read_source` (the
lines of the file the caller names, as on disk or, with `ref`, as committed; `.env`-pattern files refused) and `search`'s literal
grep mode (no `kind`: the hit lines and their `context` lines, as on disk). A secret committed to the
repository is served by both as committed; no file is filtered. `search` with `kind: "docs"` answers
from the doc index (stored text) and is not exempt. `TestSecretSurface_NoReadToolServesASecret`
asks every tool on `/rpc/extended` about the secret-bearing files and entities and finds the sentinel
(in the clear or base64-encoded) in no answer; a tool added to that surface without an entry fails
`TestSecretSurface_EveryToolHasAnEntry`.
