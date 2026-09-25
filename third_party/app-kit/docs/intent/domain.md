# Domain

## Vocabulary

| Term | Meaning |
|---|---|
| Foundation | Semantic tokens, type/spacing/motion roles, portable assets and typed validation |
| Primitive | A small reusable rendering or interaction capability |
| Recipe | A composed task or screen with behavior, states, integration contract and proof |
| Sub-kit | A cohesive family of capabilities with a user need and a clear boundary, not necessarily a module |
| Consumer profile | The app's chosen layers, renderer/runtime, design direction and constraints |
| Gallery | The running catalog and experience laboratory, built from real kit artifacts |
| Assistant | appkit-mcp, the agent access path to the same knowledge and contracts |
| Commercial reference | TailwindPlus or MatDash source used within the applicable entitlement |
| Proof | Bounded evidence tied to revision, target, task and result |
| Adoption | Selected capability integrated and exercised in a consumer, not merely installed |
| Divergence | A documented decision to use a different solution, with a revisit trigger |
| Conversion | A replayable source-to-target transformation with named supported syntax, preserved semantics and explicit exceptions |
| Target receipt | Revision-bound executed evidence for a particular source, adapter, runtime and target; not a compiler pass or a list of commands |

Layer names L0/L1/L2/L3a/L3b/L4/L5 retain ADR-0017 meanings.
CLI-kit is already implemented under appkit/cli; ADR-0018 defines its contracts.
A renderer, a recipe and a business calculation are different ownership boundaries.

## Actual consumers

[Assessment](../assessments/2026-09-13-app-kit.md) records the sampled source evidence:
Throughline, Resume-as-Code, Growing Gardens BI and Car Assistant use Go UI packages.
ESP Thumper uses a deliberate web token vocabulary and an isolated CLI-kit consumer;
Bosun and Surveyor use CLI-kit, with [explicit reporting/invocation adoption](../assessments/2026-09-16-cli-usage-migration.md).
These examples are not a complete fleet census or live UX audit.

The sample spans operational work, financial analysis, career workflows and constrained
device software. A stack mandate that discards those differences defeats the kit.
Apps own authorization, business formulas, persistence policy and their user journeys.
The kit owns reusable presentation mechanics and the evidence needed to trust them.

## Domain truths

- A chart requires a question, units and valid comparisons; a rendering function cannot
  infer them. Zero, unavailable, stale, partial and estimated are different states.
- Runtime data volumes change the correct rendering strategy. SVG is useful but not an
  unlimited-data promise; query reduction, aggregation or a bounded client island may win.
- Theme tokens make colors portable; computed contrast, hierarchy and information
  architecture still require real rendering and judgment.
- A Go import or CSS link proves dependency use, not discovery, quality or benefit.
- Typed code is not deterministic by itself; control clocks, randomness, ordering,
  external inputs and numeric semantics. Wasm is not automatically faster than browser JS.
- Static generation and request-time rendering can share templ components. A heavy
  client's local state/DOM boundary may be legitimate; ADR-0023 replaces server-only rules.
- Shared source, generated artifacts and instructions have different update mechanisms.
  Reproducibility needs the actual revision and integrity, not just a module name.
- Desktop webviews and browsers share much, but lifecycle, native access, packaging and
  accessibility must be checked on each claimed host. Native mobile adds its own proof.
- Commercial pack purchases are not a blanket redistribution right. The specific
  vendor/channel/tier and intended output govern; provenance survives adaptation.

## Assumptions with consequences

| Assumption | If false | Revisit when |
|---|---|---|
| One owner and coordinated fleet permit one module/release stream | split only the independently evolving dependency surface | release cadence or dependency cost demonstrably blocks a consumer |
| Existing Go and artifact consumers are representative first references | choose additional reference workflows before generalizing | a new product class cannot use a proven recipe without overriding its central behavior |
| Go hypermedia suits most new operational workflows | select a measured alternative profile | interaction/offline/latency or CSP requirements fail its acceptance test |
| Wails can serve the desired desktop adapter | retain browser delivery; evaluate another shell | target conformance fails or upstream support cannot resolve a shipping blocker |
| Extra design effort yields reusable gains | reduce or retire the abstraction | no measurable task benefit or reuse after the bounded incubation |

External platform facts are dated in the assessment; dependency pins remain in source.

Chromium152 with Playwright script execution disabled still resolves font readiness
but does not run the tested RAF/timer measurement callbacks ([P01 control](../assessments/2026-09-16-hero-performance.md)).
Instrument execution and application scripts are separate claims: keep CSP/input
checks and bounded awaits; no-script task proof needs its own appropriate instrument.

Font-loading eligibility is not glyph fidelity: `FontFaceSet.check()` considers
matching declared faces, including unused fallbacks, and does not prove glyph
coverage. The [P01 font fixture](../assessments/fixtures/hero-adapter/FONT.md) separates
that diagnostic from settled layout, actual rendered fonts and exact asset bodies.
