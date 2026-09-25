# Analytics reference journey

The Gallery's `/analytics` demonstrates selection → explanation → detail → export.
It is an experimental G3 reference, not a promoted reusable application workflow,
production telemetry, or evidence that a consumer has adopted the recipe.
[Experience standards](experience-system.md) own the quality bar.

## Task and design

A developer compares failed-operation counts across services and calendar months,
chooses a service to investigate, and exports the same evidence. Counts are not
normalized for workload volume: fewer failures do not establish a better reliability
rate, and this fixture cannot establish causes. The copy makes that boundary explicit.

The composition is a compact reliability report: selection, explanation, directly
labeled comparisons, exact values, then provenance. MatDash's Dashboard2 source informed
chart/table hierarchy; it was not copied or rendered for a fidelity claim. Service
names live outside SVG so long and international labels can wrap. Semantic tokens
support the Gallery's light/dark/system modes; narrow layouts retain a scrollable,
keyboard-focusable chart/table region rather than shrinking text into unreadability.

Current and previous months have separate UTC windows. Service detail retains the
overview's comparison scale. Missing current values produce no current bars or numeric
plot scale; measured zero retains both its value and scale. Partial counts are lower bounds, not extrapolations.
Partial/missing fixtures withhold the paired chart and delta; previous values remain
available in the exact-value table and CSV as reference, not like-for-like comparisons.

## Routes and state

| Route | Result |
|---|---|
| `/analytics` | selection, explanation, comparisons and exact-value table |
| `/analytics/{service}` | selected service, explanation and expanded provenance |
| `/analytics/export.csv` | all selected services, with current and optional previous rows |
| `/analytics/{service}/export.csv` | the same export contract narrowed to one selected service |
| `/analytics/recipe` | exact public single-dataset `analytics.RenderBars` specimen |
| `/analytics/recipe/export.csv` | direct `analytics.WriteCSV` context/value export of that dataset |

All support GET and HEAD. Other methods return 405; invalid selections return 400;
unknown or filtered-out services return 404 with a safe recovery link. Noncanonical
paths may receive Go ServeMux's normal canonical redirect before route validation.
Responses set a restrictive CSP, no-store and nosniff; error text never echoes input.

| Parameter | Values; default |
|---|---|
| `period` | `2026-08`, `2026-09`; September |
| `compare` | `none`, `previous`; previous |
| `category` | `all`, `api`, `worker`; all |
| `state` | `complete`, `missing`, `stale`, `partial`; complete |

The four owned fields reject duplicate, empty and invalid values. Query parsing is
bounded to 16 KiB, rejects malformed encoding/NUL/invalid UTF-8, and validates the
optional Gallery `theme` field. Generated links retain only canonical selection and
theme context; drill-down, back, export and capability links preserve that context.
Native GET forms and disclosure controls need no browser runtime. “Load complete
fixture” changes only the truth state; it does not refresh a backend or change time.

The clock is fixed at 2026-10-05 12:00 UTC. The complete source was observed one hour
earlier; the stale fixture uses an older observation. The page labels its synthetic
data and fixed evaluation time. Calendar window, observation and calculation time
are separate concepts. July/August/September totals are 15/9/5 respectively; partial
fixtures use an explicit first-20-days ledger and never fill the rest of the month.

## CSV agreement

Exports are one rectangular 33-column table: six selection/series columns, seventeen
`context_` columns, and the shared ten value columns. Current and previous rows each
carry their own source, window, freshness and coverage. Service exports narrow copied
datasets without mutating the overview selection or claiming a different source age.

The adapter uses `analytics.WriteCSV` for validated context, numeric formatting,
missingness and spreadsheet formula neutralization. It checks the exact context and
value schema before reshaping; schema drift fails rather than silently dropping fields.
Missing numeric cells remain empty, genuine zero stays numeric zero, and partial
coverage/lower-bound notes travel with the rows. CSV is presentation evidence, not a
lossless raw ledger or an independent metric calculation.

## Implementation and evidence

The Gallery owns `analytics_model.go`, `analytics_routes.go`, `analytics_render.go`
and `analytics_templates.go`. It composes shared `ui/analytics` validation/CSV and
`ui/viz` bars without introducing a query engine, JavaScript framework or app backend.
Catalog capability `analytics.comparison` opens `/analytics/recipe`, the exact public
renderer; this paired journey remains a separately linked experimental reference.
Apps remain responsible for aggregation, authorization and truthful data declarations.

The separate [public-renderer specimen](../assessments/2026-09-15-analytics-renderer-specimen.md)
uses the current dataset and canonical `compare=none`, preserving period/category/state
and theme through native filters and export. Its CSV has the public context/value
format, not this journey's 33-column adapter. Over-wide plot text receives an explicit
table-only fallback; the full comparison workflow and its data remain unchanged.
The public renderer and exports now retain explicit API/Worker group meanings, separate
from the paired chart's period encoding. [ADR-0056](../adr/0056-preserve-analytics-role-meaning.md)
adds visible per-row **Meaning** and CSV `role`/`role_label`; table fallback and missing
rows preserve those labels. Update eight-column/31-column readers to ten/33 columns.
[Migration and browser evidence](../assessments/2026-09-15-analytics-role-meaning.md)
include the unchanged144-export legacy-field digest and long-row alignment failures/repair.

Tests cover all 48 selections, exact month/service counts, chart/table/detail/export
agreement, missing versus zero, partial/stale context, CSV schema drift and formula
protection, invalid routes/queries, HEAD, recovery/theme context, escaping and long
labels. The three-service reference has a preselected 32 KiB HTML budget and no
browser scripts. Those are output checks, not browser latency or usability measurements.

The [composed-boundary tests](../assessments/2026-09-16-analytics-composition-safety.md)
also inject hostile datasets through the recipe handler: parsed HTML/SVG/table text,
explicit safe-source links, independent CSV formula/zero/missingness checks, download
headers and GET/HEAD parity. The private builder seam changes no production policy;
external HTTP(S) sources remain allowed. These are source/HTTP checks, not browser
or consumer-authorization certification.

The isolated Chrome [browser receipt](../assessments/2026-09-13-gallery-browser-smoke.md#analytics-journey-follow-up)
records 107/107 checks: selected 390/768/1440 light/dark layouts, focus/Enter activation,
detail/back context, exact downloaded CSV and missing/zero/partial/stale fixtures.
That historical probe chose native select values programmatically. A separate
[quality follow-up](../assessments/2026-09-14-gallery-analytics-quality.md) traverses
skip → native type-ahead selection → disclosure → apply → detail → back → CSV using
keys alone, across all six width/theme cases. The unchanged final oracle reports
948 passes / 54 target-size failures before repair and 1,002 passes after it.
Native summaries and table drill-down links now own at least 44px-high hit areas;
the table's padding is not mistaken for its link's hit area. Static regression tests
preserve native semantics; the browser separately measures selected rendered targets.
Sampled opaque text/background contrast passes. A separate [actual200% Chrome follow-up](../assessments/2026-09-15-analytics-zoom-and-adoption.md)
passes459 checks across light/dark keyboard, detail/back and CSV tasks at1440 outer/720
CSS pixels, with complete full-document captures. Other zoom/browser combinations,
native popup-arrow interaction, screen readers, human tasks and broader acceptance remain open.
The shared [vizcheck](../../appkit/ui/vizcheck/README.md) guard now checks scoped SVG
coverage, printed values, captioned table and source text against independent fixture
expectations across overview/detail selections, with planted failures. It cannot prove
geometry, computed visibility or source correctness. Local before/after observations
record DOM-ready p95 23.2/19.9ms against a predeclared 250ms ceiling. Five samples are
not a speedup claim. Six real handler benchmarks include rendering/encoding and remain
below 5ms/op and 512KiB allocated/op; HTML stays below 32KiB. These fixed synthetic
loopback/in-process budgets do not cover LCP/INP, remote networks, large datasets or
real devices. Consumer acceptance and human baseline/improved task evidence remain
required by G3; this is not promotion or a reason to add speculative render caching.

The [human comparison protocol](../assessments/fixtures/analytics-usability/README.md)
predeclares selection/explanation/detail/return/export tasks, timing and error recording.
Human results remain pending. A [reconstructed-composition pair](../assessments/2026-09-16-analytics-comparison-pair.md)
now passes8/8 no-script keyboard/detail/return/export cases on identical data/runtime,
with exact artifact identities and original quality failures retained. This is not
historical runtime replay. Participant prompts must not reveal the expected answers.

The [four-state comparison follow-up](../assessments/2026-09-17-analytics-truth-state-journeys.md)
proves eight fixed September/API/previous-month journeys in ordinary and actual200%
Chrome: complete, missing, stale and partial explanations, chart labels/values, tables,
Gateway detail/back and every33-column CSV cell. It preserves the older partial lanes;
all-state/all-theme/all-viewport, second-service detail and consumer/human acceptance
are not inferred. Captured outputs and the earlier narrower CSV oracle stay distinct.

The separate [200-row public-renderer boundary](../assessments/2026-09-15-analytics-maximum-rows.md)
adds268 scoped checks and exact exports, not a larger paired-period journey. Its6,420px
chart makes the distinction explicit: an API ceiling is not recommended dashboard
density. Apps own meaningful selection/aggregation and honest coverage/export scope.

## Consumer return: readable meaning

Fathom's service-share review exposed a clipped Gauge caption (INC-14). Use existing
`vizframe` for wrapping explanation, denominator and snapshot/source context; do not
nest another card around its surface. Gauge's conservative grapheme wrapping protects
shorter labels but does not replace browser/font review. Named callouts distinguish
missing configuration/comparable evidence from measured zero and provide a real next
step. [Consumer receipt](../assessments/2026-09-13-fathom-observatory-uplift.md#analytical-gap-and-caption-follow-up)
records both mirrored-consumer proof and the separate upstream correction.

The Gallery review also caught partial-state prose promising unselected previous
values. INC-15 pins overview/detail with and without comparison; the [missing-scale repair](../assessments/2026-09-16-gallery-header-and-missing-scale.md)
also refuses a numeric scale without a plot while preserving measured zero. Surrounding
copy obeys chart/table/export state; numeric agreement alone does not prove truthful explanation.
