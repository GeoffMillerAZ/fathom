# Headless prose qualification, 2026-10-24

Status: additive implementation and synthetic offline tests, **not qualified for
realistic prose, production publication or UI entry**. [HP acceptance](../designs/prose-headless-proof.md)
and its [work order](../plans/prose-headless-proof.md) retain all incomplete outcomes.
No PH/HW/G12/G13 completion follows from source code or focused fake tests.

## Implementation and evidence

The API-backed Claude lane contributed immutable session/profile/tree/change
contracts and a Catalog/Assignment/HITL scenario. The owner reviewed the actual
composition, retained the P2 source snapshot privately, added independent safety
counterexamples and repaired the selected synthetic host path. The current
[API contract](../../scenarios/prose-headless/API.md) states its authority and limits.

The document wire now accepts typed ordered assertions/labels and derives claim
IDs and exact UTF-8 byte ranges in Go. Strict decoding refuses missing/null/unknown
fields, duplicate keys, invalid Unicode and model-authored authority/coordinates.
Exposure and independent support checks still apply. Existing strict sourced wire
is unchanged. Complete wire and derived artifacts count toward byte limits,
including JSON overhead. Small-limit refusal has a dedicated boundary test.

Focused owner-run `go test -race -count=1 ./kits/governed-prose-kit
./scenarios/prose-headless` passed. The factual-heading label-laundering negative
already exists in `TestModelCannotSupplyAuthorityOrEraseRequirements`; it reaches
the actual engine and refuses. A read-only review initially missed that case and
was corrected against the test. These are offline plumbing/control observations,
not realistic semantic quality or whole HP acceptance.

Two actual synthetic provider calls passed the [transport canary](../evidence/prose-headless/2026-10-24-transport/README.md).
They establish observed API/usage formats and selected answers only. No document
cohort, support accuracy, cost saving, calibrated confidence or UI proof follows.
The independent six-type corpus and document study runner remain separate work.

## Safety review and repairs

| ID | P2 finding retained | Current selected-path repair and proof |
|---|---|---|
| HP-R01 | Readiness checked before atomic HITL update | Concrete cooperative `MemoryFence` holds source/truth/task/goal/queue checks across the synthetic effect. Spoofed qualification interfaces refuse. Direct queue writers are excluded. `TestFenceNameCannotImpersonateQualifiedImplementation`, `TestFenceExcludesDirectQueueWritersThroughEffect` |
| HP-R02 | Prepared Send omitted fresh external checks | Send re-enters the current fence; revoked truth/source/task scope prevents sender invocation. Recovery retains uncertainty and refuses blind resend. Scenario safety tests exercise the effect path |
| HP-R03 | Actor attribution used a second post-effect update | Actor attribution is carried by the atomic HITL commit receipt. No second actor write can report failure after an unreported successful effect |
| HP-R04 | Host mutation authority/task purpose absent | Trusted host authority gates mutation, exact task purpose binds effect, and caller denylist/map data is copied. `TestSuppliedHostAuthorityDenylistIsOwned` |
| HP-R05 | Unbound `Complete:true` could satisfy goals | Canonical Assignment assessment binds independent required criteria, document and policy epoch; stale evidence refuses inside and outside the fence. `TestAssessmentRejectsStalePolicyOutsideFence`, `TestFenceOwnsAssessmentMethods` |
| HP-R06 | Queue bounds/copy/provenance absent | Bounded metadata/history/overflow, authenticated records, exact document/current subject and owned copies. `TestAnswerApplicationBindsExactDocumentAndCurrentSubject`, `TestAnswerTargetsDoNotAliasPriorVersion`, `TestQueueOwnsForbiddenActors` |

Truth coverage includes generation and wider review manifests, plus recursively
all derived premises regardless of input order. `TestExposedFactsIncludesReviewAndRecursiveDerivations`
retains the reversed-order counterexample. Human answers remain input proposals,
not source truth. External source writes and production host qualification remain
consumer responsibilities.

These were pre-merge code findings, not escaped production incidents. The repairs
qualify only the tested synthetic cooperative host. Independently mutable external
sources, real identity providers/stores/senders, reentrant callbacks and distributed
atomicity are outside that scope. Required unsupported requests must hold.

## Open acceptance

| Scope | Remaining proof or implementation |
|---|---|
| Session control | Typed session budget/deadline/escalation and stopping contracts |
| Statement status | Explicit reusable observed/proposed/assumed/inferred/definition classification beyond scenario guidance |
| Selection/history | Split/merge predecessor mappings and exact stale-selection refusal |
| Variant release | Independent/all-required group publication transaction and mid-batch epoch changes |
| Disclosure | Qualified restricted/tokenized projections and exact unsupported refusals |
| Real workload | Frozen 36-cell corpus, all required negatives, independent review, bounded repair, paired Jev, complete costs/failures |
| Integration | Full local gate and four prepared consumers passed; [receipt](../evidence/prose-headless/2026-10-24-integration/README.md). Whole HP-A17 and strict structural closure remain open with unimplemented acceptance |
| UI | HP-A18 entry remains closed; browser/accessibility/attended tasks require their later evidence |

The reference supports supplied/freeform profiles and exact Catalog resolution;
it does not establish every useful document type or every PH/HW capability.
A model support judge remains heuristic. The owner must independently evaluate
all accepted real outputs. Holding every valid document fails HP usefulness.

## Spend scope

Native Claude Code phases 0..5 report **$10.3240892 session-cumulative**. The latest
phase delta is $1.6444964. The prior P2 snapshot total was $5.6358424; it is not
one invocation's spend. Do not sum restored totals. Token counters and the
[official accounting contract](https://code.claude.com/docs/en/agent-sdk/cost-tracking#accumulate-costs-across-multiple-calls)
confirm cumulative resume accounting in the installed CLI.

The [CLI limit](https://code.claude.com/docs/en/cli-reference) is an estimate and may
overshoot; it is not an invoice or hard billing fence. No further paid coding
invocation is running. The first stage remains $15 coding plus $10 direct calls.
The two direct canaries were conservatively usage-priced to $0.000029, not an
invoice. Failed/unknown calls retain reservations. No account/global policy,
auth settings, installed hooks, private disclosure or real publication changed.

## First document attempt and retained hold

The first frozen positive cell made one real generation call: HTTP200, normal
completion and five structured assertions. It held before review because the
ordinary Anthropic adapter preserves reasoning as unknown raw content and the
strict prose engine refuses non-text blocks. Usage2034 input/1162 output includes
839 thinking tokens, conservatively priced to $0.000785. No candidate or independent
acceptance was reported. The failed cell remains in the cohort/stage denominator.

A command-local canonical-text projection, with its own protocol binding, removes
only validated provider thinking metadata after full private capture. It retains
all other safety holds and includes a real-engine offline failure/pass regression.
Omitted display has an empty thinking string with an opaque signature; the test
covers that observed form. Review now uses the selected lane's bounded output cap.
Disabling thinking globally was rejected because model families have incompatible
controls. No provider-neutral core or shared adapter behavior changed.

Two properly tagged deep critic spawns were refused by the cost hook as workers.
O16 records the routing defect. No hook or repeated-denial release was used. The
existing reviewer, Fathom locator, owner source trace, official provider contract
and offline reproducer supply the stated evidence; no deep-review execution is claimed.

## Frozen range failure and prospective policy

The bounded repair completed two real generations and eight configured support
reviews. All eight required facts were cited, but86 words missed the100-word
minimum. The cell exhausted its one repair and remains failed;35 positive cells
and43 negatives are unexecuted. Goals remain unassessed, synthesis/audience proof
is limited, and cost per independent acceptance is undefined. Direct-stage
observations total$0.052716 including earlier canaries, not an invoice.
[0046](../intent/decisions/0046-separate-document-sufficiency-from-length-targets.md)
specifies prospective required/advisory length without rewriting that failure.
HP-10 remains unimplemented. The long-artifact test and independent sufficiency
review stay mandatory in the next study. UI entry remains closed.
