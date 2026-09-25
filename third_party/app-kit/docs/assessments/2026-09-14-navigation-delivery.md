# Navigation delivery: kit → Resume as Code and Fathom

Owner-authorized, no-publish navigation/focus upgrade. App-owned routes, icons,
Synthwave tokens, authorization and business behavior remain unchanged. No real
records, deployment, global configuration, schema changes or dependency downloads.
[Authorized handoff and boundaries](2026-09-14-navigation-upgrade-handoff.md).

## Status

| Boundary | Evidence | Limit |
|---|---|---|
| Isolated kit change | `c2b3a90` adds typed navigation/focus; `63b3a4f` repairs measured target/contrast/Menu gaps. Both six-file candidates pass isolated baseline gates; final full-worktree Go tests/vet also pass | Experimental; not two-consumer promotion |
| RAC integration | App-owned `navPages` adapts to `RenderNav`; exact nav/shell vendor bytes, CSS/task inputs and adoption hashes updated | Dirty working-tree subset, not a release pin or whole-vendor refresh |
| Browser review | Final unchanged 2,094-check oracle: 224 pre-repair failures → **2,094/2,094 pass**, including Menu target/focus; 24 screenshots | Synthetic shell, not whole-app certification |
| Standalone delivery | Code-only snapshot: 2,985 stable files; two identical binary builds, exact vendor resolution and synthetic shell race tests pass | Existing compiler cache; no cold-cache, clean-Git-checkout or release claim |
| Fathom | Exact `63b3a4f` mirror; seven-route adapter/shared skip; workspace and snapshot race/vet/build proof; 1,207 browser checks; pinned prospective catalog check passes | Uncommitted consumer work; tracked-only build and full propagation remain open |

## What the browser actually caught

Native skip → main, eight app-owned destinations/current state, real Tab traversal,
native mobile disclosure, link navigation/back and separate long-label stress wrapping
passed. The shared links were **38px** high, below the selected **44px** target. The
active accent-colored text on its soft accent background measured **3.0469:1**, below
the **4.5:1** text gate. The 2px focus outline itself passed at **6.3805:1**. These
are distinct checks; a visible outline does not make a label readable or target large.

The committed shared repair uses normal `ink` text on the soft active background,
retaining its accent surface, bold current state and `aria-current`, and gives links
and the mobile Menu disclosure explicit 44px minima. Consumer tokens do not change.
The Menu has explicit 2px focus treatment. The exact vendor/CSS delivery passes the
same browser thresholds: nav target height is at least 44px, sampled nav text contrast
at least **5.9986:1**, and 2px focus outlines at least **6.3805:1**. The final expanded
oracle adds 32 Menu failures to the original 192; all 224 are fixed without lowering
thresholds. All 792 recorded HTTP responses are local 200s; no unexpected traffic or
runtime errors. Desktop and narrow long-label screenshots were visually inspected.

The first partial run retained six failures and a click timeout: its oracle assumed
that layout rectangles meant closed-menu visibility and that Menu was the second tab
stop. Chrome retains layout under closed details, while RAC's header Add/palette
controls precede Menu. Corrected checks exclude closed disclosure content and traverse
the actual header with Tab without activating it. The prior native skip adds a URL
fragment; nav-order trials start a fresh document so fragment restoration cannot
change their initial focus. No application fix was credited for these oracle repairs.

## Synthetic scope and reproducibility

RAC's opt-in `TestExportAppkitNavigationPreview` renders the real shared shell for
all eight declared destinations, with synthetic content and no stores or providers.
It removes application scripts/forms and exports the exact embedded CSS plus a hashed
manifest. The separate long-label fixture does not change the app's real route labels.
Use an existing empty child of Go's system temporary directory:

```sh
APPKIT_NAVIGATION_PREVIEW_DIR=EMPTY_TEMP_DIRECTORY \
  devbox run -- go test ./cmd/serve -run '^TestExportAppkitNavigationPreview$' -count=1 -v
node browser.mjs PREVIEW_DIRECTORY EMPTY_REPORT_DIRECTORY
node standalone.mjs RAC_CHECKOUT EMPTY_REPORT_DIRECTORY
```

The [browser harness](fixtures/resume-navigation/browser.mjs) uses the previously
approved isolated Playwright 1.63.0 and installed Chrome, with a fresh profile.
Matrix: eight routes × 320/768/1440 CSS px × light/dark system preference × JS off/on;
the app intentionally keeps **one Synthwave palette** in both preferences. An additional
12 cases exercise the long-label stress fixture. Only allowlisted local GET/HEAD is
served; application scripts, form submissions, providers, external assets and sockets
are not enabled. This is actual browser-driven synthetic shell evidence, not the full
app, palette-script execution, physical-device testing or accessibility certification.

The [standalone harness](fixtures/resume-navigation/standalone.mjs) copies only
`cmd/`, `vendor/`, `go.mod` and `go.sum`, never `resume/` or `workspace/`. It uses the
existing Devbox environment to resolve the already-cached Go 1.26.2 offline, then runs
that exact executable with vendor mode, `GOWORK=off`, `GOPROXY=off`, `GOENV=off` and
`GOTOOLCHAIN=local`. Git environment redirection is removed for inventory/HEAD checks.
All copied bytes and the source inventory must remain unchanged through verification.
The original nested-shell invocation failed; Go's `-C` avoids that command envelope.
The Devbox base Go 1.24.3 was insufficient, so cached toolchain identity is explicit;
no installation or download was used to turn that failure into a pass.

## Receipts and private retention

Original temporary runs remain intact. The final private capture is
`data/design-assets/navigation-delivery.VDesq4`: **3,099 payload files, 46,751,646 bytes**,
plus manifest SHA-256 `d753d18581c394a5b6b99220131df04d65b64fdeddabab4244c7c033bd1cb200`.
It retains all four browser receipts/screenshots with their exact harnesses, both
preview exports, final code-only source snapshot, four baseline source overrides,
standalone reports and failed command attempts. Hashes and private file modes were
rechecked. No binaries or real records are retained. This ignored store needs separate
backup; it is not public catalog content or a committed consumer snapshot.

- Initial partial browser: `appkit-rac-navigation-browser.R8hcz68v49`, report SHA-256
  `5a8def9a8ccd854029f6805fd3a2f42952bd2aa05caa498417478f16115f99b9`.
- Complete pre-repair browser: `appkit-rac-navigation-browser.pxnzjrMm32`, report
  `ef6241b82117fac17c4dc0d4ee38292ef3c1fe051117a73ae2cd36a6720cb477`.
- Pre-repair standalone: `appkit-resume-navigation.dsC29Gqq`, report
  `6e6e853d43af28c667c20bb50189936383ed7bb74dd0d31bf5653317e9ab45b5`;
  identical binary SHA-256 `232a0f8d2e37a7814df014ab81b1d85bd469a393e145d502385f53f9c2d7203`.
- Final browser: `appkit-rac-navigation-browser.weIX8zRO1m`, report
  `7d917ec26d6dd4d68cbf1b9e420d1f866931b3f3527ce5bae41639a53ac7d15c`;
  harness `2b044ea47da4d8326f288ec1c7bcb36563657478f20d28dbdf21dcbe90f36e86`.
- Final standalone: `appkit-rac-standalone.qSrPBbmaaB`, report
  `b788d5b76060c0f7c610650973f6f305c2780f7a6ea283b991fd54fe76ffd617`;
  identical 65,837,250-byte binaries
  `d9bedbe5a4b63c5c424133e20bfe657546dc2af1271e4294e74b2f1a7fe18bfc`.

Final RAC artifacts are exact kit `63b3a4f` bytes: nav SHA-256
`84e23d5c9eeaee8028a906c5be7bb507fe9f0c8db26ea023703786beec6b698c`, shell
`e0e0ba904db70ae7b5aca2597c247ddabf40ee246658f94ffbee1605e57e53fa`; rebuilt CSS
`1b78ece20de6d17fae8ec2b48e67ca06c5d34ab4a8c1f99266840340c9c2487f` (28,591 bytes).
The second CSS task invocation is correctly up-to-date. Module declarations, other
vendor files and consumer HEAD `7a6c56a26dd10a7cf80128d23ef13bab90377ae8` remain unchanged.

All are children of the macOS system temporary directory, not public assets. Failed
standalone command receipts (`UgWRvguNaO`, `onHGuHGdWV`) remain separate from success.

## Remaining boundaries

RAC's whole server race suite, vet, selected navigation/vendor lint and static adoption
check pass. Full lint still fails the unrelated `TestKitVendorFresh`: agentic-kit
`kits/lesson-transfer/lesson.go` differs from its pinned mirror. That dependency is not
upgraded or its gate waived. Final post-repair browser/build evidence passes; filesystem
EPERM attempts against CWD, Devbox and the compiler cache remain recorded separately.
The owner-approved RAC change is not committed here, preserving predecessor UI/CSS
work. Fathom's owner later handed off its paused batch explicitly; the scoped result
below replaces that deferral. G2/G4 remain open beyond this navigation slice.

## Fathom owner handoff — 2026-09-14

The owner confirmed Claude Code had paused on its weekly limit and requested a
changelog explanation. The existing `e184171…` worktree was not committed or staged.
Only `obsNavigation` and the duplicate skip workaround changed in the renderer;
seven destinations, active keys, leads, body, degraded evidence and local write policy
remain. The mirror was clean before scoped `sync-vendor.sh app-kit`; its old bytes
were backed up first. Agentic-kit's `3db571b…` mirror and pin stayed unchanged.

- Shared `RenderNav` supplies current links, 44px targets and explicit focus. Shell's
  native skip now targets its single `observatory-main[tabindex="-1"]`.
- Both workspace and `GOWORK=off` focused `TestObservatory|TestAppkit|TestExportAppkit`
  race suites pass. Both full mirror byte checks pass. UI input and tokens are
  byte-identical to the pre-handoff snapshot; generated CSS is 52,776 bytes,
  SHA-256 `578e81e549996163485beab9f2e025aa825a04bca814426673cd932052d94c79`.
- A separate working-tree source snapshot contains 3,436 files plus eight recorded
  symlinks (36,172,440 file bytes). With `GOWORK=off`, app-kit resolves inside that
  snapshot; focused race/vet checks and `go build ./...` pass. Two `-trimpath` CLI
  builds are byte-identical: 79,993,346 bytes,
  SHA-256 `52decdcb23c527f248ef873ac587a0528d616568ad0a7e308ca23c11d983cafb`.
- Real Chrome 152.0.7977.84 with isolated Playwright 1.63.0: **1,207/1,207 checks**,
  84 page samples (seven states × two themes × three widths × JS on/off), eight
  screenshots. Native skip/menu/tab traversal, exact current routes, click/back,
  content fragment/back, inertness and outer geometry pass. Across 588 focused nav
  samples: minimum height 44px, text contrast 6.8085:1, focus contrast 4.9279:1.
  No unexpected browser traffic, workers, downloads or runtime errors. Intentional
  server refusal probes are recorded separately; desktop dark and narrow light/open
  screenshots were visually inspected.

Reproduce using Fathom's opt-in `TestExportAppkitNavigationPreview` and app-kit's
`docs/assessments/fixtures/fathom-navigation/browser.mjs`. The exporter renders the
real shell around inert synthetic content for all seven active keys; it does not
invoke operational handlers, query a store/provider or exercise business workflows.
Both input and report directories must be distinct existing children of Fathom's
`workspace/tmp`; the report directory must be empty. The browser harness reuses the
already approved isolated Playwright installation, not an application dependency.

Retained evidence under Fathom `workspace/tmp/appkit-navigation.1DsKeZ/`:
`before.tar` (SHA-256 `4230d5da25af0ea72c35ed645c039ab6e4fa3d0349bf8d24c318f5d5ace9f55f`),
the pre-handoff diff/status, source snapshot and manifest
`3241d854e112157b18b0bdc4a43e9552ca95f7b45416644fb272b69577240ee2`, exported previews,
and `browser/report.json`
`cdb0c77116cb5b363962ac845babf545a0c0d1800b311cc56449dc08de4a8b0d`.
Browser harness hash: `53e0aedb2a8de232c1079fb78f6709f25c0baafb0fe9b33e1bfefbc865cbff2a`.
Unrelated tracked diffs match the pre-handoff capture. This ignored workspace is
recoverable local evidence, not a durable release artifact or a clean Git clone.

### Unresolved delivery gates, not hidden passes

`clean-clone-check.sh` copies tracked files only and fails on `NavItem/RenderNav/NavGroup`
until the newly added mirrored `nav.go` is included in the eventual reviewed consumer
commit. Nothing was blanket-staged to hide this. The source-snapshot test initially
failed because its opt-in preview helper expects `workspace/tmp` to exist; creating
that documented fixture directory made the unchanged suite pass. No test was relaxed.

The initial working-tree adoption checker passed all six artifact hashes, profile,
capability, module declaration and both routers but fails **catalog revision**:
Fathom selects `0b528f5b…`, while this checker carries `2a98eb25…`. That prior selection
was preserved until the prospective review below. Static hashes
alone do not prove origins, effective module resolution or complete consumer adoption.
The shared service `fathom` currently indexes a scratch `pushclone`, not the consumer
checkout; local source/build evidence, not that index, supports this receipt. No daemon
re-root, global hook reconciliation, deployment, push, tag or real data was used.

### Prospective catalog re-selection

The exact old `0b528f5b…` catalog/checker could not be recovered; neither Git nor the
handoff backup supplies it. Do not infer old/new equality from newer intermediary
snapshots. The current `shell.layout` record was instead reviewed as a fresh selection:
`go-web`/L3b, typed app-owned navigation, native disclosure/skip, token/CSS requirements
and explicit backend/consumer-proof gaps match the bounded Fathom contract and actual
63b3a4f integration above. Historical catalog consumer names are not current adoption proof.

Private evidence: app-kit `data/design-assets/adoption-checker.DJ6MuD/` pins catalog
revision `2a98eb25cae9fc3ef6b670826fc608f11ad32222e055117253ebc518fd0dc43c`, source/schema,
normalized selection and two identical `-trimpath` checker binaries:
`52185ef494dcc0329f5eea84c52a1cf297038b035e80e35ea4adbac57088cc9d`.
All 1,248 non-standard source/embed inputs match before and after the build; their
manifest is `039f392f43cae2a4490cbd18b8c6f6341fc34ac7f61961a0b2222450eab8d749`.
Normalized `shell.layout` capture SHA-256:
`2cc9115e4434f9d56e514353316ede66c1e35e195db52989faf7cebb644c2c26`.

The pinned checker first reproduced the catalog-only failure. After changing only
Fathom's `catalog_revision`, all 16 checks pass; the remaining manifest fields compare
equal to the saved input. Checker/schema, six artifacts, profile/layers/capability,
module, routers and runtime code remain unchanged. Both repos are dirty; no clean
release, baseline-delta proof or full G4 completion follows. The global-revision
review friction fires ADR-0024's revisit trigger; this scoped explicit re-selection
does not introduce per-entry pins or relax stale-revision refusal.
Final strict report (after consumer handoff documentation): `check-final.json`,
SHA-256 `de32c2a2d0461d62b44a75219e2b34810279cad66052285405a821d22d358fe5`.
