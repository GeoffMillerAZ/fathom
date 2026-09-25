# Settings recovery after an interrupted native save

G2 follow-up to [Profile recovery](2026-09-14-shell-save-recovery.md).
This verifies existing Gallery behavior, not a new production controller or consumer
integration. [Shell contract](../design/application-shell.md); invariants I8/I12.

## Task and coverage

After browser Stop interrupts a native save, retain the draft, explain the uncertain
outcome, return keyboard focus to the first changed control, inspect saved state and
apply a reviewed draft with optimistic concurrency. No automatic replay is acceptable.

The existing harness now runs three drafts across both interruption stages, explicit
light/dark branches and 390/1440 by 1000 CSS-pixel viewports: **24 scenarios**.

| Draft | Interrupted submission | Reviewed final draft | Expected Resume focus |
|---|---|---|---|
| Profile | Synthetic submitted name | Different reviewed name | Display name |
| Settings selections | UTC, Activity, digest unchecked | London, Overview, digest checked | Time zone |
| Settings checkbox only | Phoenix, Overview, digest unchecked | London, Overview, digest checked | Digest checkbox |

Every scenario uses a fresh browser context and fictional process-memory session.
The observer page shares that session but independently GETs the saved state.
The existing bounded loopback proxy holds one POST before dispatch or after observing
the application's accepted 303. Browser Stop is CDP `Page.stopLoading`; the harness
waits for actual downstream disconnect before proxy cleanup. Owner release is not
accepted as interruption. Application handlers and responses are not substituted.

## Assertions that matter

- Preserve both selections, typed checkbox state and original version while pending
  and after keyboard Resume. Checkbox-only edits focus the checkbox, not an earlier
  unchanged select. The focused control's rectangle fits in the viewport and its
  center point hit-tests to the control; this does not exclude all edge occlusion.
- Before dispatch: independent GET proves no write. After acceptance: it proves
  exactly one write and all accepted values, despite the browser losing the response.
- Recovery exposes uncertainty, clears busy state and enables an explicit save
  without reposting. Reading saved values does not alter the draft or server state.
- Accepted-response loss requires 409 on the stale retry, displays the saved time
  zone, starting page and excluded digest, and retains every revised control.
  Only explicit Apply reviewed draft writes the reviewed values against the new version.
- Browser-observed native POSTs and proxy counts agree: exactly 2 before dispatch,
  exactly 3 after acceptance; accepted version increments are respectively 1 and 2.
  Each POST carries the expected original/reviewed version and singleton selections.
  Unchecked digest is **absent**, not a truthy string; checked digest is exactly `on`.
- Reports whitelist fictional form values and versions; cookies and CSRF are omitted.
  Exact Settings GET routes allow only the base URL and `theme=light|dark`; system,
  duplicate/arbitrary query and POST-to-Settings variants refuse without forwarding.

The shared evidence predicate has canaries for loss of each Settings field, missing
versus false checkbox observations, malformed types, extra fields, unexpected writes,
wrong acceptance/conflicts and versions above JavaScript's safe-number range.

## Observed results and artifacts

Installed Chrome **152.0.7977.84**, approved isolated Playwright **1.63.0**, macOS arm64.
Final run: `2026-09-15T03:18:35.402Z` to `03:18:53.767Z` (September14 locally).
Elapsed harness time is not an app latency or human-task performance measurement.

- Initial expanded matrix: **977/977**. Added viewport/center-hit and scheme-attribute
  assertions: **1025/1025**, repeated with fresh contexts. Independent review narrowed
  two assertion labels; a final replay also passes **1025/1025**, all 24 scenarios.
- Existing `shell_forms.mjs`: **125/125**, including its separate no-script saves.
- Node: **35/35** (12 controller, 14 evidence, 6 transport, 3 configuration tests).
- `GOWORK=off GOPROXY=off go test -race ./apps/gallery -run '^TestShellDemo' -count=1`:
  PASS, 2.112s. No full dirty-root gate or native-host certification is claimed.
- Reviewed the narrow/light checkbox-focus and wide/dark saved-versus-draft conflict
  screenshots. Labels, selected values and explicit review action remain readable;
  this is selected visual inspection, not a full contrast/accessibility audit.

All browser directories below are beneath
`/var/folders/2r/2zrkv4fd549bnlc5vwfzd4b00000gn/T/`:

| Directory / report | SHA-256 |
|---|---|
| `appkit-shell-browser-tLyUdD/shell-recovery-settings-recovery-report.json` | `10141789f3a238246b43bfcf637b360231aac0b9a9f1bafed4217f6631473243` |
| `appkit-shell-browser-nsJGLJ/shell-recovery-settings-visible-report.json` | `727818cbf0cb5eb5bbfd7e8a5a90c14fee326ec18e4dcb3c0690cb47c6d4fcbc` |
| `appkit-shell-browser-d2n6Dw/shell-recovery-settings-repeat-report.json` | `e8680aa050d74584d2beab9b35891665300f57d3c4e56207086b6fb3e509fa48` |
| `appkit-shell-browser-a48P8C/shell-recovery-settings-final-report.json` | `0addacf6458d162efca1ffcd85c9a36b0cc9c5448f919e687346b95db61b1466` |
| `appkit-shell-browser-Tt9AzO/shell-forms-settings-regression-report.json` | `db99fb4a6ce1bd520a8338d0058353db58d3941e903f6e5d75520359c312ea02` |

The freshly built `appkit-settings-recovery.3rCYKdwwBR/gallery` has SHA-256
`97b522e68037629cee2dca50a0bc42b3b84aaee83fcb8fc1df54bc0cb8577708`, identical to
the preceding Profile repair build. Production `shell_demo.js` remains
`16c13a410b038a2605185f22e877d3a4acc0905af7783a695e8ea3faeed7c04b`.
Root HEAD remains `63b3a4f01168b184f91d1e701414445820e24fdc`; uncommitted inputs exist.

| Final harness input | SHA-256 |
|---|---|
| `shell_recovery.mjs` | `1beca58b18f1e2f5928840a46bf82c4a1b3eff244dcab77a43bdbcb2ed703f74` |
| `recovery_assertions.mjs` | `8e16990f3e2ae9629b613fa4977c612fc3d554146e5e4cee256f0758bd6ef40c` |
| `recovery_assertions_test.mjs` | `f9ca4feca73104aed775aa170ac5947f8f7a3a60344a4f0a91d229b133899e0c` |
| `recovery_transport.mjs` | `6bdc99023c306116da598a57e425123876b386fcbf78d0ec2af9540d64fce5ab` |
| `recovery_transport_test.mjs` | `b4f8e85dbebb2775dae839062f0eaa4894f95c02006b94163db9388c4dc47358` |

The initial 977 run uses harness `c2427e3a7cc66288ae02402c2c616a8e967b315e8038fb1e62f86051c3f1b430`;
the first two 1025 runs use `1a7eae26ae66c535b966ea8b0c477ef117eab53d40a1724ce8101aabe0976e71`.
The final row above binds the label-corrected replay; prior receipts are not rewritten.

## Remaining acceptance

Settings-specific **browser-Stop** recovery is now verified for these controls and
cases. Inline cancellation during an actively pending native navigation, typing as
the interruption mechanism, actual Back/cache recovery, 200% zoom, complete contrast,
screen-reader, physical/native input and consumer/human-performance gates remain open.
Scheme assertions inspect `data-theme`; screenshots sample its appearance, not every
computed token. No permissions, real account data, source/dependency changes in a
consumer, commits or deployment occurred. G2 and the overall goal remain incomplete;
RAC's separate theme-guide vendor upgrade still requires owner approval.
