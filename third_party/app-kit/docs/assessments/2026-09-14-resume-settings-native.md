# Resume-as-Code: protected native credential setup

Owner-selected G2 consumer slice, preserving the app-kit Synthwave shell, local
credential writer and existing JSON API. This is not complete RAC redesign, OAuth
testing, a kit authentication service or G2 completion. No real credentials, live
records, provider actions, vendor/dependency updates or deployments were used.

## Finding and implemented behavior

Actual Chrome with JavaScript disabled submitted the old credential form by GET;
the URL contained the synthetic secret and nothing was saved. The form had only
`data-post`, with neither native `method` nor `action`. Static markup tests and
successful JS fetches would not prove native correctness.

The new local POST `/settings/google/credentials` declares real labels/help/errors,
44px controls and POST/303/GET recovery. It preserves a bounded client ID after
trusted validation/save errors, never the secret. The fixed error fragment targets
the focusable summary; its native link reaches the invalid field. Saving does not
connect Google. External client-file selection and hosted mode do not offer a false
save affordance; no-script copy names the remaining JS-only connection action.

The existing bound-address guard remains mandatory. Only this route admits native
form media; it separately requires exact Origin, unambiguous fetch metadata, an
expiring synchronizer token, a 16KiB body ceiling and three singleton fields.
Thirty-two process-local sessions have fixed 30-minute expiry, HttpOnly/Strict/path
cookies (Secure over TLS), no eviction of active forms and no secret-bearing flash.
Both HTTP writers serialize around the existing atomic 0600 file writer. Stale setup
forms keep existing credentials and explicitly say submitted values were not saved.
An error-POST/concurrent-save/GET interleaving preserves the failed-submit feedback
alongside current state, and consumes it once. Cross-process rotation is not added.

Hosted middleware now refuses native AND JSON credential routes before reading or
serializing bodies into queued intents. The handler independently refuses hosted
execution. This closes a sensitive middleware boundary; merely hiding a form or
checking `s.hosted` in an inner handler would not have prevented queued secrets.

## Reference and consumer divergence

Licensed MCP references inspected: `Application UI.Forms.Input Groups.Input with
label and help text` and `Input with validation error`, HTML/Tailwind4/dark,
source version `2026-02-02-230729`. They informed label/description/error anatomy;
no licensed source project or external runtime was copied. Role colors remain RAC's
existing Synthwave choice. Existing kit shell/token usage remains; RAC's vendored
subset lacks the current form primitive, so semantic form markup and trust remain
app-owned. No vendor refresh was used to claim shared-component delivery.

## Verification and remaining limits

Permanent RAC `cmd/serve/settingsform_test.go` covers actual SaveCreds/loadback,
0600 mode/no token creation, validation, temporary write obstruction and recovery,
exact origin/media/body/field refusal, token expiry/capacity/multi-tab behavior,
hosted no-body-read refusal, selected external-file handling, legacy GET query
scrubbing, stale outcomes, concurrent writers and unchanged local JSON responses.
Whole serve/google package tests, vet and race checks pass. Full RAC `go test ./...`
remains red solely on the existing `TestKitVendorFresh` agentic-kit drift
(`8b252304978b` → `2e1b1bbfd627`, `kits/lesson-transfer/lesson.go`). No pin or gate changed.

The [durable browser probe](fixtures/resume-settings/browser.mjs) runs against an
opt-in test server with temporary Google/discovery/registry stores. It allows only
Settings, its two local assets and the credential POST; it never runs normal
`ListenAndServe`, the poller or a Google connection. Chrome 152.0.7977.84 with
Playwright1.63.0 uses fresh contexts at 390/768/1440 CSS pixels, JS on/off.
Final result: **165/165 assertions pass**, zero browser/network errors or unexpected
requests. Six mode/width cases cover labeled fields, fixed redirects, retained ID/blank
secret, focus and native keyboard error links; one no-JS save and one stale-tab submit
exercise actual handlers. Light/dark system preferences retain the same Synthwave palette,
not two independently reviewed themes. Root inspected phone invalid/saved and desktop
invalid screenshots: visible focus, readable help/error hierarchy and no outer overflow.

Earlier 143/145 and 142/145 focus results are **oracle diagnostics**, not evidence of
a browser defect: browser-side animation-frame polling was unsuitable in the
scripts-disabled document. Bounded Node-side polling and actual Tab → error link →
Enter → invalid field prove the focus path without manually focusing any target.
The original GET-leak observation is independent and remains valid. Counts across
these evolving oracles are not a same-oracle improvement percentage.

## Final identities and private evidence

| Artifact | SHA-256 |
|---|---|
| Final browser harness | `c6c7d1b697a197a537590b2e38c235b0ff6b9caa75ecbb0643b48e75e90e8cf5` |
| Original GET-leak receipt | `1cc3df597687e7106e1dd5388d7998d8e4409b5d050b3867e9884c08e77a5101` |
| Final browser receipt | `8e586209ca0139139b51d6ec431e04c44dad9fc5ef5499047f5c15d06263e2e4` |
| Final CSS, 28,033 bytes | `80734bfae115985bc6bd269ad59beab59009a4515d0ad315d77b68c1febb1a52` |
| Historical packet manifest | `60cd3d2a1fd9385b9c37f340fd42bd8381ca17a9a643006c00a12e75e660012b` |
| Authoritative follow-up manifest | `10ab4995b8a6de4979f8acde6e572056591c32bea8e41ea0a19c07aa9141d37e` |

Private packet `data/design-assets/resume-settings-native.thMELt/manifest.json`
retains 29 payloads / 1,124,807 bytes: original leak, evolving oracle diagnostics,
the prior 165-check candidate, harness and selected before/after source. The final
CSS rebuild removed one unused 21-byte token declaration; it required a fresh browser
run. `data/design-assets/resume-settings-native-followup.mCqomb/manifest.json` binds
that run, eight screenshots, final CSS and hosted middleware source, plus the parent
manifest and current source identities: 11 payloads / 857,156 bytes. Root reread all
40 payload sizes/hashes and verified the parent/harness/source links independently.
Packets are ignored and need separate backup; never build inside this data store.
Receipts omit field values; PNGs may show generated synthetic client IDs, not secrets.
Selected hashes do not establish complete source/binary/dependency provenance.

Reproduction: [probe instructions](fixtures/resume-settings/README.md). App-kit's full
root Go tests pass with the private source copies isolated from package discovery.
The consumer adoption check first rejected CSS drift; after review its new CSS hash
passes with unchanged vendor/module identities. This remains a static check.

This slice does not certify screen-reader announcements, whole-page contrast,
genuine browser zoom, touch/IME, OAuth, hosted deployment, lost-response recovery,
cross-process writers, timing budgets or every form in the app. Existing Connect,
Disconnect, global Add and other enhanced workflows retain separate acceptance work.

## Return to the kit and durable intent

RAC ADR-027, I19, sensitive-surface routing, adoption notes, release notes and the
shared AGENTS source (imported by CLAUDE) now name the native/hosted boundary.
App-kit [shell guidance](../design/application-shell.md#sensitive-native-forms-consumer-return)
returns it through the existing MCP tool/resource, not a new server or session package.
An actual local MCP protocol session returned the exact 10,248 authored bytes through
both tool and resource, SHA-256 `ebcbcbc1dd7be2175319629d8c68533ba41822eaf3b11c71f34bec1c0d19af0b`.
INC-20 records the consumer lesson; closed kit INC-5/7 were compressed, retaining
their outcomes to keep intent within budget. No host-loading or fleet-enforcement
claim follows from documentation or hash checks.

Scoped intent-doctor follow-up (read-only; no host reload):

| Result | Coverage |
|---|---|
| PASS | App-kit root router/living budgets, agent parity, resolvable skills and observed tool-version agreement; scoped checker:90 documents,522 links,33 ADRs,6 goals,9 futures,0 failures |
| PASS | RAC ADR-027 (68 lines), 27-record index membership, I19/sensitive routing, required docs, 33 scoped links and CLAUDE's AGENTS import |
| FAIL, inherited | RAC scope169 lines, futures188, Unreleased105 exceed their budgets; not pruned by assuming old goals are complete |
| WARN | RAC router's older tool-version pins need integration re-verification; no untested version-pin update |
| SKIP | Broader RAC legacy links and actual host loading/adoption; app-kit's 17 oversized legacy ADRs remain identified history |

Conjure remains the owner-authorized optional F2 reference after RAC/Fathom; no
third migration is started. Windows and CA/GGB remain deferred. The Wails source
follow-up narrows the candidate to pre-load delegate containment; native execution,
bridge refusal and platform delivery are still unproven. All six kit goals stay active.
