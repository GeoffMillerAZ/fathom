# Native media-denial probe

Opt-in, synthetic macOS-only observation under [ADR-0046](../../docs/adr/0046-native-media-denial-in-http-only-profile.md).
Read [README](README.md) before preparing. No grants, real records, device enumeration,
capture inspection, clipboard reads, location requests or notification requests.
The ordinary browser workflow and its separate oracle remain unchanged.
[Current evidence](../../docs/assessments/2026-09-15-native-media-denial.md): camera passes;
microphone/combined time out before the delegate, so the full gate fails.

## Run

From this module, run `go run ./cmd/prepare-wails`. Substitute its exact fresh directory
for `PREPARED`; both the original HTTP-only patch and the media-denial patch are required.
Never use this probe with the uncontained baseline. Nothing is installed or signed.

```sh
go test -modfile=PREPARED/go.mod -mod=readonly -tags appkit_contained,capabilityprobe -race -count=1 .
go test -c -modfile=PREPARED/go.mod -mod=readonly -tags appkit_contained,capabilityprobe -trimpath -buildvcs=false -o PREPARED/media-probe .
APPKIT_CAPABILITY_MODE=run PREPARED/media-probe > PREPARED/media.json
APPKIT_CAPABILITY_MODE=check APPKIT_CAPABILITY_RECEIPT=PREPARED/media.json PREPARED/media-probe
```

With the mode unset, ordinary tests run without opening a window or requesting a
capability. `workflowprobe` and `capabilityprobe` are separate test binaries, never
combined. The native process may exit zero through AppKit even when a case fails;
the separate receipt check is mandatory. Retain failed receipts and exact binaries.

For a failure-isolation run only, set `APPKIT_CAPABILITY_DIAGNOSTIC_CASE` to `camera`,
`microphone` or `camera-and-microphone` with mode `run`. The receipt names that partial
selection and the full checker **always rejects it**, even if the selected case passes.

## What can count as evidence

Each of the three cases uses a fresh WKWebView and the real host's window options and
single-use HTTP bootstrap. The only page is an inert synthetic fixture without CSP or
Permissions-Policy, so those headers cannot masquerade as native enforcement.

Before a page request, main-thread instrumentation locates exactly one WebView,
checks the actual delegate and contained origin, then directly invokes the native
denial method for camera, microphone and combined media. This preflight requests no
device. Every completion must synchronously return `WKPermissionDecisionDeny` exactly
once; fresh native counters must each become one and a nil origin must not match.
Raw completion counts/results are retained alongside the derived acknowledgement.
Explicit `Show`/`Focus` and a four-second page-readiness gate precede the preflight;
the native window must be visible/key, the app active, and the page visible/focused.
Wails' navigation event alone does not establish that its independent Show listener ran.

Only a passing preflight permits `getUserMedia`. Its expected type must increment the
native counter exactly once with matching requesting origin, while the page receives
`NotAllowedError` and no stream. Missing APIs, timeouts, rejection before the delegate,
wrong origins or unexpected callbacks fail. An unexpected returned stream is stopped
immediately without reading its content and terminates further media cases as failure.

The run has a 55-second context and 60-second hard watchdog. Native shutdown must
close all three windows and the real listener, with no framework errors, warnings or
bridge messages. Unit canaries plant incomplete, reordered, granted, wrong-origin,
early-rejection and unclosed receipts. Checks establish consistency, not signed provenance.

## Explicit limits

Geolocation, notifications and clipboard-read are **presence-only** observations;
`Notification.permission` is read without requesting a change. Presence, default or
denied state is not proof that app-kit enforces those capabilities. The pinned Wails
Darwin implementation does not consume its cross-platform `Permissions` map.
No OS permission database, usage-description metadata or system preference is changed.

The evaluator is fixed-source native test instrumentation, not physical user input,
a public JS bridge, an arbitrary-script endpoint or a general permission framework.
This is not proof of grants, file picking, per-window storage, signing, recovery,
Windows/mobile or production support. Run the existing containment and Gallery workflow
oracles separately after changing the native patch; they cover different requirements.
