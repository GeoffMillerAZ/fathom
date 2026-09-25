# Static exporter cache-cold check

This dated, local macOS replay tests a narrower question than browser or consumer
acceptance: can the fixed static exporter build from a reviewed private module bundle,
without either the fleet checkout or shared Go caches, and produce identical output
in two independently cold directories?

The packet uses the maintained `tools/releasecandidate` contract (ADR-0045), the
existing exporter (ADR-0071), Go 1.26.3 darwin/arm64 with CGO disabled, and the pinned
Tailwind 4.3.0 compiler. It adds no public generator, installer or network dependency.
Source archives, public dependency archives, logs and build products remain private.

**Status: bounded local cold replay passed.** The [first attempt](../../2026-09-22-static-cold-delivery.md)
failed inventory-order preflight before any child build or sandbox probe; the
[separate corrected pair](../../2026-09-22-static-cold-replay.md) now passes with
identical binaries, CSS and eleven site files. Its output check includes the retained
false disclosure-count oracle and one source-reviewed correction. No consumer or
independent-host acceptance is implied.
`prepare-closure-v2.mjs`, `cold-runner-v5.mjs`, `launch-cold.sh` and `contract.md`
preserve exact historical bytes, including the known ordering defect. They are dated
packet sources, not a ready-to-run generic CLI. They deliberately name the private
packet and frozen identities. Do not edit them into a passing historical attempt.
The launcher independently pins the entire manifest before the runner consumes it.
Use the assessment's exact next-task controls for a separately reviewed revision.

`inventory.mjs` is the pure assessment comparator; `node --test inventory_test.mjs`
executes eleven order/identity/refusal controls. Freeze inputs before acquisition.
`replay-runner.mjs`, `replay-launch.sh`, `replay-freeze.mjs`, `replay-contract.md`
and `verify-pair.mjs` retain the successful packet's exact sources under descriptive
names. They still name the original private filenames/roots and are not generic CLI
commands. The two committed reports bind exact results; private run receipts/logs
remain in the assessment's packet. Preserve both packets when preparing fresh paths.

## Independent output check

`verify-sites.mjs` takes six absolute paths, in order:

```text
node verify-sites.mjs LEFT_SITE RIGHT_SITE NEW_REPORT CONTRACT_JSON HISTORICAL_RECEIPT PROBE_MJS
```

The helper, contract and historical preparation receipt must match their embedded
SHA-256 pins. Use the retained packet inputs, not an arbitrary current browser helper.
The checker never launches a browser. It requires exactly eleven regular bounded
files, matching output bytes, valid sorted manifests, native reading/FAQ content,
selected CSS literals and script-denying CSP. Three detached manifest poison controls
must fail. Its report is exclusively created; existing reports are never overwritten.

The historical comparison is informational: changed historical bytes do not invalidate
two identical current builds, but they also cannot inherit historical presentation
evidence. Literal CSS and HTML checks are not computed-style, general sanitization,
accessibility or browser-task certification. The CSS input remains trusted.

## Resumption boundary

Review a new source/dependency closure before another acquisition. Preserve exact
executed inputs and failures; use fresh private destinations. Never repoint the proxy
to a live module cache or silently enable network when a dependency is missing.
The bounded filesystem denylist is not a complete OS read allowlist, independent-host
proof, malicious-build containment or a CGO-enabled/cross-platform result.
Keep consumer publishing policy, real hosting and remaining quality gates separate.
