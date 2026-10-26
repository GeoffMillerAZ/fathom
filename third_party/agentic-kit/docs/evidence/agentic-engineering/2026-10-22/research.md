# Agentic-kit vision uplift: primary-source research

Retrieved 2026-10-22. Sources are official publisher docs, standards, maintainer repos, or the project's own research papers. This is design input, not proof of adoption or measured benefit in agentic-kit. Vendor reports of their own experiments are labeled as such. URLs point to sources actually located and reviewed. No course or exam items reproduced.

## Decisions the evidence suggests

1. Separate workflow shape from autonomous agent loops. Use explicit steps when paths are known; add model-directed tools only where uncertainty justifies latency, cost, and risk. Evidence: S02, S03.
2. Treat provenance as a first-class relation among source artifacts, transformation/observation activity, responsible actor, version/time, and derived claims. Preserve ambiguity and contradiction instead of forcing one truth. Evidence: S04, S12, S17, S20.
3. Treat pause, approval, resume, and side effects as independent contracts. Durable state does not imply exactly-once effects; both ADK and LangGraph document replay. Evidence: S11, S13, S16, S17.
4. Put deterministic policy/capability checks at the tool/action boundary. A model judge can prioritize or explain disputed cases, but cannot confer authority. Evidence: S09, S10, S14, S15.
5. Cheap-first is a measured per-task routing policy, not a universal model hierarchy: static checks/retrieval first, small model for narrow extraction/classification, escalate on unresolved ambiguity, high stakes, or model disagreement. Batch asynchronous review; cache stable prefixes; meter cost and quality per case. Evidence: S03, S05, S06, S23, S24.
6. Make eval artifacts inspectable: task + source truth, trial/trajectory, scorer/scan outcomes, harness version, failures, and human adjudication. Keep capability and regression suites separate; audit grader bugs and ambiguity. Evidence: S04, S18, S19, S20.

## Annotated sources

S01. Anthropic, Claude Partner Network announcement. https://www.anthropic.com/news/claude-partner-network (accessed 2026-10-22). Supports existence of Claude Certified Architect, Foundations, initially for partners, as Anthropic's first technical certification. It does not validate an unrelated architecture or a holder's real-world competence.

S02. Anthropic Academy, Claude Certified Architect - Foundations public listing. https://anthropic-partners.skilljar.com/claude-certified-architect-foundations-certification (search-index result accessed 2026-10-22; direct fetch returned 403). Search index lists domain coverage: agentic architecture and orchestration, tool design/MCP, Claude Code workflows, prompt/structured output, and a fifth domain not visible in the excerpt. This is a certification blueprint, not an authoritative general-purpose framework; no exam questions or proprietary guide quoted. Treat page details as lower confidence until direct access.

S03. Anthropic, Building effective agents. https://www.anthropic.com/engineering/building-effective-agents (accessed 2026-10-22). Distinguishes predefined workflows from autonomous agents; presents prompt chaining, routing, parallelization, orchestrator-workers and evaluator-optimizer, with selection conditions. Prescribes starting simple and increasing complexity only when needed. It is design guidance, not a universal benchmark.

S04. Anthropic, Demystifying evals for AI agents. https://www.anthropic.com/research/demystifying-evals-for-ai-agents (accessed 2026-10-22). Defines task, trial, grader, trajectory; contrasts deterministic, model, and human graders; recommends human calibration and an Unknown escape; documents grader/harness/ambiguous-task failures and distinction between capability and regression evals. Anthropic's cited score shifts are case-specific self-reports, not portable effect sizes.

S05. Anthropic, Effective context engineering for AI agents. https://www.anthropic.com/engineering/effective-context-engineering-for-ai-agents (accessed 2026-10-22). Context is finite; curate small high-signal context, use runtime retrieval and reference IDs, compaction, structured notes, and isolation of detailed subagent search. These are prescriptions and observations, not an external controlled trial.

S06. Anthropic, How we built our multi-agent research system. https://www.anthropic.com/engineering/multi-agent-research-system (accessed 2026-10-22). Describes orchestrator-worker search, query-dependent breadth, memory persistence, and tool testing. Supports bounded research fanout and synthesis; does not show that multi-agent wins on every task or at equal cost.

S07. Anthropic, Effective harnesses for long-running agents. https://www.anthropic.com/research/effective-harnesses-for-long-running-agents (accessed 2026-10-22). Initializer, explicit feature list, one feature/session, progress artifacts, Git checkpoints, end-to-end browser testing. Reports failure modes of one-shot and premature-complete behavior. Scope: internal web-app experiments; not proof for other domains.

S08. Anthropic, Harness design for long-running application development. https://www.anthropic.com/engineering/harness-design-long-running-apps (accessed 2026-10-22). Planner/generator/evaluator and explicit handoffs across fresh contexts; observes self-evaluation leniency, including evaluator limits. Scope is Anthropic's experiments, and extra orchestration increases cost/latency.

S09. Anthropic, How we contain Claude across products. https://www.anthropic.com/research/how-we-contain-claude (accessed 2026-10-22). Separates model defenses, environment containment, and tool-capability boundaries; injection and runtime attacks both matter. Supports least-authority tools and independent enforcement. No claim of perfect containment.

S10. Anthropic, Trustworthy agents in practice. https://www.anthropic.com/research/trustworthy-agents (accessed 2026-10-22). Prompt injection cannot be solved by one layer; more tools increase possible attacker impact. Design implication is to treat retrieved content/tool output as data, with bounded authority.

S11. Google ADK, Action confirmations. https://adk.dev/tools-custom/confirmation/ (accessed 2026-10-22). Boolean or conditional confirmation on selected tool calls, including structured human response; TypeScript example needs manual logic while Python/Go/Java have wrapper support. Confirmation is a specific action boundary, not a substitute for authorization or idempotency.

S12. Google ADK, Session state. https://adk.dev/sessions/state/ (accessed 2026-10-22). Session-scoped, user-scoped, application-scoped, and temporary state differ; state changes should flow through tracked context/event mechanisms. Supports explicit lifetime/version decisions for records.

S13. Google ADK, Resume agents. https://adk.dev/runtime/resume/ (accessed 2026-10-22). Can resume stopped workflow; tool executions are at least once and may run more than once; docs explicitly require duplicate prevention for effects like purchases. Workflow mutation while stopped is unsupported. This is a concrete counterexample to 'checkpoint means exactly once.'

S14. Google ADK, Types of callbacks. https://adk.dev/callbacks/types-of-callbacks/ (accessed 2026-10-22). Before-tool and other hooks expose interception points for policy checks and observability. A callback itself is extension plumbing; correct policy enforcement depends on the app's implementation.

S15. A2A Protocol v1.0.1 specification. https://a2a-protocol.org/v1.0.1/specification/ (accessed 2026-10-22). Agent Card declares skills, input/output modes, authentication, transport, optional capabilities; clients should validate capabilities; async task/poll/stream/push and multi-turn task states exist. Agent Card is a claim by a service, not independent proof of trust or ability. Beware protocol-version drift.

S16. LangGraph, Persistence. https://docs.langchain.com/oss/python/langgraph/persistence (accessed 2026-10-22). Checkpointer is thread-scoped graph state; store is cross-thread application data. InMemorySaver loses checkpoints on restart. Supports separating run cursor from durable evidence/knowledge storage.

S17. LangGraph, Interrupts. https://docs.langchain.com/oss/python/langgraph/interrupts (accessed 2026-10-22). Interrupt needs checkpointer and thread ID; resumed node restarts at its beginning, so pre-interrupt effects repeat. Place side effects after pause or make them idempotent. Counterexample to line-level continuation claims.

S18. UK AI Security Institute, Inspect Scoring and Scanners. https://inspect.aisi.org.uk/scoring.html ; https://inspect.aisi.org.uk/scanners.html (accessed 2026-10-22). Scorer grades task success per sample; scanner surfaces sparse issues in transcripts (e.g. refusals, environment/eval awareness), which are different questions. Model grading is supported, but its verdict is not objective ground truth.

S19. UK AI Security Institute, Inspect Eval Logs. https://inspect.aisi.org.uk/eval-logs.html (accessed 2026-10-22). Structured logs contain evaluation, sample, and post-eval metadata updates with provenance tracking. Supports preserving raw trial evidence and later adjudications instead of overwriting a score.

S20. SWE-agent maintainers, Trajectories. https://github.com/SWE-agent/SWE-agent/blob/main/docs/usage/trajectories.md (accessed 2026-10-22). Records thought/action/observation, model query, environment state, config and logs per issue. Useful as an open-project inspectability pattern; exact field shape is versioned, and trajectories alone are not correctness proof. Its issue tracker also shows active defects and unmerged proposals; see https://github.com/SWE-agent/SWE-agent/issues/1539 and https://github.com/SWE-agent/SWE-agent/issues/1526 (accessed 2026-10-22): these are reports/proposals, not fixed behavior.

S21. OpenHands, An Open Platform for AI Software Developers as Generalist Agents. https://arxiv.org/abs/2407.16741 (accessed 2026-10-22). Project paper describes event stream of actions/observations, sandboxed runtime, and benchmark support. This supports separating agent decision, execution substrate, and reviewable events; performance claims are benchmark- and version-specific.

S22. W3C PROV-O recommendation. https://www.w3.org/TR/prov-o/ (accessed 2026-10-22). Provides stable vocabulary for Entity, Activity, Agent and used/generated/derived/attributed relations. It is a provenance representation, not a fact-quality or truth adjudicator.

S23. Anthropic Claude Platform, Batch processing. https://platform.claude.com/docs/en/build-with-claude/batch-processing (accessed 2026-10-22). Asynchronous batch requests include per-request custom ID; docs state 50% standard API token rates. Current prices and availability can change. Batch only where latency is acceptable; record failed/expired requests for retry.

S24. Anthropic Claude Platform, Prompt caching. https://platform.claude.com/docs/en/build-with-claude/prompt-caching (accessed 2026-10-22). Cache stable prompt prefixes and inspect cache creation/read token counts. Benefit depends on repeated identical prefixes and cache lifetime; do not assume savings for one-off reviews or rapidly changing evidence.

S25. GitHub Docs, Using GitHub Copilot cloud agent to improve a project. https://docs.github.com/en/copilot/tutorials/cloud-agent/improve-a-project (accessed 2026-10-22). Shows a concrete issue -> delegated agent -> pull request -> human review/merge -> next issue loop. This is an official workflow example, not evidence that every generated PR is correct or that automatic backlog priority is safe. Contrast SWE-agent issue #1524, an open proposal for claim leases and independent exact-head review; do not treat it as shipped functionality.

## Additional official sources reviewed by the main thread

S26. OpenAI, [Decisions](https://developers.openai.com/api/docs/guides/decisions), accessed 2026-10-22. Typed predicates, choices and rubric scores, first-class refusals and shared-input questions fit the existing port. The guide directs applications to choose thresholds from labeled examples and error costs; it does not establish a universal calibrated threshold. Provider-specific fields need adapter conformance, not copy-pasted call sites.

S27. OpenAI, [Prompt caching](https://developers.openai.com/api/docs/guides/prompt-caching), accessed 2026-10-22. Reuse depends on matching reusable input; measure actual reported cache units and total cost. Provider cache behavior is not application evidence freshness, authorization or judgment reuse. This program asserts no fixed price or savings.

S28. OpenAI, [Safety in building agents](https://developers.openai.com/api/docs/guides/agent-builder-safety), accessed 2026-10-22. Describes structured data flow, approvals, guardrails and trace evaluation as layered mitigations. Its legacy Agent Builder recommendations are product-specific, not a universal requirement to prompt for every read. This design uses explicit policy authority and controlled effect checks.

S29. OpenAI, [Custom instructions with AGENTS.md](https://learn.chatgpt.com/docs/agent-configuration/agents-md), accessed 2026-10-22 through the official Codex redirect. Supports shared source instructions and host-specific resolution. Correct installation and actual invocation require versioned probes; this research does not establish native parity with Claude or Antigravity.

S30. OWASP, [LLM06:2025 Excessive Agency](https://genai.owasp.org/llmrisk/llm062025-excessive-agency/), accessed 2026-10-22. Identifies excessive functionality, permission and autonomy as distinct risk drivers. This motivates resource/effect checks independent of screens; the taxonomy does not certify our controls. Go's [vulnerability management documentation](https://go.dev/doc/security/vuln/), accessed the same day, establishes govulncheck as a specialized external tool, not proof of comprehensive security.

S31. NIST, [AI Risk Management Framework](https://www.nist.gov/itl/ai-risk-management-framework), accessed 2026-10-22. Provides a voluntary risk-management context and identifies the generative-AI profile. This is source material for applicable criteria, not a regulatory-compliance assertion or a substitute for technical failure proof.

The Claude Certified Architect Foundations credential is confirmed by S01. A
separate professional credential's public blueprint was not established by this
research. Do not invent its syllabus or treat certification/popularity as proof
that a pattern improves our workloads. Refresh vendor/protocol behavior when an
adapter is implemented; mutable documentation is dated design input, not a pin.

Firecrawl's configured credential returned Unauthorized; research used the web
tool instead. No exam content, paid model trials or private project data were used.

## Limits and counterexamples

- No source proves a universal rubric, universal confidence calibration, or that one model grade can authorize external action. Define task- and domain-specific criteria, with abstention and adjudication.
- Agent Cards/certifications are claims about capability or training, not executable proof of a particular agent's permission, identity, reliability, or measured benefit.
- Durable checkpointing does not guarantee side effects execute once. At-least-once and replay must be in the kit's interface contract, including idempotency keys and reconciliation.
- Prior issue/trajectory data are candidate eval material; provenance, representativeness, ambiguous task wording, leakage, and grader correctness need audit.
- Vendor-reported improvements are observations under the vendor's harness, not independent causal evidence for agentic-kit.
