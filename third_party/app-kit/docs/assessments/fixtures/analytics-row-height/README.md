# Row-height regression replay

Assessment-only synthetic fixture for the [bounded contract](../../2026-09-22-analytics-row-height.md).
No consumer data, purchased assets, installed browser profile or app scripts.

1. Freeze baseline and candidate source before execution. The original pair uses
   app-kit `a9e375fa566460eabdd4bd3e619443402482e5c6`; only
   `appkit/ui/analytics/{render,diverging}.go` differ in the candidate.
2. In each isolated source copy, copy `fixture.go.txt` to
   `cmd/rowheight-fixture/main.go`. Build that package with Go 1.26.3,
   `GOWORK=off GOTOOLCHAIN=local CGO_ENABLED=0 GOPROXY=off GOSUMDB=off`,
   `-mod=readonly -trimpath -buildvcs=false`. Supply the same frozen Gallery CSS.
   Existing dependency/cache contents are prerequisites, not a cold-build claim.
3. Freeze `browser.mjs` and its relative helper
   `../bar-resolution-2026-09-22/browser-owned_process.mjs`, preserving that layout.
   The manifest's `inputs` has exactly 11 `{path, sha256}` objects:
   `baselineBin`, `candidateBin`, `cssPath`, `pwPath`, `chromePath`, `node`,
   `frameworkPath`, `driver`, `helper`, `fixture`, `contract`.
   Paths must be absolute; `driver` must name the actual executed frozen file.
   Pin `node_version`, `chrome_version`, `pw_version` to reviewed runtime versions.
4. Run the frozen driver with the pinned Node executable:

   ```sh
   /absolute/pinned/node /absolute/frozen/browser.mjs /absolute/manifest.json /absolute/new-output
   ```

The driver creates fresh script-disabled browser contexts and allowlists only the
selected loopback document/CSS/native-download routes. It owns both synthetic
servers, their 90-second lifetimes and a 120-second browser owner watchdog. No existing
server, signed-in profile or installed binary is modified.

`report.json` must show all 32 cells, zero errors/unexpected requests and clean
unforced process/capture completion. It records 224 checks for the original task.
The baseline must positively reproduce overlap/clipping; the candidate must retain
exact table/CSV values, fitting-chart HTML and native narrow-table keyboard access.
Inspect the selected PNGs as well. Failure stays failure; do not broaden the route
allowlist, change geometry tolerance or replace native input to obtain a pass.

The original private packet at `/Volumes/m2ext/appkit-evidence/appkit-row-height.c68n7FlL` includes
the pre-execution manifest and complete inputs/build runners. Its runtime versions
are Node 26.3.0, Chrome 153.0.8010.53, Playwright 1.62.1. A later runtime/source pair needs
its own dated result; it does not rewrite this evidence or certify consumer fonts.
