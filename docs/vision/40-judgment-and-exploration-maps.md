# 40 - Judgment at the port, proof first: level 10 through fathom, and maps that arrive

Status: accepted 2026-09-29 (ADR-056). Owner directive of 2026-09-29, after a research pass over the
ten-levels series (level 10, agentic Jev) and fathom's own decision port: use level 10 powerfully in
fathom, keep Jev optional, and land every decision in the durable records. Plans:
`docs/plans/judgment-foundations.md` (JF-), `docs/plans/test-run-hook.md` (TR-),
`docs/plans/judged-predicates.md` (JP-), `docs/plans/judged-telemetry.md` (JT-),
`docs/plans/exploration-maps.md` (XM-), `docs/plans/carry-across.md` (CA-) and
`docs/plans/work-signals.md` (WS-). Designs: `docs/design/judgment-foundations.md`,
`docs/design/test-run-hook.md`, `docs/design/judged-predicates.md`, `docs/design/judged-telemetry.md`,
`docs/design/exploration-maps.md`, `docs/design/carry-across.md`, `docs/design/work-signals.md`. Goals
G27 to G32 in `docs/intent/scope.md`. Builds on the decision port (`32-decision-port.md`, ADR-042), the
proof-layer roadmap (`38-proof-layer-roadmap.md`) and continuity threads (`21-continuity-threads.md`).

## 1. The question

The owner asked two things. Can fathom use level 10 of the ten-levels series (agentic Jev) powerfully? And
can it do so while Jev stays optional? Around them sat five adjacent ideas: a planner-and-predicate way to
ask a fast typed model about many code units (fathom as the planner), a judged-telemetry flywheel,
question-directed exploration, exploration maps shared from planner to sub-agents, and files-at-scale
semantic columns (level 9), plus whether a compaction advisor (level 7) belongs in fathom.

## 2. What level 10 is (in our words)

The agent gets one tool. It hands over a short state, up to 20 file paths, or a command. Code runs the
command, captures the exit code and output, assembles one state, and asks Jev the agent's own typed
questions (yes or no, a choice, a score). The agent sees the answers and never the files or the output, and
a system-prompt nudge tells it to prefer the tool to reasoning. In the public demo it classified a red run
as a code bug, fixed a rounding step, scored its own diff and confirmed green, in 3 calls for a few hundredths
of a cent. The source is a public MIT repository (`apps/ten-levels/extensions/ask-jev.ts`).

## 3. The answer: yes to both, from a different place than the demo

**Proof first.** Most of what that agent asks has an exact answer fathom computes with no model: did the run
pass, which tests failed, which symbols they reach and which of those changed in the working tree, where the
frames land, whether a test was deleted, skipped or lost assertions. Jev's 0.99 on "did all tests pass"
becomes a parsed three-way answer (pass, fail, unknown).

**Delivered at a moment the agent cannot skip.** Level 10 depends on the agent choosing to call a tool, and
fathom's own logs show chosen tools die (about 98k MCP calls: localize 40, test_impact 103, discoveries 13).
Hooks and threads survive. So fathom's level 10 is a **test-run hook**: when the agent runs the repository's
declared test command, one line names the failing tests and the changed symbols they reach, and nothing is
said on a pass. It uses no model and sends nothing off the machine.

**Judgment only on what computation cannot answer,** through the existing decision port and only after a
measurement: first the failure kind of a test run, later agent-written predicates over spans the graph picks,
and only if a kill screen (J0) shows a typed model can read code at all.

**Measured against raw level 10.** The bench includes the upstream tool as its own arm. We learn whether fathom
adds anything and publish the result either way.

**An honest difference.** Level 10 saves tokens by hiding command output from the agent. Fathom does not
rewrite tool output (refused). Any saving comes from fewer search and read turns, and the bench measures it.

## 4. How Jev stays optional: per feature, not only per repository

- **The default is `none`.** Nothing leaves the machine and every feature works on its deterministic path.
- **Every new decision site is opt-in per repository, even when a provider is set.** Today a site added the
  usual way turns on in every repository that sets a provider. Fixing that is a real change, not a restatement.
- **Three providers.** `none`; `llm` (CLI and background work only: slow, 17 to 41 times Jev per call, it saves
  context rather than dollars); `typesafe` (fast enough for a hook through the daemon).
- **Calibration is per (site, provider, resolved model).** A template calibrated on Jev reads as uncalibrated
  on llm.
- **Failures degrade cleanly:** a missing key, a timeout, a 429 or 529, refuse mode or safe mode each produce
  one disclosed line and the deterministic path.
- **No dependency on Jev.** No feature is built whose value exists only if Jev exists. The decision-model bet
  (principle P24): fast typed-judgment models keep improving, fathom rides that curve without redesign, and
  judgment never replaces proof for anything fathom can compute.

## 5. The programs and their verdicts

| Program | Verdict | Plan | What it needs before anything is built |
|---|---|---|---|
| The rule-outcome instrument fix, per-site opt-in, route off by default, a deterministic certify ladder, calibration readings, doc corrections | accept | JF | nothing; corrections and guards |
| The test-run hook, tiers 0 and 1, provider none | accept, opt-in, default-on only after its bench | TR | the hook facts (exit code, failure delivery) verified |
| J0: does a typed model read code? plus the I3 masked smoke test | accept (one-way kill screen) | JP | the pre-registration committed first |
| `fathom ask` (fathom as planner, a model as a semantic predicate) | defer | JP | J0 passes and beats a lexical baseline |
| The judged-telemetry flywheel, reframed as a label economy | accept the labels and readings; defer the sampler and table | JT | the sampler waits for RT-M1 |
| Question-directed exploration (traversal) | defer | JP | the maps A/B passes, then provider none first |
| Exploration maps shared from planner to sub-agents (composable, live, discoverable) | experiment, measurement first | XM | a 14-day reading shows at least 15 percent re-derivation |
| Files at scale as semantic columns (level 9) | defer | JP | J0, RT-M1 and a named consumer's own metric |
| The compaction advisor: carry-across only | experiment with a stop rule; advice deferred | CA | SessionStart(source=compact) verified, Reading 0 shows waste |
| Stall detection, estimated weights, cost per weight point, the preflight model against a fitted baseline | accept detection and weights; nudge and model site are experiments | WS | WI-16 to WI-19 for the baseline |

## 6. The strongest finding did not need Jev

The rule-outcome events that governance learning reads (`violated`, `complied`) measure scope persistence and
churn, not compliance, for any prose rule: `violated` means the same claim topped the file's served set on
the previous turn. Every reading built on them is mislabeled. The fix is a rename and a re-label
(`still_served`, `top_changed`) and costs nothing (JF-1).

## 7. Corrections adopted from the critics

The deterministic core already exists (localize takes stack traces and failing tests; the Stop hook and PC-22
verify claimed rows), so the new value is delivering it when a test run fails. There is no certificate history
on this machine, so "changed since the last attested pass" is dropped for the working-tree diff. Stdin mode
never sees the exit code, so "all passed" is three-way. Conservation catches deleted, skipped, assertion-dropped
and golden-changed tests and does not catch an expected constant edited to match buggy output. Two design notes
assumed Jev generates text; it cannot. The limits that bind before token cost are the 20 requests per second cap
and the 32k state cap. Labels are scarce: a full design needs 700 to 1,000 human judgments and the owner's
lifetime output is 86 feature-map verdicts and 0 claim verdicts. A judgment memo saves under a cent per query
on Jev, so it is not worth its footprint there.

## 8. Program rules

R1 structural routing (a model never decides whether a question is computable); R2 new sites are opt-in; R3
Wilson-interval bars (a hint needs 90 of 100 at the shown confidence; pruning, gating or reordering a default
list needs a 95 percent lower bound of 0.90; a free-form question is shown uncalibrated and never prunes); R4
one labeled template at a time; R5 no table, durable memo, sampler queue or background judge before RT-M1
reads PASS; R6 no new MCP tool; R7 a judgment never asserts a code fact, prunes silently, sets a status or
blocks, and never prunes on a security property; R8 pin the model and mark a site for recalibration on any
move. The full text is in ADR-056 and `docs/design/judgment-foundations.md` section 3.

## 9. Sequence and budget

0. The quick fixes: JF-1 to JF-9 and the doc corrections.
1. J0 with its lexical baseline, plus the I3 smoke test: about $0.25 and 1 to 2 days.
2. The test-run hook, tiers 0 and 1, then its bench cell with the raw level 10 arm: about $90 to $360 of agent
   spend.
3. Readings that cost nothing: carry-across Reading 0 and the maps re-derivation scan (transcript scans), stall
   detection once the sensor has data, the preflight baseline when WI-16 to WI-19 exist, calibration readings as
   soon as any site has labels.
4. After RT-M1 reads PASS, each on its own trigger: tier 2, `ask`, the memo, the ai-derived graduation ADR,
   files at scale.

Jev spend is under $1 across all of it; the agent runs are what cost money. Every savings figure is a hypothesis
until its bench runs (P12).

## 10. What we do not claim

That a typed model reads code well (unmeasured); that any of this saves tokens (benches pending); that a
judgment is proof (it never is).

## 11. Open facts to verify before building

Does PostToolUse(Bash) carry an exit code, and does a failing test command arrive as PostToolUse or
PostToolUseFailure? Does SessionStart with source `compact` fire and accept additional context? Does the
SubagentStart input carry the sub-agent's prompt? Are RC-28's 591-byte rows boilerplate? Do the U6 features
(WI-16 to WI-19) exist? (They are open rows.) What is the daily test-failure volume? Provider availability moves
without notice (Jev signups were paused on 2026-09-22 and its OpenRouter access is alpha), which the design
answers by construction: nothing depends on it. These are tracked as questions Q31 and Q32 in
`docs/intent/questions.md`.

## 12. Kill criteria in one place

| Item | Kill |
|---|---|
| Test-run hook | arm B under 20 percent fewer tokens-to-green than control, or a correctness drop: the line stays off |
| J0 | any miss, or grep tying the model: no code-reading site is built |
| `ask` | under 40 percent fewer tokens than control at recall and precision within 0.05: drop the judged tier |
| Traversal | provider none under 25 percent better than control: drop it; the model under 15 percent better than none: drop the scorer |
| Maps | re-derivation under 15 percent, or the A/B misses 25 percent fewer tokens: no handoff channel |
| Carry-across | Reading 0 stop rule, or re-reads not down 30 percent: keep the deliberate silence |
| Stall nudge | episodes do not shorten or cost per weight point does not drop: keep detection, drop the nudge |
| Preflight model | under 0.05 AUROC over a fitted baseline: the fitted model serves |
| Judged template | agreement under 0.8 after one revision: drop the template |
