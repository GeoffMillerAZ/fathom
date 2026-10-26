# Agent stack tools assessment, 2026-10-19

Durable record of the tools, plugins and practices assessed for long-running orchestrator
sessions that delegate to many subagents, with fathom as the code-intelligence and
evidence engine. Converted from the working report (Claude Code 2.1.304, Codex CLI
0.163.0). Guidance distilled from it: [tokenomics.md](../../../tokenomics.md) section 6.
Method and measurements: [README.md](README.md).
Addendum: [2026-10-22 memory and assistive tools](../2026-10-22/memory-and-assistive-addendum.md).

Scores are 0 to 5 dots from the report. Impact: value to this setup. Risk: cost,
conflicts, privacy or upkeep (more is riskier). Proven: independent evidence.
Reception: adoption and sentiment. They are judgments from evidence gathered on
2026-10-19: GitHub statistics, independent benchmarks where they exist, and live tests on
this machine. Star counts measure attention, not quality.

| Verdict | Count | Meaning |
|---|---|---|
| Adopt | 9 | Built and installed today, or low-risk and read-only with strong independent evidence |
| Trial | 5 | Promising but pre-1.0, single-maintainer, or overlapping something already run |
| Pass | 10 | Vendor-only evidence, overlap with fathom or native memory, or a hook that collides with this pack. Each states what would change the answer |

## Adopt

### agent-hooks

One binary for both hosts: every spawn names its model (and Codex effort), tier ceilings by worker or critic, fork marker, main-thread output containment, and a local token ledger with doctor and report. (Built today; agentic-kit · cmd/agent-hooks)

- Scores: Impact 5/5, Risk 2/5, Proven 3/5, Reception 0/5
- Fits: Subagents carried 91% of your Claude Code tokens and Opus-class subagents nearly half the output; Haiku was used once in 463 spawns. This targets exactly that.
- Fathom: Complements. Different events from fathom's hooks, no output rewriting fathom relies on, and its ledger is the measurement fathom's thread notes lack.
- Evidence: 122 test cases with race detection and fuzzing, 91% coverage, 8 of 8 live smoke scenarios on both hosts. Savings are not measured yet: the ledger exists to measure them.
- Absorb?: Compose, keep in agentic-kit: a spawn gate is a dispatch-loop concern, which fathom excludes. The ledger could feed `fathom metrics spend` later.

### cavecrew agents

Investigator, builder and reviewer agents with compressed output contracts, identical instructions on Claude Code and Codex. (Built today; agentic-kit · assets/agents)

- Scores: Impact 3/5, Risk 1/5, Proven 2/5, Reception 3/5
- Fits: The cavecrew skill you like pointed at agents that were never installed, so it was used 0 times. Structure works where the "/caveman" nudge did not (6% compliance).
- Fathom: The investigator uses fathom's index first, then rg, so site lookups cost a cheap Haiku pass instead of main-context reads.
- Evidence: Compression measured at about 8.5% fewer tokens on 86 tasks (JetBrains); the bigger win is smaller reports landing in the orchestrator.
- Absorb?: Keep outside: fathom ships its own fathom-* skills. The investigator already calls fathom first.

### Subagent-driven development, as a pattern

A fresh implementer per task, a review per task and a final review; the orchestrator keeps a ledger and narrates one line. (Adopt the pattern; obra/superpowers)

- Scores: Impact 4/5, Risk 1/5, Proven 3/5, Reception 5/5
- Fits: It is how you already work; it is now written into docs/tokenomics.md. The full plugin is a pass: it adds a large session-start skill and workflows that compete with your intent practice.
- Fathom: Neutral. The review step pairs naturally with fathom review_diff and verify.
- Evidence: About 302k stars, maintained (pushed 2026-10-10). Skill-only, no hooks.
- Absorb?: Compose: fathom's review_diff and verify are the review half.

### ccusage

Read-only cost and token reports from local Claude Code and Codex logs, by day, session and model. (Adopt; ccusage/ccusage)

- Scores: Impact 4/5, Risk 1/5, Proven 5/5, Reception 5/5
- Fits: The whole-session dollar view that agent-hooks' per-spawn ledger does not give; together they show whether routing changes the bill.
- Fathom: Complements; no hooks, nothing injected.
- Risk: Dollar figures are API-equivalent estimates, not subscription charges. Runs via npx.
- Evidence: 19.1k stars, release 2026-10-19, 4 open issues.
- Absorb?: Keep outside: non-goal (no token dashboard).

### ccstatusline

Claude Code status line showing context-window percent, tokens, cost and git state. (Adopt; sirmalloc/ccstatusline)

- Scores: Impact 3/5, Risk 1/5, Proven 4/5, Reception 4/5
- Fits: Your clean-main-context goal needs the context percent in view; only the status line receives it, hooks do not.
- Fathom: Complements.
- Risk: One tool owns the status line slot; your settings currently set none.
- Evidence: 13.3k stars, MIT, release 2026-10-09.
- Absorb?: Compose: `fathom statusline` runs inside it as a widget.

### gitleaks

Secret scanning as a lefthook pre-commit step. (Adopt; gitleaks/gitleaks)

- Scores: Impact 4/5, Risk 1/5, Proven 5/5, Reception 5/5
- Fits: Agents commit often and fast; a deterministic gate backs your rule that secrets never enter a repo.
- Fathom: Neutral; it lives in git hooks, not agent hooks.
- Risk: Release cadence is slow (last 2026-03).
- Evidence: 30.1k stars, MIT, industry standard.
- Absorb?: Absorb findings: fathom's documented direction is to import SARIF and record where. Detection stays here.

### ast-grep

Structural search and rewrite by syntax tree, with Go support. (Adopt; ast-grep/ast-grep)

- Scores: Impact 3/5, Risk 1/5, Proven 5/5, Reception 4/5
- Fits: Mechanical refactors become one exact command a fast-tier builder can run, instead of many file edits.
- Fathom: Complements: fathom says what to change and who breaks; ast-grep changes it precisely.
- Evidence: 16.3k stars, MIT, release 2026-10-15. Not installed yet.
- Absorb?: Keep outside: rewriting is a non-goal. Structural search joins only with call-log evidence.

### difftastic

Syntax-aware diffs for human review. (Adopt; Wilfred/difftastic)

- Scores: Impact 2/5, Risk 1/5, Proven 5/5, Reception 5/5
- Fits: You review a lot of agent-written diffs; this makes moved and reformatted code obvious. For people, not agents.
- Fathom: Neutral.
- Evidence: 26k stars, MIT, release 2026-09-18.
- Absorb?: Unrelated.

### context7 on Claude Code

Current library documentation on demand; already wired in Codex. (Adopt; upstash/context7)

- Scores: Impact 3/5, Risk 2/5, Proven 5/5, Reception 4/5
- Fits: Parity between the hosts: the same doc lookup in both, instead of web fetches into the main context.
- Fathom: Complements: fathom covers your code, context7 covers third-party APIs.
- Risk: Library names leave the machine.
- Evidence: 63.3k stars, MIT, release 2026-10-19.
- Absorb?: Already has, for pinned deps: `search kind:docs pkg:` is version-exact and local. Use context7 for libraries fathom hasn't indexed.

## Trial

### worktrunk

Git worktree lifecycle for parallel agents, with its own hooks. (Trial; max-sixty/worktrunk)

- Scores: Impact 3/5, Risk 2/5, Proven 3/5, Reception 4/5
- Fits: Your waves use one worktree per agent and strict cleanup; this automates that lifecycle.
- Fathom: Untested: fathom indexes by repository root, so check that worktrees resolve to the right service.
- Risk: Pre-1.0, partly overlaps herdr.
- Evidence: 9.3k stars, release 2026-09-27.
- Absorb?: Roadmap, read side (F82 to F85 worktree overlap). Lifecycle stays with herdr.

### cass (session search)

Search across raw Claude Code and Codex transcripts. (Trial; coding_agent_session_search)

- Scores: Impact 3/5, Risk 2/5, Proven 2/5, Reception 3/5
- Fits: Fills the one real memory gap found: "what did we decide in session X", across both hosts.
- Fathom: Complements fathom's curated thread notes with raw recall; keep fathom as the authority.
- Risk: Single maintainer; it indexes private transcripts.
- Evidence: 1.2k stars, release 2026-10-02.
- Absorb?: Keep outside: "no transcript mining".

### semgrep CLI

Static analysis rules in lefthook or CI. Skip the archived MCP server. (Trial; semgrep/semgrep)

- Scores: Impact 2/5, Risk 2/5, Proven 4/5, Reception 3/5
- Fits: A deterministic check for patterns your constitution lint does not cover.
- Fathom: Overlaps fathom's guards for rules that already exist there; use it only for what they lack.
- Evidence: Widely used; the MCP repository is archived (last release 2025-06).
- Absorb?: Absorb findings via SARIF import. Fathom's own rule engine for this was pruned.

### trufflehog, for history audits

Deep secret scan of full git history; manual use only. (Trial; trufflesecurity/trufflehog)

- Scores: Impact 2/5, Risk 3/5, Proven 5/5, Reception 5/5
- Fits: A one-off audit before any repository goes public (agentic-kit's D4).
- Risk: Live verification sends found credentials to their providers; run with verification off. AGPL.
- Evidence: 28.6k stars, release 2026-10-19.
- Absorb?: Absorb findings via SARIF import, as with gitleaks.

### Anthropic official plugins (selected)

Try only session-report and security-guidance from the marketplace you already added. (Trial; claude-plugins-official)

- Scores: Impact 3/5, Risk 3/5, Proven 4/5, Reception 4/5
- Fathom: security-guidance and hookify add edit-time hooks that overlap fathom's obligations; expect duplicate guidance.
- Evidence: 43.2k stars, Apache-2.0. What session-report records was not verified.
- Absorb?: Keep outside: security-guidance duplicates obligations.

## Pass

### rtk (Rust Token Killer)

Rewrites Bash commands through a proxy that compresses output. Installed on this machine, never wired. (Pass; rtk-ai/rtk)

- Scores: Impact 2/5, Risk 4/5, Proven 2/5, Reception 3/5
- Why not: Vendor claims 60-90%; independent benchmarks (quesma.com, a $1,500 run) found no cost saving. A second Bash rewriter would race agent-hooks' containment.
- Would change: An A/B in the agent-hooks ledger showing a net saving above 10%. Consider removing the unused binary.
- Absorb?: Has a safer form: fathom readouts (ADR-071), off and unwired today.

### context-mode

Runs tool work in a sandbox so only its stdout enters context. (Pass; mksglu/context-mode)

- Scores: Impact 3/5, Risk 4/5, Proven 2/5, Reception 4/5
- Why not: 96-98% savings are self-reported only; its routing hooks would overlap yours. Main-thread containment covers the Bash case and keeps the full output on disk.
- Would change: An independent measurement, or your own ledger showing main-thread Read output dominates.
- Absorb?: Has a safer form: fathom readouts shape output without running commands.

### claude-mem

Captures every tool call with hooks, summarizes them, and injects memory at session start. (Pass; thedotmack/claude-mem)

- Scores: Impact 3/5, Risk 4/5, Proven 2/5, Reception 4/5
- Why not: Duplicates native memory and fathom continuity; a hook on every tool call plus start-up injection in your longest sessions. Open issues on unverifiable savings, a 46 GB environment, and Codex prompts captured as user prompts.
- Would change: A cross-host recall need that fathom thread notes cannot serve; then trial it on one throwaway project.
- Absorb?: Keep outside: fathom refuses captured, model-summarized memory. Thread notes instead.

### superpowers, the plugin

The full skill pack with its session-start bootstrap. (Pass; obra/superpowers)

- Scores: Impact 2/5, Risk 3/5, Proven 3/5, Reception 5/5
- Why not: The useful part is adopted as a pattern above; the rest competes with your intent-* and fathom workflows and costs context every session.
- Absorb?: Keep outside: fathom ships its own skills and has no workflow engine.

### Serena

LSP-backed code navigation over MCP, with a memories folder. (Pass; oraios/serena)

- Scores: Impact 2/5, Risk 2/5, Proven 3/5, Reception 4/5
- Why not: Fathom already does code intelligence with evidence and governance; two indexes would split agents' attention.
- Absorb?: Already has: find_symbol, symbol_context and references.

### mem0 / OpenMemory

LLM-extracted facts in a vector store over MCP. (Pass; mem0ai/mem0)

- Scores: Impact 2/5, Risk 3/5, Proven 2/5, Reception 4/5
- Why not: A model call per memory; its benchmark is self-reported, and a competitor's run found plain files did better. Preferences fit native memory.
- Absorb?: Keep outside: non-goal.

### basic-memory, Letta, Zep and Graphiti

Markdown-note memory, an agent framework, and temporal knowledge graphs. (Pass; getzep/graphiti)

- Scores: Impact 2/5, Risk 3/5, Proven 2/5, Reception 3/5
- Why not: basic-memory duplicates file memory (AGPL); Letta needs its own runtime; Zep and Graphiti need a graph database and a model per episode. No present need.
- Absorb?: Keep outside: non-goal.

### github-mcp-server

GitHub operations as MCP tools. (Pass; github/github-mcp-server)

- Scores: Impact 2/5, Risk 3/5, Proven 5/5, Reception 4/5
- Why not: The gh CLI does the same with no tool schemas in context. If ever needed, trial with a narrow --toolsets list.
- Absorb?: Keep outside: no domain connectors.

### claude-squad, workmux

Terminal managers for parallel agent sessions. (Pass; smtg-ai/claude-squad)

- Scores: Impact 1/5, Risk 2/5, Proven 3/5, Reception 3/5
- Why not: herdr already does this, with agent state reported by hooks.
- Absorb?: Keep outside: herdr owns this.

### Usage monitors and transcript viewers

Live usage dashboards and transcript browsers. (Pass; Claude-Code-Usage-Monitor)

- Scores: Impact 1/5, Risk 1/5, Proven 3/5, Reception 3/5
- Why not: ccusage covers usage, and cass covers search if you trial it.
- Absorb?: Keep outside: no token dashboard, no transcript mining.

## Fathom fit, by tool

Each tool judged against fathom's own charter: determinism first, evidence with receipts,
no net tool growth. Its non-goals rule out a token dashboard, a transformation engine, a
dispatch loop, transcript mining, IDE language services and domain connectors. External
findings come in as imports and authority never flows back out.

| Tool | Verdict | Why, from fathom's own docs |
|---|---|---|
| gitleaks, trufflehog, semgrep | Absorb findings | Import their SARIF output into the findings loop; detection stays with the scanner. Fathom's own secret scanner was pruned for this reason (reviews-and-incidents assessment 2026-10-11, vision/30) |
| context7 | Already has, for pinned deps | `search kind:docs pkg:<eco>/<name>@<ver>` serves docs for the exact pinned version from a local cache (KR-12). Fathom first for indexed dependencies, context7 for the rest |
| Serena | Already has | find_symbol, symbol_context and references match or beat it on fathom's bench. Live LSP operations are a non-goal |
| rtk, context-mode | Has a safer form | Fathom readouts (ADR-071) shape displayed Bash output without running commands. The command-wrapper design was refused because fathom never runs user commands. Readouts are off and unwired today. If turned on they overlap agent-hooks' containment: pick one |
| worktrunk | Roadmap, read side | Worktree line-overlap detection is planned (F82 to F85). Creating and removing worktrees stays with herdr |
| ccstatusline | Compose | A general status bar is a non-goal. `fathom statusline` (index freshness) is embedded as a ccstatusline widget |
| agent-hooks | Compose | A spawn gate is a dispatch-loop concern, so it stays in agentic-kit. Its ledger could feed `fathom metrics spend` as cost per proven row |
| Subagent-driven pattern | Compose | Fathom gates "done": review_diff and verify are the review half |
| ast-grep | Keep outside | Rewriting is a non-goal. Structural search would join only if call logs show queries fathom cannot answer |
| ccusage, usage monitors | Keep outside | Non-goal: "no generic token-usage dashboard". Fathom keeps a narrow slice in `fathom metrics spend` |
| cass, claude-mem, mem0, Letta, Zep | Keep outside | "No transcript mining"; fathom refuses model-summarized memory. Thread notes and `fathom prime` are its continuity answer |
| github-mcp-server, claude-squad, workmux | Keep outside | No domain connectors; coordinating machines belongs to herdr and portless |
| difftastic | Unrelated | Fathom maps diffs to symbols and impact; it does not render them |

## Related finding from the same report

A model judge filling unnamed spawn models (and flagging main-thread work) was assessed
and deferred: unproven accuracy, and a prior consumer's confidence-gated router was right
only 45.7% of the time at confidence 0.9 or higher (decision 0017). The larger leak
found: since the gate was installed, 99.6% of subagent tokens came from Workflow-tool
subagents (136 across 2 sessions, 69% deep tier, 31% standard), which never pass through
the Agent gate. Revisit once the ledger shows deny-and-retry overhead above 5% of subagent
tokens over 14 days (decision 0022), expected after 2026-11-02.
