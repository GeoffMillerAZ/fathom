# Elements original/compiled pair

Private P03/P07/P08/P09 evidence fixture under ADR-0023 and I13/I17. It prepares
one deterministic original/compiled artifact set from the frozen source-reference
manifest and the opt-in `TranslateElements` rule. Every receipt remains
`not-verified`; this fixture does not certify accessibility, target compatibility,
Datastar lifecycle, redistribution, or consumer adoption.

## Prepare

All three inputs are explicit absolute paths. The source reference must have manifest
SHA-256 `2fb9ae848b8040f8aee05f74a741253592a53a331201baca299dd813dc121f48`.
The output parent must already be a real `0700` directory. Preparation chooses a
random new `0700` child, writes every file exclusively as `0600`, and publishes its
manifest last. An interrupted run can leave an incomplete private directory; an
existing path is never replaced. Retained handles and identity checks detect
replacement at checked boundaries, not a malicious same-UID process.

```sh
GOPROXY=off GOWORK=off go run ./docs/assessments/fixtures/elements-conversion \
  -kit-root /absolute/app-kit \
  -reference /absolute/app-kit/data/design-assets/reference-assets/elements-1.0.22-pilot \
  -out-parent /private/existing-0700-parent
```

Preparation rechecks every declared source-reference file, all four archive members,
the Elements 1.0.22 runtime/tarball pins, translation replay, and the pinned templ
tool. Each emitted component is generated, compiled, and rendered before and after
`go tool templ fmt` with `GOPROXY=off`, `GOWORK=off`, local toolchain selection, and
read-only modules. Namespace, element/text/comment nodes, attribute order and values,
class whitespace, and decoded text must match the original fragment exactly.
Compiler output is confined to private scratch and is never printed. Standard output
contains only the new directory and metadata hashes/counts.

The pair retains the sealed source manifest and its declared CSS, font, licenses,
Elements module/tarball, source fragments, and eight source documents. Per case it
adds a JSON receipt, exact emitted/formatted templ, before/after-format rendered HTML,
and light/dark compiled wrappers made by replacing the original fragment only.

## Paired browser gate

Freeze and review the new `manifest_sha256` from preparation, then pass it explicitly:

```sh
APPKIT_PLAYWRIGHT_MODULE=/private/pinned/playwright/index.mjs \
  node docs/assessments/fixtures/elements-conversion/browser.mjs \
  /private/elements-conversion-RANDOM EXPECTED_PAIR_MANIFEST_SHA256
```

The harness refuses to read or serve artifacts until both the explicit pair hash and
the frozen source-manifest hash pass. It independently checks source links, distinct
compiled links and exact lane-specific wrapper substitution; rehashed alias manifests
are regression refusals. It uses installed Chrome with the same local
runtime, CSS, font, 390/1440 viewports, light/dark themes, reduced motion, CSP,
GET-only random-path loopback, Host/Origin checks, blocked forms, and declared missing
external images for both lanes. It runs the same keyboard, pointer, nested-Escape,
resize, focus, open-geometry, and three-remount tasks. Compiled remounts reinsert the
compiled fragment. Every named screenshot pair must be byte-identical with zero
tolerance and no masks; a difference stays failed rather than replacing a baseline.
Runtime-withheld disclosure canaries must fail in both lanes.

Default synthetic checks exercise exact DOM sensitivity, replay/runtime tampering,
wrapper cardinality, private publication/replacement refusal, manifest pinning, and
request admission. The actual private preparation and browser run are explicit; a
missing private input is not reported as a pass.
