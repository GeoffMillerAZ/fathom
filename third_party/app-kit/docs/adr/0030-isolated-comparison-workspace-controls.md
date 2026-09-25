# ADR-0030: Keep comparison controls outside inert reference documents

- **Status:** Accepted
- **Type:** ux
- **Date:** 2026-09-13
- **Extends:** ADR-0028

## Context

The owner's P02 screenshot shows two 1440px reference viewports clipped by narrow
Gallery columns. Matching frame dimensions alone did not make comparison useful.
The owner requests a fullscreen workspace with flipping and stacked comparisons.
This is a viewing need, not a need to execute or change the reference content.

## Decision

Use a dedicated edge-to-edge workspace at the existing P02 route. Default to one
full-width original with original/converted flipping; also offer side-by-side and
above/below layouts, fit-width and actual-size presentation, and optional browser
fullscreen including its controls. Keep viewport width/theme explicit and matched.
Fit scales a fixed 390/1440 by 1000 CSS-pixel iframe, including its surrounding
geometry; it must not resize the source's layout viewport or claim pixel equality.
Actual size means one specimen CSS pixel per page CSS pixel, independent of browser zoom.

Add only a first-party parent controller with exact served-byte SHA-256 admission
in both CSP and script integrity. No general same-origin script permission, inline
handlers, network calls, storage or frame-content/message access. Source and error
responses retain their current policies; prepared documents retain their exact bytes,
hashes, opaque-origin sandbox and no-script/form/remote-resource policy (ADR-0028).

Flip keeps both frames mounted and removes the inactive pane from layout and keyboard
navigation. Parent layout/scale/side changes need no request. Inner scroll positions
remain independent: opaque frames do not expose their document or scroll state.
Theme changes select the existing pinned document pair. Browser fullscreen requires
an explicit user action and handles unavailable APIs, rejection and exit.

GET controls and side links remain usable without the controller; fit/fullscreen
are progressive enhancements. Use no new runtime or library dependency. Do not
generalize a public comparison component before another actual consumer earns it.

## Alternatives considered

- Fluid-width frames: easier sizing but silently changes responsive layout conditions.
- Grant same-origin/script permissions for synchronized inner scrolling: unnecessary
  privilege for this static inspection task. Wins only with a separately reviewed need.
- Screenshot overlays or pixel-diff claims: require a controlled real-browser capture
  pipeline and explicit oracle review, not two equal compiler/DOM results.

## Consequences

Maintain a small parent controller and failure fallback. Scaling is inspection, not
fidelity proof. Go and Node tests cover contracts/state, not real-browser containment,
geometry, fonts, fullscreen or keyboard behavior. The reported clipping gets a recorded
incident and regression checks; visual resolution still requires browser inspection.

## Revisit when

- Browser inspection still clips a fitted preview or changes its layout breakpoint.
- A controller failure leaves no usable native comparison or traps keyboard focus.
- A second prepared case needs content-height measurement, linked scrolling or interaction.
- Browser evidence requires a different CSP or frame privilege to deliver the task.
