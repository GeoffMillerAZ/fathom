# Go field-only ENGINE adoption audit

C-R11 authorizes the C-R2 lever, field symbols without reference edges, subject
to K17/K26 and every affected reading. This branch starts on Wave A main
2b6f341e, with PR #52's reviewed cost proof merged as a declared prerequisite.
The production port is the previously measured implementation; it preserves
call targets and bare plan anchors. Five-package race verification passed,
and independent review found no implementation issue.

The first Wave-A-main cost gate **FAILS**: median wall 80.441 -> 95.389s (+18.583%,
limit +15%), store 205312000 -> 218238976 bytes (+6.296%, limit +20%). All six
launch loads were <=9, all complete-work/typed-edge controls matched, and
changed field work executed. The first baseline active-run load spike 14.48
is retained. CPU medians 85.170 -> 99.781s (+17.15%) support investigating
work and allocation costs rather than deleting an inconvenient wall trial.
See [the committed collector receipt](../bench/readings/go-members-cost-2026-10-26.json)
and [the preregistered protocol](../bench/go-members-adoption-protocol.md).

PR #52's earlier +11.62%/+6.30% critic-reproduced pass remains valid historical
evidence for its named base and candidate; it does not satisfy this run's
failed wall gate. This failed run claimed no new grade or adoption. Gin remains unspent
outside the canonical corpus. No issue #17 classification changes.

K29 scope correction: only lang/go.json actively consumes the affected Go
symbols. The Node route reading's fauxtown statement is a historical control,
not one of its 65 active JS/TS subjects. The Go/mixed fauxtown route and L0
fixture gates remain required separately. Full canonical Go, independent
scip-go cli/cobra and final frozen Gin readings follow a green cost gate.

## Optimized Wave B result

Measured: the field-only pass plus empty-reach allocation optimization passes
K17 against Wave B main de6c46e24: median wall 76.599 -> 87.653s (+14.431%),
store 205312000 -> 218238976 bytes (+6.296%). All six launches were <=9;
candidate trial 2's active-run load spike to 14.20 is retained. Complete work
and typed edges match. See the [actual collector receipt](../bench/readings/go-members-cost-2026-10-26-r2.json).

The full seven-package post-merge race suite passes. An independent review
found no issue. At 4,096 isolated fields, the allocation control fell from
3,580,568 to 1,483,416 bytes/op and 8,254 to 4,158 allocations/op. Every one
of the 84,101 centrality rows is semantically identical across the old
candidate and all three optimized trials, including all 21,564 added zero-
caller field rows. No graph node or reference was removed to pass the gate.
The earlier failure and PR #52's distinct historical pass both remain intact.
This cost result does not itself establish a language grade.

## Canonical and independent SCIP measurements

Measured: scip-go 0.2.7, Go 1.27.3 and Universal Ctags 6.2.1 on the full pinned
cli/cli and cobra modules, tests included and no file/package filters, give L4
good: def_mapping .9345, typed_recall .9889. L1-L3 remain excellent. The
separate independent producer executions reproduce every count and numeric
metric. The current [canonical reading](../bench/readings/lang/go.json) and
[independent reading](../bench/readings/go-members-independent-2026-10-26.json)
retain computed grades and median-of-three extraction throughput.

A same-oracle Wave B main control gives def_mapping .5891 and typed_recall
.6714. Call precision/recall (.9254/.9241), file-edge precision/recall
(.9956/.7030) and import resolution (1) are identical before and after the
member pass. Their differences from the historical reading are producer/
toolchain drift, not member-pass regressions. The harness reports 692
unmapped definitions, including 689 field-category terms; this records its
counts without reclassifying issue #17. No interface methods or reference
edges were added. No L4-excellent claim follows from the cost pass.
