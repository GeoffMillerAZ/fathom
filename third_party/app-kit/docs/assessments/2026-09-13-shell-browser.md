# Gallery shell browser proof — 2026-09-13

G2/G6 follow-up: actual local Chrome tasks, not a production account or native host.
The [shell guide](../design/application-shell.md) retains the app-owned boundaries and
unfinished promotion requirements. Reproducible probes now live [in the repo](../../apps/gallery/browser/README.md);
they are opt-in, require an explicitly approved local origin/existing isolated Playwright,
and never install dependencies or use a signed-in profile. Three configuration tests
join `task check`; browser execution itself does not silently join the automatic gate.

## Observed defect and narrow repair

The first real second-tab profile save returned 403. Chrome emitted `Origin: null` for
the native form navigation under `Referrer-Policy: no-referrer`; the strict local guard
correctly refused it. HTTP tests with hand-authored Origin headers had missed this.
Initial declarative Datastar Connect and its first snapshot already worked.

Only the shell referrer policy changed to `same-origin`: same-origin form provenance is
retained, cross-origin referrers withheld. No guard, CSP, CSRF, permission, session or
version check was relaxed. `TestShellDemoSameOriginReferrerPolicyPreservesStrictOriginGuard`
asserts the profile/settings/activity header and continued null/cross-origin refusal.
[INC-16](../intent/incidents.md) records the escaped composition defect and durable outcome.

The unchanged final form probe reproduces the old header/403 failure before repair and
passes afterward. Preliminary harness assumptions were corrected separately: System
appearance removes `data-theme`; actual CSS/media resolves the theme. We tested computed
backgrounds and effective state rather than changing that existing implementation.
Review also added exact expected HTTP-error multisets and a no-unexpected-dialog assertion;
console failures are not broadly suppressed. No UI oracle or server guard was weakened.

## Browser results

Chrome **152.0.7977.84**, Playwright **1.63.0**, macOS arm64; headless installed Chrome,
fresh temporary profiles/contexts, service workers blocked, exact loopback route/asset
allowlists and WebSocket refusal. Dataset: fictional process-memory accounts only.

| Probe | Result | What was exercised |
|---|---|---|
| Shell forms | **125/125** | real Tab/Enter skip/edit/submit; native required validation; server 422; profile/settings two-tab 409 review; one-shot profile 503 retry; viewer/signed-out stale forms retain drafts; load/recovery; help/about; separate no-script profile/settings saves |
| Appearance/layout | included above | light/dark at 390/1440 CSS px; System media changes and computed colors; account version unchanged; one main and no page-width overflow |
| Datastar observer | **26/26** | served nonce/declarative action; no pre-Connect request; first/updated snapshots; separate draft unchanged; stop/no retry for 1.4 seconds; one manual reconnect; second-tab sign-out clears the original observer region |
| Request/error boundaries | included above | exactly two empty-body/query-free `GET /shell/events`, `Accept: text/event-stream`; no outside requests/WS/page errors; intentional protected-route errors match exact multisets |

The form run creates 11 synthetic screenshots; observer creates four. Reviewed selected
settings/validation/conflict and terminal-redaction images. Full-page stitching places
fixed bottom navigation at the viewport boundary within the long image; that image is
not evidence of document-flow clipping. No pixel-baseline certification is claimed.

## Exact local evidence

Before: `127.0.0.1:18779`, `/tmp/appkit-gallery-analytics.x2YnFR/gallery-after`, SHA-256
`824c350f227f9fb5b72b07ef5b361960b967b00d86276946b4a1fdf21d4e90a8`.
After: `127.0.0.1:18780`, `/tmp/appkit-gallery-analytics.x2YnFR/gallery-shell-after`, SHA-256
`322b0400da44969abe3e5bcea3c9a10c3a0d9df960fbe656b3507bfbfb4edbaf`.
CSS unchanged: `c93da8da135283c0b3fb653a669a5397142606247516145f67094c479592f8ea`.

Final evidence roots below share the prefix
`/var/folders/2r/2zrkv4fd549bnlc5vwfzd4b00000gn/T/` and may be removed by OS cleanup.
The checked-in probes allow new receipts; a missing old temporary file is not a pass.

| Artifact | Relative local path / SHA-256 |
|---|---|
| Final forms receipt | `appkit-shell-browser-XdM6fd/shell-forms-after-report.json` · `13f73a272aa4cf88182a88f7e5cd641030a90c0d766a6b9e73222f5bf864744e` |
| Final observer receipt | `appkit-shell-browser-F1KAnF/shell-observer-report.json` · `213029152fbbe439c9fac7313fb8da23f5b66c447c9d7d02e335b5507262a100` |
| Final old-build form replay | `appkit-shell-browser-iAEezU/shell-forms-before-report.json` · `5bfffcb35bf5079dcbd4cf6585a392af2989e92b1e3d43ef0eef871c6ea7f208`; rejected first native save prevents later task checks |
| Form probe source | `apps/gallery/browser/shell_forms.mjs` · `b74a4c55a2d7faaffb0ba86412e23005fba8399e8ccf392bc2e8a5a877880db6` |
| Observer probe source | `apps/gallery/browser/shell_observer.mjs` · `03f245b2628a61bc726885c3bc373d9ff0a1d5c5ac12e0984134d578597fedcd` |
| Shared preflight / tests | `config.mjs` · `8a338f305d0db449bb7a1aa923be679375782e9742f2a9600111520422d71d61`; `config_test.mjs` · `01cac456bf7eb7d53fa923401c465121c5908d5a5c597fa371c658e6193faea9` |
| Served handler / HTTP tests | `shell_demo.go` · `3e2b72b98535ad992b310665f3197b563eab7e223b1760b4cd83c13349e8116a`; `shell_demo_test.go` · `86f504a338c96defd4fd664245b37894aa0229ea71720d225d7029886f2d57e0` |

`go test -race ./apps/gallery -run TestShellDemo -count=1` passes (2.651s), as do all
49 Node tests and `go vet ./...`. One full Go attempt encountered filesystem `operation
not permitted` reads in Gallery/CLI/MCP; a repeat from `/Users/geoff/code/geoffmilleraz/app-kit`
passed the complete suite (Gallery 13.736s, MCP 6.803s). The failed attempt was not a pass
or evidence of a product assertion regression. No permission/global setting was changed.

Fresh isolated MCP stdio process: `get_kit_guidance` routes the shell guide, and both
`get_kit_decision("application-shell")` and `kit://application-shell` return the exact
10,757-byte document, SHA-256 `12c395eb5526e0147d31502b2453439a8d5285c7796b9c54f3ebb09efe337cb5`.
Replay source: `/tmp/appkit-mcp-analytics.89XPnv/shell-probe.go`; candidate binary SHA-256
`2516872da3dd372529b8ea4a6b4a3e758faca0ae4a1a3d069699948e02ab79ac`.
This verifies fresh-process delivery, not a refresh of installed host/daemon processes.

## What remains open

This is partial runtime proof, not G2/G6 completion or automatic promotion of the recipe.
Full keyboard order, touch targets, screen-reader announcements, contrast and genuine
200% browser zoom need dedicated checks. Server-error navigation currently resets focus
to BODY; linked validation errors focus the input, but that is not full error-focus UX.
Native-select values except the named keyboard input task were set through browser APIs.
No-script saving passed; no-script theme controls honestly remain unavailable.

Native interrupted-save/Back-cache behavior, observer hidden/show/expiry/network/capacity
and malformed-frame paths still need browser proof. No latency/human-task improvement,
real consumer replacement, release pin, macOS containment/packaging or Windows support
is inferred. Windows and CA/GGB remain deferred; Conjure stays a future distinct test case.
Keep the Catalyst-informed composition and return concrete consumer lessons to the kit.
