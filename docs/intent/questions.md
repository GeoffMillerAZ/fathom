# Open questions

Unresolved matters that change design. Each says what depends on the answer and who can
answer it. Delete when answered; the answer lands in the doc or record it affects.

**Q3. Delete the 24 stale worktrees and 22 `worktree-*` branches (INC-1)?** Blocks:
INC-1 closure. Answerable by: owner (deletion is ask-first).

**Q4. Force-multiplier or product?** `docs/vision/07` Q4 ("who pays, and does anyone
need to?") has no written answer; it decides whether F2 (public release) is promoted or
dropped and what the A/B verdict (G2) is for. Answerable by: owner.

**Q5. The threat model.** The fleet's source is served unauthenticated on loopback to any
local process (`docs/vision/20` §7 Q2). Accept single-user in writing, or make law 9
hardening a goal before any second adopter? Blocks: F8, F15, `domain.md` assumption 2.
Answerable by: owner.

**Q6. Has XT-6b's trigger fired?** ADR-013 kept "the WASM path … if a pure-Go runtime
matures"; `tsengine` now runs four languages on wazero. Is the nine-language migration
wanted, and what is its Promote when (F1)? Answerable by: owner plus the calibration corpus.

**Q16. Should honesty-debt counts ever leave the machine?** Only through the existing
sanitized research bundle, opt-in, as counts by kind (law 13). Blocks: EP-23's fleet
column and any bundle field. Answerable by: owner.

**Q18. Does the practice's owner accept `{oracle: …}` annotations on condition lines?**
FILED, not answered: agentic-kit `docs/intent/decisions/0001-condition-lines-may-carry-oracle-annotations.md`
(`Status: proposed`, 2026-08-27, commit 72ff8b9). Its own revisit condition is the gate —
three hand-written annotations across two repos accepts it, thirty days with fewer rejects
it. Blocks: `docs/plans/evidence-plane.md` EP-16/17/18. Answerable by: owner, by ratifying
or rejecting that record — and by writing the first annotation, which is the demand signal
fathom must not fake for itself (EP-43 refused exactly that).

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
