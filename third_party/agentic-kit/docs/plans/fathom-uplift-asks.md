# Fathom uplift asks — kit seams surfaced by Phase K (2026-07-22)

**Mission** — Phase K (fathom vision 11 §3.K, "kit uplift") requires fathom to
be a **consumer, not the kit**: when Phase K's own work surfaces a kit-shaped
seam, the discipline is to file the ask and stop, not fork the code into
fathom. Two candidates surfaced while executing K1 (the `internal/vector` →
`vectorstore` donor deletion, this repo's `handoff-h4-sextant.md` Stage 4) and
K4 (auditing fathom's non-adoption of `kits/eval-runner`/`foundations/quality/qa-harness`):
fathom carries two evidence-agnostic *kernels* — a proof-gate aggregator
(`internal/verify`) and an extraction-accuracy harness (`calibration/`) — whose
typing and rollup/scoring logic is generic, but each currently has exactly
**one** consumer (fathom itself). Per D8 discipline, a single consumer does
not justify a kit package: **both rows below park with a named-consumer
trigger. Neither is built in this pass.**

**Write scope** — this repo's `docs/plans/` only (this file). No kit Go/CUE
code is added by Phase K; if either row's trigger fires, that is a future
plan's scope, not this one's.

## Parked — subscribe-when triggers (D8: no consumer, no code)

- [-] **Proof-gate kernel** — generalize fathom's `internal/verify` gate-aggregation shape: `Task{ID, Title, SpecIDs, Expect}` / `Criterion{Name, Status, Detail}` with a `CriterionStatus` enum (`pass|fail|unknown|n/a`) and a `Disposition` concept for the parked/deferred row states that today only live as unparsed prose in Markdown annotation blocks (`{disposition: deferred, reason: ..., trigger: ...}` — see this repo's own `2026-07-fathom-asks.md` rows, or fathom's `tmp/TRACKING.md`); plus the deterministic rollup `VerifyPlan(plan, ev Evidence) Report{Tasks []TaskVerdict, Summary{Total,Pass,Fail,Unknown}}` (`internal/verify/verify.go:29-143`). The kernel would ship the typing and the pure aggregation algorithm ONLY — fathom's `Evidence` interface (`FindSymbol`/`TestReaches`/`Reachable`/`Contradictions`/`SpecImplementors`, all index-backed) stays in fathom; the kit package would never gain an evidence implementation, exactly as `evidenceladder.Tier1`'s doc comment already draws that line for the ladder side. Motivating observation: this is the same shape as `foundations/quality/eval-gate`'s pass/fail rollup but keyed on static code evidence instead of judge verdicts — two structurally similar gates with zero shared typing today. {id: FA-11, disposition: deferred, reason: "fathom internal/verify is the kernel's only consumer; no second repo has a Task/Criterion/Disposition-shaped gate today", trigger: "a second named consumer"}
- [-] **Eval-harness kernel** — generalize the SHAPE of fathom's `calibration/` extractor-accuracy harness (`calibration/baseline.sh`, `calibration/corpus.yaml`): a golden/proxy-oracle corpus manifest (pinned source + expected counts) driven against a caller-supplied pipeline, producing a recall/precision report per category, with the proxy-vs-real-oracle distinction (`calibration/README.md`'s "not ground truth yet" honesty note) kept explicit in the type. This is a deliberately DIFFERENT shape from `kits/eval-runner`/`foundations/quality/qa-harness` (LLM-judge-scored agent-scenario evaluation, budget-guarded, cassette-replayed) — fathom's harness scores mechanical extraction recall against a static corpus, never an LLM judge call. Today it is a bash script, not a Go package, and has exactly one caller (`task calibrate*` in fathom's own Taskfile); no kernel exists to extract yet. {id: FA-12, disposition: deferred, reason: "fathom's calibration/ harness is bash, not a Go package, and has one consumer (fathom's own extractor calibration); no second repo runs a golden-corpus recall harness today", trigger: "a second named consumer"}

## Design constraints

- **D8 holds**: neither row gets typed, ships, or gets a package skeleton in
  this pass — filing the row IS the deliverable (per Phase K's K2 task: "file
  asks, don't build").
- If FA-11's trigger fires: the kernel package must stay evidence-agnostic —
  no `Evidence` implementation, no fathom-specific `core.SymbolNode`/tier
  vocabulary — mirroring the `evidenceladder.Tier1` precedent (kit ships the
  contract, the index-backed adapter stays downstream).
- If FA-12's trigger fires: the kernel must keep the proxy-oracle honesty
  boundary a first-class type distinction (a report must be able to say "this
  number came from a proxy, not ground truth"), not silently upgrade a grep
  proxy into a claimed oracle.
- Both rows require a **second named consumer**, not merely a second
  hypothetical use case — per this repo's own D8 doctrine, "duplication must
  demonstrably hurt first" (see FA-10's identical trigger discipline).

## Asks from fathom's evidence plane (ADR-026, 2026-08-26)

Filed by fathom's design review (`docs/design/evidence-plane.md` §11, plan row EP-34).
Each is an ask, not a build; the kit decides in its own `docs/intent/`.

- [ ] FA-13 One enforcement-rung ladder for `invariants.md` across the three Project Intent repos: admit `gate <task target | lefthook hook step>`, `task <target>`, `guard`, `lint <check>` beside `test <Name>` / `code` / `stated`, and say how a rung is written (leading token, backticks optional) so an instrument can classify it without a per-repo adapter {id: FA-13}
- [ ] FA-14 One decision-record shape rule: either one file per record (`docs/adr/NNNN-slug.md`) or one file with `## ADR-NNN` sections, each with a `**Status.**` line and a `**Revisit when.**` trailer — fathom's parser targets that rule, not three shapes {id: FA-14}
- [ ] FA-15 A `Status: proposed` decision in the kit's own `docs/intent/decisions/`: condition lines (`Done when` / `Promote when` / `Drop if` / `Revisit when`) MAY carry one `{oracle: index|plan|lexical|date, …}` annotation using the plan tokenizer; unannotated conditions stay prose and human-judged; an instrument may evaluate annotated ones and must never edit them. The practice's §13 rejection of lint tools and generated registries is kept: nothing generated is authored, prose is never evaluated {id: FA-15}
- [ ] FA-16 `intent-audit` and `intent-doctor` consume `fathom intent audit --json` when the binary is present and fall back to the documented grep otherwise; the skills state which source they used {id: FA-16}
- [ ] FA-17 The global "Project intent (all projects)" paragraph in the shared `~/.codex/AGENTS.md` (imported by `~/.claude/CLAUDE.md`) shrinks to one pointer line (the router carries the rest); measured today at ≈1,056 B ≈ 264 tokens loaded in every session of every repo, including the ones without `docs/intent/` {id: FA-17}
- [ ] FA-18 The practice's own §13 condition — "A lint tool… Revisit when a second repo shows index drift the walk did not catch" — is named as under test by fathom's evidence plane (U4-class drift: an invariant naming a test that no longer exists is exactly what the hand walk misses); the kit decides in its own record whether fathom's `intent audit` counts as that tool or as the repo's instrument reading the practice's docs {id: FA-18}
- [ ] FA-19 `intent-prune` and `intent-decide` preserve `{…}` annotation blocks on condition lines verbatim; README §11b's ladder gains a `proven` example (`fathom intent audit`); `monorepo.md` gains the nearest-router-plus-root rule for tools that read nested intent dirs {id: FA-19}
- [x] FA-20 **D10's coverage discipline gets a mechanical counterpart.** D10 requires an `ABSENT_PROVEN` verdict to carry a `CoverageManifest` accounting for every file as searched-or-skipped. That discipline exists only for LLM-derived evidence; the kit has nothing for the deterministic reads that back most tooling, and the gap is not theoretical. Measured in fathom 2026-08-27: `chmod 000 docs/intent/invariants.md` made `fathom intent audit` report 0 invariants instead of 37 and 85 unproven claims instead of 169, with nothing naming the file — a repository's most broken state reported as its healthiest-looking one, because eight readers did `if err != nil { continue }` and collapsed "absent", "unreadable" and "empty" into one fact. fathom's `internal/readset` (ADR-027) is the shape that fixes it: a read is `read | absent | unreadable(errno) | skipped(category)`, only `unreadable` makes a manifest incomplete, first-outcome-wins so a retry cannot erase a recorded blindness, and every count over an incomplete manifest renders as a NAMED lower bound rather than a total. **Not a promotion request yet** — D13 wants two consumers and fathom is one; file this so the second consumer (throughline's evidence reads are the likely candidate) has something to point at Closed 2026-09-02 by FB-8/FB-9 (practices §17). {id: FA-20, symbol: NewSet, package: foundations/telemetry/read-manifest}
- [x] FA-21 **The practice should say that an instrument may not degrade toward "healthy".** `docs/project-intent/` tells an author to record invariants and enforcement rungs; nothing tells the TOOL author that a check which cannot see must not report a pass, a skip, or a smaller number. Three failures of exactly this shape were found in fathom in one sitting: a freshness check reporting "go.mod does not declare <module>" against a repo whose go.mod declares it on line 1 (the read had failed); a dead-guard check whose remedy told the reader to delete a working guard because the files it covers are not INDEXED; and a router surface row (`internal/instrument*`) whose globs matched nothing and had silently stopped guarding the measurement code. Suggested: one paragraph in the practice's README and a line in `invariants.md`'s rung table — a rung asserted by a check that cannot distinguish "false" from "could not look" is `stated`, not `test`. Closed 2026-09-02 by FB-8/FB-9 (practices §17). {id: FA-21, symbol: walkWithManifest, package: cmd/constitution-lint}
- [ ] FA-22 **A harness manifest contract, when a second consumer deletes its copy.** fathom's ADR-029 (2026-09-02) put the coding-harness port in fathom (`internal/harness`: a descriptor registry — instruction files, hook files and event bindings with stdin/stdout field maps, `failMode`, scope-merge rule, skills dirs, MCP target, env contract, `verified_against` — plus one codec per harness) with a CUE schema over the exported descriptor so the shape is ready to move. This repo carries the same knowledge hand-rolled in three places: `docs/project-intent/integration.md`'s per-harness stanza table, `scripts/install-intent.sh`'s symlink targets, and the fathom-pack. D8/D13: the contract lands in `foundations/environment` only when this repo deletes those three onto it (`cmd/intent-install` reading manifests) — then fathom's registry exports through the kit type and the kit gains a `harnesstest` conformance suite modeled on `llm-test`. Not before: a single importer is a framework wearing a package's clothes {id: FA-22, disposition: deferred, reason: "one consumer (fathom); the kit's own three hand-rolls are the second consumer only once they are deleted onto the contract", trigger: "this repo rewrites install-intent.sh and the integration.md harness table onto a manifest"}
