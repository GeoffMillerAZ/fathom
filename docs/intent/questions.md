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
ADR-026 evaluates only annotated conditions; the practice's template says conditions are
prose and rejected generated registries (README §13). This needs a kit decision record
(`Status: proposed` filed as EP-34), not a skill note. Blocks: `docs/plans/evidence-plane.md`
wave 2 (K0's demand signal cannot form if annotations are refused). Answerable by: owner,
in agentic-kit's `docs/intent/`.

**Q19. Where does first-seen live across machines?** ADR-026's intent ledger is per-machine
(`~/.fathom`), so "fired, first seen on this machine" differs from a second machine's view;
a committed `.fathom/intent-ledger.jsonl` (the `.fathom/observations.md` precedent) would
share it through git. Blocks: EP-6, K1 on a second machine (F15). Answerable by: owner.

**Q20. Who owns the `{oracle: …}` grammar if a second evaluator appears?** Today fathom is
the only evaluator; if the practice accepts several, the grammar moves to agentic-kit and
fathom pins a version. Blocks: EP-2's key set, EP-34. Answerable by: owner, with the kit.

**Q21. The ADR-016 obligation ceiling: enforce at 150, amend it, or buy the
precision measurement first?** Measured 2026-08-27 over 1,918 post-edit and
370 pre-edit payloads (28 days): the 150-token cap is missed on 47% of
post-edit payloads and 83% of pre-edit guards, p95 ≈ 329 — about 2.2× the
limit. Enforcing it as written would drop a median of TWO rules from a
typical over-budget payload, on roughly half of all edits, with no evidence
about which two: precision is NOT computable, because the beat log records
payload bytes and rule counts, never payload text. Full measurement:
`../measurements/2026-08-27-obligation-ceiling.md`. Recommendation there:
amend ADR-016 to a measured ceiling, then treat drop-ranking as separate
work. Blocks: HF-4, and I35 stays VIOLATED until this is answered — an
invariant nobody can satisfy. Answerable by: owner.

