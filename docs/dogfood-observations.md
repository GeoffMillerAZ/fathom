# Dogfood observations — performance & recall

A living log of how sextant's MCP tools behave **in real fleet sessions** (throughline,
agentic-kit, app-kit, sextant). Kept current by whoever is dogfooding the index: every
session that hits a recall gap, a latency cliff, a contract mismatch, or a pleasant
surprise records it here while the evidence is fresh. This is the feeder queue for
sextant's own roadmap — observations graduate into `tmp/TRACKING.md` / `ADR.md` /
`CHANGELOG.md` as fixes land.

**Discipline (truth-seeking, same as the product):** never log a "tool is broken" claim
without a reproduction. Probe a known-good case alongside the failing one, root-cause the
difference, and write down both. The first instinct ("find_symbol can't find methods")
was wrong this round — the probe proved the running server was stale, not the code. An
unverified complaint is noise; a root-caused observation is a fix.

Entry shape: **symptom → evidence (verbatim probe) → root cause → fix → status → impact.**

---

## Open

### O29 — `fathom plans status` takes 2m35s: a status command priced like a full audit (2026-08-13)

- **Reproduction (verbatim):** `time fathom plans status --service fathom`
  → `0% cpu 2:35.45 total` against the healthy always-on daemon; ~40
  active plans, each verified serially over HTTP (`computePlanRows` calls
  `verifyViaHTTP` per plan with a 10s-timeout client; near-zero CPU
  locally = the whole elapsed time is round-trip waiting). The command
  produced correct output — this is latency, not wrongness.
- **Why it matters beyond annoyance:** SD-2 (docs/plans/custodian.md)
  commits the observatory's SDD panel to computing at render via this
  same engine. A 155s render is a non-starter, so the panel's design
  inherits this observation as a prerequisite: parallelize the per-plan
  verify calls (they are independent), or add a daemon-side plans-status
  endpoint that runs the engine in-process against its own store, or
  both. Also worth checking: whether each `verifyViaHTTP` re-triggers
  auto-refresh work that the first call already paid for.
- **Fix direction (concrete):** daemon-side `plans_status` (engine
  in-process, one HTTP call total) is the right shape — the CLI and the
  panel both render what the daemon computes (law 10). Parallel fan-out
  in the CLI is the cheap interim.
- **Status: RESOLVED (interim, 2026-08-14)** — the per-plan verifies now
  fan out (bounded 8-way) in `computePlanRows`; measured after deploy in
  this repo. The daemon-side single-call `plans_status` remains the right
  end shape for SD-2's render path and stays on that row.


### O28 — the first real brief was telemetry pretending to be a handoff (FIXED same day)

- **Field report (2026-08-13, the idp-and-warrants session, verbatim):**
  *"Continuity threads are thin — just 'session ended with N dirty files',
  no substantive handoff… What actually restored my context was the
  durable spec plane: git log, the two plan files, and fathom verify
  printing exactly which rows are claimed vs. proven. That last one is
  doing the real continuity work."*
- **Diagnosis — one cold-start fact and three v1 defects:**
  1. (Cold start, not a bug) No prior session ever filed a
     `fathom thread note`; the write side needs the departing agent, and
     the instruction only reached the fleet's stanzas hours earlier.
  2. Session-end deposits ACCUMULATED (unique id per /clear) — the pile
     was destined to be 100% of every brief within a week.
  3. The brief led with OTHER repos' dirty-file telemetry — cross-repo
     noise ranked above anything about the repo being opened.
  4. The brief pointed AT the derived spec state instead of CARRYING it —
     while the reader proved the spec plane is what restores context.
- **Fixes (same day):** deposits supersede per repo (stable id) and close
  themselves when the tree comes clean (`repo_clean` condition); the brief
  leads with the local plans summary (checked-vs-open, with "verify
  distinguishes claimed from PROVEN" said explicitly), foreign telemetry
  collapses to one counted line, and a thin brief TEACHES the write side
  instead of presenting telemetry as a handoff.
- **The validating part:** the reader independently confirmed the design's
  own core doctrine — "derived beats deposited" — against the v1
  implementation that had not honored it. First-contact feedback caught in
  under an hour by exactly the flywheel this program exists to run.
- **Status: FIXED** (this commit); the cold-start half resolves through
  use.


### O27 — capabilities and doctor disagree about the same facts (fleet sweep, 2026-08-13)

- **Symptom:** the 16-repo verification sweep found four cross-check
  inconsistencies between `fathom capabilities` and `fathom doctor` run
  against the SAME repo at the same moment:
  1. workstation_as_code: capabilities reports obligation rules absent
     while doctor's `obligations_served` PASSes.
  2. app-kit: doctor's summary says "4 capabilities operating" while the
     capabilities verb says 3 of 9.
  3. platform-engineering: capabilities shows guards ✗ while doctor's
     `guard_file` finds 7 rules.
  4. hooks_firing may PASS against sextant's own call log for ANY repo —
     a false-positive direction worth auditing.
- **Why it matters:** two surfaces answering the same question differently
  is worse than either being wrong alone — a reader cannot tell which to
  trust, which re-opens the exact "why isn't X happening here" gap the
  registry was built to close. Likely causes: the probe and doctor read
  different sources for the same fact (the O24 class), or count different
  things under one name.
- **Status: OPEN** — reconcile each pair to a single source of truth;
  capability detection should CONSUME doctor's checks (or vice versa),
  never re-derive them.


### O24 — the transcript reader hand-copied three fields, silently killing four instruments for weeks

- **Symptom (2026-08-12, three sessions into a 20-session counterfactual
  run):** asked "are we getting good data?", the answer was **zero
  admissible OFF-arm sessions**. `arm_coverage` read `unknown` on every
  row, `tokens_in`/`tokens_out` were absent, `first_edit_turn` was absent,
  and the exploration ledger was empty.
- **Root cause:** `transcriptSummaryFrom` copied three field pairs out of
  `summarizeTranscriptBytes` by hand. Every field added to
  `transcriptSummary` afterwards was computed by the parser and dropped
  three lines later — twelve fields in total.
- **Why it survived weeks of dashboards:** each absence surfaced as
  *"rows predate the instrument"* — plausible, self-consistent, and wrong.
  Nothing downstream could distinguish "not implemented yet" from
  "implemented and discarded". The brief, the observatory and the flywheel
  all reported the same false-but-coherent story.
- **Fix:** whole-struct assignment, so a new field is carried BY
  CONSTRUCTION. `TestTranscriptSummaryCarriesEveryField` asserts over the
  struct by reflection rather than a hand-listed set, so it cannot drift
  the way the copy did. Reverting the fix makes it name all twelve.
- **Status: FIXED** (`ac46e4f`, MI-4).
- **Generalization worth keeping:** any place that copies fields between
  two shapes of the same data is this bug waiting to happen. Prefer whole
  -struct assignment; where a subset is genuinely required, assert the
  carry structurally rather than by listing fields.

### O25 — the documented deploy command produces a binary macOS refuses to run

- **Symptom (2026-08-12):** the owner ran the deploy command quoted from
  `CLAUDE.md` (`cp bin/fathom ~/.local/libexec/fathom/fathom && launchctl
  kickstart -k …`). The copy succeeded (76 MB, correct timestamp) and the
  daemon then stayed down. `launchctl print` showed
  `last exit reason = OS_REASON_CODESIGNING`; the binary would not execute
  at all. Every fathom tool was unavailable fleet-wide until it was
  noticed.
- **Root cause:** `task deploy` (O10, 2026-08-08) signs the installed copy
  with the persistent `fathom-dev` identity, writes via a temp path +
  rename (a `cp` over a RUNNING executable truncates in place and can
  fault the live process), and then ASSERTS the running process is the
  freshly built binary. `CLAUDE.md` still documents the older raw-`cp`
  sequence, which does none of that.
- **Aggravating factor:** the stale instruction was repeated confidently
  by an agent (me) several times across one session, because CLAUDE.md is
  treated as authoritative and was never re-checked against the Taskfile.
- **Fix required:** update `CLAUDE.md`'s deploy stanza to `task deploy`,
  and state plainly that the raw `cp` is unsafe (unsigned + in-place
  truncation of a running binary).
- **Status: OPEN** — doc fix pending.

### O26 — long-lived resumed sessions break BOTH session instruments at once

- **Symptom (2026-08-12):** no OFF-arm session could ever be admissible,
  independently of O24. The owner's working style is one Claude Code
  session resumed across days; the live transcripts are 17–120 MB and one
  had been running since ~2026-08-02.
- **Two independent failures:**
  1. **Coverage is permanently `partial`.** `armCoverage` compares the
     transcript's first timestamp to the arm's `set_at`. A session that
     began before the arm carries assisted work, so it is correctly
     refused — but a resumed session NEVER starts after the arm, so it is
     refused forever.
  2. **Totals are permanently truncated.** `transcriptReadMaxBytes` is
     16 MB. The live transcripts were read at 14%, 35%, 88% and 99%. The
     row discloses `transcript_truncated: true`, but `Truncated` is not
     referenced anywhere in the impact fold — so a 14% parse yields a
     `tokens_total` consumed as if it were the session's total.
- **The deeper question this raises:** "per session" may simply be the
  wrong denominator for how this fleet is actually worked. Per-day or
  per-turn-window would measure something that exists. That is a design
  decision, not a bug fix, and belongs to the owner.
- **Status: OPEN.** Immediate remediations: honour `Truncated` (never feed
  a partial total to a median — law 3), raise or remove the cap, and
  decide the unit.


### O23 — `fathom doctor` cries wolf in every repo that is not fathom: three repo-relative assumptions that only hold at home

- **Symptom (2026-07-31, first `doctor` run inside a freshly-adopted repo):**
  after O22's scaffold fix, `doctor` in a normal adopter repo still reports
  three FAILs that are all doctor's own bugs, not the repo's:
  1. `binary_fresh FAIL — binary @ a0b74df3 does not match HEAD 061afdba`.
     `061afdba` is the ADOPTER's HEAD. The binary is fathom's, built from
     fathom's tree; comparing it to the doctored repo's HEAD is meaningless
     and fails in literally every repo except fathom.
  2. `hooks_firing FAIL — call log <adopter>/workspace/logs/mcp-calls.jsonl
     not found — no hook has ever written to it`. The call log lives with
     the DAEMON (fathom's workspace, or the configured `[daemon].call_log`),
     not under the repo being doctored. Hooks were in fact firing.
  3. `obligations_served FAIL` because it probes `CLAUDE.md` specifically,
     and this repo's compiled rules are code-scoped (`**/*.go`). Verified
     directly: the same service serves **6 rules on `cmd/conjure/main.go`**.
     The message is honest about the ambiguity ("or CLAUDE.md specifically
     is not in any compiled rule's scope"), but as a *check* it reports FAIL
     for a correctly-working install.
- **Root cause (one sentence):** `doctor` was written and exercised inside
  fathom's own repo, where "the repo I am doctoring" and "the repo the
  binary and daemon came from" are the same directory. They are different in
  every adopter, and nothing tested that case.
- **Why it matters now:** this is the **fourth** onboarding-path defect in a
  row (init hook duplication → the `?:` scaffold inversion → O22's invalid
  scaffold → this). The owner is adopting fathom across a fleet right now,
  and a health check that shows 4 red rows on a correctly-wired repo trains
  people to ignore it — the precise failure O22 already warned about.
- **Fix direction:** (1) compare `binary_fresh` against the FATHOM source
  tree the binary reports, not the cwd repo — or skip with a note when the
  doctored repo is not fathom itself; (2) resolve the call log the same way
  the daemon does (`[daemon].call_log`, else the daemon's workspace), never
  relative to the doctored repo; (3) probe `obligations_served` with a file
  that is actually likely governed — try several candidates (a source file
  from the service's dominant language, then CLAUDE.md) and report which one
  answered, or report "no rule scopes match any probe" distinctly from "never
  compiled". **Test the multi-repo case explicitly:** run doctor against a
  scratch adopter dir that is NOT the fathom tree and assert zero false FAILs.
- **Status:** open, next adoption slice. Until fixed, read doctor's output in
  a non-fathom repo with these three rows discounted.

### O22 — `fathom init`'s own scaffolds fail `fathom doctor` on every fresh adoption

- **Symptom (2026-07-31, wiring a real new repo):** `fathom init` writes
  `.fathom/guard.cue` and `.fathom/modules.cue` as fully-commented, inert
  templates — correct and deliberate (nothing should apply until the author
  uncomments and commits). But the loaders require the top-level field, so a
  fully-commented file is *invalid*, not *empty*:
  `binding_file FAIL … field "bindings": required, MUST be present` and the
  same for `guards`. Four dependent checks then SKIP. **Every repo that runs
  `fathom init` gets two immediate FAILs plus four skips from files fathom
  itself just wrote.**
- **Why it matters:** this is the third defect in a row on the *onboarding*
  path (after init's hook duplication and the scaffold's `?:` optional-field
  inversion). A first-run experience where the tool's own output fails the
  tool's own health check is the single most corrosive kind of bug — it
  teaches a new adopter that doctor's FAILs are noise, which is exactly the
  opposite of what the never-silent doctrine needs them to believe.
- **Correct semantics (the fix):** three states, not two.
  *absent* → "no config" (already PASS);
  *present but declaring nothing* (all-commented / empty) → "scaffolded,
  nothing configured yet" → PASS with that note;
  *present and malformed or wrongly typed* → FAIL loudly, unchanged.
  The loaders should treat a missing top-level field as an empty config, and
  doctor should report the scaffolded state distinctly from both "unconfigured"
  and "broken".
- **Test that pins it (mirroring the `?:` precedent):** load the ACTUAL
  `ExampleTemplate()` bytes — the exact scaffold `init` writes — and assert
  they load clean and doctor-pass. The current tests validate hand-written
  fixtures, which is why the real scaffold's shape was never exercised.
- **Status:** open, first row of the next adoption slice.

### O21 — the findings layer cannot be eval-gated with the fixture convention every other layer uses

- **Symptom (2026-07-30, first real run of the findings gate):**
  `FATHOM_EVAL_LLM=1 go test -run TestFindingsEval_Precision` aborts before
  scoring: `finding drift anchor: indexed file internal/store/pool.go has
  no stored fingerprint`. No precision/recall number was produced.
- **Root cause:** the findings layer is the only layer that records
  **scope-fp links** — the `code_drifted` remediation-drift mechanism
  (FT-6, docs/vision/03-findings-and-experts.md §2) — so compiling a
  finding requires every file in its `scope` to already carry a stored
  fingerprint. `adr-rules`/`obligations` fixtures have no such requirement,
  and the findings fixtures were authored to that (insufficient) shape.
  This is a real asymmetry in the layer contract that the fixture
  convention does not express.
- **Why it matters:** phase R's tier-promotion path is gated on this eval
  (`docs/design/findings-as-evidence.md` §3.5). Until the gate can run,
  patterns cannot be promoted — correctly, since `PromoteTier` refuses
  structurally when `!layerEvaled`, so the failure is safe, just blocking.
- **Fix direction:** either the shared eval harness seeds the scratch store
  with fingerprints for the fixture's scoped files (preferred — keeps
  fixtures free to scope realistically), or the fixtures scope only to
  files the harness already indexes (cheaper, less faithful). Whichever is
  chosen, `docs/layers.md` should record that findings has an extra fixture
  requirement so the next layer author does not rediscover this.
- **Status:** open, first row of the next findings slice. Nothing about
  findings precision may be claimed until the gate scores.

### O20 (PART 2 CLOSED 2026-07-30) — two research-report figures needed verification; the 251,783 count is root-caused and fixed, the module tally still open

Found 2026-07-30 by running `fathom research report --since 168h` on this
machine ~30 minutes after phase O's channels went live. Both are
instrument-trust issues, filed BEFORE being used in any claim.

1. **`platform-module serving: platform-standards 15` with zero bindings.**
   `~/.fathom/hooks.jsonl` genuinely contains 15 `module-served` beats
   (`{"kind":"module-served","meta":{"module":"platform-standards",
   "phase":"pre","rules":"1"}}`), yet **no repo on this machine has a
   `.fathom/modules.cue`** — `fathom doctor` reports "no platform-module
   bindings configured" — and `internal/modserve.Bind` is the only writer of
   `Obligation.Module`. Hypothesis to check first: the pre-hook's beat is
   labeling by the rule's **Layer** (a project layer pack named
   `platform-standards` was created at a repo root today) rather than by a
   bound **module** — i.e. a layer-vs-module labeling confusion in
   `reportModuleBeats`/the pre-hook render path. If so, the metric
   over-reports module adoption, which is exactly the class of dishonesty
   the flywheel exists to avoid. Verify by asserting the beat is emitted
   ONLY when a binding produced the rule.
2. **`review_active_findings=251783`** — **ROOT-CAUSED AND FIXED
   2026-07-30 (phase R).** The hypothesis in this observation's own text
   ("re-counting per call") was correct. A `review` call-log row's
   `verdict_outcome` is a SNAPSHOT — the active findings as of that single
   review run (`review.reviewVerdictOutcome`). The aggregator summed that
   snapshot across every review call in the window instead of reading the
   latest one, and since `fathom review` fires on every push, a busy week
   stacked many near-identical ~250-finding snapshots into 251,783. Fix:
   `internal/metrics.LatestReviewActiveFindings` reports the most recent
   call's snapshot only, plus a `review_active_findings_note` disclosing
   which call it is "as of". Pinned by a 50-call fixture asserting BOTH the
   pre-fix sum (12,650) and the post-fix value (253), so the bug cannot
   silently return. Notably no store was ever involved — `research report`
   opens none by design; the defect was purely in call-log aggregation.

**Genuinely trustworthy in the same report** (stated for contrast, and
because these are the numbers that matter): injection coverage
seen=4106 / answered=3753 / **rate=91%** with its per-user caveat printed;
guarded-edit coverage pre seen=280 / served=96, dedup_suppressed=15 (the
phase-O channel firing within the hour); surface traffic bare=7234 /
agent=196 / cli=36; latency p50=130ms p95=1540ms; and the honest
"not measurable: no eviction event is logged anywhere in fathom today".

**Status:** PART 2 CLOSED (the count is fixed and fixture-pinned, and the
verify-gate line is quotable again with its as-of note). **PART 1 REMAINS
OPEN** — the module-served tally naming `platform-standards` with no
binding on this machine is still unexplained; the layer-vs-module labeling
hypothesis has not been tested, and that line stays uncitable until it is.

### O19 — a copied prompt does not transfer across corpora: 202 of 219 claims rejected on the first real platform pack (7.8% acceptance)

- **What was run (2026-07-30):** the first REAL platform module —
  `platform-standards`, compiled from a platform repo's 39 `docs/adr/*.md`
  plus its `CLAUDE.md`/`AGENTS.md` (41 DocChunks, 210 sections) — using a
  BYTE COPY of fathom's shipped `adr-rules` prompt (which is eval-gated at
  1.00/1.00 on *fathom's own* ADR corpus). Result: **17 claims emitted,
  202 rejected**, budget-stopped at 171/210 sections, $0.256, haiku.
- **Where the survivors came from (the diagnosis):** 9 from `CLAUDE.md`,
  6 from `AGENTS.md`, and only **2 from the 39 ADRs** — i.e. the prompt
  transferred for the *rule-shaped* docs (a CLAUDE.md sentence already reads
  as an obligation with an implied code scope) and essentially failed for
  *decision-record prose* (an ADR narrates context/decision/consequences;
  the emitted scopes did not verify against the corpus, so the compiler's
  anchor/scope verification correctly rejected them).
- **Why this matters (it is the taxonomy's own claim, now measured):**
  ADR-024 says a platform-stratum pack is trusted via *its own* corpus
  compilation plus *its own* on-disk eval evidence. This run is the first
  quantitative evidence for WHY: the same prompt at 1.00 precision on one
  ADR corpus lands at ~8% acceptance on another org's ADR corpus. A
  platform pack needs a prompt tuned to how ITS corpus expresses
  obligations, and the eval gate is the instrument that would have caught
  this before anything served (it did refuse — the compile only ran under
  `--unsafe-skip-eval-gate`, logged).
- **Everything downstream behaved correctly, which is the good news:**
  `module doctor` returned verdict **draft** and refused every promotion
  with named reasons — eval evidence absent, 3 broken anchors, the
  three-way watermark check disagreeing, and (the phase-J fix earning its
  keep) `provenance.compiled_by="unspecified"` caught as not matching the
  store's actual compile identity `claude-haiku-4-5-20251001`. A module
  built this way is structurally unbindable, exactly as designed.
- **Fix direction:** (1) phase L's advisor gets its first concrete job —
  propose prompt/eval-fixture starting points FROM a draft compile's
  rejection pattern (the rejections are the signal, and today they are
  discarded: only a count is reported, no per-section reason is surfaced or
  stored); (2) `module compile` should report rejection REASONS grouped by
  cause (scope-unverifiable / anchor-missing / over-broad / malformed) —
  without them an author cannot act, and the studio cannot render the gap;
  (3) document that a platform pack starts from a corpus-shaped prompt, not
  a copied one.
- **Status:** open. The artifact stays at `draft` on disk as the honest
  reference case; it is NOT bound by any repo.

### O17 — `module compile` and its own warning disagree about where a module's layer pack lives; `--root` is undocumented and rejected

- **Symptom (found 2026-07-30 building the first REAL platform module, from
  platform-engineering's 39 ADRs):** the compile wrapper warns that it
  "expects a project-local layer pack named X (a `<module-dir>/.fathom/
  layers/X/` directory, LC-3)" — but after moving the pack exactly there,
  compile fails `error: resolve layer "X": semantic: unknown layer` while
  `fathom layers list` run from the module dir shows the pack loaded
  (`SOURCE project`, `STRATUM platform`). The underlying engine
  (`compileViaStore`) resolves the pack from the ingested SERVICE's root,
  not the module directory. Placing the pack at the service root compiles.
- **Second defect:** `module compile`'s flag set has no `-root`, yet
  `module eval`/`doctor` do (`cmd/fathom/module_compileeval.go:298`), and
  compile's own warning text implies a module-dir-relative resolution that
  only a `-root` could express. Passing `--root` to compile dies with
  "flag provided but not defined" AFTER the operator has already reasoned
  about roots — the usage line also omits `-root` for the verbs that DO
  accept it.
- **Why it matters:** this is the first-run experience for the exact
  workflow phase O exists to serve (an org compiling its standards into a
  bindable module). Three failed compiles before a human-obvious layout
  worked, and the tool's own guidance pointed at the wrong directory.
- **Fix direction (not yet implemented):** make one root story true — either
  the wrapper resolves the pack relative to the module dir and PASSES that
  root into the engine, or the warning names the service root. Add `-root`
  to compile's flag set (or reject it with a message naming the correct
  layout), and list `-root` in the usage line for every verb that has it.
- **Status:** open; filed as the first row of the next module-verbs slice.

### O18 — the eval gate makes first-time module authoring a chicken-and-egg

- **Symptom:** a fresh project layer pack honestly declares `evaled: false`
  (no eval evidence exists yet), and `module compile` correctly refuses:
  "layer X has no passing eval/RESULTS.md record — refusing to compile".
  But an author cannot write meaningful eval fixtures for a corpus they
  have never seen compiled — the fixtures encode "what good claims look
  like for THIS corpus". The only ways forward are
  `--unsafe-skip-eval-gate` (correctly logged into `compile_runs`) or the
  `--mock` dev lane, neither of which is a documented authoring path.
- **Why it matters:** the gate is RIGHT (an ungated pack must never serve),
  but the product currently offers no honest ramp from "I have a corpus" to
  "I have eval evidence". This is exactly the gap
  `docs/vision/13-expert-suite.md` §4 assigns to the studio (guided
  readiness) and phase L (the advisor proposing fixtures) — this
  observation is the concrete first-run evidence for both.
- **Fix direction:** a documented `draft compile` lane — compile behind an
  explicit draft flag that (a) marks every emitted claim's compile run as
  draft-lane in `compile_runs`, (b) is refused for anything a consumer repo
  could bind (the readiness ladder already holds such a module at `draft`),
  and (c) is the input the studio/advisor reads to PROPOSE eval fixtures.
  Then the honest loop is: draft-compile → read claims → author fixtures →
  eval → real compile.
- **Status:** open; recorded as the design input for phase L's advisor and
  the studio's Prove step.

### O16 — a compiled obligation over-generalized its scope: ADR-018's "CUE is output, never authored" served on every `schema/**/*.cue` edit

- **Symptom:** while implementing phase O (2026-07-29), the edit-hook injected
  "Do not hand-author CUE; generate code only via `fathom gen-plan-cue`" on
  every new `schema/binding/schema.cue` / `schema/guard/schema.cue` edit —
  files this repo's own CLAUDE.md convention *mandates* hand-authoring
  (with valid/invalid testdata). Reported by the phase-O worker.
- **Root cause (source-side, not serving-side):** ADR-018 §3's sentence
  "CUE is output, never authored" is scoped in prose to the generated `#Plan`
  shadow, but read standalone it is a universal claim — so the compiler
  produced a correctly-cited claim whose *scope glob* legitimately covers all
  CUE. This is the O14 scope-precision class (mis-scoped, not mis-cited):
  the anchor is right, the blast radius is too wide.
- **Fix applied:** narrowed the SOURCE (ADR.md §3, this commit) to say
  explicitly that it constrains generated `#Plan` documents only and NOT the
  hand-authored `schema/**` packages, pointing at the CLAUDE.md convention.
  The next compile of `adr-rules` for this service re-derives the claim from
  the narrowed text; until then the old claim serves with its citation
  intact (anchor-text healing keeps it honest, O13).
- **Standing lesson (third instance):** a rule's *prose scope* is invisible
  to a compiler reading one section — when a binding doc states a universal
  and means a specific, the claim inherits the universal. Prefer writing the
  exception into the source sentence over post-hoc claim refutation, since
  refutation kills the claim where it IS correct (here: plan CUE).
- **Status:** source narrowed; re-compile pending the next `adr-rules`
  compile cycle for service `fathom` (no flag-day needed — prompt identity
  unchanged, only corpus text).

### O15 — obligations injection genuinely INERT on fathom's own repo since 2026-07-18: the back-compat purge orphaned the hooks' directory-derived service identity

- **Symptom:** found while implementing `fathom doctor` (DI-1/DI-2,
  docs/plans/doctor-and-impact.md, 2026-07-24): the live call log shows
  1,035 `obligations` calls with origin `edit-hook` carrying service
  **"sextant"** — which `list_services` proves is NOT an indexed service
  (the fleet index has `fathom`, 5,419 symbols; no `sextant` at all). Every
  such probe returns empty rules, so the hook stays silent: **the
  PostToolUse obligations injection has not fired for fathom's own repo
  since 2026-07-18T05:09Z**, while `hooks_firing`-style traffic evidence
  makes the stack look perfectly healthy.
- **Evidence (verbatim probes, 2026-07-24):**
  - `list_services` (live daemon): `agentic-kit, app-kit, car-assistant,
    crossplane-platform, fathom, geoffmiller-cloud, growing-gardens-bi,
    resume_as_code, throughline` — no `sextant`.
  - Call-log sample (`workspace/logs/mcp-calls.jsonl`, 7,630 rows):
    edit-hook-origin `obligations` rows split `service=sextant n=1035
    (first 2026-07-16T16:17:39Z, last 2026-07-24T06:08:27Z)` vs
    `service=fathom n=363 (first 2026-07-16T16:17:39Z, LAST
    2026-07-18T05:09:02Z)`. Until 07-18 the rows come in PAIRS ~100–250ms
    apart (sextant miss, then fathom hit — injection worked via the second
    probe, at one wasted probe per edit); after 2026-07-18T05:09Z the
    fathom probes stop dead and only sextant probes remain. Also 165
    edit-hook `spec_refs` rows, ALL service=sextant — OP-4 spec-line
    injection has the same dead identity (it uses the first candidate that
    relativizes, unconditionally).
  - The cliff matches a commit to the second: **d6cb7b7** ("remove
    (back-compat): purge the sextant back-compat surface (owner
    override)"), authored 2026-07-17 22:09:20 -0700 =
    **2026-07-18T05:09:20Z**, deleted `withNameAliases` — the shim that
    expanded candidate "sextant" → ["sextant", "fathom"] in all three
    hooks. Post-purge, `serviceCandidates` (cmd/fathom/serviceident.go)
    derives identity purely from directory basenames: for this repo both
    candidates are "sextant" (the dir is still named `sextant`; the git
    toplevel is the same dir), so no candidate can ever reach the indexed
    service "fathom".
- **Root cause:** d6cb7b7's stated consequence ("stores ingested under
  'sextant' are no longer recognized — the fleet's own store already uses
  'fathom', so the dogfood machine is unaffected") checked the STORE side
  but missed the HOOK side: the store is fine, and the *hooks' directory
  basename* is the thing still saying "sextant". The alias shim was the
  only bridge between the repo's directory name and its post-rename
  service name; deleting it silently severed session-hook priming,
  edit-hook obligations injection, and spec-line injection for the one
  repo whose directory name predates the rename — fathom's own.
  Not a doctor bug and not an index bug: a service-identity resolution gap
  (directory-derived names have no fallback to the index's actual
  services or `ServiceMeta` path matching).
- **Reproduction:** `obligations({"service":"sextant","file":"CLAUDE.md"})`
  → empty rules, no watermark; the known-good twin
  `obligations({"service":"fathom","file":"CLAUDE.md"})` → 2 rules,
  watermark `2d3055cc…`. Or: `bin/fathom doctor --path
  /path/to/sextant-checkout` (no `--service`) → `obligations_served` FAIL
  "no candidate service ID (sextant) is in the daemon's index".
- **Fix direction (not landed here):** hooks need a rename-proof identity —
  match the project dir against the daemon's `ServiceMeta.Path` (the
  daemon already records it for auto-refresh) or honor a project-local
  service pin (e.g. `.fathom/config.toml`), instead of trusting directory
  basenames alone. Interim operator fix: rename the checkout dir to
  `fathom`, or re-ingest under `sextant` (undesirable — two service IDs
  for one repo).
- **Detection landed (this commit):** `fathom doctor`'s
  `obligations_served` resolves service the way the hooks do (all
  candidates ∩ the daemon's service list) and fails with the distinct
  not-indexed verdict when — as here — no candidate is indexed; an
  explicit `--service` outside the hook candidates passes with an
  injection-mismatch note instead of masking this state.
- **Status:** FIXED IN CODE (2026-07-24): `list_services` now returns each service's recorded corpus path, and all three hooks resolve their identity through it (`resolveFromServices`/`candidatesWithDaemon`, cmd/fathom/serviceident.go) — the daemon's own repo↔service mapping is authoritative, basename guessing is the fallback. The edit-hook fetches the resolution only after a basename root contains the edited file, preserving the O8 out-of-repo total-silence invariant (`TestEditHookSilentWhenFileOutsideAllCandidateRoots`). Injection on this repo restores when the daemon next serves a binary with this change (deploy pending).
- **Impact:** citation trust is the moat; a served claim pointing at the
  wrong text is worse than no claim. Once the live daemon runs this code
  and sweeps, no claim will be silently mis-cited — worst case an
  honestly-flagged stale rule, never silence. Until that deploy, the live
  index keeps serving the 73 wrong citations unflagged.

### O14 — the top-6 serving cap silently evicts previously-served claims after a recompile (eviction visibility fixed; scope-mapping precision filed for the next flag-day)

- **Symptom:** post-recompile (adr-rules + spec-bindings joining the
  blend), `obligations(internal/query/engine.go)` no longer serves the
  staleness/Honesty rule at all — evicted by the cap, not retired; and the
  blended set includes a mis-scoped rule ("Add new extractor ext→kind
  mapping in connector" on the QUERY ENGINE file) — scope-mapping noise
  compounding the eviction.
- **Root cause:** MaxObligations=6 with specificity-ranked blending across
  three layers; new layers' exact-path claims outrank older glob-scoped
  rules; nothing logs or labels an eviction.
- **Reproduction:** same curl shape as O13 against
  `internal/query/engine.go`; diff the served set against the
  pre-recompile serving (the home-bench corpus's T12 provenance records
  it).
- **Fix — eviction visibility (T5-2, docs/plans/trust-and-tokenomics.md):**
  `obligations` now reports `displaced: N` (served-eligible claims the cap
  dropped this call) and every served rule carries a `layer` label (which
  manifest layer compiled it — `obligations`/`adr-rules`/`spec-bindings`/
  `findings`), on the MCP tool response and the edit-hook payload (a single
  integer note; a non-default layer labeled inline). Test:
  `TestObligationsFor_DisplacedCountAndLayerLabels` (internal/semantic).
- **Fix — scope-mapping precision: FILED, not compiled.** The mis-scoped-
  rule half is a compile-prompt fix, and `internal/semantic/prompts.go`
  content-addresses `PromptVersion` off `prompt.md`'s exact bytes — editing
  the live prompt, even to add a clearly-marked pending block, would
  silently trigger the next flag-day as a side effect of this commit. The
  concrete fix (a worked example: a rule naming several specific packages
  must scope to the union of exactly those packages, never collapse to one
  broad catch-all) is filed as an inert sibling proposal,
  `schema/semantic/layers/obligations/prompt.PENDING-scope-precision.md`
  (not referenced by `layer.cue`'s `prompt_file`) — see also the
  ingest-specific-rule gap noted at H1 re-derivation. Lands at the next
  deliberate flag-day alongside OP-6.
- **Status:** eviction visibility fixed and tested; scope-mapping precision
  filed (docs/plans/trust-and-tokenomics.md T5-2b), landing gated on the
  next flag-day's prompt identity bump.
- **Impact:** a recompile — the recommended freshness act — can still evict
  a rule an agent relied on yesterday, but that eviction is now VISIBLE
  (`displaced: N` plus per-rule layer labels) instead of silent; the
  scope-mapping noise that compounded it remains until the filed prompt
  change lands.

### O12 — context-bench B1-T09: the fathom arm lost 6.6× to grep on its own home question (cross-repo callers) — three compounding product gaps — improved (6.6×→4.2×), residual tracked

- **Symptom:** in the first context-bench run (docs/plans/falsifiability.md
  B1-3, run-20260723T045809Z), task B1-T09 — "which agentic-kit heartbeat
  functions does cmd/fathom/instrument.go call?" — cost the fathom arm
  504,785 tokens / 20 turns / 56.5s where the grep arm took 76,294 tokens /
  2 turns / 11.1s. The fathom arm eventually PASSED, but only after burning
  turns on three distinct product gaps. This was the bench's worst data
  point and a driver of the ×10 retraction.
- **Evidence (live reproductions, daemon @ fleet store, 2026-07-23):**
  - **(a) Cross-repo call edges missing.** `symbol_context(fathom,
    cmdInstrument)` returns `callees: [fathom::main.beatLogPath,
    fathom::main.newFlagSet, fathom::main.reportInstrument]` — the
    `heartbeat.Load` / `heartbeat.CheckLiveness` / `heartbeat.Rollup`
    calls into agentic-kit (cmd/fathom/instrument.go:37,71,75) are absent.
    The reverse direction agrees: `find_callers(agentic-kit,
    CheckLiveness)` returns only the kit's own
    `heartbeat_test.TestCheckLiveness` — never
    `fathom::main.cmdInstrument`. Only `references`' text tier can save
    the query; the graph tiers are blind to fleet-crossing edges.
  - **(b) Parameter-name inconsistency across the hot quartet.** The
    B1-T09 transcript burned ~5 error turns on `-32602` invalid-params
    guess-rounds (three rounds observed in the critic's replay): the four
    most-adjacent tools each name their primary argument differently —
    `find_symbol` wants `name` (internal/mcp/tools.go:145), `references`
    wants `query` (internal/mcp/references_tool.go:19), `query_symbols`
    wants `concern` (internal/mcp/tools.go:22), `symbol_context` wants
    `symbol` (internal/mcp/nav_tools.go:147). An agent that has just used
    one tool reliably guesses the next one's parameter wrong.
  - **(c) Case-insensitive tier steers to the wrong package.**
    `find_symbol(agentic-kit, "heartbeat")` → `matched_via:
    case_insensitive` → the `Heartbeat` GoType in
    `foundations/coordination/worker-heartbeat/types.go` — not the
    `foundations/telemetry/heartbeat` package the caller (and
    instrument.go) actually uses. The honest `matched_via` marker is
    present, but the steering still sent the bench agent down the wrong
    package before it recovered.
- **Root cause:** (a) call-edge extraction resolves callees within a
  service; cross-service edges exist only in platformlink/text tiers, and
  `symbol_context`/`find_callers` don't consult them. (b) tool schemas
  were named per-tool, never normalized as a set. (c) the case-insensitive
  tier ranks bare-name equality without package-affinity context.
- **Fix (docs/plans/pulled-tools-excellence.md P1-1/P1-2/P1-3):**
  (b) find_symbol/query_symbols/references now additionally accept
  `symbol` as an additive alias for their native argument (`name`/
  `concern`/`query`); a call naming both only errors if the values
  disagree (`internal/mcp/paramalias.go`). (a) `symbol_context`/
  `find_callers` now consult the fleet backend for cross-service edges
  when one is wired (fleet stores) — new `cross_service_callers`/
  `cross_service_callees` fields, scope-labeled, additive
  (`internal/mcp/crossservice.go`, reusing `cross_repo_callers`'s own
  fleet-wide matching). (c) the case-insensitive tier now returns a
  package-match GROUP alongside the name-match group, so a query naming
  a whole package (whose own members are never literally named after it)
  is no longer hidden behind a same-named type/symbol in an unrelated
  package (`caseFoldMatches`, findsymbol.go). See the P1-1/P1-2/P1-3 rows
  in the plan for the full test list.
- **Live-probe nuance (not a regression — a real code-structure finding):**
  `cmdInstrument` calls `heartbeat.Load` directly but delegates to
  `reportInstrument` for `heartbeat.Rollup`/`CheckLiveness` — one level of
  indirection the symptom's prose collapsed. `symbol_context(fathom,
  cmdInstrument)` now correctly surfaces `Load` cross-service (plus
  `reportInstrument` as a same-service callee); `symbol_context(fathom,
  reportInstrument)` surfaces the other two. The fix is symbol-precise by
  design — full 3-function coverage takes one natural follow-up hop, which
  the P1-5 re-run (below) shows in the turn count.
- **P1-5 re-run (scratch daemon on a spare port, NOT the live 7575 daemon;
  fresh fathom+agentic-kit ingest):** B1-T09 504,785→321,100 tokens (−36%),
  20→9 turns (−55%) — improves 6.6×→4.2× vs. grep, which is **still 4.2×
  more tokens than grep (321,100 vs 76,294): a non-flip on the ratio**,
  consistent with the live-probe nuance above. B1-T10/T11 (the other two
  cross_repo_callers tasks) were already near the cheap 3-turn floor
  pre-fix (they don't need cross-service callee stitching) — also
  non-flips, ~1% token noise, not a regression.
- **Status:** fixes for (a)/(b)/(c) landed (P1-1/P1-2/P1-3; P1-4/P1-5
  alongside in the same plan wave) — but the headline metric has NOT
  flipped: grep still wins B1-T09 by 4.2× on tokens. Kept under Open for
  the residual: the remaining gap is the session/tool-schema baseline plus
  the extra hops fathom's symbol-precise answers need on delegating code
  (`cmdInstrument`→`reportInstrument`); closing it likely needs either a
  transitive cross-service callee view or a cheaper first-answer shape,
  neither designed yet.
- **Impact:** the exact query class the fleet index exists to win
  ("who calls this, anywhere?") was its worst measured case — 6.6× more
  tokens than grep, now 4.2× — better, still losing. (b)'s alias fix
  removes a tax that hit every discovery session, not just cross-repo
  ones.

### O11 — `migration_targets`' `deprecated_pkgs` REPLACES the default list instead of adding to it, contradicting its own description — FIXED

- **Tool:** `migration_targets` (`internal/mcp/extra_tools.go` `registerMigrationTargets`).
  **Severity:** low — a caller-supplied list under-reports, it never over-reports.
- **Symptom:** the tool's own `deprecated_pkgs` parameter description reads "Additional
  deprecated package paths (default list is always included)". The underlying
  `migrationtargets.Find` did the opposite: `if len(deprecatedPkgs) == 0 { deprecatedPkgs
  = DefaultDeprecatedPkgs }` — when the caller passed ANY non-empty list, `DefaultDeprecatedPkgs`
  (`io/ioutil`, `golang.org/x/net/context`, `golang.org/x/oauth2/internal`,
  `github.com/golang/protobuf`) was dropped entirely, not merged.
- **Found during:** C4-5's empty-config honesty audit (docs/plans/review-verb.md). Not the
  same bug class as `arch_conformance`'s confirmed instance — the *empty*-config case was
  always safe here (falls back to the real, non-empty default list, an honest check) — this
  was a *non-empty*-config case where the tool's own promise didn't hold.
- **Fix (P1-6, docs/plans/pulled-tools-excellence.md):** `migrationtargets.Find` now merges
  the caller-supplied `deprecatedPkgs` with `DefaultDeprecatedPkgs` via a new
  `mergeDeprecatedPkgs` helper — defaults first (declared order), then any caller extras not
  already present, deduplicated — so the default list is always checked, matching the
  parameter description's promise. `migrationtargets_test.go`'s replace-semantics assertions
  (`TestFind_DeprecatedIoutil`, `TestFind_DeprecatedContext`, `TestFind_ReasonContainsPackage`)
  were updated deliberately for merge semantics, plus two new tests
  (`TestFind_CallerPkgsMergeWithDefaults`, `TestFind_MergeDeduplicates`) covering the merge
  and dedupe contract directly.
- **Status:** resolved.

### O10 — `fathom claim` triage is store-direct: fleet-store triage requires stopping the daemon

- **Symptom (by design, not a bug):** `fathom claim refute|corroborate` (D3 findings
  tracer, FT-3) opens the DuckDB store directly (`--store` required). Against the
  always-on fleet daemon's `workspace/db/fleet.duckdb` that write collides with the
  single-writer lock — triaging a fleet claim means stopping the daemon, running the
  verb, and restarting.
- **Why it shipped this way:** a triage verdict is a rare, deliberate, single write
  best made against a store the operator can inspect; v0 mirrors `verify --store` /
  `remap --store` rather than growing the read-serving daemon a write verb.
- **Trade-off cost:** on the dogfood fleet the daemon *is* the store's owner, so the
  ergonomics are stop/start-shaped for what should be a one-liner.
- **Candidate fix (when triage sees real use):** a `claim` MCP tool or daemon verb so
  the daemon (the lock holder) performs the write — same shape as the `ingest` tool
  solving the same lock problem for refreshes.
- **Status:** open; documented v0 trade-off.

### O10 — relative corpus paths at the ingest boundary: typed edges silently absent + a daemon cross-refresh hazard — FIXED

- **Symptom:** rehearsing the ADR-020 store migration (2026-07-22), a CLI
  `ingest --service fathom --path . --store <fresh>.sqlite` produced 4723
  symbols and **0 typed edges — no error, no counter, no stderr** — while
  `TestTypedEdges_PerformanceGate` over the same tree produced 9236.
- **Root cause 1 (typed edges):** the connector-scope filter rel-maps
  go/packages' absolute file positions against `repoPath`; `filepath.Rel`
  errors on a relative base, so `repoRelSlash` fell back to the absolute
  path, which never matches the repo-relative allowed set → every package
  skipped. The CLI never printed the typed counters, so `PackagesTotal=215,
  processed=0` was invisible (probe: the perf test passes an absolute
  fixture root — the only difference).
- **Root cause 2 (worse, latent):** `ServiceMeta.Path` was recorded verbatim,
  so `--path .` (the CLI default AND both repos' post-commit hooks) records
  `"."` — which the daemon resolves from **its own** working directory on
  auto-refresh. The kit's hook would refresh the fathom corpus under the
  `agentic-kit` service ID. Latent only because DuckDB's single-writer lock
  made those hook ingests always fail while the daemon ran; ADR-020's WAL
  store would have activated it on migration day.
- **Fix (commit with this entry):** `EnrichTypedEdges` normalizes `repoPath`
  to absolute at entry (`TestEnrichTypedEdges_RelativeRepoPath`); the
  pipeline records `ServiceMeta.Path` absolute
  (`TestIngest_ServiceMetaPathAbsolute`); a never-silent stderr guard fires
  when go/packages sees packages but the scope filter excludes all of them;
  `cmd/fathom ingest` prints the typed-edge counters; both repos' hooks pass
  `--path "$PWD"`.
- **Lesson:** a "best-effort, never blocks" pass still owes a *visible*
  zero — the counters existed in `fusion.Report` and died unprinted at the
  CLI boundary. And rehearse operator runbooks against real data: this was
  found only because the migration was rehearsed on a scratch copy first.

### O9 — test_reaches recall residue after H1: aliased external-test calls and re-ambiguated methods

- **Symptom:** first HTTP-mode run of the re-armed gate on the restarted daemon
  (new binary 5983251, agentic-kit re-ingested at 149a570): evidence-spine.md
  verifies 13/15 — `{symbol: PreconditionSatisfied}` and `{symbol: Run,
  package: foundations/evidence/evidence-ladder}` fail `test_reaches`, while
  H1's store-mode proof (kit at the pre-H2 snapshot) had 15/15.
- **Evidence (verbatim probes):** both symbols ARE tested —
  `evidencesignature_test.go:190` calls `sig.PreconditionSatisfied(...)` from
  an external `_test` package via import alias; `evidenceladder_test.go:97`
  calls `l.Run(context.Background(), ...)`. Known-good neighbor on the same
  run: `{symbol: Build, package: foundations/evidence/case-file}` passes.
- **Root cause (CONFIRMED — the two initial hypotheses were wrong):** a fresh
  full-kit scratch ingest with the new binary verifies BOTH claims (2/2), so
  the H1 recall fixes (byDirName aliased-selector resolution, method edges)
  work. The fleet store was serving **stale-extractor observations**: the
  pass-3 hash-gate keyed fingerprints on file CONTENT only, so re-ingesting
  after the binary upgrade skipped every unchanged file and kept the
  pre-H1 `Calls` forms (`sig.PreconditionSatisfied` un-canonicalized, no
  ctor-scoped method forms). Extractor improvements could never reach
  unchanged files — the observation-level twin of the documented
  binary-staleness class.
- **Fix (this commit):** `fusion.ExtractorEpoch` salts every fingerprint
  (`fusion.FingerprintHash`, also adopted by `ObligationsIndexStale` so the
  formulas stay identical — the client test caught the divergence
  immediately). Bump the epoch in any commit that changes extraction
  semantics; the salt change invalidates all stored fingerprints and the next
  ingest re-extracts everything. Regression:
  `TestIngest_ExtractorEpochInvalidatesOldFingerprints`.
- **Also learned:** a `--path`-rooted scratch ingest of a single package dir
  flattens `FilePath` (no parent dirs), which disables `byDirName` — probe
  stores must be rooted at the repo, or dir-segment resolution silently
  vanishes. (Cost me one wrong intermediate conclusion.)
- **Status:** fixed in code; operational step = re-ingest the fleet on the
  new binary (every service re-extracts once — the epoch salt forces it).

### O8 — obligations v2 serves code-scoped and descriptive rules on documentation edits (fix landed; fleet recompile pending)

- **Symptom:** every `Write` to `docs/vision/*.md` (2026-07-14) injected the same six
  obligations, several of which cannot bind a docs edit.
- **Evidence (verbatim, from the PostToolUse payload):** editing `docs/vision/README.md`
  injected — among others — "Always verify code with the race detector using go test
  -race. [CLAUDE.md#L50-L52]" (a code-only rule), "Assert extractor and query outputs
  using golden files. [ADR.md#L174-L175]" (code-only), and "Bucket pending results by
  language and extract. [ADR.md#L68-L68]" — which is a *description of how the lazy
  pipeline works* (ADR-005 context), not an obligation on any editor. The identical
  6-rule set appeared for eight different markdown files, suggesting `**`-wide scopes
  rather than file-class scoping.
- **Root cause (probable):** the v2 prompt prefers narrow scopes but the compiler has
  no notion of *file-class* (code vs docs vs config), so rules whose source text doesn't
  name a path compile with repo-wide scopes; and the "binding rules only" instruction
  still passes descriptive-imperative sentences ("bucket pending results…") that read
  like commands.
- **Addendum (same session):** the hook also injected the full fathom rule set for an
  edit to `~/.claude/projects/.../memory/*.md` — a file *outside every indexed repo*.
  The edit-hook resolves the service from session cwd candidates and apparently still
  scope-matches a file that doesn't relativize into the service root; it should exit
  silent when the edited file is outside the resolved candidate's root.
- **Fix direction:** serve-time precision pass (docs/vision/04 Gap 3): sample injected
  payloads across the fleet, add file-class awareness to scoping (a code-only rule must
  not match `**/*.md`), and tighten the prompt's rejection of descriptive imperatives.
  Success metric: injected-rule precision ≥0.95 sampled at serve time.
- **Status:** fix landed in code (docs/plans/o8-precision-and-speclines.md
  OP-1..OP-3): prompt v3 (`internal/semantic/prompts.go`) instructs file-class
  scoping and forbids bare `**`; a compile-time over-broad-scope guard
  (`verifyDraft`, `internal/semantic/compile.go`) rejects any scope matching
  more than half the indexed corpus, exactly like a failed citation; the
  edit-hook (`cmd/fathom/edithook.go`) now exits silently for a file that
  relativizes under NO service-candidate root instead of falling back to the
  raw out-of-repo path, closing the addendum. Not yet observably fixed at
  serve time: every already-compiled claim in the fleet index still carries
  its old (pre-v3) scope until the flag-day recompile (OP-6, deferred —
  operator-side, needs a keyed shell) re-runs `compile` on a binary containing
  this slice. **Impact:** unchanged until OP-6 lands; the compile-time and
  edit-hook halves of the fix are what OP-6 will activate fleet-wide.

### O7 — optional-capability interfaces silently stripped by the auto-refresh wrapper (fixed)

- **Symptom:** first post-migration smoke test: `fathom compile --service fathom` over
  HTTP → "RPC error: backend does not support layer compilation", while the same call
  worked in every unit test; the edit-hook smoke test looked like an expected
  zero-obligations silence but was actually failing the same way.
- **Evidence:** `compile_layer`/`obligations` assert optional interfaces on the served
  backend; the daemon serves `mcp.NewAutoRefresh(client)` — a struct embedding the
  `QueryInterface` *interface*, which forwards only that interface's method set. Every
  semantic method fell off the wrapper. Direct-backend tests (no wrapper) stayed green.
- **Root cause:** Go interface embedding + type-assertion capability detection don't
  compose across wrappers; and the edit-hook's (correct) silent-on-error contract
  masked the failure in manual smoke testing.
- **Fix (same session):** explicit forwards on `autoRefreshBackend` (Obligations /
  ObligationsIndexStale / CompileObligations / ClaimsSummary), with `ensure()` freshness
  on the read paths — plus `TestSemanticTools_ThroughAutoRefreshWrapper`, which
  exercises the tools exactly as the daemon serves them.
- **Status:** resolved. **Impact/lesson:** any future optional backend capability MUST
  be forwarded on every wrapper and covered by a through-the-wrapper test; smoke tests
  of silent-by-design channels must use the loud path (the raw tool) first.

### O6 — test_reaches misses tests that exercise a symbol only through method calls

- **Symptom:** `fathom verify` failed `test_reaches` on two genuinely-tested symbols
  while passing a third, on the same store, same run (2026-07-07, first gated plan).
- **Evidence (verbatim):** plan contracts `{symbol: ParseMarkdownPlan}` → ✓
  `test_reaches`; `{symbol: Discover}` → ✗ "no Test* statically reaches it";
  `{symbol: manifestWatermark}` → ✗ — yet `TestLocalConnector_Discover_PaymentService`
  calls `lc.Discover(...)` and `TestLocalConnector_Watermark_ManifestStableAcrossMove`
  reaches `manifestWatermark` via `lc.Watermark(...)`.
- **Root cause of the difference:** `ParseMarkdownPlan` is reached by a *plain function
  call* from its test; the failing two are reached only through *method* calls
  (`lc.Discover`), whose call edges the Go extractor doesn't attribute to the method
  symbol, so `callgraph.TestImpact` finds no Test* path.
- **Workaround:** `skip_test: true` on method-reached contracts (symbol presence still
  verified). **Fix direction:** method-call edge attribution in `goextract` (receiver
  type resolution), then remove the workaround from `docs/plans/`.
- **Status:** open. **Impact:** verify gates under-prove any contract whose symbol is
  only test-reached through methods — a false-negative class that pushes plan authors
  toward `skip_test`, weakening the gate.

---

## Resolved

### O1 — `outline` ignored its "repo-relative" contract; needed an absolute path — FIXED
- **Tool:** `outline`. **Severity:** high (core discovery tool silently returned nothing).
- **Symptom:** `outline{service:"throughline", file:"app/participants/participant.go"}`
  → `{"symbols":[]}`, as if the file weren't indexed.
- **Evidence:** `find_symbol{name:"Participant"}` returned the symbol with
  `file_path:"/Volumes/.../app/participants/participant.go", tier:"verified"` (so the file
  **was** indexed); `outline` with the **absolute** path returned the full symbol set.
- **Root cause:** `symbolnav.Outline` matched `n.FilePath == file` exactly. The store keys
  files by absolute path; any repo-relative `file` arg missed — contradicting the tool's
  documented "Repo-relative file path".
- **Fix:** `fileMatches(stored, query)` resolves a relative query by path-segment suffix
  (leading `/` so `participant.go` can't match `myparticipant.go`); absolute queries still
  match exactly. Regression test `TestOutline_relativePathMatchesAbsoluteStore`. Verified
  over HTTP: relative `outline` returns 10 symbols.
- **Status:** fixed (this commit). **Impact:** index-first `outline` works on the natural
  (relative) call agents actually make.

### O2 — `find_symbol` appeared blind to methods — NOT A CODE BUG (stale server) → O3
- **Symptom:** `find_symbol{name:"ListAssertions"}` / `{name:"HasAlias"}` →
  `{"symbols":[]}`, while `find_symbol{name:"Participant.HasAlias"}` (qualified) returned
  the method. Looked like bare-name method lookup was broken.
- **Evidence that it is NOT the code:** current `symbolMatches` (engine.go) already has
  `strings.HasSuffix(o.Symbol, "."+name)`; a freshly-built binary serving the same data
  over HTTP returned `find_symbol{name:"ListAssertions"}` →
  `OutputAssertionReader.ListAssertions`, `Projector.ListAssertions`, … correctly. The
  fix commit `7ac8011` (2026-06-12 14:45) predates even the stale binary.
- **Root cause:** the **running MCP server was stale** — see O3. Bare-name method lookup
  works in current code; the deployed process was serving older logic/data.
- **Status:** resolved by O3 (rebuild + restart). No code change to `symbolMatches`.

### O3 — running serve-mcp outlives commits; the binary was never rebuilt — FIXED (mechanism)
- **Tool:** the whole `serve-mcp` deployment. **Severity:** high — this is the systemic
  one. It made a correct code fix invisible to consumers for days (and produced O2's
  false alarm).
- **Symptom / evidence:**
  - `bin/sextant` mtime **Jun 12 18:18**; HEAD **Jun 15** with later commits it predates
    (incl. `923977b "Persist SymbolNode.Package through the store"`).
  - **Two** `serve-mcp` processes (started Jun 13, 2PM and 7PM) from the same stale
    binary, both holding `--store .../fleet.duckdb` — and DuckDB is **single-writer**, so
    duplicate writers contend for one lock.
  - The `post-commit` hook re-ingested **data** with `bin/sextant` but never **rebuilt the
    binary** — so query-engine fixes landed in source and `bin/sextant` stayed stale.
- **Fix (this commit):**
  1. `post-commit` now rebuilds `bin/sextant` whenever the commit touched `*.go`, before
     re-ingesting — the on-disk binary tracks HEAD.
  2. `serve-mcp` logs its build identity at startup (`BuildRevision()` via embedded
     `vcs.revision`, no ldflags); `sextant version` reports the same. Compare against
     `git rev-parse HEAD` to detect a stale server in one line.
- **Operational residue (manual, by design):** a long-running `serve-mcp` process cannot
  hot-swap its own binary — it must be **restarted** to load a rebuilt one (Claude Code
  restarts MCP servers on session/MCP-config reload). And **duplicate** server processes
  should be reaped; only one can hold the write lock.
- **Status:** mechanism fixed; binary rebuilt from HEAD. **Impact:** future fixes reach
  consumers; staleness is now self-evident instead of silent.
- **Update 2026-06-16 — residual SUPERSEDED by 0.2.0.** The deployment moved to a single
  always-on HTTP daemon (launchd `serve-http` at `localhost:7575/rpc`, one shared store),
  so there are no per-session servers and no lock contention. The two manual residuals above
  are now closed by the tool: `serve-mcp --auto-refresh` (default on) re-ingests a service on
  git-HEAD drift before answering — killing the *data*-staleness loop — and a second
  `serve-mcp` on the same store now refuses with actionable guidance instead of silently
  contending for the lock (no more "reap duplicates" chore). **Only *binary* staleness
  remains a manual step**: a code change still needs the daemon's binary rebuilt and the
  daemon restarted (it can't hot-swap itself); `sextant version` vs `git rev-parse HEAD` is
  still the one-line check. See `docs/guides/fleet-index.md` for the daemon model.

### O4 — the shared daemon's index was "frozen": every refresh clobbered the service path — FIXED
- **Tool:** `ingest` / `staleness_report` / auto-refresh on the `serve-http` daemon.
  **Severity:** high — the always-on index silently stopped tracking new commits.
- **Symptom:** after a clean fleet re-ingest, `staleness_report{service:"sextant"}` returned
  **empty** `indexed_commit`/`current_commit`, and `find_symbol` missed newly-added symbols
  (e.g. `ValidServiceID`) — yet `query_symbols`/sources still worked. The data was there; the
  *freshness metadata* was gone.
- **Evidence (verbatim probe):** `scripts/reingest-fleet.sh` set `indexed_commit=79c0cad`;
  one `find_symbol` query later (which fires auto-refresh on HEAD-drift) it was **empty
  again**, while symbols still resolved. So a *refresh*, not a query, was wiping it.
- **Root cause:** `client.IngestService(service)` re-ingested with
  `ServiceRef{ID: service, Path: service}` — walking the bare service ID as a directory
  (e.g. `./sextant`, nonexistent), ingesting nothing, and `SetServiceMeta` then recorded an
  empty path + commit. Auto-refresh, `staleness_report`, and the `ingest` tool all key off
  `ServiceMeta.path`, so every refresh froze the index. (Sibling `Sources` already resolved
  the path from `service_meta`; `IngestService` just never did.)
- **Fix (`4bbaf56`):** `IngestService` now uses the stored `meta.Path` (mirrors `Sources`),
  falling back to the ID only for path-less connectors. Added `serve-http --auto-refresh`
  (the daemon runs serve-http, which never had it) and `scripts/reingest-fleet.sh` to rebuild
  cleanly (path metadata + no malformed/bloated services — sextant was 7093 symbols from
  indexing `ref/`/`tmp/`; correctly ~2923 with `.sextantignore`). Regression:
  `TestIngestService_UsesAndPreservesStoredPath`.
- **Status:** fixed; the freshness loop the 0.2.0 daemon promised now actually holds.

### O5 — one Nix/devbox symlink aborted a whole-repo ingest — FIXED
- **Tool:** `ingest` (LocalConnector). **Severity:** high — a single stray symlink made
  an entire repo un-indexable, so the repo silently never joined the fleet.
- **Symptom:** ingesting `resume_as_code` failed hard:
  `error: ingest failed: fusion: fetch: read .../.devbox/nix/profile/default-9-link: is a directory`.
  No symbols stored; the repo could not be added as a service at all.
- **Evidence (verbatim probe):** the failing path is a **symlink to a directory** (a Nix
  profile link devbox creates). `WalkDir` does not follow symlinks, so it surfaced as a
  non-dir entry and was emitted as a `SourceRef`; `Fetch`'s `os.ReadFile` on it returned
  "is a directory". The fusion fetch pass treats the **first** fetch error as fatal
  (deterministic first-error abort), so one link killed the ingest of 394 real files.
- **Root cause:** `LocalConnector.Discover` emitted every non-dir `WalkDir` entry —
  including symlinks/devices/sockets/pipes — none of which `os.ReadFile` can read as a
  source file.
- **Fix (this commit):** `Discover` now gates on `d.Type().IsRegular()`, skipping all
  non-regular entries. `resume_as_code` went from a hard abort to **1,160 symbols, 0
  skipped**, with the Nix store behind the link correctly excluded (not indexed as noise).
  Regression: `TestLocalConnector_Discover_SkipsSymlinkToDir`. `resume_as_code` added to
  `scripts/reingest-fleet.sh`.
- **Status:** fixed. **Impact:** any real-world repo with Nix/devbox, vendored symlinks,
  or build-tool profile links now ingests instead of aborting.

---

## Confirmed-good (keep regression coverage)

- **`ingest` hash-gated refresh** — `ingest{service:"throughline"}` at session start
  returned `{"ok":true}` fast with no rework when clean, as documented. Single-writer lock
  behaves.
- **`find_symbol` on types/consts/free funcs and qualified methods** — exact-name lookup
  returns multi-hit results with `file_path` + `tier` (`Participant` → 2 hits;
  `Participant.HasAlias` → the method). Bare-name method lookup works in current code (O2).

---

## How to add an entry

1. Reproduce. Capture the verbatim probe + result for **both** the failing and a
   known-good case. If a tool looks broken, **also test a freshly-built binary** before
   blaming the code — O2 was a stale server, not a bug.
2. Root-cause the difference (don't stop at the symptom).
3. File under **Open** with a concrete fix. Move to **Resolved** (link the commit) when
   fixed. Promote anything systemic into `CHANGELOG.md` / `tmp/TRACKING.md`.
