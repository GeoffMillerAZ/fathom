# G2 application-shell acceptance reconciliation

This assessment maps the finite [G2 shell outcome](../intent/scope.md#g2--make-a-complete-application-shell-reusable)
to the composed Gallery reference and the two owner-selected consumers. It is not
a replacement quality standard, permanent component promotion, or release gate.
**Status: G2's scoped behavior/adoption acceptance is satisfied.** Historical receipts
remain scoped to their executed inputs; complete starter certification remains open.

The exit test fixed before acquisition was: one composed shell recipe covers
primary/current/mobile navigation, appearance, account/profile, settings and help/about
entrypoints with applicable loading/save/error/permission states; consumers retain
identity, route policy and auth; two owner-selected reference apps replace overlapping
behavior and pass keyboard, narrow-screen, theme, long-label and recovery checks.
Do not invent their account backend. The evidence below satisfies those clauses without
adding a published release, alternate consumer palette or account-service requirement.

## Requirements fixed before the current replay

| G2 clause | Gallery reference | Resume as Code | Fathom |
|---|---|---|---|
| One composed primary/current/mobile navigation and appearance recipe | `/shell` composes `RenderShell`, `RenderNav`, appearance and app-owned routes | Shared navigation adapts its own `navPages`; preserve intentional Synthwave identity | Shared navigation adapts seven app-owned destinations; CSS follows system appearance |
| Account/profile, settings and help/about entrypoints | All four destinations use `AccountDestinations`; Profile/Settings use shared sections | Three Settings layout forks replaced; sensitive credential workflow remains app-owned | Read-only Observatory does not need an invented account/settings backend |
| Applicable loading/save/error/permission states | Synthetic loading, validation, conflict, failed save/retry, viewer and signed-out states | Protected native POST/303/GET, validation, non-secret recovery, already-saved state and hosted refusal | Unreadable input remains unmeasured; ordinary navigation recovers through the actual cache policy |
| Two independent apps replace overlapping behavior | Kit reference, not a third consumer | Navigation and settings presentation replace local forks | Navigation and skip-target behavior replace local forks |
| Keyboard and narrow-screen checks | Native skip/input/submit plus mixed-input account traversal at390/1440px | Historical navigation at320/768/1440px; actual Settings journey at390/768/1440px | Actual landing/fleet/detail Tab/Enter/Menu/skip at390/1440px |
| Theme checks | Light/dark and explicit appearance, including no-script fallback | Both system preferences retain Synthwave; app-owned guide separately demonstrates token changes | Actual light/dark CSS differs while preserving route, missingness and recovered values |
| Long-label checks | Separate compiled settings-section layout with long rendered copy | Separate synthetic navigation stress label, not an altered production route | Long service name originates in the synthetic registry and flows through actual handlers |
| Recovery checks | Native validation/conflict/retry plus separately retained interruption/history proof | Actual sensitive Settings error-to-save recovery; secrets are never retained like ordinary drafts | Same handlers and four browser contexts: unreadable, immediate cached error, exact3-call/2-agent recovery after unchanged30s TTL |
| Identity, route policy and auth remain app-owned | Synthetic session controller is not exported as an account service | App owns routes, icons, palette, CSRF, Origin checks, persistence and hosted refusal | App owns routes, local data visibility and read-only state; fixture controls are not product endpoints |

Separate source-bound tests may jointly prove a clause. G2 does not demand one giant
harness, an alternate theme in every app, shared identity infrastructure, permanent
promotion of every experimental helper, or a published release. Equally, a source hash
or checked document does not prove an interaction. A changed composition needs review
or replay; a missing observation stays missing.

## Existing evidence and freshness limits

- [Shared navigation delivery](2026-09-14-navigation-delivery.md): RAC2,094 and
  Fathom1,207 browser checks on inert synthetic shells, with exact artifact receipts.
  RAC's later `renderPageShell` extraction means the old whole-file identity is not
  current; inspect the actual delta before inheriting its behavior evidence.
- [Settings-section adoption](2026-09-16-settings-section-recipe.md): Gallery72
  long-text component checks and215 real-route checks; RAC165 actual-handler checks,
  exact vendored section sources and two matching offline builds. This is not OAuth,
  provider or production-data proof.
- [Fathom recovery](2026-09-16-fathom-shell-acceptance.md): final `2wUD3G` receipt
  retains four actual-handler lanes,20 captures and27 HTTP/control/cleanup checks.
  Fixed primary labels and long service labels are different observations. No cache
  reset, replacement production renderer or live call log supplied the result.
- [Shell performance](2026-09-16-shell-performance.md):30 ready/error/retry
  observations on a named binary meet local budgets, not complete G2 task coverage.
- [RAC theme guide](2026-09-15-resume-theme-contrast-and-zoom.md):105 normal and62
  actual200% checks on app-rendered specimens. This does not add an Appearance menu
  to RAC or claim that system preference changes its default Synthwave palette.

## Current acquisition

The retained read-only review at `/Volumes/m2ext/appkit-evidence/appkit-g2-source-review.gG0T9I`
matches32 identities: three historical receipts, sixteen Fathom selected sources,
seven RAC Settings handler/browser/CSS sources and six RAC adoption/vendor artifacts.
`source-review.json` SHA-256:
`6259cfaa48d0f4999387b9d65c81f75da7ef57bd120c09624e48e09b4d3ba10c`.
The executed `review.mjs` was retained before execution. Matching these bytes is not
a new runtime pass or complete dependency-closure check.

RAC's `renderPageShell` extraction preserves the prior `RenderShell` arguments; its
actual Settings journey uses that path. The older navigation snapshot does not own
the new whole-file identity. A concrete assertion gap was navigation after
Settings validation/recovery: exact destinations, current Settings and mobile Menu
keyboard behavior. The existing form checks do not assert these cross-state facts.

The separate [retained runner](fixtures/shell-acceptance/run.mjs) prepares a fresh
private Gallery build and replays the existing full forms and settings-layout probes.
It must freeze executed harness/configuration and selected source bytes before tests,
retain failures, bind generated/served CSS and the binary, and prove owned cleanup.
No historical pass is inherited by that run. Results are recorded here after execution.

### Executed Gallery replay

`/Volumes/m2ext/appkit-evidence/appkit-shell-acceptance-wESNcr/run-report.json`, SHA-256
`3d08d91e5851fddb46d1ec0dcfce74689895325f0f1ecad8c5444fa522020a0c`,
passes215 existing full-route form checks and72 separate long-text component checks.
The53 selected inputs/configuration and approved browser/module identities were frozen
before execution and unchanged afterward. Generated and served51,715-byte CSS match
`f77f7eb665a10432a0f391975004ac9cdb95fdd1874b279c24dd1b8b4fef6941`;
the private Gallery binary is
`af55b0230fdac95204382a314bb6b5220c7e27245be96d12868bb8cda24e4900`.
All six commands exit0 with no timeout/output overflow; the owned server exits0 and
its port refuses afterward. The existing browser oracles were not changed.

The form report SHA-256 is
`18c261dacc7224261454287f36fda612aaf7b71859752865dcf91b2dae9ad77b`;
the layout report is
`a6db2676570811f4aa4b53f204be0822c4586241632b4c38405ea0640232e8e6`.
Twenty-six captures are retained. A bounded Sol review of four captures covers mobile
dark/desktop light Settings, mobile conflict recovery and long-text dark layout:
no observed task-blocking clipping; recovery hierarchy and wrapped labels are readable.
Stitched fixed/sticky navigation alone is not evidence of an overlap defect. This
review does not establish computed contrast or full accessibility.

### Targeted RAC follow-up

The existing [Settings browser probe](fixtures/resume-settings/browser.mjs) now adds
the missing cross-state navigation assertions without removing the original165 form
checks. Native Tab/Enter opens and closes Menu without activating another route;
both navigation copies must retain exact app-owned destinations and current Settings.
The receipt adds the current shell and vendored navigation/frame identities. A separate
owned-Chrome identity record permits exact-process cleanup by the acquisition runner.
Two frozen runtime attempts are retained; neither is a shell acceptance pass:

- `MDdP6V` stopped before UI tasks because Chrome would not return
  `Browser.getBrowserCommandLine` without `--enable-automation`. Its report SHA-256 is
  `b6e0589b5cea3acbb24c5be5ee5cdac22d206d5af8e859d070ea11fbb2f4da48`.
  The next revision reads the exact CDP-identified process through bounded `ps`,
  without changing Chrome flags. The first attempt lacks an owned-browser receipt;
  its separate no-matching-process observation is not a complete cleanup proof.
- `emQdz2` executes269 checks:268 pass, including all165 original credential checks;
  `nojs-390-invalid: shell has no horizontal overflow` fails with125px immediately
  after reload at the native secret-field fragment. The later Menu-open observation
  is0px; the existing invalid screenshot precedes reload and cannot settle the cause.
  Receipt SHA-256:
  `7666dedbc361bfdd81d53e347c7f7b7da2c5bfe65adfccbb5bec788d39d448f0`;
  run-report SHA-256:
  `c1184f073011a6c0ae7e9251bad2eb80fecba227480a6c289b450def1fb84680`.
  All3,097 copied code inputs remain identical to the selected consumer sources;
  HEAD/dirty inventory remain unchanged. Owned Chrome exits normally; the test server
  exits0 and its port refuses. No consumer file changed.

The third frozen attempt, `IMWK5x`, reproduces the same268/269 result with the
original zero-overflow check intact. At11.4ms, the125px measurement has
`readyState=interactive`, no attached stylesheet and the shell's flex classes render
as `display:block`. At87.1ms, before any keyboard input, CSS has49 rules, the document
is complete and overflow is0px. The100ms and first-Tab samples remain0px. This
identifies an unstyled-document measurement, not evidence of persistent styled overflow.
It does not establish first-paint or degraded-network performance.
Its browser receipt SHA-256 is
`25f1adbc5090d4985f78dba1ade48d670e84c9070cac60feaef87ff3a8478473`;
run-report SHA-256 is
`561bf1e1ad8162e9bd7a963bd8c650c2cff4af06e12dff48aeaf6928ff43b802`.
All selected source/inventory and owned cleanup checks pass; no consumer edit occurred.

Each root is `/Volumes/m2ext/appkit-evidence/appkit-rac-shell-state.<suffix>` and contains the runner,
browser, selected code, pre-execution configuration and logs. The browser receipt is
`tmp/browser/receipt.json`. A non-relaxing follow-up retains geometry, stylesheet and
focus state at the original oracle, then later screenshot/time/native-Tab observations.
It does not retry the failed assertion until green. The corrected probe adds a bounded
load/CSS-readiness prerequisite before measuring styled layout; the0px threshold and
original165 checks stay intact. Missing-CSS and deliberately overflowing styled-page
controls demonstrate refusal. Earlier failures remain failed historical acquisitions,
not retroactive passes.

### Corrected RAC acceptance

`sX5bLw` passes288/288 checks:165 original credential checks,104 cross-state shell
checks,14 readiness prerequisites and five negative-control assertions. Readiness
requires bounded load completion, the single enabled/readable/applicable `/assets/app.css`
and the actual shell's computed flex layout. It never waits for overflow to disappear.
Blocking CSS fails readiness; injecting128px overflow into the styled actual-handler
page fails the unchanged geometry condition. Control contexts never submit credentials.

Browser receipt SHA-256:
`6367a3bd33a01239b164f02ad6ce6b5cc39bae662b9498737d437927c7513261`;
run-report SHA-256:
`8cdcc727302763fdbd34a8fe13efb94223445af96d511f60763117c96888f4da`.
The retained code-only copy builds the actual test handler;3,097 selected source bytes,
consumer HEAD/dirty inventory and executed input identities stay unchanged. Chrome
exits normally; the server exits0 and its port refuses. The run records55 allowed
requests,54 ordinary-lane response observations and zero unexpected browser errors;
the response count is not all control-context traffic.

A bounded Terra review of four retained captures finds no task-blocking clipping:
mobile saved/stale paths wrap, feedback remains distinct, and the desktop invalid
state has a readable alert/focus hierarchy. The post-reload diagnostic capture lacks
the one-use error alert and cannot substitute for the invalid-focus capture. This is
not computed-contrast, assistive-technology or first-paint proof.

The finite G2 acceptance now holds across the Gallery composition, actual RAC Settings
recovery and Fathom's actual read-only recovery, with historical renderer stress
evidence used only where selected bytes/behavior remain applicable. No third adopter,
new account backend, consumer palette change or permanent promotion was required.
The broader `core.shell-settings` standards/nine-gate item stays partial; G4 delivery,
G6 native parity and other open goals are not completed by this result.

Consumer source, dependencies, installed binaries, existing previews, real data and
global configuration are outside this acquisition. Full accessibility, physical/IME
input, other browser engines, general performance, release delivery and native-host
conformance retain their separate applicable gates; this matrix does not waive them.

## Audit findings

1. The shell guide incorrectly described Fathom's recovery fixture as unimplemented.
   It now links the executed result and the completed finite acceptance matrix.
2. Gallery has the required composed behavior; another wrapper would not close a
   demonstrated feature gap. Refresh the relevant runtime evidence instead.
3. RAC/Fathom's clause-by-clause reconciliation closes G2 without manufactured account
   backends or a third consumer. Synthetic and actual-handler observations remain distinct.
4. The RAC geometry test ran before CSS loaded. Retained timing/stylesheet evidence
   justified an explicit readiness prerequisite; both negative controls preserve refusal.
