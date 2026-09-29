# 0017: Fast judgment is an optional, calibrated tier; the kit ships guidance and a provider-neutral port, not thresholds

**Status:** accepted
**Type:** architecture
**Date:** 2026-09-29

## Context

`foundations/decision` landed 2026-09-22 (`None`, `LLM`, `TypeSafe`; DP-10/DP-11) and
fathom pulled for it. The Jev assessment
([2026-09-21](../../assessments/2026-09-21-jev-opportunities.md)) and a consumer's
program found three things: nobody has measured these models' accuracy on code; an
explicit `unknown` option did not produce reliable abstention in the kit's own live
smoke; and a consumer's routing site was right 45.7% of the time at confidence 0.9 or
above, non-monotonic. Meanwhile `BandOf` hard-codes 0.5 and 0.9, and a call site added
the usual way turned on in every project that set any provider. The owner directed on
2026-09-29 that (1) fast typed-judgment models are a bet the architecture rides without
redesign, (2) the decision port be hardened against vendor lock-in, and (3) guidance
good for other projects lives in this repository, org-agnostic.

## Decision

1. **The bet.** Fast, cheap typed-judgment models will keep improving in accuracy,
   calibration, price and latency; the port takes each one as a measurement, not a
   redesign. **Boundary:** judgment never replaces proof for anything computable, and a
   model answer is never a receipt a stranger can re-derive.
2. **Rules guidance states and kit code must not contradict.** Every judgment is a
   typed port site with a provider (`none`, an emulating LLM, a native provider, a
   future one), never an ad-hoc model call. A site's value survives provider `none`.
   Calibration is per (site, question version, provider, resolved model). Judgments
   are cached by (template version, content hash) and stamped with provider and model,
   apart from proof. Sites are opt-in individually; a configured provider never enables
   a new site by itself.
3. **The kit ships method and port, not thresholds.** [fast-judgment.md](../../fast-judgment.md)
   and its [catalog](../../fast-judgment-catalog.md) carry the calibration method and
   bars. `BandOf`'s 0.5 and 0.9 are documented as uncalibrated starting bands. No
   accuracy or savings claim is made without a reading.
4. **Placement.** Practice that is good for other projects is contributed here in
   project-agnostic form; the source project keeps its own evidence and program records.

## Alternatives rejected

- **A vendor as the default provider, or dropping `none`.** Loses the offline path and
  makes a feature's value depend on availability (sign-ups were reported paused a week
  after launch). Wins if: never for a governed workflow.
- **Calibrated default thresholds in the kit.** Thresholds belong to the question, the
  options, the model and the workload. Wins if: the kit itself ships a site with a
  shared, versioned reference corpus and a recalibration process.
- **A judgment as evidence.** The evidence doctrine ([practices §2](../../practices.md))
  forbids it. Wins if: never.
- **Keeping the guidance in the source project.** Other projects would not see it.

## Consequences

Guidance can go stale as vendor documents move; each vendor fact is dated and tagged.
The method has no in-kit tooling for Wilson bounds, Brier or AUROC, and `decision.Record`
lacks question version, digest and input hash. Until the readings library exists,
consumers keep their own calibration logs.

## Deferred

Accepted by the owner 2026-09-29, not started; they follow the reference consumer's in-flight push (owner's
sequencing). Recorded as open plan rows on 2026-09-29 (`docs/plans/decision-port.md`, section "Hexagonal
hardening", DP-12 to DP-24; the reference consumer's ADR-056 and its rows JF-13, JF-14 and JF-24 track the same
work). Renaming `KindNoul` and moving the adapters are breaking changes and need `compat.md`, a migration note
and a `### Breaking` line when they land (DP-24).

- A generic kind name (for example `KindPredicate`) with `noul` kept as a wire and config alias (DP-12).
- Adapters declare vendor limits (Score's 2 to 10 levels) instead of the core enforcing them (DP-13) and live in
  subpackages, so the core imports no vendor code (DP-14).
- `Capabilities()` (kinds, limits, batching, latency and cost class, confidence semantics) (DP-15) and a
  `decisiontest` conformance kit every adapter passes (DP-16).
- Decorators implementing `Decider`: cascade, fallback, cache, record and replay, with policy in configuration
  (DP-17 to DP-21); ensemble later, deferred with a trigger (DP-D1).
- Calibration mapping in the core (DP-22) and a readings library: n, accuracy by band, Wilson bounds, Brier,
  AUROC, label provenance, recalibration-due flag (DP-23).

## Revisit when

- A second consumer runs the calibration template and asks for shared tooling (admit the
  readings library under D13).
- A consumer's reading shows 0.9 is unsafe or too cautious for a site, making the band
  constants configuration.
- A provider's retention, training or access terms change so that guidance section 5 is wrong.
- A study passes a site's bar, so guidance can cite a measurement instead of "unmeasured".
- DP-12 to DP-24 are proven (the Deferred section closes), or the reference consumer's push does not land and the
  sequencing note goes stale.
