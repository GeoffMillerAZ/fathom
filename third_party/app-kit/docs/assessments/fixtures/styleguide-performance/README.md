# Theme-guide request-cost specimen

This is a server-side slice of `core.themes-style-guide`, not its complete browser
performance gate. It uses Gallery's actual `NewThemeLab`: Synthwave source, three
app renderers, two schemes, eight contrast checks and one selected application CSS
path. No paid assets, real application data, listener or browser is required.

## Prospective workload and budget

Freeze these limits before the first measurement. On the named development Mac,
with the recorded Go binary, `GOMAXPROCS=2`, ordinary non-race instrumentation and
the existing module graph, run five samples of ten operations per benchmark.
These are warm-process, in-process handler measurements; construction, fixture
setup, sockets and browser rendering are outside timing:

- Initial guide, candidate preview POST, source-comparison POST and reviewed ZIP export.
- The actual dashboard, components and forms iframe responses in light and dark.
- Mean server work per operation at most 50 ms in every sample; allocated bytes per
  operation at most 16 MiB. These are local-development request costs, not p95 user
  latency, retained heap, peak process memory or a deployed service SLA.
- Editor HTML at most 128 KiB; each preview response at most 600 KiB; exported ZIP
  at most 1 MiB. Exact response sizes include server-emitted HTML or ZIP, but not
  separately fetched stylesheets, headers, compression or browser cache effects.

Record response status, complete headers and body hashes separately from timing.
An optimization must preserve those outputs for the same fixed source and identity,
including invalid edits, stale acknowledgement, baseline comparison and reset.
Timing assertions stay out of ordinary package tests. All refusals and source/schema
validation remain in place; no process-global candidate cache is admitted.

The limits target responsive local authoring while leaving explicit headroom for
the selected three-specimen workload. They do not certify the maximum supported
configuration or arbitrary app-owned callbacks. A smaller measured allocation/time
cost can justify a simple internal change; passing a budget is not a reason to add
a cache, runtime, dependency or public option.

## Commands

```sh
go test ./apps/gallery -run '^TestThemeLabRequestParitySnapshot$' -count=1 -v
go test ./apps/gallery -run '^$' -bench '^BenchmarkThemeLabRequest$' \
  -benchmem -benchtime=10x -count=5
node --test docs/assessments/fixtures/styleguide-performance/check.test.mjs
node docs/assessments/fixtures/styleguide-performance/check.mjs \
  /absolute/retained/benchmark.stdout /absolute/retained/parity.stdout
```

The parity test logs one `theme-lab-parity-json=` observation. An optional explicit
`APPKIT_THEME_LAB_PARITY_OUTPUT` path writes a new exclusive JSON file instead of
an accepted golden; the checker accepts either representation. It requires the
exact ten-case/five-sample/ten-operation matrix, the declared response/time/allocation
ceilings and the twelve parity states. Missing, malformed, extra or over-budget
observations fail. The checker does not authenticate the logs, source provenance or
response-body meaning; retain successful process exits and source identities separately.

Retain executed source/tests, tool identity, environment, command and output before
changing an implementation or harness. Compare only matching workload revisions;
never replace a failing run with a later receipt. Run race/package regressions
separately, because race-instrumented timing is not comparable to this contract.

## Remaining complete-workload evidence

The [prospective browser contract](browser-contract.md) and its [ordinary-lane
evidence](../../2026-09-16-theme-guide-browser-cost.md) add first/repeat useful-state
timing observations, complete selected CSS/body accounting and verified native exports
at both viewports. INC-52 qualifies the old timing endpoint: it did not explicitly wait
for every required asset body. The [transport follow-up](../../2026-09-16-theme-guide-transport.md)
retains that failure and tests a complete-phase barrier. Warm-cache, degraded-network,
memory and cancellation/recovery remain
incomplete. App-owned authentication and fonts remain separate boundaries; no full
performance or starter acceptance follows from this slice.

The independent ordinary checker requires all120 tasks even with `--scoped` and
reopens actual body/ZIP and selected identity files. It refuses a full-gate pass:

```sh
node --test docs/assessments/fixtures/styleguide-performance/browser-check.test.mjs
node docs/assessments/fixtures/styleguide-performance/browser-check.mjs \
  /absolute/retained/theme-guide-browser-v1-receipt.json \
  /absolute/retained/snapshot.json --scoped
```

Freeze the checker and tests before execution. Source/tool drift or an unavailable
artifact fails; never point this at a later edited runner to certify old results.
Acquisition setup: [browser guide](../../../../apps/styleguide/browser/README.md).
