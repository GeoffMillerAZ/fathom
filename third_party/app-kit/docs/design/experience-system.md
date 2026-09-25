# Experience system: capabilities, recipes and proof

ADR-0019–0023 govern. APIs: [ui-components](../ui-components.md), [cli-kit](cli-kit.md); catalog available, G1–G6 incomplete ([dated assessment](../assessments/2026-09-13-app-kit.md)).

## Design starts with a task

Record user, scene, task/decision, data, inputs, targets and failure costs. Choose
typography, density, palette roles, composition and motion from that context, with
references and anti-references; the dashboard category does not choose its aesthetic.

Choose a complete visual reference separately from the implementation primitive.
Catalyst, MatDash and paid templates inform composition even when Go renders the app.
Compare original and adaptation for typography, spacing, surfaces, contrast, themes
and interaction; never flatten them with palette-family substitutions. [ADR-0022](../adr/0022-private-design-references-and-fidelity.md)
owns private source preservation and the [purchase audit](../assessments/2026-09-13-paid-design-assets.md) records coverage.

Judge reading order, next actions, data, controls, feedback and narrow layout in context.
Apps may look distinct; the foundation carries shared meaning and behavior.
## Capability and recipe contract

[catalog.cue](../../appkit/catalog/catalog.cue) drives MCP and Gallery's first-party entries,
including experimental shell/analytics recipes and private-adapter CLI metadata, with one schema and revision. Scoped
[conversion evidence](conversion-pipeline.md) is not target certification; [runtime selection](runtime-selection.md) claims only declared profiles.

| Field family | Must answer |
|---|---|
| Identity | stable ID, kind (token/primitive/recipe/reference), revision, replacement/migration |
| Task | who uses it, problem solved, when useful and when inappropriate |
| Fit | layers, rendering/runtime profiles, platforms, CSS scanning, JS/runtime dependencies |
| Integration | real symbol/source, minimal compiled example, configuration, app-owned seams |
| Behavior | states, input/focus/keyboard contract, recovery, authorization boundary |
| Data | units, period, grain, aggregation, missingness, provenance and uncertainty as relevant |
| Design | theme/density/viewport examples, semantic roles, allowed variation |
| Maturity | experimental/available/proven; each entry declares its maturity and evidence gaps |
| Proof | fixture and test entrypoints, revision/target/results, known gaps |
| Provenance | original source/version/license reference, adaptation lineage, permitted audience |
| Adoption | originating consumers, divergences, upgrade guidance and observed outcomes |

Consumer history is not fresh host proof; commercial availability is not integration or conversion.

Search admits all query words within one declared task phrase or the capability ID/name,
not combined unrelated tasks or explanatory prose ([ADR-0062](../adr/0062-search-declared-capability-tasks.md)).
Use reviewed phrases such as "dark mode" or "command configuration"; missing tasks stay
empty. The [source-free evaluation corpus](../../internal/catalogtest/testdata/selection.json)
covers every record through the catalog and MCP/Gallery JSON/HTML, including all pages.
Old search cursors must be discarded; exact contracts and capability IDs remain stable.

## Common application recipes

| Recipe | Complete behavior to demonstrate | App retains |
|---|---|---|
| Shell/navigation | current route, breadcrumbs, keyboard focus, discoverable mobile nav, responsive density | information architecture, routing and permissions |
| Profile/account | identity summary, editable fields, validation, save progress/error/retry, session actions where supported | identity provider, profile schema, authorization |
| Settings/appearance | grouping, search where useful, unsaved changes, reset/defaults, persisted theme and recovery | settings schema, persistence and security policy |
| Help/about | contextual help, keyboard help, version/build info, safe diagnostic copy | support destinations and redaction policy |
| Data browser | search/filter/sort/page, selection, empty/no-results, denied/error/retry, detail/back context | queries, access rules and record semantics |
| Long-running work | start/cancel/progress, reconnect, partial success, actionable error, result | job engine and side effects |
| Destructive/approval flow | consequence preview, reviewable diff, authorization, failure and recovery | approval policy and irreversible action |

These are coverage to earn, not prebuilt backends; persistence and authorization remain app-owned.
Compound forms need native/controlled reset, dependent membership, visible/serialized agreement,
explicit length semantics, preserved drafts and associated errors/first-invalid focus ([Catalyst probes](../assessments/2026-09-13-catalyst-hardened-reference.md)).
Rich editors need named/reflowing controls, selection-preserving pointer and keyboard paths,
history/preview agreement, explicit code language and image alt/origin policy ([MatDash probes](../assessments/2026-09-14-matdash-hardened-reference.md)); neither reference supplies a save backend.

`hitl.ApprovalCard` owns its surface: do not nest it inside `dashboard.Widget`.
Use a bounded, single-column ordered queue with count and honest empty state; preserve
app-owned ranking, consequence previews and actions. The [RAC comparison](../assessments/2026-09-13-resume-today-uplift.md)
also fixed `WidgetGridOptions{Cols: 1}` at tablet width for default-span widgets, not other grids.

## Analytics excellence

Begin with the analytical question, not a chart. [RAC's proof](../assessments/2026-09-14-resume-pipeline-uplift.md) requires per-record
cohort checks: numerator ≤ denominator alone does not validate a rate. Carry refusal
through a domain-owned render contract so legacy, web and CLI readers inherit it.
The [assessment ledger](../assessments/2026-09-14-resume-assessment-evidence.md) normalizes reads,
keeps raw counts but refuses malformed rates and source-free scores. Recorded references
are not current resolved evidence; show that distinction and provenance failures.

- **Metric contract:** name, unit/currency/timezone, grain, aggregation, numerator/
  denominator, comparison window, direction of improvement and valid range.
  Apps calculate values and evidence validity; the kit displays the declared semantics.
- **Truth states:** distinguish zero, missing, loading, stale, partial and error.
  Show material observation time/source/uncertainty; never turn missing data into zero,
  smooth unexplained gaps or average unlike score sources.
  Aggregate coverage as well as values: none known is absent, incomplete sources give a
  named lower bound with coverage, and only complete coverage permits an exact zero.
  Qualitative health summaries obey the same rule: unknown is not "all fresh";
  [Fathom's adopted display repair](../assessments/2026-09-16-fathom-display-adoption.md)
  returns synthetic counterexamples and scoped consumer proof, not a generic aggregate
  API, second-consumer propagation or full dashboard acceptance.
  Share that app-owned projection across overview, detail, narrative and exports; a
  current snapshot is not a period total. [Fathom triage](../assessments/2026-09-14-fathom-observation-triage.md)
  supplies counterexamples and replayable narrow/keyboard proof, not a generic aggregate API.
  A failed source invalidates every dependent total, ratio and row set, including stale
  populated values; independently measured sources remain usable. Test both empty and
  poisoned populated inputs. [Fathom's cost repair](../assessments/2026-09-14-fathom-evidence-journey.md)
  demonstrates this distinction and a complete summary/evidence/return task.
  Test glyph bounds and keep short values with units/inequality signs. Let captions
  and dense columns reflow; do not hide source reasons to make a number fit.
  [Stat reflow checks](../assessments/2026-09-14-stat-reflow.md) measure text bounds,
  overlap, partial-row separators and actual contrast; a mark's 3:1 floor does not
  qualify small text. Text geometry alone does not prove a reflowed grid's borders.
- **Honest encoding:** identify series, units and scale; distinguish measured from
  estimated values; label meaningful comparisons and annotate current values.
  Colors have legends or direct labels; color is not the only channel.
- **Exploration:** period and comparison selectors, meaningful facets, clear reset,
  linked detail and back navigation preserve context. Persist/share filter state in
  a URL when appropriate; avoid putting sensitive state in it.
- **Agreement:** chart, summary, table and exported data use the same query/selection
  semantics. Export includes relevant source/period/filter context and resists
  spreadsheet formula interpretation for untrusted text where applicable.
- **Accessibility:** readable labels, keyboard filters/drill-down, table alternatives,
  focus, zoom and narrow layouts; test actual SVG text/cell bounds, not just page overflow.
- **Scale:** record rows/marks/update rate; aggregate or page before overwhelming the
  DOM. Admit virtualization or a richer renderer only on measured need.
- **Live data:** stable identities, bounded updates, cancellation, visible freshness,
  reconnect/resnapshot and no misleading jumps while users read or select.

The opt-in [vizcheck helper](../../appkit/ui/vizcheck/README.md) adapts RAC's rules at
the composition boundary with adversarial fixtures and scoped SVG inventory. ADR-0025
limits this to static output evidence; preserve ADR-0017's primitive escape hatches.

## Gallery: a working catalog and laboratory

Launch `task gallery:run` (http://127.0.0.1:8765); build CSS with `task gallery:css`.
`/catalog` searches shared records; `/catalog/{id}` shows usage, source anchors and gaps.
MCP `search_capabilities`/`get_capability` and `kit://catalog` carry the same revision.
JSON: `/api/catalog`, `/api/catalog/{id}`, `/api/catalog/search`; filters are `q`, `profile`,
`layer`, `kind`, `maturity`, `limit`, `cursor` (MCP uses `query` for text). Limit: 1–50, default 10.
[Analytics](analytics-journey.md) now demonstrates selection/detail/export and truth-state controls.
[Application shell](application-shell.md) demonstrates synthetic account saves/recovery; browser proof and promoted consumer recipes remain work.

The [theme workbench](theme-workbench.md) preserves identities; [local app guides](app-owned-styleguide.md)
add experimental editing/export, not consumer proof. Show schemes/densities, narrow/wide layouts,
long labels, empty/error/stale/denied states and realistic synthetic data.
Supported recipe controls complete an action; label static specimens. Isolate full-page
shells to avoid nested main landmarks. CLI transcripts need actual terminal replay;
HTML screenshots cannot prove terminal restoration or mouse input.

First-party discovery needs no licensed corpus. `/references` browses private originals, not converted recipes. The nine `reference.*-adapter` records (hero, pricing, FAQ, directory, form, navigation, menu, drawer, navbar) expose CLI tooling through `profile=cli`, `kind=reference`, with private export/check guidance, not licensed bodies or certified web-profile support. Read exact source requirements before invoking the CLI.
## Quality gates

| Gate | Required evidence |
|---|---|
| Contract/build | typed example compiles, metadata validates, assets reproduce |
| Behavior | complete representative task, all applicable failure/recovery states |
| Accessibility | keyboard/focus and assistive semantics, zoom/narrow layout, computed contrast; automation plus review |
| Analytic integrity | expected fixture numbers, units/time/missingness, consistent table/export and chart encoding |
| Craft | running-product review at intended viewports, theme/density, hierarchy and motion |
| Performance | named hardware/browser/data, initial payload, render/update/input latency and memory |
| Adoption | real consumer integration and retained domain behavior; reusable fix reaches another app |
| Host | actual target input/lifecycle/native-capability evidence for any host claim |

Set numeric budgets before measuring; preserve accessibility. Snapshots supplement
interaction checks; missing evidence stays failed or unknown, never green.
## Terminal excellence

Retain Cobra, explained CUE config, stable output/exit contracts and Charm theming.
Use Bubble Tea for persistent interaction; mouse hit-testing/focus/scrolling must retain
complete keyboard paths and native selection/copy opt-out. Test resize, Unicode, NO_COLOR,
pipes, cancellation, errors and restoration across supported terminals. Crush and Herdr
inform interaction goals; do not copy their implementation by assumption.

## Technology admissions

Prefer Go/templ/Datastar for operational apps; use portable artifacts elsewhere.
NATS, WASM, desktop and richer clients need a named task and measured proof (ADR-0021,
futures.md). No speculative adapter family; app intent governs profile selection.
