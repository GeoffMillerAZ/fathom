## Cost classes

Every tool declares a cost class (design `docs/design/responsible-tenant.md` §3.1,
RT-55); registering a tool without one fails. The admission gate admits each
`tools/call` in its class before the handler runs: per-class slots derived from the
budget, a per-call byte ceiling and a server deadline (`point` 10 s, `index` 30 s,
`tree` 60 s, `fleet` 120 s; `ingest` runs detached, and the `ingest` tool queues for
the daemon's one ingest slot — never beside another ingest or a summary backfill). A class is the tool's cost
**today** (`internal/mcp/testdata/toolcost.json`, held by
`TestCostClassMatchesMeasuredCost`); a tool moves to a cheaper class only in the commit
that rewrites it. `references` with `services: ["*"]`, or naming more than one service, is `fleet`. `batch` holds no
ticket: at most 16 calls, each admitted in its own class.

The class ceilings are measured. `internal/mcp/costtest` (RT-93..RT-95, design §4.8) calls
every registered tool cold and warm against a generated fleet at 1× and 10×. It checks
allocations against `point` 8 MiB, `index` 64 MiB, `tree` 256 MiB and `fleet` 512 MiB,
bytes read against 2 MiB for `point`, and growth for `point` and `index` at no more than
1.5× when the fleet grows 10×. A new tool needs a row in
`internal/mcp/costtest/testdata/costrows.json` (`TestEveryToolHasCostRow`).

Admission defaults to `observe` (`[tenancy].admission`, `docs/configuration.md`):
every would-refuse is logged and counted (`fathom_would_refuse_total`) and the call
runs. Under `enforce` a refused call answers HTTP 503 with `Retry-After` and JSON-RPC
error `-32050` (`message: "fathom busy: <reason>"`, `data: {reason, class, level,
retry_after_ms, in_flight}`); a call whose context ends (deadline, client gone, the
refuse rung) answers a named `abandoned` error, never an empty result; a call whose
producer the meter stopped at the per-call ceiling carries `budget: {partial: true,
elided, bytes_read, reason}` in its envelope (or its `batch` entry). A test binary reaching the managed daemon gets 409.

| Tool | Class |
|---|---|
| `blast_radius` | `tree` |
| `compile_layer` | `ingest` |
| `cross_repo_callers` | `fleet` |
| `find_callers` | `index` |
| `find_symbol` | `index` |
| `findings_near` | `index` |
| `ingest` | `ingest` |
| `list_services` | `fleet` |
| `localize` | `index` |
| `obligations` | `tree` |
| `outline` | `index` |
| `pack_context` | `index` |
| `query_symbols` | `index` |
| `read_source` | `point` |
| `references` | `tree` |
| `remove_service` | `ingest` |
| `rename_service` | `ingest` |
| `repo_map` | `index` |
| `review_diff` | `fleet` |
| `search` | `tree` |
| `staleness_report` | `tree` |
| `status` | `tree` |
| `symbol_context` | `tree` |
| `test_impact` | `index` |
| `verify_plan` | `index` |
