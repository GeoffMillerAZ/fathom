# app-kit project intent

app-kit makes each application better to use and the next application easier to build.
This is the root router for all kit targets. Practice: agentic-kit's
[Project Intent](../../../agentic-kit/docs/project-intent/README.md).
Existing ADRs retain their home; ADR means any durable decision record.

## Docs map

| Question | Read |
|---|---|
| Mission, experience, ambition and success | [VISION.md](../../VISION.md) |
| Why one repo, sub-kits, membership and dependency direction | [repo.md](repo.md) |
| Tradeoffs and design judgment | [principles.md](principles.md) |
| Binding rules and actual enforcement | [invariants.md](invariants.md) |
| Vocabulary, consumers and assumptions | [domain.md](domain.md) |
| Active goals, compatibility and non-goals | [scope.md](scope.md) |
| Options that are not current implementation work | [futures.md](futures.md) |
| Decisions and observable revisit triggers | [ADR index](../adr/README.md) |
| Agent authority and branch model | [autonomy.md](autonomy.md) |
| Failures and durable outcomes | [incidents.md](incidents.md) |
| Unresolved decisions and their evidence needs | [questions.md](questions.md) |
| Experience recipes, analytics standards and shared catalog design | [experience-system.md](../design/experience-system.md) |
| Personal-site themes, visual identity and app-owned style tools | [theme-workbench.md](../design/theme-workbench.md), [ADR-0029](../adr/0029-named-themes-and-app-owned-style-guides.md) |
| Local app-owned token editing, preview and snapshot export | [app-owned-styleguide.md](../design/app-owned-styleguide.md), [ADR-0031](../adr/0031-local-app-owned-theme-review.md) |
| Profile/settings/help composition and synthetic recovery workflow | [application-shell.md](../design/application-shell.md) |
| Stabilization/resumption, consumer setup, proof and contribution loop | [current priority and resume packets](../design/stabilization-and-resumption.md), [adoption](../design/adoption.md), [static contract/check](../design/adoption-contract.md), [return improvements](../design/contribution-loop.md) |
| Fathom guards, semantic admission, configuration and measured limits | [maintenance](../design/fathom-maintenance.md) |
| Source and browser baseline, including limitations | [assessment](../assessments/2026-09-13-app-kit.md) |
| Purchased source, import and design context | [reference guide](../design/private-references.md), [audit](../assessments/2026-09-13-paid-design-assets.md), [ADR-0022](../adr/0022-private-design-references-and-fidelity.md) |
| Deterministic conversions, agent repairs and web/client fit | [conversion pipeline](../design/conversion-pipeline.md), [runtime selection](../design/runtime-selection.md), [ADR-0023](../adr/0023-profile-specific-conversion-and-client-compute.md) |
| Plain-file reading/FAQ delivery, hosting prefix and error recovery | [static reading guide](../design/static-content-delivery.md), [ADR-0071](../adr/0071-deliver-a-bounded-static-reading-reference.md) |
| Core-first scope, standards adoption and on-demand enhancement | [JIT workflow](../design/just-in-time-conversion.md), [converted-core-v1](../design/converted-component-standard.md), [ADR-0069](../adr/0069-prioritize-starter-patterns-and-just-in-time-conversion.md) |
| Complete native Catalyst settings pilot and source/reference boundaries | [P10 composition](../design/catalyst-settings-adapter.md), [ADR-0044](../adr/0044-native-catalyst-settings-composition.md) |
| Pinned native-host experiment and actual evidence limits | [Wails Gallery](../../spikes/wails-gallery/README.md), [ADR-0026](../adr/0026-isolated-http-desktop-experiment.md), [HTTP-only patch](../adr/0035-reproducible-macos-http-only-experiment.md), [media denial](../adr/0046-native-media-denial-in-http-only-profile.md), [process recovery](../adr/0047-process-owned-http-and-native-recovery.md) |
| Browser-first Wails workflows, topology and native exceptions | [parity contract](../design/web-desktop-parity.md), [ADR-0033](../adr/0033-web-first-workflows-and-native-enhancements.md) |
| Implementation sequence / changes for consumers | [ROADMAP.md](../../ROADMAP.md) / [CHANGELOG.md](../../CHANGELOG.md) |
| Existing package APIs / terminal conventions | [UI components](../ui-components.md) / [cli-kit](../design/cli-kit.md) |

## Routing

| Task | Read first |
|---|---|
| Typo, isolated test repair, no behavior change | nothing |
| Implementation on a sensitive surface | invariants, ADR index, relevant evidence row below |
| UI feature, dashboard, navigation or reusable recipe | scope, principles, experience-system, matching ADRs |
| Consumer adoption, agent/MCP guidance, upgrade | adoption, [agent setup](../design/agent-setup.md), scope, invariants; consumer's own intent |
| New sub-kit, renderer, runtime or architecture | vision, repo, domain, scope, futures, relevant ADRs in full |
| Source conversion, static export or client-heavy UI | conversion-pipeline, runtime-selection, invariants, ADR-0023 |
| CLI/TUI | cli-kit, invariants, ADR-0018, experience-system terminal section |
| Strategy, roadmap, continuation or release readiness | all root intent; stabilization-and-resumption; assessment as dated evidence |
| Commit, push, tag, deployment or real user data | autonomy |

## Sensitive surfaces

| Surface | Why | Done needs |
|---|---|---|
| Tokens/themes: `cue/tokens/**`, `appkit/ui/tokens/**`, `appkit/ui/themes/**`, `appkit/cli/theme/**` | changes every consumer | task check; real theme/contrast review for affected marks |
| UI/scripts/recipes: `appkit/ui/**`, `apps/gallery/**`, `apps/styleguide/**`, `apps/receipts/**` | visible and interaction contracts | package tests, representative browser task, keyboard/narrow/theme states |
| Analytics: `appkit/ui/analytics/**`, `appkit/ui/viz*/**`, `apps/gallery/analytics*`, `apps/receipts/**` | a polished wrong answer is still wrong | fixture with expected values, units/time/missingness, chart/table/export agreement |
| MCP: `cmd/appkit-mcp/**` | steer agents across projects | protocol roundtrip, current document identity, no unavailable feature claims |
| Distribution: `internal/vendor/**`, `cmd/appkit/vendor*`, `go.mod`, `<consumer>/kit.lock` | propagation and provenance | deterministic generation/drift check; clean consumer rebuild without sibling checkout |
| CLI/TUI: `appkit/cli/**`, `cmd/appkit/**` | scripts and humans share one binary | exit/output/config tests; terminal input, resize and cancellation evidence when touched |
| Desktop/streaming: `spikes/wails-gallery/**`, `apps/gallery/shell*` | host, security and reconnect behavior | ADR-0021 conformance matrix on actual claimed targets; failures cannot be skips |
| Guidance: `AGENTS.md`, `CLAUDE.md`, `docs/intent/**`, `docs/adr/**`, `fathom-pack/**`, `.fathom/**` | policy can drift or overreach | link/budget/index/stanza checks; source-exclusion canary; host invocation evidence separately |
| Licensed sources/evidence: `internal/designassets/**`, `internal/designconvert/**`, `internal/designattempts/**`, `internal/designreceipts/**`, `internal/designevidence/**`, `data/**` | access, integrity and redistribution boundaries | provenance/byte checks; no licensed data in distributable outputs; recorded claims are not certification |
| Plans: `docs/plans/*.md` | a checkmark is not proof | fathom verify --service app-kit --plan docs/plans/<plan>.md |

## Before designing and when to update

State applicable invariants, decisions, non-goals and fired Revisit when / Promote when
conditions. Build the present requirement; preserve documented options only when free.
An accepted design is not shipped behavior. Tests are not evidence of host adoption.
Update the one owning document in the same change when a decision, assumption, scope
or enforcement claim changes. Close failures with a test, invariant or decision.
Keep router ≤80 lines, living docs ≤150, new ADRs ≤80; inherited long ADRs are historical debt.

Host/setup evidence: [2026-09-21 assessment](../assessments/2026-09-21-mcp-adoption-setup.md).
Tools observed 2026-09-21: Claude Code 2.1.278; Codex CLI 0.155.1.
Installed versions do not prove either host loaded instructions; adoption.md defines that check.
