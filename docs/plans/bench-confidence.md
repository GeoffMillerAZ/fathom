# Bench confidence — repetition, pairing, and a committed ledger

The context-bench harness was never the weak part. Pre-registered criteria,
an honest negative verdict published against them, hand-verified ground
truth, fairness rules, and a documented protocol call in the audit is better
methodology than the agent-memory projects fathom competes with for
attention publish. Three things were weak, and none of them is method:

1. **n=1.** Every run to date executed one trial per (task, arm) cell. A
   task that passes two thirds of the time and a task that always passes
   were indistinguishable, and the per-task token figures the 2026-07-24
   verdict turned on had no measured spread at all.
2. **Totals only.** One task at 5.20× erased a win on 8 of 13 tasks. The
   audit that followed had to reason by hand about which perturbations were
   individually defensible — reasoning that belongs in code.
3. **Nothing committed.** Results lived under gitignored `workspace/`. A
   benchmark whose numbers only its author can see is a claim, not evidence,
   and no trend across runs was recoverable.

Automation was considered and **rejected as the lever**: a home run costs
real LLM tokens (~4.5M end-to-end across both arms at n=1, three times that
at the new floor), so it cannot go in `validate`, a hook, or CI without
violating `docs/intent/autonomy.md`'s ask-before-LLM-spend rule. Cadence is
operator-initiated by design. What automation buys is *scoring* — the verdict
is computed from committed criteria, never asserted — and that part is done.

Registered 2026-09-01, before the next run: a **replication floor of 3
trials** on the home corpus. It is new — the original H2-1 registration had
none. Runs below it score `indeterminate`, not `pass`. The 2026-07-24 and
T5-8 runs are therefore indeterminate under it going forward; their recorded
verdicts stand as accurate history of an n=1 instrument, and are not
retroactively rewritten.

## Tasks

- [x] Per-trial results: `ArmResult` carries a 1-based `Trial`, additive and omitempty so every artifact written before repetition existed still parses {id: BC-1, symbol: ArmResultFromClaudeJSON, package: internal/contextbench}
- [x] `--repeat N` in the runner: N trials per (task, arm) cell, per-trial raw and scored artifact paths, linear cost disclosed in the script header {id: BC-2, symbol: LoadTasks, package: internal/contextbench, skip_test: true, skip: "shell runner; the Go side it drives is BC-1/BC-3 and is tested there. bash -n is enforced by task scripts-check"}
- [x] Per-cell statistics across trials: mean, median, population stddev, and a `Flaky` flag for a cell that neither always passed nor always failed — the signal n=1 could not produce {id: BC-3, symbol: StatsByTaskArm, package: internal/contextbench}
- [x] Paired per-task comparison: per-task token ratios, a median ratio an outlier cannot move, and explicit counts of which arm was cheaper where {id: BC-4, symbol: Pair, package: internal/contextbench}
- [x] Coverage honesty: a task present in one arm only is reported in `Absent`, never dropped, so a crashed or skipped cell cannot improve the surviving ratio (ADR-027) {id: BC-5, symbol: Pair, package: internal/contextbench}
- [x] Machine-evaluable criterion committed in the corpus beside its prose registration, read from the corpus and never passed on the command line, with its values pinned by a test so loosening is a visible edit {id: BC-6, symbol: LoadCriterion, package: internal/contextbench}
- [x] Tri-state verdict: an incomplete or under-replicated run is `indeterminate`, never `pass`; `fail` stays reachable and distinct as a measured refutation {id: BC-7, symbol: Criterion, package: internal/contextbench}
- [x] The aggregate ratio is what the criterion reads; the friendlier median is reported for interpretation only, pinned by a test so a bar cannot be softened after the data arrives (law 12) {id: BC-8, symbol: Evaluate, package: internal/contextbench}
- [x] Committed JSONL ledger, one row per run, appended never rewritten; an unparseable line errors with its line number rather than being skipped into a healthier-looking history {id: BC-9, symbol: AppendLedger, package: internal/contextbench}
- [x] Ledger row is built from the paired summary it was judged over, carrying `Absent` and `FlakyTasks` beside the verdict so the row cannot read cleaner than the run was {id: BC-10, symbol: NewLedgerRow, package: internal/contextbench}
- [x] `contextbenchscore ledger` pairs, evaluates, appends, and exits 0/1/3 for pass/fail/indeterminate; the runner calls it and refuses to score a corpus with no registered criterion {id: BC-11, symbol: cmdLedger, package: cmd/contextbenchscore, skip_test: true, skip: "flag-wiring binary over the tested internal/contextbench library, per B1-2's established precedent for this command"}
- [x] `docs/bench/README.md` explains how to read a row, why two ratios are reported, and what `indeterminate` means; the results table is re-rendered from the ledger by a free read-only command {id: BC-12, symbol: LoadLedger, package: internal/contextbench}
- [x] `task bench-home` / `bench-lookup` / `bench-ledger`, with the token cost stated in the description and none of them wired into `validate`, a hook, or CI {id: BC-13, symbol: RenderLedgerMarkdown, package: internal/contextbench, skip_test: true, skip: "Taskfile entries; scripts-check lints the shell they call"}

### Open — needs the operator

Re-anchored 2026-09-02 (PB-7, ADR-031): the three rows below were anchored
on `Pair` / `LoadTasks` / `LoadLedger` — symbols that already existed — so
flipping any of them to `[x]` passed the gate with zero new work (I2 F5).
Each now names the symbol only the work itself produces. `TestLedgerHasReplicatedRow`
is deliberately NOT written yet: a skipped test still statically reaches its
anchor, so writing it early would re-open the hole; it is written by the
operator run that produces the row it asserts.

- [x] (op) First `--repeat 3` home run against the registered floor, ledger row committed, verdict honest either way (run-20260913T002832Z: FAIL under H2-1 and H2-3, O31); `TestLedgerHasReplicatedRow` asserts a row with `min_trials >= 3` exists and is written in the same change as the row {id: BC-14, symbol: TestLedgerHasReplicatedRow, package: internal/contextbench, skip_test: true, skip: "the test is the anchor; it does not exist until the replicated row does — fathom verify reports this row unproven until then"}
- [x] A public, third-party-reproducible corpus: every ground truth here is derived from *this* repo, so no outsider can re-run the bench and get a number. `RequiresSatisfied` (PB-12) is the split that lets a deterministic subset run on a scratch daemon; the row flips only with `bench-public.md` PB-19's public-corpus ledger row (`TestLedgerHasPublicRow`), not on the split alone — 2026-09-17: closed on the Long Code Arena bug-localization corpus (testdata/scenariobench/lca/lca-bugloc.json, 40 public tasks with public gold; lca-run-1 filed as 78 ledger rows with corpus_sha256, paired CI includes zero, docs/bench/readings/lca-run-1-reading.md) — the third-party-reproducible corpus this row asked for; bench-public.md PB-19, the separate three-arm public-Go run on `TestLedgerHasPublicRow`, stays open on its own track {id: BC-15, symbol: RequiresSatisfied, package: internal/contextbench}
- [ ] Regression gate: `contextbenchscore regress` (`cmdRegress`, PB-13) compares the newest ledger row against the previous same-corpus row and fails on a per-task treatment-success regression; built, but the row flips only once two same-corpus rows exist and `task bench-regress` has compared them {id: BC-16, symbol: cmdRegress, package: cmd/contextbenchscore, needs: BC-14}
