# Native process recovery probe

Opt-in synthetic G6 work under [ADR-0047](../../docs/adr/0047-process-owned-http-and-native-recovery.md).
This tests the actual process boundary and an isolated Chrome recovery workflow, not physical native input.
No private design sources, real account data, OS grants, signing or installation.

## Run

Prepare the exact two Wails patches with `go run ./cmd/prepare-wails` as in
[README](README.md). Replace `PREPARED` with that fresh preparation's absolute path.

```sh
go test -tags server -race ./... -count=1
go test -modfile=PREPARED/go.mod -tags appkit_contained -race ./... -count=1
go build -modfile=PREPARED/go.mod -tags appkit_contained -trimpath -buildvcs=false -o PREPARED/gallery-native .
shasum -a 256 PREPARED/gallery-native
CGO_ENABLED=0 go test -c -tags server,recoveryprobe -trimpath -buildvcs=false -o PREPARED/recovery-check .
APPKIT_RECOVERY_NATIVE=/ABSOLUTE/PREPARED/gallery-native APPKIT_RECOVERY_SHA256=EXACT_SHA256 PREPARED/recovery-check -test.run '^TestActualNativeCompanionRecovery$' -test.v
```

The selected `recoveryprobe` requires both inputs: missing identity fails, never skips.
The checker is cgo-free and does not import Wails. It runs the exact private copy of
the selected native binary, with its real guarded Gallery server in the parent.
The parent/child control channel carries only private bootstrap and lifecycle messages.

## Evidence boundary

| Gate | What executes |
|---|---|
| Identity and protocol | Exact private-copy hash/mutation/refusal tests; canonical framed messages, unknown/duplicate fields, URLs, deadlines and concurrency |
| Synthetic lifecycle | Real helper subprocesses exercise selected faults, clean-close/handoff, cancellation, rate limits and failure reporting; helpers are not Wails |
| Actual native crash | Wails navigation acknowledgement, exact child selection by this test parent's process tree, SIGKILL, one injected browser-opener exchange, preserved saved HTTP session, CSRF/stale refusal, existing SSE update and child reap |
| Actual control loss | Wails navigation acknowledgement, control EOF, clean native exit/reap; HTTP remains owned by the parent |
| Actual orderly termination | Wails navigation acknowledgement, SIGTERM, PostShutdown plus zero exit; parent HTTP closes without browser recovery; not a menu/last-window-close test |

The injected opener exchanges a real host launch with an existing Go HTTP cookie jar.
It neither opens Chrome nor simulates session transfer from WKWebView to another client.
The actual-native test runner owns assertions and exit status outside Wails, so a
native exit zero or lifecycle `ready` message alone cannot pass this gate.

Retain exact source/binary hashes, test output, OS/SDK/Go versions and elapsed bounds.
These runtime checks do not verify source-code signing, supply-chain provenance or an
adversarial same-user process boundary. The SHA selects trusted bytes, not trusted behavior.

## Parent loss, startup refusal and Chrome continuation

The same server/recovery checker also exposes `TestActualNativeParentLoss`. It starts
an actual separate HTTP parent, verifies its staged native child's ancestry/command,
checks HTTP health, then sends SIGKILL only to that owned parent. The native child must
disappear within six seconds without a test-side kill on the passing path, and the
listener must refuse connections. Observation errors are failures, not absence.
The test removes only the verified staged copy and its empty directory afterward;
production SIGKILL cannot run deferred cleanup and can leave that temporary copy behind.

For `TestActualNativeStartupRefusal`, build an ordinary **uncontained** native binary
with `go build -trimpath -buildvcs=false -o PREPARED/gallery-uncontained .` and compute
its hash. Supply `APPKIT_RECOVERY_REFUSAL_NATIVE` and `APPKIT_RECOVERY_REFUSAL_SHA256`
to the checker. Its exact profile refusal must exit 1 before navigation; parent recovery
must exchange a fresh launch and render the guarded HTTP profile exactly once. This is
the actual profile guard, not an injected arbitrary failure inside Wails' constructor.

The Chrome variant reuses the entire existing fourteen-step workflow/console/network
oracle unchanged. Build it with `server,workflowprobe,recoveryprobe`:

```sh
CGO_ENABLED=0 go test -c -tags server,workflowprobe,recoveryprobe -trimpath -buildvcs=false -o PREPARED/browser-recovery-check .
```

Run from this directory with `APPKIT_WORKFLOW_PROFILE` unset and the contained native
identity above. Set `APPKIT_PLAYWRIGHT_MODULE` to the existing authorized Playwright
`index.mjs`, and `APPKIT_RECOVERY_CAPTURE_DIR` to a fresh canonical absolute private
directory (create it with `mktemp -d`). Optionally select an already-installed browser
with `APPKIT_BROWSER_EXECUTABLE`. Run the checker with
`-test.run '^TestActualNativeBrowserRecovery$' -test.v -test.timeout 110s`.
Nothing installs or uses the signed-in profile. The injected opener launches Chrome
with `headless:false` and a fresh context; no LaunchServices/default-browser test occurs.
The log calls this **headed configured**, not physical visibility/input proof.

After actual native SIGKILL/reap, a fresh Gallery session must render the default
profile without a bridge. Capture `handoff.png` is exclusive, 0600 and at most 4 MiB;
the checker rereads its hash and PNG dimensions. It then runs all fourteen tasks,
including intentional 422/409/503/401 responses, draft recovery, two views, SSE and
sign-out. Exact observations, shutdown and one opener through parent termination must
pass. The full non-secret task receipt is retained in test output. Inspect the capture
separately; a screenshot alone does not prove these task results or transfer native state.

## Still required

The separate [external native menu/input probe](NATIVE-UI-PROBE.md) now covers AX
menu/second window/normal last-window close; its positive Cmd+N gate still fails.
Actual OS-selected browser handoff and app-owned cross-client session/recovery policy;
constructor and other startup faults
beyond the tested profile refusal; physical input/IME; full native capability and
authorized packaging/update failure recovery. Mic/combined media denial failures remain
open. Windows/mobile and CA/GGB remain deferred.
