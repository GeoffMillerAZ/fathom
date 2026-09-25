# ADR-0046: Deny media capture in the strict macOS HTTP-only profile

- **Status:** Accepted
- **Type:** platform
- **Date:** 2026-09-15
- **Extends:** ADR-0035; supersedes no record

## Context

Pinned Wails v3 beta.21 consumes `WebviewWindowOptions.Permissions` on Windows and
Linux, but its Darwin window constructor does not consume that map. The installed
public `WKUIDelegate.h` contract says that omitting the camera/microphone permission
callback leaves the decision as `Prompt`. Source inspection establishes this platform
gap, not that capture was requested or granted. Existing app-kit guidance accurately
described native permissions as configured but unexercised.

The strict HTTP-only experiment already carries one pinned patch for navigation,
bridge and runtime containment. Its nonempty exact origin identifies the one profile
where native media capture has no admitted use and an ambient prompt would contradict
the profile's narrow authority.

## Decision

Add a second pinned patch to the same isolated prepared Wails copy. Preserve the
original HTTP-only patch and its hash as an independent prerequisite. When
`HTTPOnlyOrigin` is nonempty, install the public macOS 12+ media-capture permission
callback and always deny camera, microphone and combined camera/microphone requests.
Record callback counts and whether the requesting origin matches for the probe. The ordinary,
non-contained profile retains Wails' documented `Prompt` behavior.

Do not build a general permission framework or an allow path. Native preflight and an
actual rejected `getUserMedia` attempt must both succeed before documentation claims
denial. The attempt must reach the callback with the expected origin and counters.
A missing API, rejection before the callback, timeout or absent result is not a pass.
Do not request baseline device access, change OS grants or add usage-description keys.

Geolocation remains presence-only and unproven: its public callback requires the
macOS 27 runtime while the measured runtime is macOS 26.6.2 (SDK 27). Notifications and clipboard
also remain presence-only and unproven. The probe never requests any of them. This
record adds no plist changes, signing, installation, Windows/mobile work, root module
dependency, public app-kit API or consumer change.

## Alternatives rejected

- Trust `Permissions` uniformly: the pinned Darwin constructor ignores it.
- Treat an early JavaScript rejection as denial proof: it never proves native policy ran.
- Request camera/microphone in the baseline for comparison: unnecessary device and OS
  authority for a profile whose only admitted result is denial.
- Generalize permissions now: no allowed capability or second profile needs that surface.

## Revisit when

- Wails supplies effective supported Darwin controls; rerun the same native oracle.
- The strict profile's media callback is bypassed, misattributes origin or fails denial.
- A named native task requires a currently denied capability; design and prove that
  capability separately rather than weakening the HTTP-only default.
