# Go field-only ENGINE adoption audit

C-R11 authorizes the C-R2 lever, field symbols without reference edges, subject
to K17/K26 and every affected reading. This branch starts on Wave A main
2b6f341e, with PR #52's reviewed cost proof merged as a declared prerequisite.
The production port is the previously measured implementation; it preserves
call targets and bare plan anchors. Five-package race verification passed,
and independent review found no implementation issue.

The current-main cost gate **FAILS**: median wall 80.441 -> 95.389s (+18.583%,
limit +15%), store 205312000 -> 218238976 bytes (+6.296%, limit +20%). All six
launch loads were <=9, all complete-work/typed-edge controls matched, and
changed field work executed. The first baseline active-run load spike 14.48
is retained. CPU medians 85.170 -> 99.781s (+17.15%) support investigating
work and allocation costs rather than deleting an inconvenient wall trial.
See [the committed collector receipt](../bench/readings/go-members-cost-2026-10-26.json)
and [the preregistered protocol](../bench/go-members-adoption-protocol.md).

PR #52's earlier +11.62%/+6.30% critic-reproduced pass remains valid historical
evidence for its named base and candidate; it does not satisfy this run's
failed wall gate. No new grade or adoption is claimed. Gin remains unspent
outside the canonical corpus. No issue #17 classification changes.

K29 scope correction: only lang/go.json actively consumes the affected Go
symbols. The Node route reading's fauxtown statement is a historical control,
not one of its 65 active JS/TS subjects. The Go/mixed fauxtown route and L0
fixture gates remain required separately. Full canonical Go, independent
scip-go cli/cobra and final frozen Gin readings follow a green cost gate.
