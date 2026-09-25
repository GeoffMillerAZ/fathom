# Safe and disabled form-button destinations

## Prospective contract

Baseline `b9146a6`. `form.RenderButton` escapes Href but accepts executable URL
schemes and ignores Disabled in link mode. Fix the existing presentation boundary,
not app-owned authorization, routing or persistence. I4/I7/I12/I15/I17 apply.

Preserve enabled valid relative references (including query and fragment) and
absolute HTTP(S). Refuse malformed URLs, protocol-relative URLs, raw backslashes,
leading/trailing whitespace, C0/DEL controls and non-web/opaque schemes. Never
normalize a refused destination into an accepted one. A nonempty unsafe or disabled
Href becomes a nonfocusable span with link role, aria-disabled and disabled styling;
no href, form fields or implicit submit. Empty Href keeps the native button contract.
No public type, dependency, script, consumer or installed-host change.

Fixed proof: parsed-DOM URL/disabled/escaping/compatibility canaries, frozen original
red and repaired green, scoped race/vet/CSS checks. Browser task in 1440/light and
390/dark, JavaScript disabled: read unavailable actions, Tab past them, activate a
relative Help destination and return, then submit the original form with native
Enter. Disabled actions must never request a destination or submit the form.
Include pointer clicks, Chrome AX disabled semantics, exact completed documents/CSS,
visible focus, no page overflow, retained captures and bounded process cleanup.
This is not route authorization, screen-reader certification or consumer acceptance.

Retain executed source, runner, fixtures, browser oracle, binaries and responses
before acquisition. Evidence root: `/Volumes/m2ext/appkit-evidence/appkit-button-safety.dLOO0tcF`.
No standards gate or weighted completion category is promoted by this packet.

## Results

`red` retains the original refusal/disabled test-group failure. Accepted destinations
and native button compatibility pass against the original. First `green` passes form,
dashboard, Gallery and qualityplan tests but fails MCP's decision-register parity
check: D79 lacked its body summary. The missing summary was added without weakening
that check. `green-repaired` runs 1,306 test/subtest events with race detection,
13 explicit skips and no failures, plus successful scoped vet, pinned CSS and offline fixture builds.
Skips include opt-in live/private/browser probes and the Git-only binary guard in the
archive without .git; these are not passes. The normal checkout guard is separate.

Chrome 153.0.8010.53 / Playwright 1.62.1 / Node v26.3.0 runs the same frozen oracle
against both response sets. Baseline fails ten of 48 checks across 1440x1000/light
and 390x844/dark: both actions remain anchors, AX exposes them as enabled, keyboard
navigation visits them, and pointer eligibility is refused by the harness. It does
not activate the baseline executable URL or claim an observed navigation exploit.

The candidate passes all 48 checks: inert labeled actions, disabled AX semantics,
actual pointer clicks without navigation/submission, Tab skip, focused relative Help,
native Help/Back and Enter submission of exact theme/display_name/action values.
Original/native submits still work; no form controller or persistence is added.
Application scripts are disabled. Exact Go-rendered document and Gallery CSS bytes
complete before inspection. No unexpected requests, forced process termination,
timeouts, unresolved children or incomplete output capture occur in either run.

Both settings/focus captures were visually inspected: disabled controls are distinct,
explanations remain readable, Help focus is visible, and the action row wraps at narrow
width without clipping. Six candidate captures and AX trees are retained. The image
reader refused external-volume access; two SHA-identical copies under
`/private/tmp/appkit-button-images.sZyncJeU` allowed inspection without policy changes.
These intercepted response tasks are not live-server lifecycle, screen-reader,
other-engine, physical-input, consumer or full accessibility/security certification.

Candidate response SHA-256:
`8e748bb4baa7b0c556f64a16377255130a1832941a446ecb426418ad01a4267b`.
Candidate CSS SHA-256:
`f5da664c9007f1bcf3e7da4e4025b1c781722ceb8a5494be696b894b142a0f2d`.
The checked-in CSS was regenerated with the pinned compiler and matches those tested
bytes. Independent production/oracle review found no material issue. Pre-execution
review corrected the native full-width class expectation and removed compensating
browser reloads; the executed baseline/candidate oracle bytes remain identical.

The final frozen replay again has no failures across 1,306 test/subtest events
(13 named skips), plus vet/build/CSS success. Final response bytes match the browser-
tested packet exactly; regenerated CSS is byte-identical to the checked-in output.
The real checkout separately passes the Git binary guard, both ADR indexes/MCP range
and current-guidance tests. Final fixture binary SHA-256:
`4416b710ca400f276c560af13a98016cc0142073244276159530dfe2ba606a8b`.
This source-archive build uses `-buildvcs=false`, not an installed or VCS-stamped release.

## Adoption handoff

Rebuild selected Go source and scanned CSS; review disabled/unsafe-link snapshots and
the app's native form task. Empty-Href buttons and enabled accepted anchors retain
their byte contracts. Non-web schemes require separately reviewed app-owned links.
No consumer source/pin, installed tool, dependency, daemon or default theme changed.
The CUE readiness record links this scoped evidence while all existing partial/full
gate statuses remain unchanged. ADR-0079 and the MCP-served shell guide document the
contract; instruction availability is not installed-host or consumer adoption proof.
