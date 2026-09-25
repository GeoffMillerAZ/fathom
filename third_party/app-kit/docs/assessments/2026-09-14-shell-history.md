# Shell history and optimistic-version recovery — 2026-09-14

## Result and defect

Chrome network Back restored an abandoned Profile/Settings draft into a newly fetched
form, but kept the server's fresh hidden version. After another tab saved version3,
the old version2 draft therefore submitted as version3 and succeeded303 instead of
requiring409 review. Actual BFCache retained the complete old document/version pair
and correctly conflicted. `Cache-Control: no-store` alone did not prevent the mix.

The Gallery edit form now declares `autocomplete="off"`; Profile no longer overrides
that policy with `autocomplete="name"`. This keeps the native/no-script fallback in
the same scope. No JS state store, API, session, dependency or consumer change is added.
Server version/CSRF/origin checks and existing failed-save draft retention stay intact.

In the pinned Chrome matrix, network Back now loads current saved fields and version;
an abandoned draft is not promised to survive that reload. Actual BFCache preserves
the unsent fields and original version, then explicit save409 → reviewed save303 works.
Autocomplete is a browser policy hint, **not server-enforced or cross-browser protection**.
Other engines and native hosts require their own history conformance before adoption.

## Same-oracle before/after evidence

[shell_history.mjs](../../apps/gallery/browser/shell_history.mjs) runs32 independent lanes:
Profile/Settings × BFCache disabled/enabled × scripts/no-script ×390/1440px × light/dark
system scheme, height1000px. Settings checks both selections and checkbox state.
Chrome152.0.7977.84; isolated Playwright1.63.0; fresh contexts and fictional sessions.

| Build | Result |
|---|---|
| Original | 660 assertions pass;16 mixed-snapshot assertions fail plus16 matching lane-abort records; all32 start, only16 finish |
| Native-policy candidate | 788/788 assertions pass;32/32 lanes finish;16 actual BFCache draft restores |
| Fresh-process repeat, same binary/harness | 788/788 pass;32/32 finish;16 actual BFCache draft restores |
| Existing native form regression | 125/125 pass |
| Existing Profile/Settings interruption regression | 1025/1025 pass across24 journeys |

Each lane saves once, goes Back/Forward without reposting, then edits an unsent draft.
Scripted lanes dismiss and accept actual native departure dialogs; no-script lanes
expect none. A second tab saves newer values before Back. The oracle requires either
the original cached draft/version or current network-loaded saved fields/version,
never a mixed pair. Cached drafts reach409 with all newer values displayed, preserve
their edits, and require **Apply reviewed draft**. Network reloads permit a new edit
against the now-visible current values. Exact POST counts/statuses and version increments
are checked; a separate same-session GET confirms the final saved snapshot.

The harness enables BFCache by removing only Playwright's cache-disabling default;
repost-prompt suppression is also removed. Application cache/security headers remain
unchanged. Actual CDP `Page.frameNavigated` type `BackForwardCacheRestore`, request counts
and not-restored reasons distinguish cache restoration from network history. No synthetic
pageshow, app-handler call, DOM mutation or forced click stands in for browser behavior.
Automated browser wheel input exposes mobile controls; geometry and center hit-testing gate clicks.

## Receipts

Private artifact base: `/var/folders/2r/2zrkv4fd549bnlc5vwfzd4b00000gn/T/`.
Each directory below contains its named report and synthetic screenshots where applicable.

| Artifact relative to base | SHA-256 |
|---|---|
| `appkit-shell-browser-izBDvR/shell-history-history-final-baseline-report.json` | `5e3fb8cad699594bbd6930c6b7a5f6d5e257ba6c613dc4fbaaa5b22b21985ee1` |
| `appkit-shell-browser-OLteXZ/shell-history-history-final-candidate-report.json` | `eb5b055ecbcbd694665b20fe7a44d5b274563ed0a22fa0dc81635d4f8c7bbe88` |
| `appkit-shell-browser-RmdaX1/shell-history-history-fresh-repeat-report.json` | `c932df8dd248ec1591c7367f5996df0470d77a1bf9fa88ba6c5e279ba4eb4b13` |
| `appkit-shell-browser-irnmnu/shell-recovery-history-recovery-report.json` | `39345a5b9511e8ae0d0d35e0535a15953729f3b88e49ce86a7e884904f199f08` |
| `appkit-shell-browser-tKytbv/shell-forms-history-regression-report.json` | `83145e8e3f2fa1b550f1718925bc8bb93c978579f00aa63d7f21395426eba9d9` |
| `appkit-history-recovery.2oyhm9064c/gallery` (original binary) | `97b522e68037629cee2dca50a0bc42b3b84aaee83fcb8fc1df54bc0cb8577708` |
| `appkit-history-recovery.2oyhm9064c/gallery-candidate` | `be85769d7a70906d0e2f15c75bfd83ea925136a4ffcbf6466a1fcb73df1d048d` |

Harness SHA-256: `39e916e635baabcddf6fabcda42d6e70741e27128cd77d16900e4a10bbce9d59`.
Candidate renderer SHA: `db1c0efa90a02152c17df9f86372d0bc027506d99bf77066deefd9ea2274b008`;
template SHA: `518cade2cb4cf05eabcfe1c650758e7596af9975ff32d905fb327165d506f1dd`.
Production draft JS is unchanged (`16c13a410b038a2605185f22e877d3a4acc0905af7783a695e8ea3faeed7c04b`).
Before/after final runs span2026-09-15T03:44:53–03:45:33Z, September14 local.

Initial diagnostic `appkit-shell-browser-JJYEtU/shell-history-history-version-baseline-report.json`
(`d60e2b4b2d3502d6bf447af4a5f3f300910f65215b0f04a64f84cdf2e0d66309`)
also records the actual incorrect303 in both original network-history forms.
Earlier diagnostic expectations were corrected openly: network history may restore
fields without BFCache; BFCache does not fire a new load; the expected final Profile
value differs for retained versus re-entered drafts. The no-script mobile wait failure
was harness-only: native wheel exposed Save at y883–919 above navigation y951, while
in-page animation-frame polling failed to advance. Host-side bounded polling fixes
that wait without changing the app layout or weakening hit-test checks.

A repeat on the heavily reused test process reached its intentional128-session cap:
`appkit-shell-browser-HXbaUg/shell-history-history-repeat-report.json`
(`5825b7ef05dc6c9c524e96bc55cd39b6366c887fc99db3c0531f0869205c0b28`)
retains115 passes/69 failures and only4 finished lanes, with admission503s. It is not
a pass or a script regression. Only the two owned servers were stopped; a fresh process
of the identical candidate binary then passed the unchanged32-lane matrix. The runner
guide now calls for a fresh owned process; no shared server or capacity policy changed.

## Verification and remaining gates

`TestShellDemoNativeHistoryDoesNotOptIntoMixedVersionRestoration` checks both forms,
participating-control overrides and200/409/422/503 render states. Uncached full Gallery
and MCP race tests pass (21.759s/23.427s); the35 controller/transport/config Node checks
pass. Existing VM pageshow coverage is labeled as such, not actual BFCache evidence.
Independent review caught the final-value oracle issue before the final pair; source
and scrolling-harness review found no remaining false-pass issue in this scope.
Narrow dark no-script conflict and wide light Profile screenshots were visually inspected;
fixed navigation in a stitched full-page image is not proof of control occlusion.

The same-path pre-save entry returned through network navigation in these runs, not
BFCache. Cached pending-submission uncertainty therefore remains unproven; the script
still clears submitting status on pageshow. No acceptance claim covers that case.
The native policy is not durable draft storage, all-browser safety, production auth,
full keyboard/screen-reader/contrast/zoom acceptance or Wails proof. Active-navigation
cancellation, typing interruptions and the wider G2/consumer gates remain open.
Subsequent [inline Resume proof](2026-09-14-shell-inline-recovery.md) closes the pinned-browser cancellation gap separately; cached pending submissions and typing remain unproven.

Scoped intent maintenance: router/living-doc budgets,41-record ADR membership,
CLAUDE/AGENTS intent-stanza parity and182 relative file links pass (anchors not checked).
The history note was folded into the existing unreleased shell entry to preserve the
60-line budget; two closed incident entries were reflowed without removing their facts.
Tool versions, skill wiring and host invocation were not reverified by these checks.
