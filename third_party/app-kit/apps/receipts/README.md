# Receipts: a runnable analytical reference

An experimental first-party task under [root intent](../../docs/intent/README.md),
with synthetic historical facts and no live data or account backend. This exercises
the [P11 task contract](../../docs/design/matdash-analytics-adapter.md); it is **not**
the original MatDash Dashboard2, a fourth converted candidate, or a certified target.
[ADR-0038](../../docs/adr/0038-runnable-receipts-reference.md) records the boundary.

## Inspect it

From the app-kit root:

```sh
task receipts:css
task receipts:run
```

Open [the local report](http://127.0.0.1:8766). The command accepts a literal loopback
`-addr`; `go run ./apps/receipts/cmd/receipts -addr 127.0.0.1:0` chooses a free port
and prints the actual address. SIGINT/SIGTERM drains and closes the server.
CSS uses the kit's pinned Tailwind executable; application builds need no Node.

Choose period, channel, comparison and fixture age, then Apply. Inspect a product
through its chart or table link, use Back to restore the overview, and download
the exact applied selection. Theme links preserve selection and detail identity.
Changing filters intentionally returns to the overview. Only light/dark modes of
the default token theme are exercised here, not every named identity.

September starts at $1,550.00 net versus August $2,300.00. Atlas contributes -$600.00
of the -$750.00 change. Cedar's -$150.00 is drawn left of zero, not clamped.
October distinguishes known zero from missing inputs. November shows its actual
partial interval, never a fabricated monthly total. Fixture freshness is evaluated
at the fixed 2024-12-05 clock; it is not a claim that these records are fresh today.

## Reuse the pattern, not the example's business model

- `analytics.Dataset` and `WriteCSV` carry declared units, periods, missingness,
  source, freshness and uncertainty. Raw receipts/refunds remain app-owned.
- `dashboard.RenderShell`/`RenderNav` provide escaped native navigation, the mobile
  disclosure and skip-target focus. `vizframe` supplies provenance framing.
- The paired signed SVG stays private to this task, with a common scale, distinct
  current/previous marks, visible values and native detail targets. The shared
  nonnegative bar renderer still refuses negative values.
- Native GET forms, links, details and CSV downloads work without client JavaScript.
  The loopback handler has a closed route/query surface and no providers, stores,
  mutations, sockets, external assets or production deployment configuration.

`fixture.cue`, model/export code and their independent numerical tests moved here
from the [assessment fixture](../../docs/assessments/fixtures/matdash-analytics/README.md).
Source disclosure now names the actual file. There is no public financial model API,
generic chart engine, new runtime dependency or automatic consumer migration.

## Verify

```sh
go tool cue vet -c apps/receipts/fixture.cue
go test -race ./apps/receipts/... ./internal/analyticscsv ./apps/gallery
go vet ./apps/receipts/...
go test ./apps/receipts -run '^TestReceiptPagePayloadAndScriptBudget$' -v
go test ./apps/receipts -run '^$' -bench '^BenchmarkReceiptsHTTP$' -benchmem -count=3
```

Tests retain literal receipt/refund/net oracles across 72 projection selections,
signed/zero/missing geometry canaries, forged-projection refusal, strict HTTP/query
contracts, source immutability, detail/export agreement and concurrent access.
The application has no client scripts. Declared three-product limits are HTML
≤32 KiB, local handler ≤5 ms/op and ≤512 KiB allocated/op. Benchmarks include
request/response allocation but exclude one-time CUE loading, process startup and
network transport. Passing these limits does not establish production performance.

`browser.mjs` uses the separately approved isolated Playwright 1.63.0 harness and
installed Chrome with a fresh temporary profile, never a signed-in profile. Set
`RECEIPTS_BASE_URL` to the loopback origin and `RECEIPTS_EVIDENCE_DIR` to an existing
empty evidence directory; then run `node apps/receipts/browser.mjs`. It refuses
outside requests and mutations, records actual downloads and preserves a JSON receipt.
Browser execution is a separate gate, not implied by Go tests or harness existence.

Observed 2026-09-14: [144 browser checks, local budgets and reproducible build evidence](../../docs/assessments/2026-09-14-receipts-reference.md)
pass within the documented scope. The receipt keeps original MatDash fidelity,
synthetic task behavior and consumer adoption separate.

[2026-09-15 CSV migration](../../docs/assessments/2026-09-15-analytics-role-meaning.md#receipts-executable-migration)
adds `role`/`role_label` to the35-column export; migrate exact readers with the kit pin.
The updated actual browser task passes144 checks, retaining the old schema failure.

Complete source fidelity, real consumer adoption, human task-time improvements,
screen-reader/touch hardware and native-host certification remain separate.
