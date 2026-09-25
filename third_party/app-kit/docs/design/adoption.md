# Adopting app-kit and returning improvements

Accepted process: ADR-0020/0024/0045. First-party discovery, documentation routing and
[read-only adoption inspection/checking](adoption-contract.md) are implemented.
The [G4 reconciliation](../assessments/2026-09-16-propagation-reconciliation.md#acceptance-audit-original-checklist-proven-extensions-still-open) proves the original adoption/propagation checklist; admitted theme-guide/core/JIT acceptance remains open. Profile generation remains unimplemented, and delivered-release proof and broader upgrade receipts remain outstanding; [inventory](../assessments/2026-09-13-adoption-inventory.md) records replacements/deferrals.
The [CA rehearsal](../assessments/2026-09-13-clean-consumer-build.md) proves repeat private-artifact
builds without sibling-source/network access and generated/served CSS canaries, not a release or migration.
Active consumer receipts: [RAC Today](../assessments/2026-09-13-resume-today-uplift.md), [Pipeline/People](../assessments/2026-09-14-resume-pipeline-uplift.md) and
[Fathom frame](../assessments/2026-09-13-fathom-observatory-uplift.md), including workspace/mirror divergence; neither completes G4.

## Choose the app's contract

Read the consumer's own intent first. Record selected layers, profile, sub-kit capabilities,
app-specific visual direction, constraints, kit revision and a representative user task.
Use [runtime selection](runtime-selection.md) for static, operational and client-heavy
tradeoffs. Originals and conversions remain explicit alternatives; a renderer change
requires a fitting profile, not silent fallback. [Conversion proof](conversion-pipeline.md)
is accepted work; current tool/source availability does not certify a converted target.
Use the [JIT enhancement workflow](just-in-time-conversion.md) and [converted-core-v1](converted-component-standard.md) when a selected component has gaps; update its CUE standards/gate/adoption status and return the verified improvement, without modifying imported originals or unrelated variants.
Keep human design context in docs/app-kit.md; appkit.adoption.json binds the explicit
profile/layers, whole-catalog or selected-metadata pins, artifacts and optional module/router checks.
For team/offline use, keep a bounded versioned documentation snapshot or resolvable module
reference; a sibling checkout or personal skill symlink is not a reproducible dependency.

| Profile | Existing path | Evidence needed |
|---|---|---|
| Portable web | L0/L1, optional L2 through appkit vendor | declared artifact provenance, theme/behavior checks, kit.lock integrity when generated |
| Go static/operational | L3a SVG, selected L3b/L4; CSS scanner where required | compiled examples, real CSS output, app task and state checks |
| Go streaming | operational profile plus app's pinned HTTP/SSE integration | mutation/reconnect/cancellation/freshness tests; shared runtime not yet shipped |
| CLI/TUI | appkit/cli command/theme/output/config/tui | machine-output and terminal interaction contracts |
| Native desktop | accepted Wails/HTTP adapter design | G6 matrix before calling it supported |

Token reuse without artifact provenance is legitimate; do not invent kit.lock or pass a stale vendor task.

## Available discovery today

Use get_kit_guidance, then get_kit_decision with id=intent. Fetch only the routed
documents, e.g. intent/scope, intent/invariants, adoption, experience-system or cli-kit.
Use search_capabilities(query, profile, layer) → get_capability(id) for the implemented
first-party token/primitive records; kit://catalog exposes their full versioned snapshot.
Gallery /catalog and /catalog/{id} show the same IDs/contracts/revision. JSON endpoints
are /api/catalog, /api/catalog/{id} and /api/catalog/search; search uses q, profile, layer,
kind, maturity, limit and cursor. Limits default to 10 (1–50); retain query/filters/limit
for continuation. Empty compatible results identify a catalog gap, not a promised recipe.
Use ui-components/cli-kit docs and actual symbols for kit APIs outside this initial seed.

Commercial search_components → get_component remains available with an explicit source.
Check framework, source Tailwind major and dependencies; MatDash now preserves its source
major (local manifests: ^3.4.16). Rebuild older indexes with appkit data matdash-build.
First-party discovery starts without licensed artifacts. Missing discovered artifacts
degrade with a diagnostic; broken explicit flag/env paths fail with a repair hint.
Docs load at startup: refresh the affected server within approved scope and check exact bytes; embedded catalog changes also need a rebuild.
Neither an MCP connection nor catalog consumer history proves a new adoption or host run.

## Agent routing in each consumer

Print `appkit adoption router --contract docs/app-kit.md` and review the same block
for both CLAUDE.md and AGENTS.md. Preserve unrelated consumer policy.

```markdown
<!-- appkit:begin -->
## App-kit

App-kit adoption lives at docs/app-kit.md: selected layers/profile, design context,
revision, proof commands and divergences. Before material UI/CLI work, consult it and
the kit's intent router through appkit MCP or the pinned local docs. Select a fitting
capability/recipe; verify the actual task and states in the target UI. Record reusable
improvements or a reasoned divergence and return applicable fixes upstream.
<!-- appkit:end -->
```

The router imports neither global authority nor unselected runtime requirements.
Task skills may guide discovery, integration, review and upgrade through these contracts;
do not create four policy copies before a distinct skill need is demonstrated.

## Registration and host proof

Follow [agent setup](agent-setup.md) for the supported build, registration and protocol
probe. Claude Code uses `.mcp.json` for shared project registration; Codex uses
`config.toml`. Preserve existing entries; global configuration requires scoped authority.

Verify Claude Code and Codex separately in a fresh session: discover appkit, fetch the
intent router and one chosen recipe/component contract, then retrieve the real source.
Capture host/version, server build/revision, document identity, query and result.
If only one host works, report that coverage. Installed/configured/invoked are different.
The [fresh synthetic trial](../assessments/2026-09-13-agent-host-routing.md) records
successful Codex/Claude contract and source retrieval at one frozen revision. Automatic
CLAUDE loading remains inferred; real consumer registration and UI proof remain separate.

New automation should generate reviewed project-scoped changes, check collisions and
working-tree integrity, and test missing artifacts or stale profiles as failures.
Do not auto-pull policy on session start or silently overwrite consumer choices.

## Delivery workflow

1. **Understand:** define a user task, target scene, data, constraints, design direction
   and success criteria from consumer intent.
2. **Discover:** choose complete visual references and compatible first-party
   capabilities separately. Inspect original composition, states and dependencies,
   exact integration contracts and evidence gaps. Record what to preserve and exclusions.
3. **Integrate:** compose through real APIs and app-owned seams. Verify L3a versus L3b
   CSS requirements; do not copy shared implementation into the app.
4. **Prove:** compile and run the task with representative data; exercise keyboard,
   narrow viewports, themes, error/retry, stale/permission states as relevant; measure
   performance against declared budgets and compare the adapted design with its reference.
5. **Return:** file a reusable improvement or divergence with source evidence and an
   observable revisit condition. Promote at the lowest layer under ADR-0017/0018.
6. **Propagate:** prepare a versioned upgrade and migration note; verify another
   applicable consumer receives the behavior and preserves its domain/visual identity.

An adoption record names consumer and kit revisions, selected profile/layers, task,
artifact hashes where relevant, invoked checks and their results, visual/interaction
evidence, known limitations and next disposition. The static selection schema/checker
exists; an automated runtime/upgrade receipt does not. Omit private data from examples.

## Reusable improvement loop

Use [contribution-loop](contribution-loop.md) at discovery and UI/CLI handoff (MCP:
`get_kit_decision("contribution-loop")`). It defines triggers, ownership, dispositions,
the compact return record and consumer intent/agent-guidance upkeep under ADR-0066.

Record: originating app/task → observed problem or successful pattern → smallest
kit change → fixture/contract → upgrade → second consumer result.
A lesson or defect can become a hazard/test immediately. Permanent UI APIs need the
existing promotion evidence. Two divergences for the same reason elevate a kit issue.
The originating app should use the shared fix; leaving its old fork defeats propagation.

CA/GGB already use shared navigation/appearance; further adoption is owner-deferred
under [ADR-0032](../adr/0032-defer-car-assistant-and-growing-gardens-adoption.md). No consumer writes, repeat approval prompts or automatic substitutes.
Resume only an owner-named app/task, refresh the inventory, then earn the unchanged
browser/host/delivery and promotion evidence. Domain policy stays in each app.

## Release and measurement

Local replaces aid co-development. Delivery needs a clean consumer build at an exact
resolvable release/pseudo-version, generated-asset checks and no sibling checkout.
Tags remain the release target; [private preparation](../../tools/releasecandidate/README.md) binds working-tree bytes, not a release.
[ADR-0045](../adr/0045-exact-pins-and-private-candidate-preparation.md) requires reviewed exact pins; unconstrained `go get -u` is not upgrade proof.
Resolve selected utility renderers from the actual module; missing directories fail.
Regenerate an ignored local scanner fragment, never commit machine-specific cache paths.
Use `@import "tailwindcss" source(none)` and explicit sources; unrelated files must not
alter CSS. Check required selectors even when Tailwind exits zero. Compare two isolated
roots with fixed inputs/output locations, then inspect actual wrapping and contrast.
Incremental task inputs must cover every renderer, scanner manifest, theme and compiler
input. Correct scanning alone does not prove rebuild invalidation. Role names do not
guarantee readable text; regeneration must preserve selected font/radius aliases.
Validate the entire CSS entry: an extra import can undo `source(none)`. Refused generation
preserves outputs; per-file replacement is not a multi-file transaction. Test stale output.
Source-copy proofs include ignored scanner-selected files or refuse, disable ancestor
VCS stamping, and distinguish working-tree snapshots from clean Git builds.
[RAC's ledger](../assessments/2026-09-14-resume-assessment-evidence.md) records scanner/cache canaries and browser proof.

Track comparable task success/error/time, app integration effort, escaped defects,
performance, upgrade lag and useful improvements adopted by another consumer.
Import/server/download counts measure availability only.
