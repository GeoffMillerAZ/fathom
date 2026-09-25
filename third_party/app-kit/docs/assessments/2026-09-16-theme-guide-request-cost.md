# Theme-guide request cost and exact-browser replay

The `core.themes-style-guide` performance work now has a declared, measured
server-side workload. It remains partial: this is not cold/warm browser latency,
degraded-network, peak-memory, cancellation or consumer certification.
[ADR-0031](../adr/0031-local-app-owned-theme-review.md) still owns the experimental
editor; [ADR-0069](../adr/0069-prioritize-starter-patterns-and-just-in-time-conversion.md)
requires applicable starter checks without full-catalog optimization.

## Prospective workload and result

The [request-cost contract](fixtures/styleguide-performance/README.md) was written
before execution. `BenchmarkThemeLabRequest` uses the actual Gallery adapter with
Synthwave source, three real specimens, both schemes and eight contrast checks.
Construction/setup is outside timing. A fixed loopback request identity drives an
in-process `httptest.ResponseRecorder`; no socket or browser is in those timings.

Hardware: Apple M4, 32 GiB RAM, Darwin25.6.0 arm64. Tool: Go1.26.3 darwin/arm64,
`GOMAXPROCS=2`, non-race measurement, five samples of ten operations per case.
These are warm-process sample means, not percentiles or startup measurements.

| Observed dimension | Result | Prospective ceiling |
|---|---:|---:|
| Mean request cost across 50 samples / 10 cases | 0.491–1.328 ms | 50 ms per sample |
| Allocation per operation | 754,294–1,585,056 bytes | 16 MiB |
| Largest normal benchmark editor HTML | 20,536 bytes | 128 KiB |
| Largest actual specimen response | 57,929 bytes | 600 KiB |
| Fixed-identity candidate ZIP | 16,036 bytes | 1 MiB |

The separate parity test compares two executions of twelve cases using status,
sorted complete headers and body hash/length. It includes invalid-field422 and
stale-acknowledgement409, not merely successful exports. Its DOM-derived digest
comes from the preceding preview/source-comparison response, not an invented token.
This records determinism, not a cross-version golden or proof of every HTTP state.

The measured costs do not justify extra candidate-cache or request-reuse plumbing
for this workload. Production templates, validation, export recomputation, public
APIs and dependencies are unchanged. Future optimization still needs a named cost
and a before/after comparison; repeated source work alone is not a user-visible defect.

## Retained identities

Private root: `/Volumes/m2ext/appkit-evidence/appkit-theme-request-cost.POyzPh4P/`.
`baseline/` retains310 selected repository source/test/config inputs, the executed
runner, explicit environment, tool identities, raw logs, test binary and server.
Inputs match before/after execution. This is a working-tree build with existing local
module/cache inputs, not a hermetic dependency closure or independent cold build.

| Artifact | SHA-256 |
|---|---|
| Preparation/measurement receipt | `35c5a44dbeb75268e9009f1c83ad1d9407f4b4f79c10eccb8d791f30b66bdd5f` |
| Executed test binary | `98cbb3789f1812f56a9bb952e4dd63ca86d0a0c12271537459ad13b06440643d` |
| Separately built Gallery server | `d3fba0eb7ec44f0b091532efa1173769e5afca64ffd3bd6ac45f955c6361a0a7` |
| Browser launch/cleanup receipt | `3c8d7328672c34dc67e950a64b764a1b49ceb3778aac5246982f32d2af960bd6` |
| Native review-loop receipt | `5f0edac648b3790c6edcff9d68632a0aa8e4084088880837cf72d06950972809` |

The original measurement runner's `passed` means command success and unchanged
inputs only. Independent review of the raw50 rows found every declared request
budget satisfied; it does not turn that runner into a budget-enforcing gate.
The subsequently retained `checker-01/` applies the maintained prospective checker
to those unchanged logs:10 cases,50 samples and12 parity states pass. Its16 synthetic
tests include missing/extra observations, wrong operation/concurrency counts,
nonfinite or over-budget values and malformed response identities. The original
runner receipt is not rewritten. Checker source SHA-256 is
`7699a05bd52994c40e788e369044c81cb47a0b4148fea2b2f7283055b08929fa`;
the retained test source is
`e3418a5afe8151fdb459b57bdd18035870d01ccb46a5514ee17bcc34fb7f85fe`.
Before execution, root review corrected an ineffective synthetic byte-budget
replacement and added concurrency/nonfinite/null cases; no executed failure was
discarded. The checker validates recorded values, not evidence authenticity.

## Browser task and asset boundary

The unchanged, retained `apps/styleguide/browser/review_loop.mjs` ran against the
exact server above in a fresh Chrome152.0.7977.84 profile. The document's executable
identity matched the retained server. All47 scoped checks pass: real native preview,
three specimens in light/dark, source comparison, deterministic ZIP download,
stale/invalid-edit refusal, correction and reset. Nine captures are retained; this
slice adds no exhaustive visual inspection or accessibility certification.

The first successful guide, guide CSS, dashboard preview and selected app CSS bodies
total98,344 decoded bytes:20,267 +11,265 +15,439 +51,373. That is body accounting,
not wire traffic, a cold-cache measurement or an independently declared browser-byte
budget. Both downloads are16,364 bytes and identical. Their larger source label and
actual binary identity differ intentionally from the in-process fixture's fixed
identity; cross-profile ZIP byte equality is not expected.

No out-of-scope requests or page errors were observed. The owned server exits0 and
its selected port refuses afterward. The harness closes its fresh browser; no
signed-in profile, source/preferences, consumer, installed binary or deployment changes.

## Remaining gate

Measure complete cold/repeat/degraded browser tasks, first useful view, update/export
latency, retained/peak memory and cancellation with the selected assets and actual
consumer workload. Keep app-owned callback CPU/I/O and authentication boundaries
explicit. The CUE inventory records partial request-cost evidence, never a complete
`converted-core-v1` performance pass or a completed starter area.

Current working-tree race checks pass for `apps/styleguide`, `apps/gallery`,
`internal/qualityplan` and `cmd/appkit-mcp`. They validate the new fixture and
authored metadata/doc routing, not the remaining runtime performance dimensions.
