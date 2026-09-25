# Consistent table action columns

## Prospective contract

R1/R4 core repair, baseline `9cbc039`. Actual shared table source renders a managed
Href cell without necessarily rendering its header; unlinked rows omit that cell
even when the action column is declared. [ADR-0081](../adr/0081-derive-one-table-action-column.md)
records the correction. I4/I7/I12/I17 and ADR-0069's unmet starter-task trigger apply.
Prior destination safety, native targets, app-owned slots and domain policy remain.

Derive action-column presence once from explicit reservation/custom slot/nonempty
row destinations, and align header/body/empty-state cells without mutating inputs.
Keep unlinked actions truly absent; a blank cell is not an unavailable link.
Footer colspans remain app-authored. No sorting engine, new API, styling redesign,
consumer migration, dependency, installed-tool/configuration or live-data change.

Freeze parsed-DOM tests before RED/GREEN: selected/nonselected tables, explicit and
inferred columns, custom slots, valid/refused/missing links and empty results.
Run table/Gallery/MCP/qualityplan race tests, scoped vet and templ/CSS regeneration.
Use the retained table-link collector/driver with actual renderer and Gallery CSS,
but omit `RowHrefs` to exercise the default. Native Alpha/detail/Back and Beta/detail/
Back, unsafe-row refusal and tab skipping stay required. Add exact three-column
header/body agreement and an empty final cell for the unlinked row.

Browser acceptance remains 1440×1000/light and390×844/dark, no application scripts,
exact completed document/CSS responses, 44px targets/focus and no page overflow.
Same frozen oracle on baseline/candidate; never activate an unsafe baseline link.
Retain source/runtime/driver identities, captures and bounded owned-process cleanup.
Budget: one RED/GREEN, scoped regressions, one paired browser acquisition, at most
one diagnosed harness correction. Stop after this correction; full table/core and
consumer gates remain separate. Packet: `/Volumes/m2ext/appkit-evidence/appkit-table-columns.hSoD5yb0`.

## Result

The frozen 24-case parsed-DOM matrix reproduces both defects on `9cbc039`, then
passes with inference and empty cells. It covers no/mixed/empty rows crossed with
selection, explicit reservation and custom slots, plus input preservation. Original
test bytes and RED log remain under `columns-red.tar` and `red.log`.

The first candidate passed that structural matrix but assigned to the templ closure's
captured parameter. Before browser acquisition, a new concurrent-render regression
reproduced its data race; `captured-params-red.tar` and its log retain the failed candidate.
The final `p := withActionColumn(p)` shadows the captured value inside each render.
Eight concurrent workers render one component eight times each under the race detector.
No original oracle was weakened; the added check catches a separate implementation risk.

Full table/Gallery/MCP/qualityplan race suites and table/Gallery vet pass. Repeated
templ generation is identical; pinned Tailwind regeneration leaves Gallery CSS
unchanged at46,191 bytes. Current production/template/CSS bytes match the archived
candidate used for the browser. No browser-driver correction or extra acquisition
was needed: exactly one baseline and one candidate run, each with two lanes.

| Browser observation | Baseline | Candidate |
|---|---|---|
| Inferred Actions header | absent | present |
| Four body-row cell counts | 3,3,3,2 | 3,3,3,3 |
| Last cell in unlinked row | count12 occupies last cell | empty action cell; count12 unchanged |
| Result | 84 ordinary checks;4 expected shape failures;0 other errors | 98 checks;0 errors |

Both retain native Alpha/detail/Back and Beta/detail/Back, guarded inert-pointer
checks and Tab skipping the unavailable action. Exact completed response bodies,
44px targets/visible focus and page containment pass. No unexpected requests or
application scripts; all input hashes remain unchanged, and owned Chrome processes
exit/drain without force. Four screenshots were reviewed: header and row backgrounds
now span the full table; values, labels, refusal and focus remain readable.

## Identity and adoption

Both collectors were built from isolated archived module trees with Go1.26.3, not
a sibling checkout or a clean-VCS release. Frozen manifests bind the collector,
binary, responses, lifecycle helper, Node26.3.0, Playwright1.62.1 and Chrome153.0.8010.53.
The [shared fixture](fixtures/table-links/fixture.go.txt) now omits the redundant flag;
the [driver](fixtures/table-links/browser.mjs) adds shape checks. Earlier link-safety
receipts retain their original source and driver bytes in their separate packet.

| Artifact | SHA-256 |
|---|---|
| `baseline.tar` | `0a15f9c77cd6a4ef463b5e3d5719c4c346e6728e207631491bf700fb51a8ad6b` |
| `candidate.tar` | `b322afb0dfd579bff4ed97caea8ee31ee607def11ff36d7e492cbaed0873864d` |
| `harness/table-links/browser.mjs` | `4e67149eca2d4923d8f79a4e96723e582d69c1e36d5803b4c1f7e9b0a729ab3b` |
| `browser-baseline/report.json` | `71edeacd19b6c15873a4fa479aa3aa6d92a4dbbdf7e7297eb34560da706a8487` |
| `browser-candidate/report.json` | `1a374b873ad4980f04a1237c470b5bd4a1a46404ea3549d99c69b15d95af0c44` |

Rebuild Go/templ, review selectors/snapshots and any app-authored footer spans, and
review the changed selected `data.table` metadata pin. No consumer, installed tool,
dependency, service or configuration was modified. Full table accessibility, sorting,
selection, real consumer adoption and core/G3/G4 acceptance remain separate.
The bounded parallel adoption/router and analytical rounding/export reviews found no
other candidate within their reviewed files; this is not a whole-project audit.
