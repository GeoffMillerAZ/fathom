# P12: actual Datastar morph with a client-owned editor

Scoped G1/G4 progress under [ADR-0023](../adr/0023-profile-specific-conversion-and-client-compute.md)
and the [prospective ownership contract](../design/client-editor-island.md).
The [base editor and large-document evidence](2026-09-16-client-editor-island.md)
remain separate. This is an opt-in private specimen, not a public adapter or certified target.

## What now works

Go serves one synthetic HTML fragment. The exact Gallery Datastar1.0.3 runtime performs
an actual `outer` morph of the surrounding section while a paired, keyed
`#editor-slot[data-ignore-morph]` retains the React/Tiptap-owned subtree. The browser
checks changed surrounding text, identical live slot/editor nodes, focus, selection,
document/revision and callback count; pre-morph formatting still supports undo/redo.
Continued typing makes no request. Keyboard activation of the host button also works.

The external first-party module calls the runtime's exported GET action using an empty
form, no query/body, zero retries, explicit cancellation and a five-second request cap.
It reports failure/cancellation without claiming a save. Separately intercepted503 and
held-response cancellation probes preserve the draft; concurrent invocation returns
busy. Explicit disposal and fresh remount still work after the morph.

No script CSP relaxation: `script-src 'self'`, no inline executable scripts or
`unsafe-eval`. This seam does **not** exercise Datastar's declarative expression compiler.
The runtime, license and proof module must appear together in the manifest; partial
sets and wrong runtime/license pins fail. Ordinary builds expose neither morph UI nor
the fragment route. All request origin/method/query guards remain in force.

## Measured evidence

Final private build: `data/design-assets/p12-client-island.UM4cXe/build-GL7iOz/`.

| Surface | Result | Boundary |
|---|---|---|
| Build | two identical outputs,20 files/1,073,483 bytes; protocol/typecheck pass | reused dependency tree, not clean-install proof |
| Cost |235,039 JS gzip bytes;32,545 CSS gzip bytes;24,836 first-view font bytes | same declared budgets; includes pinned Datastar/proof module |
| Host | seven test functions with refusal subcases pass under race; ten live-host probes pass | local synthetic host, no auth/save backend |
| Browser |218 task/global assertions,82 asset observations across ten paths; zero failed |1440/390px light/dark plus no-script |
| Morph subset |36 checks across four views, plus completion gate | actual success HTTP; failure/cancel responses explicitly injected |
| Scale subset |256 paragraphs/65,535 initial bytes,30 keys,40 total lifecycle cycles | separate scale task, not morphing a large draft |
| Scale latency |p95 two-frame proxy30.8–30.9ms, no input-window long task above50ms | automated desktop Chrome, not physical paint or mobile hardware |
| Captures |24 PNGs; narrow-light/desktop-dark morph views inspected | no original-paired pixel-fidelity claim |
| Cleanup | owned host exits0 on SIGTERM; no forced kill | no unrelated preview touched |

Scale explicit-GC diagnostics retain12 additional nodes, zero additional listeners and
812,904–819,056 additional heap bytes per view. Do not call this leak-free. The source
CSS remains unpruned and Vite/Iconify warnings remain visible. Runtime/device versions
are recorded in the browser config and the base assessment.

## Retained identities

- Output manifest: `648d2f0f415813a847464115f67d79c37dba5d2c2be5f98c52cca86ba34bf351`.
- Host source: `3298c14109ad47d2e832597b9e1037e79b242e73032a7185e806ec42f8336bb9`.
- Host binary: `81ca5934c848193b625008fc6c6d7f580db4677d999a48ed02173c56dcd56cfd`.
- `trial-sCefc1/run-receipt.json`: `708adc5c7838875800a6017146902d097a5bbda99f81f65538cb2246096e8f8c`.
- `browser-B46MD6/receipt.json`: `76e0c2702fc940789c5188c69b37a7c979013628b0459008747b3207209e71e6`.
- Actual139-byte morph body: `6dca9fb3c65ac3aaab3589b03713c945ac1689f26df50be929b63baaf11fe376`.
- Datastar33,538-byte bundle: `5d6b7794a50a83d82da962aec5e382f5ae83ac7afbc751f903f7a9c6bd433c65`.
- Adjacent1,052-byte license: `17da0e216d3c6a819526f5b72a6a44d84d34bcd9beb4191ac6af89c1508ef08f`.

The runner retains input bytes before launching. The browser verifies its own retained
script/helpers, manifest and startup against `harness-inputs.json`, writes those exact
bytes once, then imports the retained helpers. Browser config records their identities.
The four expected503 resource diagnostics remain in the receipt and consume one
allowance per fulfilled injected response; other console/runtime errors fail.
Independent `/Volumes/m2ext/appkit-evidence/appkit-p12-morph-checks.NKAS2e/` retains five seeded
helper/manifest/startup mismatch refusals before Chrome launch. Its exact extracted
cleanup-function probe passes graceful/forced owned-child exits and a never-close
branch with accelerated timers. This is function proof, not exceptional whole-runner
integration. The normal full trial above independently proves the success path.

`build-dr1RA7/trial-bHzPKJ` remains failed: a literal `[role=region]` locator missed the
preview's implicit named section. The repaired role-based locator passed in
`trial-C468A1/browser-IvdL0P`. Later input binding, build-time license verification,
console classification and exceptional cleanup hardening produced the final trial
above without changing editor/runtime/output bytes or task expectations.

Race source, command and log: `/Volumes/m2ext/appkit-evidence/appkit-p12-morph-host.eLNeyC/`.
[Replay](fixtures/client-editor/README.md) uses the optional `--datastar` build flag.

## Still open

The ignore marker is not a security or lifecycle boundary. Direct replacement/removal
and ancestor omission can destroy an island; intentional removal requires disposal.
This test does not prove those transitions, declarative CSP, SSE/reconnect, remote
hosting, large-draft morphing, persistence, arbitrary imported rich content, IME,
clipboard, assistive technology or real-consumer adoption. A separate [actual200% zoom follow-up](2026-09-16-client-editor-zoom.md) now covers its declared tasks. Full P12
fidelity/accessibility/consumer gates remain open; certified-target count remains zero.
