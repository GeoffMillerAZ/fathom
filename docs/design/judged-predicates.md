# Judged predicates: fathom plans and audits, a typed judgment model only fills the residue

Status: accepted 2026-09-29 (ADR-056): J0 and the I3 smoke test are accepted work; everything that
reads code with a model waits for J0. Vision: `docs/vision/40-judgment-and-exploration-maps.md`.
Plan: `docs/plans/judged-predicates.md` (JP-). Goal G28. Builds on the decision port (ADR-042,
ADR-056), the join rule (ADR-051 decision 1), semantic layers (`docs/design/semantic-layers.md`,
ADR-016) and the strengths bench (ADR-033). Constraints carried in: R1 to R8
(`docs/design/judgment-foundations.md` section 3), ADR-042 decision 5 (no new MCP tool), P4, P19 (cost
scales with change size), ADR-041 decision 8 (nothing inferred from annotation text), I46, X6 (a model
never prunes on a security property).

## 1. The idea, in one line

Fathom plans and audits; a judgment model only fills the residue. An agent that wants "which of these N
units have property P" today reads N files. Fathom can pick the N exactly (callers, blast radius, scope,
tests reaching, plan anchors, a diff) and can compute many properties outright; for the rest it can hand a
fast typed model a small, attributable unit plus the context the graph assembles, get a three-band answer,
and show the agent a receipted table. The model never decides what is computable, never sees more than the
declared unit, and its answer is never a fact.

## 2. What must be measured first: J0 (accepted)

Nobody has measured a typed judgment model's accuracy on code, and every code-reading site depends on it:
the ask predicates, the traversal scorer and the files-at-scale labels. J0 is the cheapest decisive reading
and is a one-way kill screen, with the I3 smoke test alongside.

**J0 mechanism.**

- **Spans:** about 1,000 symbol spans from fathom and two open source Go repositories with typed edges. A
  TypeScript arm is a distribution check only (its edges are syntactic and unverified).
- **Oracle templates the index can compute:** calls the storage layer directly; performs network I/O
  directly; returns an error callers must handle. Transitive variants run span-only, then with callee
  signatures assembled, which measures how much fathom's context lifts the model.
- **Lexical baseline arm:** grep the span for sink identifiers. The model has to beat grep.
- **Residue set:** 150 semantic units ("validates input", "parses untrusted input"). A frontier pass labels
  them; the owner adjudicates every disagreement plus a 20 percent random sample; provenance is recorded as
  human, frontier or computed. Security residue is measured here and never served (R7).
- **Note:** the oracle properties are exactly the ones routing never sends to the model, so a pass tells us
  nothing about the residue by itself; it is a kill screen only.

**I3 smoke test (never counts toward SI-M4).** Take the 1,346 proven anchored plan rows. Mask every anchor
fragment from the row text (package names, file names, `reachable_from` test names, camelCase parts).
Take 8 candidates from `find_symbol` and `pack_context`, ask one noul per candidate, take the argmax with an
abstain margin, and report the candidate generator's gold-in-top-8 rate separately. These rows were written
by agents in fathom's own grammar, while SI-M4 targets foreign dialects.

**Cost and labor.** J0 $0.07 to $0.15 and I3 $0.18 to $0.20 of provider spend; 1 to 2 days of bench code
and about 50 to 80 owner labels. Bench code lives in `workspace/`, not the product.

**Bar (screen pass), all four.** Direct-visible properties reach AUROC at least 0.9 and recall at least
0.95 at a threshold that prunes at least half; the model beats the lexical baseline by at least 0.05 AUROC;
context assembly lifts transitive recall by at least 20 points; residue precision is at least 0.9 (point
estimate). **Kill:** any miss, or grep tying the model, means no code-reading site is built: `ask` (F34),
the traversal scorer (F36) and files at scale (F37) stop. Text-shaped sites (failure excerpts, prompts, doc
spans) are unaffected. The result is recorded as reading JP-M1 (and JP-M2 for I3) and as an observation.

## 3. `fathom ask` (deferred, F34; trigger: J0 passes and beats the lexical baseline)

**Delivery.** A CLI verb plus a lazily loaded skill, so it adds no schema bytes. Before J0 its computable
half would duplicate `find_callers`, `test_impact`, `query_symbols` and `blast_radius` behind a new grammar,
and agents do not adopt tools they must choose to call.

**Planner steps.**

1. Selectors map one to one onto existing query functions (callers, blast, scope of a rule, tests
   reaching, a plan's anchors, kind and path queries, a diff, localize). No new engine.
2. Resolve rows (default cap 60, maximum 255).
3. Evaluate computable operators first (reaches, called by, exported, generated, certified live, passed at
   HEAD, changed since, in scope, conserved) and drop the rows they decide; computed rows are shown apart
   from judged rows and never mixed.
4. Build each unit: the span plus the context the template declares, at most about 2k tokens per row, about
   14 rows per call under the 32k state cap.
5. Build options from index facts, always with a `none` exit.
6. Look up the in-process cache; send misses as one call per row carrying all questions, concurrency 16, at
   most 20 requests per second.

**Bands and output.** Each row gets yes, no or read-it (the unsure band sends the row back to the agent to
read, level 4's confidence gating applied to exploration). Output is a receipted table: rows `{key, span,
unit_hash, tier: computed or judged, answer, p, band, template@v, calibration, cache}`, `pruned[]` with
reasons, and cost `{calls, input_tokens, usd, source: reported | estimated | unknown}`.

**Provider none.** Returns the computed rows, "judgment unavailable (no provider)" and the redacted request
plan. `--plan-only` prints the redacted request bodies and sends nothing, so any harness (or Review Kit's
typed-judgment lane) can make the call with its own key (deferred, F43, trigger: a named consumer asks).

**Pruning rules.** A template prunes only at R3's pruning-and-gating bar. A free-form `--q` answer is shown
uncalibrated and never prunes or reorders: this is how the agent writes its own questions, the essence of
level 10, without fathom acting on an uncalibrated answer. Missing operators are added to `query_symbols`
on call-log demand, not to `ask`.

**Jev primitive and schema.** One noul per row, for example `state: {unit: {key, signature, body},
context: {route_registration, middleware_chain}}`, `questions: {handles_without_pagination: {type: noul,
instructions: ..., criteria: {true: ..., false: ...}}}`. Context assembly is mandatory: a property such as
authorization is often applied by middleware at route registration, so a span-only judgment is confidently
wrong. A choice takes its options from facts (the layer names in the repo's own arch rules, with `none`).
A score is used only for ordinal properties.

**Question authoring.** Templates are written offline; the llm lane may draft one and a human commits it. A
validator enforces the caps, an `other` exit, one literal condition per question and no double negatives.
Nothing generates questions on the hot path. The model reads literally and cannot count, compare dates,
follow indirection or handle negation, so those shapes are never asked.

**Cost.** About $0.00084 for 40 rows on Jev, about $0.033 on llm, $0 on a cache hit. The hypothesized
counterfactual (reading 40 spans puts about 16 to 20k tokens in a frontier context) is unproven until the
bench runs.

**Privacy.** Sends spans, assembled context and question text; never whole files unless the unit is a file,
never transcripts. Secret-shaped strings are redacted and counted (`internal/llmsec`), `[decide] exclude`
globs are honored, and the site `ask` must be listed in the repository's `[decide] sites` (opt-in). This is
program rule I61 (`docs/intent/invariants.md`).

**Calibration.** Per (template@v, provider, resolved model): a shown verdict needs 9 of 10 right on at least
100 adjudicated rows; a pruning filter needs recall at least 0.95 at a prune rate of at least 0.5 on at
least 200 rows. Labels come from oracles (J0), settled forecasts and owner adjudication (at least 20 percent
of the set, provenance recorded).

**Bench when built (JP-M3): the 40-handlers cell.** Three open source web services fathom did not write (Go
with typed edges, TypeScript, Python), each with at least 40 handlers; 5 questions by 3 corpora by 3 trials,
$110 to $450. The showcase property must not be a security property (X6): a non-security semantic property
the graph cannot compute, such as which handlers are idempotent or return an unbounded list. Truth labels
include owner adjudication (frontier-only labels would be circular with arm A). Arms: A control; B the
fathom default surface; C B plus `ask` on typesafe; D B plus `ask` on llm; E the computable operator alone.
**Kill:** C does not use at least 40 percent fewer tokens than A at recall and precision within 0.05 (CI
excluding 0): drop the judged tier and keep the operators. E at least C: the predicate belongs in the
computable catalog.

## 4. Question-directed traversal (deferred, F36)

Localize's loop generalized to a question (`localize kind: question`), not a new engine or tool. Seeds from
`find_symbol` or `search` on the question's terms; the frontier is the typed neighbours; each node's unit is
its signature plus a short body excerpt (150 to 300 tokens); each hop scores relevance in one fan-out (the
provider-none arm scores by identifier and doc overlap, graph distance and evidence tier); expansion is best
first with a beam; **strong nodes are never pruned** (a frame in a stack trace, a scope match for a governing
rule, a depth-1 neighbour of a seed), only the weak tail is reordered or pruned. It returns a cited subgraph
with the pruned list and reasons. It has no delivery channel yet (localize had 40 calls in about 98k, and the
earlier A/B was null end to end with fathom's arm at 1.64 times control's tokens), so its natural delivery is
as a map builder (`docs/design/exploration-maps.md`): a planner's traversal becomes a map the handoff
delivers. **Trigger:** exploration maps' A/B (XM-M2) passes; then run provider none first, and add the model
scorer only after J0 passes and provider none beats control. **Bars:** provider none beats control by 25
percent on tokens-to-correct-answer with the CI excluding 0; the model beats provider none by 15 percent at
equal recall (recall of truth nodes at least 0.95 at the chosen threshold). **Kill:** either miss drops the
step. Cost: 4 hops of 40 nodes at about 300 tokens is about $0.002 on Jev and about 1.6 s.

## 5. Files at scale for semantic layers (deferred, F37; level 9)

Level 9 asks one cheap question per file in parallel. For fathom the same idea is a judged layer: a CUE
template plus a unit selector, run only on units whose content hash changed (P19), scoped to a live map's
footprint instead of the whole repository, so an expensive column is computed only where a saved question
needs it (`docs/design/exploration-maps.md` section 3).

**Measured cost.** A full pass over fathom's 2,493 Go files (21.1 MB) is $0.22 to $0.27 per label; symbol
level (25k symbols) about $0.53 and about 21 minutes at 20 requests per second; fleet-wide symbol level about
$3.6 and about 2.4 hours. Tokens are trivial; the 20 requests per second limit binds first, and on llm the
work costs 17 to 41 times more per call and saves context, not dollars.

**Labels.** Most candidate labels are computable and are refused as model work: generated or vendored files,
config versus code, public API, ownership, I/O reach, TODO admission. What remains: test kind for the T2
portfolio (only the residue after signatures, build tags and testdata), concern membership (it may reorder
only `pack_context`'s weak tail, the O113 lesson), doc type (low value), and architecture role only for
adopters whose own arch rules supply the options. Security-sensitive residue is measured in J0 and never
served (X6). File or symbol purpose one-liners are generation and belong to the compile lane's llm.

**How fathom does it better than level 9.** It sends spans, not whole files (12 files exceed 60 KB); options
come from index facts; incremental runs are keyed by content hash; the pass is scoped to a live map.
**Trigger (all three):** J0 passes; RT-M1 reads PASS (the job and its memo need tenancy, and a fleet memo is
about 130 MB so it must be disk backed); a named consumer's own bench metric (not "ranking changed") judges
it. The first candidate when the gates open is test-kind residue for T2. **Graduation:** a template that
passes R3's pruning-and-gating bar becomes a CUE-declared judged layer whose rows are claims at tier
`ai-derived`; that needs an ADR promoting F7, because `schema/semantic/schema.cue` rejects the tier today.
**Kill:** J0 fails, or a graduated label does not change any consumer's ranking measurably within 30 days.

## 6. What stays deterministic

Rows, units, context assembly, options, computable operators, cache keys, invalidation, thresholds, bands,
redaction, budgets, and every computed row. A model supplies only a band on a residue property, and the
band is labeled with its tier and calibration state (I62).

## 7. Readings

| Reading | Threshold |
|---|---|
| JP-M1 | J0 screen: direct-visible AUROC at least 0.9, recall at least 0.95 at a prune rate of at least 0.5, beats the lexical baseline by at least 0.05, context lift at least 20 points, residue precision at least 0.9; or the kill is recorded |
| JP-M2 | the I3 masked plan-row smoke test, gold-in-top-8 reported separately |
| JP-M3 | the 40-handlers cell (deferred behind J0) |
| JP-M4 | question-directed traversal cell (deferred behind XM-M2) |
