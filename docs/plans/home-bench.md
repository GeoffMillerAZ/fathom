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
  (O12's error-turn taxes would confound), (b) the operator recompile
  fires (fresh obligations + first adr-rules/spec-bindings production
  claims + redeployed daemon — the current daemon serves ≥112-commit-stale
  claims (112 at corpus authoring, growing; an earlier draft of this doc
  said 73 — the gap widens while OP-6 waits), which would sandbag the
  fathom arm unfairly), and (c) the recompile-exposed ground truths are
  re-derived against the post-recompile daemon and re-hand-verified at the
  run commit, per the corpus's `post_recompile_rederivation` block
  (tasks-home.json) — H1-T12's staleness verdict INVERTS post-recompile,
  so running without re-derivation would score the fathom arm wrong for
  being right. Re-derivation updates premises/ground truths only, in a
  committed corpus revision; the H2 criterion is never touched. Reuses the
  B1 harness/scorer unchanged.

## Tasks

- [x] Index-native corpus with hand-verified ground truths + provenance; fairness rules applied and documented per task {id: H1-1, symbol: cmdVerify, package: cmd/fathom, skip_test: true, skip: "corpus is data; the existing tested runner/scorer (B1-2) is the machinery"}
  - `testdata/contextbench/tasks-home.json` — 13 tasks (2 each of obligations, verify, blast_radius, claim_provenance, concern_search, staleness; 1 contradiction), each carrying the new `class` field (`internal/contextbench.TaskClass`, additive/optional — `Task.Validate` accepts absent-or-known, so the original grep-native `tasks.json` keeps validating unchanged). Every ground_truth hand-verified directly against fathom @ d1ead30 (grep/read, recorded per-task in `source`) — the live daemon (localhost:7575) and a scratch store built by `fathom ingest`/`serve-http -addr 127.0.0.1:7599` under `workspace/tmp/` were used for reconnaissance only, never as the sole evidence, per fairness rule 1. Rule 3 honored: a 4-item `unmeasurable_by_comparison` section (claim status with no citation given, compile-run cost ledger, claim_events refutation trail, exhaustive whole-index contradiction scan) lists store-only capabilities excluded from scoring rather than padded in. `internal/contextbench/task_home_test.go` (`TestLoadTasks_HomeCorpus`, `TestLoadTasks_HomeCorpus_PreRegistration`) proves corpus shape (12-15 tasks, no dup IDs, every task validates, all 7 classes represented) — `go test -race ./internal/contextbench/...` green. Critic round applied (merge-with-fixes verdict): T03's any-of extended from an arbitrary 5-of-16 subset to all 16 dispatching `TestCmdVerify_*` functions; T08 re-anchored to on-disk CLAUDE.md text (its original asked for a compile-time LLM scope decision underivable by grep, with a fathom-native citation format in the truth — rigged FOR fathom); T11 switched to any-of (prompt asks for a singular phrase); T12/T13 truths use collision-proof `verdict: <word>` tokens (bare `stale` is a substring of `staleness_report`, `fresh` of `not fresh`); and a mandatory `post_recompile_rederivation` block (T01/T02/T07/T12/T13, test-pinned) prevents H3's own recompile precondition from silently invalidating pre-recompile ground truths.
- [x] Pre-registered criterion committed before any run (this plan's design section is the registration; the row flips only citing it) {id: H2-1, symbol: cmdVerify, package: cmd/fathom, skip_test: true, skip: "procedural registration row; evidence is git history ordering — criterion commit precedes results commit"}
  - `pre_registered_criterion` block at the top of `testdata/contextbench/tasks-home.json`: metric `"task success"`, criterion = fathom strictly better on success AND ≤1× grep's tokens (both required for a win claim), `registered_before_any_run: true`, `registered_at_commit` pinned to d1ead30. `TestLoadTasks_HomeCorpus_PreRegistration` asserts the block's presence/shape so a future results commit can't land without it already having existed — this commit precedes any H3 results artifact, so git history itself is the registration evidence per the plan's design section.
- [x] (op-gated: post-recompile + post-Wave-3a) The run: results artifact + verdict recorded here, honest either way {id: H3-1, symbol: cmdVerify, package: cmd/fathom, skip_test: true, skip: "measurement row; evidence is the committed artifact + this row's recorded verdict"}
  - **Run 2026-07-24 (home-results-20260724T033131Z.json + home-summary-20260724T033131Z.md under `workspace/artifacts/context-bench/`): all 26 arm-runs real** (daemon reachable throughout, zero degradation), model sonnet both arms, 13 tasks — the full corpus, T12 re-anchored rather than dropped. All three gate conditions held before the run: Wave 3a's O12 fixes merged; the operator recompile fired (recompile-stamp 2026-07-24T02:20:30Z, resume-finish 02:48:59Z; live daemon serving obligations 122 fresh + adr-rules 26 + spec-bindings 97); and the mandatory re-derivation was committed FIRST as its own corpus revision (1d685a3 — T01/T02 provenance refreshed with ground truths unchanged, T06 corrected for SS-8's DuckStore deletion, T07/T13 re-anchored off the deleted duckdb.go, T11's 'single-writer' collision fixed, T12 re-anchored to a confirmed-stale-now claim after the predicted inversion did NOT occur: the original claim was crowded out of the served top-6 by the two new production layers instead of being re-cited fresh). **Verdict against the pre-registered criterion, honest and negative: fathom does NOT win its home turf. Success 12/13 vs 12/13 — EQUAL, not strictly greater (criterion 1 NOT met). Tokens: fathom 2,323,666 vs grep 2,169,058 = 1.071× — a 7% token premium (criterion 2 NOT met). Both halves failed independently. Per this plan's own consequence line: "equal-or-worse success on index-native tasks = the pillar's pulled-tool story is refuted on its own ground."** The failures offset: fathom uniquely failed H1-T08 (index DocChunk granularity cannot pin a sentence-start line number; grep read `cat -n` directly) and uniquely passed H1-T13 (recovering current ADR.md text at a cited line took 16 turns/601k tokens — the bench's priciest call — because the companion surface has no raw-text read). Fathom was cheaper on 9/13 tasks (best 0.27×, H1-T05 two-hop blast radius); the four expensive tasks (T08 2.56×, T10 2.10×, T12 2.59×, T13 5.20×) — precisely the ones lacking a direct evidence path on the 14-tool read-only surface — erased the aggregate. Zero protocol errors / empty stderr / zero is_error envelopes: capability truth, not harness artifact (T13/grep's FAIL audited — quoted ADR-014 from outside the required line range where only ADR-010 lies inside; legitimate miss, symmetric prompt). B1's caveats carried: n=1 per task per arm, single machine, end-to-end tokens include the symmetric session baseline, dollar cost not registered. Follow-up owed by the program (one slice, per H2): propagate this refutation to the docs where the pulled-tool/home-turf story is asserted.
