# Analytics guidance replay

The [assessment](../../2026-09-23-adopter-guidance.md) covers signed comparison
guidance, not a new chart or export API. All data is synthetic; no running app,
real logs, external assets or application JavaScript are used.

1. Extract tracked baseline `5107d437b17f506d9924802efa0ea0fe5abe4872` into two
   private directories. Install `fixture.go.txt` as
   `cmd/analytics-guidance-fixture/main.go` in both. Replace only
   `appkit/ui/analytics/render.go` and `diverging.go` in the candidate with the
   assessed repair. Keep the baseline's Gallery CSS as the common stylesheet.
2. Freeze complete source archives, runner, tool versions and environment before
   execution. Build each with CGO disabled using
   `go build -mod=readonly -trimpath -buildvcs=false -o FIXTURE ./cmd/analytics-guidance-fixture`.
   Execute each once with `light` and once with `dark`, retaining exact stdout HTML.
   The recorded pair used Go1.26.3 and warm reviewed caches, not a cold installation.
3. Preserve the relative path from `browser.mjs` to
   `../bar-resolution-2026-09-22/browser-owned_process.mjs`. Freeze both, the fixture
   and prospective contract before executing. Prepare a JSON manifest with `inputs`
   mapping these keys to `{ "path": "absolute path", "sha256": "hex hash" }`:
   `baselineLight`, `baselineDark`, `candidateLight`, `candidateDark`, `cssPath`,
   `pwPath` (Playwright index.mjs), `chromePath`, `node`, `frameworkPath` (actual Chrome
   framework executable), `driver`, `helper`, `fixture`, `contract`. Record exact
   `node_version`, `chrome_version`, `pw_version`; the driver verifies identities.
4. Run `ABS_NODE ABS_DRIVER ABS_MANIFEST ABS_NEW_OUTPUT` with a new private output
   directory. Capture process exit and inspect `report.json`: require exit0,
   `pass:true`, four cells, no errors/unexpected requests and clean owned termination.
   Inspect all four full-page screenshots separately.

Cells are baseline/candidate ×1440×1000/light and390×844/dark. Fresh contexts disable
scripts and service workers; only virtual loopback document/CSS GETs are fulfilled.
The driver requires exactly two explanation replacements per document and identical
remaining HTML. Expected rows are Tiny±0.01 and Large1e12, with unchanged presence,
bounds and meaning. Native Tab must focus both fallback table regions; explanation
fragments fit their containers with a fixed1px tolerance. Ordinary signed marks and
the actual long-label fallback remain. Native CSV download is not part of this task;
`guidance_test.go` proves table/CSV preservation separately.

Keep failed preparer sources and original results. This is no-script explanation,
data and focus evidence, not actual zoom, touch, screen-reader, performance, human
usability or consumer certification. No selected quality status is promoted.
