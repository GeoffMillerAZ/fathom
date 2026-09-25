# Native media denial: camera proven, audio-bearing requests unresolved

Date: 2026-09-15. G6 progress, not completion. [ADR-0046](../adr/0046-native-media-denial-in-http-only-profile.md)
extends the isolated HTTP-only experiment. **The complete three-case native gate fails.**
No permission grants, usage-description changes, signing, installation, root dependency
changes, consumer upgrades, commits or publication were performed.

## Implemented boundary

Pinned Wails beta.21's Darwin constructor does not consume its cross-platform
`Permissions` map. Windows/Linux do; the public `WKUIDelegate.h` contract defaults an
absent media callback to Prompt. This was source discovery, not an observed baseline
capture. No device request was made against the unpatched baseline.

A second checksum-pinned patch adds the public macOS 12+ media callback to the existing
delegate. A contained origin always returns Deny, including nil/mismatched origins;
uncontained behavior remains Prompt. It records native per-type denial counts and an
origin-match Boolean, with no new bridge, allow path or generic capability registry.
`prepare-wails` checks and applies the two patches sequentially in a fresh module copy.
The original patch/hash and upstream dependency checksums remain unchanged.

## Actual observations

macOS **26.6.2 (25G83)**, arm64; SDK **27.0**; Go **1.26.3**; Wails **v3.0.0-beta.21**.
The WebView reported `AppleWebKit/605.1.15 ... wails.io` in its user agent; its masked
`Intel Mac OS X 10_15_7` text is not the measured host OS/architecture.
The fixture is an inert page behind the real single-use bootstrap/loopback guard,
without CSP or Permissions-Policy that could mask native denial. No private inputs.

| Case | Actual result | Claim allowed |
|---|---|---|
| Native preflight | Three direct callbacks per fresh window, each synchronous Deny exactly once; nil origin does not match | The actual native method returns Deny for all three types; no capture API used by preflight |
| Camera | Real `getUserMedia({video:true})` reaches camera callback once with matching origin; page gets `NotAllowedError`, no stream | Scoped end-to-end camera denial passes |
| Microphone | Real request remains pending for the eight-second bound, without entering the native permission callback | **Failed/unproven**, not denied |
| Camera + microphone | Same pending/no-callback failure | **Failed/unproven**, not denied |
| Fresh-process microphone diagnostic | Microphone first, still pending; visible/focused page and active/key window confirmed | Prior camera request/order is not the sole cause; full checker rejects diagnostic receipts |
| Other capabilities | Geolocation, notification and clipboard-read APIs present; `Notification.permission` is `default` | Presence only; none requested and no enforcement inferred |
| Teardown | Three windows removed; native shutdown and listener closure; zero framework errors/warnings/bridge messages | Scoped process/HTTP cleanup passes |
| Native regressions with both patches | Existing 14 containment cases and 14 Gallery save/conflict/retry/stream/revocation DOM tasks pass their separate checkers | Those existing cases remain passing; not physical-input or full platform proof |

The final native process exited zero through AppKit; the **separate media receipt
checker exited 1**, correctly retaining microphone/combined failure. No timeout was
converted into a pass. A stream, had one unexpectedly returned, would be stopped
without inspecting its content and would stop subsequent media cases as failure.

## Probe defects diagnosed and corrected

The initial direct callback returned raw Deny but acknowledgement serialization failed:
Objective-C `@(a && b)` boxed an integer and emitted JSON `1`, which Go correctly refused
for a Boolean. A device-free Foundation reproduction established the wire-type cause.
Explicit `numberWithBool:` fixes the producer; raw call/result tuples remain mandatory.
`TestCapabilityNativeAcknowledgementUsesJSONBooleans` exercises that native serializer
with absent, exact, duplicate, Prompt and Grant outcomes. Initial failing receipts remain.

Wails' post-navigation Show handler is independently scheduled from the probe's load
notification. Instrumentation observed native visible/key/active state with an initially
hidden/unfocused page. The final driver explicitly Show/Focuses and waits for actual
page readiness before preflight/request. Audio-bearing failures persist after this
normalization, so visibility is **not** asserted to be their root cause.

Tests also reject 39 altered receipt cases, malformed/extra/unknown JSON, unselected
API calls in the fixed scripts, and partial diagnostic runs as complete proof.
Root `go test ./...`, nested server race tests, contained capability race tests and vet
pass. The inherited linker warning about duplicate `-lobjc` remains; new source warnings
were resolved. These tests do not turn the failing runtime gate green.

## Retained artifacts and replay

Runbook: [CAPABILITY-PROBE.md](../../spikes/wails-gallery/CAPABILITY-PROBE.md).
Durable local evidence directory:
`/Users/geoff/Library/Application Support/app-kit/native-evidence/2026-09-15-media-4DWopP/`.
It contains the final binary/receipt, earlier failure binaries/receipts, separate
workflow/containment binaries/receipts and a native-spike source archive. That archive
is not a complete root dependency snapshot or independently installable release.

| Artifact | SHA-256 |
|---|---|
| Original HTTP-only patch | `48adbb4904968b6ece05484262a4b8bf936a8c44c7afe53145b71fce19e98f90` |
| New media-denial patch | `859403aeb7ab3a36737f3d0f352209740fd72bb6bf496cdb1510e8011b1b1c35` |
| Final media binary | `00e7e9d3b81c35cc08958bec74d2eb5ab23999b21e7cfd644917ef8846f50894` |
| Final failing media receipt | `8bf4ffddf4c7711d7d73018d88cc4e2a6b73df74570cced4333df7fe927d6b36` |
| Native workflow binary / receipt | `3cbeef4b9bcf1331f74b859eb0a3da62b6809883fcaee94023db075c8fa7a4db` / `8e2e6d4ab6f6e725f9120eb2a4029060313b2d43652a28fc733bb1249b775585` |
| Containment binary / receipt | `ac29dc8c4de542cd1845a5d2d5132f97f26b92f532ff70b4284b061ec43d9e10` / `b15bfafe86bc508cd6651384ee167646b912b3f1e61cef9a7da1fc51d4249e7b` |
| Native-spike source archive | `c89c9d75e12257dd356609e3cd93a59e085fc57ff4efd5735d303c942dd26c45` |

Final build uses prepared stage `appkit-wails-beta21-3874078951`; regression builds use
`appkit-wails-beta21-3894262957` under the macOS temporary root. Both preparations produced
identical five patched-file hashes, including Darwin `.m`
`168d33bf782d7bb2745e230cc406bd88293737f5313ec7adf72c5c792416354b`.
No cached module was edited. Native processes completed and no test listener remains.

## Remaining G6 work

2026-09-16 bounded source follow-up: [WebKit bug259465](https://bugs.webkit.org/show_bug.cgi?id=259465)
attributes an older pending request to a hidden view, but this probe already requires
native and page visibility/focus, and its fresh-process audio diagnostic still fails.
Current [public WebKit request processing](https://github.com/WebKit/WebKit/blob/main/Source/WebKit/UIProcess/UserMediaPermissionRequestManagerProxy.cpp)
validates device constraints before the permission UI path. That suggests an earlier
stage to investigate, not a diagnosis of this installed WebKit build. No native rerun,
patch, permission change or new denial claim follows from this source review.

Determine why audio-bearing requests never reach this callback before claiming denial;
do not add grants, usage strings or weaken the oracle to force progress. The tested
profile has no admitted capture use. Geolocation's public callback requires macOS 27
runtime (not merely SDK 27); notification/clipboard enforcement remains unproven.
Physical input/IME, other capability boundaries, startup/browser recovery and applicable
packaging/update recovery remain open. Browser delivery stays first-class, with no
native grant requirement. Windows/mobile and CA/GGB remain owner-deferred.
