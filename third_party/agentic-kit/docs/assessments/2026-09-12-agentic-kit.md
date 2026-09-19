# Agentic-kit assessment — 2026-09-12

This is the assessment baseline, before the owner-authorized implementation.
Current delivery evidence: [2026-09-13 uplift](2026-09-13-governed-workflows.md).

The premise is sound: recurring agent failures should become contracts, controlled
execution paths and regression tests that workloads can inherit. This repository has
substantial working foundations. Its largest opportunity is to turn those foundations
into easier, demonstrably governed workflows and a repeatable transfer of lessons.

The main risk is the distance between an available safeguard and a safeguarded
consumer. More patterns alone will not close it. The recommended product direction is
defined in [VISION](../../VISION.md), [assurance](../intent/assurance.md), and
[the implementation design](../designs/governed-workflows.md).

## Evidence and limits

Baseline: agentic-kit `3db571b` (2026-09-02), with the pre-existing uncommitted
`Taskfile.yml` fourth-consumer check preserved. Inspection covered all root intent
documents, indexed decision records, the practices ledger and roadmap, selected code
and tests, and four consumers' integration points. Consumer source revisions inspected:
fathom `f67f06a`, throughline `7717f99`, growing-gardens-bi `7503dd1`, resume_as_code
`dd234df`. Source inspection establishes integration evidence, not production uptime.

`task validate` passed on the assessment baseline and after the intent changes: formatting, vet, CUE/schema
fixtures, constitution checks, root race tests and nested-module test commands.
The PDF/XLSX nested modules report no test files. The combined downstream task was
not run because its pre-existing pending change mutates a consumer. No live model calls, deployment,
vendor refresh, consumer-tree changes, or installed hook/config changes were performed.
This is an architectural and product assessment with targeted code review, not an
exhaustive security audit or a measured production-quality benchmark.

## Findings, ranked by consequence

| Priority | Finding and evidence | Consequence / action |
|---|---|---|
| High | Prose assigns every input evidence ID to every generated line; `buildPrompt` supplies IDs, not resolved facts; `has-citations` passes if any node has a ref. [pipeline.go](../../kits/governed-prose-kit/pipeline.go), lines 230–301 | A bibliography can make unsupported wording appear traceable. Make exact claim support the G6 flagship. |
| High | Governance is optional: nil detector disables screening; effect policy defaults to permissive; budget is an optional dependency. [harness config](../../kits/agent-harness/types.go), [effect policy](../../kits/agent-harness/effect.go) | Importing the harness cannot mean all safeguards are active. Add explicit profiles that validate wiring and prove their execution paths (G5). |
| High | `hitl.Engine.Apply` accepts state and op, checks the optional guard, and applies; it does not validate approval or expected state version. [engine.go](../../kits/hitl/engine.go), lines 135–144 | A shared preview/apply function does not by itself prevent stale authorization. Bind actor, operation, state version and expiry; test interruption/retry (G8). |
| High | `task validate` passes despite bare regeneration timestamps and two upward foundation imports. [pipeline.go](../../kits/governed-prose-kit/pipeline.go), lines 178/204; [run-report](../../foundations/review/run-report/render.go); [otel-genai](../../foundations/telemetry/otel-genai/types.go) | Declared determinism and layering exceed enforcement. Record INC-8, repair the code and add appropriate regression checks before claiming these invariants. |
| Medium | `claudecli` and `codexcli` reject caller tools and accept text-only messages. [Claude adapter](../../foundations/llmclient/claudecli/client.go), [Codex adapter](../../foundations/llmclient/codexcli/client.go) | Completion adapters do not integrate governance with a host coding agent. Build and probe native host adapters separately. |
| Medium | Budget USD is an estimate; zero is valid. [Config.EstimatedUSDPerCall](../../kits/agent-harness/types.go), lines 113–122 | Request limits can hold while actual spend differs. Distinguish estimates, per-attempt reservations, actuals, retries and delegated spend. |
| Medium | Throughline bypasses the chain's detection stage to retain structured findings and advisory semantics. [untrusted.go](../../../throughline/app/chat/untrusted.go), lines 48–59 | Solve TRACKING F8 with typed findings and explicit decision policy. A more convenient composition must preserve the consumer's useful semantics. |
| Medium | Resume_as_code reproduces structured-output handling, routes effort through provider parameters, and discovers family through an optional naming seam. [kitclient.go](../../../resume_as_code/cmd/llm/kitclient.go), lines 24–49 | Provider capabilities and identity need a supported contract; reduce glue that consumers must copy (G5). |
| Medium | Lessons were parked behind central memory (former F3→F2), although the owner explicitly wants workloads to learn from each other | Admit versioned lesson/reproducer/remedy transfer independently of memory (0006, G7). |
| Medium | Missing downstream checkouts can report successful skips; the pending `racli-check` rewrites a consumer's module/vendor tree and requires a clean kit tree | Run compatibility in isolated checkouts with revision/coverage receipts. Separate verification from adoption writes. Preserve the existing uncommitted change for review. |

The most important change of direction is G5 plus G6: make the adoption path prove
its controls, and use precisely sourced prose to test the evidence architecture end
to end. That is a more valuable next milestone than adding another broad family of nouns.

## What is already worth protecting

- The root has no external Go dependencies, and dependency-bearing extraction handlers
  are separated into nested modules. Consumers can embed a narrow part of the kit.
- Replay, typed failures, budgets, context compaction, evidence grades and provenance
  provide practical seams for deterministic tests and inspectable results.
- Tool registry, MCP bridge, synchronous effect approval, HITL and scenario examples
  are real implementations; the next work should compose and harden them.
- The project explicitly states non-guarantees around injection detection, redaction,
  confinement and append-only storage. Preserve that honesty at the workflow level.
- `eval-runner`, reliability-stats and the read-manifest model give the learning loop
  useful substrate. They do not need to be replaced with another evaluation framework.
- Intent's small routed documents and explicit revisit conditions are valuable. The
  defects found are primarily stale claims and insufficient enforcement, not a reason
  to abandon the system.

## Consumer lessons

| Consumer | Observed use | Most useful lesson for the kit |
|---|---|---|
| Fathom | imported heartbeat/read coverage and model primitives; fleet indexing and obligations | transferable incidents need precise evidence, coverage limits and a checked failure case; audit results must not improve when reads fail |
| Throughline | guardrails wrapping, direct injection findings, evidence citations | preserve structured findings and domain-specific advisory/blocking policy; factual support can be contested |
| Growing-gardens-bi | document extraction and format handlers in [chat intake](../../../growing-gardens-bi/internal/web/handlers_chat.go) | the next composition should retain page/cell provenance from ingestion through proposed data changes and human review |
| Resume_as_code | vendored client adapter and AI provenance in authored artifacts | constrained fact-to-prose is a strong first pilot; provider glue and vendor integrity are adoption costs worth eliminating |

These are candidate transfers, not instructions to modify those repositories. A lesson
earns shared status when a recipient reruns its independently specified failure case
and adopts the remedy. Deleting copied code is useful evidence of completed extraction.

## Intent audit disposition

| Record / condition | Assessment | Disposition |
|---|---|---|
| P1, D2 retirement criterion, uplift amendment 2 | owner direction explicitly changes the subscriber-only premise | superseded by bounded incubation 0004; D8 nesting remains |
| D3: fourth subscriber arrives | fired: resume_as_code imports the kit | consumer map corrected; isolated verification remains G5 |
| D13: two live consumers by next release | cannot establish for every D13 package from this sample | no tier promotion; evaluate per package at the next value review |
| F2: consumers own memory | fired for the proposed central-store direction | narrow F2 to adapters; 0006 preserves consumer ownership |
| F3: repeated lessons | owner-sponsored requirement now explicit | promoted to G7, independent of F2 |
| F6: eval-runner lands | fired, but offline replay does not establish batch economics | sharpen trigger to measured bulk benefit and latency tolerance |
| F7 / F9 / F12 | ordered failover, MCP client and HTTP trace portions exist | narrow futures to remaining work; remove F9's completed future entry |
| F16.4 / F16.17 | owner-sponsored assurance and approval workflows admitted | move into G5/G8; correct the misleading measured-preview claim |
| I12 / I13 | violated; governing tests absent | explicitly annotate, record INC-8, prioritize G3/M0 repair |
| I14 | no `_reference` import found in the inspected foundation import graph | remains stated; this is a scoped check, not universal proof |
| I15 | invariant/guide allowed a bypass that autonomy prohibited | reconcile wording to autonomy's no-bypass rule; no hook semantics changed |
| I3/I4/I7/I9/I21 | named mechanisms exist but prove narrower properties than their prose suggested | clarify scan, coverage, portability and skipped-check limits |
| I1/I2/I5/I6/I8/I10/I11 | named gate checks/tasks exist and baseline validation passed within their documented scope | retain; scanner waivers and runtime/model limits remain explicit |
| I16/I17/I18/I20/I22 | compatibility, optional composition, plan-only execution and commit policy remain applicable | no API/tier/executor change; retain as policy, not newly proven runtime guarantees |
| I19 / INC-1–5 | named heartbeat, credential, walkthrough, conformance and pin-honesty outcomes still have code/test anchors | retain; this audit does not replay original incidents in consumers |
| 0001 | 30-day deadline is 2026-09-26; multi-repo human annotation count not established | keep proposed; no inferred annotations |
| 0002 | implementation is opt-in Strict with per-call decisions; index falsely named a durable token | correct index; G8 owns durable approval, no existing default changed |
| 0003 | deprecation-floor tests pass; no consumer rejection established here | preserve decision; not a fresh certification of the whole external protocol |
| G1 / F15 / D4 | public/1.0 authorization not established by this task | keep deferred, independent of product uplift |
| G2, Q2 | production import still uses experimental guardrails | keep classification pending conformance and explicit risk/promotion decision |
| G3 | F8/F9/H8-1 remain open; a GGB manifest adaptation already exists in source | prioritize actual friction; do not check off consumer integration without its verification |
| G4 / INC-6 | unreleased block exceeds budget; release not requested | keep release cleanup visible; do not rewrite released history |
| INC-7 | anchored ignore fix exists; proposed new lint not implemented | retain the open enforcement follow-up |
| Q4 | index already specifies promotion on reopening | remove redundant question; 0004 demonstrates the procedure |
| Q5/Q6 | corpus authorization unresolved; enforcement need now evidenced | retain corpus question, reframe Q6 around implementation choice |

For remaining futures: no new runtime subscriber was established for F1/F4/F5/F8/F10/
F11/F13/F14 or the unpromoted F16 subentries. Their domain/consumer predicates are
`cannot tell` from this targeted review, not assumed false. The calendar floor for
0001 has not fired; other undated relative-release/year deadlines cannot be evaluated
reliably without an anchored start, and should gain dates at their next review.
No plan-only worker executor or future memory service was implemented in this change.

## Intent wiring check

`intent-doctor` results after the edits: 19 PASS, 1 FAIL, 1 WARN, 2 SKIP. Structure
checks do not upgrade the runtime findings above into passes.

| Check | Result | Evidence |
|---|---|---|
| Router exists / budget | PASS | 79 lines, limit 80 |
| Living-doc budgets | PASS | 11 root documents within limits; charter 90 / 120 |
| Required documents | PASS | all eight required documents present |
| Decision index | PASS | six local records and six rows; legacy decisions indexed separately |
| Record shape | PASS | all six have Status, Revisit when and at most 80 lines |
| Trigger registry | PASS | 79 condition mentions across intent and records |
| Goals and futures | PASS | eight goals with exit tests; 14 future sections, including F16's shared defaults and delegated entries |
| Compatibility surface | PASS | explicit API, error, schema, module and gate boundaries |
| Sensitive surfaces | PASS | nine rows with nonempty evidence requirements |
| Changelog shape | PASS | Unreleased starts with Release impact: minor |
| Unreleased budget | FAIL | 177 lines versus 60; legacy G4 / INC-6, cleanup retained for the release ritual |
| Autonomy | PASS | branch model and may / ask / never tiers |
| Incidents | PASS | all three open incidents have outcomes and explicit pending work |
| Agent stanzas | PASS | Project intent blocks match ignoring blank lines |
| Claude skill resolution | PASS | eight intent skills resolve |
| Codex skill resolution | PASS | eight intent skills resolve |
| Installed host versions | WARN | Claude 2.1.270 / Codex 0.154.0 differ from last integration-verified 2.1.245 / 0.146.0; router records recheck due |
| Relative links | PASS | 23 assessment/design/intent-related files checked; targets resolve |
| Router paths | PASS | literal paths resolve; wildcards and placeholders excluded |
| Repository charter | PASS | 21 components, fixed evolution vocabulary; no nested intent routers tracked |
| Nested stanzas / autonomy / budgets | SKIP | two checks not applicable: no tracked component intent directories |

Next operational action: repair the demonstrated I12/I13 defects with regression
evidence, then produce the host integration receipts in G5. Use `intent-prune` for the
retained changelog debt at the next release; do not treat this check as fully green.

## Platform guidance

Source pages checked on 2026-09-12. These findings inform the design; no paid
compatibility probe was performed against the installed clients.

- Both coding hosts now document lifecycle/tool hooks. Codex's tool-path exclusions
  and unsupported-output behavior make a blanket enforcement claim unsafe; probe the
  actual installation. The old local text saying Codex has no SessionStart hook was
  removed. [OpenAI hooks](https://learn.chatgpt.com/docs/hooks).
- Claude's blocking behavior depends on event and output/exit semantics; malformed
  responses can become non-blocking errors. A hook adapter must test these cases.
  [Claude Code hooks](https://code.claude.com/docs/en/hooks).
- Native skills already support loading detail on demand, and Codex has command
  prefix rules. Use those surfaces with explicit coverage rather than stuffing every
  practice into always-loaded instructions. [OpenAI skills](https://learn.chatgpt.com/docs/build-skills),
  [OpenAI rules](https://learn.chatgpt.com/docs/agent-configuration/rules).
- OpenAI recommends Responses for new projects while continuing to support Chat
  Completions. The current kit adapter is Chat Completions: preserve it and add the
  modern lane with conformance evidence. [OpenAI migration guide](https://developers.openai.com/api/docs/guides/migrate-to-responses).
- Agent evaluations should check outcomes, use suitable code/model/human graders,
  and measure variation across distinct trials. Repeated replay of a scripted judge
  is not current-model reliability. [Anthropic evaluation guidance](https://www.anthropic.com/engineering/demystifying-evals-for-ai-agents).
- Add workflow complexity when it improves measured outcomes; start from simple
  compositions and good tool contracts. That supports bounded incubation and a small
  workflow menu. [Anthropic agent patterns](https://www.anthropic.com/engineering/building-effective-agents).

The certification-inspired anti-pattern ledger is a useful starting vocabulary. No
claim is made here that passing the kit's gate satisfies every certification topic,
security standard, or domain's engineering requirements.

## Fathom: useful, with concrete friction

Net judgment: useful in this exercise. It found code and cross-repo integration points
quickly enough to change the assessment. I would keep it and improve evidence delivery.
This was not a controlled time/token benchmark; no numeric productivity gain is claimed.

| Operation observed | Value or friction | Improvement to consider |
|---|---|---|
| `list_services`, targeted `references` | found the fourth consumer and precise guardrail/client integration files without a fleet-wide shell sweep | preserve this workflow; add path/include/exclude filtering for deeper reviews |
| `query_symbols("prose")`, `outline`, `read_source` | led directly to the all-references-per-line defect and exact code; line-capped reads are useful | retain clear paths, tiers and range continuation |
| `references` over several services | response was truncated to 28 occurrences, all from the first service, despite requesting 160; narrower service queries found the others | fair per-service allocation, pagination/cursor and explicit explanation of all active caps |
| `references` for prose imports | returned four binary-looking records from throughline's built executable alongside the real source import | exclude binary/generated artifacts consistently, including occurrence search |
| `symbol_context("Pipeline.Generate")` | legacy `callers`/`related_tests` were empty, while `typed_edges.callers` contained six tests; legacy callees included an unrelated recording-client append | prefer one coherent typed view; label heuristic edges separately and derive related tests from it |
| `repo_map` | ranked large test functions and repeated `main.run` entries with no source paths; less useful for product/component orientation | optional package/directory map, entrypoint grouping and paths |
| `pack_context("codexcli")` | returned only a helper, a test and a plan heading (79 tokens), omitting adapter behavior | expose concern matching/fallback and include relevant definitions or request clarification |
| staleness signals | concern queries recommended re-ingest while the summary showed matching HEADs and no re-ingest needed | distinguish concern age, source drift and HEAD drift with one explainable freshness reason |
| obligations | TRACKING rules were useful; most doc calls returned only file/service; later edit hooks warned about staleness | make no-rules, stale and not-configured distinguishable and provide a successful wiring probe |
| startup continuity | identified dirty work and open plans; also reported nine missing hook templates | useful warning; claimed installation should have a measured canary receipt |
| latest field-absorption plan verification | CLI `/rpc` timed out (exit 2); MCP `verify_plan` succeeded: 18 static passes, 7 unknown/no-contract rows, 0 failures | keep the operational failure distinct from failed proof; clarify how documentation-only acceptance is verified |

Broad MCP result printing initially duplicated structured and text payloads and caused
truncation; that was my orchestration error, not Fathom's. Returning only structured
results fixed it. Some scoped shell reads were still appropriate for exact editable
text and complete document checks after the index had located the scope.
Shell reads briefly failed with `Operation not permitted`; using an explicit directory
and non-login shell restored access. The observation does not establish Fathom as the cause.

Static verification is valuable for symbols and wiring, but cannot establish test
pass/fail, source truth, semantic entailment, or quality of an assertion. Keep that
boundary explicit wherever Fathom says a task is proven. The right next measurement
is a small paired task set: same questions with and without Fathom, independently
checked answers, elapsed time, tool/context tokens, false links, incomplete searches
and recovery from stale indexes. This assessment supplies useful cases for that set.
