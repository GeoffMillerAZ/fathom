# Gallery explicit CSS source closure

Base revision: `3e18dca`. Scope: Gallery build inputs and regressions, not a token,
renderer, consumer, dependency or runtime migration. This is an INC-9 recurrence:
consumer guidance required `source(none)` while Gallery's canonical input still
enabled implicit checkout discovery. An executed structural regression caught it.

## Change and controls

`apps/gallery/ui/input.css` now disables automatic discovery and keeps its explicit
renderer/token vocabulary. It also scans Gallery-owned `../*.js`: `shell_live.js`
creates utility-bearing activity entries, previously covered only incidentally by
the same utilities in Go. Imported motion and generated theme CSS are unchanged.
The regenerated CSS is **43,446B, down from 55,167B**: 11,721B (21.2%) less source
payload, not a measured latency, memory, compression or whole-app performance gain.

- `TestGalleryCSSDisablesImplicitSourceDiscovery` fails on the original import and
  passes with the guarded import and explicit Gallery JavaScript source.
- Opt-in `TestGalleryCSSClosureWithPinnedCompiler` verifies the existing platform
  compiler checksum before four compilations. An explicitly selected utility appears;
  adding an unselected HTML canary leaves guarded output byte-identical. The implicit
  control changes and admits the canary. This proves the minimal fixture's discovery
  boundary, not exhaustive coverage of every Gallery state or a compiler network sandbox.
- Test HTML is constructed from split class fragments so the fixture itself does not
  add canary utilities to shipped CSS. Initial literal and intermediate printf inputs,
  their outputs and executed test revisions remain retained, not overwritten.
- Full Gallery, catalog and MCP race suites pass; scoped vet passes. The pinned
  compiler integration passes separately with its opt-in enabled.
- Repeating the pinned minified build into a separate output file produces the same
  43,446B CSS. This is repeatability in this checkout, not an isolated consumer build.

## Paired browser observations and retained non-passes

Retained first-party HTML from 15 routes was compared under old/new CSS at1280/390px,
light/dark, in initial and programmatically expanded-details states: **120/120 measured
style/geometry pairs match; 107/120 PNGs are byte-identical**. The other13 PNG pairs
differ, so the exact-pixel acquisition remains failed. Both sides use the same HTML;
30 source documents are hash-bound. Application scripts are disabled. All admitted
requests are fulfilled from local retained bytes; no app server or signed-in profile.

A separate same-CSS control uses identical candidate CSS on both sides and the
unchanged driver: again120/120 measured-style/geometry matches and107/120 identical
PNGs, with13 differences at partly different cells. This demonstrates variability
without a CSS change; it does not prove the cause of each old/new difference or waive
the failed pixel oracle. No image-difference threshold or baseline was relaxed.
Both acquisitions completed60 cells, recorded zero blocked requests and no browser
close error. This is not OS network containment or descendant-process attestation.

Measured properties include layout boxes, typography, color, borders/radii, spacing,
overflow, outline, shadow, SVG fill/stroke and transform; not every computed property.
Visual review included wide-light components and narrow-dark dashboard. Existing
narrow provenance overflow is unchanged, not accepted as a new layout standard.
Keyboard tasks, JavaScript states, overlays, custom theme-guide flows, actual zoom,
assistive technology and consumer acceptance are outside this static comparison.
No inventory gate is promoted. The signed page's separate20KiB HTML failure remained
open at this acquisition; a later [navigation payload repair](2026-09-22-gallery-nav-payload.md)
passes that unchanged target without reinterpreting these pixel failures.

## Retention and reproduction

Private packet: `/Volumes/m2ext/appkit-evidence/appkit-css-closure-review.3oidAJ`.
It retains red source/test/log, intermediate and final CSS, `snapshot.go` plus its
compiled producer,30 HTML documents/manifest, `parity.mjs`, both browser packets and
test logs. The producer calls `gallery.New()` with private sources/guide unmounted.
Chrome153.0.8010.53 and installed Playwright1.62.1 were used; no install or upgrade.

| Artifact | SHA-256 |
|---|---|
| Old CSS | `60db385940a53cca76b8a0ef717be304cb2342bbb1eb23e63f59965a42a29987` |
| New CSS | `0abe503d969b308d005c7d6632386638fe75f92ea53ff4ba9d65770fe41b7a33` |
| Final input.css | `ddcf11bb547033f44d2e9e831c3e7b9cc2d4e3d4f96ef1b9dee4409635d753cc` |
| Final closure test | `3a8713223888dbe5394bda87e7564611bb4d41c7efe49d5cb5189c321c810e34` |
| Browser driver | `f3366c716789f5b7b7fb21efc743a5e55fc37767d7a6bdb131fd49c1170cf2a3` |
| HTML producer binary | `3fca38ddefd4fcae29ef3ee8ab50342711f9e17385a10cb2da2724192a6c3569` |
| Old/new browser report | `c4b50bc06ec16afc78386368b136109f08ae8e3dfa2f2ab9d5f8ebee9056bf46` |
| Same-CSS control report | `89c7829cd607c1027045bd5628504d553e24122730c3cc7cd897f09b43639b8a` |

The compiler is pinned Tailwindv4.3.0, darwin-arm64 SHA-256
`56b4bbc62dbdc4614a78930d9c6986423a2ec63e4e640144a59a5d95c914322e`.
To reproduce the structural check, run `go test ./apps/gallery -run
'^TestGalleryCSSDisablesImplicitSourceDiscovery$'`. For the compiler control also set
`APPKIT_CSS_CLOSURE=1`, `APPKIT_CSS_COMPILER` to an existing absolute pinned executable
and `APPKIT_CSS_EVIDENCE_ROOT` to an existing private directory, then select both tests
with `-run '^TestGalleryCSS(DisablesImplicitSourceDiscovery|ClosureWithPinnedCompiler)$'`.
The control retains a new private child directory; it neither resolves nor downloads
a compiler. Regenerate shipped CSS with `task gallery:css`; do not hand-edit it.

Browser replay uses retained driver arguments: Playwright entry, Chrome executable,
HTML root, old CSS, new CSS, new output directory. Check tool and input identities
first; missing artifacts cannot support a fresh pass. Treat whole-catalog quality,
consumer adoption and pixel-oracle investigation as separate demand-driven work.
