# Measurements

One file per measurement that a decision rests on: what was measured, over
what window, and — the part that makes it evidence rather than a number —
what could NOT be measured and why.

A measurement lives here (not in `workspace/artifacts/`, which is gitignored)
when a record, an invariant, or a plan row cites it. The raw artifact may
stay in `workspace/`; the finding has to survive a clean checkout.

Rules: no figure without its n and its window. An estimate is labelled
ESTIMATE. An uncomputable half is named, never dropped (law 3), and a
measurement never carries a savings claim (law 12).

| Measurement | Decides |
|---|---|
| [2026-08-27 — the obligation ceiling](2026-08-27-obligation-ceiling.md) | Q21 / HF-4: whether ADR-016's 150-token cap is enforced, amended, or bought out |
