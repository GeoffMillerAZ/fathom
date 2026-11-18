# Dependency edges

Grammar v2 (`docs/plans/README.md`). Prefix DEP-. Decision ADR-114.
[Design](../design/dependency-edges.md) · [Contract](../contracts/dependency-edges.md).

## Records and public door

- [x] (op) Record the accepted dependency-edges boundary, contract, preregistered admission bars and register {id: DEP-0, oracle: lexical, path: ADR.md, pattern: "^## ADR-114: Standalone dependency edges disclose unknowns per file"}
- [ ] Public door, closed sets, completeness, deterministic output, CUE schema and API snapshot {id: DEP-1, needs: DEP-0}
- [ ] Go control reader and independent-oracle grading harness {id: DEP-2, needs: DEP-1}
- [ ] JSON CLI with golden and the declared-edge export {id: DEP-3, needs: "DEP-1,DEP-2"}

## Dialect readings and extension

- [ ] TypeScript and JavaScript reader and resolution with DEP-M1 {id: DEP-4, needs: DEP-3}
- [ ] Python reader and resolution with DEP-M2 {id: DEP-5, needs: DEP-3}
- [ ] Dockerfile reader and resolution with DEP-M3 {id: DEP-6, needs: DEP-3}
- [ ] Resolver registration, pack captures where needed, declared edges, trust, conformance helper and runnable example {id: DEP-7, needs: "DEP-4,DEP-5,DEP-6"}

## Deferred

- [-] Audit under-keying by comparing declared inputs, static edges and observed reads (formerly TQ-38) {id: DEP-8, disposition: deferred, reason: "the consumer is named and needs measured static edges", trigger: "DEP-M1 through DEP-M3 have been read and one consumer wires dependency edges"}
- [-] Move the existing closure engine into the tool family as its own module {id: DEP-P1, disposition: deferred, reason: "the move amends ADR-063 O1 and the CI kit's boundary", trigger: "two of DEP-M1 through DEP-M3 pass B1 and B2 and the owner approves the move"}
- [-] Add remaining dialects, cheapest class first {id: DEP-9, disposition: deferred, reason: "first prove the readers and the declared exchange", trigger: "DEP-P1 lands or a second consumer is named"}
