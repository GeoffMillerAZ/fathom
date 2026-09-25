# Actual Fathom handler proof exposes unreadable-as-zero

Date: 2026-09-15. G2/G3 remain incomplete. Initial investigation preserved the consumer;
the [authorized adoption](#authorized-reader-adoption) below now applies the narrow repair.

## Finding

The app-kit-owned [actual-handler probe](fixtures/fathom-handler/README.md) discovered
a source-to-UI defect that the exported renderer fixtures could not test. Fathom's
`readCallLog` opens the input and scans it, but returns `read=true` without checking
`Scanner.Err()`. `classifyCallLogRead` trusts that success. The actual Observatory then
receives no read-failure note and can display zero traffic instead of unavailable data.

The synthetic input is an existing empty directory used as `FATHOM_CALL_LOG`. Stat and
Open succeed; reading it fails. This both reproduces the error and prevents fallback
searches from reaching real logs. A scan failure after a valid prefix is a related
incomplete-read risk; it requires its own regression, not an inferred browser pass.

This does not invalidate the previous exported-model navigation/analytics results.
Those results explicitly bypass handlers and data readers. It shows why that evidence
cannot establish correctness of the complete source → model → rendered task.

## Probe boundary

`run.mjs` verifies the supplied canonical Fathom Git root/module, refuses an existing
virtual test path, generates a Go `-overlay` and checks that Go discovered the test.
It runs the real landing/fleet/service/CSS handlers against an exact synthetic registry,
without the operational daemon/RPC server, providers, stores or mutations. Both HTTP
transport and browser navigation are restricted to the owned loopback fixtures.

Arm, narrate and ledger paths are temporary. The pathless alpha service avoids corpus
Git/config/observation reads. Code/module input remains the consumer's current dirty
worktree, while compilation uses a private cache and already-present offline modules.
The wrapper checks selected source/CSS/module/mirror bytes, fixture hashes, Git inventory
and absence of the virtual file after execution. This is not full build-closure proof.

Registry503 and pathless-service scenarios exercise actual native skip/Menu/Tab/Enter,
home/fleet/detail/return, theme and viewport states. The Go side separately checks
405/404, exact registry operations, bounded responses and pathless service JSON.
Browser stdout alone cannot pass: the Go probe verifies the persisted scenario/origin,
source hash, all assertions, four unique finished cases and actual screenshot hashes.

## Initial retained run

Evidence root:
`/var/folders/2r/2zrkv4fd549bnlc5vwfzd4b00000gn/T/appkit-fathom-handler-nFbCMH`.

- Registry unavailable: four browser journeys /136 checks pass, light/dark at390/1440.
- Pathless service: the first browser journey fails at the missing unreadable-log warning
  after22 passing checks. It is not a completed matrix.
- HTTP report has82 checks and three failures: the browser child plus two incorrect
  harness assumptions that compiled CSS must contain `--color-` names.
- The CSS assertion is corrected to exact response/embedded `obsAppCSS` byte equality,
  a stronger delivery check. The unreadable-log oracle is unchanged; no product repair
  is credited for correcting a test's token-name assumption.
- Selected source hashes, fixture hashes and consumer dirty inventory remained unchanged;
  no virtual file was materialized. All Go-owned servers and browsers shut down.

All receipts remain retained and failing; temporary artifacts are not durable backups.
Chrome152.0.7977.84, local macOS arm64; not a native-host, physical-input,
assistive-technology, full accessibility, production performance or live-fleet claim.

## Corrected-harness confirmation

Root suffix `appkit-fathom-handler-xt8ViL`, in the same temporary parent, finishes with
**81/82 HTTP/evidence checks passing**. The only remaining failure is the pathless
browser child. Its actual fleet page shows **tool calls (log):0** and **traffic:0 calls
(0 agent)** despite the unreadable input. Registry-outage navigation again passes all
four journeys/136 checks. The service browser stops after22 passes at the unchanged
read-error assertion; no service-detail/return completion is claimed.

The selected source and fixture hashes, consumer inventory and absent virtual path all
match before/after. Go1.26.3 darwin/arm64; Fathom HEAD
`e1841718fa07af964a3d5dc5c9d6f68eb26516c5`. The inspected reader remains clean against
that HEAD, SHA256 `e56cf78ddc16160a0536e245b26f1d5c063a2b2ec0749764d4fb9986228ef197`.
The original manifest did not include that reader; subsequent wrapper inventories do.

| Retained artifact under `xt8ViL` | SHA-256 |
|---|---|
| `run-report.json` | `4619bb3c79527b3b5b688d88ad5b57fc8fba30aa842d768a3422342bb72a8d92` |
| `reports/http-report.json` | `126d98aaf14cd4b0645810561a434e30c4c9064cba33116e50742687bf3cb996` |
| `reports/registry-unavailable-browser/browser-report.json` | `087a66a4422c149d13bf1057931b05521b221ba4c9b45dd691be61c067c039c5` |
| `reports/pathless-service-browser/browser-report.json` | `6cab6bccf4a180a444b68ef10f5f16f516e48c0537d0620c86cc5697d83d48bb` |
| `reports/pathless-service-browser/failure-pathless-service.png` | `66a78717c15015a9ca25da1aaa60f966423f2dc44a4acd1e5e93b4c1e9898197` |

Runner identities for this receipt: `run.mjs`
`e7e63a415d29bcd938424230d897899fc282626c6574ee94fd709c1de7d26556`, browser
`d3109eb88c7bbdc5b237b8648b5517b08b2561d329578ccbfe6c3e35300a6074`, Go fixture
`43ae3384da0524f841e8a182703067beab5933c1a735c5438f56a25743f83036`.
Later wrapper inventory additions do not change this historical identity.

Reviewed light/pathless and dark/registry-outage phone captures expose additional
open G3 issues: the aggregate says **all fresh** while the pathless row says freshness
unknown (and even when the registry is unavailable); the pathless grade bar extends
past its narrow visible region. Page-wide `scrollWidth` did not detect that local clipping.
These are separate freshness/geometry gaps, not covered or repaired by the requested
call-log change. The registry navigation passes must not be presented as data-quality
acceptance. No consumer UI patch was applied for these findings.

## Repair boundary and return to the kit

Owner approval was requested for the narrow Fathom call-log reader, synthetic tests and
changelog only. Until granted, no consumer repair is applied. Preserve both current
reader signatures and valid/malformed-line behavior; fail incomplete reads explicitly,
retain the cause, and avoid the classifier's second-open file-descriptor leak. A prepared
patch is a candidate, not deployed or integrated behavior.

## Unapplied reader candidate: unit red/green evidence

The [reader patch](fixtures/fathom-handler/call-log-read-errors.patch) and
[synthetic regressions](fixtures/fathom-handler/call_log_read_errors_test.go.txt) stay
in app-kit. `git apply --check` succeeds against the unchanged consumer reader, but
the patch has not been applied there. Temporary Go overlays evaluated its original
candidate in `/tmp/appkit-fathom-handler.DezLOg5c`:

| Exact named unit test | Current reader | Temporary candidate |
|---|---|---|
| Directory read error; oversized line after a valid prefix | 2 fail | 2 pass |
| Valid/malformed JSON; measured empty file; ENOENT | 3 pass | 3 pass |

`reader-before-report.json` SHA256:
`e47722d3f7458eb4b28476538152c95b2f09ca68d4f33a9a6ebd551434f84054`.
`reader-candidate-report.json` SHA256:
`958e55121919f99d78729e25ad08c9196b9bae300849fc729f644fd7e71ee10a`.
That candidate source was
`f8c2207af6bfdcbea53cd1a4d9f970715219dfb1460ccff6c660a6c4c09b94f8`;
the test fixture was
`4eb495d254d6a8d30d5f2d5a3ca5022f1d31d9f193beabcebd38e0baac08431f`.
The two old failures stop at the classified-reader assertion; only the candidate
executes both API assertions in those cases. No consumer file was materialized.

The subsequent actual-handler candidate run `appkit-fathom-handler-a292cp` completes
all eight browser journeys: registry136 and pathless208 checks, plus88 HTTP/evidence
checks and the same five reader cases. The unchanged browser oracle now sees an explicit
unreadable warning. Reviewed phone output replaces aggregate traffic0 with an em dash;
it still has false aggregate freshness, grade-bar overflow and clipped per-service
read-error text. Therefore these passes do not establish complete analytic quality.
The run receipt binds the old candidate above and all selected consumer/fixture hashes;
source bytes, dirty inventory, absent virtual files and temporary candidate snapshot
are unchanged. All owned browser/HTTP processes completed.
`a292cp/run-report.json` SHA256:
`97ad71b582fe558a7be83be57cffd60a794a0c060e486f0bd3ecbeee20468be6`.

Review then clarified only the candidate comments: decode-invalid JSON differs from
an incomplete scanner read, and general scanner errors do not establish a TCC cause.
Open-error handling preserves the existing helper behavior; no new host remedy is
inferred. Historical receipts retain the earlier source identity.

### Final candidate replay

Final source: `/tmp/appkit-fathom-reader-final.Qe9aF0O0/instrument_adoption.go`, SHA256
`20052f0bf5cf364aaaf36f3d05a19f09295206cff8dbb4f467fd82ad93ec0292`.
Patch SHA256: `f895d801ac251c50f5dacd3894e5d0f105ee86f857fcd58f56552be907faa2c0`.
The wrapper admits only this source on the checked base, snapshots it privately and
requires both discovered test names to pass. This deliberately is not arbitrary overlay
support. Its eight synthetic refusal tests pass separately; no real consumer is used
by those refusal tests.

`appkit-fathom-handler-58vxBy` (same system-temporary parent) reruns the final source:
five reader cases,88 HTTP/evidence checks and eight complete browser journeys with
136 registry/208 pathless checks pass. All five source/inventory/fixture/virtual-path/
candidate-snapshot preservation checks are true. The final dark1440 screenshot was
reviewed: aggregate unknown traffic is explicit, but per-service error text exceeds
its card even at desktop width. Freshness and local containment remain failing quality
requirements, not waived gates. No read-error recovery transition or full UI acceptance
was tested. All owned servers and Chrome instances exited normally.

| Final receipt | SHA-256 |
|---|---|
| `run-report.json` | `b480508a7a0b556e265af4853eca6f48df7f7d6495e9daaa8eb821493c867bd9` |
| `reports/http-report.json` | `4fa8d45c23a0323d0bb235bbf5811be8311eaf5c161a8c27b166c14ea68d57ff` |
| `reports/registry-unavailable-browser/browser-report.json` | `fbdea0e7893c90af365a4198871afb5e4c259eafb23134872853e11d00ed1440` |
| `reports/pathless-service-browser/browser-report.json` | `9235ab4cc562f997ac171f2edd61bb58ce277530d80b585accff541844fc1f7a` |

The final manifest binds runner `227f58dc61cbe524d0b1ca6c565637274ff0cefd1cf0ff40bf7c17642f440020`,
Go handler fixture `106f34bbbf389c65a7cbc33d2a845c49f682e25aeaa7a5936df9e1d7d130efd3`
and unchanged browser/reader-test identities above. Gofmt changed the earlier Go
fixture's whitespace; it does not retroactively change either historical browser run.

Remaining caller risks are explicit: `metrics_io.go:readCallDataFor` discards the read
boolean, while `flywheel.go:flywheelCallLog` reopens a failed scan and can leak that
second descriptor. This patch fixes the classifier's own reopen only. Neither those
callers, the complete instrumentation report, permission-specific runtime behavior nor
the broader Fathom suite is certified by these five tests.

Final local checks: `go test ./cmd/appkit-mcp ./apps/gallery`, eleven Node tests
(eight candidate refusals plus three shared browser-config tests), both runner syntax
checks, Go fixture formatting and `git diff --check` pass. A focused link/budget check
resolves133 local paths (not anchors); router78/80, scope150/150, incidents150/150 and
Unreleased58 body lines remain within budget. These are not a full intent-doctor or
host-wiring audit. Independent static review found no actionable defect in candidate
mode or its refusal tests. The staged index remains empty; no commit or consumer edit.

G2 does not require inventing a Fathom account backend. Its applicable recovery includes
truthful degraded data and usable navigation. Keep source-reader failure tests alongside
the pure renderer oracles. Neither a correct model fixture nor an HTML200 proves that
an operational data failure reaches the person using the app.

INC-33 records this consequence for app-kit. Fathom I38 and app-kit I8/I11/I15 govern;
no new ADR, dependency, runtime abstraction, consumer pin or future promotion is implied.

## Authorized reader adoption

The owner authorized the reader, synthetic tests and changelog on 2026-09-15. Applied
only `cmd/fathom/instrument_adoption.go`, new `call_log_read_errors_test.go` and a
changelog handoff in `/Volumes/m2ext/code/geoffmilleraz/sextant`, still HEAD `e1841718`.
The shared Fathom index points at a scratch clone; direct checked source, not that
index, establishes this consumer's state. App-kit observation O19 records the mismatch.

The unchanged baseline reproduced two failures (directory and oversized line after a
valid prefix), while three unaffected cases passed. Applied source is byte-identical
to the final reviewed candidate above. The five new cases now pass with `-race`,
including wrapped path/scanner-cause assertions; the three existing `TestReadCallLog_`
cases also pass. Both signatures, valid/malformed-line handling and measured empty
reads remain unchanged. Reader test SHA256:
`15695a80c7361c90e83469c358b582798fa792b51d9cfed30e28897fe8332777`.
Red/green transcripts and the preserved unrelated tracked diff live in the consumer's
`workspace/tmp/appkit-reader-adoption.RBWp11/`. That diff compares byte-for-byte before
and after, excluding only the edited reader and changelog; it is not a whole-tree lock.

Fresh actual-handler run `appkit-fathom-handler-MGCXKP` uses **current-consumer** mode,
not a reader overlay: 88 HTTP/evidence checks plus eight completed browser journeys
(136 registry and 208 pathless checks) pass. Selected source/fixture identities, dirty
inventory and absent virtual probe remain unchanged during the run. Run receipt SHA256
`cb5e1b4f40c3dc30d915289d3ed6733daaab35b884a42b704fd7feb87a44c498`;
HTTP receipt `091d20de2e88f43f3056f487de06550c84339aa939f6f64d9d1af6a44da7de13`.
The unchanged runner/browser/Go fixture hashes are recorded in the previous section.

Root reviewed dark390 and light1440 captures: traffic is visibly unavailable, not zero.
False aggregate freshness, overflowing grade content and per-service error clipping
remain outside this authorization. No full-suite, recovery-transition, live-log,
native-host or production acceptance claim. The test-owned browsers/servers terminated;
no commit, deployment, dependency, global configuration or real-data operation occurred.

## Freshness and local-containment refusal

Later2026-09-15 app-kit work makes those remaining findings executable in the existing
actual-handler browser collector. No Fathom source is changed by this follow-up;
the requested freshness/grade/error display repair awaits owner authorization.

Both registry-unavailable and pathless scenarios now inspect the specific index-
freshness metric in `fleet evidence status`, refusing **all fresh** and requiring an
explicit unknown/unavailable headline or the kit's absent marker plus its named reason.
The pathless scenario also measures the named grade-distribution SVG against its
widget and the unreadable-log text's line rectangles against the alpha card and its
local clipping ancestors. Positive SVG dimensions, unique matching error text and
local display/visibility/opacity checks prevent selected hidden-content false passes.
This is a scoped geometry/absence check, not photometric visibility or whole-page
uniqueness proof: service-region/freshness-label fields are presence indicators after
selection. It does not accept a cropped chart solely because page `scrollWidth` fits.

Quality failures accumulate while the original keyboard navigation task continues,
then produce a nonzero exit. Every original navigation assertion remains; all eight
journeys finish and retain their screenshots even though the overall run fails.
This extends the oracle, not the consumer's measured capabilities.

Retained root:
`/Volumes/m2ext/appkit-evidence/appkit-fathom-quality-final.YaLFho3p/appkit-fathom-handler-wCk5Fw/`.

| Report under that root | Outcome | SHA256 |
|---|---|---|
| `run-report.json` | Current-consumer run failed as required | `db6bdbee2b05a86266f6d9bad92839a509645c6acffef3bb28ddf4ba2ffb5796` |
| `reports/registry-unavailable-browser/browser-report.json` | 140 checks; four freshness failures | `2aef7153661d7032649b67440ea5d79f4c2d1f409f7365855d8aec043c63fa7a` |
| `reports/pathless-service-browser/browser-report.json` | 220 checks; ten quality failures | `edb87af7f76a4f0b022e121d319283f98d320fdac84c50fe21a84b563a1c7c51` |

The fourteen browser failures are eight false freshness summaries, two narrow grade
overflows and four unreadable-log containment failures. At390 CSS pixels the420px SVG
extends beyond its358px widget. Error lines reach x620.7 beyond the phone card's x374
edge and x888.7 beyond the desktop card's x648 edge. The wider viewport still lays out
the service card at368px; screen-wide checks alone cannot detect this failure.
Phone-dark and desktop-light captures were visually reviewed; all eight saved PNGs
independently match their recorded hashes. No blocked traffic, page or console errors.
The HTTP report has76 checks and two expected browser-child failure records, not an
all-green HTTP/browser acceptance claim.

Collector SHA: `b97d59cc04e254df494e2d5e0d02be16345d29679b4f15000e8da84c8d821b62`.
Consumer HEAD remains `e1841718fa07af964a3d5dc5c9d6f68eb26516c5`;
selected source/CSS/module/mirror identities, dirty inventory, fixture identities and
absent virtual test path were preserved. All owned probe processes exited. These are
working-tree and synthetic-input checks, not full immutable-source or real-data proof.
Keep this red result and rerun the reviewed oracle after an authorized repair; do not
replace it with the older navigation-only green receipt. G2/G3 acceptance stays open.
