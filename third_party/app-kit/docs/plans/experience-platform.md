# Experience platform execution

Owner approved: 2026-09-13. Authority: [active outcomes G1–G6](../intent/scope.md),
[experience contracts](../design/experience-system.md), ADR-0019/0020/0021.
This work order records implementation evidence, not a competing product backlog.
An implementation checkbox does not satisfy the broader goal's consumer/host exit test.

## Catalog and discovery (G1)

- [~] Validate first-party records and their required evidence fields with CUE; deterministic revisions and detached snapshots {id: EXP-1, symbol: Load, package: appkit/catalog}
- [~] Bound task/profile/layer discovery with deterministic ranking and revision-bound pagination; reject invalid filters and stale cursors {id: EXP-2, symbol: Catalog.Search, package: appkit/catalog}
- [~] Serve identical first-party contracts through exact MCP tools/resources and Gallery JSON without commercial corpora {id: EXP-3, symbol: registerCatalogTools, package: cmd/appkit-mcp}
- [~] Make the Gallery task-first with stable capability pages, dependencies, maturity and known gaps; preserve filter/theme context {id: EXP-4, symbol: registerCatalog, package: apps/gallery}
- [~] Derive MatDash Tailwind metadata from source manifests and refuse unprovable versions {id: EXP-5, symbol: Walk, package: internal/matdash}

Additional G1 acceptance: fixed discovery evaluation, source/preview inventory canaries,
browser keyboard/narrow/theme/error paths, commercial unavailability and migration notes.
Record measurements rather than interpreting a metadata count as task quality.

## Shared shell and appearance (G2)

- [~] Extract safe typed navigation with route-boundary matching and exactly one current item {id: EXP-6, symbol: RenderNav, package: appkit/ui/dashboard}

Next contracts are grounded when their implementation symbols exist: appearance lifecycle,
functional isolated shell reference, Resume-as-Code/Fathom integrations, profile/settings/help paths and
failure recovery. Preserve consumer auth, identity, persistence keys and chart redraws.

## Remaining approved outcomes

G3: shared analytical semantics and complete filter/detail/export journey, RAC canary,
two real consumer integrations and recorded task/performance comparisons.

G4: selected-artifact profile/doctor, host invocation proof, clean-checkout reproducibility
and a reviewed second-consumer upgrade with release-candidate notes.

G5: bounded Charm keyboard/mouse reference and two CLI integrations with terminal restoration
and stable machine-output evidence.

G6: pinned Wails candidate with real web/macOS workflow/lifecycle evidence. Windows is
deferred by the owner under ADR-0027; mobile remains a future. ADR-0033 and the
[web/native parity contract](../design/web-desktop-parity.md) require independently usable
browser tasks and explicit native exceptions. A cross-compile or skipped host test is not a target pass.

## Verification

Run `fathom verify --service app-kit --plan docs/plans/experience-platform.md` before
claiming any checkbox. Contract reachability supplements actual Go/CUE tests and runtime
observations; it does not prove the broad prose acceptance on its own. No `skip_test`
waivers are planned. Record unresolved target or authority limits explicitly.
