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

The composition is a compact reliability report: applied selection, explanation, directly
labeled comparisons, exact values, then provenance. MatDash's Dashboard2 source informed
chart/table hierarchy; it was not copied or rendered for a fidelity claim. Service
names live outside SVG so long and international labels can wrap. Semantic tokens
support the Gallery's light/dark/system modes; narrow layouts retain a scrollable,
keyboard-focusable chart/table region rather than shrinking text into unreadability.

The native “Change comparison” disclosure keeps filter editing available without
pushing the report behind a full screen of controls. Its closed summary shows the
applied group and comparison; the month remains in the header. Truth-state warnings
stay outside all filter disclosures. [Scoped checks](../assessments/2026-09-22-analytics-selection-disclosure.md)
are separate from historical runtime receipts and the still-open human A/B gate.
Use this arrangement for read-first reports. A query-building or filter-first app
may keep controls expanded; never hide the applied scope or a warning that changes
the interpretation of a number. This is composition guidance, not a required kit API.

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
| `/analytics/recipe/table.csv` | opt-in `analytics.WriteTableCSV` rectangular export of the same dataset |

All support GET and HEAD. Other methods return 405; invalid selections return 400;
unknown or filtered-out services return 404 with a safe recovery link. Noncanonical
paths may receive Go ServeMux's normal canonical redirect before route validation.
Responses set a restrictive CSP, no-store and nosniff; error text never echoes input.
The synthetic source-error selection returns 503 HTML on these routes, including CSV
destinations, without an attachment or usable dataset. It is not an invalid query.

| Parameter | Values; default |
|---|---|
| `period` | `2026-08`, `2026-09`; September |
| `compare` | `none`, `previous`; previous |
| `category` | `all`, `api`, `worker`, `empty` (explicit empty fixture group); all |
| `state` | `complete`, `missing`, `stale`, `partial`, `error`; complete |

The four owned fields reject duplicate, empty and invalid values. Query parsing is
bounded to 16 KiB, rejects malformed encoding/NUL/invalid UTF-8, and validates the
optional Gallery `theme` field. Generated links retain only canonical selection and
theme context; drill-down, back, export and capability links preserve that context.
Native GET forms and disclosure controls need no browser runtime. “Load complete
fixture” changes only the truth state; it does not refresh a backend or change time.

`state=error` demonstrates failure to read the shared synthetic source. The Gallery
refuses before constructing either reporting window, resolving details or exporting,
including if a recipe test builder could supply populated stale values. The error
page keeps the selected month/group/comparison and appearance, but withholds totals,
charts, exact-value tables, detail links and downloads. Recovery loads the complete
fixture with the same selection; recipe comparison stays its existing canonical none.
It does not retry a live service. A source failure takes precedence over an empty
group; only successful recovery can produce the normal empty result. This is
Gallery-owned failure policy, not a new shared Dataset state or error backend.
See the [bounded error/recovery task](../assessments/2026-09-23-analytics-source-error.md).

`category=empty` demonstrates a successful selection with no matching services
when the source has not failed. It does not establish zero failures or turn missing
reports into an empty result. The overview omits empty chart/table scaffolding and
offers “Show all service groups”, changing only the group while preserving the period,
comparison, condition and theme. A state-only reset is not offered as group recovery.
The public-renderer specimen keeps its actual shared empty output, with app-owned
explanation and recovery. See the [bounded task](../assessments/2026-09-23-analytics-empty-selection.md).

The clock is fixed at 2026-10-05 12:00 UTC. The complete source was observed one hour
earlier; the stale fixture uses an older observation. The page labels its synthetic
data and fixed evaluation time. Calendar window, observation and calculation time
are separate concepts. July/August/September totals are 15/9/5 respectively; partial
fixtures use an explicit first-20-days ledger and never fill the rest of the month.

## CSV agreement

The recipe offers two explicit formats. Existing renderer CSV retains its separate
context section; spreadsheet CSV repeats the same 17 context values on each of its
10-column data rows (27 columns total). Empty selections in the spreadsheet format
contain only headers; use renderer CSV to retain empty-selection context. This
[experimental writer](../adr/0077-incubate-rectangular-analytics-export.md) changes
neither the existing exports below nor their app-owned selection/series columns.

Exports are one rectangular 33-column table: six selection/series columns, seventeen
`context_` columns, and the shared ten value columns. Current and previous rows each
carry their own source, window, freshness and coverage. Service exports narrow copied
datasets without mutating the overview selection or claiming a different source age.
With no selected rows, this format contains only headers, not context values or a
manufactured zero row. The empty overview labels that limitation and links to the
existing renderer CSV for current-period context only, not both comparison windows.

The adapter uses `analytics.WriteCSV` for validated context, numeric formatting,
missingness and spreadsheet formula neutralization. It checks the exact context and
value schema before reshaping; schema drift fails rather than silently dropping fields.
Shared `Window.TimeZone` requires an explicit IANA name or `UTC`, never Go's special
host-dependent `Local` sentinel. Callers previously using `Local` must choose their real
reporting zone; validation refuses before chart/table/CSV output rather than guessing.
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
and theme through native filters and export. Optional `Row.DetailHref` now links
its table labels to Gallery's existing service detail; browser Back returns to the
selected recipe. The destination's own Back link still returns to the paired overview.
Apps own both return policies; the kit does not invent routes or a router.
Its CSV has the public context/value
format, not this journey's 33-column adapter. Over-wide plot text receives an explicit
table-only fallback; the full comparison workflow and its data remain unchanged.
Rounded positive bars below one logical plot pixel also receive an explained
table-only fallback: never inflate a tiny mark or silently draw a positive value
as zero. This guard belongs to `analytics.RenderBars`; low-level `viz.Bars`,
dataset/CSV values and existing CSS classes are unchanged.
[Resolution regression](../assessments/2026-09-22-bar-resolution-and-guide-discovery.md).
For native keyboard checks, input dispatch is not visible-state completion: retain
the immediate sample and wait for an explicit postcondition under a fixed deadline,
with a no-input control. The [bounded scroll task](../assessments/2026-09-22-analytics-scroll-settling.md)
checks the rightmost Meaning column and return-left without application scripts;
its local pass does not replace full accessibility or consumer acceptance.
Native zoom also needs an inspected raster, not just a passing bounding box. A
[same-page Fathom capture comparison](../assessments/2026-09-22-native-zoom-paint.md)
reproduces Playwright's wrong-region 200% screenshot while no-clip compositor captures
show the chart correctly. Reuse its guarded static-page recipe only within that scope;
do not change app layout to compensate for capture-coordinate errors or promote an
uninspected image. Different themes/targets still require their own evidence.
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

The separate [styled-report readiness acquisition](../assessments/2026-09-22-analytics-report-readiness.md)
adds24 initial/Apply/detail/return observations after complete document/CSS bodies,
exact task/style/layout checks and font/frame settlement. Maxima are103.4ms ordinary
and595.0ms with declared throttling; combined decoded payload stays≤81,186B. This is
an automation-inclusive readiness endpoint, not LCP/INP, human speed or a real-device
SLO. The original harness timeout is retained; app-script denial is enforced by the
unchanged exact CSP while DevTools measurement callbacks can run. Memory, warm-cache,
cancellation and consumer-performance acceptance remain open; no new cache is needed.

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

The experimental [signed single-dataset renderer](../adr/0074-incubate-signed-analytics-comparisons.md)
adds `/analytics/signed` and `/analytics/signed/export.csv`: symmetric signed geometry,
explicit zero/missing distinctions and the same context/table/CSV. Existing positive-only
`RenderBars`, paired-period journey and consumer-owned calculations remain unchanged.
Gallery integration alone does not promote the renderer or complete G3.

### Consumer adoption checks

Apply the same truth contract to each app's actual domain projection, not only to
Gallery fixtures. The [RAC/Fathom resumption](../assessments/2026-09-21-adoption-analytics-resumption.md)
found gaps that separate renderer and export unit tests had missed:

- Exercise the visible path from the empty state as well as the populated chart.
  An export endpoint passing by direct invocation does not prove its link exists.
- Compare **every selected row**, units, denominator/rate floor and validity fields
  against independent fixed expectations. A first-row spot check is not agreement.
- Distinguish an unsent record from a measured zero outcome and a thin sample.
  Reject unknown explicit aggregate selectors rather than falling back to detailed
  exports. Only measured roles belong in the chart legend; missing rows keep their
  table/export meaning. [Integrated regression evidence](../assessments/2026-09-22-analytics-adoption-integration.md).
- Narrowing an export to a service must preserve the outer snapshot's timestamp,
  coverage and read-error qualifications. A zero-valued backing field with an
  unavailable source must never become an unexplained measured zero in JSON/CSV.
- Distinguish page/export agreement on one frozen fixture from consistency across
  separate live reads. If the app has no immutable snapshot/version, disclose that
  boundary; do not claim transactions or independently verified source freshness.
- Test source failures independently. A call-log failure does not establish a
  session-ledger failure, and zero listed services under an unavailable registry
  does not establish an empty fleet. Do not manufacture a successful model merely
  to make a style-guide specimen look finished.
- For streaming dashboards, test failure before the first good read, failure after
  success, recovery and a fresh connection. Retained fragments need an explicit
  last-good timestamp; a current connection does not mean current data. Match the
  transport's patch mode to complete versus inner markup, and check stable-id
  uniqueness across updates. [Conjure's bounded repair](../assessments/2026-09-21-conjure-dashboard-validity.md)
  returns this lesson without extracting its quarantined consumer code. Observe an
  actual client patch before asserting reconnect isolation: initial HTML plus an open
  socket can pass vacuously. Independently test connection-indicator event contracts
  and overlay geometry; a successful data update can coexist with a broken indicator.
- Run through the consumer's real routing/auth boundaries when claiming those
  properties. Direct-handler tests and unchanged middleware are narrower evidence.
- Predeclare payload and renderer budgets for the actual selected fixture. Record
  the envelope (widget/page, CSV/JSON, CSS inclusion, dataset size, toolchain) so a
  fast in-process render is not reported as fast browser loading or better UX.

These are applications of G3's existing agreement/uncertainty requirements, not a
new blanket framework or prescribed export schema. RAC's scoped integration does not
establish Fathom integration. Keep domain calculations and authorization app-owned.

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
