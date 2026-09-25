# P01 prospective isolated-cache-lineage performance contract

[ADR-0067](../../../adr/0067-prove-isolated-warm-cache-lineage.md) admits an opt-in v2
experiment. It does not amend [v1](PERFORMANCE.md), reinterpret failed receipts, or
certify P01. The pure refusal checker, graph-derived oracle and retained collector are
implemented. The [first full matrix](../../2026-09-16-hero-cache-lineage.md) failed the
header/event evidence gate despite passing numeric budgets; live controls are unperformed.

## Workload and applicability

Use the exact retained P01 font-subset candidate: same fragment, copy, images, CSS,
source references and rendered font proof. Freeze all input hashes independently of
the new observations. Two owned Chrome launches;390/1440×1000, light/dark, DPR1,
reduced motion, cold then warm in each of eight fresh isolated contexts. Preserve
10Mbps down/5Mbps up,80ms latency and4xCPU emulation. This is not a physical-device
or field-performance claim. No app, signed-in profile or global configuration changes.

Retain v1's per-observation budgets, without averaging:

| Metric | Cold | Warm |
|---|---:|---:|
| Uncached decoded response bytes | 524,288 | 16,384 |
| Browser-observed requests | exactly7 | exactly7 |
| FCP | 1,000ms | 600ms |
| Sampled LCP before input | 1,800ms | 1,000ms |
| Readiness upper-bound observation | 2,000ms | 1,200ms |
| Instrumented JS heap | 16,777,216 bytes | 16,777,216 bytes |

Keep native visible/Tab link order, three decoded/two visible images, no application
scripts, settled fonts and exact pinned resources. Reuse the owned CSP and source
pins. Add explicit identity-representation headers to this fixture only: strong
`"sha256-<body digest>"` ETag, exact MIME/Content-Length and immutable private static
cache policy. No Vary, Content-Encoding, cookies, range support or redirects. HTML
is no-store. Do not describe these headers as having existed in earlier experiments.

## Required paired evidence

- Unique owned loopback origin per pair; known browser/context; cache clear before
  cold; no prior prime, intervening navigation, request interception or service worker.
- Exact seven request paths per navigation. One selected Font request, one document,
  GET200, matching frame and current loader, ordered request/response/finish events.
  Separate raw HTTP-cache, disk-cache, prefetch and service-worker facts; no combined
  cache boolean. Require the warm font's own cache event, not another asset's hit.
- Exact cold font bytes/hash plus identical selected request/response representation
  headers. No warm font body is invented. A missing body must retain the pinned CDP
  error; other errors refuse this method. Record both request IDs and loader identities.
- Exactly one cold server response per selected resource; only no-store HTML may
  contact the server in warm. Pin frozen server/resource manifests before and after.
- Identical paired PNG and geometry; both paints use the expected custom Inter at
  every visible text parent. The unused full-font fallback must not be requested.

The checker returns `identityMethod: isolated-cache-lineage`,
`directWarmBodyObserved: false` and `verification: not-verified`. A matching lineage
is one v2 gate input, not a performance result. The complete v2 oracle must derive
request/uncached-body counts from its graph and still require all numeric, task,
image/font, matrix and cleanup evidence. An empty failure list authored by a collector
must not substitute for those derivations. Graph claims do not authenticate their author.

## Refusal controls and collection order

Freeze the contract, collector, checker and tests before collecting a v2 result.
First seed wrong cold bytes; extra/prior prime; changed URL/frame/loader or request
headers; Vary/encoding/length/ETag drift; redirect/304/range; SW/prefetch/interception;
missing/misordered font cache event; extra warm server contact; missing font request;
BFCache; system-font fallback; manifest drift; malformed or absent facts. Each must
refuse lineage independently of an unrelated budget failure. No source-file mutation
or oracle replacement is allowed to make a failed historical packet pass.

Then collect the complete matrix with retained executable snapshots, original and
candidate input copies, request/server traces, rendered observations and owned-process
cleanup. Run live cache-clearing and representation-policy fault controls separately;
their detections do not count as successful workload observations. Replay the original
asset control: its oversized cold payload must still fail the unchanged numeric budget.
Keep v1 results separate and failed where applicable. Independent review precedes any
claim that v2 closed this scoped measurement; all other target gates remain required.

## Retained acquisition

Use the already-approved isolated Playwright installation and installed Chrome; no
downloads, signed-in profile or consumer files. Supply absolute paths to the exact
historical candidate/original receipts pinned in `cache-lineage-contract.json`:

```sh
APPKIT_PLAYWRIGHT_MODULE=/absolute/existing/playwright/index.mjs \
  node lineage-performance.mjs /absolute/candidate/receipt.json \
  /absolute/original/receipt.json /absolute/private/output-parent matrix
```

Run the same command separately with `cache-clear`, `representation-policy` and
`original-control` instead of `matrix`. The launcher retains its full local import/test
graph before execution, runs those retained tests, and retains every input, raw CDP
event, server trace, available response body, screenshot and process-cleanup result.
Fault packets report detection, never workload success. No averaging, retry-until-green
or omission of failed packets is allowed. `Taskfile.yml` runs the pure refusal suites;
that does not launch Chrome or satisfy the live contract.
