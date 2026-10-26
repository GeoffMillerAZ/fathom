# Inventory/evidence/review design verification

Baseline: `5af1c0b10ecb6b245b3f032b62cbe85ef2f71138`, 2026-10-23.
Deliverable: accepted design [0037](../../../intent/decisions/0037-inventory-and-evidence-compose-with-domain-reviews.md),
[IER contracts](../../../designs/inventory-evidence-review.md),
[EDR workflows](../../../designs/evidence-driven-reviews.md) and 14 deferred IR rows.
Implementation remains unbuilt and unadmitted. No public package/API/schema changed.

## Evidence

- Two explicit Astra consultations through one read-only Herdr pane informed kit
  ownership and then workflow-specific improvements. [Dispositions and handoff](../../../research/2026-10-23-inventory-evidence-review.md)
  distinguish consultation advice from tests. The owned consultation pane was closed;
  bounded captures are retained privately with [paths and SHA-256 pins](source-pins.json).
- Scoped source inspection records 16 exact file digests across Agentic Kit and the
  concurrently dirty Platform Engineering checkout. Platform source was read only;
  its proposed contracts are not presented as implemented consumer behavior.
- An independent final documentation review identified a G12/G13 routing omission
  and missing capture locators. Both were corrected; the reviewer rechecked matching
  capture bytes/digests and the complete G9..G14 goal mapping.
- Full `task validate` passed: formatting/vet/CUE, constitution and race suites
  including the existing nested modules. No runtime code, dependency, gate, global
  configuration or consumer checkout was changed.
- Strict work-order parsing recognized all 14 deferred rows and zero claimed rows,
  with zero failing. This proves parsing/dispositions, not implementation.
- Relative file links, unique IR IDs, EDR-A01..31 coverage and JSON source records are
  checked. Router 80 lines, scope 150, futures 145 and decision 0037 under 80 lines.

The new scenario-template mapping in EDR-6 is a domain-owned catalog profile, not
a general incident engine. Unknown prerequisites/unsupported families remain gaps;
its positive/negative requirement is EDR-A31. Existing lesson acceptance remains required.

## Limits

No live provider/scanner, source migration, model calibration study, UI, runtime
interop, review accuracy, cost saving or incident likelihood is demonstrated here.
Native third-party Review Kit ingress must be qualified first; existing producers and
internal portal readers do not establish that ingress. No whole G9..G14 exit is met.
F17/F19/F20 retain their separate promotion gates. IR-01 is the first candidate for
later bounded admission; the active session has the durable handoff rather than a
duplicate self-queued prompt.
