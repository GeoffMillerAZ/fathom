# P01 retained-source performance experiment

This opt-in local benchmark measures the unchanged enhanced P01 specimen. It does
not certify the converter, the complete component, portal fidelity or consumer use.
The prospective contract is `performance-contract.json`; budgets, dimensions,
throttling and applicability must be fixed before running. Never relax a failed
budget or reinterpret a harness failure as an acceptable product result.

## Run

Use the already-authorized isolated Playwright installation and installed Chrome,
never a signed-in profile. Three positional paths must be absolute: retained hero
probe, its exact prior browser receipt, and an existing private output parent.
The source/receipt/tool version pins deliberately refuse other inputs.

```sh
APPKIT_PLAYWRIGHT_MODULE=/absolute/approved/playwright/index.mjs \
node docs/assessments/fixtures/hero-adapter/performance.mjs \
  /absolute/private-hero-probe \
  /absolute/private-hero-probe/browser-NAME/receipt.json \
  /absolute/private-output-parent
```

No downloads, dependency installation, source repair or application mutations occur.
Each invocation creates a new private `hero-performance-*` directory, snapshots the
runner/oracle/tests/contract first, runs the retained tests and executes the retained
runner. Licensed markup and assets are copied only into that private evidence root.
The browser uses a fresh temporary profile and an owned loopback server. Do not run
`performance-run.mjs` directly. Keep failed directories intact before making repairs.

The ordinary run requires all16 observations: two browser launches,390/1440px,
light/dark and cold/warm. Every observation must pass, not merely an average. Warm
means a fresh navigation with uncached HTML and observed HTTP-cache hits, never
back/forward cache. Bodies are byte-checked, including cached responses; a missing
response body is an evidence failure, not a reason to substitute stored-file bytes.

The opt-in [cached-font diagnostic](../../2026-09-16-cached-font-protocol.md) probes
Chrome recording alternatives separately. Its exit0 means diagnostic completion,
not warm-body verification or a performance pass; it never changes this oracle.

[ADR-0067's prospective v2](CACHE-LINEAGE.md) is a separate explicitly derived
cache-lineage experiment. It leaves this v1 contract, oracle and failures intact;
its checker alone supplies neither a full performance pass nor target certification.

Append one of `missing-font`, `delayed-css`, or `oversize-image` for a single-cell
negative control. Each must detect its exact fault, with complete surrounding
resource and navigation evidence. An unrelated payload overrun or an incomplete
16-cell matrix cannot prove detection. Canary `passed` is always false: exit0 means
the injected fault was detected and cleanup completed, not that the UI passed.

### Separate font-delivery candidate

After `font-proof.mjs` has earned its generation, shaping and paired-browser gates,
append `--font-candidate ABS_FONT_PROOF_RECEIPT EXPECTED_SHA256` instead of a canary.
This admits only that explicit retained receipt and its font/CSS identities. The
original source/reference pins and all performance budgets remain unchanged; the
report labels the candidate separately. Only the declared font faces and subset
delivery change. The original font remains available for fallback, but loading it
for the fixed Latin hero is a failure, not a silently accepted extra request.

The launcher now snapshots nine files, including font-delivery/paint helpers/tests,
and runs all21 Node checks before either profile. Historical five-file runs retain
their own executed snapshots. A passing font proof cannot substitute for the full
16-cell performance matrix or repair a failed warm response-body identity check.

Candidate readiness waits for `document.fonts.ready` and requires a settled font
set plus custom Inter glyphs at every visible text-parent through CDP. Matching
declared-face `document.fonts.check()` remains diagnostic: the unrestricted unused
fallback can make it false. Exact request bodies, subset-only hero selection and
the fixed numerical gates remain mandatory; baseline readiness stays unchanged.

## Receipts and limits

`snapshot.json` records the exact executed source graph. `launcher-receipt.json`
binds retained test/run logs, command exits and source consistency. `receipt.json`
contains browser/host identity, request graphs, navigation/cache facts, raw metrics,
per-cell failures, screenshot hashes, canary findings and owned process exits.
Chrome's executable is hashed before/after; Playwright's selected entry is pinned,
not its whole transitive installation. Node and OS versions are recorded.

The runner has bounded browser/server cleanup and a120-second watchdog; the launcher
requests shutdown at150seconds, allowing25seconds for cleanup before SIGKILL.
Forced shutdown is never a cleanup pass. An outer
forced stop leaves descendant cleanup unverified and requires explicit inspection.
No existing user browser or preview is stopped.

FCP/LCP/readiness are controlled lab observations. Readiness is an upper-bound
measurement after load/fonts/images/two frames, not TTI. Payload excludes headers,
TLS and cached bodies; heap excludes native/GPU/image memory. There is no application
JavaScript. The font is a controlled reference, not established portal provenance.
Screenshots need visual inspection; their mere existence proves no accessibility.

Instrument callbacks require browser JavaScript enabled; CSP `script-src 'none'` and
zero script elements still prohibit application scripts. An empty inline data favicon
prevents implicit favicon traffic. The [first measurements and retained harness failures](../../2026-09-16-hero-performance.md)
show why these conditions are explicit; this is not script-disabled task proof.
