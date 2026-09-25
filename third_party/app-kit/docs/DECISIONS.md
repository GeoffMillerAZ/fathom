# Decision register

A compact pointer for existing D-number citations and MCP clients. For status, type and
revisit triggers use the [ADR index](adr/README.md); for implementation use
[intent/scope](intent/scope.md). Historical records remain readable but successors govern.
No dated dependency claim in an old record is a current pin.

| # | Decision / current interpretation | ADR |
|---|---|---|
| D1 | Record durable decisions (Accepted) | [0001](adr/0001-record-architecture-decisions.md) |
| D2 | Go/CUE and reproducible kit builds (Partly superseded by 0017) | [0002](adr/0002-go-and-cue-only-no-node.md) |
| D3 | Original universal hypermedia profile (Superseded by 0021) | [0003](adr/0003-hypermedia-first-datastar.md) |
| D4 | templ for the Go template profile (Accepted) | [0004](adr/0004-templ-templates.md) |
| D5 | Original universal loopback rule (Superseded by 0021) | [0005](adr/0005-loopback-invariant.md) |
| D6 | Original Wails alpha choice (Superseded by 0021) | [0006](adr/0006-wails-v3-desktop-shell.md) |
| D7 | CUE authors contracts and generated truth (Accepted) | [0007](adr/0007-cue-single-source-of-truth.md) |
| D8 | Original mandatory embedded NATS default (Superseded by 0021) | [0008](adr/0008-embedded-nats-spine.md) |
| D9 | SQLite default for selected durable storage (Accepted) | [0009](adr/0009-sqlite-default-store.md) |
| D10 | Commercial corpus pipeline (Accepted) | [0010](adr/0010-component-data-pipeline.md) |
| D11 | One typed MCP assistant (Accepted; extended by 0020) | [0011](adr/0011-mcp-design-assistant.md) |
| D12 | Original full-platform Gallery (Superseded by 0020) | [0012](adr/0012-gallery-living-style-guide.md) |
| D13 | Original one-module propagation policy (Superseded by 0045; 0026 exception retained) | [0013](adr/0013-monorepo-versioning.md) |
| D14 | Local-host trust, secrets and signed updates (Partly superseded by 0021/0026) | [0014](adr/0014-security-baseline.md) |
| D15 | Agent-readable decisions and contracts (Accepted; extended by 0020) | [0015](adr/0015-ai-first-surface.md) |
| D16 | Static-client router carve-out (Superseded by 0017) | [0016](adr/0016-client-view-router.md) |
| D17 | Layered adoption and promotion rules (Partly superseded by 0023) | [0017](adr/0017-multi-target-layered-kit.md) |
| D18 | CLI-kit on Cobra and Charm (Accepted) | [0018](adr/0018-terminal-target-cli-kit.md) |
| D19 | Proven experience outcomes and sub-kits (Accepted) | [0019](adr/0019-experience-outcomes-and-subkits.md) |
| D20 | Shared catalog, Gallery and adoption evidence (Accepted) | [0020](adr/0020-shared-catalog-and-adoption-evidence.md) |
| D21 | Scoped runtime profiles and platform proof (Partly superseded by 0027 for current targets) | [0021](adr/0021-runtime-profiles-and-platform-proof.md) |
| D22 | Private whole-project references and adaptation fidelity (Accepted) | [0022](adr/0022-private-design-references-and-fidelity.md) |
| D23 | Tested conversion pipeline and static/client runtime choices (Accepted) | [0023](adr/0023-profile-specific-conversion-and-client-compute.md) |
| D24 | Explicit read-only adoption contracts (Accepted) | [0024](adr/0024-read-only-adoption-contract.md) |
| D25 | Scoped static chart composition evidence (Accepted) | [0025](adr/0025-static-chart-composition-evidence.md) |
| D26 | Isolated native host and authenticated HTML commands (Accepted) | [0026](adr/0026-isolated-http-desktop-experiment.md) |
| D27 | Web/macOS now; Windows on demonstrated demand (Accepted) | [0027](adr/0027-web-and-macos-now-windows-on-demand.md) |
| D28 | Explicitly prepared private static comparisons (Accepted) | [0028](adr/0028-prepared-private-static-comparisons.md) |
| D29 | Named identities and app-owned living guides (Accepted) | [0029](adr/0029-named-themes-and-app-owned-style-guides.md) |
| D30 | Isolated full-width comparison controls (Accepted) | [0030](adr/0030-isolated-comparison-workspace-controls.md) |
| D31 | Local app-owned token preview and reviewed snapshot export (Accepted) | [0031](adr/0031-local-app-owned-theme-review.md) |
| D32 | Defer Car Assistant and Growing Gardens BI adoption (Accepted; promotion evidence unchanged) | [0032](adr/0032-defer-car-assistant-and-growing-gardens-adoption.md) |
| D33 | First-class browser workflows with explicit native enhancements (Accepted) | [0033](adr/0033-web-first-workflows-and-native-enhancements.md) |
| D34 | Source-specific private adapter export and exact replay check (Accepted) | [0034](adr/0034-source-specific-private-component-delivery.md) |
| D35 | Reproducible opt-in HTTP-only macOS experiment (Accepted) | [0035](adr/0035-reproducible-macos-http-only-experiment.md) |
| D36 | Share private filesystem assurance; keep adapter contracts distinct (Accepted) | [0036](adr/0036-share-private-delivery-assurance.md) |
| D37 | Share internal CSV structure checks; keep report semantics distinct (Accepted) | [0037](adr/0037-share-internal-analytics-csv-sections.md) |
| D38 | Runnable synthetic receipts reference; keep its signed renderer private (Accepted) | [0038](adr/0038-runnable-receipts-reference.md) |
| D39 | Explicit CLI diagnostic acknowledgement; preserve exit classification (Accepted) | [0039](adr/0039-explicit-cli-error-reporting.md) |
| D40 | Explicit runtime-bound Elements syntax family; preserve legacy replay (Accepted) | [0040](adr/0040-explicit-elements-syntax-family.md) |
| D41 | Explicit typed FAQ reference and native disclosures (Accepted) | [0041](adr/0041-explicit-faq-reference-and-native-details.md) |
| D42 | Revision-bound evidence integrity, separate from certification (Accepted) | [0042](adr/0042-separate-evidence-integrity-from-certification.md) |
| D43 | Immutable private historical evidence (Partly superseded by D55 for Gallery images) | [0043](adr/0043-private-historical-evidence-catalog.md) |
| D44 | Native Catalyst settings with explicit app authority (Accepted) | [0044](adr/0044-native-catalyst-settings-composition.md) |
| D45 | Exact reviewed pins and private content-identified candidates (Accepted) | [0045](adr/0045-exact-pins-and-private-candidate-preparation.md) |
| D46 | Deny media capture in the strict macOS HTTP-only profile (Accepted) | [0046](adr/0046-native-media-denial-in-http-only-profile.md) |
| D47 | Process-own HTTP state and isolate native recovery (Accepted) | [0047](adr/0047-process-owned-http-and-native-recovery.md) |
| D48 | Explicit exits and isolated invocation-origin proof (Superseded by 0070) | [0048](adr/0048-prove-cli-invocation-origins-before-adoption.md) |
| D49 | Private menu reference and app-owned actions (Accepted; no auth backend) | [0049](adr/0049-explicit-menu-actions-and-private-reference.md) |
| D50 | Explicit measured menu repair candidate (Accepted; not v1 delivery) | [0050](adr/0050-explicit-menu-geometry-and-focus-repair.md) |
| D51 | Explicit eight-file private menu repair selection (Accepted; base replay unchanged) | [0051](adr/0051-explicit-private-menu-repair-delivery.md) |
| D52 | Default-off candidate invocation origins with nearest-ancestor opt-in (Superseded by 0070) | [0052](adr/0052-default-off-cli-invocation-origins.md) |
| D53 | Private profile drawer with explicit app-owned navigation (Accepted) | [0053](adr/0053-explicit-profile-drawer-and-app-owned-navigation.md) |
| D54 | Explicit CSS-only drawer focus companion (Accepted; base replay unchanged) | [0054](adr/0054-explicit-drawer-focus-delivery.md) |
| D55 | Private retained PNG inspection (Accepted; no live preview or certification) | [0055](adr/0055-private-retained-image-inspection.md) |
| D56 | Preserve per-row analytical meaning; explicit ten-column CSV migration (Accepted) | [0056](adr/0056-preserve-analytics-role-meaning.md) |
| D57 | Private responsive navbar with one app-owned route model (Accepted) | [0057](adr/0057-explicit-responsive-navbar-and-route-identity.md) |
| D58 | Explicit navbar repair and persistent native route directory (Accepted; candidate, not CLI availability) | [0058](adr/0058-explicit-navbar-repair-and-native-route-access.md) |
| D59 | Explicit source-pinned navbar runtime focus guard (Accepted; no automatic replacement) | [0059](adr/0059-guard-selected-navbar-runtime-focus.md) |
| D60 | Explicit nine-file navbar repair and runtime delivery (Accepted; base bytes unchanged) | [0060](adr/0060-deliver-explicit-navbar-runtime-repair.md) |
| D61 | Complete capture histories with64 slots per gate (Accepted; larger packets require new readers) | [0061](adr/0061-bound-larger-historical-capture-sets.md) |
| D62 | Search coherent declared tasks, not explanatory prose (Accepted; restart old cursors) | [0062](adr/0062-search-declared-capability-tasks.md) |
| D63 | Incubate typed account destinations and semantic feedback (Accepted; not permanent promotion) | [0063](adr/0063-incubate-account-presentation-recipe.md) |
| D64 | Incubate responsive settings sections without form authority (Accepted; not permanent promotion) | [0064](adr/0064-incubate-responsive-settings-sections.md) |
| D65 | Opt-in selected-capability metadata pins (Accepted; v1 semantics unchanged, migration separately reviewed) | [0065](adr/0065-pin-selected-adoption-capabilities.md) |
| D66 | Close contribution loops and refresh consumer guidance at handoff (Accepted; no automatic upgrades) | [0066](adr/0066-close-consumer-contribution-loops.md) |
| D67 | Prove isolated warm-cache lineage prospectively (Accepted; v1 failures preserved, no certification) | [0067](adr/0067-prove-isolated-warm-cache-lineage.md) |
| D68 | Private unsigned macOS bundles first (Accepted design direction; no installation/signing authority or delivered proof) | [0068](adr/0068-prepare-private-unsigned-macos-delivery-first.md) |
| D69 | Starter patterns and just-in-time enhancement (Partly superseded by 0072 for immediate completion priority; standards and evidence retained) | [0069](adr/0069-prioritize-starter-patterns-and-just-in-time-conversion.md) |
| D70 | Opt-in CLI invocation classification and consumer-owned dependency patches (Accepted; default execution and operational policy preserved) | [0070](adr/0070-opt-in-cli-invocation-classification.md) |
| D71 | Bounded plain-file reading reference (Accepted; build-only Go, explicit CSS and prefix-aware error recovery) | [0071](adr/0071-deliver-a-bounded-static-reading-reference.md) |
| D72 | Stabilize and resume on demand (Accepted; bounded tasks and budgets, no completion credit for deferral) | [0072](adr/0072-stabilize-and-resume-on-demand.md) |
| D73 | Separate memory metrics from attribution (Accepted; prospective heap budgets, total browser cost reported, no acquired memory pass) | [0073](adr/0073-separate-memory-metrics-from-attribution.md) |
| D74 | Incubate signed analytics (Accepted; one dataset, symmetric scale, old renderers unchanged) | [0074](adr/0074-incubate-signed-analytics-comparisons.md) |
| D75 | Incubate app-owned analytics detail links (Partly superseded by D76 for later layout repair; original link-delta evidence retained) | [0075](adr/0075-incubate-app-owned-analytics-detail-links.md) |
| D76 | Reserve readable missing-state columns (Accepted; independent layout repair, native scroll, unchanged data/CSV) | [0076](adr/0076-preserve-readable-missing-state-columns.md) |
| D77 | Incubate rectangular analytics CSV (Accepted; additive writer, shared encoding, unchanged existing exports) | [0077](adr/0077-incubate-rectangular-analytics-export.md) |
| D78 | Draft explicit adoption selections (Accepted; read-only v2 authoring, no automatic upgrade approval) | [0078](adr/0078-draft-explicit-adoption-selections.md) |
| D79 | Disable unsafe form-button links (Accepted; web-relative/HTTP(S), no implicit submit) | [0079](adr/0079-disable-unsafe-form-button-links.md) |
| D80 | Harden managed table row destinations (Accepted; internal web-reference policy, native target/focus) | [0080](adr/0080-harden-table-row-destinations.md) |
| D81 | Derive one table action column (Accepted; consistent rows, explicit empty-state reservation) | [0081](adr/0081-derive-one-table-action-column.md) |
| D82 | Discover adoption capabilities without a host (Accepted; shared catalog, guarded exact pins, no consumer reads) | [0082](adr/0082-discover-adoption-capabilities-without-a-host.md) |
| D83 | Render missing select values as unavailable prompts (Accepted; no valid fallback or automatic error) | [0083](adr/0083-render-missing-select-values-as-unavailable.md) |

### D1 — Record durable decisions

Accepted. [Record](adr/0001-record-architecture-decisions.md); the ADR index carries the reopen condition.

### D2 — Go/CUE and reproducible kit builds

Partly superseded by 0017. [Record](adr/0002-go-and-cue-only-no-node.md); the ADR index carries the reopen condition.

### D3 — Original universal hypermedia profile

Superseded by 0021. [Record](adr/0003-hypermedia-first-datastar.md); the ADR index carries the reopen condition.

### D4 — templ for the Go template profile

Accepted. [Record](adr/0004-templ-templates.md); the ADR index carries the reopen condition.

### D5 — Original universal loopback rule

Superseded by 0021. [Record](adr/0005-loopback-invariant.md); the ADR index carries the reopen condition.

### D6 — Original Wails alpha choice

Superseded by 0021. [Record](adr/0006-wails-v3-desktop-shell.md); the ADR index carries the reopen condition.

### D7 — CUE authors contracts and generated truth

Accepted. [Record](adr/0007-cue-single-source-of-truth.md); the ADR index carries the reopen condition.

### D8 — Original mandatory embedded NATS default

Superseded by 0021. [Record](adr/0008-embedded-nats-spine.md); the ADR index carries the reopen condition.

### D9 — SQLite default for selected durable storage

Accepted. [Record](adr/0009-sqlite-default-store.md); the ADR index carries the reopen condition.

### D10 — Commercial corpus pipeline

Accepted. [Record](adr/0010-component-data-pipeline.md); the ADR index carries the reopen condition.

### D11 — One typed MCP assistant

Accepted; extended by 0020. [Record](adr/0011-mcp-design-assistant.md); the ADR index carries the reopen condition.

### D12 — Original full-platform Gallery

Superseded by 0020. [Record](adr/0012-gallery-living-style-guide.md); the ADR index carries the reopen condition.

### D13 — One module and versioned propagation

Superseded by [0045](adr/0045-exact-pins-and-private-candidate-preparation.md); the unpublished native exception in 0026 remains. [Historical record](adr/0013-monorepo-versioning.md).

### D14 — Local-host trust, secrets and signed updates

Partly superseded by 0021/0026. [Record](adr/0014-security-baseline.md); the ADR index carries the reopen condition.

### D15 — Agent-readable decisions and contracts

Accepted; extended by 0020. [Record](adr/0015-ai-first-surface.md); the ADR index carries the reopen condition.

### D16 — Static-client router carve-out

Superseded by 0017. [Record](adr/0016-client-view-router.md); the ADR index carries the reopen condition.

### D17 — Layered adoption and promotion rules

Partly superseded by 0023. [Record](adr/0017-multi-target-layered-kit.md); the ADR index carries the reopen condition.

### D18 — CLI-kit on Cobra and Charm

Accepted. [Record](adr/0018-terminal-target-cli-kit.md); the ADR index carries the reopen condition.

### D19 — Proven experience outcomes and sub-kits

Accepted. [Record](adr/0019-experience-outcomes-and-subkits.md); the ADR index carries the reopen condition.

### D20 — Shared catalog, Gallery and adoption evidence

Accepted. [Record](adr/0020-shared-catalog-and-adoption-evidence.md); the ADR index carries the reopen condition.

### D21 — Scoped runtime profiles and platform proof

Partly superseded by 0027 for current targets. [Record](adr/0021-runtime-profiles-and-platform-proof.md); the ADR index carries the reopen condition.

### D22 — Private whole-project references and adaptation fidelity

Accepted. [Record](adr/0022-private-design-references-and-fidelity.md); the ADR index carries the reopen condition.

### D23 — Tested conversion pipeline and static/client runtime choices

Accepted. [Record](adr/0023-profile-specific-conversion-and-client-compute.md); the ADR index carries the reopen condition.

### D24 — Explicit read-only adoption contracts

Accepted. [Record](adr/0024-read-only-adoption-contract.md); the ADR index carries the reopen condition.

### D25 — Scoped static chart composition evidence

Accepted. [Record](adr/0025-static-chart-composition-evidence.md); the ADR index carries the reopen condition.

### D26 — Isolated native host and authenticated HTML commands

Accepted. [Record](adr/0026-isolated-http-desktop-experiment.md); the ADR index carries the reopen condition.

### D27 — Web/macOS now; Windows on demonstrated demand

Accepted. [Record](adr/0027-web-and-macos-now-windows-on-demand.md); the ADR index carries the reopen condition.

### D28 — Explicitly prepared private static comparisons

Accepted. [Record](adr/0028-prepared-private-static-comparisons.md); the ADR index carries the reopen condition.

### D29 — Named identities and app-owned living guides

Accepted. [Record](adr/0029-named-themes-and-app-owned-style-guides.md); the ADR index carries the reopen condition.

### D30 — Isolated full-width comparison controls

Accepted. [Record](adr/0030-isolated-comparison-workspace-controls.md); the ADR index carries the reopen condition.

### D31 — Local app-owned token preview and reviewed snapshot export

Accepted. [Record](adr/0031-local-app-owned-theme-review.md); the ADR index carries the reopen condition.

### D32 — Defer Car Assistant and Growing Gardens BI adoption

Accepted. [Record](adr/0032-defer-car-assistant-and-growing-gardens-adoption.md); only explicit owner resumption selects new app work; promotion evidence remains required.

### D33 — First-class browser workflows with explicit native enhancements

Accepted. [Record](adr/0033-web-first-workflows-and-native-enhancements.md); parity is an outcome contract, not a current platform claim.

### D34 — Source-specific private adapter export and exact replay check

Accepted. [Record](adr/0034-source-specific-private-component-delivery.md); delivery replay is not target certification or adoption.

### D35 — Reproducible opt-in HTTP-only macOS experiment

Accepted. [Record](adr/0035-reproducible-macos-http-only-experiment.md); native refusal receipts do not establish complete platform support.

### D36 — Shared private delivery assurance

Accepted. [Record](adr/0036-share-private-delivery-assurance.md); source-specific replay and inventories remain independent.

### D37 — Shared internal analytics CSV sections

Accepted. [Record](adr/0037-share-internal-analytics-csv-sections.md); shared CSV encoding remains authoritative and consumer metric semantics remain separate.

### D38 — Runnable synthetic receipts reference

Accepted. [Record](adr/0038-runnable-receipts-reference.md); the task model owns its chart and routes, without certifying the original MatDash conversion or promoting a financial backend.

### D39 — Explicit CLI diagnostic acknowledgement

Accepted. [Record](adr/0039-explicit-cli-error-reporting.md); opt-in reporting does not classify usage errors or establish consumer adoption.

### D40 — Explicit Elements syntax family

Accepted. [Record](adr/0040-explicit-elements-syntax-family.md); runtime-bound syntax output does not enter the legacy store or certify a converted target.

### D41 — Explicit FAQ reference and native disclosures

Accepted. [Record](adr/0041-explicit-faq-reference-and-native-details.md); faithful Elements and native disclosures remain explicit independent choices.

### D42 — Revision-bound evidence integrity

Accepted. [Record](adr/0042-separate-evidence-integrity-from-certification.md); matching artifact hashes never promotes a claimed pass to target certification.

### D43 — Private historical evidence catalog

Partly superseded by D55 for Gallery PNG access. [Record](adr/0043-private-historical-evidence-catalog.md); immutable bundles and shared metadata preserve original membership and historical integrity, not current target certification.

### D44 — Native Catalyst settings composition

Accepted. [Record](adr/0044-native-catalyst-settings-composition.md); preserve the complete private form with native controls, responsive local feedback and app-owned submission authority; implementation and target proof remain separate.

### D45 — Exact pins and private candidate preparation

Accepted. [Record](adr/0045-exact-pins-and-private-candidate-preparation.md); one root version stream and the isolated native exception remain. Working-tree candidate identity and standalone rehearsal are not a release or adoption certificate.

### D46 — Native media denial in the HTTP-only profile

Accepted. [Record](adr/0046-native-media-denial-in-http-only-profile.md); the strict macOS profile denies camera and microphone only after native preflight and callback-reaching proof. Other native capabilities remain presence-only and unproven.

### D47 — Process-owned HTTP and native recovery

Accepted. [Record](adr/0047-process-owned-http-and-native-recovery.md); the cgo-free parent owns guarded HTTP state and treats the exact native companion as a bounded, recoverable child. Native task and lifecycle acceptance remain separate evidence.

### D48 — Explicit exits and isolated invocation-origin proof

Superseded by [0070](adr/0070-opt-in-cli-invocation-classification.md). [Historical record](adr/0048-prove-cli-invocation-origins-before-adoption.md); additive introspection preserves explicit exit policy. The original private experiment did not itself adopt dependencies or change consumer usage exits.

### D49 — Explicit menu actions and private reference

Accepted. [Record](adr/0049-explicit-menu-actions-and-private-reference.md); preserve the original separately from typed links, explicit POST/CSRF binding and opt-in usability styles. The application retains all action authority; generation is not certification.

### D50 — Scoped menu geometry and selected-focus repair

Accepted. [Record](adr/0050-explicit-menu-geometry-and-focus-repair.md); an explicit internal companion may bound available space and restore Elements' existing selected item. Browser proof and a reviewed delivery revision precede export; original sources and v1 remain unchanged.

### D51 — Explicit private menu repair delivery

Accepted. [Record](adr/0051-explicit-private-menu-repair-delivery.md); `-repair` selects a distinct nested artifact and eight-file inventory with explicit app-owned mount/disposal. Default JSON and six-file replay remain unchanged; exact delivery is not target certification or consumer adoption.

### D52 — Default-off experimental CLI invocation origins

Superseded by [0070](adr/0070-opt-in-cli-invocation-classification.md). [Historical record](adr/0052-default-off-cli-invocation-origins.md); command-local selection inherits from the nearest explicit ancestor, preserving unselected error identity. Its candidate proof remains distinct from the later authorized opt-in migration.

### D53 — Private profile drawer and app-owned navigation

Accepted. [Record](adr/0053-explicit-profile-drawer-and-app-owned-navigation.md); preserve the constant reference, bind typed identity/image/details and explicit navigation, and leave presence/action authority with the app. Nested Elements behavior and private delivery require scoped proof, not inferred certification.

### D54 — Explicit CSS-only drawer focus companion

Accepted. [Record](adr/0054-explicit-drawer-focus-delivery.md); a selected app-owned wrapper and stylesheet preserve base output and Elements interaction ownership. Browser proof precedes the distinct six-file CLI variant; no extra constructor, JavaScript or certification is implied.

### D55 — Private retained PNG inspection

Accepted. [Record](adr/0055-private-retained-image-inspection.md); exact receipt-declared PNG slots become privately inspectable after bounded decoding and historical integrity checks. Gallery offers native selection/stacked comparison; MCP and JSON remain byte-free, and captures never certify a target.

### D56 — Preserve declared analytical meaning

Accepted. [Record](adr/0056-preserve-analytics-role-meaning.md); app-authored role labels survive table-only rendering, missing data and export. The explicit eight-to-ten-column CSV migration preserves established fields; Gallery/Receipts adapters adopt the new shape, with no inferred classification or external consumer upgrade.

### D57 — Share responsive navigation route identity

Accepted. [Record](adr/0057-explicit-responsive-navbar-and-route-identity.md); desktop/mobile share typed links and one explicit current key; optional app-owned GET destinations never infer account mutations. Preserve the private original and Elements ownership; no certified target is claimed.

### D58 — Keep native route access persistent

Accepted. [Record](adr/0058-explicit-navbar-repair-and-native-route-access.md); an explicit repair candidate adds scoped geometry/focus CSS and a native route-directory companion using the same model. No automatic runtime-driven hiding, new JavaScript state machine, base drift or implied CLI/certification claim.

### D59 — Guard selected navbar runtime focus

Accepted. [Record](adr/0059-guard-selected-navbar-runtime-focus.md); an independently replayed runtime derivative guards only deferred container focus in selected navbar companions. Original runtime/base/repair bytes and unselected behavior remain; complete runtime-bearing delivery and target acceptance are separate gates.

### D60 — Deliver the selected navbar runtime repair

Accepted. [Record](adr/0060-deliver-explicit-navbar-runtime-repair.md); explicit repair/runtime selection binds nine private files with source-replayed checks. The original ESM is a required explicit input; default five-file output, app authority and separate consumer/target gates remain unchanged.

### D61 — Keep complete capture histories bounded

Accepted. [Record](adr/0061-bound-larger-historical-capture-sets.md);64 slots per gate retain full selected runs without changing byte ceilings or certification rules. Larger packets require upgraded private readers; old packet bytes and identities remain unchanged.

### D62 — Search declared capability tasks

Accepted. [Record](adr/0062-search-declared-capability-tasks.md); a query must fit the capability ID/name or one declared task, not negated prose or unrelated phrases. Source-reviewed vocabulary and one cross-surface evaluation corpus retain compatibility boundaries. Search semantics change; restart old cursors. IDs, schemas and maturity remain unchanged.

### D63 — Incubate account presentation without an account framework

Accepted. [Record](adr/0063-incubate-account-presentation-recipe.md); experimental typed destinations and plain-text notices replace Gallery duplication. App-owned routing, authorization and save/recovery remain separate; existing frame APIs and the two-consumer permanent-promotion gate remain unchanged.

### D64 — Incubate responsive settings sections without form authority

Accepted. [Record](adr/0064-incubate-responsive-settings-sections.md); shared escaped headings, responsive description/control layout and reviewed content replace five Gallery sections. Apps retain form semantics and state. The catalog labels this separately as experimental; two-consumer promotion remains required.

### D65 — Pin selected metadata without silently migrating consumers

Accepted. [Record](adr/0065-pin-selected-adoption-capabilities.md); opt-in adoption v2 binds each complete selected catalog record while v1 retains whole-catalog refusal. Read-only candidate pins include full metadata for review; artifact/module/router gates remain independent. A rebuilt checker is required, and no consumer is migrated automatically.

### D66 — Close consumer contribution loops

Accepted. [Record](adr/0066-close-consumer-contribution-loops.md); assess reuse at discovery and UI/CLI handoff, record a disposition and verify originating-app delivery separately from second-app propagation. Refresh owning intent/agent guidance without policy duplication, automatic upgrades or weakened promotion bars.

### D67 — Prove isolated warm-cache lineage prospectively

Accepted. [Record](adr/0067-prove-isolated-warm-cache-lineage.md); a separate P01 v2 experiment may derive warm font identity from one exact controlled cold seed and a strictly bound HTTP-cache lineage. Its first full acquisition fails observed header/event evidence, so the method is unproven on this instrument. V1 failures and numeric budgets remain unchanged; negative controls and accepted full evidence remain prerequisites, not delivered proof.

### D68 — Prepare private unsigned macOS delivery first

Accepted. [Record](adr/0068-prepare-private-unsigned-macos-delivery-first.md); the owner selects a private unsigned bundle and manual replacement/rollback as the first delivery path. Browser independence, exact provenance and native failure gates remain. This is design direction, not bundle acceptance, installation, signing authority or automatic updates; Windows stays deferred.

### D69 — Finish starter patterns and convert the long tail on demand

Partly superseded by [D72](adr/0072-stabilize-and-resume-on-demand.md) for immediate completion priority. [Record](adr/0069-prioritize-starter-patterns-and-just-in-time-conversion.md) retains the five starter areas, CUE standards/readiness and just-in-time enhancement. Original failures, profile-specific gates, consumer authority and permanent promotion requirements remain unchanged.

### D70 — Opt-in CLI invocation classification

Accepted. [Record](adr/0070-opt-in-cli-invocation-classification.md); explicit classifier execution preserves the default API and operational/explicit exit policy. Bosun and Surveyor select reviewed consumer-owned Cobra/pflag changes for typed usage origins. Scoped working-tree migration proof is not installed-binary or published-release proof.

### D71 — Deliver a bounded static reading reference

Accepted. [Record](adr/0071-deliver-a-bounded-static-reading-reference.md); build the existing first-party reading/FAQ and article as deterministic HTML/CSS files with system/light/dark choices and prefix-aware 404 recovery. No Gallery server or visitor JavaScript is required. This is an experimental fixed example, not a publishing framework or full consumer/quality certification.

### D72 — Stabilize and resume on demand

Accepted. [Record](adr/0072-stabilize-and-resume-on-demand.md); stop the open-ended campaign after a bounded handoff. Retain standards, failed receipts and unfinished outcomes; resume one named need or owner-funded maintenance batch with fixed acceptance, inputs and a stopping budget. Existing MCP/source routing is sufficient; no new semantic layer, consumer migration or completion credit for deferral.

### D73 — Separate retained-memory metrics from app attribution

Accepted. [Record](adr/0073-separate-memory-metrics-from-attribution.md); independently bound observed JS/embedder/backing growth while reporting total browser and server RSS. The pure v2 metrics subcheck never certifies attribution, detached retention or full memory acceptance; future runtime acquisition needs lifecycle evidence and independent controls. Historical v1 limits and failures remain unchanged.

### D74 — Incubate signed single-dataset analytics

Accepted. [Record](adr/0074-incubate-signed-analytics-comparisons.md); one experimental renderer adds centered signed geometry with symmetric scale, explicit zero/missing semantics and unchanged table/CSV truth contracts. Existing renderers remain unchanged; Gallery proof does not establish consumer promotion or a multi-series engine.

### D75 — Incubate app-owned analytical row destinations

Partly superseded by D76 for later layout repair. [Record](adr/0075-incubate-app-owned-analytics-detail-links.md); optional root-relative row URLs become native table links, not routing or authorization policy. Its original empty-link byte comparison remains evidence; CSV stays unchanged. Positional Go row literals need named-field migration; consumer promotion remains open.

### D76 — Reserve readable missing-state columns

Accepted. [Record](adr/0076-preserve-readable-missing-state-columns.md); missing-reason cells receive a relative minimum width with full text/native scrolling. This separate markup repair requires CSS/snapshot review and does not change data, CSV or all-present output; original link-introduction evidence and broader quality gaps remain intact.

### D77 — Incubate rectangular analytics CSV

Accepted. [Record](adr/0077-incubate-rectangular-analytics-export.md); opt-in `WriteTableCSV` repeats context in one rectangular table while retaining the existing report writer. Empty selections contain only headers; domain composition and permanent promotion remain separate.

### D78 — Draft explicit adoption selections

Accepted. [Record](adr/0078-draft-explicit-adoption-selections.md); CLI-only v2 draft authoring binds explicit capability metadata and artifact bytes without writes, inferred choices or upgrade approval. Review/save/check remain separate; existing commands and schemas are unchanged.

### D79 — Disable unsafe form-button links

Accepted. [Record](adr/0079-disable-unsafe-form-button-links.md); nonempty unsafe or disabled destinations become noninteractive link labels, never form submits. Relative web references and HTTP(S) stay enabled; routing, trust and explanations remain app-owned. Rebuild source/scanned CSS and review markup snapshots; consumer upgrades and broader quality acceptance are separate.

### D80 — Harden managed table row destinations

Accepted. [Record](adr/0080-harden-table-row-destinations.md); reuse the form-button web-reference predicate internally for table Open actions. Unsafe destinations are inert disabled labels; accepted native links gain explicit target/focus styling. App-owned slots, routes and permissions remain separate. Rebuild source/CSS and review density/snapshots; no automatic consumer upgrade or table certification.

### D81 — Derive one table action column

Accepted. [Record](adr/0081-derive-one-table-action-column.md); infer an action column once from actual destinations or a custom slot, retaining `RowHrefs` as an explicit reservation. Header and rows share its shape; unlinked rows receive an empty cell, never an invented action. Review rendered selectors/snapshots, app-authored footer spans and selected metadata pins. Full table and consumer acceptance remain separate.

### D82 — Discover adoption capabilities without a host

Accepted. [Record](adr/0082-discover-adoption-capabilities-without-a-host.md); JSON-only CLI search and guarded exact retrieval share the embedded catalog and canonical pins with MCP/Gallery. No host, consumer reads or second registry; metadata discovery remains separate from reviewed drafting, adoption checks and runtime proof.

### D83 — Render missing select values as unavailable prompts

Accepted. [Record](adr/0083-render-missing-select-values-as-unavailable.md); a nonempty value absent from the option set renders an empty selected disabled prompt, never a valid fallback or the raw missing value. A configured placeholder exclusively owns initially empty selection. Apps own validation, explanation and empty-option meaning; rebuild and review snapshots before adoption.
