# Settings input interruption and consumer-fit boundary

Date: 2026-09-15. Synthetic Gallery evidence; G2 remains incomplete.

## Result

All **24 Settings journeys pass 1,218 checks** on the existing repaired Gallery binary.
Each editable control is independently the interruption trigger: time zone, starting
page and digest checkbox, before dispatch / after acceptance, light / dark, at
390 / 1440 × 1000 CSS pixels. The same final runner against the old binary fails the
accepted-write checkbox case after 12 passing checks: returning to the baseline says
“No unsaved changes” and disables Save even though the server saved the opposite value.
This is one diagnostic baseline, not a full old-version matrix.

The [Profile repair](2026-09-15-shell-typing-recovery.md) already supplies persistent
`outcomeUnknown`; no further production code, transport or dependency changed here.
This extension proves the repair beyond text input and strengthens the test oracle.

## Actual browser sequence

An ordinary native Save submits the selected synthetic fields. The bounded loopback
proxy holds the POST before forwarding, or holds its303 response after one accepted
write. Raw CDP Shift+Tab reaches the named enabled field. The downstream request must
remain connected for200ms after traversal, before the edit. Native letter type-ahead
changes a select; Space toggles the checkbox. That input must close the held downstream
request before fixture release. No Stop, Resume activation, DOM event injection or
pending-form value assignment substitutes for the input.

| Trigger | Submitted fields: time zone / start page / digest | Retained after input |
|---|---|---|
| Time zone | UTC / activity / false | Europe/London / activity / false |
| Starting page | UTC / activity / false | UTC / home / false |
| Digest | America/Phoenix / home / false | America/Phoenix / home / true |

The original baseline is America/Phoenix / home / true. Every journey verifies the
whole typed triple, exact native POST arrays (including unchecked omission), original
draft version, retained focus/visibility, warning, enabled Save, independent saved-state
read and absence of automatic reposts. A second native key reverses the first edit
without resolving uncertainty. Select edits are separated by two seconds so type-ahead
starts a new prefix; this is not a throughput or performance measurement.

Returning all fields to the original baseline must still retain uncertainty. A real
attempted same-route navigation must raise one dismissed `beforeunload` dialog and the
exact target-specific `net::ERR_ABORTED`; the retained draft/version is checked again.
The expected-dialog listener is now bound to its initiating page. Unexpected dialogs fail.

Explicit baseline retry returns303 before dispatch, or409 after the accepted write.
The conflict shows saved versus retained values and requires **Apply reviewed draft**
before a new303. Exact counts, bodies and independent final reads prove the outcome.
The separate `settingsInputRecoveryVerdict` rejects malformed/noncanonical snapshots,
untyped checkbox values, missing/extra observations, edits to the wrong control, stale
versions, automatic reposts and skipped conflicts. It does not normalize evidence into
a pass. Existing Stop/Resume/Profile modes retain their own expected-value contracts.

## Verification and retained identities

| Probe | Result | Receipt suffix |
|---|---|---|
| Final Settings, all three controls | 24 finished; 1,218 pass; zero failures | `utOcOB` |
| Old binary, accepted checkbox / dark / 390 | Expected failure: recovery controls disabled | `gDWxVg` |
| Profile regression during Settings development | 8 finished; 418 pass; zero failures | `Vf67yY` |
| Controller / recovery oracle / config / transport Node tests | 59 pass; zero skips | Local command below |

Profile's418 includes eight new continued-input focus/version assertions. That run
used runner `c204cd0bdde9ac0515402d9350f4fcbb940c43bd71a33bccc9eb7e2ef1b3a50d`
and helper `729214afc1896e0f6435878d55ee78cdcd2d912ca37fba87e66ad33e4654a0b1`,
before the additive Settings starting-page lane. Do not reattribute it to final bytes.
The earlier16-case Settings matrix (`Z4BBUy`,814 checks) is superseded for coverage by24.
Prior Stop/Resume/history/form/Go proof is preserved in the Profile assessment, not
represented as freshly rerun for this test/document-only extension.
Post-document `go test ./cmd/appkit-mcp ./apps/gallery`, JavaScript syntax and whitespace
checks pass. Twelve root structural groups pass, including budgets, links, ADR membership
and stanza parity; this is a subset, not a full host/intent-doctor or semantic audit.
Independent review found no blocking mismatch. All five owned Gallery test processes
were stopped; the staged index remained empty and unrelated work was preserved.

| Artifact | SHA-256 |
|---|---|
| `shell_recovery.mjs` | `df832daace9cacd98117a83a8a9ee57554b2c2267bb19849e243be290edce9ef` |
| `recovery_assertions.mjs` | `bda1da1b5e17a184ba54178ec8351c5cd45c6ea54d72e96d211495c4a93f43c8` |
| `recovery_assertions_test.mjs` | `a32d1aafadcd56001a1f35bf27994bf664a50e54de208aa5f7a7e2d6d37593fd` |
| `/tmp/appkit-shell-typing.BHFSen1Wr7/gallery-final` | `ec41221a214860287fe14b9a3d4c0ce4ec3711071f3f7728eefbda8d73167276` |
| Same directory, `gallery-before` | `c3e2446e369bd7d23217919916587823c2a39b24d0d39d7aa51372ffde7ebc14` |
| Served/source `shell_demo.js` | `1c9a3096767c4864f05fcf861957ca7a2ea6c0584cb7f7001fa5ce4ddbfec6a4` |

Receipts are under
`/var/folders/2r/2zrkv4fd549bnlc5vwfzd4b00000gn/T/appkit-shell-browser-`:

| Suffix / filename | SHA-256 |
|---|---|
| `utOcOB/shell-recovery-settings-all-fields-report.json` | `1d9777e4718a26c37bb528ced45654029d9e79cd6ba3e9e8d5e87b6d8e08fb03` |
| `gDWxVg/shell-recovery-settings-old-allfields-report.json` | `7367a49fab78568e5df45e172833c6012e55b3639cc0d4c7df0bf40e37f755ee` |
| `Vf67yY/shell-recovery-settings-profile-regression-report.json` | `41a58b6b2d30f5db0de9c0a952ebbcc9462210d2a9a12beb7abc17dcac6f7b48` |

Playwright1.63.0, Chrome152.0.7977.84, Node26.3.0, Go1.26.3, local Apple Silicon macOS.
Fresh synthetic contexts, reduced motion and private assets disabled. Reviewed final
phone/dark time-zone recovery, desktop/light starting-page conflict and phone/light
checkbox recovery screenshots. Stitched fixed navigation is not viewport-clipping proof.
Temporary evidence is retained, not a durable backup or certification packet.

Arrow/popup keyboard diagnostics did not demonstrate selection changes; rapid consecutive
letter prefixes also failed the intended second value. Native label type-ahead with
a prefix gap supplied the measured path. Those earlier failures remain retained
(`eLDTOR`, `84ZYEG`, `VtYrTf`); they are not successful causality or no-repost evidence,
nor proof that every browser's popup/arrow interaction fails. No oracle was weakened.

## Reproduce and remaining gates

Use the [approved isolated browser setup](../../apps/gallery/browser/README.md), a fresh
owned Gallery process and `-design-assets=''`:

```sh
APPKIT_RECOVERY_INPUT=settings-input node apps/gallery/browser/shell_recovery.mjs
node --test apps/gallery/shell_demo_test.mjs apps/gallery/browser/config_test.mjs \
  apps/gallery/browser/recovery_assertions_test.mjs apps/gallery/browser/recovery_transport_test.mjs
```

This proves automated native control input on this Chrome build, not physical input,
IME, select popup/arrow/pointer paths, assistive speech, other engines, pending-save
BFCache, native-host input, full contrast/zoom, human performance or production saves.
It does not certify a converted target or finish G2's two-consumer acceptance gates.

## Consumer fit: reuse behavior, not invented authority

Read-only source review found shared `dashboard.RenderNav` / `RenderShell` in RAC's
`cmd/serve/shell.go` and Fathom's `cmd/fathom/observatory_render.go`. Their reviewed
[navigation evidence](2026-09-14-navigation-delivery.md) already covers selected
keyboard/narrow/theme/long-label behavior; Gallery coverage is not a substitute for it.
RAC's `settingsform.go` is a secret-bearing native credential workflow whose deliberate
nonretention/re-entry policy must remain. Inspected Fathom Observatory home/service
handlers are GET-only; degraded data recovery is not a new account-save backend.

The audit did not establish two comparable non-secret versioned save workflows ready
for a shared recovery runtime. Keep the recipe app-owned; use existing shared navigation
and native-form guidance. G2 requires replacement of overlapping behavior, not identical
account models. The next useful consumer proof is actual Fathom read-only handler and
degraded-state navigation, beyond exported renderer fixtures; secret-specific RAC
lost-response recovery is separate. The subsequent [actual-handler probe](2026-09-15-fathom-handler-read-failure.md)
exposes a read-classification defect; no consumer files were changed by either audit.

Intent: I8/I12/I15 and ADR-0019/0021/0026 remain; no public API, new ADR, future promotion,
dependency or breaking contract. INC-32 owns the defect. Impeccable hardening informed
the retained-work and explicit-recovery checks, without changing either app's identity.
