# 26 — Proof credibility: a bench a stranger can re-run, instruments that cannot lie, and done-means-proven at the claim

> Owner ask, 2026-09-01: *"the next most effective areas of opportunity …
> benchmarking, additional features, better features … what are the
> optimizations? the big brain moves? the wise moves?"* Method: the same
> 14-agent evaluation as `25-harness-port.md`. Lanes cited: E1 landscape,
> E3 field patterns, E4 bench corpora, I2 bench machinery, I4 open rows,
> I5 adoption surface.
>
> Status: ACCEPTED 2026-09-01 as ADR-030 (lexical oracle) and ADR-031
> (bench credibility). Plans: `../plans/proof-credibility.md`,
> `../plans/bench-public.md`, `../plans/blast-depth.md`.

---

## 0. The verdict in one paragraph

Fathom's differentiation survived the survey intact and alone: nobody
verifies "done" deterministically against an index, injects cited
obligations at edit time, or expires knowledge by symbol drift (E1). Its
bench *method* — pre-registered machine criterion, tri-state verdict,
paired ratios, replication floor, committed ledger — already exceeds the
field, where the closest published analog is graded by its first author
in a single run with no confidence interval (E4 F10). What is weak is
*credibility*: every ground truth is derived from this private repo, the
home corpus has drifted false at HEAD, the ledger the last ADR calls
"committed" has no rows, and the open bench rows already pass the verify
gate because their anchors pre-exist (I2). Meanwhile the gate's own
verdicts depend on which binary built the store and nothing says so: five
of today's claimed-unproven rows are false failures from a stale daemon
(I4 F1). So the program is: **make the instruments incapable of lying
about themselves, then make the bench re-runnable by an outsider, then
bind done-means-proven to the instant of the claim.** Every number leaves
the repo with its denominator in the sentence.

## 1. Instruments that cannot lie (plan PC, wave 1)

### 1.1 Store provenance on gate verdicts (O20, O5)

The verify gate's verdict is a function of the store, and the store is a
function of the binary that built it. Today `fathom verify` against the
daemon's store and against a HEAD-built probe store give different
answers for the same tree (70/5 vs 75/0, reproduced 2026-09-01) and
doctor's `binary_fresh` compares the *local* binary to the *repo's* HEAD
(O5: the wrong repo's HEAD in every consumer). The fix is structural:

- `/statusz` carries `binary_commit`, `binary_dirty`, `built_at`
  (additive; `schema_version` bump).
- Every verify-ledger row carries `daemon_commit` and `store_commit`; the
  CLI prints one provenance line when the daemon's commit differs from the
  local binary's `vcs.revision` — never from the cwd repo's HEAD.
- Doctor gains `daemon_fresh` (daemon binary vs local binary) and
  `binary_fresh` is re-scoped to what it can know.
- `plans status` and `intent audit` mark rows whose newest reading came
  from a stale daemon as **measured-stale**, not failing. A stale reading
  is a coverage gap (ADR-027), not a refutation.

### 1.2 The honesty sweep

Small, mechanical, and each one a lie the instrument currently tells:
the U2 remedy promises a `skip:` reason lifts the debt while the counter
never honors it; scope G6 cites a thread closed 2026-08-26; DP-2 is
subsumed by EP-9; EP-44's `needs:` cannot cross plans; three ADR
citations point at files that do not exist at HEAD; six router surfaces
name no `Test*` in "Done needs"; doctor reports `list_modules` as *tool
not found* because it calls it on the wrong surface; eight handoff
threads list only shipped work; O20 is uncommitted with root cause TODO.
All fixed in one wave with the numbers in the `Verified:` line.

### 1.3 A deterministic proof for non-code rows (ADR-030)

Eighteen of fathom's claimed-unproven rows are `[x]` rows with a
`skip_test` reason and no symbol: docs, measurements, changelog lines,
operator actions. The verify ledger shows a second puller — an external
repo's plans at 9 claimed / 0 verified and 37 / 12 — whose rows are
mostly non-code (I4 F8). The grammar already accepts the ADR-026 oracle
vocabulary in its tokenizer. So: a plan row may carry
`{oracle: lexical, path: <repo path>, pattern: <regex>}`; verify checks
the path exists at HEAD, is non-empty, and the pattern matches; the row
counts as verified and leaves U2. Lint refuses a markdown path without a
pattern (a path that merely exists proves nothing about a claim). One
evaluator, the one ADR-026 already has; no second grammar (Q20 named, not
decided). Kill: if more than 30% of new lexical rows point under
`docs/vision` or `docs/design`, the lint tightens to measurements,
changelog, tests and specs — aspirational prose is never proof.

### 1.4 Wiring drift covers what an agent could be made to change

The year's worst agent incidents were failures of trust configuration,
not detection (E3 F7): a clean package that turned exfiltrating after
fifteen versions, skill frontmatter hooks that register on invocation, an
editor's auto-run allowlist bypassed. Fathom already digests the stanza
regions and hook entries it owns (law 15) and `fleet reconcile` reports
drift without merging. The critics' reframe of the first draft (a separate
lock file and a principal check) was right: a second lock file widens the
committed file list for no new evidence, and a principal check is a
heuristic. So the existing digest extends to the two things it did not
cover — MCP server entries (argv and env *names*, never values) in the
project and user MCP files each descriptor names, and fathom-owned skill
trees — reported by kind and path, never merged; one doctor row
`wiring_drift` over hooks, stanzas, MCP entries and owned skills (PASS at
zero drift, FAIL naming each, SKIP when a file is unreadable per I38); a
custodian concern and a session attention line. The `ConfigChange` hook
(§4) is the harness-side trigger.

### 1.5 The rest of the sweep the critics found missing

One canonical verify figure with its window and denominator computed from
the ledger (the three circulating figures are the O6 double-count family;
none may leave the repo until this exists). Three rung lifts named in
`invariants.md`: a denylist test for I19, a `## Maintained` parser test for
I36, `fathom config audit` in `task validate` for I16/I34/I37. I31 stops
being VIOLATED: `-store` and `-call-log` defaults resolve under `workspace/`
or `~/.fathom`, never the cwd. Q21 gets a drafted, `proposed` amendment of
ADR-016's obligation ceiling with the measured numbers — the owner decides,
the record is agent work. And `fathom verify --format sarif`, so a refusal
renders in code scanning and editor problem panes for free.

## 2. A bench a stranger can re-run (plan PB, ADR-031)

### 2.1 Truth at HEAD first

The home corpus is false at HEAD: one ground truth names a rule that
moved, one is inverted, three premises moved (I2 F4), and one all-of
token is the substring `29`, which matches any number containing 29.
The first `--repeat 3` run (BC-14, operator-gated spend) cannot be
honest until a committed re-sweep revision lands. The ledger row shape
also cannot support the regression gate: no per-task outcomes, no corpus
hash, alias model name, no cost. Additive fields fix it
(`per_task`, `corpus_sha256`, `model_id`, `cost_usd`, `dirty_tree`). The
runner gains `--resume` (one lost cell in 78 calls currently forces a
$13 re-run), a cell-loss count, and subject parameterization
(`--subject-root/--service/--fathom-url/--prompt-subject`, opt-in
`--scratch-daemon`) because both system prompts hard-code "the fathom
codebase" and the fleet daemon is implicit. And BC-14/15/16 are
re-anchored to symbols that do not exist yet, because today flipping them
to `[x]` passes the gate with zero new work (I2 F5) — done-means-proven
is blind to a measurement row anchored on a pre-existing symbol.

### 2.2 Method the field respects

Three lessons the design lacked, each cheap and pre-registrable (E4):

- **Memorisation floor.** Frontier models name the buggy file from issue
  text alone 76% of the time on SWE-bench vs 53% off-bench. A `none` arm
  (all tools disallowed) measures it; a task the none arm passes in ≥2 of
  3 trials is `leaked` and excluded by a rule committed in
  `machine_criterion` before the run.
- **Precision term.** `all-of` substring scoring is passable by dumping
  the tree. Answers go in a fenced block, a task carries
  `max_predicted` (default `max(5, 2·|gold|)`), and the score carries
  precision and recall beside pass/fail. Test files are excluded from
  gold, as the localization papers do. Because this changes the success
  definition after H2-1 has data, it is registered as a new criterion
  **H2-2** in the corpus; H2-1's recorded verdicts stand as history (T5).
- **Statistics.** Seeded, task-clustered paired bootstrap 95% CI on the
  success difference; exact Wilcoxon signed-rank for n ≤ 25;
  Vargha–Delaney A12; `pass@1` and `pass^3`. Reported in the row, never
  gating: the pre-registered criterion (H2-1 plus the replication floor)
  stays the gate. These primitives live in **agentic-kit**
  (`foundations/quality/reliability-stats`), because fathom has now
  hand-built paired statistics twice (BC-4 and this wave) and the kit's
  own eval-runner wants `pass^k` — VISION's "hand-built twice → the kit
  owns it" fired.

### 2.3 The corpus (BC-15): derived, not authored

Since 2025 the field converged on a task shape fathom can adopt verbatim:
*issue → the files and functions the developer actually changed*, read
off the merged gold patch at a pinned base commit (E4 F1). That truth is
developer-authored, license-clean as metadata, needs no container or
test execution, and maps onto fathom's `find_symbol` / `references` /
`find_callers` classes. Go slices exist: SWE-bench Multilingual (Go 42),
Multi-SWE-bench (CC0, JSONL; `cli/cli`, `grpc-go`, `go-zero`),
SWE-Explore (Go 84, line-level truth, MIT, an `eval_runner` that accepts
registered explorers). So `contextbenchscore import` derives
`tasks-public-go.json` with no LLM from two pinned sources — SWE-Explore's
Go subset for line-level localization truth (an outsider built it) and
Multi-SWE-bench (CC0) gold patches for developer-authored file and
function truth and for callers truth — under a pre-registered selection
(gold patch touches 1–5 non-test files; sort by `instance_id`; first
30–40),
`file_localization` (diff paths, all-of with cap), `function_localization`
(hunk-enclosing functions via `go/parser` at `base_commit`),
`find_callers` (a committed `go/types` script as truth). Each task carries
its derivation and a `requires` list; the derivation reproduces
bit-for-bit (`corpus_sha256`) from a clean clone. The runner ingests each
instance's `base_commit` snapshot into a scratch store under
`workspace/`, never the fleet store and never the post-fix tree (index
leakage is the bench-specific hazard). Home-corpus tasks that depend on
the LLM-compiled obligations layer are split from deterministic ones so a
public subset is honest.

What is built now: the importer, the runner parameterization, the
scoring, the runbook. What waits on a human: the run (real spend, ~35M
tokens at n=3 across three arms), and publishing — a third party needs
the binary, which needs the repo public or a release (Q4, F2). The
follow-on that turns "reproducible" into "scored by an outsider" is
registering *Claude Code + fathom MCP* as an explorer in SWE-Explore's
Go subset.

### 2.4 Regression (BC-16)

`contextbenchscore regress -ledger [-baseline run_id]`: newest row vs the
previous row with the same `corpus_sha256`; any task whose treatment
success rate fell is flagged; flaky tasks are listed, not failed; added
tasks are not regressions; exit 0/1/3. Semantics mirror the kit's
eval-runner `Regression` (a third convention was the wheel warning). It
joins `validate` only after two same-corpus rows exist; until then it is
`task bench-regress`, free to run.

## 3. Depth and confidence in the blast radius (plan BD)

The one product shape the commodity graphs made expected and fathom
lacks: GitNexus groups blast radius by depth with per-edge confidence
("depth 1 WILL BREAK / depth 2 LIKELY AFFECTED"); Graphify tags every
edge `EXTRACTED` or `INFERRED`; code-review-graph names affected tests
and test-gap risk per diff (E1 F2, F3). Fathom already has the facts —
typed edges vs syntax-pass edges, `test_impact`, `diff_impact` — and
does not say them. Additive response fields, no new tools (law 4):
`find_callers`, `references` and `blast_radius` results carry
`depth` and `edge_confidence: resolved | heuristic` per edge (typed pass
= resolved; syntax-only = heuristic), with a `by_depth` summary;
`review_diff` composes `test_impact` to name the tests that reach the
changed symbols and the changed symbols no test reaches (`test_gap`).
The `blast_radius` use-case doc's "Maintained" line and
`docs/mcp-tools.md` change in the same commit (I11).

## 4. Done-means-proven at the claim moment (plan PC, wave 3)

The verify gate is the product spine (180 catches in 436 runs over five
days at the last audit) and it fires at push. The first draft of this
section bound it to Claude Code's `TaskCompleted` event; the verified
contract shows that event belongs to the built-in task list, which is
absent by default on current models. The claim moment in fathom's model is
the `[x]` itself — an Edit on a `docs/plans/*.md` file — and the edit hook
already sees every one of those. So:

- **Claim-moment verify on the existing edit hook.** When a PostToolUse
  edit touches a depth-1 plan and adds a `[x]`, the hook runs the verify
  path for that one plan and injects the refusal, if any, as
  `additionalContext`. Silent on a clean claim. Ledger rows carry
  `origin: claim-moment`. Cost: one plan verify on the edits that claim,
  and nothing on the rest.
- `InstructionsLoaded` → `fathom session-hook --event instructions-loaded`:
  a silent beat naming the file the harness loaded and why — the first
  *harness-side* evidence that the stanza reached a session (I37 today
  reads the file, not the harness).
- `ConfigChange` → `fathom config audit --hook`: an attention line on the
  next session line, never a block (a block surfaces no message there).
- `TaskCompleted`: deferred, with the trigger "the task-list tools are
  enabled fleet-wide".

Pre-registered: ≥1 ledger row with `origin: claim-moment` that refused
within 14 days of the fleet apply, else the claim-moment row is
dispositioned deferred. The two new hook rows are additive entries in the
one hook table, carried by init, plugin render, reconcile and doctor.

## 5. Recorded, not built (futures)

- **Anchored-memory export into harness memory formats** (Copilot's cited
  JIT-verified memory, Claude Code's typed `MEMORY.md`): the thesis is
  validated externally, but ADR-028's boundary and a second memory
  surface both argue for waiting. Future F19.
- **Spec-kit / OpenSpec `tasks.md` as plan rows and a "code nothing claims"
  converge report**: the scaffolds emit checklists fathom could gate
  deterministically. Future F20; promote when a wired repo uses one.
- **Incident window query** (changed symbols ∩ symptom call graph) as a
  mode of `investigate_incident`: promote when the incident use case has
  a consumer session.
- **Fleet selection manifest for external batch runners**: declined, no
  consumer, and the dispatch boundary makes it a one-line `--json` if one
  appears.
