# Actual macOS containment probe

This is a synthetic WKWebView boundary test, not a Gallery UI, accessibility or
platform-certification test. [ADR-0035](../../docs/adr/0035-reproducible-macos-http-only-experiment.md)
defines its scope. No signed-in browser profile, paid source, real records, file reads,
OS URL handlers or native privileged actions are used. Windows is deferred.

## Build and run

From `spikes/wails-gallery`, run `go run ./cmd/prepare-wails`. It requires the pinned
dependencies already cached, verifies them offline, and prints the fresh stage's
`modFile`. Below, replace `PREPARED` with that exact directory. Do not reuse an old
stage after source/patch changes; preparation refuses drifted identities.

```sh
go test -tags nativeprobe ./cmd/native-probe -count=1
go build -tags nativeprobe -trimpath -buildvcs=false -o PREPARED/baseline ./cmd/native-probe
go build -modfile=PREPARED/go.mod -tags nativeprobe,appkit_contained -trimpath -buildvcs=false -o PREPARED/contained ./cmd/native-probe
PREPARED/baseline -allow-uncontained-baseline > PREPARED/baseline.json
PREPARED/baseline -check PREPARED/baseline.json
PREPARED/contained > PREPARED/contained.json
PREPARED/contained -check PREPARED/contained.json
```

The baseline check is expected to exit **1**; retain that failure. The contained check
must exit **0**. Neither a missing JSON file nor the native process's exit 0 is a pass.
The macOS termination path may never return through Go code after `app.Run`: the
shutdown callback closes listeners and writes the receipt; `-check` independently
recomputes outcomes. Tests plant incomplete/altered receipts and missing controls.

Each run opens temporary synthetic windows sequentially and self-closes. A hard
deadline exits with failure if native execution stalls. Do not run both binaries at
once or interrupt the run and label its partial output completed.

## What the oracle observes

| Cases | Expected |
|---|---|
| Bootstrap, same-origin redirect, reload | Actual requests, synthetic HttpOnly cookie, JS start and required continuation |
| Foreign port/host/redirect | Attempt executes; original page remains; destination receives no request |
| `about:`, `data:`, `wails:` navigation | Original test page remains after the attempted navigation |
| Same-origin and foreign frames | Source page executes; frame destination receives no request |
| Same-origin and foreign popups | Attempt returns no window; destination receives no request; source remains |
| Native bridge | Direct harmless script-message attempt unavailable, no host canary receipt, no injected runtime globals |
| Case/application shutdown | Prior window removed before next; completed native shutdown callback; no framework errors/warnings |

The source page intentionally has no CSP, so page policy cannot mask a native-policy
failure. Reports require exact Host/Origin, loopback peer, a fresh run token, bounded
JSON and known case/kind. A lost page cannot pass: the negative oracle also requires
a positive script-start control and a post-attempt report. This is bounded observation,
not proof against arbitrary future timing or every WebKit feature.

Attribute improvement only where the unchanged baseline actually fails. Existing
WebKit/Wails popup and data-URL refusals are regression controls if both runs pass.
Verify the foreign-host destination was reached on the baseline; a DNS/network failure
alone cannot demonstrate origin enforcement. Framework callbacks and server observations
prove their measured boundary; checking a JS method alone does not prove host delivery.

## Limits and remaining work

The fixture's cookie/redirect is synthetic, not a substitute for the host's real
single-use launch/auth/CSRF tests. No hostile-local-process sandbox, network firewall,
per-window storage isolation or removal of all browser capabilities is claimed.
The frame-denying profile does not support Gallery's private iframe comparisons;
private inputs are disabled here and their independent browser viewer stays available.
Application-created windows remain allowed. HTTP command/SSE parity, simultaneous
Gallery views, actual keyboard/IME/pointer/zoom, capability grants/denials, crash/startup
recovery and applicable distribution checks remain separate G6 gates. A native probe
does not repair the unavailable desktop Computer Use connection or prove visual craft.
