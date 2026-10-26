# Decision-site delivery: independent final review

Scope: uncommitted `feat/decision-sites-20261023` changes against `da35deae528f`, including the offline CLI, CUE contract, fixtures, guide, survey, skill, and 0033. Read-only review except this report. No paid call or consumer mutation.

## Finding and resolution

**F1 — Medium, resolved in this change: CUE accepted inputs the CLI rejected.** The first review reproduced six invalid fixture mutations that CUE accepted: missing `unknown`, duplicate option name, `unknown` as favorable, whitespace-only captured state, and captures with both a cause and response or neither. Go rejected them at [`contract.go:44`](../../../../cmd/decision-lab/contract.go) and its adjacent validators. The amended [`schema.cue:7`](../../../../cmd/decision-lab/schema.cue) now rejects all six mutations. I independently reran `cue vet` on each and on all five valid typed causes (`no-provider`, `not-authorized`, `budget-exhausted`, `timeout`, `transport-error`); every result matched the expected validity. The new [`schema_test.go:13`](../../../../cmd/decision-lab/schema_test.go) pairs CUE and Go boundary checks. I found no regression in the patched cause alternatives or nonblank-text rule. The CLI was fail-closed throughout; F1 was an authoring-contract mismatch, not an execution bypass.

## Review disposition

The implemented checker binds captured case, pack, input, request, provider and resolved model; requires one exclusive outcome per question; rejects foreign IDs and malformed probabilities; and keeps unknown/refusal/invalid outcomes out of usable success. Study coverage is anchored to the independent manifest, duplicate request exposures are rejected, exact answer reuse is excluded per question from fresh reading samples, and JSON output omits captured source and provider reason text by default. No further concrete defect found in these paths.

The review-triage and retrieval fixtures demonstrate synthetic diagnostics, including false favorable and unknown cases. Their frontier-authored labels and declared attempts are not live calibration or a billing ledger. The clarification pack is a clarity classification example, while DS-05 question-family routing remains a proposed consumer stage. These are stated scope limits, not defects.

Fathom `review_diff` was partial because its index predates the uncommitted files; its stale/unknown obligations are not code findings. I inspected the current source and unified diff. [Official OpenAI documentation](https://developers.openai.com/api/docs/guides/decisions) supports the dated GA, 400-question, JSON input and prompt-cache claims in the survey. The root agent reported an earlier `task validate` exit 0. Owner follow-up: the post-fix full gate also passed, exit 0; see the [delivery receipt](README.md). The reviewer did not rerun the full gate or independent guide trial.
