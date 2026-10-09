# Using fast judgment models (Jev and its kind) effectively

Practical guidance for any project that wants a cheap, fast model to answer bounded
questions inside its own code or agents. Written 2026-09-29 from the kit's own
[decision port](plans/decision-port.md) and [Jev assessment](assessments/2026-09-21-jev-opportunities.md),
the vendor's public documentation, IndyDevDan's open-source demonstrations, and one
consumer's field readings. Use cases and how to calibrate each are in the
[catalog](fast-judgment-catalog.md); the ledger entry is [practices §25](practices.md).

**Evidence tags used below.** [vendor] is the provider's own claim. [demo] is what a
public demonstration showed, not a measurement we reproduced. [kit] is verified in this
repository. [reading] is a measurement by a consuming project on its own data, small N,
not transferable to yours. Unmarked statements are method, not measurement.

## Verdict first

- **What it is.** A fast judgment model (the vendor calls the class "System One") takes a
  piece of text and typed questions and returns typed probabilities: yes/no, one of N
  options, a position on a rubric. It cannot write. Jev, from TypeSafe AI, is the first
  widely visible one. [vendor] It lists at $0.042 per million input tokens with free
  output. [vendor] The public demonstrations show answers in well under a second. [demo]
- **Where it pays.** Bounded judgments you would otherwise do with a brittle regex or an
  expensive chat call, at a volume where the price difference decides whether you try:
  gates, labels, scores, routes, and a cheap first pass in front of an expensive reader.
- **Where it does not.** Anything your code can compute exactly, anything that needs
  proof, numbers, dates, counting, generated text, or a lone security control.
- **The one rule.** A model's confidence is a claim, not a measurement. Before an
  answer changes what a system does, an adjudicated sample on your own data must say
  how often it is right at that confidence, and it must beat the deterministic baseline
  you already have. The vendor says the same about thresholds: they belong to your
  question, options, model and workload. [vendor]
- **Keep it optional.** Every feature works on its deterministic path with the provider
  set to none. Judgment turns on per site, and only in the confidence bands that passed
  calibration. No feature is built whose value exists only if the model does.
- **Weigh it every time.** Every design, assessment and decision record that touches a
  feature a fast model could serve carries a short "judgment, weighed" section, even when
  the answer is "none" (section 9).
- **What is not claimed here.** That any of these models reads code well, that any of
  this saves tokens end to end, or that a judgment is ever proof. Accuracy on code was
  unmeasured when this was written; the catalog says how to measure it.

## 1. Primitives and when each fits

| Kind (kit) | Asks | Returns | Reach for it when | Watch out |
|---|---|---|---|---|
| Predicate (`KindPredicate`; vendor term and wire alias "noul") | a yes/no statement, optional criteria for each side | probability of yes, no confidence field | gates, flags, per-item screening, "is this one relevant" | the vendor's other kinds carry a `confidence`; a threshold tuned on a predicate does not carry to a choice [vendor] |
| Choice (`KindChoice`) | pick one of up to 255 options you define | probability per option, a `confidence` | classification, routing, triage | relative, not absolute: it says which option, never that none applies, so give it an exit and add per-option predicates when "act at all" matters [vendor] |
| Score (`KindScore`) | a position on 2 to 10 ordered levels you describe | a value that can fall between levels, a distribution | grading, risk, severity, difficulty | levels are positions, not measurements: compare to a threshold, never interpolate a magnitude [vendor] |
| Abstain (not a primitive) | an `other` or `unknown` option, or a low-confidence band | the model declining | every choice; every gate that has a human or a deterministic fallback | an explicit unknown option did not produce reliable abstention in the kit's own two-request live smoke, which marked uninformative captured text as satisfied [kit]. Treat abstention as something to calibrate, not a feature to trust. |

**In this repository.** `foundations/decision` defines the three kinds, `Confidence` (the
vendor's k/max statistic, recomputed locally and never taken from the wire), `BandOf`,
and four deciders that swap by construction: `None` (always "no provider", in the core),
`llm.LLM` (a generic completion asked to emulate the contract, in
`foundations/decision/llm`), `typesafe.TypeSafe` (Jev's native endpoint, or the same wire
through OpenRouter's alpha Decisions router, in `foundations/decision/typesafe`) and
`openai.OpenAI` (OpenAI's Decisions API, a public beta that also takes images and can
refuse a question, in `foundations/decision/openai`). The core imports no adapter package,
so the port carries no vendor code. Swappable is not interchangeable: each provider and
route gets its own calibration reading
([decision 0018](intent/decisions/0018-one-decision-port-optional-provider-adapters.md)).
A refusal arrives in `Response.Declined`, never as a "no" or a zero. Remote adapters read
their key from a chmod-600 file reference or an environment variable name, never from
configuration inline. Two
cautions. `BandOf` hard-codes 0.5 and 0.9, which are the vendor's illustrative starting
bands, not calibrated bands for any of your sites; `CalibratedBand` keeps a band for a
site only where that site's reading (`NewReading`) clears a stated bar (`HintBar`,
`GateBar`) and reads everything else as unsure, see
[decision 0017](intent/decisions/0017-fast-judgment-is-optional-and-calibrated.md).
And the `LLM` adapter emulates the shape, not the model: its numbers are self-reported,
so calibrate it separately from the native provider.

## 2. Where judgment sits: code, judgment, agents

Think in "and", not "or". Three classes of worker, each doing what it does best:
deterministic code for numbers, thresholds, exact lookups and anything computable; a
fast judgment model for bounded semantic questions; agents for open work that needs
reasoning and writing. [demo] The demonstrations make this point repeatedly and it is
the design rule that keeps a project from paying twice.

- **Route by question shape, with structure, not with a model.** Named operators or
  existing tools answer every computable question. Only an explicit judgment question
  reaches the port. A model must not decide whether a question is computable; one
  consumer's routing site did that and was right 45.7% of the time at confidence 0.9
  or above, and non-monotonic. [reading]
- **Proof first.** If the system can compute the answer (who calls this, did the run
  pass, which tests failed), compute it. A judgment is never a receipt a stranger can
  re-derive, however good the model becomes.
- **Authority is narrow.** A judgment never asserts a fact about the code, prunes
  silently, sets a status, or blocks, until its template has cleared the
  pruning-and-gating bar in section 3. Until then it is shown to the asker, labeled
  with its band, and rows keep their deterministic order.

## 3. Calibration first

### Why

Confidence summarizes how concentrated the distribution is; it says nothing about how
often the model is right on your data. The vendor documents literal reading, weakness
with arithmetic, dates and indirection, sensitivity to irrelevant context, and
susceptibility to adversarial text. [vendor] Its published workflow comparisons use model
consensus references rather than independently adjudicated defects, so they support
investigation, not a claim about your accuracy. The consumer readings agree: a routing
site failed calibration outright, and 41 held-out prompts could not have established a
0.9 claim even at 38 of 41 correct, because the Wilson 95% lower bound of 38/41 is
0.806. [reading]

### The method

1. **Write the site down before touching the model.** The question and its options;
   the unit of input (a span, not a file); the action the answer drives; the cost of a
   wrong answer in each direction; the deterministic baseline it must beat.
2. **Build an adjudicated sample.** Cover positives, benign lookalikes,
   missing-context cases, adversarial cases and defects nobody catalogued. Split
   development from holdout by independent unit (a repository, a change), never by row.
   Freeze the holdout before the first run.
3. **Get honest labels.** Record each label's provenance: human, frontier model, or
   computed. Have a human adjudicate every disagreement plus a random sample (20% is a
   workable floor). Never let a frontier model's labels be the only truth when the arm
   you compare against is a frontier model; the comparison becomes circular.
4. **Run and record.** Keep the raw probabilities, the resolved model ID, the question
   version and digest, and the input hash for every answer. In this kit
   `decision.Response.Records` flattens answers with the site, provider, resolved model,
   confidence, latency and token counts; it does not carry the question version, question
   digest or input hash, so add those to your own log.
5. **Read the result, not the average.** Report n; accuracy by confidence band with
   Wilson intervals; whether accuracy rises with confidence (if it does not, no
   threshold can repair it); Brier score and AUROC; the label provenance mix; coverage
   at the bar. Correlated questions and repeated replays are not independent trials.
   In this kit `decision.NewReading` computes all of these but coverage for one (site,
   template version, provider, resolved model) from your labeled samples: n and accuracy
   with a Wilson 95% interval, the split by confidence band with per-band intervals,
   monotonicity, Brier, AUROC and the provenance mix, and `Reading.RecalibrationDue`
   flags a moved model. It is pure compute with no table, so you keep the samples
   (`reliability-stats` offers paired bootstrap, Wilcoxon and A12 for comparing arms;
   [decision 0017](intent/decisions/0017-fast-judgment-is-optional-and-calibrated.md),
   [plan](plans/decision-port.md#hexagonal-hardening)).
6. **Apply the bar for the authority the answer will have.**

   | Authority | Bar |
   |---|---|
   | A hint shown as calibrated | at least 90 of 100 right at the confidence shown (point estimate, n at least 100) |
   | Prunes, gates or reorders anything served by default | Wilson 95% lower bound at least 0.90 |
   | A free-form question the agent wrote | never calibrated: show the answer labeled uncalibrated; rows keep their order; nothing is pruned |

   The lower-bound bar is stricter than it sounds. Correct answers needed for a Wilson
   95% lower bound of 0.90 (computed for this guide):

   | n | 30 | 50 | 100 | 200 | 300 | 500 | 1000 |
   |---|---|---|---|---|---|---|---|
   | correct needed | unreachable | 50 | 96 | 189 | 281 | 464 | 919 |

   Zero misses in n cases bounds the miss rate below about 3/n at 95% (0.095 at n=30,
   0.030 at n=100), so a small clean challenge set says little about production.
7. **Beat the deterministic baseline, by a margin.** Run the same items through the
   cheapest deterministic method: a lexical search, a regular expression, or a fitted
   model such as logistic regression on the features you already have. A workable pass
   is AUROC higher by at least 0.05 with the interval excluding zero. When the features
   are numeric a fitted classical model will often win, and that is the right outcome:
   drop the model site. Judgment earns its place on text, where features are hard.
8. **Write kill criteria before the run.** For example: any miss on the screening
   properties means no code-reading site is built; a tie with grep means the predicate
   belongs in the computable catalog; a routing gain under the stated margin means the
   route stays deterministic. Text-shaped sites are not killed by a code-reading kill.
9. **Ship in bands.** Use the model only in bands where calibration passed and fall back
   to the deterministic path elsewhere. Bands widen as later readings pass. Nothing
   about a band survives a model, alias, question or provider change.
10. **Recalibrate continuously.** Sample judged decisions in production and collect
    human verdicts cheaply: one keypress on a decision the person was going to make
    anyway, in batches of about ten, with provenance stored on every label. Re-grade a
    site when the resolved model changes, an alias moves, the question digest changes,
    or the provider changes. Flag "recalibration due" automatically. Fitted recalibration
    and automatic disabling need at least about 100 labels per (site, provider, model)
    and a minimum n per window, or they flap. Pin the versioned model ID once thresholds
    are tuned: an alias moves and takes the calibration with it. [vendor]
11. **Spend labels where they pay.** Human labels are scarce (one consumer's owner had
    produced under a hundred verdicts in total). Label one template at a time until it
    passes or is dropped.

### Calibration record template

```yaml
# One record per (site, question version, provider, resolved model).
site: triage_failure
question: {id: failure_kind, kind: choice, version: 3, digest: "sha256:<question bytes>"}
unit: "redacted excerpt of at most 1.5k tokens plus the candidate span"
authority: hint            # hint | prune-gate-reorder ; free-form is never calibrated
options: {source: "closed set built from facts", exit: other}
baseline: {name: "runner-output regex", metric: "accuracy at equal coverage"}
provider: {name: typesafe, requested: jev-latest, resolved: jev-1.13.0}
sample:
  frozen: 2026-01-01
  split_unit: repository
  n_dev: 0
  n_holdout: 0
  strata: {positive: 0, benign_lookalike: 0, missing_context: 0, adversarial: 0, uncataloged: 0}
  labels: {human: 0, frontier: 0, computed: 0}
  adjudication: "every disagreement plus a 20% random sample, by a human"
bars: {hint: "90/100 at shown confidence", gate: "Wilson95 lower bound >= 0.90", baseline_margin: "AUROC +0.05, CI excludes 0"}
kill: ["written before the run"]
reading:
  n: 0
  by_band: []              # {band, n, correct, wilson_lo, wilson_hi}
  monotonic: null
  brier: null
  auroc: null
  baseline_auroc: null
  coverage_at_bar: null
verdict: insufficient-n    # pass | fail | insufficient-n
recalibrate_when: [resolved model changes, alias moves, question digest changes, provider changes, sampled verdict accuracy leaves the band over a window of adequate n]
```

## 4. Cost, latency and offline degradation

**Arithmetic, from the list price** (input tokens only; output is free) [vendor]:
20,000 input tokens cost $0.00084; a call of about 500 tokens is $0.000021, so a
million such calls is about $21. Questions share one state, so batching matters more
than the per-token price: 24 questions of 120 tokens over a 6,000-token state cost
$0.00617 as 24 separate calls and $0.000373 as one, 16.5 times less, before protocol
overhead. That is arithmetic about repeated input, not a measured end-to-end saving.

**Limits that bind before cost.** 64k tokens per request and 32k for the state plus the
longest question, about 14 rows of 2k tokens. Rate limits of 250,000 tokens per second
and 1,200 requests per minute (20 per second), stated to change without notice. [vendor]
At fleet scale the request rate binds long before the token bill does.

**Latency.** Sub-second is the vendor's claim and the demonstrations'. [vendor][demo] One
consumer measured 130 to 375 ms on small states and 421 to 587 ms at about 22k tokens
of state. The generic-completion emulation took 1.5 to 2.8 seconds, and 3 of 6 calls
timed out at 3 seconds, at 17 to 41 times the native price per call. [reading] Only the
native lane fits inside a hook deadline; keep the emulated lane to command-line and
background work. On a missed deadline, skip silently and append at the next opportunity.

**Degradation is a designed path, not an exception.**

- Provider none is the default: nothing leaves the machine and every feature answers
  from its deterministic path. Under none, still return the redacted plan of what would
  have been sent, so a person can see the cost and the egress before opting in.
- A missing key, a timeout, a 429 or 529, a refusal or a safe mode each produce one
  disclosed line and the deterministic path. Never fall back silently to a different
  provider: its numbers are uncalibrated. (`typesafe.NewTypeSafe` and `openai.NewOpenAI`
  refuse to construct without a key source for this reason; a caller wanting a fallback
  composes one on purpose, and `Fallback` discloses every miss, a refusal included.)
- Access itself is unreliable while a product is young. A secondary source reported
  new sign-ups paused for demand within a week of launch, the vendor says its limits
  move without notice, and the aggregator route used in the kit's smoke was an alpha
  endpoint. [vendor][kit]
- Cache judgments by (question template version, content hash) and stamp each with
  provider and model version, kept apart from anything provable, so a better model
  re-grades cleanly. On the native lane a query costs under a cent, so a durable memo
  rarely pays for its memory; it matters on the expensive lane. [reading]
- Keep a spend ledger per session (calls, questions, tokens, dollars) and print it once
  at the end. The demonstrations do this and it is cheap. [demo]

## 5. Privacy

Every call sends text off the host. Design for that.

- **Opt-in twice.** Per repository and per decision site. Setting a provider for one
  site must not switch on another: a site added the usual way otherwise turns on in
  every project that has any provider configured. Freeze the default list and add
  every new site to an explicit opt-in list.
- **Send only the spans the question needs.** A claim and the lines that support it, not
  the file; a redacted excerpt of a failing run, not the log. Smaller state is also more
  accurate (section 6).
- **Screen before you send.** Secrets, credentials and personal data are removed or the
  call is skipped. Log hashes and counts, not text.
- **Treat a labeling pass as an egress event.** A sweep over a repository sends every
  span in it. Provider none keeps all of it local.
- **Know the terms.** The vendor states it does not train on customer requests and
  offers zero data retention for enterprise customers, and on one aggregator a
  per-request flag. [vendor] Check the terms for your data class before enabling a site.
- **State is data and can steer the model.** Text written to argue for its own
  classification, or an injected instruction inside judged content, can move an answer.
  [vendor] A judgment over untrusted text is one signal among several, never the only
  control.

## 6. Schema patterns: options come from facts

- **The model chooses among candidates code produced.** Build the option list from
  index facts (the callers a graph returned, the agents you actually run, the rules that
  apply) and let the model pick. A choice can only return one of your options; it cannot
  invent a label. Prune to the 255-option cap in code before building the question.
- **Always give an exit.** `other`, `none_of_these`, or a "read it yourself" band. Then
  calibrate the exit, section 1.
- **One snap judgment per question.** "Does this convey urgency?", not "analyze and
  decide the best action". Split broad claims into atomic questions and combine them in
  code. An aggregate score must not hide a required failure.
- **Describe situations, not degrees.** "Blocking, no workaround exists" beats
  "moderately severe". Put boundary cases in the criteria. Keep the question and its
  criteria consistent; a criterion that contradicts its instruction lowers accuracy.
  [vendor] The model reads literally: when you catch yourself explaining what you really
  meant, that explanation is the missing half of the instruction.
- **Fan out.** Ask every question you might need in one request, including speculative
  ones; they are evaluated in parallel against the shared state and adding questions
  usually costs little time. Decide relevance in code. Use a second request only for a
  real dependency, where the next question's options depend on the last answer.
  [vendor]
- **Numbers, dates and counting stay in code.** The model picks a card from the deck; it
  does not name one. To count, ask one question per candidate and add up in code. To
  compare dates, extract the parts with a choice over a closed set (with a "not stated"
  option) and compare in code.
- **Isolate the state.** Filter first, send the fields the question needs. A whole file
  or a whole document dilutes the answer. [vendor] Point questions at named fields in a
  structured state.
- **Do not lean on structural identities.** A yes/no choice, a predicate on the same
  question, and a predicate on its negation are separate judgments and need not add up
  (the vendor shows a pair summing to 1.19). [vendor] Never carry a threshold across kinds.
- **Extraction is a choice over extracted candidates.** Use a regular expression or a
  generative model to propose values and let the fast model pick; it does not write.
- **Keep questions and thresholds together, versioned and reviewable.** One record per
  site, digested, so a change to wording or a threshold is visible in review and forces
  recalibration. Weights live in code, so tuning is changing a number, not a prompt.

## 7. Anti-patterns

1. **Trusting confidence as correctness.** A shown 0.9 with no reading behind it.
2. **Asking a model whether a question is computable.** Route structurally.
3. **Judging what code computes.** Callers, reach, exports, counts, dates, arithmetic,
   generated-file detection, ownership.
4. **Believing an `unknown` option is abstention.** Calibrate it; the kit's smoke did
   not show it working. [kit]
5. **Calibrating on a few dozen items, or on labels from the comparator's own family.**
6. **Carrying a threshold across a kind, a model version, an alias move, a reworded
   question or a provider.** Each is a new site.
7. **Letting a judgment prune, gate or set status before it passed the strict bar.** One
   concrete hazard: showing "the test's expectation is outdated" uncalibrated invites
   editing the test to match buggy output, which nothing downstream catches.
8. **Model-judged security properties as a served signal.** Measure them if you must;
   never serve them. Also negation-shaped and double-negative questions, which the
   vendor lists as weak. [vendor]
9. **Whole-file, whole-repository or secret-bearing state.**
10. **A feature whose value exists only if the model does.** Build for the deterministic
    path and let the model improve it monotonically.
11. **One request per question over a shared state.** Batch.
12. **Making the agent choose to call it.** Tools an agent must remember to call are
    adopted rarely. Deliver at a moment the agent cannot skip (a hook, a handoff) and
    keep the explicit tool for questions only the agent can write.
13. **Demo thresholds in production.** The floors and bars in tutorials are illustrations
    ("the correct values depend on your domain and the model's performance" [vendor]).
14. **Asking it to generate.** It is not trained to; use a generative model.
15. **New sites default-on when any provider is set.**
16. **Silent fallback to a different provider or an older model.**

## 8. Agentic use: what "level 10" is, what to borrow, how to keep it optional

IndyDevDan's video "10 Levels of Jev for Agentic Engineers" (about 35 minutes) walks a
ladder of ten uses. Levels 1 to 5 call the model from plain code: a yes/no gate,
multiple choice, composite scoring with weights in code, confidence-gated actions, and
routing in front of expensive things. Levels 6 to 10 put it inside a coding agent: as
hooks the agent never sees (a command gate and a tool-result screen, level 6; a
"should I compact" check after every turn, level 7), then as tools the agent chooses to
call (a judgment about one file without reading it, level 8; the same question over many
files in parallel, level 9), and finally level 10, which he calls agentic Jev: the agent
writes the questions itself.

**Level 10 in the repository.** One tool. The agent passes a short state of its own, up
to 20 file paths for code to read, or a command for code to run, plus its own question
block. Code assembles one state, runs the command through the level-6 command gate
first, makes one call, and returns only the answers: the agent never receives the files
or the command output. A short nudge in the system prompt says the tool exists; the tool
description carries the question schema, worked examples, and a "not for" list (exact
lookups, counting, anything a search answers). An oversize state is refused with a
message that names each part's size and a split that fits. A spend ledger prints at the
end. In the demonstrated run the agent classified a red test run as a code bug at 0.99,
fixed a rounding step, scored its own diff, and confirmed green: 3 calls, 4 questions,
$0.000084. [demo]

**What is worth borrowing** (ideas, not code; see attribution):

1. Code assembles the state and the agent gets typed answers. The context the agent
   never reads is the saving, and the answer is a number it can branch on.
2. The tool description is the teaching: what the model is, the three question kinds, one
   example each, what it is not for.
3. Refusal messages that say how to fix the call ("split into two calls: these paths,
   those paths") instead of "too large".
4. Any command the tool runs goes through the same gate as the agent's own commands.
5. The ledger: what the habit cost and what it replaced, in one line.

**What to change for a governed project.**

- **Proof first.** Most of what the demonstrated agent asked has an exact answer:
  did the run pass, which tests failed, where the frames land, which changed symbols
  they reach. Compute those; a "did all tests pass" answer of 0.99 becomes a parsed
  pass, fail or unknown. Keep the model for what computation cannot answer.
- **Deliver at a forced moment.** Level 10 depends on the agent choosing to call the
  tool. One consumer's logs show chosen tools going almost unused (a localizer had 40
  calls and a test-impact tool 103, out of about 98,000 tool calls). [reading] A hook on the test command,
  which returns one line only on failure, reaches every session.
- **Options from facts, with an exit** (section 6), and free-form agent questions shown
  as uncalibrated: they never prune or reorder anything.
- **Bench it against the raw version.** Put the unmodified level-10 tool in as its own
  arm in any test-fix comparison, so you learn whether your additions help, and publish
  the result either way.
- **Audit what the agent no longer sees.** Hiding command output is where the saving
  comes from, and it also removes the agent's chance to notice what the classifier
  missed. Sample and audit the "no issue" answers.
- **The command gate is one control among several.** The repository's own notes record an
  agent that was blocked writing a protected file and wrote it with a shell heredoc
  instead; a "do not work around this" notice is an instruction, not a control. [demo]

**How it stays optional.** Default provider none. Each site opts in on its own list.
The tool description states the deterministic route. A failure produces one disclosed
line, then the deterministic path. No other feature depends on the tool.

## 9. Weigh judgment in every design

A judgment site is easy to add and hard to remove, and the reasons to say no are easily
forgotten. So weigh it every time, on paper. Every design document, assessment, decision
record and plan that touches a feature a fast model could serve carries a short section,
"Judgment, weighed", that answers the questions below. "None: everything here is
computable, by this operator" is a complete answer, and it records why the question was
not handed to a model so nobody has to ask again. One consuming project made this a
standing rule after finding it practiced in every design but written down nowhere; a
project that wants it should put one line in its contributor guide.

1. **Sites.** Which typed questions could a fast model answer here (predicate, choice or
   score), from which facts the system already holds, with what exit option? Which
   questions look like judgment but are computable, and which operator computes them
   (section 2)?
2. **Authority.** For each site, is the answer a hint shown to the asker, something that
   prunes, gates or reorders, or a status? The bar for that authority is in section 3;
   until it is met the answer is labeled and rows keep their deterministic order.
3. **Bands.** Which calibrated confidence bands may act, what adjudicated sample supports
   them, and what triggers recalibration (a model, alias, question, kind or provider
   change).
4. **Provider none.** What does the feature do with no provider? Which one disclosed line
   appears when the provider fails, and does the deterministic path then run? A feature
   whose value exists only if the model does is not built (section 7, item 10).
5. **Facts it must never assert.** A judgment never says what code computes and never
   sets a status (section 2).
6. **Data.** What leaves the machine, declared per site, off unless the site opts in
   (section 5).
7. **Cost and baseline.** Volume, latency, the deterministic baseline to beat, and the
   price of a wrong answer in each direction (section 4).
8. **Port.** Provider-neutral: generic kinds, limits and capabilities declared by the
   adapter, and a conformance check every adapter passes, so adopting a model is a
   measurement and not a rewrite. Record one future per site, each "promote when
   provider P passes calibration on this site"; no vendor is the default
   ([decision 0017](intent/decisions/0017-fast-judgment-is-optional-and-calibrated.md)).
9. **Measurement.** The bar, written before any run, that proves the site, and the kill
   condition that ends it, with the baseline arm named.

The section is a prompt, not a gate. Its value is that judgment is weighed each time and
not remembered when convenient, and that a "no" is a recorded answer with its reason.
Revisit a "no" when the model class, its price or its measured accuracy on your data
changes.

## 10. Sources and attribution

The ideas about levels, the "and, not or" framing, and the agentic tool shape come from
IndyDevDan. This guide summarizes them in its own words and reproduces no code. All
figures marked [demo] are his spoken or published claims, not measured here. His
$20-per-million-calls figure and the multiples he quotes against other models are
spoken claims; recompute against current prices before relying on them.

- Video: [10 Levels of Jev for Agentic Engineers](https://www.youtube.com/watch?v=_U-O5lYhJ7Q)
  (also https://youtu.be/_U-O5lYhJ7Q).
- [disler/ten-levels-of-jev](https://github.com/disler/ten-levels-of-jev), MIT,
  Copyright (c) 2026 IndyDevDan / AgenticEngineer.com; read at commit 777adaf.
  Level 10 is `apps/ten-levels/extensions/ask-jev.ts` and `src/levels/level10/`.
- [disler/self-compact-pi-agent](https://github.com/disler/self-compact-pi-agent), MIT,
  Copyright (c) 2026 IndyDevDan; read at commit 576fe4a. The base for level 7: the agent
  sees a gauge, writes a note to itself, and compacts at a clean checkpoint.
- If code is ever copied from either repository, keep its MIT copyright notice. Do not
  borrow the branding, images or the paid-course marketing at the foot of the README.
- **Correction worth knowing.** Neither repository contains a Claude Code hook. Both
  target the `pi` coding agent's extension events; mapping them to Claude Code hooks is
  our translation, not his code.
- Named but not read (pointers only): `7hemas7er/jev-hooks` (typed decisions for a commit
  reviewer), `Akramovic1/jev-pilot` (steers effort, subagent model and skill),
  `spring-ai-community/spring-ai-typesafe` (a Java SDK), `walidboulanouar/awesome-jev-use-cases`
  and `cobanov/awesome-jev` (lists). The assessment also cites `NiazMorshed2007/jev-review`
  and `gargpratyush/jev-router` as design references, likewise unaudited.
- Vendor documentation (TypeSafe AI, checked 2026-09-28): [primitives](https://docs.typesafe.ai/primitives),
  [confidence](https://docs.typesafe.ai/confidence), [models](https://docs.typesafe.ai/models),
  [Jev 1.13 jaggedness](https://docs.typesafe.ai/model-jaggedness/jev-1.13),
  [speculative fan-out](https://docs.typesafe.ai/patterns/fan-out),
  [confidence-gated routing](https://docs.typesafe.ai/patterns/confidence-routing),
  [SDE cascade](https://docs.typesafe.ai/cookbooks/sde_cascade),
  [parallel questions](https://docs.typesafe.ai/cookbooks/parallel_questions),
  [Jev with coding agents](https://docs.typesafe.ai/introduction/coding-agents),
  [legal and retention](https://docs.typesafe.ai/legal).
  The vendor says Jev is not a coding-agent LLM and is meant to be called from code you
  write. The docs move; reread them before relying on a limit or a price.
- Vendor documentation (OpenAI and OpenRouter, checked 2026-10-09): OpenAI's
  [Decisions guide](https://developers.openai.com/api/docs/guides/decisions) (public beta;
  `gpt-6-luna` only; predicate, choice and score questions, images as base64 data URLs, a
  `refusal` answer type; US$0.10 per million input tokens and no output charge; no
  calibration claim, thresholds from your own labelled examples) [vendor], and OpenRouter's
  [Decisions API reference](https://openrouter.ai/docs/api/api-reference/alphadecisions/submit-a-decisions-questions-and-answers-request)
  (POST `https://openrouter.ai/api/alpha/decisions`, the System One wire, model
  `typesafe/jev-1.13`; an alpha endpoint) [vendor]. No live call to either has been made
  from the kit.
- In this repository: [decision port plan](plans/decision-port.md), the
  [assessment](assessments/2026-09-21-jev-opportunities.md), the
  [typed-judgment review design](designs/typed-judgment-review.md) (a worked criterion
  contract) and [futures F7 and F14](intent/futures.md).
- Figures tagged [reading] come from one consuming project's decision logs and
  calibration runs in September 2026; they are small-N and specific to its data.
