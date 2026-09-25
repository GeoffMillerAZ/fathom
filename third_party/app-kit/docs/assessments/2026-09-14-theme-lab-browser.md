# Theme-lab browser review and recovery

Scoped G1/G4 progress under [ADR-0031](../adr/0031-local-app-owned-theme-review.md).
The existing local app-owned editor now has actual Gallery browser task evidence.
This is not a second consumer, reusable-widget promotion or completion of G1–G6.

## Findings and repair

The initial native Preview POST returned403: Chrome generated Origin:null from the
form page's no-referrer policy. HTTP tests had supplied Origin explicitly and missed
the failure. INC-16's shell lesson had not reached this receiver. The new regression
test fails before repair; the same final browser canary also fails against the old binary.
Only form-bearing responses now use same-origin referrer policy. Preview, CSS, ZIP and
access refusals retain no-referrer; null/missing/cross-origin POSTs remain refused.

At390px, initial review controls were5232.91px below the top. They now begin at594px.
Token sections start collapsed and reopen when edited. Native section shortcuts move
focus between editing, preview and export; ordinary controls and disclosures are44px.
The narrow heading no longer compresses Reset into a three-line sliver.

Invalid edits previously returned inside closed sections without focused recovery.
The error summary now receives native focus, names the failed control and links to it.
An internal typed field error supplies aria-invalid and associated error text without
parsing prose or duplicating validation. Correction clears stale error semantics.
Source remains immutable; no JavaScript, preference store or editor framework was added.

Impeccable technical audit/hardening informed these changes. Conservative reviewer
scores describe this editor only, not a WCAG claim or objective performance rating:

| Dimension | Before → after /4 | Evidence and remaining limit |
|---|---|---|
| Accessibility | 2 → 2 | Focus/repair path proved; full keyboard, screen-reader and zoom audit remain |
| Performance | 2 → 2 | No page scripts/fonts/images; no task-time or rendering-budget study |
| Responsive design | 2 → 3 | Initial320/390/720/1440px geometry and edit targets checked |
| Theming | 3 → 3 | Actual selected font/radius/accent use; no branded-font fidelity claim |
| Anti-patterns | 3 → 3 | Restrained task editor; consumer fixture identity stays separate |

Total12→13/20, an intentionally partial assurance result. Baseline issues: oneP1
broken preview workflow and threeP2 recovery/discoverability/target-size gaps. These
are repaired within the tested scope; broader accessibility/performance work is open.
No decorative image, template replacement or redesign of the paid references was needed.

## Final browser evidence

Chrome152.0.7977.84, approved isolated Playwright1.63.0, fresh temporary profiles.
All probes are [repo-owned and opt-in](../../apps/styleguide/browser/README.md).
They allow only selected loopback theme routes, reject external traffic/WebSockets,
retain errors and require the complete named assertion set; missing tasks fail.

| Probe | Result | What it establishes |
|---|---|---|
| Native preview, JavaScript enabled | 15/15 | Real keyboard POST, exact Origin, retained edit and acknowledgement |
| Native preview, JavaScript disabled | 15/15 | Same task without browser page JavaScript |
| Review/export journey | 47/47 | Three specimens × both schemes, source/draft/reset and guarded export |
| Editor usability | 30/30 | Four viewport geometries, native shortcut/Tab path, invalid-field recovery |

107 assertions pass across four runs. The final native probe fails on the original
binary; the final usability probe fails on the policy-only intermediate binary.
Do not substitute either intermediate result for the final tested implementation.

The journey reads real rendered styles: approved serif body fonts in all six specimen
states,12px on an existing rounded-md element and rgb(255,126,219) on a text-accent
element. Show source restores the original font/accent; candidate review restores edits.
The narrow iframe is at most390px. Contrast failure disables export; stale review409
and invalid radius422 preserve the draft and clear acknowledgement. Correction succeeds.

Two actual native downloads are identical:16,338bytes, SHA-256
`f9aea55f8b929e396f3607682e98ea031686a19f128d5e749e483da7c7030f92`.
The five expected members, exact original CUE, member hashes/sizes, response hash and
acknowledgement receipt agree. Files stay in the probe's temporary directory, not Downloads.
The source SHA remains
`228a32cb3be5943124020fd791858571112cbc546c20810cbb8fb44eb92d8f08`.

Final journey:70 observed responses,2,062,754 total decoded response-body bytes across
repeated navigations/downloads; this is not unique payload, transfer cost or a speed budget.
Four positive runs retain18 screenshots. Root inspected desktop/narrow editor and error
screens plus selected specimen output. Screenshot presence alone is not visual approval.
No unexpected console/page errors, outside requests or service-worker events occurred.
Expected409/422 network console errors are matched one-to-one to actual responses.

## Harness findings, not app regressions

Playwright's serviceWorkers:block injects a navigator.serviceWorker getter into every
frame; that throws in this deliberately opaque sandbox. Omit that injection, not the
sandbox. Require successful preview responses with script/form/font/image denial,
fresh contexts with no workers, worker-event assertions and the route allowlist.
Do not add allow-same-origin, bypass CSP or filter away the exception.

After native fragment navigation, a combined Playwright autoscroll/click could put
pointer-down into the iframe and pointer-up over a disclosure. An isolated diagnostic
confirmed ordinary clicks after explicit pre-scroll and keyboard activation both work.
The probe separates scrollIntoViewIfNeeded from the unforced click and asserts open.
Neither harness correction changes application behavior or makes a failed app check pass.

## Artifact identity and verification

Private packet: `data/design-assets/theme-lab-browser-v1.bpIzYk/manifest.json`.
Manifest SHA-256:
`811d7d1a0637d050a8f1537ed44f46b98a69ca0906781ef25e7ab54b196c9ae7`.
It binds six receipts, exact probes, positive/negative screenshots, exports, selected
source snapshots and three binaries; every copied artifact hash was checked.
This is not a full source/dependency build-reproduction receipt.
Final executable SHA-256:
`64c1b0a6ff154178cefe36d253d973de234f8dcf8ce082199bba7e71e7c53f8c`.
The tested live process is `http://127.0.0.1:56315/theme-lab`; local lifetime is temporary.
Earlier18766/53444/54707 processes are not this implementation identity.

Final scoped race runs pass: styleguide3.434s, Gallery18.938s, Gallery command cached;
the command also passed uncached1.670s earlier. Scoped vet and diff whitespace checks pass.
The two new named handler regressions preserve native form policy and section/error recovery.
Focused MCP tests pass; actual stdio tool/resource retrieval matches the6852-byte guide,
SHA-256 `bc40d55622707f06dae7fde7cea560a6af36887fe51b5a22425bc5b411ff6040`.
Scoped intent checks:83docs/496links/33ADRs/six goals/nine futures, zero failures;
17 over-budget legacy ADRs remain documented debt. This is not the full intent doctor
or host instruction-loading proof. No consumer, global config, dependency or deployment changed.

## Remaining gates

App-owned fixtures remain static, opaque and unable to run scripts/forms or load fonts
and images. Selected CSS measurements do not prove all token uses, contrast, chart
geometry or brand fidelity.720px reflow is not an actual browser-zoom test. Full keyboard,
screen-reader, mobile-device, task-time and render/update budgets remain unmeasured.
Mount a real consumer-owned guide with its actual theme/styles/fixtures before claiming
consumer adoption; local font or active specimen support requires a separately reviewed
boundary. The six goals, conversion certification and native-host delivery stay open.
