# Interrupted shell saves: keyboard recovery and unknown outcomes

G2 follow-up to the [application shell](../design/application-shell.md). The user
task is to resume a retained draft after stopping a slow native save, inspect what
was actually saved, and explicitly resolve a stale-version conflict. This remains
the Gallery's fictional process-memory account, not a new persistence or session API.

## Result

The Gallery-only script now returns keyboard focus from the disappearing Resume
control to the first changed enabled participating field, or the first enabled field
for a canonical retry. Recovery triggered by input/change preserves the field already
being edited. Native POSTs, validation, optimistic versions, draft warnings and
outcome-unknown copy remain unchanged; no automatic fetch/repost behavior was added.

The original script failed the real keyboard-focus assertion after 13 checks: Resume
restored the controls but left focus on the page. The repaired script passes all eight
interruption journeys twice, 281 checks per run. These are repeated checks over eight
scenarios, not 562 independent user workflows or complete G2 acceptance.

| Scenario | Required saved state | Recovery |
|---|---|---|
| Stop before the proxy forwards the POST | Original value and version unchanged | Retained draft, keyboard Resume, one explicit save |
| Stop after the app returns 303 but before the proxy delivers it | Submitted value saved, version advanced exactly once | Retained original version; revised draft gets 409; explicit review saves once more |

Both scenarios run at 390×1000 and 1440×1000 CSS pixels, in light and dark themes,
using Chrome 152.0.7977.84 / isolated Playwright 1.63.0 on macOS arm64. Browser-level
Stop is invoked through CDP `Page.stopLoading`; actual Tab/Enter traversal activates
Resume afterward. Other fixture edits/submissions use normal browser controls. No
test directly focuses an element or invokes the app's recovery function.

An independent same-session tab reads the actual saved value/version before retry
and after each accepted write. Exact POST counts are two before-dispatch and three
after-acceptance, including the rejected 409. No outside request, WebSocket, page
script error or unexpected dialog occurred. The four expected conflict responses
are checked separately from unexpected console errors. Phone-light resumed-focus
and desktop-dark conflict screenshots were visually inspected.

## Why the transport boundary matters

The test-only loopback proxy forwards to one explicitly selected local Gallery. It
preserves native Host/Origin, cookies and response security headers without logging
them. It allows only profile/static routes and the native action endpoint, limits
POSTs to 8 KiB and responses to 512 KiB, and does not follow redirects. No app handler,
save function, auth guard or server state is patched for this test.

Earlier browser Fetch-interception and raw Runtime/AX probes could not inspect the
old document while native navigation was pending. A compositor screenshot showed the
pending UI, but that did not prove input delivery. Those failed probes are retained;
the passing claim is recovery **after browser-level Stop**, not inline cancellation
of an actively pending navigation, socket-loss coverage or physical toolbar input.
Later [inline Resume evidence](2026-09-14-shell-inline-recovery.md) covers pending-navigation keyboard activation separately; it does not change what these earlier runs proved.

A candidate run then caught premature test cleanup: acknowledging Stop did not prove
the held request had disconnected. Closing its response too soon allowed another POST
and changed the saved value in a before-dispatch case. The transport now waits for the
held downstream response to close from the client before owner cleanup. Unit canaries
prove neither `release()` nor `close()` can satisfy that interruption barrier. Extra
requests or version increments remain failures; no retry tolerance was added.

## Verification and reproduction

- Two final browser runs: **281/281 each**, eight complete scenarios each.
- A further **281/281** replay verifies the final harness after cleanup hardening:
  transport cleanup is attempted even if browser-context closure fails.
- Existing native/no-script/validation/conflict/appearance/recovery browser suite:
  **125/125** against the repaired build, unchanged assertions.
- Node tests: **33/33**, including focus ownership, changed-field priority,
  disabled-field/canonical fallback, transport refusal/cleanup and evidence canaries.
- Gallery `go test -race ./apps/gallery -run '^TestShellDemo' -count=1` with
  `GOWORK=off GOPROXY=off`: pass (2.076s), including existing HTML/script budgets.
- No CSS generation, dependency change, consumer write, commit, release or deployment.

The shared [evidence predicate](../../apps/gallery/browser/recovery_assertions.mjs)
rejects wrong dispatch/acceptance, extra POSTs, lost draft/version, wrong saved values,
skipped conflict and incorrect version increments. Its canaries include versions beyond
JavaScript's exact-number range. The same predicate consumes browser observations;
mock evidence is not substituted for a real browser result.

Use the existing approved browser configuration in the [probe guide](../../apps/gallery/browser/README.md),
then run `node apps/gallery/browser/shell_recovery.mjs`. The tool creates only fresh
contexts and loopback test proxies; it never installs a browser or runtime dependency.
The pure Node checks are wired into `task check`; browser runs remain explicitly opt-in.
This receipt does not claim a full dirty-root `task check` or clean release gate pass.

## Retained identities

Reports live under `/var/folders/2r/2zrkv4fd549bnlc5vwfzd4b00000gn/T/` and are temporary,
not a backup. Full filenames are `shell-recovery-<variant>-report.json`, except the
existing regression suite's `shell-forms-recovery-focus-regression-report.json`.

| Evidence | Directory / variant | Report SHA256 |
|---|---|---|
| Original focus failure | `appkit-shell-browser-YutL2R` / `recovery-stop-baseline` | `8fa900b029a8dc1f4c9dff944626846d5060f5f4ca55b96503021688877bcdf3` |
| Final interruption suite | `appkit-shell-browser-eh6OPz` / `recovery-disconnect` | `a663db00c4860d591db3f85de7e6b406bd6768b81aba4edc634440b2363aeb4c` |
| Independent fresh-context repeat | `appkit-shell-browser-UsOuRn` / `recovery-repeat` | `b9d473707a7adaf4561b1a706a1973c235d89ae00128e2eaf7da4506ca339661` |
| Final cleanup-hardened harness | `appkit-shell-browser-K2OqQs` / `recovery-final` | `38007fad9bfca700acfb8a8f5144b572516efc4546dc5fd15077218f999a24d9` |
| Existing browser regression | `appkit-shell-browser-GQjbXU` / forms | `c4af26657047dd5084c94a7c6612eddaf789082f140291fa7551fede43b6a389` |

The source-build pair is under `/tmp/appkit-shell-recovery.Pn2wPV/`: original `gallery`
SHA256 `5afafccfd0c8764bb1a2f42106477d4d283d62d254ffd60ae44c43772da3388c`, repaired
`gallery-candidate` SHA256 `97b522e68037629cee2dca50a0bc42b3b84aaee83fcb8fc1df54bc0cb8577708`.
Original script SHA256 `d9c009d2632536c0b30a8a0dea063b410975383a709a6da2dd35a2e3a3481553`;
repaired script `16c13a410b038a2605185f22e877d3a4acc0905af7783a695e8ea3faeed7c04b`.
The first two passing runs bind harness `8a389bb1966658ac67066259d86c384184207da5157e4a8325ad025e3b037af8`;
the final cleanup-hardened harness is `6dd0b09ee45c2d81b2c02579102a939270231073b184463cc97330c66df95877`;
transport `f8c8d1bffa1cc70b524daca5385102e7d045bb6514dc53652703a8726327569b`.

Earlier failed probes remain in `appkit-shell-browser-zlwus8`, `8DV4BM`, `yYX0sZ`,
`5psNlF`, `ECTHQP`, `TIFOyG`; the failed cleanup-race candidate is `Ylm7uO` (each suffix
uses the same `appkit-shell-browser-` prefix). One hung diagnostic Chrome was explicitly
terminated; none of those probes is a pass or a target-support claim.

## Remaining acceptance

The [Settings follow-up](2026-09-14-shell-settings-recovery.md) now verifies scoped
browser-Stop recovery for select and checkbox drafts, retaining this Profile history.
Inline cancellation during active native navigation, interruption by typing, real
browser Back/cache behavior, 200% zoom,
screen readers, complete focus/contrast and human task/performance acceptance remain
open. Native-host and real-consumer recovery require their own evidence. G2 is not
complete; G1–G6 remain active and Windows/CA/GGB remain deferred.
