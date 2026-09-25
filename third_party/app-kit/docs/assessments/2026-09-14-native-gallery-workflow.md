# Actual Gallery browser/macOS workflow

Date: 2026-09-14. **14/14 tasks pass in both independent Chrome and macOS WKWebView,
including teardown gates.** G6 experiment under ADR-0026/0033/0035; not platform certification.

## Scope

The same Go oracle drives the real synthetic Gallery through `startHost` and its
bootstrap guard. Both clients use ordinary HTML forms, application CSRF and version
checks, and the pinned Datastar observer. No application response, cookie, token,
Origin header, POST body or SSE frame is supplied by a fake transport.

The browser executable is `CGO_ENABLED=0`, `server,workflowprobe`; it imports no Wails.
Chrome uses a fresh isolated context, not the owner's signed-in profile. Native uses
the prepared beta.21 HTTP-only patch, the host's shared window policy, two actual
WKWebViews and a test-only `evaluateJavaScript` completion callback. It does not
re-enable the Wails bridge or replace the delegate. Private inputs remain disabled.

Input is programmatic DOM field/change events and scheduled native form submission.
No physical keyboard, IME, trusted pointer, focus traversal or visual review is claimed.
The test's explicit shutdown/menu orchestration is not normal host-menu proof.
Native JavaScript instrumentation is privileged and cannot prove page CSP enforcement.

## Shared task

| Task | Required outcome |
|---|---|
| Two views | Fresh profile loads; later conflict/update/revocation demonstrate shared session |
| Validation | 81-character name rejected 422 with error semantics and retained draft |
| Save and conflict | Corrected save 303; stale draft 409 exposes saved values; reviewed save 303 |
| Retry | Explicit fixture arms one failure; 503 retains draft/version; retry saves once |
| Live observer | Idle without a stream; Connect observes v5; other view saves v6 |
| Stop and reconnect | Stop leaves no stream or automatic reconnect for 1.4 seconds; explicit resnapshot preserves other draft |
| Sign-out and shutdown | Observer redacts and submitter finishes its signed-out page; two views close, streams finish, listener closes |

The exact POST sequence is `422,303,409,303,303,503,303,303,303`; exactly two explicit
streams open. There are fourteen ordered task checks, plus teardown and framework
counter gates. Missing execution is failure, not an inferred denial/pass.

## Run record

| Final observation | Independent Chrome | Patched macOS WKWebView |
|---|---|---|
| Task oracle | 14/14 | 14/14 |
| POST status sequence | exact nine | exact nine |
| Streams opened / active at shutdown | 2 / 0 | 2 / 0 |
| Views closed | 2 | 2 |
| Listener shutdown and separate strict receipt check | pass | pass |
| Observed viewport / theme | 1440 × 1000 CSS px / light | 1200 × 750 CSS px / light |

Final raw receipts: [browser](fixtures/native-workflow/browser.json),
[native](fixtures/native-workflow/native.json). Exact selected source, binary and
receipt SHA-256 identities: [identity manifest](fixtures/native-workflow/identity.json).
Native reports zero framework errors, warnings and bridge messages. Browser finish
requires zero unexpected page/network/console/dialog/cleanup signals and the exact
intentional HTTP-error console multiset (422, 409, 503 and signed-out 401).
This is page-request observation, not a process-wide network firewall or native console audit.

Both binaries ran on macOS 26.6.2 (25G83), arm64, Go 1.26.3. Chrome installed version
152.0.7977.84; Playwright 1.63.0. WKWebView reports AppleWebKit/605.1.15 with `wails.io`.
User-agent compatibility strings say Intel/10_15_7; those are not the actual CPU/OS.
Runs used `/tmp/appkit-gallery-workflow.nrH5D6/{browser,native}-verified`, built with
the runbook's exact flags; the prepared module path is in the manifest. Browser ran
first with no native candidate present, then native ran separately. Both process exits
were zero and both saved receipts passed the independent `check` invocation. The
manifest is a selected-source record, not a complete dirty-worktree snapshot or release.

The first pair passed; independent review then tightened callback scheduling and
timeouts. A subsequent Chrome run completed the fourteen assertions but correctly
failed overall: cleanup reached the submitting view during its sign-out navigation.
The final oracle now requires **both** observer revocation and the submitter's completed
signed-out page. No timeout or gate was relaxed. Fake-child regressions cover EOF,
kill fallback, poisoned-driver refusal and immediate final-reply ordering; Node close
exceptions now fail finish. Browser and native final runs above use that stronger oracle.

Tagged server/native race suites, ordinary nested race suite, both tagged vet profiles,
three Node classifier tests and the root MCP package suite pass. Root intent/ADR-index
relative links and instruction stanza parity pass; router/scope budgets remain 77/150.
This was not a full root `task check`, full intent-doctor or global host/Fathom audit.

## Reproduction and boundaries

See [WORKFLOW-PROBE.md](../../spikes/wails-gallery/WORKFLOW-PROBE.md) for exact build
profiles, isolated browser setup, strict receipt checking and negative test gates.
Run browser with the native candidate absent; then run native separately.
Record actual command/binary/source identity alongside the receipt. The JSON checker
validates consistency, not signed provenance: a fabricated or relabeled receipt is
not prevented by a local format checker. Hashes establish byte identity only.

The 12-second task and 90-second overall budgets detect failures; a 95-second native
watchdog bounds a wedged event loop. Driver round trips and polling affect timings;
Stop includes a deliberate 1.4-second wait. Do not compare engines or infer human
task times, rendering performance or accessibility from these measurements.

Remaining G6 work includes physical input/visual checks, full session/authorization
and failure-state matrix, native permission behavior, ordinary menu/last-window
lifecycle, startup/crash recovery and applicable delivery evidence. Signing, remote
hosting and consumer migration need their own authority. Windows stays owner-deferred.
No reusable desktop adapter, released pin, general parity schema or consumer adoption
is promoted by this experiment.
