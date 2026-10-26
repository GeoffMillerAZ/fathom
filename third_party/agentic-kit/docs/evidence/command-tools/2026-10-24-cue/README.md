# CUE byte-contract compatibility

INC-22/GT-CUE-1, 2026-10-24. Source and native Linux CI qualification passed.
The full command program remains open. This repairs schema validation paths, not command execution behavior,
native platform support, classifier calibration, policy authority or installed hooks.

The [failed main run](https://github.com/GeoffMillerAZ/agentic-kit/actions/runs/40088051504)
at `dd2e47f` installed CUE 0.17.1. Its gate printed the command-guard directory,
then the silent `cue vet ./testdata/` invocation failed as incomplete. Earlier runs
at `07e28b0` and `82c6495` failed at the same step. Race/WASM steps were not reached.
Local CUE 0.16.1 passed, including explicit concrete vet. Neither local success nor
normal Git hook success established remote CI success.

An isolated 0.17.1 reproduction passed `cue vet ./...` and failed fixture vet;
concrete vet identified `len(pattern)` and `len(executable)` conditions. Replacing
their same-field bottom bodies with unrelated private bottoms did not fix it.
Replacing eager conditional guards with private numeric constraints, for example
`_byteLimit: len(pattern) & <=2048`, passed both validator versions.

The repair covers all three byte guards: pattern 2048, scanner executable 4096 and
literal 1024. Rune bounds remain. A rune limit alone would admit too many UTF-8 bytes
and is explicitly rejected. No workflow, gate flag, module dependency or global tool
installation is changed; a scratch validator is used for qualification only.

The full 0.17.1 gate then exposed a separate `harnessctl` control-shape failure:
conditional closure left the class disjunction unresolved and rejected valid mandatory
fields. An explicit closed union over the shared base preserves mandatory/advisory
values. `TestControlSchema` compares strict Go and CUE acceptance for 15 concrete
cases: both classes valid, mandatory weakening, invalid advisory ranges and unknown
fields/classes. Toy signed registrations, layers, envelopes and actual check/explain
reports were also checked with both validators by the implementation worker.

`TestCUESchemaByteLimits` runs the actual fixture command and concrete, otherwise
valid instances of `#Rule`, `#ScannerPolicy` and `#SearchRequest`. For each it checks
ASCII at/over limit, UTF-8 at byte limit, and one/two UTF-8 bytes over while rune count
remains within the existing bound. CUE absence is an explicit skip, not qualification;
CI installs CUE before tests. Source, executed test and remote CI receipts follow.

| Instrument | Result and limit |
|---|---|
| Controller | Full `task validate` with CUE 0.17.1 passed, including race suites. Normal pre-push also passed with default CUE 0.16.1. |
| Dual-version regressions | Byte boundaries and strict control parity passed with CUE 0.16.1 and 0.17.1; no skipped-CUE result is called qualified. |
| [Independent helper](helper-trial.json) | Exact 468-file source snapshot: 295 CUE 0.17.1 schema/fixture checks, focused race/vet on Go 1.27.3, CUE 0.16.1 byte tests and 15 independently counted control subtests passed; source bytes unchanged. Not the full repository gate. |
| Independent review | Four source files reviewed; mandatory floors, byte ceilings, closed fields and Go acceptance retained. Same-version counterexamples closed. |
| Native Linux CI | [Run 40093730634](https://github.com/GeoffMillerAZ/agentic-kit/actions/runs/40093730634) at `c7633a4`: Go 1.26.9, CUE 0.17.1, full race gate and WASI/WASM passed. [Receipt](../2026-10-24-rollback/ci-result.json) binds the source and steps. |

The first post-repair run, 40091343269, passed CUE but exposed the separate scaffold
rollback failure (INC-23). The later green full run closes both incidents; a passed
CUE step alone was never treated as a green repository gate.

The helper's original CUE 0.16 test selector matched only the byte test; its later
explicit control run required all 15 subtests to pass. A zero-test pass is not evidence.
Controller disk pressure briefly prevented a shell temporary write; source copies were
retained, heavy focused checks used the spare Mac and temporary files stayed external.
No active apps or sessions were stopped. The controller gate ran serially after space
recovered. These operational facts establish no measured command-performance benefit.

## Uplift decisions from the current source survey

JEV/TypeSafe, OpenAI Decisions, LLM emulation, None/replay, confidence cascades and
calibration already exist under `foundations/decision`; reuse those ports. The lab
is offline and has no provider transport. No native Ollama decision adapter was
found. Existing DLP/sensitivity primitives are regex-based; no structured checksum
recognizer was found. GT5/12 retain these actual gaps and their qualification bars.
Do not duplicate a decision engine, infer PII absence from Gitleaks, or use a model
to answer exact byte/scope/permission questions. The CI parity repair takes priority
over another feature. This survey admits no new provider call or implementation slot.

Primary upstream context: [CUE 0.17.1 release](https://github.com/cue-lang/cue/releases/tag/v0.17.1).
The release page is not proof of this particular failure; the retained same-version
reproduction and independently tested bounds establish the local observation.
