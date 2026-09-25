# Native process-owned HTTP recovery

Date: 2026-09-15. G6 progress, not completion. [ADR-0047](../adr/0047-process-owned-http-and-native-recovery.md)
extends ADR-0026/0033/0035 after their native lifecycle revisit condition fired.
Windows/mobile, real consumers, signing, installation and deployment remain outside this slice.

## Why a process boundary

The exact cached Wails `v3.0.0-beta.21` source establishes:

- `pkg/application/application.go:42-99`: `New` installs a singleton and starts resources;
  constructor failure reaches `handleFatalError` at 571-574 and unconditional `os.Exit(1)`.
- `application.go:644-699`: `Run` permanently marks startup; native initialization precedes
  returned startup errors. Neither a new error handler nor retrying `Run` gives isolation.
- `application_options.go:83-89` and `application_darwin.go:315-362`: Cocoa termination
  can prevent `Run` returning; PostShutdown is the last cleanup acknowledgement.

Those paths can kill HTTP if it shares the native process. The opt-in parent now owns
the unchanged real guarded listener and Gallery state; only the native window/lifecycle
lives in the companion. Domain traffic remains HTTP, with existing CSRF/session/version rules.

## Implemented boundary

- Explicit absolute executable plus lowercase SHA256; verify a private immutable copy
  before execution. Missing/mismatched identity refuses before HTTP or browser launch.
- Cgo-free parent/server imports remain independent of Wails. Direct native/manual
  `-browser` recovery and direct browser mode remain; no root dependency or public kit API.
- Private inherited socket, canonical 2 KiB control frames, strict origin/launch grammar,
  matching outstanding replies, bounded partial-frame/write completion and control rate.
  No application RPC, public control endpoint or companion argv/environment credential.
- Native menu actions wait for navigation acknowledgement; programmatic callbacks also
  refuse while starting. Ready is lifecycle evidence, never task certification.
- One native attempt; failure stops/reaps it, then attempts browser continuation once.
  OS opener failure retains existing HTTP sessions until Ctrl-C and returns an error.
- PostShutdown `closed` plus bounded zero exit means normal close, even during loading;
  only an explicit browser continuation keeps the parent alive after that close.
  Control failure cannot masquerade as normal close. Native cancellation has a four-second
  child watchdog; parent waits two seconds after SIGTERM, then kills/reaps within two more.

## Executed evidence

Host: macOS 26.6.2 (25G83), darwin/arm64, SDK 27.0, Go 1.26.3.
Both checked Wails patches replayed unchanged in fresh stage `appkit-wails-beta21-165886108`.
No cached dependency, root pin or installed host configuration changed.

| Gate | Result and limit |
|---|---|
| Parent/control unit and race tests | Pass: private-copy identity/mutation/refusal, flags, canonical malformed/duplicate frames, deadlines, concurrent writes and request/reply matching |
| Synthetic subprocess faults | Pass: EOF, nonzero/zero-without-close, startup stall, malformed/out-of-order traffic, partial-frame stall, late failed close, control flood and SIGTERM-ignoring reap; helpers are not Wails |
| Synthetic normal lifetime | Pass: clean close before/after ready, browser handoff retains HTTP, opener failure retains HTTP then returns sanitized error, nonzero close remains failure |
| Actual Wails crash | Pass: navigation ack, exact child SIGKILL, one injected browser launch exchange; existing saved HTTP session, invalid-CSRF/stale refusal and already-open SSE survive; child reaped |
| Actual Wails control EOF | Pass: native navigates, control closes, native exits cleanly/reaps, parent HTTP remains live |
| Actual Wails orderly SIGTERM | Pass: native navigates, signal reaches PostShutdown and zero exit, parent HTTP closes, no browser opener called; this is not a menu/last-window-close test |

The actual-native assertions run in a separately compiled cgo-free Go test executable.
Native exit zero alone cannot pass them. The browser opener is injected and exchanges
a real launch using an existing HTTP cookie jar: no Chrome handoff or WKWebView cookie
transfer is claimed. Ordinary Gallery semantics were not changed to share clients.

Commands and bounds: [RECOVERY-PROBE.md](../../spikes/wails-gallery/RECOVERY-PROBE.md).
Root `go test ./...`, nested server/contained race tests and server/contained vet passed.
Existing native containment (14/14) and Gallery workflow (14/14) replay and pass their
separate receipt checkers; their narrower outcomes do not certify the new parent menus
or delivery. Capability-tag race/vet/build isolation checks pass without retrying the
unresolved audio request; native microphone/combined outcomes remain failures.

## Retained evidence and limits

Artifacts: `/Users/geoff/Library/Application Support/app-kit/native-evidence/2026-09-15-recovery-klgt7e/`.
The final selected native binary is `gallery-native-final`; `recovery-check-verified` owns
the three actual-native assertions and `recovery-verified.txt` records their execution.
Earlier binary/receipt iterations are retained, not silently replaced.
The selected native binary SHA256 is
`de52ac38826dc723de669b3f0f7d74862ecd9b7305067d69b77ee17cc4de6aae`.
Final checker SHA256: `3ed65c2148b47a919e13b890c622186a2a04752a66b1f1de0ddca91ab417e9cc`;
final output: `73c7de21363bcdba9e9ab0811aff0380cd9623d3a84037540b2d6422734a6ddb`.
Workflow binary: `48ddcaf747425517c00520c77f95d0194c8ee8941ebde1ce8695467f16b2fb45`;
workflow receipt: `a8726562aff8bb7a1b33c7bcd7fbcc503b896eec63b23bbc770c1473768c5174`.
Containment receipt: `b15bfafe86bc508cd6651384ee167646b912b3f1e61cef9a7da1fc51d4249e7b`.
Final source archive: `46793314c230cfa6fe37fd79afe1d37e2ef6dc8c979c14bf761251d9b05e977b`.
The source archive records this spike only, not the complete root/dependency closure;
this is a dirty-checkout experiment, not a clean released consumer build or signed proof.

Independent review found early-menu ordering and unsolicited-reply gaps; code/tests
now gate readiness and correlate replies. Clean-close tail drain uses the three-second
frame bound, with stalled termination classified as failure. These fixes do not replace
actual graphical menu/input acceptance. Intent/router/ADR/changelog updates stay current.
Final test review also corrected two false-positive risks: the SIGTERM-ignoring helper
acknowledges only after arming refusal and requires the forced-kill wait; the flood helper
stays alive after 40 requests, so a missing rate gate cannot pass through ordinary EOF.
Both negative-path tests pass twice under race detection; the complete server/contained
race suites pass afterward. Focused intent checks pass 6, skip 2: 47 indexed ADRs,
206 valid links, paired stanzas and budgets (router78, Unreleased60); no global wiring claim.

## Follow-up: actual parent loss and Chrome continuation

The same contained native binary now passes two further actual-macOS paths:

| Case | Final observed result |
|---|---|
| Separate parent SIGKILL | Real HTTP parent and exact native child start; parent health passes; owned parent terminates by SIGKILL; child disappears within six seconds without direct test-side kill; listener refuses connections |
| Uncontained native startup | Separate uncontained Wails-linked build produces the exact profile-refusal message and exit 1; no navigation ack or native launch exchange; parent recovers once to guarded HTTP |
| Native failure → isolated Chrome | Exact native child killed/reaped; injected opener starts Chrome with `headless:false` in a fresh context; default profile renders; all fourteen existing workflow checks and their exact console/network oracle pass |

Chrome 152.0.7977.84 / existing Playwright 1.63.0, same OS/SDK/Go above. Final Chrome
run: 5.90 seconds total; fourteen steps range 65–1,404 ms; 1440×1000 CSS px, light theme,
instrumented DOM input. Nine POST statuses remain `[422,303,409,303,303,503,303,303,303]`;
two streams open and zero remain after shutdown. No extra allowed request, console
exception or permissive finish profile was added. Full task receipt is in test output.

The source-reviewed driver explicitly configures headed mode; there is no independent
OS-window/physical-input assertion. Both retained 1440×1000 PNGs are byte-identical;
visual inspection shows the complete default profile without visible clipping. PNG
type/dimensions, private exclusive write, size and reread SHA256 are checked separately.
LaunchServices/default-browser selection, native cookies and native drafts are not used.
The new browser gets its own synthetic Gallery session; the earlier existing-browser
state/CSRF/SSE survival result is separate, not proof of cross-client state transfer.

The refusal tests the explicit pre-Wails profile guard, not arbitrary constructor
failure. Parent SIGKILL cannot run deferred temporary-copy cleanup: the test removes
only its exact hash-validated staged executable and empty directory; production abrupt
parent death may leave that private executable copy. No app state is persisted there.

Review tightened the tests before final replay: bound metadata acquisition; distinguish
failed process observation from absence; require actual connection refusal; enforce
opener count and returned shutdown error through termination; detect oversized HTTP
bodies; independently parse the PNG dimensions. Synthetic process-observation canaries
pass ten race runs. Final server, prepared-contained and server/workflow race suites,
three selected vet configurations and five Node driver tests pass.

Follow-up artifacts are retained separately in
`/Users/geoff/Library/Application Support/app-kit/native-evidence/2026-09-15-recovery-followup-hj1Sji/`.
Its README maps exact binaries, logs, source archive and image hashes. Earlier runs are
retained; only final checker outputs support the final assertions above. No production
logic, root dependency, consumer, commit, global configuration or deployment changed.
Final focused intent check: six passes (required docs, budgets/changelog, 47-record ADR
index/shape, 221 relative link paths, paired stanzas and goal/evidence routing), two
explicit skips (host/global wiring and semantic goal completion); link anchors were not
checked. Router remains78 lines and scope150. Final independent review found no further
actionable probe/assessment issues. No owned recovery probe/native child remained running.

## Remaining G6 work

OS-selected browser handoff and app-owned cross-client recovery policy; native menu/
second-window/last-window close; constructor and other startup faults beyond the tested
profile refusal; physical keyboard/IME/pointer; unresolved native
permissions including the [microphone/combined failure](2026-09-15-native-media-denial.md);
and applicable, separately authorized packaging/update recovery. No native certification,
conversion promotion, consumer upgrade or G1–G6 completion is implied.
