# Readable missing-state explanations in analytical tables

## Prospective task

Baseline: `8cc2a7c7cc0f1dd9b281a0b6df98511dc51b72bd`.
Prior detail-link image review found excessively tall missing-data rows on narrow
screens. The suspected cause is the State column's unrestricted minimum width,
combined with `overflow-wrap:anywhere` and other columns' explicit minimums.
Measure that hypothesis before editing the renderer.

The task is to read why an observation is missing, then return to its row label.
Keep the entire reason, units, period, missingness and CSV. Reuse native table
scrolling; no truncation, disclosure widget, new renderer or consumer migration.
I4/I7/I8/I11/I12/I17 apply; this is bounded G3/core maintenance under ADR-0069/0072.

## Frozen acceptance

- Execute the same actual-handler synthetic responses at 320px/light,
  390px/dark and1440px/light for August/API/missing; add390px/dark complete
  September/all as a control. Fixed height:1000px. No application JavaScript.
- Measure State widths and row heights. At a16px root font, missing-reason cells
  must be at least192px wide and the two fixed65/64-character reasons must fit
  in rows no taller than128px. No page-level horizontal overflow.
- With native Tab and ArrowRight input, reach the full State cell in the table's
  existing scroll region. After the measured scroll settles, all reason text must
  lie within the cell and visible viewport. Native ArrowLeft returns to the labels.
  Retain a no-input scroll control; input dispatch alone is not completion.
- Values, exact reasons, table headers, native links, all-present HTML and every
  CSV response must remain identical. Candidate missing-row HTML changes only by
  the declared minimum-width class. Preserve strict CSP and response completion.
- Freeze baseline failures, candidate inputs and tested harness before execution;
  no oracle relaxation. Inspect captures. One separately frozen harness correction
  is permitted; application failures remain failures. Bound implementation and
  acquisition to45 minutes, then preserve a coherent result and exact remaining work.
- Gallery HTML stays below32KiB; no new application asset or dependency. The pinned
  CSS rebuild may add the selected utility. This is not a latency/memory claim.

Evidence root: `/Volumes/m2ext/appkit-evidence/appkit-analytics-state-width.DXuqIGjn`.

## Results

Baseline and candidate are independent private source snapshots built offline with
Go1.26.3; the candidate uses the checksum-pinned Gallery CSS builder. The browser
executes exact retained actual-handler responses in a fresh no-script Chrome profile,
not a running consumer or signed-in browser. Frozen versions: Node26.3.0,
Playwright1.62.1, Chrome153.0.8010.53; manifests bind executables and source bytes.

| Fixed lane | Baseline State width / row height | Candidate State width / row height |
|---|---|---|
| Missing320/light | 59.45px /297px | 192px /77px |
| Missing390/dark | 59.45px /297px | 192px /77px |
| Missing1440/light | 309.22px /61px | 312.59px /61px |
| Complete390/dark | 65.55px /61px | 65.55px /61px |

- Original `baseline-browser`:54 checks, four width/height failures, four cells.
  Independent review found first-row-only visibility/return evidence. The one
  frozen browser correction asserts every reason's contained/visible text and
  every label's visibility after native return. It does not change the data or limits.
  `baseline-browser-reviewed` retains the same four failures; the identical driver
  in `candidate-browser-reviewed` passes54 checks/four cells, with completed assets,
  no unexpected requests and clean bounded process exit/output drain.
- Native Tab reaches the named table; measured no-input windows remain still;
  ArrowRight exposes both full reasons, ArrowLeft restores all labels, and Tab
  reaches the first detail link. Root review inspected all four candidate captures
  and the320px baseline. Reasons no longer split into one-word columns; horizontal
  travel remains an explicit tradeoff (narrow missing tables grow512px to640px).
- `red` is a retained **no-tests-run diagnostic**, not a pass. Its selector did not
  match the authored test; numeric expectations were also corrected before execution.
  `red-corrected` executes four renderer/link combinations and fails the missing
  class; `green` passes all four plus the all-present control. Source bytes survive.
- `response-comparison-final` proves all11 non-CSS response bodies are identical or
  differ only by the declared missing-cell class. All three CSVs and the complete
  recipe/detail responses are byte-identical. Headers retain exact policy, allowing
  only correctly recalculated Content-Length. CSS grows46106→46152 bytes, exactly
  the46-byte minimum-width utility. Earlier comparison diagnostics incorrectly
  required unchanged Content-Length and guessed minifier whitespace; both remain
  retained, with no application or acceptance change made to satisfy them.
- Focused analytics/viz/Gallery/catalog/MCP race suites pass (`regressions`).
  MCP/catalog/quality-plan tests and the intent budget/link/stanza subset pass
  (`final-checks`,656 links); this is not a full intent-doctor or whole-repo test run.
  The final source snapshot reproduces the tested response bundle exactly:
  SHA256`30a0b8a1aeb2ea080caca74d5ab2b4921298ea3e4bbf23aa19f6df0fe6937769`.
  The active pinned CSS rebuild matches the tested candidate byte-for-byte.
  Independent scoped diff/evidence review found no blocking issues.

The repair changes missing-row HTML even without `DetailHref`.
[ADR-0076](../adr/0076-preserve-readable-missing-state-columns.md) resolves ADR-0075's
historical link-delta guarantee explicitly. Rebuild L3b CSS; review HTML snapshots
and selected capability pins. App-owned font/zoom/language and broader density work
remain separate; the 128px ceiling applies only to these fixed65/64-character reasons.
Full quality, human A/B, real consumer propagation and G3/G4 acceptance stay open.
