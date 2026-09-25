# app-kit

**One person. Many exceptional apps. Every app improves the next.**

A personal product platform: reusable experience capabilities, a living Gallery and
an MCP Assistant. Start with [vision](VISION.md) and [project intent](docs/intent/README.md).
The [assessment](docs/assessments/2026-09-13-app-kit.md) explains where existing adoption
delivers value and where the kit needs to improve.

## What exists

- CUE semantic tokens, deterministic CSS/DTCG/Go generation, portable browser assets.
- Go SVG visualization, dashboard layout, tables, forms, overlays and related
  [UI packages](docs/ui-components.md).
- [CLI-kit](docs/design/cli-kit.md): Cobra/Fang, CUE config, themed output and Bubble Tea.
- [Gallery](apps/gallery/README.md): eleven manually composed specimen routes.
- [Assistant](cmd/appkit-mcp/README.md): commercial source retrieval and kit guidance,
  including intent/adoption docs through tools and resources.
- `appkit vendor` for selected portable artifacts plus drift detection.

Complete recipes, unified first-party catalog, automated adoption receipts and native
desktop runtime remain [active goals](docs/intent/scope.md). The kit does not yet ship
the previously designed app/web/shell/bus/store spine. Sampled consumers still rely on
local replace directives; tagged propagation is not established.

## See it

```sh
task gallery:run
# http://127.0.0.1:8765
```

Run `task gallery:css` after changing source classes. Current pages demonstrate
primitives; the [experience design](docs/design/experience-system.md) defines their
evolution into complete task demonstrations and quality evidence.

## Adopt it

Read [adoption.md](docs/design/adoption.md), select layers/profile and preserve the
consumer's own intent. Portable CSS adoption, Go UI and CLI-kit are all valid paths.
Use `get_kit_guidance` followed by `get_kit_decision("intent")`; fetch only the
applicable documents. First-party APIs are available as `ui-components` and
`cli-kit` document IDs while the searchable recipe catalog is being built.

The existing local MCP startup still requires the TailwindPlus index:

```sh
task twplus:build
task mcp:build
```

See [getting started](docs/getting-started.md) for flags and project registration
examples. Verify each agent host's actual registration; do not assume a single config
file is consumed by every host. Rebuild/restart an existing MCP process to load the
updated server and documentation. This repository change does not update global hosts.

Commercial data is licensed/private. Local verified inventory at the assessment:
657 TailwindPlus components / 10,452 variants and 762 MatDash records. Source dependencies
and license provenance govern adaptation; catalog tags alone do not prove compatibility.

## Develop

```sh
task gen
task check
```

[Roadmap](ROADMAP.md) sequences outcomes; [scope](docs/intent/scope.md) owns exit tests.
[Decision index](docs/adr/README.md) records choices and when to reopen them.
