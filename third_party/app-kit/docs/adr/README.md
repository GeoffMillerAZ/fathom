# Decision index

General records retain ADR numbering. Status is design, not implementation; see [scope](../intent/scope.md).
The register at [DECISIONS.md](../DECISIONS.md) is a compact compatibility pointer.

| ID | Status | Type | Decision | Revisit when |
|---|---|---|---|---|
| [0001](0001-record-architecture-decisions.md) | Accepted | process | Record durable decisions | 50 records or a second contributor |
| [0002](0002-go-and-cue-only-no-node.md) | Partly superseded by 0017 | architecture | Go/CUE and reproducible kit builds | island ceiling or standalone CSS unavailable |
| [0003](0003-hypermedia-first-datastar.md) | Superseded by 0021 | architecture | Original universal hypermedia profile | follow 0021 |
| [0004](0004-templ-templates.md) | Accepted | dependency | templ for the Go template profile | templ v1, inactivity or material codegen friction |
| [0005](0005-loopback-invariant.md) | Superseded by 0021 | platform | Original universal loopback rule | follow 0021 |
| [0006](0006-wails-v3-desktop-shell.md) | Superseded by 0021 | platform | Original Wails alpha choice | beta trigger fired; follow 0021 |
| [0007](0007-cue-single-source-of-truth.md) | Accepted | data | CUE authors contracts and generated truth | CUE/codegen lifecycle or diagnostics change |
| [0008](0008-embedded-nats-spine.md) | Superseded by 0021 | architecture | Original mandatory embedded NATS default | follow 0021 |
| [0009](0009-sqlite-default-store.md) | Accepted | dependency | SQLite default for selected durable storage | native mobile, driver lifecycle or measured scale |
| [0010](0010-component-data-pipeline.md) | Accepted | data | Commercial corpus pipeline | 100k vectors, source format or dependency change |
| [0011](0011-mcp-design-assistant.md) | Accepted; extended by 0020 | architecture | One typed MCP assistant | spec/SDK, host resource parity or remote serving |
| [0012](0012-gallery-living-style-guide.md) | Superseded by 0020 | product | Original full-platform Gallery | follow 0020 |
| [0013](0013-monorepo-versioning.md) | Superseded by 0045 | process | Original one-module propagation policy | follow 0045; 0026 native exception retained |
| [0014](0014-security-baseline.md) | Partly superseded by 0021/0026 | security | Local-host trust, secrets and signed updates | CSP/form transport triggers fired; multi-user or remote trust boundary |
| [0015](0015-ai-first-surface.md) | Accepted; extended by 0020 | product | Agent-readable decisions and contracts | host conventions or failed ambient-knowledge test |
| [0016](0016-client-view-router.md) | Superseded by 0017 | architecture | Static-client router carve-out | follow 0017 |
| [0017](0017-multi-target-layered-kit.md) | Partly superseded by 0023 | architecture | Layered adoption and promotion rules | second non-Go demand, new provenance/renderer needs |
| [0018](0018-terminal-target-cli-kit.md) | Accepted | platform | CLI-kit on Cobra and Charm | new interaction need, third CLI or dependency change |
| [0019](0019-experience-outcomes-and-subkits.md) | Accepted | strategy | Proven experience outcomes and sub-kits | repeated divergence or failed incubation value |
| [0020](0020-shared-catalog-and-adoption-evidence.md) | Accepted | architecture | Shared catalog, Gallery and adoption evidence | catalog divergence, bad recommendations or access boundary |
| [0021](0021-runtime-profiles-and-platform-proof.md) | Partly superseded by 0027 | platform | Scoped runtime profiles and platform proof | conformance failure or measured unmet requirement |
| [0022](0022-private-design-references-and-fidelity.md) | Accepted | data | Private whole-project references and adaptation fidelity | source changes, unreproducible reference or skipped design review |
| [0023](0023-profile-specific-conversion-and-client-compute.md) | Accepted | architecture | Tested conversion pipeline and static/client runtime choices | pilot fidelity/cost failure, stale receipt or unmet local interaction need |
| [0024](0024-read-only-adoption-contract.md) | Accepted; extended by 0088 (agent routing) | architecture | Explicit read-only adoption contracts | missing-artifact false pass, incompatible consumer need or unrelated catalog drift |
| [0025](0025-static-chart-composition-evidence.md) | Accepted | architecture | Scoped static chart composition evidence | planted false pass, escaped coverage or repeated unsupported shape |
| [0026](0026-isolated-http-desktop-experiment.md) | Accepted | platform | Isolated native host and authenticated HTML commands | navigation/bridge boundary failure, target failure or adapter promotion |
| [0027](0027-web-and-macos-now-windows-on-demand.md) | Accepted | platform | Web/macOS now; Windows on demonstrated demand | named Windows use case and authorized test host |
| [0028](0028-prepared-private-static-comparisons.md) | Accepted | architecture | Explicitly prepared private static comparisons | containment/fidelity failure, interactive need or baseline drift |
| [0029](0029-named-themes-and-app-owned-style-guides.md) | Accepted | product | Named identities and app-owned living guides | incomplete theme switch, source preference or repeated guide contract |
| [0030](0030-isolated-comparison-workspace-controls.md) | Accepted | ux | Isolated full-width comparison controls | fitted clipping, fallback/focus failure or inner-frame interaction need |
| [0031](0031-local-app-owned-theme-review.md) | Accepted | architecture | Local app-owned token preview and reviewed snapshot export | repeated consumer contract, richer preview or acknowledgement failure |
| [0032](0032-defer-car-assistant-and-growing-gardens-adoption.md) | Accepted | strategy | Defer Car Assistant and Growing Gardens BI adoption | owner resumes a named app/task; no automatic substitute |
| [0033](0033-web-first-workflows-and-native-enhancements.md) | Accepted | platform | First-class browser workflows with explicit native enhancements | OS-essential task, leaked native authority, selected topology or failed parity |
| [0034](0034-source-specific-private-component-delivery.md) | Accepted | architecture | Source-specific private adapter export and exact replay check | second adapter, consumer upgrade/platform need or false pass |
| [0035](0035-reproducible-macos-http-only-experiment.md) | Accepted | platform | Reproducible opt-in HTTP-only macOS experiment | native refusal/lifecycle failure, equivalent upstream controls or consumer promotion |
| [0036](0036-share-private-delivery-assurance.md) | Accepted | architecture | Share private filesystem assurance; keep adapter contracts distinct | non-flat/mutable delivery, compatibility drift or named consumer/platform need |
| [0037](0037-share-internal-analytics-csv-sections.md) | Accepted | architecture | Share internal CSV structure checks, not report semantics | schema drift, false acceptance or named external composition need |
| [0038](0038-runnable-receipts-reference.md) | Accepted | architecture | Runnable synthetic receipts reference; keep its signed renderer private | repeated consumer need, budget/interaction failure or ambiguous lineage |
| [0039](0039-explicit-cli-error-reporting.md) | Accepted | architecture | Explicit CLI diagnostic acknowledgement; preserve exit classification | authorized consumer migration, typed invocation provenance or new error composition |
| [0040](0040-explicit-elements-syntax-family.md) | Accepted | architecture | Explicit runtime-bound Elements syntax family; preserve legacy replay | new runtime/grammar, parity failure, typed or persisted consumer need |
| [0041](0041-explicit-faq-reference-and-native-details.md) | Accepted | ux | Explicit typed FAQ reference and JavaScript-free native disclosures | richer/stateful content, isolation/fidelity failure or repeated consumer demand |
| [0042](0042-separate-evidence-integrity-from-certification.md) | Accepted | architecture | Revision-bound evidence integrity, separate from certification | semantic/consumer gate completion, false promotion or unrepresentable lineage |
| [0043](0043-private-historical-evidence-catalog.md) | Partly superseded by 0055 | architecture | Immutable private historical evidence with shared discovery | capacity/cost limits, false compatibility, promotion or multi-user durability need |
| [0044](0044-native-catalyst-settings-composition.md) | Accepted | architecture | Complete native Catalyst settings with explicit app authority | task/budget failure, source replay drift or repeated consumer boundary |
| [0045](0045-exact-pins-and-private-candidate-preparation.md) | Accepted | process | Exact reviewed pins and private content-identified candidates | incomplete artifact, escaped source/drift, native promotion or authorized release |
| [0046](0046-native-media-denial-in-http-only-profile.md) | Accepted | platform | Deny media capture in the strict macOS HTTP-only profile | effective upstream controls, native denial failure or named required capability |
| [0047](0047-process-owned-http-and-native-recovery.md) | Accepted | platform | Process-own HTTP state and isolate native recovery | HTTP fault, control-boundary failure, upstream lifecycle, consumer promotion or signed packaging |
| [0048](0048-prove-cli-invocation-origins-before-adoption.md) | Superseded by 0070 | architecture | Explicit exit introspection and isolated invocation-origin proof | untyped origin, callback misclassification, upstream equivalent or authorized migration |
| [0049](0049-explicit-menu-actions-and-private-reference.md) | Accepted | architecture | Separate the private menu reference from app-owned links and POST actions | richer/stateful actions, runtime failure, consumer boundary or replay drift |
| [0050](0050-explicit-menu-geometry-and-focus-repair.md) | Accepted | architecture | Explicit measured menu repair; keep Elements interaction ownership and v1 delivery | placement interference, upstream fix, consumer lifecycle or delivery promotion |
| [0051](0051-explicit-private-menu-repair-delivery.md) | Accepted | architecture | Explicit eight-file menu repair variant; preserve exact base delivery | lifecycle/spacing need, runtime failure, base drift or upstream fix |
| [0052](0052-default-off-cli-invocation-origins.md) | Superseded by 0070 | architecture | Default-off candidate origins with nearest explicit ancestor policy | default compatibility, misclassification, upstream change or authorized migration |
| [0053](0053-explicit-profile-drawer-and-app-owned-navigation.md) | Accepted | architecture | Private profile drawer with explicit app-owned navigation | nested focus, source/assets drift or consumer action/state need |
| [0054](0054-explicit-drawer-focus-delivery.md) | Accepted | architecture | Explicit CSS-only drawer focus companion; preserve base delivery and Elements ownership | theme/clipping regression, upstream fix, repeated consumer wrapper or base drift |
| [0055](0055-private-retained-image-inspection.md) | Accepted | architecture | Private retained PNG inspection without source execution or certification | image bounds/read cost, false promotion or richer/public access |
| [0056](0056-preserve-analytics-role-meaning.md) | Accepted | data | Preserve declared per-row meaning in tables and CSV | mapping divergence, obsolete schema acceptance or richer consumer dimensions |
| [0057](0057-explicit-responsive-navbar-and-route-identity.md) | Accepted | architecture | Private responsive navbar with one app-owned route model | navigation/geometry failure, source drift or consumer action/layout need |
| [0058](0058-explicit-navbar-repair-and-native-route-access.md) | Accepted | architecture | Explicit navbar repair with persistent native route access | consumer density/task failure, late-load focus loss, source drift or proven native alternative |
| [0059](0059-guard-selected-navbar-runtime-focus.md) | Accepted | architecture | Explicit source-pinned navbar deferred-focus guard | lifecycle/unselected regression, upstream fix or runtime-bearing delivery |
| [0060](0060-deliver-explicit-navbar-runtime-repair.md) | Accepted | architecture | Explicit nine-file navbar runtime-bearing repair delivery | runtime drift, false variant acceptance or consumer/platform need |
| [0061](0061-bound-larger-historical-capture-sets.md) | Accepted | architecture | Complete historical capture sets within64 slots per gate | slot/byte bounds, read costs, old identity drift or false promotion |
| [0062](0062-search-declared-capability-tasks.md) | Accepted | architecture | Search coherent declared tasks, not explanatory prose | missed supported tasks, false recommendations, phrase costs or transport divergence |
| [0063](0063-incubate-account-presentation-recipe.md) | Accepted | architecture | Incubate shared account destinations and semantic feedback | consumer fit, destination/outcome drift or two releases without adoption |
| [0064](0064-incubate-responsive-settings-sections.md) | Accepted | architecture | Incubate responsive description/control sections without form authority | consumer fit, heading/layout/focus regression or two releases without adoption |
| [0065](0065-pin-selected-adoption-capabilities.md) | Accepted | architecture | Opt-in selected-capability metadata pins; preserve version1 adoption | missed selected drift, unrelated invalidation, transitive need or canonical/schema change |
| [0066](0066-close-consumer-contribution-loops.md) | Accepted | process | Contribution dispositions and consumer guidance upkeep at handoff | repeated workaround, retained fork, lost handoff or host routing gap |
| [0067](0067-prove-isolated-warm-cache-lineage.md) | Accepted | architecture | Prospective isolated warm-cache lineage; preserve strict v1 failures | false admission, server/profile change or changed browser cache semantics |
| [0068](0068-prepare-private-unsigned-macos-delivery-first.md) | Accepted | platform | Private unsigned macOS bundle first; preserve independent browser delivery | distribution/signing choice, host refusal, failed rollback or second-app seam |
| [0069](0069-prioritize-starter-patterns-and-just-in-time-conversion.md) | Partly superseded by 0072 (immediate completion priority only) | strategy | Finish starter patterns; convert and enhance the long tail on demand | unmet core task, named adopter/batch, stale proof or measured budget failure |
| [0070](0070-opt-in-cli-invocation-classification.md) | Accepted | dependency | Explicit CLI invocation classification and consumer-owned patched dependencies | upstream equivalence, compatibility/source drift or another CLI selection |
| [0071](0071-deliver-a-bounded-static-reading-reference.md) | Accepted | architecture | Plain-file reading reference with relative links and no Gallery runtime | repeated consumer configuration, host behavior or measured task failure |
| [0072](0072-stabilize-and-resume-on-demand.md) | Accepted | strategy | Stabilize and resume one bounded task on demand; preserve quality and failures | named adopter, funded maintenance batch, lost handoff or measured retrieval gap |
| [0073](0073-separate-memory-metrics-from-attribution.md) | Accepted | architecture | Separate prospective retained-memory metrics from app attribution and total browser cost | qualified lifecycle controls, changed instrument semantics or measured retention failure |
| [0074](0074-incubate-signed-analytics-comparisons.md) | Accepted | architecture | Experimental signed single-dataset renderer, preserving existing contracts | named richer task, two-consumer promotion, measured failure or numeric disagreement |
| [0075](0075-incubate-app-owned-analytics-detail-links.md) | Partly superseded by 0076 | architecture | Optional in-app row destinations; app-owned routing, unchanged CSV; original byte-identity evidence retained | external/action need, two-consumer promotion, disagreement or native-link failure |
| [0076](0076-preserve-readable-missing-state-columns.md) | Accepted | ux | Readable missing-state width with native scrolling; independent HTML repair, preserved data/CSV | font/zoom/viewport failure, unreachable reason or changed complete-data output |
| [0077](0077-incubate-rectangular-analytics-export.md) | Accepted | data | Experimental single-dataset table CSV; shared cells, unchanged report exports | consumer format/empty-set need, payload budget, encoding drift or promotion evidence |
| [0078](0078-draft-explicit-adoption-selections.md) | Accepted | architecture | Read-only explicit v2 adoption drafts; reviewed saving and checking remain separate | draft/check disagreement, automatic blessing or repeated multi-file authoring need |
| [0079](0079-disable-unsafe-form-button-links.md) | Accepted | security | Disable unsafe form-button links; preserve native forms and relative web navigation | legitimate refused destination, browser interpretation drift or disabled activation |
| [0080](0080-harden-table-row-destinations.md) | Accepted | security | Share internal web-reference refusal for managed table links; native targets and focus | legitimate refused shape, browser interpretation drift or composed target failure |
| [0081](0081-derive-one-table-action-column.md) | Accepted | ux | Derive one action column; align linked/unlinked rows and preserve explicit reservation | different consumer shape, mutated inputs or footer/header disagreement |
| [0082](0082-discover-adoption-capabilities-without-a-host.md) | Accepted | architecture | Read-only CLI capability search and guarded exact metadata; no host or second catalog | transport disagreement, output bounds or repeated projection need |
| [0083](0083-render-missing-select-values-as-unavailable.md) | Accepted | ux | Render missing select values as unavailable prompts, never valid substitutions | representation need, browser semantics or empty-option conflict |
| [0088](0088-agents-md-carries-the-adoption-router.md) | Accepted | architecture | AGENTS.md alone carries the adoption router; CLAUDE.md is optional and must link, import or carry it | a host reads CLAUDE.md but not AGENTS.md, no paired block remains, or an import kept outside the root CLAUDE.md is refused |

## Recording and maintenance

Record a choice when reversal is costly, it crosses a package boundary, changes an
invariant/non-goal, accepts a meaningful tradeoff or dependency, defers a significant
direction, or a future reader would otherwise rebuild it differently.
Use intent-decide and the [template](template.md); new records stay under 80 lines
and carry Status, Type, Context, Decision and an observable Revisit when.
The existing `- **Status:**` header form is supported by the repository checks.

Accepted records are historical; superseding changes edit only their status line.
The successor restates the surviving constraints and identifies its scope.
ADR-0002 retains the earlier ADR-0017 strikethrough exception already applied to it.
Update both indexes and affected living intent in the same change.
Do not rewrite old source/version research as if it were reverified.

The founding records often exceed the new budget; preserved history does not imply implementation. Protocol/document tests check index and status integrity, not
architectural truth. Active goals and current platform claims live in root intent.

Next ID: 0089. No reserved IDs. Check the current maximum before allocating.
