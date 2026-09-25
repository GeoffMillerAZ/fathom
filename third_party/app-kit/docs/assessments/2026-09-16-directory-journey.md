# P04 native directory journey

The [prospective fixture](fixtures/directory-journey/README.md) composes app-owned
filtering, pages, detail/Back and all-matching CSV around the actual privately generated
`NewDirectoryEnhanced`, not copied/test-only table HTML. Under ADR-0023 this remains
a synthetic Go-web composition, not a shared directory backend or target certification.

## Result

- Fresh immutable eight-file export/check, pinned source CSS/font preparation, templ
  compilation and standalone tests pass. Two warm same-directory builds have identical
  binaries, without sibling app-kit imports/replaces or dependency installation.
- **1,203 browser checks, 68 captures, 20 CSV downloads** pass in Chrome 152.0.7977.84:
  390/1440×1000 CSS pixels, light/dark, scripts disabled. Native keyboard filtering,
  Next, View, Back and export retain the selection and match independent expected rows.
- Complete/partial/empty/unavailable states, malformed pages and unknown/excluded
  details remain distinct. Unavailable export fails with HTTP 503; out-of-range complete/
  partial CSV selections fail with HTTP 400. Export includes all matches, not one page.
- Quotes/commas remain literal; formula-leading names and query metadata receive an
  explicit CSV safety apostrophe without changing displayed source values.
- Wrapper contrast is at least 4.836:1 in these samples; input boundaries reach 3:1,
  controls reach 44px and pages do not overflow horizontally. Wide/light and narrow/dark
  detail/quoted-formula captures were visually reviewed.
- Separate local runs pass eight Go tests with race detection and fifteen Node oracle/
  CSV tests, including planted data disagreement. These runs are session evidence, not
  fields in the build/browser receipts. Node tests are wired into `task check`; the
  entire maintainer gate was not run here.

Maximum observed HTML: 6,815 bytes; source/font/companion CSS: 486,878 bytes;
wrapper CSS: 1,070 bytes; application scripts/requests: zero. Automated task durations
415–561ms include instrumentation: not human timings or a performance budget/pass.
The 1MiB HTML limit is unchanged. Touch, screen-reader, zoom, full performance,
source-fidelity certification, cold/hermetic build and real-consumer proof remain open.

## Retained failures

Private directories below live under `/Volumes/m2ext/`; none were removed:

| Run | Result |
|---|---|
| `directory-journey-NJ3SYb` | Preparer refused `cache`: environment omitted real home needed for read-only cache lookup. Fixed without repurposing home or installing tools. |
| `directory-journey-EGZXPE/browser-FKQJLN` | 98 checks/five pages, then collector incorrectly called Fetch's numeric `status` as a function. Collector repaired. |
| `directory-journey-EGZXPE/browser-mQJDd6` | 1,055 task checks passed, but screenshots exposed invisible dark wrapper text and borderless search; not visual acceptance. |
| `directory-journey-EGZXPE/browser-4d9zmc` | Stronger collector records 66 failures: 30 contrast and 36 input boundaries among 1,151 checks. |
| `directory-journey-PC6E0I/browser-Af9QDB` | Same collector passes 1,115 after wrapper repair. 36 duplicate-action checks disappear because the redundant Reset control was removed. |
| Final below | Review found CSV page-bound disagreement; new Go regression first failed `/directory.csv?page=999 = 200`. Repair plus two browser error cases brings final coverage to 1,203 checks. |

Wrapper repair uses source palette variables, visible controls and one generated Reset
action. Original table/companion are unchanged. No consumer, dependency, release,
deployment or global setting changed. No expected data or visual baseline was weakened.

## Final evidence

Build root: `/Volumes/m2ext/directory-journey-2aD9d5`; browser subdirectory:
`browser-jqg2mB`. Exact source snapshots, generated private delivery, module/sums,
command logs, dataset, CSS, binaries, HTML, PNG and CSV are retained there.

| Identity | SHA-256 |
|---|---|
| Build receipt | `99b59ff07891b0309716c615c2c9b286b8b341c82d77b8f49fc8110a33af9390` |
| Browser receipt | `ef5a174c1fdc518e000287b0441bdcdbfaebc99257cbcc918216fe3bb6c686f8` |
| Executed collector | `48f260a80eeece01501caf8fd06c8c7a336b22905c3a7025a6fbbe6361a71065` |
| Binary, 9,383,538 bytes | `df9de6663c38ac2bf36e23de6d30e312f14903f5669255303a1e8bfb69cbf9d0` |
| Canonical 12-row JSON revision | `01c6683c7c6f9d9cf2a975996d803c20b1b9c9c46be6c3a93af7c9b50f2278f0` |

Final 68 PNGs and 20 CSVs were independently rehashed. Owned host 82499 and Chrome 82500
exited 0 and were confirmed absent. Twenty `ERR_ABORTED` events match exact completed
download URLs, saved bytes and independent HTTP 200 CSV responses; other failures fail.

## Reusable return

Adapters supply presentation; apps own selection, coverage, routes and export scope.
Return this test recipe and lessons through [the contribution loop](../design/contribution-loop.md):
independent values, filter-before-page, truthful partial data, retained context,
table/export agreement and actual theme/input affordances. This fixture is not a second
consumer or a reason to promote a permanent directory service/API.
