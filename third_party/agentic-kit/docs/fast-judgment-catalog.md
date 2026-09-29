# Fast judgment models: use cases and how to calibrate each

Companion to [Using fast judgment models effectively](fast-judgment.md); read its sections
3 (calibration), 6 (schema) and 7 (anti-patterns) first. Every entry follows one shape:
the idea, the question and options, the deterministic baseline it must beat, how to
calibrate, the kill criterion, and what authority the answer may have. Written 2026-09-29.

**Evidence tags.** [vendor] provider claim. [demo] shown in IndyDevDan's public
demonstrations, not reproduced here (attribution in the guide, section 10). [reading] a
consuming project's measurement on its own data, small N. [plan] a bar or budget one
consumer wrote down for a study it had not run when this was written: a design to copy
and adapt, not a result.

## At a glance

| # | Use case | Measure first | Authority the answer may earn | Evidence |
|---|---|---|---|---|
| A | Semantic predicates over a planned set | a screening study against a lexical baseline | order and filter a list an agent reads; prune only past the strict bar | [plan] |
| B | Files-at-scale labels with hash-keyed caching | which labels are not already computable | a stored column, only where a saved question needs it | [reading][plan] |
| C | Sampled judged telemetry to tune a tool | whether an event join already answers it | proposals a human accepts | [plan] |
| D | Question-directed exploration | provider-none traversal against plain search | reorder the weak tail; never prune strong candidates | [demo][plan] |
| E | Compaction advice | a transcript scan of re-reads, no model | advise only; the harness compacts | [demo][plan] |
| F | Routing (intent, agent, model) | regret against fixed and rule baselines | choose among declared paths in calibrated bands | [demo][reading] |
| G | Triage (tickets, failures, evaluations) | a deterministic parse of the run | a labeled hint; never an edit licence | [demo][reading] |
| H | Review narrowing | misses in an audited "no issue" sample | narrow what a reviewer reads; never close a rule | [vendor][kit] |
| I | Smaller uses | each its own baseline | advisory | mixed |
| J | Not model work | not applicable | none | n/a |

Sequence for a new project: build the deterministic sensors first (they are needed by
almost every case below); run one screening study (A); calibrate a single template at a
time; open each next site only when its own trigger fires.

## A. Semantic predicates over a planned set

**Idea.** Your code picks the set by structure (a graph query, a search, a plan), then a
predicate is asked once per unit: "of these 40 request handlers, which validate their
input?" The model never picks the set and never decides whether a question is
computable.

**Shape.** One predicate per unit over a span plus the context your code assembles
(callee signatures, the rule text), each unit at most about 2k tokens, so roughly 14 per
call at the 32k state cap. Answer bands: yes, no, read-it-yourself.

**Baseline.** Lexical search of the span for the identifiers that would prove the
property.

**Calibrate (a kill screen first).** [plan]
- Spans: about 1,000, from your repository plus two open-source repositories that have
  typed call edges. Treat syntax-only edges as a distribution check, not truth.
- Free oracle: properties your index can compute exactly (calls the storage layer
  directly, performs network I/O directly, returns an error callers must handle). Run
  each span alone, then with callee signatures assembled; the difference is what your
  context assembly is worth.
- Residue: about 150 genuinely semantic units ("validates input"). A frontier model
  labels them; a human adjudicates every disagreement plus a 20% sample. Record
  provenance on each label.
- Pass: on directly visible properties AUROC at least 0.9 and recall at least 0.95 at a
  threshold that prunes at least half; AUROC at least 0.05 above the lexical baseline;
  context assembly lifts transitive recall by at least 20 points; residue precision at
  least 0.9.
- Budget written for it: cents of model spend, one to two days of harness work, and 50
  to 80 human labels.

**Kill.** Any miss, or a tie with the lexical baseline, means no code-reading site is
built. Sites over prose (failure excerpts, prompts, documents) are unaffected.

**Authority.** Ordering and filtering what an agent reads. Pruning only past the strict
bar in guide section 3. Never prune on a security property; measure those in the
screen and do not serve them. Output is a receipted table that lists what was pruned.

## B. Files at scale, with hash-keyed caching (level 9)

**Idea.** Ask the same small question of many files in parallel without reading any of
them. In the demonstration, code expands globs, drops dependencies, binaries and
oversize files, caps the list at the 255-option ceiling, makes one call per file, then a
second pass picks the best match. [demo] Asked which of 17 files bear on a rounding
bug, two came back relevant at 0.93 and 0.96 and one was opened. [demo]

**What to change.** Send spans, not files (some files exceed 60 KB and the largest
touches the 32k cap). Build options from facts. Key every answer by (template version,
content hash, provider, model), and re-ask only units whose hash or template changed.
Scope a pass to the footprint of a saved question (a working set) so that an
expensive column exists only where something needs it, and store it apart from proof.

**Cost, before deciding.** One consumer estimated a label pass over about 2,500 source
files (21 MB) at $0.22 to $0.27, symbol-level over 25,000 symbols at about $0.53 and 21
minutes at 20 requests per second, and fleet-wide symbol-level at about $3.6 and 2.4
hours. [reading] Tokens are not the constraint. What decides it is which labels are not
already computable, the request-rate cap, and accuracy.

**Refuse as model work** (all computable): generated or vendored files, config versus
code, public API, ownership, I/O reach, TODO detection. Compute them.

**Residue that may qualify:** test kind after signatures, build tags and fixtures have
classified most tests; membership in a concern; document type; architecture role, only
where your own architecture rules supply the options (results from a layered repository
do not transfer to one without layers).

**Calibrate.** Per label template, at least 100 labels; the strict bar when the label
reorders or prunes anything served by default. Start with the residue with the clearest
consumer, such as test kind.

**Gates before building.** The screen in A passed; the storage has a tenancy budget (a
fleet-sized memo was about 130 MB and should be disk-backed [reading]); a named
consumer's own metric, not "the ranking changed", judges the value.

## C. Sampled judged telemetry to tune a tool

**Idea.** Judge a small random sample of a tool's own outputs to measure what its event
logs cannot: was the top result relevant, was the hint useful. The verdicts feed tuning
proposals and the calibration sets of other sites.

**First ask whether a join answers it.** Whether an answer was useful can often be
read from events (did the agent search again right after, did the edit land in the
returned file). A join is deterministic and free; a model adds nothing there.

**Design.** [plan]
- Sample with a recorded inclusion probability so results can be weighted. Cap spend per
  day. Label the spend and exclude it from any A/B unit's cost.
- Anchor the judge with humans: a subsample gets a one-key verdict, in batches of about
  ten, with provenance stored.
- Weak labels: a later event (the fix landed in the reported file) is evidence, not
  truth. Oversample positives and have a human adjudicate them.

**Circularity risk.** A tool tuned to a model's taste drifts toward that model. Keep a
human-labeled holdout the judge never tunes on.

**Bar.** The judge agrees with humans at the hint bar on at least 100 items before its
output tunes anything. Tuning changes are deterministic proposals that a person
accepts; a proposal never turns on enforcement by itself, and model-judged text is never
written into tests.

**Kill.** If the deterministic join answers the question, do not build the judge.

## D. Question-directed exploration

**Idea.** To find where a defect or a concept lives, ask one relevance predicate per
candidate over a frontier your graph enumerates, then take the best. In the
demonstration this scouted a small repository in one parallel pass. [demo]

**Order.** Build the traversal with provider none first (bounded hops, static
ranking). Add the scorer only after the screen in A passed and the provider-none
traversal already beats plain agent search.

**Calibrate.** [plan] Paired runs on identical questions. Metrics: tokens to a correct
answer and recall of the true locations at the cap. Bars: provider none beats the
control by 25% on tokens with the interval excluding zero; the model beats provider
none by 15% at equal recall; strong candidates are never pruned, only the weak tail is
reordered.

**Delivery matters.** Agents rarely call optional tools. Deliver the result as a
prepared working set at a moment the agent cannot skip (a handoff or a hook;
[orchestrating agents](orchestrating-agents.md), section 1, describes shared
exploration maps), not as another tool.

**Kill.** No gain over the provider-none traversal: keep the deterministic traversal.

## E. Compaction advice (level 7)

**Idea.** After each turn ask four questions in one call: did the request switch gears,
did the last turn complete a unit of work, how much of the earlier work does the next
step need (a score), is the agent mid-way through a multi-step edit. Numbers stay in
code: context tiers decide when a notice, a recommendation or a request is even
possible, and the agent hears nothing until a tier is earned. A later choice picks the
turn where the live work begins and becomes the summary instruction. [demo] Its base is
a self-compacting agent that sees a context gauge, writes a note to itself at the
warning line, and is limited to the compact tool at the hard cutoff (defaults at 10%,
20% and 30% of the window). [demo]

**Reality checks.** Whether a hook can trigger compaction, and whether a pre-compact
event can inject context, depends on your harness and version; one consumer concluded from
its harness's documentation that neither is possible and that only a session-start event
after compaction can add context, and listed confirming that as its first step. [reading]
Verify yours. The act of compacting belongs to whatever
component owns compaction (in this kit, the evidence-preserving compactor of
[practices §1](practices.md#1-lossy-progressive-summarization)); a judgment can only
advise it.

**Measure first, with no model.** [plan] For the last two weeks of real sessions, take
the files read or edited in the 40 turns before each compaction and count re-reads of
them in the 20 turns after, split into re-read then edited within three turns
(legitimate) and re-read only (waste). Also record tokens and turns to the next proof
event (a passing test run, a verification, a checked-off plan row). Stop rule: if the
median compaction has fewer than two re-reads, or re-reads are under 10% of all reads
afterwards, there is nothing to fix and the current behavior stands.

**If waste shows.** First try a computed carry-across with no model: a working set of
at most about 1,200 bytes injected after compaction (dirty files and their changed
symbols, plan rows touched with their verification state, the last failing tests,
unresolved rules). Test by a seeded hash of the session identifier, at least 150
compactions per arm over at least seven days. Pass: re-reads of the pre-compaction set
fall by at least 30% with a bootstrap interval excluding zero and tokens to proof are
not worse. Otherwise remove it.

**When to add the judgment.** After the carry-across passes, and calibrated like a
routing site (F): it is the same prompt-classification shape.

## F. Routing (intent, agent, model)

**Idea.** One cheap decision in front of expensive things: which agent, which model,
which path. In the demonstration a choice over declared agents returned a browser agent
for a competitor-research prompt and a fast agent for a localized test fix, each with
a confidence. [demo]

**A caution from the field.** A routing site in one consumer was right 45.7% of the time
at confidence 0.9 or above and non-monotonic, and 41 held-out prompts could not have
proved 0.9 anyway (38 of 41 has a Wilson lower bound of 0.806). It was turned off by
default; re-admission needs a larger replay. [reading]

**Calibrate.** [plan]
- At least 100 real prompts replayed through 4 to 6 decomposed predicates ("needs a
  browser", "a localized change", "needs a desktop") with weights fitted on half and
  tested on the other half, against the strict bar. Fit only after enough labels; at
  about 100 a fitted correction is noise, and a non-monotonic site cannot be repaired by
  scaling.
- Baselines: fixed cheap path, fixed strong path, a deterministic task policy. Measure
  achieved outcome and regret against the best eligible observed path, plus missed
  escalations. Counterfactual runs only in an authorized, bounded study.
- Confidence in the choice is not evidence that the chosen path succeeds downstream.
  Route on task properties first; evaluate a direct choice as the alternative.
- A cascade (a cheap extractor, a verifier that estimates the probability something is
  wrong, an expensive escalation on a fired signal) follows the vendor's cookbook;
  calibrate the verifier's recall of true errors, since a missed escalation is the
  costly miss. [vendor]

**Kill.** A gain below the stated margin over the deterministic policy means the route
stays deterministic. Health- or cost-aware routing beyond ordered failover is a
future in this repository: [F7](intent/futures.md).

## G. Triage

**Idea.** Classify an incoming item cheaply: a support ticket (category and priority in
one call) [demo]; the kind of a failing test run; which evaluation cases deserve costly
adjudication.

**Failing-run triage, the worked case.**
- **Deterministic first.** Parse the runner output into pass, fail or unknown (pass only
  when the final summary covers every package; a piped run without an exit code is
  unknown), failing test identifiers, panics, file and line frames and build errors.
  Then name which changed symbols each failing test reaches. One line, nothing on a
  pass, no model. This alone answers the questions the agentic demonstration asked of
  its model.
- **The judged residue.** A choice over code bug, environment or setup, timing or
  flaky, test expectation outdated, other, over a redacted excerpt of at most about
  1.5k tokens plus the candidate and assertion spans. Cost was about $0.0001 and
  130 to 590 ms per failure on the native lane. [reading]
- **Guardrail.** Do not show "test expectation outdated" until it clears the strict
  bar. Shown uncalibrated it licenses editing the test to match buggy output.
- **Labels.** Where the eventual fix landed settles some cases weakly. Oversample
  positives and have a human adjudicate. Wait for at least 100 failing runs that later
  went green. [plan]
- **Bar and kill.** The hint bar on at least 100 settled and sampled failures; drop the
  judged tier if it does not beat the deterministic line by 10% on turns or correctness.
  [plan]

## H. Review narrowing

**Idea.** Turn a written review rule into a small program: decide scope, collect
evidence, evaluate atomic criteria, apply a deterministic decision table, and render the
rule's authored feedback with exact source links. The model answers only the semantic
criteria; escalate the remainder. See the kit's
[design](designs/typed-judgment-review.md) and [assessment](assessments/2026-09-21-jev-opportunities.md).

**Calibrate.** Freeze development and holdout by independent change. Include positive,
benign lookalike, missing-context, adversarial and uncataloged-defect cases. Compare the
same criteria on a native provider and on a structured-output LLM, an ordinary free-form
review, and deterministic-only checks. Report per-rule misses, actionable precision,
abstention, source support and localization, calibration, latency and total cost
including preparation, escalations and audit samples. Audit a sample of "no issue"
outputs and of non-escalated cases; a sampled audit records inclusion probabilities
and weights its estimates. Grade cascade outcomes against independent truth, not against
the escalated reviewer's answer.

**Cautions.** One passing child criterion never passes its parent rule; an aggregate
score cannot hide a required failure. The kit's live smoke saw both responses mark
uninformative captured text as satisfied where the criterion intended unknown, so test
applicability and sufficiency decisions explicitly. [kit]

## I. Smaller uses

| Use | Question shape | Baseline and calibration hint |
|---|---|---|
| Source support | a claim and a supplied span: supported, contradicted, insufficient | deterministic quote matching locates the span, the model judges support; label the result "judged support", never "verified" [vendor] |
| Passage relevance filter before an expensive reader | one predicate per retrieved passage | recall of truly relevant passages after filtering; false rejection must be visible [vendor] |
| Extraction verifier | per field: is this value absent from, or lifted from unrelated text in, the source | cheap extractor plus verifier; measure escalation recall [vendor] |
| Command, write and result guards (level 6) | reversible or destructive intent; credential in content; instructions aimed at an agent | one signal among several, never the only control; measure false allow and false refusal against the actual protected effect; the harness owns hard blocks [demo] |
| Stall classification | productive iteration or stuck, for ambiguous episodes only | detect deterministically first (the same test failing repeatedly, an identical call repeated); a red phase in test-first work looks like stagnation, so check candidates against abandoned sessions and later green runs |
| Documentation drift triage | order flagged spans by likelihood of real drift | deterministic order stays the default; worthwhile only at volume |
| Skill or tool suggestion | a choice picks the candidate; one predicate per candidate decides whether to suggest at all | choice is relative, predicates are absolute; use both on one shortlist [vendor] |
| Difficulty or weight estimates | a score for effort | features are numeric: fit a held-out classical model first; the fast model may refine only if it beats it by the margin in guide section 3 |

## J. Not model work

Do these in code, whatever the model's price: callers and reach; exported or public
surface; generated or vendored files; ownership; counting anything; dates and windows;
version comparison; arithmetic and thresholds; whether a run passed; whether a test was
deleted, skipped or lost assertions; scope and applicability of a rule. Also not model
work: writing, summarizing, renaming, rewriting a rule, reformulating a query (the model
picks among candidates that code or a generative model produced); proof, "done", cited
risk reasons and any security assertion served as fact; runtime facts that are not in
the text, such as performance or production usage.
