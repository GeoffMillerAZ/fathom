# Concern-site qualification: implementation present, hosted observations pending

Source: [`7299f5d`](https://github.com/GeoffMillerAZ/agentic-kit/commit/7299f5d3f36518cf9db60f6124e98eeb32d81311).
Contract: [0042/MC](../../../designs/command-concern-classifier.md). **GT12A remains
in progress:** the implementation and owned Mac qualification pass; the new hosted
Linux, Windows and macOS observations did not execute. Broader GT12 runtime/model,
private-input, residency and quality qualification remains open.

## Observed evidence

| Instrument | Observation | Limit |
|---|---|---|
| Controller Go 1.26.3, CUE 0.16.1 | Full `task validate` passes for ordinary and `command_classifier` builds; normal pre-commit/pre-push hooks pass | Local gates, not hosted CI |
| Owned Mac, Go 1.27.3 | Core 18/18, scanner 27/27, client fixture 30/30 | Actual darwin/arm64; owned synthetic inputs |
| CUE 0.16.1 and 0.17.1 | Both build contexts, actual emitted ordinary/text/compact reports and positive/negative controls pass | Shape and rejection tests, not model accuracy |
| Actual HTTP fixture tests | Whole-input framing, context binding, hard-hold precedence, reply/request/header/input bounds, cancellation/deadline after arrival and second-attempt rejection | No inference or real provider |
| Independent source review | No remaining confirmed blocker after closing missing unsupported receipt and forged held-row acceptance | Source review, not production security attestation |
| Independent guide-only trial | Revised complete example, three formats, concern/unknown/pin holds, ordinary-build unsupported hold and 30/30 fixture cases pass | No implementation/schema consultation for the revised setup |
| Fresh isolated Fathom index | Nine claimed rows verify strictly; the unclaimed GT12A also has verified static criteria in the tagged context | Static reachability, not an ingested runtime attestation |
| Linux/Windows cross-builds | Ordinary guard, tagged guard and qualifier compile | Compilation is not native execution |

The owned builds preceded source publication. [owned-source.json](owned-source.json)
binds their exact source-file bytes to the published revision. Final test/schema
refinements reused the A3 binaries after independently checking unchanged production
Go inputs. Each [native receipt](owned-mac-files-loopback-fixture.json) includes its
actual guard byte pin, roster, OS/architecture and every fixed case. The scanner
receipt identifies its independent scanner pin. The fixture receipt explicitly
requires `syntheticPeerOnly: true`.

Fathom's typed graph needs the selected build context: use
`GOFLAGS='-p=1 -tags=command_classifier'` during an isolated ingest before verifying
the advanced engine. An ordinary-build graph correctly sees the unavailable stub.
No library exemption or lowered verification criterion substitutes for that context.

## Startup regression and correction

MC-A10 declared nine alternating pairs, three inspection formats, eight public files
(2328 bytes), identical controls/host and a maximum median ratio of 1.25 before
measurement. Every observation below retains byte parity in all three formats.

| Build observation | Median elapsed ratio to saved baseline | Result |
|---|---:|---|
| Initial eagerly linked client | 1.674609 | Failed; retained in [failed observation](default-parity-failed-eager.json) |
| First lightweight split | 0.996828 | Within declared bar |
| Final lightweight build | 1.008963 | Within declared bar; [all timings](default-parity-final.json) |

Initialization traces, binary sizes and help-only timings implicated startup and
linked dependencies; they did not account for every microsecond. The ordinary build
excludes the engine. The explicitly tagged artifact retains HTTP/model startup cost
and has no performance qualification from the ordinary-build result. These are
observations on one synthetic corpus and host, not general latency, token or cost
claims. No threshold was relaxed and the failed run was not discarded.

## Hosted observations remain unknown

The [source CI attempt](https://github.com/GeoffMillerAZ/agentic-kit/actions/runs/40141845632)
and [six-job native attempt](https://github.com/GeoffMillerAZ/agentic-kit/actions/runs/40141845568)
were refused before any steps ran. No hosted qualification receipts were produced.
[hosted-attempts.json](hosted-attempts.json) preserves the observed failure and zero
executed steps while assigning qualification **unknown**, not pass or test failure.
Detailed service/account diagnostics remain private.

To close GT12A, restore the hosted execution prerequisite, run both workflows at the
source revision, retain all six actual native receipts, and reconcile every required
case. Re-run at a newer revision if production inputs changed. The prior GT4A hosted
core evidence does not qualify the new classifier fixture profile. Do not replace
missing native execution with cross-builds or a workflow definition.

Raw helper logs, fixture controls, guide trials, reviews and all failed observations
are retained outside the repository. [SHA256SUMS.json](SHA256SUMS.json) pins the
published receipt files. No model, runtime, hook or global configuration was installed.
