# Benchmark evidence verification — 2026-09-16

Scope: decision [0011](../../../intent/decisions/0011-workload-selection-evidence.md).
Baseline kit `2e1b1bbfd6277bc4f8835928e200b7ef516ff1be`; implementation and this record
land together. This is implementation/compatibility evidence, **not measured model
quality, runtime selection, production adoption or authorization**. No paid calls.

## Kit checks

| Check | Result / scope |
|---|---|
| `task validate` | PASS: formatting, vet, CUE including fixtures, race tests and nested modules |
| `task constitution-lint` | PASS; no gate, dependency, tier or package-count changes |
| `task eval` | PASS: six replay scenarios, six records, zero regressions; observed probe agreement, not live quality |
| `go test -race ./kits/eval-runner -count=1` | PASS, including actual Go JSON → CUE projections |
| `TestRun_InvalidOutputBindingRefusedBeforeSideEffects` | PASS: missing/duplicate/foreign/empty IDs and producer hash mismatches cause no probe/model/journal activity |
| Producer-binding negative control | EXPECTED FAIL: an isolated Go overlay weakening equality to model-only makes both prompt/parameter mismatch cases fail with an unexpected nil error |
| Benchmark negatives | PASS: malformed IDs/digests/timestamps, non-finite/overflow costs, conflicting price/spec identities, missing acceptance artifacts, unpaired/unknown/stale/replay/synthetic comparison inputs |
| `TestBenchmarkRetainedConsumerReceipt` | PASS: reconstructs the retained summary and verifies exact trial/acceptance/output artifact bytes |

The negative control changed only a temporary copy of `runner.go` through a Go test
overlay. The actual source was never weakened. Returning the real producer comparison
to the same test passes. No gate bypass, network or cassette regeneration was used.

## Real consumer, synthetic inputs

[Overlay bridge](../../../../examples/benchmark-evidence/README.md) ran inside actual
resume_as_code `7a6c56a26dd10a7cf80128d23ef13bab90377ae8`, `cmd/redteam`.
`go list` confirmed the proposed kit root, not the consumer's vendored kit. A temporary
module file used app-kit `d22871195401594f1083e2952d938ec5dfd655d3` and the proposed kit;
`GOWORK=off GOPROXY=off`, `-mod=readonly`, `-race`, `-count=1`.

`TestKitBenchmarkReceiptPilot` passed. Existing `cmd/llm`, `cmd/redteam` and `cmd/ops`
race suites also passed. The actual `Faithfulness`/`Ladder` paths produced one accepted,
one rejected and one unavailable/error task; both scripted reviewers were called for
each task. Three unknown costs stayed unknown. Synthetic evidence was refused by the
comparator. Consumer source, vendor, pins, model choices and résumé data were untouched.

[`consumer-receipt.json`](consumer-receipt.json) retains the evidence, computed summary
and six exact UTF-8 JSON artifact strings keyed by their SHA-256. Hash the **string's
bytes**, not a parsed/reformatted artifact. The receipt's fixture clock and task durations
are injected, not latency observations. Internal consumer critique fields may contain
incidental execution timings; they do not turn this synthetic test into a benchmark.
Manifest component materialization is reproducible from the pinned consumer source and
overlay; this small fixture is not a production collector or a complete artifact store.

## Downstream compatibility

Sources were tested through isolated module overrides/copies, without mutating actual
consumers or populating private fixtures. Candidate mode is distinct from default-vendor
production adoption. All commands used offline module resolution.

| Consumer revision | Build | Tests |
|---|---|---|
| throughline `a007075fab6ac156be44de46df941cce4a0915ed` | PASS | PASS; candidate and frozen-kit baseline, including architecture subprocesses |
| fathom `68a534854f1bb8d4e68b2377a2ff4d198e6bef48` | PASS | Known failure: `TestNarrowProfileSchemaBytes`, 34,821 bytes exceeds 34,000 |
| growing-gardens-bi `7503dd1c270bf6ceb72d42c742f507b9513db0dc` | PASS | Known failure: 14 finance cases lack `../../tmp/financials.json`; no private fixture copied |

Throughline needed identical offline, temporary-only module graph normalization for
candidate and frozen kit `8b252304978bd249e06658f2bd6be81194ae3df5`: x/sys .46→.47
and x/sync sums .21→.22. Command-scoped `GOFLAGS=-mod=readonly` made architecture-test
subprocesses use the same mode; otherwise default-vendor consistency checks failed.
No actual go.mod, vendor or global Go configuration was changed. Fathom/GGB failures
were already reproduced against the prior kit in the earlier delivery assessment.
Fathom's final run used `env -u GOFLAGS ... go test -mod=readonly ./...`: applying
Throughline's inherited GOFLAGS also trips Fathom's environment-integrity test. Clearing
that command-local variable leaves only the known schema-size failure.
The aggregate downstream gate is **not green**; no main merge or tag is claimed.

## Intent-doctor (read-only wiring check)

Working-tree result: 20 PASS, 1 WARN, 2 SKIP. This includes the owner's separate,
uncommitted intent-distribution work; it does not claim that work was delivered here.

| Checks | Result | Evidence |
|---|---|---|
| 1–4 Router, budgets, required living docs | PASS | Router 80 lines; other living docs ≤150, futures 111≤180; all required docs present |
| 5–6 Local decisions/index/shape | PASS | All local record links match; Status/Revisit sections present; new 0011 is 65 lines |
| 7–8 Triggers and exits | PASS | G1 has Done when; futures have individual or explicit inherited confidence/horizon/promotion/drop conditions |
| 9–10 Compatibility/sensitive surfaces | PASS | Scope defines the surface; router has non-empty Done needs for each sensitive surface |
| 11–12 Changelog | PASS | Release impact first; Unreleased body 54≤60 lines in this working tree |
| 13–14 Autonomy/incidents | PASS | Three tiers/branch model; INC-7 has a proposed outcome; INC-14 closes to executable regressions |
| 15–17 Stanzas/skill paths | PASS | CLAUDE/AGENTS intent blocks identical; both local skill roots resolve to the same project bodies; host invocation not inferred |
| 18 Installed versions | WARN | Claude Code 2.1.273 differs from tested 2.1.270; Codex 0.154.0 matches. Historical native-host receipt is not extended |
| 19–21 Links/pointers/charter | PASS | Relative links resolve; only root owns intent; charter maps components with lifecycle rules |
| 22–23 Nested stanzas/hygiene | SKIP | No component-owned intent routers |

Most useful wiring follow-up: reverify the newer Claude host under the existing native
test scope before claiming current host enforcement. No hook/config reconciliation was
performed as part of this benchmark implementation.
