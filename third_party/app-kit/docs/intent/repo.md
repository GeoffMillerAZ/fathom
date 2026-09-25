# Repository charter

## Why one repository

Tokens, renderers, recipes, guidance and the Gallery must evolve as one reviewable
change. One release should carry the implementation, its proof and its adoption notes.
Splitting those now would create the drift this kit exists to remove.

## Membership

A capability belongs when a current consumer needs it or the owner admits a bounded
experience experiment with an exit test. Permanent promotion follows ADR-0017/0018.
App business models, proprietary customer data and project-specific screens stay in apps.
Licensed source is a private input, not an independently redistributable kit product.

## Component map

Paths are repository-relative. All components currently inherit root intent; their
existing design guides carry local detail. Different folders alone do not earn routers.

| Component | Kind | Intent | Consumers | Evolution |
|---|---|---|---|---|
| cue/tokens, appkit/ui/tokens | contract | root | every selected target | stays |
| appkit/ui, excluding tokens | library | root | Go and portable-asset consumers | stays |
| appkit/cli | kit | root | Bosun, thumperctl | split when: independent cadence/dependency constraints are measured |
| appkit/catalog | contract/library | root | Gallery, terminal reference and MCP | stays |
| cmd/appkit, internal/adoption | tooling | root | generators, consumer builds, static adoption checks | stays |
| cmd/appkit-mcp | tooling | root | coding agents | split when: an access/deployment boundary requires it |
| apps/gallery | example | root | owner, agents, verification | stays |
| apps/terminal | example | root | owner, G5 bounded interaction evidence | stays |
| apps/receipts | example | root | owner, synthetic G3/P11 task evidence | stays app-owned; no financial backend or shared chart engine |
| internal/twplus, internal/matdash | tooling | root | private reference catalog | split when: provenance or dependency isolation needs a separate distribution |
| docs, fathom-pack | contract | root | agents and consumers | stays |
| spikes | example | root | evidence for admitted bets | retire when: superseded proof is retained in the owning record |

Terminal experiments stay bounded; shared widgets need repeated consumer demand before promotion.
App-shell and analytics sub-kits are accepted capability groupings, initially composed
from existing UI packages and recipes. No directory move or nested module is required.
A desktop adapter is accepted design, not an existing appkit/shell implementation.
The unpublished `spikes/wails-gallery` module isolates Wails/native build dependencies
under [ADR-0026](../adr/0026-isolated-http-desktop-experiment.md); it has no separate release stream.
The shared catalog exposes implemented primitives and experimental recipes through Gallery
and MCP. This coverage does not claim every API, completed recipes or G1–G6 proof.

## Dependency direction

CUE/token contracts → generated assets/palettes → target primitives → composed recipes
→ Gallery and app integration. The Assistant reads metadata/docs and references source;
runtime apps do not call MCP to render. Domain apps never become dependencies of the kit.
The catalog's CUE records feed both Gallery and MCP; optional private source adapters do
not gate first-party discovery. Catalog consumer names record history, not new adoption.
Current package boundaries provide some separation; no complete import-direction lint
exists. State that explicitly until a deterministic control is introduced.

## Evolution procedure

Add a named need and catalog contract with a fixture. Demonstrate a recipe in a real
consumer before promoting an opinionated permanent component. Extract only after an
observable split trigger; define the new release and migration surface in an ADR.
Retire unused capabilities under ADR-0017's two-release rule with consumer checks.

## Ecosystem

agentic-kit supplies the intent practice and separate agent governance capabilities.
fathom supplies code/evidence navigation and optional rule delivery, not UI taste.
app-kit supplies experience capabilities; consumers retain their own intent and authority.
The commercial catalogs supply licensed design references; Charm, Go, CUE and renderer
dependencies supply implementation capabilities. Record their versions and boundaries
without importing their product policies into consumer instructions.
