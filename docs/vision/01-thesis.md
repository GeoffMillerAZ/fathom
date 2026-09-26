# 01 — The thesis: what fathom is at its best

**Status:** accepted 2026-08-25 as a record, not as a binding spec — the intent
corpus adopted that day (commit c18ded21) carries its load-bearing parts:
`docs/intent/domain.md` cites §1–2 and `docs/intent/scope.md`'s non-goals cite
§2–3; ADR-044 rejects a transformation engine on its §3.

**Verdict.** Fathom is not a code-search engine, an analyzer, or a reviewer.
It is a **governed knowledge engine for code**: the system that makes
knowledge *about* code behave like code — versioned, cited, tested for
freshness, and enforced at the moments that matter. The two-strata
architecture (deterministic index below, LLM-compiled governed claims
above, forced delivery channels in front) is the right shape; the July-2026
landscape scan confirms nobody else occupies this lane.

## 1. The product is the freshness contract

Every adjacent tool treats knowledge about code as either ephemeral (review
comments, chat answers) or manually curated with no expiry (rules files,
wikis, runbooks). Fathom's claims are different in kind:

- **content-addressed** — `ClaimID = sha256(layer, kind, subject, repro_key)`;
- **cited** — every claim pins the exact source lines by hash (`ClaimLink rel="cites"` + `TargetHash`);
- **scoped** — globs verified at compile time against real indexed files (the scope-trap defense);
- **swept for $0** — every ingest re-hashes citations; drifted claims flip `stale`, unresolvable ones `contradicted`, and they are *served flagged*, never silently wrong;
- **provenance-stamped** — model, prompt version, input digest, compile watermark, cost ledger;
- **delivered by configuration** — injected at edit time (PostToolUse hook), blocked at push time (verify gate) — because your own telemetry falsified the pull model (0–7% tool-call share, then a measured 0% baseline: 0 fathom calls vs 193 greps).

That bundle — *knowledge with a freshness contract, force-delivered* — is
the product. Everything else in the repo is either the floor it stands on
(the deterministic index) or a consumer of it (modules, gates, reports).

## 2. The honest verdict on "up-front AI preprocessing" (your preprocessing question)

Your instinct is **half right, and the half matters**. Split the thesis:

**(a) Compile-time LLM distillation of *code* — not differentiated, and
your own repo proves it.** The `WithEnricher` path (knowledge cards, doc
summaries) is fully built, schema'd, tabled — and has zero production
callers. It was never missed, because modern coding agents *are* query-time
reasoning engines with the code in reach: pre-summarizing code pays LLM
cost at compile time for answers the agent derives more accurately at query
time from ground truth, and the summaries stale the moment code moves.
2026 GraphRAG research lands the same way: deterministic AST-derived graphs
beat LLM-extracted knowledge graphs for code corpora (arXiv 2601.08773).
**Recommendation: tombstone enrichment** (delete or explicitly mark
dormant-by-decision). Keep the index deterministic; that's the defensible
version of the idea, and it's what you already built.

**(b) Compile-time distillation of *binding documents* into governed
claims — genuinely differentiated.** The obligations pipeline is different
in kind from query-time RAG: tiny corpus (3 binding files/repo, not the
codebase — note this supersedes the "~15 slow-churn governing docs" corpus
described in `semantic-layers.md` §6 and `expert-modules.md` §3.1; design
docs were excluded by decision after the v1 fleet compile, see
[08 §6](08-roadmap.md)), digest-gated (recompiles cost pennies, only on change),
compile-time verified (citations must hash-resolve, scopes must match real
files, rules capped at 28 words), staleness-swept free on every ingest.
RAG over CLAUDE.md would inject plausible-but-unverifiable text with no
lifecycle — and the v1 fleet compile already demonstrated what noise does
to trust. Where compile-time loses even in form (b): aspirational or
fast-moving content (the `docs/design/**` lesson), large corpora, and
*questions* as opposed to rules. Don't compile FAQs.

**The winning hybrid is what fathom already is architecturally:**
deterministic index for facts + compiled governed claims for norms + the
agent itself as the query-time reasoning layer. Fathom should never bundle
chat or embeddings-first answering; the harness supplies that better, free.

Honesty requires one more sentence: this thesis is *plausible but unproven*.
Injection coverage is unmeasured, the precision eval has not run in anger,
and the null hypothesis — a well-written CLAUDE.md that Claude Code already
injects, plus grep — has not yet been beaten by measurement. The context
bench and the injection-coverage gate (see [04](04-coding-agents.md)) exist
precisely to defeat it. Run them before repeating the claim externally.

## 3. Where fathom sits in the July-2026 landscape

| Lane | Occupant | Fathom's position |
|---|---|---|
| Code nav / symbol intel | Sourcegraph (SCIP, enterprise-only), Serena (free, LSP, 40+ langs), codebase-memory MCP servers | **Being commoditized.** Fathom's find_symbol/references tier cannot win here and doesn't need to; it's the floor that feeds the gates. |
| Org-scale code search | Meta Glean, Glean.com | The "unwinnable Glean game" — already a standing non-goal. Correct. |
| SAST / findings | Semgrep (5,000+ rules, MCP server in the agent loop), CodeQL | Owned. Fathom's secret_scan/antipatterns family is a strictly worse Semgrep — zero calls in 8 days agrees. Demote/prune. |
| AI code review | Greptile (82% bug-catch, free tier), CodeRabbit | Saturated; "good, not differentiated" verdict confirmed. |
| Migration campaigns | Moderne/OpenRewrite ($30M, 10k recipes) | Trap. No transformation engine, don't build one. |
| Behavioral/temporal analytics, M&A | CodeScene, CAST, Sema | Services-led B2B; not a solo-dev market. |
| **Governed claims + verified completion + forced delivery** | **Nobody** | **Fathom's lane.** No surveyed tool has citation-hashed claims with a freshness sweep, or a gate that checks whether checked boxes are true. |

The adjacent wave worth riding: **spec-driven development**. Spec Kit
supports 30+ agents and its verify phase is human prose — no enforcement.
`fathom verify` is the missing enforcement layer for that entire movement
(see [04 §4](04-coding-agents.md)).

## 4. The most legible pitch

For the 2026 market, the clearest one-sentence framing of what's already
built: **fathom is the truth-maintenance system for agent instructions.**
Every team hand-maintains CLAUDE.md / AGENTS.md sprawl that silently rots.
Fathom's obligations layer is, generically: agent rules that are scoped,
cited, expire when their source drifts, and inject only when relevant.
That reframing sells the shipped mechanism without promising a platform.

## 5. The trust model is the moat — treat it as a strategy item

Why should anyone believe a fathom claim? Because of the machinery: tiers,
hash-resolving citations, honest failure ("cannot verify" exit 2 — never
"all claims false"), served-flagged staleness. Two gaps currently undercut
it, and both should be treated as *strategy* items, not bugs:

1. **O6** — `test_reaches` misses method-call edges, so 12 of 16 gated plan
   claims lean on `skip_test:true`. The flagship gate proves symbol
   *presence*, not test *reachability*, for 75% of its own dogfood plan.
2. **Serve-time precision is unsampled** — the compile eval gates at ≥0.90,
   but nobody measures what actually lands in the edit loop. (Live example
   from the day this review was written: editing a *markdown* file injected
   "always verify with the race detector" — a code-scoped rule with an
   over-broad scope. Noise here is trust-fatal, per ADR-016's own warning.)

The moat is only as deep as these are fixed. See [04](04-coding-agents.md).
