# Settings control-column reflow replay

This [bounded assessment](../../2026-09-22-settings-control-reflow.md) tests public
form controls inside the experimental Settings layout, not submission or persistence.
No live consumer data, server, external assets or application scripts are used.

1. Extract tracked baseline `34a0cc5ca6efa60ef8521c6dcacf9574dae4d198` into two
   private directories. Install `fixture.go.txt` as `cmd/form-reflow-fixture/main.go`
   in both. In the candidate only, replace `appkit/ui/form/settings.go` with the
   assessed control-column repair. Do not replace the native form primitives.
2. Freeze both source archives, exact runner, tool versions and environment before
   executing. With Go1.26.3 and reviewed cached dependencies, build each using
   `go build -mod=readonly -trimpath -buildvcs=false -o FIXTURE ./cmd/form-reflow-fixture`.
   Execute each binary once with `light` and once with `dark`, retaining exact stdout
   HTML. Preserve the baseline's Gallery CSS as the common stylesheet.
3. Preserve the relative path from `browser.mjs` to
   `../bar-resolution-2026-09-22/browser-owned_process.mjs`. Freeze both, the fixture
   and prospective contract before execution. Prepare a JSON manifest with `inputs`
   mapping exactly these keys to `{ "path": "absolute path", "sha256": "hex hash" }`:
   `baselineLight`, `baselineDark`, `candidateLight`, `candidateDark`, `cssPath`,
   `pwPath` (Playwright index.mjs), `chromePath`, `node`, `frameworkPath` (actual Chrome
   framework executable), `driver`, `helper`, `fixture`, `contract`. Also record exact
   `node_version`, `chrome_version`, `pw_version`; never assume the launcher identifies
   an unchanged framework. The driver verifies inputs before launching.
4. Execute the frozen driver with that manifest and a **new** private output directory:
   `ABS_NODE ABS_DRIVER ABS_MANIFEST ABS_NEW_OUTPUT`. Capture process exit, then read
   `report.json`; success requires exit0, `pass:true`, eight cells, zero errors and
   clean owned-process termination. Inspect the six retained screenshots separately.

Each original/candidate uses390×844 and1440×1000 in light/dark, with fresh contexts,
scripts disabled and service workers blocked. Only virtual loopback document/CSS GETs
are fulfilled. Native Tab/ArrowRight/type/Space drive inputs; no DOM value writes.
Eight exact long strings must overflow in the original and wrap in the candidate.
Short-field dimensions/in-column alignment, complete text, native input width and
label/ARIA associations stay intact. The1px tolerance is frozen; the12px native-input
width floor is a shrinkage regression check, **not** an accessibility target standard.

Keep failed acquisitions and exact inputs. The original driver failure and its one
correction are retained in the assessment's private packet. This fixture does not
prove standalone controls, actual200% zoom, touch targets, assistive technology,
select popups, save/recovery or consumer/full-quality acceptance. Rebuild adopters
from their reviewed kit source and scanned CSS; no consumer upgrade is automatic.
