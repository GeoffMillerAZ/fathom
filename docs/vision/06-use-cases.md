# 06 — The use-case portfolio: focus, options, traps

**Verdict.** Pour everything into the obligations + verify loop for AI
coding agents, with **ADR-derived architecture rules as the layer-2
candidate** and the spec-driven-development wave as the distribution
channel. Keep vendor-intake, compliance evidence, and self-verifying docs
as cheap options on the same substrate. Treat onboarding, migration,
test-impact-as-product, bus-factor, and security posture as traps —
funded incumbents own each, and fathom's versions are unwired or
toy-grade.

## 1. The portfolio

| Use case | Verdict | Why | Shape if pursued |
|---|---|---|---|
| **Evidence-backed verification (anti-slop gate)** | **FOCUS — the headline** | Only tool anywhere that checks whether checked boxes are true and blocks the push. SDD wave (Spec Kit, 30+ agents) has no enforcement layer — distribution someone else built. Precondition: O6. | Already shipped; position as "the enforcement layer for SDD" |
| **Agentic coding companion (obligations injection)** | **FOCUS — the proving ground** | Forced channels work (~95% of traffic); precision work remains | [04](04-coding-agents.md) |
| **ADR/architecture-drift enforcement** | **FOCUS — recommended layer 2** | Whitespace between Semgrep (syntax) and CodeScene (behavior): *architectural rules as governed claims*. "Your ADR says connectors never import query — and you're editing a connector," in the agent loop. Nobody does this. | Layer 2 pack: corpus = ADRs (17 exist, fleet-wide); `arch_conformance` verdicts persist as claim events |
| **Governed agent-rules management** | **FOCUS — the pitch** | The unnamed use case: every team's CLAUDE.md/AGENTS.md sprawl silently rots; fathom is the truth-maintenance system for it. Already built — this is positioning, not code. | [01 §4](01-thesis.md) |
| **Vendor/OSS intake review** | Option — document as a recipe | Ingest a candidate dep, compile its docs' claims, verify README/security claims against actual code, emit a portable evidence store. Nobody does claim-vs-code verification on third-party repos as a product. | Expert-module recipe; zero new code beyond findings layer |
| **SOC2/ISO code-level evidence** | Option — bounded probe only (H2, as planned) | Vanta/Drata stop at the control plane; citation-hashed, swept code evidence is real whitespace — but an enterprise trust sale. One design partner, no more. The probe is what justifies wiring `claim_events`. | Compliance pack + `fathom render` evidence report |
| **Self-verifying documentation** | Absorb, don't productize | The sweep already does it: any doc statement compiled into a cited claim flags when its source drifts. Surface as `fathom claims --stale`. | Free strengthening of the core story |
| **Incident precedent memory** | Defer (as already gated) | Entry-conditioned on ≥15–20 resolved findings; you have ~5. AI SREs (Traversal et al.) own the telemetry side; fathom's slice is only precedent. | Failure-mode pack, when the corpus exists |
| **M&A due-diligence packs** | Do nothing | The portable store is a genuinely better *deliverable format*, but the market is services-led (Sema). If it happens, someone else builds it on the store format — which is what "platform is an event" means. | — |
| **Onboarding / learning platform** | **TRAP** | Every AI assistant explains codebases; chat-on-top competes with your own distribution channel. Onboarding is a byproduct of a good index. | — |
| **Migration campaigns** | **TRAP** | Moderne's whole company; fathom has no transformation engine and shouldn't build one. | — |
| **Test-impact CI as product** | **TRAP** (keep as internal feature) | Needs coverage-grade maps; even Go call edges are incomplete (O6). `test_impact` serves the verify gate, nothing more. | — |
| **Bus-factor / knowledge succession** | **TRAP** | People+history data is CodeScene's moat; `ownership` is unwired for good reason. | — |
| **Security posture mapping** | **TRAP** | Semgrep with 5,000 rules already sits in the agent loop; the call log shows zero demand for fathom's family. Demote in the prune. | — |
| **LLM eval grounding** | Internal instrument | The private context-bench is the right shape; publishing results is marketing, selling grounding is not a business. | — |

## 2. The layer-2 decision (decision required)

The current H2 plan names **failure-mode playbook** as layer 2. This review
recommends **ADR/architecture-rules** instead, for one decisive reason:
**the corpus already exists** (17 ADRs here, more fleet-wide) while the
incident corpus (~5 findings) is below its own entry condition (≥15–20).
Layer 2's *job* is to prove the generic manifest runtime with a second real
layer ([02 §3 M2](02-shape.md)); a layer whose corpus you'd have to
manufacture first proves nothing on schedule.

Nuance to be honest about: ADR.md is already *in* the obligations corpus,
so its prose rules already compile. What the arch layer adds beyond
obligations: (a) **edge-based rules** ("connector must not import query")
checked against the call/import graph via `arch_conformance`, i.e. claims
whose verification is *deterministic*, tier `verified`, not `doc-claimed`;
(b) conformance verdicts persisted as claim events over time; (c) per-repo
architecture manifests as pack config rather than per-call MCP args. If
that scope feels too code-heavy for the second layer, the fallback is
`change-review` — but pick using the same test: corpus exists, consumer
wired, kill gate definable.

## 3. Landscape summary (July-2026 web survey; sources below)

- **Sourcegraph**: retreated to enterprise; SCIP symbol intel via MCP; Amp
  spun out. The individual-dev context lane was contested enough that even
  they exited it.
- **Meta Glean / Glean.com**: org-scale indexing-as-service — the
  "unwinnable game" the vision already avoids.
- **Semgrep/CodeQL**: SAST owned, MCP-integrated, with the findings
  lifecycle (dedup/triage/baselines) fathom's analysis tools lack.
- **Moderne/OpenRewrite**: migration transformation engines at scale.
- **Greptile/CodeRabbit/Unblocked**: AI review saturated; repo-graph
  context is table stakes now.
- **Serena + free LSP/tree-sitter MCP servers**: the sharpest fact —
  fathom's code-nav tier is being commoditized in real time by free tools
  with broader language coverage. Fathom cannot win as a code-nav MCP
  server; none of these have claims, citations, sweeps, gates, or forced
  delivery.
- **GraphRAG research**: deterministic AST graphs beat LLM-extracted
  graphs for code corpora — direct validation of fathom's determinism
  split.
- **Spec Kit / SDD**: the wave to ride; verify phase is prose, no
  enforcement.
- **Vanta/Drata**: control-plane evidence only; code-level evidence with
  provenance is open.
- **Kythe/stack-graphs**: cautionary — build-integrated precise indexing
  dies without a business attached; hash-gated best-effort is the survivor
  design.

Sources (survey claims, not measurements — figures like "82% bug-catch"
and "5,000+ rules" are the vendors'/reviewers' own):
[Sourcegraph agentic coding](https://sourcegraph.com/blog/agentic-coding) ·
[Cody 2026 review](https://www.buildfastwithai.com/ai-tools/sourcegraph-cody) ·
[Meta Glean](https://engineering.fb.com/2024/12/19/developer-tools/glean-open-source-code-indexing/) ·
[Glean.com code search](https://www.glean.com/blog/code-search-code-writer-jan-drop-2026) ·
[Semgrep MCP](https://github.com/semgrep/mcp) ·
[Semgrep vs CodeQL](https://konvu.com/compare/semgrep-vs-codeql) ·
[Moderne/OpenRewrite](https://moderne.ai/blog/overview-of-openrewrite-and-moderne) ·
[CodeScene](https://codescene.com/product/behavioral-code-analysis) ·
[Sema](https://www.semasoftware.com/blog/the-rise-of-comprehensive-codebase-scans-transforming-technical-due-diligence) ·
[Greptile vs CodeRabbit](https://www.greptile.com/greptile-vs-coderabbit) ·
[Unblocked](https://getunblocked.com/blog/best-ai-code-review-tools/) ·
[Serena](https://github.com/oraios/serena) ·
[codebase-memory-mcp](https://github.com/DeusData/codebase-memory-mcp) ·
[AST vs LLM graph-RAG, arXiv 2601.08773](https://arxiv.org/abs/2601.08773) ·
[GitHub Spec Kit](https://github.com/github/spec-kit) ·
[SDD 2026 guide](https://thebcms.com/blog/spec-driven-development) ·
[Vanta/Drata comparison](https://truvocyber.com/blog/soc-2-audit-guide-drata-vanta) ·
[AI SRE tools](https://www.anyshift.io/blog/top-10-ai-sre-tools-2026-comparison) ·
[Kythe](https://github.com/kythe/kythe) ·
[aider stack-graphs issue](https://github.com/paul-gauthier/aider/issues/534)

## 4. How this stays focused (your bloat question)

The portfolio above is one substrate and N configurations. The engine never
grows a use case; a use case earns a *pack*, every pack carries a KILL.md,
and the admission rule ([02 §5](02-shape.md)) plus the rung ladder
([03 §5](03-findings-and-experts.md)) govern promotion. That — not a
plugin API, not satellites — is how a general tool serves many domains
without bloating: the domains live in data.
