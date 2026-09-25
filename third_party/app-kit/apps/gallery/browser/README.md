# Opt-in synthetic shell browser checks

These probes exercise a running local Gallery, not a consumer or production account.
They create fresh temporary browser contexts and synthetic process-memory sessions.
Ordinary POST actions save fictional profiles/preferences, change laboratory fixtures,
sign out/in and recover failures. No user profile, portal, existing cookies, provider,
database, dependency installation, shared process restart or deployment is used.

First start an explicitly approved local Gallery build. Reuse an approved isolated
Playwright installation and installed Chromium-compatible browser; do not install one
implicitly. From the app-kit root, substitute the actual local origin and module path:

```sh
export APPKIT_SHELL_BROWSER_ORIGIN=http://127.0.0.1:18780
export APPKIT_PLAYWRIGHT_MODULE=/absolute/approved/node_modules/playwright/index.mjs
export APPKIT_BROWSER_EXECUTABLE='/Applications/Google Chrome.app/Contents/MacOS/Google Chrome'
export APPKIT_BROWSER_VARIANT=candidate
node apps/gallery/browser/shell_forms.mjs
node apps/gallery/browser/shell_observer.mjs
node apps/gallery/browser/shell_recovery.mjs
node apps/gallery/browser/shell_history.mjs
```

Configuration fails before browser launch unless the origin is an exact HTTP loopback
IP plus non-default port, with no extra URL data. Each run prints its fresh temporary
evidence directory: JSON results, browser version and synthetic screenshots. Exit 1
means a failed assertion or execution; absent prerequisites are not passes. Requests
are allowlisted to the chosen origin's exact shell routes/assets; WebSockets are refused.
Reports omit cookie/CSRF values. Response error logs are retained; only the explicitly
expected protected-route errors are separated from unexpected console/CSP failures.

`shell_forms.mjs` covers native keyboard skip/edit/submit, pointer/API fixture setup,
validation, two-tab conflicts, retries, denied drafts, recovery, browser-local appearance,
390/1440 px layouts, help/about and separate no-script saves. `shell_observer.mjs` runs
the actual declarative Datastar action under served CSP, then checks update delivery,
unchanged drafts, stop/manual reconnect and second-tab sign-out redaction.

The forms probe also checks all four account-menu destinations/current selection with
Tab,44px targets and viewport bounds in both schemes/widths, with scripts enabled and
disabled. Pointer opens the native disclosure; this is mixed-input proof, not a complete
keyboard/contrast audit. [Recipe evidence](../../../docs/assessments/2026-09-16-account-presentation-recipe.md).

The forms probe also requires one truthful appearance fallback at390/1440px in both
no-script schemes and actual saved status after controller initialization. Native form
navigation is armed before submission, not inferred from an already-loaded old page.
[Appearance repair evidence](../../../docs/assessments/2026-09-16-exact-gallery-browser.md#working-tree-appearance-repair)
is a working-tree build, separate from the preceding exact private-candidate rehearsal.

`shell_recovery.mjs` covers 24 Profile/Settings interruption journeys through a bounded
loopback test proxy: before forwarding and after acceptance, both schemes at 390/1440px.
Settings includes all-control and checkbox-only drafts. Exact native submissions prove
unchecked omission, both selections, original/reviewed versions and no automatic replay.
The default `APPKIT_RECOVERY_INPUT=browser-stop` invokes browser Stop, waits for actual
client disconnect, then keyboard-activates Resume. Set
`APPKIT_RECOVERY_INPUT=inline-resume` to use raw automated CDP Shift+Tab/Enter while
the native navigation remains pending; that mode requires the actual downstream close
before fixture release. Both modes verify retained drafts, focus, exact writes and
explicit conflict review. `APPKIT_RECOVERY_CASE` may select one exact reported case
name for diagnosis; an unknown name fails and a partial run is not the matrix. The
proxy preserves app authority headers; no handler or save state is replaced.
[Profile evidence](../../../docs/assessments/2026-09-14-shell-save-recovery.md),
[expanded Settings evidence](../../../docs/assessments/2026-09-14-shell-settings-recovery.md),
[inline activation evidence](../../../docs/assessments/2026-09-14-shell-inline-recovery.md).

`APPKIT_RECOVERY_INPUT=typing` runs eight Profile-only stage/theme/width cases. Enter
submits from the focused text field; raw CDP text insertion interrupts the held request
before fixture release. Continued typing and baseline reversion must retain uncertainty,
focus, version and explicit retry. A real attempted navigation and dismissed beforeunload
dialog must retain that state; exact POSTs and independent reads prove conflict/review.
[Typing evidence](../../../docs/assessments/2026-09-15-shell-typing-recovery.md) distinguishes
automated ASCII Profile input from physical keyboard or IME.

`APPKIT_RECOVERY_INPUT=settings-input` runs24 cases: time-zone select, starting-page
select and digest checkbox each independently interrupt a held native Save, at both
stages/themes/widths. Raw Shift+Tab reaches the field; letter type-ahead or Space edits
it. A200ms connected hold precedes input; actual downstream close must precede release.
Select edits use a two-second prefix gap, not a performance target. Whole-field snapshots,
continued input, baseline reversion, page-bound canceled departure and explicit retry
remain mandatory. [Settings evidence](../../../docs/assessments/2026-09-15-shell-settings-input.md)
records24 journeys/1,218 checks and the old-binary failure. Select popup/arrow/pointer
paths, physical keyboard, IME and other engines remain unproven.

`shell_history.mjs` distinguishes actual BFCache restoration from network Back/Forward
across32 Profile/Settings lanes, including scripts disabled, both schemes and widths.
It checks exact native departure decisions, coherent draft/version pairs, second-tab
changes, conflict review, POST counts and independent saved-state reads. Mobile controls
use automated browser wheel input and hit-tested clicks; no forced click or synthetic pageshow.
`APPKIT_HISTORY_LANE` may select one exact reported lane name for diagnosis; a partial
run is not the matrix. [History evidence and native-policy limits](../../../docs/assessments/2026-09-14-shell-history.md).
Use a fresh owned Gallery process for each full matrix: synthetic sessions intentionally
survive browser-context closure until their30-minute expiry, with a128-session cap.
Admission503 is a failed run, never a skip or permission to restart a shared server.

The dependency-free config, recovery-assertion and transport Node tests are in `task check`.

`settings_sections.mjs` separately verifies a Go-rendered component fixture against the
running Gallery's CSS, with no page scripts or form submission. Export into a new private
directory (the fixed output filename must not already exist):

```sh
APPKIT_PROBE_SETTINGS_SECTIONS_OUT=/absolute/existing/private/directory go test ./appkit/ui/form -run '^TestExportSettingsSectionsFixture$' -count=1 -v
```

Then set `APPKIT_SETTINGS_FIXTURE` to its `settings-sections.html` and run
`node apps/gallery/browser/settings_sections.mjs` with the browser environment above.
The virtual route is supplied by Playwright, not Gallery. This probe checks long-text
Range bounds, narrow/wide layout, heading/label relationships and native Tab focus at
390/1440px in both schemes. It supplements, never replaces, real-route form/save tests.

The browser runs are deliberately opt-in and are **not** part of that automatic gate.
Review screenshots separately. A full-page screenshot can show fixed navigation midway
down the stitched image; inspect viewport scrolling before calling that content clipping.
These fixtures do not certify complete keyboard/screen-reader behavior, contrast, 200%
browser zoom, latency, native-host conformance, real-consumer acceptance or production auth.
Remaining IME/select popup/pointer, visibility/capacity and full-input cases: [shell guide](../../../docs/design/application-shell.md).
