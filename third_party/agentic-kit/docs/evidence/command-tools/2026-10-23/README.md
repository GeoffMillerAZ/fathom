# Governed command tools: first controlled inspection

Scope: GT1..3 under [0039](../../../intent/decisions/0039-governed-command-tools-preserve-evidence-and-authority.md).
This is implementation/qualification of one selected-file CLI, not completion of
the [full delivery](../../../plans/governed-command-tools.md), installed interception,
organizational authorization, absence of secrets/PII or measured production benefit.

## Instruments and scope

| Instrument | Observed result | Limit |
|---|---|---|
| `task validate` | Full gate passed, including root/nested race tests, vet, CUE and constitution checks | First run correctly failed stale generated catalog/README count; regeneration and count correction fixed both without gate changes |
| Focused `go test -race -count=1 -cover ./cmd/command-guard` | Pass; 83.2% statements | Test coverage is not completeness of threat coverage |
| Independent security review | Reproduced original symlink swap disclosure; verified held-handle identity fix with zero disclosures in 500 repetitions of the same probe | One bounded adversarial schedule, not proof against all aliases/filesystems |
| Independent skill/guide trial | Actual binary read/search 0; late finding 1; missing required layer/rule, malformed policy and later-file failure 2; no content rows on input/scan failure | Synthetic local inputs; no automatic host skill-selection claim |
| [Retained entrypoint trial](entrypoint-trial.json) | Five final-binary cases, exits 0/0/1/2/2, exact returned reports and binary hash | Native macOS arm64, Go 1.26.3 only |
| Windows amd64 / Linux amd64 `go build` | Both pass | Compilation only; GT4's native host proof remains open |
| Strict isolated-store plan verification | Three proposed claims verified before checking boxes; actual plan rechecked afterward | Static symbol/test reachability, not runtime correctness; runtime gate is separate |
| Skill metadata/link checks and forward trial | Project skill points to actual tool and limits; forward trial passed | Bundled `quick_validate.py` could not run because its Python lacks PyYAML; no dependency installed |

The shared daemon's identity probe timed out, and current MCP symbol lookup did not
resolve the uncommitted command. The edit-time hook therefore reported GT1..3 unknown
against that stale index. The isolated store was freshly ingested outside the shared
daemon store and verified those exact command-local anchors. Normal pre-push performs
its own fresh scratch ingest. This does not resolve existing observation O13 or prove
shared daemon health. No index-store, daemon, hook or configuration repair was attempted.

## Specific corrections and residual limits

Independent review found that Lstat followed by a multi-component open could follow
a replacement symlink into a denied directory. The corrected path opens one directory
component at a time and compares each observed identity to the opened child root,
then checks the final opened file before reading. Committed stress and FIFO cases
supplement the separate review probe. Hardlinks, mounts, moves of the same inode and
hostile filesystems remain outside a path-name sensitivity guarantee.

Aggregate reads now stop at the tighter remaining budget plus one probe byte.
`bytesRead` and actually evaluated `bytesScanned` are separate. Output construction
bounds the candidate prefix before logarithmic encoding checks. ASCII case-folded
prefix/duplicate checks, Windows alias rejection, single-line search literals and
strict forbidden-field presence avoid silent request reinterpretation. CUE uses byte
limits for rule/literal strings as well as structural constraints.

All selected input checks complete before the first content write. Sink failure can
leave an already-checked partial prefix at the destination; short/error writes return
2 without retry. This is not an atomic-delivery promise. Failure reports contain fixed
categories/counters, never paths, query text, source hashes, rule errors or excerpts.

No five-root library/exported API changed, so the downstream consumer task was not
required for this command-local increment. No actual scanner/model/service integration,
hook installation, machine configuration or upstream app-kit source change occurred.
The sample regex is explicitly a synthetic marker, not a real secrets/PII ruleset.

## Reproduction and retention

Use the [command guide](../../../../cmd/command-guard/README.md) for complete synthetic
read/search commands and help. [Source pins](source-pins.json) bind the command and
fixtures used for these checks. Gate/plan/entrypoint logs are retained privately under
`/Volumes/m2ext/backups/agentic-kit/design-reviews/2026-10-23-governed-command-tools/`.
The normal push receipt is retained after delivery; the absence of a receipt is not a pass.

The next executable delivery is GT5's pinned scanner adapter and failure coverage,
followed by GT6's hostile-repository Git recipe. GT8..11 must prove authenticated
operator policy and native hook maintenance before any installation claim. All
GT4..17 remain required; this first increment does not close the active full goal.
