# The test-run hook: the plan (vision 40, ADR-056)

Grammar v2 (`docs/plans/README.md`). A row flips to `[x]` only when
`fathom verify --service fathom --plan docs/plans/test-run-hook.md` proves it. Design:
`docs/design/test-run-hook.md` (section numbers below are its). Goal G27.

Fathom's level 10: when the agent runs the repository's declared test command, one line comes back
naming the failing tests and the changed symbols they reach, and nothing on a pass. Tiers 0 and 1,
provider none, opt-in. Beside `docs/plans/test-intelligence.md` (the parse and the reach) and
`docs/plans/archive/harness-port.md` (the hook contract row); the level 10 bench cell sits under the strengths
bench. Tier 2 (a failure-kind judgment) is deferred behind the sensor's data and J0.

## Wave 1: verify the facts, then the sensor (sections 3 and 7)

- [x] (op) TR-V1: the hook facts are verified against the current Claude Code: whether PostToolUse(Bash) input carries an exit code, and whether a failing test command arrives as PostToolUse or PostToolUseFailure {id: TR-V1, oracle: lexical, path: docs/bench/readings/test-run-hook/hook-facts.md, pattern: "^TR-V1 reading:"}
- [x] The declared test commands are read from the Taskfile, Makefile, package.json and a `[tests]` config, and a Bash command is matched against them {id: TR-1, symbol: matchDeclaredTestCommand, package: cmd/fathom, reachable_from: TestMatchDeclaredTestCommand}
- [x] A new hook contract row (HP-27 stays silent for tools that are not fathom's): register on PostToolUse(Bash) and PostToolUseFailure(Bash); a Bash call that does not match a declared test command exits before any daemon call {id: TR-2, symbol: cmdTestRunHook, package: cmd/fathom, reachable_from: TestTestRunHookSilentForNonMatchingCommand, needs: "TR-1,TR-V1"}
- [x] Tier 0 parse: plain `go test`, `go test -json` and JUnit XML give failing test ids, panics, file:line frames and build errors; other runners only on demand {id: TR-3, symbol: ParseTestRun, package: internal/testrun, reachable_from: TestParseTestRunGoPlainJSONAndJUnit}
- [x] The three-way result: pass only when the runner's final summary covers every package, otherwise unknown (stdin mode never sees the exit code); the exit code is used only when the hook input carries it {id: TR-4, symbol: testRunVerdict, package: internal/testrun, reachable_from: TestTestRunVerdictUnknownWithoutFullSummary, needs: "TR-3"}
- [x] Tier 1 candidates: `localize` on the failing test or stack trace with hops capped at 4, and `test_impact` over the `git diff HEAD` symbols intersected with the failing tests, naming the changed symbols each failing test reaches {id: TR-5, symbol: testRunCandidates, package: internal/testrun, reachable_from: TestTestRunCandidatesNameChangedSymbolsFailingTestsReach, needs: "TR-3,test-intelligence.md#TQ-4", weight: 3}
- [x] The output line stays within the I35 caps, says nothing on a pass, and names the path of the full output saved under `workspace/tmp` (I31) {id: TR-6, symbol: renderTestRunLine, package: cmd/fathom, reachable_from: TestRenderTestRunLineSilentOnPassAndNamesSavedPath, needs: "TR-5"}
- [x] One row per run in the existing hook log (runner, three-way result, failing ids and counts, no output text), never counted as certificate evidence (I43) {id: TR-7, symbol: logTestRunRow, package: cmd/fathom, reachable_from: TestTestRunLogRowCarriesNoOutputText, needs: "TR-4"}
- [x] The hook injects only: no `updatedToolOutput` rewriting, no blocking, no command execution {id: TR-8, symbol: cmdTestRunHook, package: cmd/fathom, reachable_from: TestTestRunHookInjectsOnly, needs: "TR-2"}
- [x] `[hooks] test_run` is the opt-in switch, default off until the bench {id: TR-9, symbol: Load, package: internal/config, reachable_from: TestConfig_HooksTestRunDefaultsOff, needs: "TR-2"}
- [x] The hook stays inside its descriptor's deadline (I53) and skips silently on a missed one {id: TR-10, symbol: runTestRunWithDeadline, package: cmd/fathom, reachable_from: TestTestRunSkipsSilentlyOnMissedDeadline, needs: "TR-6"}
- [x] `docs/configuration.md` documents `[hooks] test_run` and what the hook does and does not do {id: TR-11, oracle: lexical, path: docs/configuration.md, pattern: "test_run", needs: "TR-9"}
- [x] A piped failing Go test run (exit 0, a tail of the output) injects the test-run line: the PostToolUse(Bash) row is wired wherever its PostToolUseFailure twin is (fleet-adoption-fixes.md#FA-7), and a failing run is read from its output (--- FAIL, FAIL<tab>pkg, panic:, a panic's stack naming its test when the --- FAIL line fell outside the tail) whatever the exit status, within the stdin cap (O175) {id: TR-13, symbol: cmdTestRunHook, package: cmd/fathom, reachable_from: TestTestRunHookPipedFailingRunInjectsLine, needs: "TR-2"}

## Wave 2: the level 10 bench cell (section 6)

- [ ] The bench scenario doc: 20 mutation-seeded test-fix tasks on one open source Go repository fathom did not write, arms A control, B the hook (tiers 0 and 1, provider none) and D raw level 10 (bench sandbox only, because it executes commands) {id: TR-12, oracle: lexical, path: docs/bench/scenarios/test-fix-level10.md, pattern: "raw level 10", needs: "TR-8"}
- [ ] (op) TR-M1: arm B uses at least 20 percent fewer tokens-to-green than arm A, the paired CI excludes 0, and correctness does not drop {id: TR-M1, oracle: lexical, path: docs/bench/readings/test-run-hook/level10-cell.md, pattern: "^TR-M1 verdict: (PASS|FAIL)", needs: "TR-12", weight: 5}
- [ ] (op) TR-M2: arm D (raw level 10) is read against B and the result is published either way {id: TR-M2, oracle: lexical, path: docs/bench/readings/test-run-hook/level10-cell.md, pattern: "^TR-M2 reading:", needs: "TR-M1"}
- [ ] (op) TR-M3: hook p95 under 1 s on the owner's traffic, with the silent-skip rate named {id: TR-M3, oracle: lexical, path: docs/bench/readings/test-run-hook/latency.md, pattern: "^TR-M3 reading:", needs: "TR-10"}

## Deferred and refused (recorded so they are not re-proposed)

- [-] Tier 2 failure kind: opt-in site `triage_failure`, a choice over code_bug, environment_or_setup, timing_or_flaky, test_expectation_outdated and other, over a redacted excerpt of at most 1.5k tokens plus two spans (section 5) {id: TR-D1, disposition: deferred, reason: "no failure stream exists until the sensor ships; it is the program's one labeled template", trigger: "the hook has logged at least 100 failing runs later followed by green, and J0 has run (F35)"}
- [-] `test_expectation_outdated` shown to the agent before it clears R3's pruning-and-gating bar {id: TR-D2, disposition: deferred, reason: "shown uncalibrated it licenses editing the test to match buggy output, which conservation does not catch", trigger: "the template's Wilson 95 percent lower bound reaches 0.90 on settled failures"}
- [-] MCP exposure of test-run output as a parameter of `localize` (kind: test_output) or `test_impact`, tool count unchanged {id: TR-D3, disposition: deferred, reason: "no new tool without call-log demand (ADR-042 decision 5, ADR-053)", trigger: "hook or CLI use by at least 5 percent of sessions (F42)"}
- [-] A public `fathom triage` verb as the delivery {id: TR-X1, disposition: refused, reason: "tools agents must choose to call die; an internal subcommand exists for tests only"}
- [-] Output rewriting (`updatedToolOutput`) and command execution by fathom {id: TR-X2, disposition: refused, reason: "fathom stores and serves; the harness runs commands under its own permissions; the deleted shell_out stays deleted"}
- [-] A Stop-hook completion check judged by a model {id: TR-X3, disposition: refused, reason: "it duplicates the Stop hook and PC-22, and its certificate half has no data"}
