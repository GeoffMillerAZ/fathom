# Shell typing interruption and persistent outcome uncertainty

Date: 2026-09-15. G2 remains incomplete; this is synthetic Gallery evidence.

Subsequent [Settings input evidence](2026-09-15-shell-settings-input.md) covers all three
Settings fields, repeats Profile with418 checks and binds expected dialogs to their
initiating page. Historical identities and limitations below describe this earlier run.

## Result

The repaired Gallery passes **410 checks across eight Profile typing journeys**:
before dispatch / after acceptance, light / dark, 390 / 1440 × 1000 CSS pixels.
The same final runner against the old binary fails on the second input: its warning
disappears. That old run is a one-case diagnostic, not an eight-case baseline matrix.
Two VM regressions also failed before the repair and pass afterward.

The defect was a missing state distinction. Comparing a draft with the old baseline
cannot establish whether an interrupted native POST already changed the server.
The first input recovered editing, but a subsequent input/change erased the uncertainty;
returning to baseline also disabled Save and removed the departure warning.

`shell_demo.js` now retains document-local `outcomeUnknown` after interrupted submission.
Continued editing, checkbox input/change pairs, baseline reversion and pageshow cannot
clear it. Save stays available for an explicit version-checked retry; active submission
still disables duplicates and suppresses the departure prompt for that submission.
A fresh response document initializes its own state. No automatic replay, local draft
storage, new transport, account API, dependency or consumer migration was added.
Identical status text is not rewritten on each keystroke in the polite live region.

## Actual browser sequence

The existing bounded loopback proxy preserves the Gallery's real handlers and authority.
An ordinary Enter submission from the focused Profile input is held before forwarding,
or its accepted303 response is held after one forwarding. The request must remain
connected for200ms before raw CDP `Input.insertText` inserts a distinct ASCII suffix.
The input itself must close the held downstream request **before fixture release**.
Neither browser Stop, Resume activation nor a synthetic DOM input event does that work.

The runner checks the exact pre-edit native POST, edited retained draft, original version,
field focus/visibility, independent saved-state read and absence of automatic reposts.
A second text insertion retains uncertainty. Reverting to the old baseline keeps Save
enabled. A real attempted Profile navigation raises exactly one `beforeunload` dialog;
dismissal produces the observed URL-specific `net::ERR_ABORTED`, and the same uncertain
draft/version remains on screen. No synthetic unload event counts as this proof.

The explicit baseline retry returns303 before dispatch, or409 after prior acceptance.
The latter shows the newer saved value and requires **Apply reviewed draft** before303.
Exact POST bodies/counts and independent reads verify all final values/version increments.
The separate `typingRecoveryVerdict` keeps edited and submitted observations distinct;
legacy Stop/Resume assertions retain their original expected-draft semantics.

## Verification

All following final application runs use the same `gallery-final` binary:

| Probe | Result | Receipt directory suffix |
|---|---|---|
| Profile typing with real departure cancellation | 410 checks; 8 journeys | `WvDpYT` |
| Browser Stop regression | 1026 checks; 24 journeys | `L4T1KN` |
| Inline keyboard Resume regression | 978 checks; 24 journeys | `haOOPr` |
| Native/BFCache history regression | 788 checks; 32 lanes | `HJbsJk` |
| Standard shell forms, including no-script paths | 125 checks | `Kwsrgv` |

The Stop/Resume counts include one new explicit zero-expected-dialog assertion.
All45 focused Node tests pass:15 controller tests,21 recovery-oracle tests and9
configuration/transport tests. They include lost draft, extra/missing observation,
unexpected repost, version/conflict and repeated-live-region-write canaries.
Root Go tests/vet and Gallery/command race tests pass; root tests do not test Wails.
The existing Taskfile selections already include all changed Node tests.
Post-document MCP/Gallery tests and12 root intent structural groups pass (budgets,
links, records, goal/future conditions and stanzas); this is not a full host or semantic
audit. Whitespace checks pass, the staged index stays empty and all ten owned test
Gallery processes were stopped after browser completion. Evidence files were retained.

Isolated Playwright1.63.0, Chrome152.0.7977.84, Node26.3.0 and Go1.26.3 on the local
Apple Silicon macOS host; fresh synthetic browser contexts, reduced motion, private
assets disabled. Screenshots of phone/dark recovery and desktop/light conflict were
reviewed. Full-page stitched fixed navigation is not a viewport clipping measurement.

## Retained identities

Binaries are under `/tmp/appkit-shell-typing.BHFSen1Wr7/`:

| Artifact | SHA-256 |
|---|---|
| `gallery-before` | `c3e2446e369bd7d23217919916587823c2a39b24d0d39d7aa51372ffde7ebc14` |
| `gallery-final` | `ec41221a214860287fe14b9a3d4c0ce4ec3711071f3f7728eefbda8d73167276` |
| Final `apps/gallery/shell_demo.js` | `1c9a3096767c4864f05fcf861957ca7a2ea6c0584cb7f7001fa5ce4ddbfec6a4` |
| Final `shell_recovery.mjs` | `3691fda8ea3b6d2a2a6ed9c1c338dce81ead19a22aee49eec8cbaa5b24eb6c91` |
| `recovery_assertions.mjs` | `ddb7b72e18e236c2b865926bcf6d96f2562e687605cb2cad2c4e13c82c42250f` |

Receipts live under `/var/folders/2r/2zrkv4fd549bnlc5vwfzd4b00000gn/T/appkit-shell-browser-`:

| Suffix / filename | SHA-256 |
|---|---|
| `HMmMOF/shell-recovery-typing-before-abort-report.json` | `c3b768e90455db717d9fd419d808b6e4488693229877229c6d2bcf60b13798fb` |
| `WvDpYT/shell-recovery-typing-final-abort-report.json` | `13ebf20f3b89a06aba52de4bd308cb5ab6a1c7372e85bbbf5f47b8ce464de07e` |
| `L4T1KN/shell-recovery-typing-stop-regression-report.json` | `5cd0b315b8d7aafe1da5234d04de00403c9bfe818e6290f301489bb39591bc71` |
| `haOOPr/shell-recovery-typing-inline-regression-report.json` | `e00ecd8efc3398e2c4aacf34043edcb8a3c716c055360e2241ecf903bfb912de` |
| `HJbsJk/shell-history-typing-history-report.json` | `db2e4b80e851c8aa269b97f66fa3c07010cb9eb97d9bebf638fda5883be77314` |
| `Kwsrgv/shell-forms-typing-final-forms-report.json` | `6c10f70d578337414c3881931f57aa1cf46766609fa66bc78d8c3011b3e0ede9` |

Stop/Resume ran runner `829fd330afc72c384810225b7c45f8f7ed116430ffa8bb0c5dafe21f67c43bb8`;
the final change only corrected the typing-only rejection-string guard. The retained
`QM1tue/shell-recovery-typing-confirmed-report.json` (SHA256
`30c177feb4576c0b672b4f14d3c6c3f50fa0ae89e1ce5625685c00cf01ab7a9d`) failed because
that new guard expected Playwright's internal cancellation wording rather than the
observed Chrome `ERR_ABORTED`. Final proof requires that exact target-specific abort
**and** the dismissed real dialog, not an arbitrary rejection or timeout. Earlier
`GxeWkb`377-check proof used only a synthetic unload-handler assertion and is superseded
for departure behavior; `lqwRbA`402-check proof lacked the exact rejection-string guard.
No failed receipt was edited or promoted. Temporary artifacts are not durable backups.

## Reproduce and remaining gates

Use the approved isolated setup in the [browser guide](../../apps/gallery/browser/README.md),
with a fresh owned Gallery process and `-design-assets=''`, then:

```sh
APPKIT_RECOVERY_INPUT=typing node apps/gallery/browser/shell_recovery.mjs
```

This proves automated ASCII Profile input on the pinned Chrome build, not physical
keyboard, IME/composition, Settings input interruption, screen-reader announcements,
all browser engines, pending-save BFCache, native-host input or production persistence.
The unchanged-live-region test proves no repeated DOM writes, not assistive speech.
Full contrast/zoom/performance and G2's two-consumer adoption gates remain open.
Review found no blocking mismatch. The expected-dialog listener is context-scoped;
the observer does not navigate during the departure trial. Adversarial simultaneous
observer-dialog routing is not claimed; a future harness change can bind the expected
dialog explicitly to its initiating page.

Intent: I8/I12/I15 and ADR-0019/0021/0026 govern; no ADR/future trigger, new public API
or breaking contract. INC-32 records the defect. Impeccable hardening guided retained
work, honest recovery and quiet status updates; app identity and authority stay intact.
