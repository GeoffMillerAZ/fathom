# Independent decision-lab review

Reviewed the current uncommitted `cmd/decision-lab/**` against baseline `da35dea`, [DL-1..6](../../../designs/decision-lab.md) and decision 0033. This is an offline utility review, not provider or consumer acceptance. No source code, paid calls, consumer state or commits were changed by the reviewer.

## Finding dispositions

**F1 closed: mixed cache provenance.** The capture now names cached answer IDs (`types.go:63-66`); `check` rejects duplicate, foreign or non-answer cache IDs and requires an attempt for fresh outcomes (`contract.go:190-203`). `study` excludes only named hits from fresh samples (`study.go:150-195`). A current CLI probe with two valid answers, one cached, exited 0/complete with one cache hit, one declared attempt, and per-class fresh `Reading.N` of **0 and 1**. This matches the core cache's per-question source contract.

**F2 closed: typed failure and attempt accounting.** `study` reports five distinct cause counts, declared attempts, failed attempts and captures whose usage is unreported (`study.go:34-38, 141-149`). A CLI probe with `no-provider`, `timeout` and `budget-exhausted` exited 0/complete with one count for each cause, two declared and failed attempts, and one usage-unreported capture. Preflight causes with attempts and transport failures without attempts are rejected (`contract.go:141-156`). Counts are declarations from captures, not billing evidence.

**F3 closed: duplicate exposure.** `study` recomputes each manifest case's `RequestKey` and rejects a repeated key before calibration (`study.go:90-99`). I repeated the previous renamed-case probe with identical state and contradictory expected labels. The CLI now exits 2/incomplete with `duplicate_exposure` and the two case IDs. This excludes exact repeated requests; it cannot authenticate empirical independence of distinct requests.

## Additional checks

- A two-question partial response with one known answer and one `unknown` exited 2/`unusable`/`partial`. Its JSON retained both validated choices, recomputed confidences and cache flags in `observedAnswers`, with `noActionAuthority=true` (`contract.go:205-260`).
- A valid 1/0/0 probability distribution passed. Confidence above 1 by `1e-13` and a distribution summing to 1.1 were rejected by the CLI. Probability accumulation follows pack option order, and two identical retrieval-study invocations produced byte-identical JSON (`contract.go:263-300`).
- The complete seven-case retrieval fixture exited 0/complete: five known answers, two unknowns, one false favorable, one abstention against an expected favorable label, and all five expected classes represented. Its semantic labels are `frontier` provenance; the fixture is a diagnostic example, not independent quality evidence.
- `go test -race ./cmd/decision-lab` passed. CUE vet passed for the retrieval pack, manifest and capture array. The root owner runs the full `task validate` gate.

**Remaining findings in this review scope: none.** Fathom's earlier `review_diff` indexed `da35dea` and marked the working-tree files unanalyzed, so these dispositions rely on current source and executable CLI probes rather than its neutral band.
