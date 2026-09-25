# P09 private proof fixtures

The [assessment](../../2026-09-15-navbar-typed-adapter.md) names exact retained results
and their limits. These scripts use private outputs, synthetic data and an isolated
browser profile. They do not download dependencies, access the signed-in portal,
modify consumer projects, certify targets or publish licensed source.

## Explicit runtime guard replay

`TestNavbarRuntimeLive` requires APPKIT_PROBE_CONVERSION_KIT_ROOT and optionally
APPKIT_PROBE_NAVBAR_RUNTIME_OUTPUT pointing to a new empty0700 child of command TMPDIR.
It writes a separate runtime-artifact.json, elements-navbar.js and runtime-receipt.json;
the original module, paired repair and base artifact must replay unchanged. Ordinary
`TestNavbarRuntime*` tests additionally exercise exact-site refusals and19 controlled
callback transitions. A skipped private or Node test is not proof.

```sh
node runtime-acceptance.mjs BASE_PROBE REPAIR_PROBE REPAIR_COMPILE_SHA256 SUPPLEMENTAL_ROOT SUPPLEMENTAL_RECEIPT_SHA256 RUNTIME_ROOT RUNTIME_RECEIPT_SHA256
```

Use the same absolute arguments/storage as the supplemental acceptance below, adding
the runtime output directory and its receipt digest. This harness requires the frozen
acceptance.mjs hash, derives a private runner with only runtime/provenance/output-name
substitutions, and retains that exact runner. All original task/quality assertions stay
byte-identical. Its report binds both harnesses, executed source, original/served runtime,
paired companion and rendered inputs. The normal-size run does not reuse the earlier
actual200% pass as evidence for this new derivative; broader target gates remain open.

`runtime-lifecycle.mjs` takes the same seven arguments. It adds real-runtime keyboard,
pointer and programmatic close/removal checks plus original-versus-derivative unselected
comparisons. Its explicitly synthetic scrolling fixture repeats short links and applies
the same bounded external CSS to both runtimes. This is scoped behavior/isolation proof,
not a source-fidelity baseline or a substitute for full target and consumer acceptance.

The derivative's own actual200% replay uses:

```sh
node runtime-zoom.mjs BASE_PROBE REPAIR_PROBE REPAIR_COMPILE_SHA256 RUNTIME_ROOT RUNTIME_RECEIPT_SHA256
```

Keep RUNTIME_ROOT below command-scoped TMPDIR; historical base/repair roots are read-only.
It binds the frozen repair task block and measurement functions, copies the unchanged
zoom helpers into a private runner directory, and records only the selected runtime's
new evidence. The original runtime's earlier result is not inherited.

## Runtime-bearing private delivery

`assets-adapt-navbar -repair -runtime ABSOLUTE_ORIGINAL_ESM` selects the separate
nine-file variant for JSON, `-out` or `-check`. The runtime input is the exact original
module, never the delivered derivative. See [the guide](../../../design/responsive-navbar-adapter.md).
`TestAssetsAdaptNavbarRepairLive` opts into the private checkout and optionally retains
both variants in APPKIT_PROBE_NAVBAR_DELIVERY_PARENT, an empty0700 child of command TMPDIR.
It writes delivery-receipt.json after replay, no-clobber, drift and cross-variant checks.

```sh
node repair-standalone.mjs ABSOLUTE_KIT REPAIR_DELIVERY DELIVERY_RECEIPT_SHA256 REPAIR_PROBE REPAIR_COMPILE_SHA256 GO_BINARY PUBLIC_MODULE_ZIP_CACHE APPKIT_BINARY
```

REPAIR_DELIVERY is the exported `repair` directory; its parent contains the receipt.
The harness creates two fresh consumer/cache roots, compiles both exported templates
with pinned public module fixtures, and compares typed/minimal HTML to the bound repair
oracle. Go/CSS/runtime delivery bytes and logs are retained with private modes. Actual
CLI checks run before/after; sandbox canaries deny sibling/source/oracle/network access,
and missing generated code must fail. Existing evidence is never overwritten.
This command proves synthetic rendering and byte retention, not browser CSS/runtime
integration, a complete dependency graph, real adoption or target certification.

The separately scoped browser command consumes that retained standalone result:

```sh
node repair-delivery-browser.mjs ABSOLUTE_KIT STANDALONE_ROOT STANDALONE_RECEIPT_SHA256 DELIVERY_RECEIPT_SHA256 REPAIR_PROBE REPAIR_COMPILE_SHA256
```

It builds CSS twice from private copies of both exported templates, serves only the
delivered repair CSS/runtime plus explicit local assets, and binds received response
bytes. Inputs, executed harness and scan copies remain private. Read the assessment
for failed setups, capture coverage corrections and exact acceptance limits; a script
or a nonblank PNG is not evidence that the full task was captured or a target certified.

## Base adapter proof

1. Create a private temporary root on suitable storage and an empty 0700 child directory.
2. Set command-scoped TMPDIR to that root and APPKIT_PROBE_NAVBAR_OUTPUT to its child.
   Set APPKIT_PROBE_CONVERSION_KIT_ROOT to the absolute private checkout; run
   `go test ./internal/designadapt -run '^TestAdaptNavbarLive$' -count=1 -v` with the
   existing external Go cache, GOPROXY=off, GOSUMDB=off and GOTOOLCHAIN=local.
3. Hash the resulting compile-receipt.json. The browser command is:

```sh
node browser.mjs ABSOLUTE_KIT ABSOLUTE_PROBE COMPILE_RECEIPT_SHA256
```

The browser probe requires the already-approved Playwright/Chrome paths and pinned
local Elements, Inter and Tailwind compiler used in the assessment. It verifies input
hashes, writes a new private browser-* child and retains failures as well as passes.
Original images remain blocked; typed images are explicitly synthetic. Three classifier
canaries are distinct from two actual DOM-mutation/refusal/restoration canaries.

Standalone rendering uses the same compile receipt and an explicitly built appkit CLI:

```sh
node standalone.mjs ABSOLUTE_KIT ABSOLUTE_PROBE COMPILE_RECEIPT_SHA256 GO_BINARY PUBLIC_MODULE_ZIP_CACHE APPKIT_BINARY
```

Keep the same command-scoped TMPDIR. The module ZIP cache must be an existing
`cache/download` directory. The harness reuses the pinned public-module fixtures from
pricing-adapter, derives only the module name, and creates two fresh consumer/cache roots.
Source/network-denied builds check the actual renderer against the typed HTML oracle.
It creates no global Go configuration and does not require a sibling kit at runtime.

All copied paid derivatives remain inside private temporary directories. No automatic
cleanup removes evidence; interrupted directories remain available for inspection.
The macOS sandbox proof covers rendering only, not CSS/assets/Elements or native Wails.

## Actual zoom, focus and runtime outage

```sh
node --test zoom-contract.test.mjs
node zoom.mjs ABSOLUTE_PROBE NORMAL_BROWSER_RECEIPT EXPECTED_RECEIPT_SHA256
```

Keep the same command-scoped TMPDIR. The diagnostic admits the exact normal P09
receipt and unchanged compiler/artifact/assets, then creates fresh private profiles.
Chrome's actual Appearance setting is 200%: outer 1440/780 by 1000 windows yield
720/390 by 500 CSS viewports. A separate local capture contract checks that ratio;
the frozen Gallery/drawer capture helper is unchanged.

It records conservative focus/contrast/containment failures and unavailable essential
destinations after an explicit module HTTP503. Expected503 responses are distinct
from operational errors. Exit1 and `failure.json` preserve failed quality gates;
they must not be relabeled successful fallback or target certification. Raw computed
paint, unfocused baselines, composited backdrops, screenshots and input hashes remain
available. Keyboard canaries distinguish permanent decoration from induced focus.
The assessment names results and limitations; source and existing delivery stay intact.

## Explicit repair candidate

Create a new private root and empty0700 child under its command-scoped TMPDIR. Set
APPKIT_PROBE_CONVERSION_KIT_ROOT to the private checkout and
APPKIT_PROBE_NAVBAR_REPAIR_OUTPUT to that empty child. Using the existing offline Go
configuration, run `go test ./internal/designadapt -run '^TestNavbarRepairLive$' -count=1 -v`.
The generated artifacts are an internal compiler fixture, not CLI-exported delivery.
Hash its compile-receipt.json, then run:

```sh
node repair.mjs ABSOLUTE_BASE_PROBE ABSOLUTE_REPAIR_PROBE REPAIR_COMPILE_SHA256
```

The base probe is read-only and may be outside this new temporary root. The repair
probe must be inside it. The script binds the exact38-failure baseline and checks
that the original focus/geometry measurement functions remain byte-identical.
The geometry locator selects the enhanced nav; explicit separate task checks exercise
native route access, its focus and real Elements arrival during an active native task.
All output goes to a new private browser-* child. Existing failures and sources remain.

## Supplemental bounds and instance fixtures

The repair compiler also emits `repair-maximum.html` and `repair-multiple.html`.
The first exercises twelve primary links, twelve account links, a notification link,
and maximum-length names, labels, keys, URLs, summary and image alt text. The second
contains two explicitly repaired navbars and one unchanged base instance. Constructor
tests reject one-over-limit inputs; browser acceptance is a separate operation.

To bind these fixtures to the frozen repair without overwriting it, use another empty
0700 output child and set APPKIT_PROBE_NAVBAR_REPAIR_BASELINE to the absolute original
`compile-receipt.json` identified in the assessment. Run the same private compiler
test. It requires that frozen receipt's exact hash, the same candidate artifact, and
byte-identical replay of all eight earlier renders before writing
`acceptance-fixtures.json`. New fixture hashes and the model-test source hash are
recorded separately; no existing receipt is rewritten or promoted.

The separate acceptance harness accepts read-only historical base/repair roots and
puts all new browser profiles and results under the supplemental root:

```sh
node acceptance.mjs BASE_PROBE REPAIR_PROBE REPAIR_COMPILE_SHA256 SUPPLEMENTAL_ROOT SUPPLEMENTAL_RECEIPT_SHA256
```

Use the command-scoped TMPDIR containing SUPPLEMENTAL_ROOT. It records normal-size,
maximum-data and repeated-instance checks, plus real JavaScript-disabled contexts
served the same module-bearing HTML. A failed assertion stays in `failure.json`;
the presence of this harness is not proof that its acceptance matrix passes.
