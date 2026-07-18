# Evidence spine — verified capabilities (dogfood plan)

Contract-bearing checklist gated by `fathom verify` at pre-push (convention:
fathom's `docs/plans/README.md`). `[x]` is a claim; claims are checked against
the code index.

- [x] Conflict detection across assertions with proposition awareness {symbol: DetectWithPropositions}
- [x] Precondition gate for evidence signatures {symbol: PreconditionSatisfied}
- [x] Evidence ladder evaluation against a concern signature {symbol: Run, package: foundations/evidence/evidence-ladder}
- [x] Case file assembly from assertions and spans {symbol: Build, package: foundations/evidence/case-file}
