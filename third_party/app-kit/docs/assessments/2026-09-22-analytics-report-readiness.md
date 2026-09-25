# Analytics report readiness: bounded browser performance

## Prospective contract

- Need: G3 and `core.analytics` lack end-to-end useful-report/update measurements.
  Existing DOM-ready and in-process budgets remain unchanged. Measure the existing
  Gallery composition before proposing any optimization (ADR-0069/0072; I8/I11/I15/I17).
- Subject: root `39b0030`, a private Gallery build from its tracked source archive,
  existing compiled CSS, synthetic calendar/service fixtures only. No CSS regeneration,
  app behavior change, consumer files, paid corpus, real data, install or deployment.
- Task: September/API/previous/complete overview, then native form selection of
  August/API/no-comparison/partial, Apply, Gateway detail and Back. Initial exact
  current Gateway/Search counts are4/1; previous August counts6/3, both2 fewer,
  total5 versus9. Apply/Back require exactly the displayed strings `At least 4` and
  `At least 2`, partial/lower-bound qualification and no previous comparison, not a
  numeric ≥ test. Detail requires only Gateway's exact `At least 4` plus the same
  partial qualification. Every phase retains its full period/category/comparison/
  state/theme selection, failed-operation unit, UTC month and source/coverage
  disclosure: September's complete month versus August's first20days as lower bounds.
  Independently fixed expected rows apply to all four phases. This does not retest CSV.
- Cells: three fresh contexts each at1440/light/ordinary and390/dark/degraded,
  height1000, reduced motion, application scripts disabled. Disable HTTP cache in
  every cell; no warm-cache claim. Degraded CDP settings:100ms latency,200,000B/s
  download,100,000B/s upload, no CPU throttle. These are emulated conditions on the
  named Mac, not mobile hardware or production network evidence.
- Timed phases: initial navigation; native Apply; native detail; native Back.
  Start immediately before the navigation/action. End only after the new document
  and exact CSS responses fully complete, correct task values/selection and usable
  computed layout are verified, fonts settle, and two animation frames occur.
  Record this as **observed styled-report readiness**, including automation/oracle
  overhead. It is not LCP, INP, physical paint time or human task completion speed.
- Budgets, fixed before acquisition: every phase ≤500ms ordinary / ≤2500ms degraded;
  HTML ≤32KiB and complete decoded document+CSS ≤160KiB per phase. Require exactly
  the intended GET document and `/static/app.css`, HTTP200, matching source CSS hash,
  no unfinished body, unexpected request, page/console error or hidden/clipped task
  result. Retain all individual samples; three trials do not establish a percentile
  distribution, speedup, production SLO or the full converted-core performance gate.
- Integrity: freeze this contract, both drivers and reused ownership/barrier helpers
  before execution. Record binary, CSS, source archive, Node, Chrome and Playwright
  entry/package versions/hashes before and after. Retain raw response metadata,
  timing/task checks and one post-task screenshot per cell.
- Ownership: start one copied Gallery on a printed ephemeral loopback address and
  one fresh Chrome process. Bound actions, close calls, exit and pipe-drain waits;
  forced termination, truncation, unresolved child or remaining owned listener fails.
  Leave existing previews/profiles/listeners untouched.
- Stop: one six-journey acquisition, at most one correction replay for a demonstrated
  harness defect with failed executed inputs retained. No full theme/state matrix.
  A real failure remains failed and supplies a concrete next repair, not a relaxed
  budget. No speculative optimization if the selected workload meets its budgets.

## Result

The corrected `run-02` passes **six journeys /24 timed phases**, with the same
executable, CSS, source archive, budgets and data oracle as the failed acquisition.
It uses the instrumentation correction recorded below, not the same browser setting.

| Phase (maximum of3 trials, ms) |1440/light ordinary|390/dark degraded|
|---|---:|---:|
| Initial report |103.4|595.0|
| Apply filters |77.8|583.6|
| Gateway detail |62.5|573.2|
| Return to selected overview |71.9|574.2|
| Fixed per-phase ceiling |500|2500|

Maxima above round upward to0.1ms. Every phase has exactly two complete HTTP200
responses, exact source CSS, script-denying CSP and verified task/style/geometry
facts. Maximum decoded HTML is29,813B; document plus CSS is81,186B (ordinary) and
81,139B (degraded), below the32KiB/160KiB limits. No application optimization is
warranted by this selected workload. In-process rendering evidence stays separate.

Chrome153.0.8010.53, Node26.3.0, Playwright1.62.1; all12 selected input hashes match
before/after. Chrome, probe and Gallery exit0, with complete output drain, no force
or unresolved child; Gallery receives its owned SIGTERM and port58784 refuses
connection afterwards. Independent review confirms raw phase/payload/identity/cleanup
records. No consumer or production code changed.

Six viewport screenshots are retained; hashes are identical across the three trials
within each cell. Main review inspected one per cell: upper-page controls, selected
month/scheme and partial-state copy, not full-page/chart visual certification.
Plot text containment and table scrolling structure are measured by the DOM oracle;
this acquisition does not supply human comprehension or screen-reader evidence.

| Artifact under the private root | SHA-256 |
|---|---|
| `inputs-02.tar` (executed contract/drivers/helpers) | `15e820d2b110c7df0de67612eae9e28b4d449a50e23f8521be537e10ce36f1f8` |
| `run-02/report.json` | `92d6fbaabea9459aa6e2e90cfaf45bbdbbab32ecaa6e72053cbff650e9fb92e5` |
| `run-02/browser/report.json` | `cd5244ccb930cba2e6e13aa57b18b61b62d7259b105a775665767f78a0888cf1` |

Human A/B, second-app acceptance, memory/cancellation, warm-cache behavior, physical
devices, full accessibility/performance and reviewed versioned propagation remain
independent open requirements; this cannot complete G3/G4 or accept core.analytics.
Reuse this receipt when its exact subject and scope apply; do not repeat acquisition
without a changed requirement, artifact or decision-changing regression hypothesis.

## Retained first failure and instrumentation correction

`run-01` stopped in the initial phase at the75s workload watchdog. Exact HTTP200
document/CSS bodies, source hash, task values, scheme/type canaries and geometry had
passed; `document.fonts.status` was `loaded`. The asynchronous font/two-frame wait
did not resolve with the browser-wide JavaScript engine disabled. This is a failed
acquisition, not a valid app-latency sample or proof of a particular callback's cause.
Chrome and Gallery exited0 without forced termination, all pipes drained, and the
owned port57683 refused connection. The probe correctly exited1.

The only correction replay permits the browser engine for DevTools measurement
callbacks. Application scripts remain forbidden by the **unchanged** exact response
CSP (`default-src 'none'; style-src 'self'; base-uri 'none'; form-action 'self';
frame-ancestors 'none'`), zero app script nodes and exact document+CSS request closure,
all asserted in every phase. Font/two-frame settlement now has its own3s watchdog;
the500/2500ms timing budgets, asset limits, data/layout oracle and native task remain
unchanged. This is a recorded harness configuration change, not a retrospective pass.
`inputs-01.tar` retains the failed executed contract/drivers/helpers, SHA-256
`4e1a4ef5cee959fb6aea94cf7f6f12199782e3f50e6e3b509d3e304982ec91b3`.
Failed owner receipt SHA-256:
`1082f58565843f9302fc097b7225c73598fee2b8a536a953bb5e9dfae24a84be`;
failed browser receipt:
`176e6815cd8d7dd4a6ef6bc4a1b9b824a3685f7e4a2518a966f0fbad6f90d613`.

## Prepared subject

Private root: `/Volumes/m2ext/appkit-evidence/appkit-analytics-readiness-perf.X7Heq5`.
`source-39b0030.tar` is the tracked source archive, SHA-256
`ad15e73d51ebb264368861c0524936f61be959e60e649b5d5cb3b045fbb6d8b5`.
Built from its extracted `src/` with Go1.26.3, `GOWORK=off GOTOOLCHAIN=local
GOPROXY=off GOSUMDB=off go build -trimpath -o ../gallery ./apps/gallery/cmd/gallery`.
This uses the existing local module cache, not an archived dependency closure or
a two-cold-build reproducibility claim. Build exit0; stdout/stderr are retained.
Executable SHA-256 `807127451ca088a3334d3b2dc04d66cb52758675cd6bea69b67d6344bcbe6d20`.
Compiled CSS is51,373bytes, SHA-256
`94589496d9aa1e0e72c629b316df2c2629104af3dec2dda27e5a73726179a01b`.
Host: Mac16,10,10 logical CPUs,32GiB, macOS26.6.2 build25G83.

Maintained runner: [report-readiness-owner.mjs](fixtures/gallery-analytics/report-readiness-owner.mjs),
arguments: copied Gallery binary, compiled CSS, source archive, approved Playwright
module entry, Chrome executable, new absolute evidence directory. It owns only the
children it starts. Restore the source and selected frozen inputs for historical
replay; later documentation edits must not replace executed contract bytes.

Final checks: both driver syntax checks, `go test ./internal/qualityplan
./cmd/appkit-mcp ./apps/gallery -count=1`, `git diff --check`,263 relative document
targets and scoped intent/changelog budgets pass. This is not a full intent/host audit;
running Gallery/MCP binaries must be rebuilt to expose updated embedded readiness.
