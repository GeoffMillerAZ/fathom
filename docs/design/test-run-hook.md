# The test-run hook: level 10 through fathom, proof first, judgment second

Status: accepted 2026-09-29 (ADR-056). Vision: `docs/vision/40-judgment-and-exploration-maps.md`.
Plan: `docs/plans/test-run-hook.md` (TR-). Goal G27. Builds on test intelligence
(`docs/design/test-intelligence.md`, TQ rows), localize (`internal/mcp/localize_tool.go`), conservation
(ADR-044), the harness port (HP-27, `docs/plans/archive/harness-port.md`) and the decision port (ADR-042,
ADR-056). Constraints carried in: inject only (ADR-029 section 5.3), fathom never runs commands (the
deleted `shell_out` stays deleted), I31 (artifacts under `workspace/`), I35 (payload caps), I43 (a piped
output is not an attested run), I9 and I53 (fail open, inside the hook deadline).

## 1. What level 10 is, and the question it answers

Level 10 of the ten-levels series (a public MIT repository, `apps/ten-levels/extensions/ask-jev.ts`)
gives the agent one tool, `ask_jev`. The agent passes a short state, up to 20 file paths, or a command.
Code runs the command (capturing exit code and output), assembles one state, and asks Jev the agent's
own typed questions (yes or no, a choice, a score). The agent sees the answers and never the files or the
output. A system-prompt nudge tells it to prefer the tool over its own reasoning. In the demo the agent
classified a red run as a code bug at 0.99, fixed a rounding step, scored its own diff and confirmed
green: 3 calls and $0.000084.

The owner asked whether fathom can use that powerfully and keep Jev optional. Yes to both, but the power
arrives from a different place: most of what that agent asks has an exact answer fathom computes with no
model, and the tool that asks it must fire at a moment the agent cannot skip.

## 2. Why the design differs from level 10

- **Proof first.** Did the run pass, which tests failed, which symbols those tests reach and which of them
  changed in the working tree, where the frames land, whether a test was deleted, skipped or lost
  assertions: all computable. Jev's 0.99 on "did all tests pass" becomes a parsed three-way answer
  (pass, fail, unknown).
- **A forced moment.** Level 10 depends on the agent choosing to call a tool, and fathom's own logs show
  chosen tools die (about 98k MCP calls: localize 40, test_impact 103, discoveries 13). Hooks and threads
  survive. So the delivery is a hook on the repo's declared test command, not a tool.
- **Judgment only on the residue** (tier 2, deferred), through the existing decision port and only after
  measurement.
- **Measured against raw level 10.** The bench includes the raw `ask_jev` as its own arm.
- **An honest difference.** Level 10 saves tokens by hiding command output from the agent. Fathom does not
  rewrite tool output (refused). Any saving comes from fewer search and read turns, and the bench measures it.

## 3. Mechanism (tiers 0 and 1, provider none)

**Registration.** A new hook contract row (HP-27's failure handler is silent for tools that are not
fathom's, so a Bash failure needs its own row). Register on PostToolUse(Bash) and
PostToolUseFailure(Bash) until the codec is checked, because `tool-failure-hook` fired only 19 times in
4 days, which suggests failing test commands arrive as PostToolUse results. The hook acts only when the
command matches the repository's declared test commands (Taskfile, Makefile, package.json, a `[tests]`
config); a Bash call that does not match exits before any daemon call.

**Tier 0, parse (deterministic).** Plain `go test`, `go test -json` and JUnit XML; other runners on
demand. Output: the three-way result (pass only when the runner's final summary covers every package,
otherwise unknown, because stdin mode never sees the exit code), failing test ids, panics, file:line
frames and build errors. The exit code is used only if the hook input carries it (to verify, Q32).

**Tier 1, candidates (deterministic).** Run the existing `localize` on the failing test or stack trace
with hops capped at 4, and `test_impact` over the `git diff HEAD` symbols intersected with the failing
tests. The result names which changed symbols each failing test reaches, usually 1 to 3 with spans. The
wired hook asks the daemon no `localize` (EW-76, `docs/bench/readings/efficiency-wave/hook-path-deadlines.md`):
its first candidate restated the failing test, and its call-graph build outlasted the hook's deadline. There
is no certificate history on this machine (0 live declarations), so "changed since the last attested pass"
and "flaky 2 of the last 3" are dropped in favour of the working-tree diff.

**Output.** One line within the I35 caps and nothing on a pass, for example: `2 failed: TestRound,
TestTotal; both reach money.Round (changed in working tree, money.go:41)`. The full output is saved under
`workspace/tmp` (I31) and its path is named, so the exact lines stay one read away.

**Hard limits.** Inject only: no output rewriting, no blocking, no command execution. No public `triage`
verb (tools agents must choose to call die); an internal subcommand exists for tests and debugging.

**Logging.** One row in the existing hook log per run: runner, three-way result, failing ids and counts,
and no output text. A piped output is not an attested run (I43) and never counts as certificate evidence.

**Rollout.** Opt-in behind `[hooks] test_run`; default-on only after the bench. **Cost:** $0 egress; the
daemon's point and index cost class; target p95 under 1 s, and on a missed deadline the hook skips silently.

## 4. What stays deterministic

Everything in tiers 0 and 1: the parse, the reach, the working-tree diff, the line, the cap, the log and
the saved output. The line is byte-identical with provider `none` and with any provider (a provider
changes nothing until tier 2 is promoted).

## 5. Tier 2, failure kind (deferred, F35; site `triage_failure`, opt-in)

**Jev primitive.** A choice over {code_bug, environment_or_setup, timing_or_flaky,
test_expectation_outdated, other}, with criteria text per option, over a redacted excerpt of at most 1.5k
tokens plus the candidate span and the assertion span. Schema: `state: {failure: {test, excerpt},
candidate: {key, span}, assertion: {key, span}}`, `questions: {failure_kind: {type: choice, criteria:
{...}}}`. A score `fix_plausible` (0 to 2) over a diff hunk and the excerpt is available later; every free
form question is labeled uncalibrated.

**Delivery.** Through a daemon endpoint within the hook deadline (the `/route` precedent); on a miss the
line is appended at the next hook firing. **Guardrail:** `test_expectation_outdated` is never shown until
it clears R3's pruning-and-gating bar. Shown uncalibrated it licenses editing the test to match buggy
output, which conservation does not catch (an expected constant edited to match reads as `edited`).

**Labels.** `failure_kind` is the program's one labeled template. Settlement from where the fix lands is
a weak label: test files only means outdated expectation; the next run passes with no change to reached
symbols means flaky; a change to reached non-test code before green means code bug. The owner adjudicates
oversampled positives.

**Cost.** About $0.0001 and 130 to 590 ms per failure on Jev; on llm about $0.001 to $0.003 and 1.5 to
2.8 s, CLI only. **Privacy.** Only the redacted excerpt and two spans leave; the raw log never does.
**Trigger:** the hook has logged at least 100 failing runs later followed by green, and J0 has run (a J0
failure does not block this site, which is text shaped). **Bar:** R3's hint bar on at least 100 settled and
sampled failures. **Kill:** if arm C fails to beat arm B by 10 percent on turns or correctness, drop tier
2; Jev then stays out of fathom's level 10.

## 6. The bench cell (TR-M1, TR-M2)

- **Setup:** one open source Go repository fathom did not write; 20 mutation-seeded test-fix tasks (a
  flipped comparison, an off-by-one, a dropped rounding step); n = 3 trials per arm.
- **Arms:** A control; B the hook, tiers 0 and 1, provider none; D raw level 10 (the upstream `ask_jev`
  with command mode and no fathom, ported as a bench-only tool that runs only in the bench sandbox because
  it executes commands). Arm C (B plus tier 2) joins when the tier 2 trigger fires. The bench widens to
  TypeScript and Python only if B passes.
- **Scale and cost:** about 180 runs, $90 to $360 of agent spend (Jev spend is pennies).
- **Metrics:** input tokens, turns and wall time to green; correctness (the mutant is killed, hidden tests
  pass, conservation shows nothing deleted, skipped or assertion-dropped); wrong-file edits.
- **Bar:** B uses at least 20 percent fewer tokens-to-green than A, the paired CI excludes 0, and
  correctness does not drop. **Kill:** if B misses, the line stays off by default (the tier 0 sensor stays
  if stall detection uses it); if D beats B, publish that and consider tier 2 sooner.

## 7. Open facts to verify before building

Whether PostToolUse(Bash) input carries an exit code, and whether a failing test command arrives as
PostToolUse or PostToolUseFailure (Q32, reading TR-V1). Answered 2026-09-29 for Claude Code 2.1.284
(`docs/bench/readings/test-run-hook/hook-facts.md`): no exit code on PostToolUse; an unpiped failing
command fires PostToolUseFailure with `Exit code N` leading `error`, a piped one (536 of 541 failing runs
scanned) fires PostToolUse, so both registrations stay. The daily test-failure volume (the sensor will
measure it).

## 8. Deferred and refused

- **Deferred (F42):** MCP exposure of test-run output, only on hook or CLI use by at least 5 percent of
  sessions, folded into `localize` (kind: test_output) or `test_impact`, tool count unchanged.
- **Refused:** a public `fathom triage` verb as the delivery; output rewriting (`updatedToolOutput`);
  command execution by fathom; an I12-style Stop-hook completion check (it duplicates the Stop hook and
  PC-22, and its certificate half has no data).

## 9. Readings

| Reading | Threshold |
|---|---|
| TR-V1 | the hook facts are verified against the current Claude Code: exit code presence, PostToolUse versus PostToolUseFailure for a failing command |
| TR-M1 | arm B uses at least 20 percent fewer tokens-to-green than arm A, paired CI excludes 0, correctness not lower |
| TR-M2 | arm D (raw level 10) is read against B and published either way |
| TR-M3 | hook p95 under 1 s on the owner's traffic, with the silent-skip rate named |
