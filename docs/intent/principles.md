# Principles

How to reason when goals pull against each other. Principles guide judgment; invariants
(`invariants.md`) are tests. Laws 1–15 are the repo's own doctrine, adopted in the vision
series; the wording below is theirs.

## Principles

**P1. Governed evidence is the moat.** Claims with provenance, tiers, staleness, and
refutation durability; "competitors do vibes." Source: law 1, `docs/vision/11` §2.

**P2. Determinism first.** CUE-validated rules and pure analyzers wherever possible; the
LLM only at the semantic margin, always eval-gated. Source: law 2; ADR-012.

**P3. Never silent.** Cited refusals beat confident fabrication; "empty config reads as
clean" is a lie; an uninstrumented metric renders as absent, never as zero. Source: law 3;
`cmd/fathom/CLAUDE.md` (`stat.Absent`).

**P4. Consolidate, don't grow.** No net tool growth; every addition rides a forced
channel. Source: law 4; `docs/vision/05-surfaces.md`.

**P5. Minimal theater.** Every move carries a kill criterion or a done-when-stamped
terminal state. Source: law 5.

**P6. Activation is stamped, never prose-documented.** An operator step writes a stamp
file the moment it runs; plans cite stamps. Source: law 6.

**P7. Instruments get existence checks.** A measurement whose source can silently vanish
is theater; every channel ships an "is anything flowing?" assertion. Source: law 7.

**P8. Kill clocks require a consumer population.** Zero-call while consumers are paused
proves exposure-zero, not demand-zero; a tool is held as unmeasured, never killed, until
qualifying traffic exists. Source: law 8.

**P9. Hosted read-only before writable.** No writable hosted surface, ever, before auth
and read-only are proven. Source: law 9, `docs/vision/12`.

**P10. Verb-first surfaces.** Every capability is a scriptable CLI verb first; TUI and
web render the same state; a surface that can do something the verbs cannot is a bug.
Source: law 10, `docs/vision/13` §3.

**P11. The advisor recommends; the gate decides.** Agentic passes propose with citations;
only deterministic checks block. Source: law 11.

**P12. No saving claim without a counterfactual.** A token figure is a cost until an arm
establishes the alternative; if the A/B shows ON no better, every savings claim is
retracted where it appears. Source: law 12, `docs/vision/14`.

**P13. User scope contracts, repo scope governs; sharing is a human verb.** Nothing
fathom writes crosses the network unasked; the sanitized bundle is the only shareable
artifact and a human runs the verb. Source: law 13, `docs/vision/14` §5, `docs/design/capability-registry.md`.

**P14. Narrative is a lens, never a source of truth.** Nothing downstream consumes a
story row as evidence; a claim persists no higher than its enforceable tier.
Source: law 14, `docs/vision/14` §6, `docs/design/truth-seeking-tiers.md`.

**P15. Fathom writes only what it owns and never resolves a conflict.** Inside its own
markers; a drifted region stops and reports; every write has a receipt. Source: law 15, `docs/vision/20` §6.

**P16. Delivery is configuration, not agent habit.** Hooks inject, gates block; the tool
surface is the secondary pull path. Source: `docs/design/product-vision.md` §2.

**P17. State is computed, never stored.** Markdown is authored, Go evaluates, CUE
validates; every status is a re-derivation. Source: ADR-018, `docs/design/sdd-reconciler.md` doctrine 1.

**P18. Fail open on infrastructure, closed on evidence.** A hook never blocks because a
daemon died; a gate never passes a claim the index refutes. Source: `docs/design/coding-agent-excellence.md` §5.3.

**P19. Costs scale with change size, never repo size.** Hash-gated ingest, digest-gated
compile. Source: ADR-005, product-vision standing non-goals.

**P20. Build for a named consumer; generalize from two, not one.** A pack, layer, or
abstraction ahead of demand is theater. Source: ADR-024, `docs/vision/07` R3.

**P21. Monorepo now, split on an enforced boundary.** Split when a boundary already
exists and is enforced; never to create one. Source: `docs/handoff/repo-strategy.md`.

**P22. Measured, not argued.** Every repo-facing number cites an instrument; completion
claims cite `fathom verify` or test output. Source: `docs/vision/README.md` "How this
review was grounded", `CLAUDE.md`.

**P23. A guest on the operator's machine.** Fathom declares what it takes in the unit the
host measures (`phys_footprint`), measures itself and stops itself (shed, refuse, leave,
each with a reason), does less when the host is struggling, leaves when asked or when its
parent leaves, and stays visible when it cannot answer. A request pays for its answer, never
for the repository (P19). Self-protection is deterministic thresholds, never judgement.
Source: ADR-047, `docs/vision/36-responsible-tenant.md`, INC-23.

**P24. Judgment rides the improvement curve; proof never moves.** Fast typed-judgment models (Jev is the first)
will keep improving in accuracy, calibration, price and latency, and fathom rides that curve without redesign:
every judgment is a typed decision-port site (a schema plus a provider), every site owns a calibration set that
grows, features are deterministic with provider none and use judgment only in the bands where calibration has
passed, judgments are cached by (template, content hash) stamped with provider and model and kept apart from
proof, and nothing locks in a vendor. A model answer is never a receipt a stranger can re-derive, however good
the model becomes, and judgment never replaces proof for anything fathom can compute. No feature is built whose
value exists only if a future model arrives. Source: ADR-056, `docs/design/judgment-foundations.md` section 2.

## Tensions

**T1. Fail-open pre-push vs gate strength.** Default: pre-push fails open on exit 2, CI
fails closed. A bypass under a wedge is named in the thread note, never silent
(`incidents.md`). Source: §5.3; `docs/vision/24` §7.

**T2. Surface breadth vs support liability.** Every registered tool is a forever promise.
Default: the 30-day clock deletes, but only once a consumer population exists (P8).
Source: `docs/vision/05`, `docs/plans/surface-prune.md`.

**T3. Forced channels vs token cost.** Injection pays cache rent (the measured 22× line).
Default: bounded payloads (≤320-token obligations payload, the measured p90 — ADR-016
amended 2026-09-24; ≤600-token brief) and a
pre-registered saving with a kill. Source: `docs/vision/21`, `docs/design/budgeted-answers.md`.

**T4. Determinism vs LLM-compiled knowledge.** Default: the index is the spine; claims
are additive, tiered, swept, and served flagged. Source: ADR-016.

**T5. Pre-registration vs learning from the data.** Default: a criterion may be amended
only before data exists under it, and the amendment is recorded (MI-5). Source: `CHANGELOG.md` 2026-08-12.

**T6. Dogfood at home vs an org-agnostic product.** Home-repo assumptions made doctor cry
wolf in every other repo (O23). Default: a check an operator cannot clear is not a FAIL.
Source: `docs/dogfood-observations.md` O23.

**T7. Velocity vs second-system risk.** Five waves in four weeks left dead packages and
unwired schemas. Default: subtraction and entry conditions before new capability.
Source: `docs/vision/07`.

**T8. Honest refusal vs erosion.** A gate escaped twelve times trains the `skip_test`
reflex. Default: fix recall so the escape is unnecessary; never loosen the gate.
Source: `.fathom/observations.md` O3, `docs/plans/verify-spine.md`.
