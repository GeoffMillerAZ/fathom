# Analytical meaning survives color, fallback and export

Date: 2026-09-15. G3 progress under I4/I7/I11/I12 and
[ADR-0056](../adr/0056-preserve-analytics-role-meaning.md), not full G3 acceptance.
No external consumer, dependency, global configuration, commit or release change.

## Product repair and migration

The shared renderer previously left app-authored role meanings in a color legend,
then removed that legend in table-only layouts; CSV omitted the mapping altogether.
`RenderBars` now repeats matching legend labels in a per-row **Meaning** column.
Mapping follows role identity, not legend order. Missing values retain their reasons
and declared meanings; unmapped rows say **Not specified**, without guessed semantics.
Datasets without matching labels keep the compact table. Values and wrapped labels
share a top baseline, with full text and native horizontal scrolling preserved.

The actual Gallery dataset declares API services / Worker services. Those meanings
are service categories, separate from the paired chart's Current/Previous encoding.
No counts, time windows, sources or calculations were changed.

**Breaking CSV output:** `WriteCSV` appends `role,role_label` after the original eight
data fields, preserving seventeen context records. Gallery's rectangular report grows
from31 to33 columns; Receipts grows from33 to35. Exact readers must migrate with the
kit pin and retain refusal of unknown schemas. Formula/control-led text protection
applies exactly once; unchanged public Go signatures do not make this output compatible.
Rebuild consumer CSS. External apps have not been upgraded by this change.

## Browser findings, including failed evidence

The opt-in [Go launcher](../../apps/gallery/analytics_roles_browser_test.go) mounts the
actual Gallery and an additional **test-only** long-label route. The production mux
has no new long-label route. [roles.mjs](fixtures/gallery-analytics/roles.mjs) runs
four fresh 390/1440 CSS-pixel light/dark contexts in installed Chrome152.0.7977.84,
using approved Playwright1.63.0, disabled application scripts and read-only loopback
requests. No signed-in profile, external requests or dependency installation.

Initial testing exposed a genuine strict-CSP failure: legend swatches used inline
styles. Explicitly decorative SVG presentation attributes replace them; CSP was not
weakened. The collector also needed three corrections: ignore CSV's blank separator,
allow enough native ArrowRight input to reach the last column, and classify Chrome's
attachment navigation abort only when the exact download URL has verified saved bytes.
Those collector changes are not product improvements or comparable fixed-oracle passes.

Screenshot review of the first green run then found values floating halfway down a
tall wrapped-label row. An independent DOM Range first-text-line oracle reproduced
four failures before the top-alignment repair and passed unchanged afterward. It also
checks wrapping without ellipsis; the final phone-dark/desktop-light captures were
visually inspected. The phone capture is intentionally scrolled right to expose Meaning:
the row-label column is offscreen, not removed. Very long labels still produce tall rows.

| Retained local role run | Pass / fail | Receipt SHA256 |
|---|---|---|
| `/Volumes/m2ext/appkit-evidence/appkit-analytics-role-browser.eakEpke3/analytics-roles-AzrQX0` | 74 / 9; initial CSP/collector failures | `fa032252275179482d586070f535357b1aa47463848a35325ab8bae8629d6f0b` |
| `/Volumes/m2ext/appkit-evidence/appkit-analytics-role-browser.iRWL8mc6/analytics-roles-nXSyKx` | 88 / 0; before first-line oracle | `b0f8439911e3bc7a7bd663f4a8f4cd756b2f6ea61d4d6d9c18566683159a3434` |
| `/Volumes/m2ext/appkit-evidence/appkit-analytics-role-browser.nCGm61RW/analytics-roles-ezknVK` | 92 / 4; final oracle, unaligned cells | `c63faa34e0933db68c71e65a4a51614b7b34a22d7cd08bc980bddfc3fcd40bb5` |
| `/Volumes/m2ext/appkit-evidence/appkit-analytics-role-browser.kpXy166E/analytics-roles-cGJjyc` | 96 / 0; same oracle, aligned cells | `194c1f7772c94bc7d7277e07b96387c9189378577288baa84236d37ba616777a` |

Each directory contains `receipt.json`; the final run retains four PNGs and four
identical1236-byte CSVs. All eight artifacts independently match recorded sizes/hashes
and mode0600. Twenty-four admitted responses succeed, with zero errors/blocked requests.
The unchanged red/green collector SHA is
`6037183b5214cf8d9468b16175684f2c187bcd1e34ce9b9dffc2329870c22440`.
Its saved CSV SHA is `9da8122be2ea58621a10f74f697bc34dd65b15fc2a9d5677685cfcde3e050142`.
Assertions are repeated observations, not96 independent product requirements.

The maintained production-renderer and six-case journey probes were rerun against
`/Volumes/m2ext/appkit-evidence/appkit-analytics-meaning.cpthG2wg/gallery-aligned` at loopback56865:

| Run below that build directory | Result | Receipt SHA256 |
|---|---|---|
| `appkit-analytics-recipe-4gRbd0` | 158 / 0 | `29930882a2976350113d9b4ab417fd59fcca786f2fbca6b241a2836c1d6fb877` |
| `appkit-analytics-quality-CvS98s` | 1,026 / 0 | `2079cfeb906edae553b797df305be46f6235da57bb09bc6117ffd42669c17599` |

Observed binary SHA: `2def0a8695ec8c4bbd36880b0eff4980425276ec0ab7471685d4983c6e19d12b`.
CSS SHA: `812d7e1a50e65f6bed881893cc291f8501c48cb3260ac556c05a71310df6cab7`.
Five local DOM-ready samples give p95 16.7ms under the existing250ms diagnostic ceiling,
not a speedup, cold-network or human-task measurement. Receipts bind response/artifact
bytes and selected tool files, not a complete immutable source/test-binary/toolchain closure.

### Receipts executable migration

The in-repo Receipts browser collector still expected33 columns. The unchanged
collector reached124 passing checks, then failed on the new35-column download.
Its reviewed update requires an independent exact ordered header and each synthetic
row's declared default `accent`/empty-label pair, retaining all prior numeric/context
checks. A source-inventory canary detects reintroduced old width expectations; it is
not itself browser proof. The actual isolated browser rerun passed144/144 checks,
including the existing native task, long-label specimen and selected actual200% lane.

Retained base: `/Volumes/m2ext/appkit-evidence/appkit-receipts-migration.hxgcxPdk/`.
`evidence/report.json` SHA `06ad957cb54cded75e13b5533c0a30a9d01e9634f1c8217f126e87c4d935e570`;
`evidence-green/report.json` SHA `88b69cc47cdb22ae0b2334be2cc7b6114e5b39dbb44a6a60e5914068c969e5bf`.
Both used the same owned binary SHA
`35a9ddd0a7d86bab7b893128647610bd44b364b8d1d50e31b5a2765f59d99ec9`.
The final report retains six screenshots and one12,914-byte CSV, SHA
`7a4e9b69a6b7441201727ca259479a0e2caddb01b39a5a495a97a757cfbd220c`.
No blocked requests; its deliberately invalid-query400 is recorded separately from
unexpected browser failures. The owned server/browser closed. This is the synthetic
Receipts reference, not a production consumer or converted MatDash certification.

## Deterministic evidence and limits

Mapping/default/missing/unmapped/escaping/formula tests pass. The internal section
parser explicitly refuses a complete old eight-column CSV, not just malformed rows.
Both report adapters retain exact header/row-width checks. Across144 Gallery exports,
independent expectations check the added fields; removing only those two columns
retains the original length-framed SHA
`f8747c6ec1f964bb8cb6d9af9a6def1ee94612c1f3ae4edcbd3b3962c9279afe`.
No golden was blindly replaced to accommodate the migration.

The full root `go test ./...` passes; analytics/internal CSV/Receipts race tests and
analytics/internal CSV/Gallery/Receipts vet also pass. MCP's existing register canary caught a missing D56
body summary; the documentation was completed without weakening the check.
Scope, current guides, ADR indexes, INC-42 and the breaking changelog are synchronized.
Historical receipts and old dated schema claims remain historical, not rewritten.
The focused intent-doctor subset passes ten living-document budgets,56 ADR/index/body
entries,378 relative links and identical intent stanzas; Unreleased is60 body lines.
Intent Prune condensed related unreleased entries without dropping linked evidence.
`63b3a4f` is the last committed changelog revision, not a backup of these uncommitted
notes; detailed current facts remain in their linked assessments. Host wiring, tool
version parity and semantic goal completion were not checked by this subset.

This is synthetic Chrome100% evidence for three services, not screen-reader, touch,
all-browser/zoom, large-data, production-data or two-consumer acceptance. No catalog
entry or target is promoted. G1–G6 remain active; Windows stays owner-deferred.
