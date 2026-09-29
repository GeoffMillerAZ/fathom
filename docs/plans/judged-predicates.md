# Judged predicates: the plan (vision 40, ADR-056)

Grammar v2 (`docs/plans/README.md`). A row flips to `[x]` only when
`fathom verify --service fathom --plan docs/plans/judged-predicates.md` proves it. Design:
`docs/design/judged-predicates.md` (section numbers below are its). Goal G28.

J0 comes first: nobody has measured a typed judgment model's accuracy on code, and every site that reads
code with a model (`fathom ask`, the traversal scorer, files at scale) waits for its reading. J0 and the
I3 smoke test are accepted work; the rest are deferred rows with triggers, recorded so they are neither
built early nor re-proposed. Bench code lives in `workspace/` (not the product), so its rows are
measurement rows whose contract is the committed reading. Beside `docs/plans/decision-port.md` (DP).

## Wave 1: does a typed judgment model read code? (section 2)

- [ ] (op) JP-1: the J0 pre-registration is committed before any run: the span set (about 1,000 spans from fathom and two open source Go repositories), the oracle templates the index computes, the lexical baseline arm, the residue set and the four bars, unchanged after data (P22, T5) {id: JP-1, oracle: lexical, path: docs/bench/readings/judged-predicates/j0-preregistration.md, pattern: "^JP-1 pre-registered"}
- [ ] (op) JP-2: the 150-unit residue set is labeled: a frontier pass, the owner adjudicates every disagreement plus a 20 percent random sample, provenance recorded as human, frontier or computed; security residue is measured and never served (R7) {id: JP-2, oracle: lexical, path: docs/bench/readings/judged-predicates/j0-residue-labels.md, pattern: "^JP-2 labeled", needs: "JP-1"}
- [ ] (op) JP-M1: the J0 screen: direct-visible properties AUROC at least 0.9 and recall at least 0.95 at a prune rate of at least 0.5; beats the lexical baseline by at least 0.05 AUROC; context assembly lifts transitive recall by at least 20 points; residue precision at least 0.9; or the kill is recorded {id: JP-M1, oracle: lexical, path: docs/bench/readings/judged-predicates/j0.md, pattern: "^JP-M1 verdict: (PASS|KILL)", needs: "JP-1,JP-2", weight: 3}
- [ ] (op) JP-M2: the I3 smoke test over the 1,346 proven anchored plan rows with every anchor fragment masked, 8 candidates per row, the candidate generator's gold-in-top-8 rate reported separately; it never counts toward SI-M4 {id: JP-M2, oracle: lexical, path: docs/bench/readings/judged-predicates/i3-masked-rows.md, pattern: "^JP-M2 reading:", needs: "JP-1"}
- [ ] The J0 result is filed as an observation with `fathom observe`, so a kill or a pass is on the live log and not only in a reading {id: JP-3, oracle: lexical, path: .fathom/observations.md, pattern: "^### O[0-9]+ .*J0 screen", needs: "JP-M1"}

## Deferred behind J0: `fathom ask` (section 3; F34, F43)

Every row below waits for the trigger on the first one. None is built, none is a promise.

- [-] `fathom ask`: a CLI verb plus a lazily loaded skill (no schema bytes); selectors map one to one onto existing query functions; computable operators evaluated first; units of at most about 2k tokens, about 14 rows per call; options from index facts with a `none` exit; bands yes, no and read-it; a receipted table that lists `pruned[]` {id: JP-10, disposition: deferred, reason: "before J0 its computable half duplicates find_callers, test_impact, query_symbols and blast_radius behind a new grammar, and agents do not adopt tools they must choose to call", trigger: "JP-M1 reads PASS, including beating the lexical baseline (F34)"}
- [-] Provider none returns the computed rows, "judgment unavailable (no provider)" and the redacted request plan {id: JP-11, disposition: deferred, reason: "part of the ask verb", trigger: "JP-10 starts"}
- [-] Free-form `--q` answers are shown uncalibrated and never prune or reorder; a template prunes only at R3's pruning-and-gating bar {id: JP-12, disposition: deferred, reason: "the agent writes its own questions without fathom acting on an uncalibrated answer (R3)", trigger: "JP-10 starts"}
- [-] `--plan-only` export for other harnesses and Review Kit's typed-judgment lane {id: JP-13, disposition: deferred, reason: "no consumer has asked", trigger: "a named consumer asks (F43)"}
- [-] The 40-handlers cell: 3 corpora, 5 questions, 3 trials, arms A to E, owner-adjudicated truth, a non-security showcase property (X6) {id: JP-14, disposition: deferred, reason: "the bench costs $110 to $450 and its unique value is the judged predicate", trigger: "JP-10 has shipped and JP-M1 read PASS"}
- [-] (op) JP-M3: arm C (ask on typesafe) uses at least 40 percent fewer tokens than arm A at recall and precision within 0.05, CI excluding 0; arm E at least C moves the predicate to the computable catalog {id: JP-M3, disposition: deferred, reason: "the cell runs only after the ask verb exists", trigger: "JP-14 runs"}
- [-] A durable judgment memo {id: JP-15, disposition: deferred, reason: "R5: no durable memo before RT-M1 reads PASS; on Jev a memo saves under one cent per query", trigger: "responsible-tenant.md#RT-M1 reads PASS and the llm lane is in use (F40)"}
- [-] Graduation of a passing template to a CUE-declared judged layer whose rows are claims at tier ai-derived {id: JP-16, disposition: deferred, reason: "schema/semantic/schema.cue rejects the tier today; it needs an ADR promoting F7", trigger: "a template passes R3's pruning-and-gating bar and an ADR promotes F7"}

## Deferred behind J0 and the handoff maps: traversal and files at scale (sections 4 and 5; F36, F37)

- [-] Question-directed traversal as `localize kind: question`, provider none first, strong nodes never pruned, the model scorer only after provider none beats control {id: JP-20, disposition: deferred, reason: "no delivery channel (localize had 40 calls in about 98k) and the earlier A/B was null end to end", trigger: "exploration-maps.md#XM-M2 passes so a delivery channel exists (F36)"}
- [-] (op) JP-M4: provider none beats control by 25 percent on tokens-to-correct-answer with the CI excluding 0; the model scorer beats provider none by 15 percent at equal recall {id: JP-M4, disposition: deferred, reason: "the traversal cell runs only after JP-20 exists", trigger: "JP-20 runs"}
- [-] Files at scale: a judged layer (a CUE template plus a unit selector) run only on units whose content hash changed and scoped to a live map's footprint; the first candidate is test-kind residue for T2 {id: JP-21, disposition: deferred, reason: "the job and its memo need tenancy, and a fleet memo is about 130 MB", trigger: "JP-M1 reads PASS, responsible-tenant.md#RT-M1 reads PASS, and a named consumer's own bench metric judges it (F37)"}

## Refused (recorded so they are not re-proposed)

- [-] `ask`'s computable-operator grammar as a new verb {id: JP-X1, disposition: refused, reason: "the operators duplicate existing tools and grow the compatibility surface; missing operators are added to query_symbols on call-log demand"}
- [-] Computable labels as model work: generated or vendored files, config versus code, public API, ownership, I/O reach, TODO admission {id: JP-X2, disposition: refused, reason: "each is already computable (headers, extensions, exports plus the inventory, CODEOWNERS, graph reachability, grep)"}
- [-] Model-judged security properties as a served signal, even as a separately labeled section {id: JP-X3, disposition: refused, reason: "X6 and ADR-051 require cited reasons; security residue is measured in J0 only"}
- [-] Generated text from Jev: summaries, names, rule text, rewrites, query reformulations {id: JP-X4, disposition: refused, reason: "Jev cannot generate; at most it chooses among candidates code or an llm produced"}
