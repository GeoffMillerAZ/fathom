# Wave 3b — The home-turf bench (index-native tasks)

The lookup bench measured grep's home turf and fired the ×10 retraction
(falsifiability.md B1). This bench measures fathom's: governance and
enforcement questions. **If fathom cannot win decisively here, that is the
existential signal, and this plan's job is to surface it loudly — not to
construct a bench fathom wins.**

Design (normative):
- **H1 corpus.** 12–15 index-native tasks: what governs file F
  (obligations), is plan row X proven / which claimed rows fail (verify),
  transitive blast radius of changing symbol Y's signature (typed edges,
  cross-repo), which sources feed claim C and is it stale, concern-scoped
  search ("error-handling paths in the store layer"), contradiction scan
  against a synthetic diff. FAIRNESS RULES, the mirror of the lookup
  bench's: (1) ground truths must NOT be index-derived alone — each is
  independently verified by hand against the repos and recorded with
  provenance; (2) the grep arm gets the identical prompt and may read any
  file (the docs, the plans, the CUE packs are all on disk — grep can
  legitimately try); (3) no task whose answer is literally unreachable
  without the store (that proves nothing about quality — mark those
  capabilities as unmeasurable-by-comparison instead of padding the score).
- **H2 pre-registration (BEFORE any run, committed in git).** Primary
  metric: task success. Criterion: the fathom arm must exceed the grep arm
  on success across the corpus (strictly more tasks correct), at
  ≤ 1× the grep arm's tokens (i.e. no token premium for winning). Record:
  "fathom wins home turf" requires BOTH; equal-or-worse success on
  index-native tasks = the pillar's pulled-tool story is refuted on its own
  ground and the docs say so within one slice. n=1 caveats carried like
  B1's.
- **H3 the run (op-gated).** Runs only after (a) Wave 3a's fixes land
  (O12's error-turn taxes would confound), and (b) the operator recompile
  fires (fresh obligations + first adr-rules/spec-bindings production
  claims + redeployed daemon — the current daemon serves 73-commit-stale
  claims, which would sandbag the fathom arm unfairly). Reuses the B1
  harness/scorer unchanged.

## Tasks

- [ ] Index-native corpus with hand-verified ground truths + provenance; fairness rules applied and documented per task {id: H1-1, symbol: cmdVerify, package: cmd/fathom, skip_test: true, skip: "corpus is data; the existing tested runner/scorer (B1-2) is the machinery"}
- [ ] Pre-registered criterion committed before any run (this plan's design section is the registration; the row flips only citing it) {id: H2-1, symbol: cmdVerify, package: cmd/fathom, skip_test: true, skip: "procedural registration row; evidence is git history ordering — criterion commit precedes results commit"}
- [ ] (op-gated: post-recompile + post-Wave-3a) The run: results artifact + verdict recorded here, honest either way {id: H3-1, symbol: cmdVerify, package: cmd/fathom, skip_test: true, skip: "measurement row; evidence is the committed artifact + this row's recorded verdict"}
