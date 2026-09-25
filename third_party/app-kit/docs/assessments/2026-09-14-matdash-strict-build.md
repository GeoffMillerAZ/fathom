# MatDash strict-build candidate: bounded follow-up

This follows the [hardened reference](2026-09-14-matdash-hardened-reference.md).
The historical auto-build mismatch and its failing replay remain authoritative for
that revision. No original, dependency, source manifest or tested-build pin was replaced.
This is a private candidate, not a promoted port, consumer integration or runtime package.

## Build experiment

The minimal Axios/mock-adapter experiment did not reproduce the nondeterminism. Two
full builds from the same candidate root, with only the external Vite override
`build.commonjsOptions.strictRequires=true`, matched exactly: 383 output files,
15,406,919 bytes and 9,486 transform-hook calls. Source, configuration files and
selected dependencies stayed unchanged. Transform calls are not distinct module counts.

The strict tree SHA-256 is
`bbab4848eafdd40118279e82636e1ede2c025a90ee5652b6ee5214f25a9c3f3c`.
Its size is 17,756 bytes above the historical browser-tested auto build. Two matching
same-root builds identify a useful candidate setting, not cause isolation or a
cross-root/clean-install determinism guarantee. The original large-bundle warning remains.

## Unchanged behavioral oracle

A separate external shadow copied the strict distribution and 24 selected source pins.
Build metadata retained all 133 routes and source identities; only output asset
identities/total bytes changed. The shadow admission module changed only its build
metadata hash. Browser/editor probes, shared delivery server and source-closure module
were byte-identical to the existing reviewed versions. No behavioral expectation was relaxed.

One fresh-profile Chrome run passed **112/112 assertions**, with zero observed page
errors, console warnings/errors or blocked/outside/write requests. All 174 observed
responses returned 200 and matched the new pins. Twenty PNGs were retained and hashed.
The selected P11/P12 chart/editor task matrix is unchanged; see the predecessor for
exact actions, input refusal and target/focus checks. Owned browser/server processes closed.

Eighteen of 20 PNGs matched the prior run byte-for-byte. Root inspected both differing
pairs: `p11-1440-light.png` showed no substantive visible change at full-image inspection;
`p12-390-dark-image.png` showed a vertical viewport offset, with consistent visible
content and controls. The existing editor probe scrolls the editor into view before
capture, but records no scroll position. Its role in the difference is an inference,
not an established cause. This is **not** full pixel or semantic equivalence proof.

## Evidence and recovery

| Identity | SHA-256 |
|---|---|
| Strict build metadata | `bc8fbc6b4ab59ef404b8de7a4bfb7fd6312c22f776a720e43de925b1dfe0974b` |
| Browser receipt | `08f154142d95a7e3b40c315dac5ee36ec38e968632b0d81bfc66ab992e5e7b57` |
| Private packet manifest | `7c9099b7e50cce6a0d450d2735ed18b0b4a87b0b2214b34f21a8a58bd2763775` |

Packet: `data/design-assets/matdash-strict-browser.1ilLwb/manifest.json`.
All 47 payload files / 3,960,663 bytes were independently hash/size verified. The
packet retains receipts, 20 images, unchanged probes/server modules, shadow preparation,
comparison/launch scripts and strict-build configuration/logs. Full bundle bodies and
dependencies are omitted; external build/source identities are references, not a
self-contained source closure or clean dependency installation recipe.

## Remaining boundary

Do not silently promote this setting or rewrite the old red receipt. Next useful proof
is a separately admitted cross-root reconstruction and repeated strict builds with
the same source/dependency checks, followed by the same behavioral oracle. Investigate
capture geometry before treating screenshot differences as either defects or harmless.
Genuine zoom, screen readers, input latency, IME/clipboard, consumer data/authorization,
payload selection and clean installation remain separate requirements. Paid React
sources stay private; no kit core dependency or universal bundler policy follows.
