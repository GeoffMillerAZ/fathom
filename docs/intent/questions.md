# Open questions

Unresolved matters that change design. Each says what depends on the answer and who can
answer it. Delete when answered; the answer lands in the doc or record it affects.

**Q4. Force-multiplier or product?** `docs/vision/07` Q4 ("who pays, and does anyone
need to?") has no written answer; it decides whether F2 (public release) is promoted or
dropped and what the A/B verdict (G2) is for. Evidence added 2026-09-27
(`docs/assessments/2026-09-27-agentic-coding-landscape.md`): free-only tools struggled (Roo Code shut down, Vibe
Kanban found no business model, Continue was absorbed by Cursor), the money went to review and governance (fathom's
lane), and stars followed one-command packaging more than depth. Answerable by: owner.

**Q5. The threat model.** The fleet's source is served unauthenticated on loopback to any
local process (`docs/vision/20` §7 Q2). Accept single-user in writing, or make law 9
hardening a goal before any second adopter? Blocks: F8, F15, `domain.md` assumption 2.
Answerable by: owner.

**Q16. Should honesty-debt counts ever leave the machine?** Only through the existing
sanitized research bundle, opt-in, as counts by kind (law 13). Blocks: EP-23's fleet
column and any bundle field. Answerable by: owner.

**Q19. Where does first-seen live across machines?** ADR-026's intent ledger is per-machine
(`~/.fathom`), so "fired, first seen on this machine" differs from a second machine's view;
a committed `.fathom/intent-ledger.jsonl` (the `.fathom/observations.md` precedent) would
share it through git. Blocks: EP-6, K1 on a second machine (F15). Answerable by: owner.

**Q20. Who owns the `{oracle: …}` grammar if a second evaluator appears?** Today fathom is
the only evaluator; if the practice accepts several, the grammar moves to agentic-kit and
fathom pins a version. Blocks: EP-2's key set, EP-34. Answerable by: owner, with the kit.

**Q23. When is a test edit a legitimate refactor and when is it a weakening?** The
integrity check normalises renames and counts assertions; an assertion rewritten as a
helper call, a table case merged, or a matcher swapped for an equivalent still flags as
`edited` or `weakened`. CV-M2's replay sets the false-flag rate. Blocks: arming
`refactoring.proof-integrity` at deny. Answerable by: evidence (CV-M2). **Measured:**
`docs/bench/readings/cv-m2-guarded-test-commit-replay-2026-09-24.md` — 0 of the last 50
commits touching guarded tests produced a true loss-kind finding (0% against the design's
10% ceiling); 9/50 produced `edited`, the informational-only kind, which never gates.
`docs/bench/readings/cv-m1-guards-pilot-8-replay-2026-09-24.md` confirms the other
direction: both real i28 cells still flag `deleted`. The false-flag rate this question
asked for is now measured near zero on this sample; whether that generalizes past 50
commits, and the deeper conceptual line between an edit and a weakening, stay open.

**Q24. Should conservation block at push, or only in CI, and for whom?** Today it warns
at push (fail-open, I8/I17) and CI runs `--strict`. A team shipping library APIs may want
push-time blocking on `surface` losses; an application team may not. Blocks: nothing
today. Answerable by: owner, per consumer.

**Q25. How do certificates, ratchets and baselines behave on a long-lived branch while main
keeps moving?** A baseline pinned weeks ago drifts; runs attested on a branch commit are
not on main; I30 forbids re-rooting a service at a worktree, so mid-flight answers may
reflect main. `fathom refactor status` reports drift; whether that is enough is open.
Blocks: CV-M6 on a multi-week program. Answerable by: evidence.

**Q26. Which refactors does this round not see?** Data and schema migrations (store
formats, SQL DDL), infrastructure moves (Terraform `moved` blocks, Kubernetes apiVersion),
security properties (an authorization chain dropped when a handler moves), performance and
concurrency (bench certificates, race-attested runs). Futures F25 and F26 hold them.
Answerable by: the first consumer who loses one.

**Q27. When agents write most code, what should teams choose languages and architectures
for, and how can fathom measure it honestly?** `fathom report evidence` gives evidence
density per service (typed share, certificate density, inventory, receipts) and ranks no
language; a cross-language comparison waits until at least three languages have measured
typed edges (CV-M7). Answerable by: evidence, then owner. **Evidence so far (threshold not
met): CV-M7** (`docs/bench/readings/l0-ts-py-2026-09-24.md`) measured the file-resolved
tier, not typed edges, for two languages: TypeScript (zod) and Python (flask), 26/26
hand-verified correct; the scip and file tiers separately scored 0.87–0.95 precision,
3–12% recall (high precision, low recall, as the tier's own name promises). This is real
per-language measurement but neither the tier this question names (typed edges) nor the
language count (two, not three) — the cross-language comparison this question asks about
stays blocked.

**Q28. Promote the `refactoring-conservation` pack from warn to deny?** Q22 (what a reviewer
must be shown to accept a large agent refactor without reading the diff) is answered by
measurement in `docs/design/refactor-conservation.md` §3.6: on this repository's CV wave-3
range the reviewer opens 13 of 31 changed functions, each at the changed lines that never
ran, plus the 21 changed files fathom does not analyze, instead of 47 files; on a
weaker-tested repository the flagged set grows instead of hiding the gaps. At deny the
pack's findings block instead of warn. Blocks: arming the pack at deny
(`.reviewkit/config.cue`). Answerable by: owner.

**Q29. CLA, or DCO plus an explicit grant, for outside contributions?** FSL-1.1-ALv2
(ADR-052) keeps dual or commercial licensing possible only if the licensor holds enough
rights in every contribution. Blocks: accepting a first outside contribution; F2. Answerable
by: owner (with counsel). Nothing is built until a contribution is offered.

**Q30. Is "Fathom" protectable as a trademark?** Other products use the name; the licence's
Trademarks clause reserves marks but establishes none. Blocks: F2's public naming (a
marketplace listing, a brew tap). Answerable by: owner (with counsel). Not legal advice.

**Q31. Can a fast typed-judgment model read code well enough to order or prune anything?** Nobody has
measured it: the model reads literally, cannot count or compare and does not follow indirection, and code is
full of indirection. J0 (`docs/plans/judged-predicates.md` JP-M1) is the kill screen, with a lexical baseline the
model must beat. Blocks: `fathom ask`, the traversal scorer and files at scale (F34, F36, F37). Answerable by:
evidence (JP-M1), then owner. Amended 2026-10-16 (ADR-073): stage 0's E1 and E2 (`docs/plans/agentic-decisions.md`
EXP-M1, EXP-M2) are its first, smaller reading, and EXP-M4 gates `ask`'s judged column; JP-M1 still gates the rest.
2026-10-17: that first reading says no at stage-0 scale (EXP-M1 NONE, EXP-M2 NONE, EXP-M3 STOP, EXP-M4 CLOSED,
`docs/bench/readings/decide/exp-stage-gate.md`), so `ask`'s judged column stopped; JP-M1 is still unread.

**Q32. What do the harness hooks carry at the moments the judgment program needs?** Answered for Claude
Code 2.1.284 by TR-V1 (`docs/bench/readings/test-run-hook/hook-facts.md`): PostToolUse(Bash) carries no exit
code; an unpiped failing command arrives as PostToolUseFailure with `Exit code N` leading `error`, a piped one
(536 of 541 failing runs scanned) as PostToolUse. XM-V1 (`docs/bench/readings/exploration-maps/handoff-facts.md`):
the SubagentStart input carries no prompt (the common fields plus `agent_id` and `agent_type`), and RC-28's
591-byte rows are the constant base primer with no discoveries brief (549 of 874 emitted rows are base-only).
PR-V1 (`docs/bench/readings/served-not-kept/hook-facts.md`, 2026-09-29): Claude Code 2.1.284's
PostToolUse(Bash) carries the command's stdout and, when git prints it, the `[branch sha]` summary line (also
pre-parsed as `gitOperation.commit.sha`), but `git commit -q` prints none (71 percent of the 2,446 successful commit
results scanned, which fired ADR-057's revisit trigger); context added by a PostToolUse hook reaches the model in
Claude Code and, observed for `apply_patch`, in Codex 0.158.0. Unverified: whether SessionStart with source `compact`
fires and accepts additional context (CA-V1); what Codex's PostToolUse `tool_response` holds for a shell command
(one captured Bash hook input settles it; no $0 source does); what a Stop block does in Claude Code (PR-V2); whether
hooks fired inside a sub-agent carry `agent_id` (PR-V3). Blocks: the test-run hook's registration and exit-code use,
carry-across, the commit pin line for Codex (its shell input is unverified), turn-end feedback (PR-D2) and
orchestrator constraints (F54). Answerable by: evidence (the verification readings).

**Q33. Unfreeze the assess lanes (ADR-046) as the judged-telemetry host, or supersede them?** Judged telemetry
would otherwise rebuild assess's consent capability, spend isolation and judge process, and a parallel lane is
refused; assess itself is frozen by ADR-053. Blocks: the sampler, queue and judgments table (F40), needed only
after RT-M1 reads PASS. Answerable by: owner.

**Q35. Do the owner's own interactive sessions turn readouts' lean mode on (Q-RO-2's open half, ADR-071)?**
Yes or no, answerable by the owner; no bench spend, though lean may raise a session's own spend. Until then lean
runs only under `[readouts] roots`, the owner-architect's wave sessions (RO-20). Q-RO-1, Q-RO-3, Q-RO-4 and the rest
of Q-RO-2 were answered 2026-10-16; they live in ADR-071 and `docs/plans/readouts.md`. Delete this entry when answered.

**Q36. Do the owner's own interactive sessions install fathom-ui (Q-MOD-2's open half, ADR-072)?** Yes or no,
answerable by the owner. The wave sessions load it once MOD-M2 reads PASS; Q-MOD-1 and that half were answered
2026-10-16 and live in ADR-072 and `docs/plans/claude-code-mods.md`. Delete this entry when answered.

**Q37. The decision-model program's two open owner inputs (ADR-073).** Q-DM5, labels beyond ADR-056 decision 14 (an
optional spot-check of E2's frontier labels, and the H2 phrase labels RJ-D1 needs); and stage 2's spend (JP-14, $90 or
more), asked only after EXP-M5 reads KEEP. Answerable by the owner. 2026-10-17: EXP-M4 read CLOSED, so stage 2 stopped
and its spend is not asked (JP-14 is back behind J0); Q-DM5 alone is open. Q-DM1 to Q-DM4, Q-DM6 and Q-DM7 were answered
2026-10-16 and live in ADR-073. Delete this entry when both are answered.
