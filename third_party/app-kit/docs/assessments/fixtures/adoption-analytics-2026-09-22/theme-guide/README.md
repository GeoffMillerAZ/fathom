# Root-kit native export diagnostic

## Prospective boundary

Starting app-kit revision: `f905a33`. This is a root-kit control for the
[retained Fathom failure](../../adoption-analytics-2026-09-21/fathom/browser-results.md),
not a retry against that consumer or evidence of its adoption.

One reviewed radius candidate, one native pointer activation and one focused Enter
activation at 1440 × 1000, light preference, reduced motion. Capture form ownership,
validation, trusted input/submit events, POSTs, responses and actual ZIP downloads.
Observation listeners must not cancel events or submit the form themselves. No
handler, Origin, CSP, sandbox or cache-policy workaround; no real data or external
page requests. At most one correction for a demonstrated harness defect, preserving
its executed inputs and failed output first. Stop the exact test-owned fixture.

Use `TestAnalyticsAdoptionBrowserFixture` with `APPKIT_ANALYTICS_BROWSER=1`, compiled
before launch. Its random loopback origin is printed on readiness; lifetime is capped
at 180 seconds. Run `export-activation.mjs` with the installed Playwright module,
Chrome executable, that exact origin and a new absolute output directory. The driver
requests authenticated fixture shutdown; no existing listener is touched.

Private evidence root: `/Volumes/m2ext/appkit-evidence/appkit-theme-export.pB6sAv`.

| Input | SHA-256 |
|---|---|
| `inputs-02.tar` | `c62bfa3a683592753aa780fd1118ca816c6c56dbe65a795a8fb428dfe919973a` |
| `gallery.test` | `8cb787937568fdd02c247446c86a0fa4042a73312fa5f70119d9fc3c588cdddb` |

`inputs-02.tar` freezes styleguide, Gallery, UI and token source, module files and
the browser driver. The first archive attempt named a nonexistent source file;
`inputs-01.tar` is retained as incomplete preparation, not qualified inputs.
Browser selection: installed Chrome153.0.8010.53; Node26.3.0. The sole fixture
change flushes its stop acknowledgment before closing client connections, the
harness-only correction already demonstrated by RAC's retained run-02/run-03.

## Result

Run-01 completed without a browser retry. Both native actions produced trusted
submit/formdata events, same-origin export POST200 responses and saved attachments.
Each ZIP is 14,967 bytes with SHA-256
`896b409ef8ac2a32a816041044778e9b472d6c7a1ee8023283e43e8f9cce101c`.
Report: `run-01/report.json`, SHA-256
`bacb619d8fd29b9e027607b39610b4c7dec07779f4c756f9411d8692317bae77`.

The form is valid, owns the enabled submit button and retains a64-character digest.
Its JavaScript `action` property is still an object because of named controls: that
condition alone does not prevent native export in this root-kit control. Both
attachment requests also emitted `net::ERR_ABORTED`; the independently saved ZIPs,
HTTP200 attachment responses and matching response hashes establish these downloads,
not a general rule to ignore failed requests. No external requests were recorded.

Post-acquisition `unzip -t` passed all five entries. An independent content check
verified the four receipt file hashes/sizes, exact original source and sole radius
change from0.375rem to0.75rem. Its first inspection incorrectly assumed a top-level
`radius` instead of `tokens.radius`, producing a TypeError after the hash/source
checks. Exact failed inline source is retained in `zip-inspection-01.mjs`; replay
with only `.radius.md` changed to `.tokens.radius.md` passes. This inspection error
does not change the executed browser driver, candidate or downloaded bytes.

The driver reports `fixtureStopped:true`, the Go fixture exits0, and no listener
remains on its port54906. The after-export screenshot was inspected for the export
control only; its scrolled iframe is not a full specimen or visual-fidelity verdict.
`observationComplete` denotes diagnostic collection, not a whole quality-gate pass.

The historical Fathom failure remains unresolved: its executable, specimen, CSS and
workflow differ. When that consumer is available, capture the same native event/form
observations on its frozen candidate before changing either its handler or policy.
Do not infer propagation, consumer success, accessibility or memory acceptance here.

## Focused checks

`node --check export-activation.mjs` passes. `go test ./apps/styleguide ./apps/gallery
-count=1` initially failed because existing token/CSS/module files returned macOS
`operation not permitted`. One unchanged-source retry passed both packages; no
permission or configuration change was made. Strict Fathom verification of the
four existing plans reports56 total,40 claimed,40 verified,0 failing: static contract
evidence, not runtime acceptance of G3/G4. No consumer files changed in this control.
