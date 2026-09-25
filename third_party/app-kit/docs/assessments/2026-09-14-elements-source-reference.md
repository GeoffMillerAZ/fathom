# Elements source-native interaction reference — 2026-09-14

## Result and boundary

The four frozen Elements cases now have a privately retained, reproducible runtime
reference: P03 FAQ, P07 menu, P08 nested drawer/menu and P09 mobile navigation.
**404/404 scoped Chrome assertions pass**, including a runtime-withheld negative
control. This earns a source-native interaction baseline, not a conversion, complete
visual baseline, accessibility certification or real consumer integration.
The existing translator still refuses these four cases. The 439/657 syntax/compiler/DOM
count, five saved syntax attempts, four typed candidates and zero certified targets
are unchanged. [ADR-0023](../adr/0023-profile-specific-conversion-and-client-compute.md)
already admits this runtime-preserving family; no default profile/API was broadened.

## Inputs and private replay

Exact archive/member identities and SHA-256 values remain those in the
[twelve-case manifest](2026-09-13-conversion-pilot.md). No paid source or source class
was changed. The authored [fixture](fixtures/elements-reference/README.md) records
the conditions and assertions separately from its generated private output.

The official [Elements documentation](https://tailwindcss.com/plus/ui-blocks/documentation/elements)
identifies the vendor interaction runtime. This run resolves and pins the
[1.0.22 npm tarball](https://registry.npmjs.org/@tailwindplus/elements/-/elements-1.0.22.tgz),
not the mutable major-only URL in the downloaded HTML comments. Its ESM runtime,
commercial license and exact tarball remain private; no public package dependency,
signed-in browser access or consumer installation was added. This is not a new
redistribution-rights determination.

`prepare.go` reads the exact archives and bounded verified tarball without network
access or vendor-script execution. It preserves four original fragments, supplies
eight light/dark documents, pinned InterVariable plus license, and CSS built twice
from only these sources using cached checksum-pinned Tailwind 4.3.0. Those font/CSS
choices are explicit comparison conditions, not recovered portal settings.
Publishing requires a real 0700 parent and retains identity-checked directory handles
for staging and destination. It reserves a new 0700 directory, writes exclusive 0600
files and publishes the manifest last; an existing destination is refused. A planted
directory-to-symlink replacement is detected before any writes reach the sentinel or
displaced directory. This confines writes and detects replacement at checked boundaries,
not a claim of protection against malicious same-UID processes. Failure may leave an
incomplete private directory, not an allegedly atomic successful bundle.

Retained output, relative to app-kit:
`data/design-assets/reference-assets/elements-1.0.22-pilot`.
It is Git-ignored, not a Git-backed backup. An independent offline replay using its
retained tarball at `/tmp/appkit-elements-reference.UJ123j/retained-replay` matches
all **18 declared files plus manifest byte-for-byte**.
After publication hardening, `/tmp/appkit-elements-reference.UJ123j/reviewed-replay`
matches those same 19 files using the final preparer. The existing retained bundle
was not overwritten; no parent-directory permissions were changed.

| Artifact | SHA-256 |
|---|---|
| Elements tarball | `fc684f17d960fb60dadfdf9a0b49ba8fd295af4b8fd1715a26cd20b832ea245b` |
| Executed ESM runtime | `120ff5b81c8b3bb7e6de13d8b83a92da984c4312aa017b7392c9cb876cd65013` |
| Prepared manifest | `2fb9ae848b8040f8aee05f74a741253592a53a331201baca299dd813dc121f48` |
| Browser report | `298345872a60f91d6304e020d1f39535e797d63fb6b846f16bdda5875caee8f1` |
| Browser harness | `79ab528135ce53da0245ff04fc3d3d3702968a92c2d55ec4de29ae0138b18872` |
| Browser input/request contract | `ec21562301b9a9036adcb8f4c93e55f047f230cd4c0a43475df44236d8407c7d` |
| Preparer | `0badc8cdf6217f2d57a3d077ddd06522e774b39d315002fbb7eb57fd1cd22091` |
| Preparer tests | `1cd208719d2899d16b2dad1358d6ffe31cf6efa10f7b7edf7ca3b11aba71a5ca` |

## Browser evidence

Report and 34 hashed screenshots are retained beneath `browser-LuQIpn/` in that
private output. Playwright 1.63.0 drives installed Chrome 152.0.7977.84 with fresh
isolated contexts, 390/1440 CSS-pixel widths, 1000px height, light/dark themes,
ready fonts and reduced-motion preference. Original transitions are allowed to finish;
the harness does not rewrite their styles to make assertions pass.

| Family | Exercised contract |
|---|---|
| P03 FAQ | Six initial panel states; Tab entry; Enter/Space/pointer toggle; sibling and focus preservation; three remounts |
| P07 menu | Keyboard/pointer open; focus entry and arrow movement; Escape return; outside dismissal; three remounts |
| P08 nested menu | Native dialog entry; nested menu; first Escape closes menu only; second closes dialog and returns focus; three dialog remounts |
| P09 navigation | Narrow/wide visibility; mobile Enter/Space/pointer toggle; resize; account-menu movement/Escape; three menu remounts |

P03 uses `display:contents`: the visible-content assertion checks rendered descendants,
not the boxless host. The same disclosure assertion rejects the runtime-withheld
control. Repeated remount behavior is not proof of absence of heap leaks or duplicate
listeners under every lifecycle, and no Datastar morph was exercised.

Only verified artifact bytes are served through a random-path, GET-only loopback
server with exact Host/Origin checks. The complete independently replayed manifest
is pinned before artifact loading: changing a document, CSS, font, license or tarball
and rehashing the editable manifest cannot redefine the baseline. Denied same-origin
requests fail even when the browser also emits a generic blocked-resource message.
CSP permits same-origin scripts but no inline
script/eval, denies connections and all form actions, and allows inline CSS needed
by runtime positioning. There are 68 allowed local requests and no offsite routed
requests. Actual `securitypolicyviolation` events demonstrate refusal of the declared
P08/P09 remote images before they reach the network. No other runtime/policy errors
were observed; retained console image refusals are not discarded as unexplained noise.

Local payload: runtime 84,655 bytes (gzip 24,517; Brotli 21,857), CSS 28,892 bytes,
font 352,240 bytes, and documents 2,554–9,511 bytes. Compression sizes are calculated,
not a claim of compressed HTTP delivery. Across 196 trusted-input samples, second-RAF
p95 was 32.6ms and maximum 45.6ms. This is a local instrumentation observation, not
input-to-paint, production performance, a regression budget or conversion speedup.

## Preserved gaps and next gate

All 34 sampled states record original controls below the kit's 44px target goal. These
are design findings, not automatically WCAG failures and not silently repaired source.
Visual inspection of FAQ, menu and nested drawer captures retains the purchased
hierarchy; the drawer's blocked image is visibly missing. P08/P09 image closure and
complete visual baselines remain open. Placeholder links and submit items have no
app-owned action; all submission is refused. Touch, true zoom, screen readers,
cross-browser/native hosts, complete focus coverage and real tasks remain unproven.
Geometry now includes open overlays, not just closed post-remount states. Eighteen
keyboard-state screenshots and computed focus observations are retained. The simple
outline screen is inconclusive for 14 states; that is not a claim of invisible focus.
Sampled visual review shows the FAQ's outline and the nested menu's highlighted item.
Full indicator contrast and focus coverage still require separate verification.

The preliminary `browser-b6GLRq` report retains its 336 passing assertions. Review
then found unsealed render inputs, uncounted blocked local requests and missing
open-state/focus observations. The final report above replaces it as current evidence;
the original vendor bytes, runtime and assertion meanings were not weakened.

A read-only scout found identical runtime bytes already vendored in Throughline's
`surfaces/web/static/tailwindplus-elements.js`, loaded by `shell.templ`, with menu
uses in `cases.templ`. This is a reuse lead, not new runtime/adoption evidence; no
Throughline file, dependency or instruction was changed.

Next: use this baseline for a **separately versioned, opt-in Elements translation
rule**, preserving an explicit runtime dependency and testing original/compiled DOM
and interactions together. Keep default refusals and historical saved-attempt replay
valid; do not widen the old allowlist or relabel its receipts. Resolve source imagery
and test Datastar lifecycle before claiming those gates. Usability adaptations must
be explicit derivatives with original-linked visual/behavior comparisons.

## Verification

The explicit live command passes with actual private sources, cached compiler and
pinned font:

```sh
APPKIT_ELEMENTS_LIVE_ROOT=/absolute/app-kit GOPROXY=off \
  go test -race -count=1 ./docs/assessments/fixtures/elements-reference
```

Default fixture tests deliberately skip that live
path and exercise synthetic safety/replay cases; a skipped test is not live proof.
`GOPROXY=off go test ./...`, focused `go vet` and Node syntax checks pass. Nested
native modules, live production data and consumer acceptance are separate.
Uncached MCP tests pass after the document changes. The intent-doctor subset checks
15 documents, 191 relative links, 39 ADR shapes and line budgets, with no failures;
it does not prove agent-host loading, hook wiring or architectural truth.
Three Node contract tests pass with `APPKIT_ELEMENTS_REFERENCE` explicitly set to
the private output, including five in-memory artifact/manifest rehash canaries and
same-origin missing-image/stylesheet refusals. Default synthetic execution skips the
private manifest test. Independent re-review cleared the three browser findings.
The final live/race test and actual replay also pass after the publication fix;
independent re-review found no remaining issue within its stated trust boundary.
