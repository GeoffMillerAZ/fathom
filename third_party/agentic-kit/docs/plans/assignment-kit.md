# Assignment Kit implementation

Request (Geoff, 2026-10-23): "whatever you decide on these kits fully spec out and implement to completion".

Authority: [0036](../intent/decisions/0036-assignments-compose-goals-evidence-and-stopping-rules.md).
Scope and acceptance: [AS-1..8 / AS-A01..10](../designs/assignment-kit.md).
Pure offline library and two reference workflows. No executor, provider, installation,
new intent store or claimed production adoption. Same bounded W1 reference slot.

- [x] AS-1: bounded typed assignment and graph validation {id: AS1, symbol: Validate, package: kits/assignment-kit}
- [x] AS-2: independent expected coverage and existing profile evidence assessment {id: AS2, symbol: Evaluate, package: kits/assignment-kit, needs: AS1}
- [x] AS-3: exact confidence and observation bindings {id: AS3, symbol: ObservationFingerprint, package: kits/assignment-kit, needs: AS2}
- [x] AS-4: blocking/advisory concern projection {id: AS4, symbol: Evaluate, package: kits/assignment-kit, needs: AS2}
- [x] AS-5: finite budgets/deadline and deterministic escalation/pivot/stop causes {id: AS5, symbol: Evaluate, package: kits/assignment-kit, needs: AS4}
- [x] AS-6: immutable canonical assignment identity and detached receipts {id: AS6, symbol: Fingerprint, package: kits/assignment-kit, needs: AS5}
- [x] AS-7: runnable catalog-backed command and investigation workflows {id: AS7, symbol: snapshots, package: examples/assignment-kit, needs: AS6}
- [x] AS-8: boundary fixtures, agent/human adoption and independent trial {id: AS8, symbol: Evaluate, package: kits/assignment-kit, needs: AS7}

AS-1..8 passed the full gate, independent review, guide trial and committed-candidate
consumer checks. The declared aggregate passes under supported correctness-test
capacity 4; unchanged timeout and performance gates are retained. [Delivery](../evidence/assignment-kit/2026-10-23/README.md) preserves failed attempts and scope. Strict
verification after these claims passed all 15 claimed CG/AS rows; static proof is not statistical calibration.
