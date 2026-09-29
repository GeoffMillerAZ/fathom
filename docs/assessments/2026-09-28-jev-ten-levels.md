# Ten levels of Jev, and what fathom does with each

Created: 2026-09-28 (the memo and the idea board); updated 2026-09-29 (the eight traces, the exploration
of level 10 through fathom); summarized into the repository 2026-09-28.
Status: a reading of a public video series and two public repositories, read against fathom at commit
`71d101db` (memo) and `177fd297` (traces). Every saving named below is a hypothesis until its bench
runs. The decisions it drove are ADR-056 (judgment at the port, proof first), `docs/vision/40-judgment-and-
exploration-maps.md`, goals G27 to G32, futures F34 to F49 and the plans TR-, JP-, JF-, JT-, XM-, CA-, WS-.
Readable forms: artifacts 6Z6EdpiYXq4cRf92Ywx8n4 (Jev Ideas for Fathom) and F3oEGQ7jPuARmyhVZewJva (Ask Jev
with Fathom).

## Verdict

Can fathom use level 10 (agentic Jev) powerfully and keep Jev optional? Yes to both. The power comes
from computing the answer first and asking Jev only for the judgment that is left. Most of what a level
10 agent asks (did the run pass, which tests failed, which changed symbols they reach) has an exact
answer fathom already computes with no model. Fathom's own logs show that tools an agent must choose to
call go unused, so the asking moves into a hook that fires on every test run: **fathom's level 10 is a
test-run hook** (tiers 0 and 1, provider none, opt-in, default-on only after a bench that includes raw
level 10 as its own arm). Every new Jev feature is its own opt-in per repository, and every one keeps a
working path without Jev. Nobody has measured Jev's accuracy on code.

**Not claimed:** that Jev reads code well (*unmeasured*), that any of this saves tokens (*hypothesis*,
benches pending), or that a judgment is proof (it never is, however good the model becomes: I59).

## What Jev is, in our words

Jev is a fast, cheap service from TypeSafe. You send a state (text or JSON) and typed questions and get
typed answers, each with a confidence your code can compare to a threshold. It is not a chat model and
cannot write text; a choice answer is always one of the options you wrote, so it maps straight onto a
code path.

| Question type | You ask | You get back |
|---|---|---|
| noul | A yes or no question, with optional wording for each side | The probability of yes, 0 to 1 |
| choice | Pick one of up to 255 options you wrote | A probability for every option, plus a confidence |
| score | A position on a 2 to 10 level scale you describe | A score that can land between levels, plus probabilities |

## Sources and the provenance rule

| Source | Read | Licence and use |
|---|---|---|
| The video "10 Levels of Jev for Agentic Engineers", about 35 minutes | 2026-09 | The product name is transcribed as Jev, Jeb and Jeev. Every speed and price figure from it is the presenter's spoken *claim* |
| `disler/ten-levels-of-jev` (the video's repository), level 10 in `apps/ten-levels/extensions/ask-jev.ts` | commit `777adaf`, 2026-09-27 | MIT, copyright 2026 IndyDevDan / AgenticEngineer.com; notice kept where code, prompts, questions or thresholds are reused |
| `disler/self-compact-pi-agent` (the prior week's video, the base for level 7) | commit `576fe4a` | MIT |
| Pointers only, not read: `7hemas7er/jev-hooks` (MIT, a Claude Code commit reviewer driven by Jev-style typed decisions), `Akramovic1/jev-pilot` (licence listed as other, a Claude Code plugin that steers reasoning effort, sub-agent model and skill) | | not used |

**The provenance rule (owner instruction, 2026-09-28).** Summarize the material in our own words, with no
long quotes. MIT allows code, prompts, questions and thresholds to be reused with the copyright notice
kept; the branding, the images and the paid-course marketing at the end of the README are not for reuse.
**No Claude Code hook exists in either repository:** every hook is an extension for the pi coding agent,
run over `pi --mode rpc`. The ideas port to fathom's hooks; the code does not (the only Claude Code files
in the repositories are two slash commands and a skill). The wire contract is plain HTTP with a state and
typed questions; the OpenRouter alpha endpoint is the only path the repository says it verified live, and
the direct TypeSafe path is untested without a key.

## The ten levels in our words, with fathom's verdict for each

Levels 1 to 5 call Jev from ordinary code. Levels 6 to 10 put Jev inside a coding agent, first as hooks
the agent never sees, then as tools it chooses to call, until it writes the questions itself.

| Level | The idea | Verdict | What fathom does |
|---|---|---|---|
| 1 Single decisions | One yes or no question steers plain code; your code picks the confidence line | deferred | An injection screen for third-party text waits until fathom serves such text by default, or an injection incident occurs; it can only flag (F47) |
| 2 Multiple choice | Several pick-one questions in one call | already there | The decision port already speaks choice, score and yes or no (ADR-042); no idea of its own |
| 3 Composite scoring | One score per factor on a scale you define, weights kept in your code | experiment | Preflight difficulty is tested against a plain regression (WS-M4); a Jev input to the risk band is refused because risk needs cited reasons (ADR-051, ADR-055) |
| 4 Confidence gating | Act only when confidence clears a bar, else ask a human | refused | Gating a command on Jev is refused: the harness owns blocking (P11, ADR-029 section 5.3, I14). The bar idea lives on as the calibration bars |
| 5 Intent and model routing | One cheap decision placed in front of expensive things | accepted as a correction | Fathom's own `route` site failed calibration (45.7 percent right at confidence 0.9 or more), so it leaves the default site list (JF-7); re-admission needs 100 replayed prompts against R3 (JF-8) |
| 6 Guardrail hooks | Jev checks each tool call or result inside a hook; the agent never sees the check | refused | Blocking commands or writes is refused; screening results for planted instructions waits (F47) |
| 7 Should I compact | After each turn, four questions decide whether context has moved on | experiment | Only the no-model part is tested: carry the computed working set across a compaction (CA- rows, G31). When-to-compact advice waits (F38) |
| 8 Cheap reads | Ask a question about one file and get a typed answer; the file never enters the context | deferred | Judged predicates over spans the graph picks, as a CLI verb (`fathom ask`, F34); waits for J0 |
| 9 Files at scale | The same question to many files in parallel; code trims the list | deferred | Judged labels on changed units only (F37); waits for J0, RT-M1 and a named consumer's own bench |
| 10 Agentic Jev | The agent gets one tool and writes its own questions; code runs the command and reads the files; the agent sees only the answers | accepted | Fathom's version is the test-run hook, computed first (TR- rows, G27); Jev joins later for the failure kind (F35, deferred) |

## Level 10 in fathom, and where it differs from the demo

As demonstrated: the agent has one tool, `ask_jev`; it passes a short state, up to 20 file paths or a
command, plus its own typed questions; plain code runs the command, reads the files, builds one state and
makes one call; the agent sees the answers and never the files or the output; a nudge in the system prompt
tells it to prefer the tool over reasoning. In the demo the agent, told the tests were red, classified the
run a code bug at 0.99, fixed a rounding step, scored its own diff and confirmed green in three calls for
$0.000084 (*claimed* by the presenter).

| Question | Level 10 | Fathom |
|---|---|---|
| Who picks the rows | The agent lists up to 20 paths or a command | The graph picks them exactly: callers, blast radius, rule scope, tests reaching, plan anchors, a diff. Default cap 60 rows, at most 255 |
| What the model reads | Whole files up to a budget | A span plus context the graph assembles, about 2k tokens at most; twelve files in fathom's own tree exceed 60 KB and the largest sits at the 32k state cap |
| Where options come from | The agent writes them | Index facts, always with a none exit: candidate symbols, the repository's own layer names, a fixed failure-kind list |
| Repeat questions | Every call is fresh | Cached by question template plus the content hash of the span, stamped with provider and model version; change the code and the old answer stops being served. In-process now; a durable cache waits for RT-M1 |
| What comes back | Bare answers with a confidence | A receipted table: key, span, unit hash, computed or judged, answer, probability, band, template version, calibration state, cache hit; a pruned list with reasons; cost with its source |
| When the ask happens | When the agent decides to call the tool | At a hook on the repository's declared test command; for agent-written questions, a CLI verb plus a lazily loaded skill, and no new MCP tool |

**An honest difference.** Level 10 saves tokens by hiding command output from the agent. Fathom does not
rewrite tool output (refused, R-12). Any saving from fathom has to come from fewer search and read turns,
and the bench measures exactly that.

**Why a hook beats a tool the agent must choose (*measured* in fathom's own logs).** Out of about 98,000
MCP calls, `localize` had 40, `test_impact` 103 and `discoveries` 13; a recall tool built the same way,
anchored memory, ended with no memories and was deleted (ADR-053 decision 2). Moments the harness forces
did work: 584 sub-agent starts, 550 pre-compact pointers written by a hook and 452 handoff notes agents
filed, over about four days, and 143,000 hook events of all kinds. The level 10 repository is candid about
the same limit: it nudges the agent in the system prompt, and the agent still decides.

The demo's questions, answered through fathom: "did all the tests pass" is a parsed three-way result (pass,
fail or unknown; pass only when the runner's final summary covers every package); "why did it fail" is the
failing tests, panics, frames and the changed symbols each failing test reaches (the failure kind as a
judgment is tier 2, deferred); "where should I look" is `localize` and `test_impact` over the working-tree
diff; "is my own fix risky" is conservation's facts (a deleted or skipped test, lost assertions, a changed
golden file; it does not catch an expected constant edited to match buggy output), and a model score as
an input to risk is refused; "confirm it is green" is the next parsed run, or `fathom verify` for plan rows.

## Facts corrected by the research pass

These replace figures and claims in earlier notes (recorded in vision 40 section 7 and ADR-056):

- The deterministic core already exists: `localize` accepts a stack trace or a failing test and
  `test_impact` exists; what is missing is the hook that delivers them at a test run.
- There is no certificate history (0 live certificate declarations); stdin mode never sees the exit code,
  so "all passed" is three-way.
- Conservation's reach is narrower than briefed (it catches deleted, skipped and assertion-dropped tests,
  not an edited expected constant).
- The rule-outcome events do not measure compliance: `complied` fires whenever the top claim id changes,
  `violated` whenever it repeats (O120, JF-1).
- Corrected cost and speed figures (*measured* in the decision log unless marked):

| Figure | Value | Grade |
|---|---|---|
| The decision log's real Jev use | 2 requests (64 and 56 questions); 44,072 tokens for 60 candidates; about $0.000031 per candidate | measured |
| Latency on Jev | 421 to 587 ms at about 22k tokens of state (130 to 375 ms only for small states) | measured |
| Limits that bind before token cost | 20 requests per second; a 32k state cap (about 14 rows of 2k tokens) | measured |
| `llm` versus Jev per call | 17 to 41 times the cost; 1.5 to 2.8 s a call; 3 of 6 timed out at 3 s; it saves context, not dollars (40 rows at 2k tokens is about $0.09, the same as the frontier reading it replaces) | measured |
| The corpus at `71d101db` | 2,493 Go files (21.1 MB); one label pass costs $0.22 to $0.27; about $0.53 at symbol level (25k symbols, about 21 minutes at 20 requests per second); about $3.6 fleet-wide | estimate |
| J0 plus its smoke test | about $0.25 of provider spend, 1 to 2 days, 50 to 80 owner labels | estimate |
| Failure kind, one failure | about $0.0001 and 130 to 590 ms on Jev; $0.001 to $0.003 and 1.5 to 2.8 s on `llm` | estimate |
| `ask` over 40 rows | about $0.00084 on Jev, about $0.033 on `llm`, $0 on a cache hit | estimate |
| Agent runs in the benches | $90 to $360 for the level 10 cell, $110 to $450 for the 40-handlers cell, $180 to $720 for traversal; Jev spend across the whole program is under $1 | estimate |
| Speed and price in the demo repository and video | about 300 ms; three calls for $0.000084; $16.80 versus $3,500 per million calls (README); about $20 versus $11,000 for millions of calls (video) | claimed by a vendor |
| Reading 40 spans puts 16 to 20k tokens in a frontier context | the counterfactual behind every `ask` saving | hypothesis |

Labels are scarce, not tokens: the design needed about 700 to 1,000 human judgments while the owner's
lifetime output so far is 86 feature-map verdicts and 0 claim verdicts (*measured*), so labels go to one
template (`failure_kind`) at a time (R4) and arrive in batches of ten through the one-key verdict with
provenance (human, frontier, computed). A judgment memo on Jev saves under a cent per query, so it does
not justify its memory footprint; a memo matters only on the `llm` provider.

## Measured Jev results in fathom

| Reading | Result | Meaning |
|---|---|---|
| The `route` site's calibration | 45.7 percent right at confidence 0.9 or more (16 of 35), no cutoff better than 58.5 percent, non-monotonic | the site leaves the default list; a fitted map cannot repair a non-monotonic failure |
| The obligations within-tier re-rank A/B (O113) | paired mean -0.19 (95 percent interval -0.375 to -0.062), 3 tasks worse and 0 better, n = 2 trials | `rank_within_tier` defaults to false |
| Routing to `localize` on the LCA bug-report benchmark | right-file rate 4.6 percent to 23.4 percent (a small sample: 2 tasks better, 0 worse) | routing helped where the question really was a localization |

Cheap-model judgments are therefore mixed: each site needs a passing calibration and an A/B before
anything is served. Why pruning needs the lower end of a range: the route check used 41 held-out prompts,
and even 38 of 41 right (0.927) has a Wilson lower bound of about 0.81, so 41 prompts cannot establish
0.9. A run of 96 of 100 clears the pruning bar (lower bound 0.902) and 90 of 100 (0.826) does not.

## The eight traces (Ask Jev, graded 2026-09-29)

Each trace has seven steps: the ask, the route, the rows, the options, the provider call, the table that
comes back, and what the agent does next. A model touches at most one of the seven, and in the first
trace none. Names in the traces are illustrative except where the records give them.

| Trace | Verdict | Model | Trigger or gate | Recorded in |
|---|---|---|---|---|
| The test-run hook, tiers 0 and 1 | accepted, opt-in | none | Bench: at least 20 percent fewer tokens-to-green than control, paired interval excludes 0, correctness does not drop; kill leaves the line off; if raw level 10 beats the hook, publish that | ADR-056 decision 6; TR- rows; G27 |
| Failure kind (tier 2) | deferred | Jev or `llm` | the hook has logged at least 100 failing runs later followed by green, and J0 has run; bar 90 of 100 right at the shown confidence; `test_expectation_outdated` never shown until it clears the pruning bar (it licenses editing a test to match buggy output) | F35; TR-D1 |
| Anchor proposals for foreign specs ("Add retry to the payment client") | accepted, behind a gate | Jev or `llm`, gated | SI-M4: on at least 60 foreign tasks with human-chosen anchors the top choice is right 9 times in 10 at the confidence shown, else the drop is recorded; the masked smoke test I3 (1,346 plan rows, 8 candidates) never counts toward SI-M4 | ADR-049 decision 7; SI-17, SI-M4; JP-M2 |
| A non-security showcase predicate (handlers that return an unbounded list) | deferred | Jev or `llm` | J0 passes and beats the lexical baseline, then the 40-handlers cell (3 corpora, 5 questions, 3 trials); kill under 40 percent fewer tokens than control at recall and precision within 0.05; the property must not be security | F34; JP-M3 |
| Doc-drift triage | deferred | Jev, later | doc linking ships (it has) and more than 20 spans are flagged per week; drop if 20 or fewer for 90 days; drift means the target moved, nothing more | F46; KR-6, KR-7 |
| Review narrowing (which callers depend on a change) | deferred | Jev, later | a refactor frontier over 100 sites and a corpus to A/B a model ordering against caller-count order | F23; CV-51 |
| Migration site disambiguation (same concept as the moved sites) | deferred | Jev, later | CV-M7 (already published) and a JavaScript or Python rename corpus | F23; CV-52 |
| The security question ("which handlers check authorization") | refused | never served | no trigger named: a model never prunes on a security property; J0 measures such questions offline only | ADR-056 decision 13; R7; ADR-051 |

## What would be fantasy to say today

| Claim | Grade | Where it stands |
|---|---|---|
| Most of a level 10 agent's questions have exact answers fathom computes | accepted | the pieces exist (`localize`, `test_impact`, conservation); the hook that delivers them at a test run is not built |
| Tools an agent must choose to call go unused | measured | `localize` 40, `test_impact` 103, `discoveries` 13 of about 98,000 calls |
| A typed model reads code well enough to order or prune | unmeasured | J0 is the kill screen (Q31) |
| The hook saves at least 20 percent of tokens-to-green | hypothesis | the level 10 cell decides; not run |
| `ask` saves at least 40 percent at equal recall and precision | hypothesis | the 40-handlers cell decides, after J0; not run |
| Fathom adds something to raw level 10 | hypothesis | raw `ask_jev` runs as its own arm; the result is published either way |
| A judgment is proof | refused | I59: never served as a code fact, sets no status, blocks nothing |

## The idea board: 34 ideas, each with one verdict

The board numbered ideas A (accept), E (experiment), D (defer) and R (refuse). Accept goes on the roadmap
now; experiment needs a measurement first, with a named bar and a kill criterion; defer waits for a named
trigger; refuse conflicts with a rule or adds nothing over a computed answer.

| Idea | Verdict | Landed in |
|---|---|---|
| A1 Fix the rule-outcome instrument: rename `violated` and `complied` to `still_served` and `top_changed` | accept | JF-1, JF-2, JF-3, JF-22; O120; ADR-056 decision 8 |
| A2 Opt-in per site for every new decision site | accept | JF-4 to JF-6; I60; ADR-056 decision 3 |
| A3 The test-run hook, provider none | accept | TR- rows; G27; ADR-056 decision 6 |
| A4 J0: does Jev read code at all (a one-way kill screen against a grep rival) | accept | JP-M1, JP-M2; G28; Q31 |
| A5 Correct the design notes that assumed Jev writes text | accept | JF-16 to JF-18 (proven) |
| A6 Turn the `route` model call off by default | accept | JF-7, JF-8 |
| A7 Compute the `certify_suggest` strength ladder | accept | JF-9 |
| A8 Calibration readings, not fitted maps | accept | JF-10 to JF-12, JF-23, JF-M1; G29 |
| E1 Carry the working set across a compaction | experiment | CA- rows; G31 |
| E2 Sub-agent handoff maps (measure re-derivation first) | experiment | XM- rows; G30 |
| E3 Stall detection (spinning wheels), deterministic first | experiment | WS- rows; G32 |
| E4 Preflight difficulty: Jev against a fitted baseline | experiment | WS-M4 |
| D-1 Tier 2 on the test-run hook: failure kind | defer | F35 |
| D-2 `fathom ask`: judged predicates over spans | defer | F34 |
| D-3 Question-directed traversal | defer | F36 |
| D-4 Files at scale for semantic layers (level 9) | defer | F37 |
| D-5 When-to-compact advice | defer | F38 |
| D-6 Fitted recalibration and auto-disable | defer | F39, JF-15 |
| D-7 The judged-telemetry sampler, queue and judgments table | defer | F40; Q33 |
| D-8 Guard proposals from rule outcomes | defer | F41 |
| D-9 MCP exposure of test-run or `ask` output (at 5 percent of sessions) | defer | F42 |
| D-10 Smaller deferrals: ADR-to-rule pairs, doc-drift triage, the injection screen, skill suggestion, review narrowing, migration disambiguation | defer, each with its own trigger | F45, F46, F47, F48, F23 |
| R-1 A public `fathom triage` verb, or `ask`'s operator grammar as a new verb | refuse | ADR-056 decision 13 |
| R-2 Judged rule compliance | refuse | JF-X1 |
| R-3 Jev for answer usefulness, config-custodian proposals and product digests | refuse | JF-X2 |
| R-4 Computable labels as Jev work (generated files, config versus code, public API, ownership, I/O reach, TODO admission) | refuse | R1; JF-X3; ADR-056 decision 13 |
| R-5 Model-judged security properties as a served signal | refuse | R7; JF-20 |
| R-6 A model-judged Stop-hook completion check | refuse | ADR-056 decision 13 |
| R-7 Verdict proposals for the feature map (deferred by the owner on 2026-09-29 instead) | refuse, then deferred | F51 |
| R-8 Commit classification | refuse | ADR-056 decision 13 |
| R-9 Jev inputs to the risk band | refuse | ADR-051, ADR-055 |
| R-10 Jev gates on destructive commands or secret writes | refuse | P11, I14 |
| R-11 Jev rewrites and query reformulation | refuse | JF-16, JF-17 |
| R-12 Output rewriting and command execution by fathom | refuse | ADR-056 decision 13 |

## Guardrails (why Jev stays optional and a judgment never becomes proof)

Three providers behind one interface: `none` (the default; nothing leaves the machine; every feature runs
its deterministic path), `llm` (a chat model, for the CLI and background work only, never inside a hook)
and `typesafe` (Jev; fast enough for a hook through a daemon endpoint; availability changes without notice:
signups were paused on 2026-09-22 and OpenRouter access is alpha, so nothing depends on it). A site
calibrated on one provider shows as uncalibrated on another. A missing key, a timeout, a 429 or 529,
refuse mode or safe mode each produce one disclosed line and then the deterministic path.

Authority: a judgment never asserts a code fact, sets a status or blocks, prunes silently, judges a
security property as a served signal, or decides what is computable. Optionality: never on by default,
never on because a repository set a provider, never a broken feature when it fails, never a default MCP
tool. Evidence: never an uncalibrated confidence shown as calibrated (a hint needs 90 of 100 right at the
shown confidence, pruning needs a Wilson 95 percent lower bound of at least 0.90), never an unpinned model
(pin `jev-1.13.0`, log the resolved model), never proof (a hook row is not an attested run), never written
text. These are ADR-056 rules R1 to R8, principle P24 and invariants I59 to I61.

## What it drove

| Decision | Where |
|---|---|
| Judgment at the port, proof first; the decision-model bet; rules R1 to R8; per-site opt-in | ADR-056; P24; I59 to I61 |
| Goals G27 to G32 and futures F34 to F49 | `docs/intent/scope.md`; `docs/intent/futures.md` |
| The test-run hook, J0, calibration, carry-across, maps, stall detection and preflight plans | TR-, JP-, JF-, JT-, CA-, XM-, WS- |
| The provenance rule for third-party summaries | this file; `docs/assessments/README.md` conventions |

## Sources

The video and the two repositories named under "Sources and the provenance rule", read as stated; the
research memo "Jev research: final decisions" (2026-09-28), read against fathom at `71d101db`; the
owner-architect notes of 2026-09-29; fathom's decision log, call log, hook log and readings (`docs/bench/
readings/decision-port-calibration-2026-09-24.md`, `decision-port-ab-2026-09-23.md`).
