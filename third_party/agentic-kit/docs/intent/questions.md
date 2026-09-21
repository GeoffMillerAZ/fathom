# Open questions

Unresolved matters that change design. Delete when answered; the answer lands in the doc
or record it affects. Bootstrapped 2026-08-25 from the repo's own open markers.

**Q1. Does the kit go public, and at 1.0?** Uplift D4 is explicitly unratified
("genuinely yours, Geoff"); H8 Stage B is blocked on it. Blocks: `scope.md` G1,
`futures.md` F15, the form of I16/I17. Answerable by: Geoff, as a decision record.

**Q2. Is `kits/guardrails-pipeline` `stable`?** It is `experimental` with a live
production consumer, which means that consumer has no compatibility promise
(`compat.md` §75–83). Blocks: G2; throughline's upgrade risk. Answerable by: Geoff
(promote, or record accepted risk).

**Q3. Should `CallTool` verify the pre-call pin?** TRACKING H8-1: the docs overclaimed
this behavior, were corrected, and the row now governs the gap. Blocks: G3; the security
baseline's honesty. Answerable by: Geoff ("decision Geoff's").

**Q4. Should D1–D10 move from the uplift plan into decision records?** They are the
kit's load-bearing decisions and live in a plan marked EXECUTED. The index points at
them; a plan is a poor long-term home. Blocks: nothing today; discoverability.
Answerable by: the next time one is reopened (promote then, per the practice).

**Q5. What is the eval corpus's provenance ruling?** TRACKING marks "corpus provenance
ratification = owner hand-back." Blocks: eval-runner verdicts being citable. Answerable
by: Geoff.

**Q6. Should I12–I14 (injected clocks, downward imports, no `_reference` imports) get
a lint?** They are the three `stated` invariants most likely to be violated silently; a
`go/parser` test is small. Blocks: nothing; the evidence ladder. Answerable by: an agent
with `intent-decide` if Geoff agrees it is worth a constitution check.
