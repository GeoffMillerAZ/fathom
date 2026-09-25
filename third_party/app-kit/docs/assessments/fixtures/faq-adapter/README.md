# P03 private typed FAQ rehearsal

Contract: [FAQ adapter](../../../design/faq-adapter.md) and
[ADR-0041](../../../adr/0041-explicit-faq-reference-and-native-details.md).
This fixture produces private, source-bound evidence for one typed adapter. It does
not widen the Elements translator, publish a component, certify a target, or mutate a
consumer. Purchased text and generated derivatives never enter this tracked directory.

## Prepare

First prepare the unchanged four-case Elements source reference. Then choose an
existing real `0700` output parent and run from the app-kit checkout:

```sh
go test ./docs/assessments/fixtures/faq-adapter
node --test docs/assessments/fixtures/faq-adapter/contract_test.mjs
go run ./docs/assessments/fixtures/faq-adapter \
  -kit-root "$PWD" \
  -reference /absolute/private/elements-source-reference \
  -out-parent /absolute/existing/private-parent
```

The command verifies the frozen source-reference manifest SHA-256, all of that
manifest's files, the independent P03 archive member, and current `AdaptFAQ` replay.
It exports/checks the six-file delivery, then compiles raw and templ-formatted copies
in independent `example.invalid/faq-proof` modules using cached dependencies only.
`go tool templ` must match v0.3.1020; `GOPROXY=off`, `GOTOOLCHAIN=local`,
`GOFLAGS=-mod=readonly`, and `GOWORK=off` are enforced. Dependency output must contain
templ and contain neither app-kit imports nor the sibling checkout.

The first-party Go fixture is injected with the private source/defaults only in the
temporary module. It proves exact namespace-aware source/reference DOM, emits the
native two-instance/long-content fragment, checks cancellation, unique derived IDs,
and refuses Elements tags/commands in native output. The preparation copies only the
P03 source/documents plus pinned CSS/font/runtime inputs into a random exclusive private
directory. `manifest.json` is deterministic and written last as the completion marker.
The reported manifest SHA-256 is required by the browser command.

## Browser task

Main/owner execution only; this implementation step does not run live Chrome:

```sh
node docs/assessments/fixtures/faq-adapter/browser.mjs \
  /private/faq-adapter-DIRECTORY EXACT_MANIFEST_SHA256
```

The harness requires local Playwright 1.63.0 and installed Google Chrome. Source and
`NewFAQReference` use identical frozen wrappers, CSS, Inter font, and host-provided
Elements 1.0.22; their DOM and PNG bytes must match at 390/1440 in light/dark before
and after the same disclosure task. A runtime-withheld control must fail to toggle.

`NewFAQDetails` runs with document JavaScript disabled and must request no module. Two
different prefixes, all answers, independent opening, initial states, open/closed icon
mapping, keyboard/pointer use, visible focus, 44px targets, long wrapping, both themes/
widths, and actual browser 200% zoom are checked. Native CSS is absent from source and
reference pages. No masks, image tolerance, fallback constructor, Datastar, auth, HTTP
backend, persistence, or paid source body is added.

Temporary outputs and screenshots remain private. Source small-target findings stay
separate from native improvements. Screen-reader, touch, cross-browser, consumer,
native-host, cold-cache, hostile-same-user isolation, and target certification remain
unverified.

Current [assessment](../../../assessments/2026-09-14-faq-typed-adapter.md) retains an
intermittent 28-pixel SVG mismatch despite equal runtime DOM/geometry/styles and all
native task checks passing. A single all-pass run is not repeatable visual proof.
The byte-exact comparison remains a failure until its cause is resolved.
