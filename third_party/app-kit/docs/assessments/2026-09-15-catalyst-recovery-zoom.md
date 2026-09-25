# P10 backend-refusal recovery and actual browser zoom

Follow-up to [native/enhanced settings](2026-09-14-catalyst-enhancement.md), under
[ADR-0044](../adr/0044-native-catalyst-settings-composition.md). Complete private form,
not a sixth catalog adapter, certified target, account backend or consumer migration.
Later [native-focus evidence](2026-09-15-catalyst-native-focus.md) records the CSS
companion, new sixteen-file replay and retained concurrency failures; this page's
binary/receipt identities remain historical, not silently replaced by later results.

## Implemented recovery

The synthetic fixture accepts an explicit `--refuse-first-validation` argument.
Only the first otherwise-valid validation returns503; normal launch does not opt in.
Framing/origin400, CSRF403, field-validation422 and Reset200 do not consume refusal.
An atomic counter test covers128 concurrent attempts; exactly one refuses.

A503 retains all ten fields, leaves them editable and focuses the visible status:
the backend is unavailable, the draft is retained, nothing was saved, and Validate
changes retries. Forward Tab follows the preserved source order: Reset, then Validate.
Correction requires no request; native Enter validates the corrected retained draft.
Reset restores the independent initial snapshot. Enhanced Reset keeps the same focused
button without a request; the native route uses its separate protected reset form.

No source layout, auth boundary, canonical options or persistence policy was weakened.
The focused status is an explicit app-selected presentation target, not a field error.

## Selected inputs and exact replay

Private source: `data/design-assets/p10-source-ohWeRq`.
Reference SHA256: `94d3ab4c145e33bf29af750918429833d833fff3f9ac8820e1314c7f2c67f53d`.
Two current modules, `p10-go-1747876674` and `p10-go-4188805750`, match all fourteen
recorded files byte-for-byte; actual file hashes were independently checked.
`SHA256SUMS`: `6663ff9ca98d4a78e8a60d4d066688b13049976b266d5f29f740943d6c10cf7a`.
Binary: `bfe046e3f8c71e700d4666a02ef11a7151f639e61e967f41952e7e12d20bf764`.
The tested server's earlier module `p10-go-3755798389` has the identical executable;
its manifest differs because the later replay includes the additional origin test.
Go1.26.3 darwin/arm64, templ v0.3.1020 via the kit-pinned modfile; no LLM in replay.

## Browser evidence

Chrome152.0.7977.84, Playwright1.63.0, Node26.3.0; local synthetic data, fresh isolated
headless profiles, reduced motion. No signed-in profile, portal or external app traffic.
The runners freeze plans before tasks; report status is not target certification.

| Final directory under `data/design-assets/` | Scope and result |
|---|---|
| `p10-recovery-browser-2RKBYL` | 114 checks, eight390/1440 light/dark native/enhanced cases. Each owns a fresh refusal process; all eight exit0 with empty stderr. |
| `p10-zoom-browser-9t24in` | 76 checks, four actual200% light/dark native/enhanced cases; complete default/error captures. All four owned profiles removed after successful close. |
| `p10-native-browser-PK3p4d` | Unchanged native runner:78 checks, including invalid drafts, Enter and Reset. |
| `p10-enhanced-browser-w6TbY4` | Unchanged enhancement runner:81 checks, including local feedback/reset and withheld-module fallback. |
| `p10-seam-browser-Nc0aU0` | Unchanged seam runner:23 checks, including hostile/duplicate configuration and lifecycle. |

All372 scoped checks pass with no reported browser exceptions or forbidden requests.
They are assertions across named scenarios, not372 unique components or user studies.
Report SHA256s in table order:

- `4099886fb7e23ce672b0bd4e245c1b1c5ecf7ba5b6c4c4435ea3b4e1e3bc02d8`
- `08eb362af2ac8fac7e69a534b8800fa86808983c69670803451beff6bd99bae3`
- `62af95878e9ca02f3226959e9a54d13e4178624a82144687c0d2b1187998afb4`
- `8374002da4a230f59a4f2a6c6d5772162cb92f80638a9be117c8028a8aa9ef4b`
- `fa48d6d75e1b094687b5221c1ae045c31479423f2654cb94b0dc3995c394a458`

Recovery plan: `315211797cba8bda33ebccd34c68916539a67a4f3169d8dfc58b0c42058fe9ba`.
Final zoom plan: `0ca901d1798dc6078826e291bb9952cc332de19d1700386cc153df5953af8115`.

## Actual zoom and visual-evidence repair

Chrome's own Appearance setting selects200%, then leaves WebUI before application
restrictions. Observed1440px outer width,720 CSS-pixel inner/client width, DPR2 and
visualViewport.scale1. Native cases disable application script execution; no CSS zoom,
device-scale-factor substitution or CDP page scaling is used. Controls and checkbox
labels satisfy the selected44 CSS-pixel geometry checks; no document overflow.

The first collector `p10-zoom-browser-qMzsfy` passed68 DOM/task assertions but its
Playwright locator captures were cropped: default656x1178 rather than the full
1440x2484 document. Manual inspection caught missing controls/sections. Preserve those
images as insufficient visual evidence, not a successful full-frame baseline (INC-28).

The replacement uses measured CDP `contentSize` DIP coordinates and records the2x
ratio to `cssContentSize`, complete form/final-action bounds and exact PNG dimensions.
For this pinned Chrome, defaults are1440x2484 and errors1440x2660. This is a measured
version-specific workaround, not a universal guarantee for the deprecated CDP field.
Six source-only tests reject the historical crop, viewport-only capture, malformed
metrics, missing/clipped actions, truncated PNGs, CRC corruption and incomplete pixel
rows. The helper checks the measured non-interlaced RGB/RGBA encoding and fails closed.
Review found the initial header-only check insufficient; it was tightened before the
final run. The helper identity is frozen in the final plan.

Intermediate `p10-zoom-browser-8qv6My` and final `9t24in` each pass76 tasks and match
all eight PNGs exactly. Default dark/enhanced, error light/native and mobile dark503
frames were visually inspected; complete form/action composition is visible. All eight
native regression PNGs match `TWjgEj`; all eight enhanced PNGs match `I6TijX`.
No screenshot tolerance or purchased-source baseline was changed.

Recovery's earlier `p10-recovery-browser-MZGH2l` failed after ten assertions because
the test expected Shift+Tab from status to reach Validate. It correctly reached Currency:
source order is Currency, status, Reset, Validate. Independent review approved testing
forward Tab twice, without moving the source layout. The failed report remains retained.

## Other checks and remaining work

Two private replays pass complete-source generation, standalone race/HTTP tests and
binary builds. Source-only race: `internal/designadapt`27.966s and `cmd/appkit-mcp`22.315s;
both pass `go vet`. The six capture tests and five controller tests pass; capture tests
are wired into `task check`, but the entire maintainer gate was not rerun.

Open: physical-input/screen-reader and full contrast/accessibility review, named payload
and task/update performance budgets, complete paired source-fidelity review, immutable
private export/CLI/catalog exposure, vendor-upgrade replay and real consumer integration.
HTTP503 is a responding synthetic backend, not a dropped connection/proxy-outage test.
No consumer writes, dependencies, global config, commits, publishing or real data changed.
G1–G6 stay active;439/657 compiler/DOM inputs, five catalogued adapters, zero certified.
Windows and CA/GGB remain owner-deferred.
