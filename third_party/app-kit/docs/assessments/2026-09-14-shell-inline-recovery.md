# Inline shell recovery during a pending native save — 2026-09-14

## Result

The Gallery's existing Resume control can be reached and activated by automated
keyboard input while its native form submission is still pending. The inline lane
passes **977/977** checks across 24 journeys; an identical fresh-context repeat also
passes **977/977**. No production code changed for this proof.

The matrix covers Profile, Settings selections and a checkbox-only Settings draft,
before dispatch and after server acceptance, light and dark schemes, and 390×1000 and
1440×1000 CSS-pixel viewports. It uses Chrome 152.0.7977.84 through isolated
Playwright 1.63.0 against the fictional process-memory Gallery.

## Causal boundary

The bounded loopback transport first proves that the native POST is held in the
selected stage. CDP `Input.dispatchKeyEvent` sends Shift+Tab from the disabled Save
control, then a screenshot records the pending UI after traversal. The held downstream
connection must remain open for 200 ms before Enter is sent. The test then waits for
the browser to close that connection **before** releasing the fixture. This lane does
not call `Page.stopLoading`, directly invoke the app handler or inspect/mutate the DOM
through Runtime while navigation is pending.

After the disconnect, the existing oracle requires usable controls, outcome-unknown
copy, the retained draft and original version, and focus on the first changed enabled
field. An independent tab proves the saved value and version. Exact POST counts prove
no automatic replay: a before-dispatch retry succeeds with 303; an after-acceptance
retry gets 409, retains every revised field for review, then saves once with 303.
Unchecked-checkbox omission and both Settings selections are checked from the
synthetic request body; cookies and CSRF values are not recorded.

Independent source/receipt review found no false causal or filter pass in this scope.
The transport's interruption promise resolves only when the held downstream response
closes before owner release/cleanup. Wrong activation is additionally rejected by the
recovered state, focus, independent reads, version transitions and exact write counts.

## Reproduction modes

From the approved setup in the [browser guide](../../apps/gallery/browser/README.md):

```sh
# Existing default: browser-level Stop, followed by keyboard Resume.
node apps/gallery/browser/shell_recovery.mjs

# Keyboard-activate inline Resume while the native navigation is pending.
APPKIT_RECOVERY_INPUT=inline-resume node apps/gallery/browser/shell_recovery.mjs

# Diagnostic only: one exact reported case name, not the full matrix.
APPKIT_RECOVERY_INPUT=inline-resume \
  APPKIT_RECOVERY_CASE=profile-before-dispatch-light-390 \
  node apps/gallery/browser/shell_recovery.mjs
```

`APPKIT_RECOVERY_INPUT` accepts only `browser-stop` (the default) or
`inline-resume`. An unmatched `APPKIT_RECOVERY_CASE` runs zero cases and fails the
exact selected-case count; a selected case is diagnostic evidence, not a matrix pass.
The full default Stop regression remains green at **1025/1025** across 24 journeys.

## Retained identities

Temporary artifacts are evidence, not backups:

| Artifact relative to `/var/folders/2r/2zrkv4fd549bnlc5vwfzd4b00000gn/T/` | SHA-256 |
|---|---|
| `appkit-shell-browser-EpsM02/shell-recovery-inline-activation-report.json` | `9a9b44dd4040ecef252455760fb474df4837cd2c0f5fcb30c162814a368a266d` |
| `appkit-shell-browser-cXVQhW/shell-recovery-inline-repeat-report.json` | `aadfc0122b87c10f774cfd0ceaeab068fbb4dea0cd3375db3ab82d893ba108d7` |
| `appkit-shell-browser-l20W7a/shell-recovery-inline-stop-regression-report.json` | `19cbdcd859d569f064cc6d023ad2c4478efbbe54f03823b166ee071a7f68fe51` |
| `appkit-history-recovery.2oyhm9064c/gallery-candidate` | `be85769d7a70906d0e2f15c75bfd83ea925136a4ffcbf6466a1fcb73df1d048d` |

Harness SHA-256:
`a3dd6408365706dde70888ce8e339a5b8bbbca635c0b6476feda07c12f0c011c`.

## Limits

This is automated CDP keyboard input against one pinned Chrome build, not physical
keyboard, browser-toolbar, other-engine, native-host, consumer or production proof.
The proxy models two controlled interruption points, not every network failure.
Screenshots are retained observations, not complete accessibility or visual
certification. No claim covers typing interruption, screen readers, contrast, 200%
zoom, real authentication or persistence. This evidence does not complete G2 or its
two-consumer adoption gate.
