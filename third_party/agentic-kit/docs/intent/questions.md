# Open questions

Unresolved matters that change design. Delete when answered; the answer lands in the doc
or record it affects. Reviewed 2026-09-13; workflow adoption does not require public release.

**Q1. Does the kit go public, and at 1.0?** Uplift D4 is explicitly unratified
("genuinely yours, Geoff"); H8 Stage B is blocked on it. Blocks: `scope.md` G1,
`futures.md` F15, the form of I16/I17. Answerable by: Geoff, as a decision record.

**Q5. What is the eval corpus's provenance ruling?** TRACKING marks "corpus provenance
ratification = owner hand-back." Blocks: eval-runner verdicts being citable. Answerable
by: Geoff.

**Q7. What baseline should the first attended prose pilot beat?** Choose
`resume_as_code`'s constrained fact-to-document workflow before throughline's contested
evidence. Synthetic contract tests have passed; accepted supported claims, false blocks
and human review effort still need an authorized real-workload baseline. No production
content or semantic effectiveness is inferred from the provider/lesson adoption.

**Q8. What additional live host coverage is worth its authority and cost?** The
[recorded matrix](../evidence/native-hosts/2026-09-13/README.md) pins known limitations:
Claude's untrusted fixture cannot prove allowed editing, Codex live tools are unknown,
and compaction/nested-instruction/repeated-run behavior is not established. Expand only
under normal native trust and a verified spend bound; configuration success is not
enforcement. The initial setup trial took 142 seconds, not a human-usability sample.

**Q9. How does a portable receipt state its producer's origin?** fathom's planned receipt
record carries "a producer in agentic-kit's `evidence-producer` shape (kind deterministic,
origin self)". Here `Origin` is relative to the consuming store, and `Catalog.Admit`
refuses `self` as circular (`foundations/evidence/evidence-producer/types.go:83-96,260-265`),
so a store that copies the field refuses fathom's facts. Options: omit origin on the wire and
let each consumer's catalog assign it, or define a wire field that means "computed by the
producer, not imported". Blocks: fathom's EVX-16 shape, FX-2 in `docs/plans/fathom.md`.
Answerable by: the owner with fathom's receipt design, before EVX-16 is built.
