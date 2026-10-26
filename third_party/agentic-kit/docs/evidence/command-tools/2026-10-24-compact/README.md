# Compact selected-view qualification

GT13a/CV-1..5, 2026-10-24. [CV](../../../designs/command-compact.md) and 0039 own
this command-local profile. It qualifies optional lossless selected-row encoding,
not caching, token counts, native authentication, installation or production savings.

| Instrument | Actual result |
|---|---|
| [Source pins](source-pins.json) | Exact command/schema/tests/guide/skill/spec bytes based on `07e28b0`; artifact was built from uncommitted qualified source, not a claimed clean release. |
| [Native trial](native-trial.json) | 34/34 owned synthetic Darwin arm64 cases passed. Independent Python decoding of the published tuple grammar matched ordinary JSON headers, row order/values and omission/coverage fields. Source/control bytes and modes remained unchanged. |
| [Report shapes](report-schema-trial.json) | All 66 nonempty actual output reports passed CUE; ten single-fault tuple/header/operation negatives were rejected. Two intentionally silent tiny-cap outputs contain no JSON report. |
| [Security review](security-review.md) | Independent source/counterexample review closed scan-budget diagnosis, future-header fallback and INC-21 findings; all 101 tested inspection prefix caps were monotone and exact. Shared Fathom was resource-refused; current source was read directly. |
| [Guide trial](guide-trial.md) | Final binary independently decoded all four layouts, preserved capped prefixes/omissions, help, failures and tiny-cap behavior; all 137 owned repository/control files remained byte-identical. |
| Full gate | `task validate` passed; focused race tests passed at 75.0% statement coverage, which is not assurance of every behavior. Skill structural validation passed; routing checks were manual, not installed automatic activation proof. |
| Static plan proof | Fresh isolated-store strict candidate proof: seven claimed GT rows verified, zero failing. Symbol/test reachability is separate from executed CLI qualification. |
| Cross-builds | Windows amd64 and Linux amd64 passed. No native execution or filesystem qualification on those operating systems. |

Final native binary SHA-256:
`acdea56bc680817ac4e178405f7b793965b39da1315efe4e548034ee46428091`.

The 400-line marker task encoded 30,229 ordinary bytes versus 21,868 compact bytes
(27.66% fewer). The 30-path status task encoded 2,347 versus 1,808 (22.97% fewer).
Known answers, identities and diagnostics were preserved. These are synthetic byte
measurements, not measured tokens, review quality, human effort or production savings.

The bounded 100-iteration Apple M4 encoding benchmark measured ordinary JSON at
26,923 ns/op, 16,594 B/op, 2 allocations and compact at 263,809 ns/op, 85,201 B/op,
662 allocations. Compact adds encoding work; it remains opt-in. Removing redundant
per-row JSON round trips reduced the earlier compact sample of 531,198 ns/op,
390,241 B/op and 7,463 allocations. Host load varies; this is no end-to-end latency
or production p95 claim. Current Git status/diff/history profiles cap at 32 rows.

## Retained counterexamples and corrections

- INC-21: the baseline accepted cap of one byte emitted a 325-byte JSON or 382-byte
  text failure. Final-write checks now carry the accepted cap and include newline;
  undersized caps emit zero bytes. Exact-fit/one-byte-short and writer tests pass.
- A compact prefix that fits output can exceed the normalized scan representation's
  total budget. Selection now checks both before scanning; a zero-prefix total-budget
  failure is `payload_limit`, not `output_limit`. No finding triggers a smaller retry.
- Unknown future headers or row wire behavior retain ordinary JSON; closed compact
  CUE must never receive an unrecognized extra header. Custom value/pointer JSON row
  methods and unfamiliar tags/kinds conservatively retain ordinary output.
- The initial native fixture's local hook-path config was correctly refused. The
  fixture now uses a per-invocation preparation override and default registered config;
  guard policy was not loosened. The initial 40-path selection also correctly held at
  GT6's 32-row ceiling; retain it as a negative and use 30 for successful qualification.
- One trial oracle wrongly predicted an incomplete result at a 500-byte Git cap.
  A valid 497-byte report fit, disclosed zero rows and truthfully omitted 30. The final
  corpus checks that success and the 496-byte failure; no implementation was relaxed.

Private retained evidence includes exact fixtures, outputs, runner/decoder, original
failing receipts, source snapshot, guide corpus and logs in the maintainer's dated
`2026-10-24-command-compact` evidence directory. No real credentials or provider calls
were used. Native Windows/Linux behavior, cache invalidation and full GT13/17 remain
unqualified. Prose/HITLMAN production UI and the broader goal remain open under 0038.
