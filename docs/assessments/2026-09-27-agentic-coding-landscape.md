# The agentic coding landscape, September 2026

Created: 2026-09-27 (summarized into the repository 2026-09-28).
Status: a reading of vendor and project documentation on 2026-09-27; not re-run. Decisions it
drove are in ADR-049, ADR-050 and ADR-051 and in `docs/vision/38-proof-layer-roadmap.md`.
Readable form: artifact LwjYnxTFUTqTmDYnErbYYD (The Agentic Coding Landscape).

## Verdict

Keep building, and point fathom at the gap. The 2026 market has plenty of ways to find code (grep,
language servers, a dozen new graph tools) and plenty of ways to describe work (specs, memory,
rules files). It has very few ways to prove things about code: what breaks, which rule applies,
whether the work is really done. That is fathom's lane, and review and governance is where the
year's money went. The most valuable next moves are not new engines. They are reach: install in one
command, read the rules and specs people already write, feed the reviewers they already use, and
start with a handful of tools instead of dozens.

## Method, and how to read the grades

Three research passes on 2026-09-27 covered coding clients, context engines, and memory, spec and
review tools, about 60 tools in all. Stars, licences and dates came from the GitHub API that day
(*measured* on that date). Funding, valuation and usage figures come from vendor pages and press
coverage (*claimed by a vendor*); where only a secondary source existed the figure is approximate.
Vendor benchmark numbers, for example a README's "120 times fewer tokens", are the vendor's claims
and are not repeated here as facts. The capability matrix in section 4 is a reading of each tool's
documentation (*estimate*, not a hands-on test). Stars measure attention, not quality or usage.

## 1. The market in five lanes

Most tools fit one lane. Fathom spans the context lane and parts of the spec and review lanes,
which is both its strength and why it takes longer to explain.

| Lane | What it does | Tools and figures (GitHub stars unless stated) |
|---|---|---|
| Coding clients | Where the agent runs: IDEs, terminals, cloud agents | OpenCode 210k; Codex 127k; Gemini CLI 107k; Zed 91k; Cline 69k; Claude Code (39 percent of surveyed developers use it, per a JetBrains developer survey, 2026); Cursor (a reported $60B deal); Copilot (4.7M paid seats, as reported); Devin and Windsurf (a reported $48B); Kiro (AWS) |
| Context engines | Give agents facts about the code: graphs, search, references | Graphify 122k; CodeGraph 72k; Context7 (library docs) 62k; GitNexus 48k; codebase-memory 45k; Serena 30k; Augment and Sourcegraph (hosted); fathom |
| Memory | Remember decisions and past sessions | claude-mem 95k; Mem0 66k; Hindsight 36k; Graphiti and Zep 31k; qmd 30k; Letta 25k; Basic Memory 4k; Claude OS 0.4k |
| Specs and plans | Turn a goal into specs, tasks and a process | Superpowers 292k; Spec Kit 139k; OpenSpec 70k; BMAD 54k; Task Master 28k; Beads 27k; Agent OS 5k |
| Review and governance | Check changes before they merge | CodeRabbit (a reported $1.5B valuation); Cursor Bugbot (44k learned rules); Qodo (a reported $120M raised; the same source page also says $70M for its latest round, so treat both as approximate); Greptile (a reported $180M valuation); Copilot review (generally available); PR-Agent 13k |

Three of the fastest risers are code-graph tools launched in 2026: Graphify, CodeGraph and
GitNexus. The idea behind fathom (a graph of the code for agents) is clearly in demand; the race is
on packaging and trust.

## 2. Eight trends, and what each meant for fathom

| # | The market did | Evidence in the sources | Meaning for fathom | Landed in |
|---|---|---|---|---|
| 1 | Fast search beat stored embeddings | Cursor says it does not store embeddings of your code; Cognition trained models that fire many greps at once; Claude Code and Codex search first; precise answers come from language servers | Good: fathom never relied on embeddings and a resolved graph complements grep | ADR-049 decision 8 (FTS5, no embeddings), decision 12 |
| 2 | Hooks became the place to enforce rules | Six major clients have before-tool hooks that can allow, deny or rewrite; Claude Code's docs call rules files context and hooks enforcement | Validates rules served at the edit | ADR-049 decision 9; ADR-038 |
| 3 | AGENTS.md became the standard | 60k+ projects, now under the Linux Foundation; Copilot also reads CLAUDE.md and GEMINI.md | Compile rules from every client's rules files | ADR-049 decision 9; GL-1 to GL-6 |
| 4 | Memory must cite its source | Copilot Memory stores code locations and re-checks them before use; Graphiti keeps old facts marked no longer true | Validates anchored, re-checked claims and validity windows | ADR-049 decision 9 (N8.2); GL-15 to GL-17 |
| 5 | Spec-driven development went mainstream | Spec Kit, OpenSpec, Kiro, Superpowers and BMAD turn goals into specs and tasks; checking that code meets the spec is mostly a model's opinion | Fathom can be the proof layer under these specs | ADR-049 decision 6; SI- rows |
| 6 | Background agents that wake on events | Cursor subscriptions, Claude routines, Amp orbs, Kiro Crew, Jules run on PRs, schedules or messages | Fathom's impact and verify results can be an event, but dispatch stays a non-goal | ADR-049 decision 10; F28 (deferred) |
| 7 | Review became its own product | Bugbot learns rules from reviewer feedback; Copilot review mixes CodeQL with the model; CodeRabbit splits huge agent diffs into layers | Fathom's deterministic findings can feed these reviewers | ADR-049 decision 9 (SARIF, check payload, layered diffs); ADR-055 |
| 8 | Resource use became a real complaint | Users report tens of GB from Claude Code and Codex; claude-mem burns tokens; OpenCode's docs warn that language servers cost memory | Fathom's hard memory budget is a selling point | ADR-047; `2026-09-27-peer-stacks-and-rust.md` |

## 3. What the market is like, in one line per lane

Clients converge on grep plus language servers plus subagents; context engines race on packaging;
memory tools compete on capture and recall; spec tools all check code against spec with a model;
review products learn rules from feedback. Free-only tools struggled: Roo Code shut down, Vibe
Kanban found no business model, and Continue was absorbed by Cursor. The money is in review and
governance.

## 4. Capability matrix (a reading of documentation, not a hands-on test)

Y means a real feature, P partial or limited, a dash not a feature. Based on each tool's own docs as
of September 2026.

| Capability | Fathom | Claude Code | Cursor | Copilot | Kiro | Spec Kit | CodeGraph | GitNexus | Augment | claude-mem | Claude OS | CodeRabbit |
|---|---|---|---|---|---|---|---|---|---|---|---|---|
| Who calls this / what breaks | Y | P | - | - | - | - | Y | Y | P | - | - | P |
| Calls confirmed by a type checker | Y | P | - | - | - | - | - | - | - | - | - | - |
| Impact across several repos | Y | - | - | - | - | - | - | Y | P | - | - | - |
| Team rules served at the edit | Y | P | P | P | P | - | - | P | - | - | - | - |
| Rules with citations to their source | Y | - | - | P | - | - | - | - | - | P | - | P |
| "Done" proven against the code, not by a model | Y | - | - | - | P | - | - | P | - | - | - | P |
| Spec and task workflow | P | P | P | Y | Y | Y | - | P | - | - | P | - |
| Automatic memory capture | P | Y | P | Y | - | - | - | - | - | Y | Y | P |
| Search by meaning (embeddings) | - | - | P | P | - | - | - | P | Y | Y | Y | P |
| Index updates live as files change | P | Y | Y | Y | Y | - | Y | Y | Y | P | P | P |
| Works in many clients (MCP) | Y | - | - | - | - | Y | Y | Y | Y | Y | P | P |
| Runs fully on your machine | Y | P | P | - | P | Y | Y | Y | P | Y | Y | - |
| One-command install | P | Y | Y | Y | Y | Y | Y | Y | Y | Y | P | Y |
| Hard limit on its own memory use | Y | - | - | - | - | - | - | P | - | - | - | - |

Notes from the source: Claude Code's "who calls this" is partial because its language-server tool
gives call hierarchy for one project at a time. GitNexus gates commits on the blast radius of a
change, which is close to, but not the same as, proving a task is done, and its licence is
non-commercial.

## 5. The honest read

**Ahead.**
- Proof instead of opinion. Spec Kit's converge step and OpenSpec's verify ask a model whether the
  code matches (OpenSpec's check does not even block archiving). Fathom's verify is a gate that
  reads the code.
- Rules with sources, at the edit. Others store rules as text; fathom serves the rule that governs
  this file with its citation and can block a violating edit.
- Calls confirmed by the compiler, across repositories. Most new graph tools read syntax only; only
  Sourcegraph and Moderne match compiler accuracy, and both are commercial.
- Honest measurement. Fathom's "about as good as the best tools at lower cost" matches the most
  credible numbers in the market and is more modest than README claims of 120 times fewer tokens.
- A good guest on a laptop: a hard memory budget, while users complain about other tools using tens
  of GB.

**Behind.**
- Getting started: the breakout tools install in one command from a plugin marketplace.
- Attention: new graph tools gathered 30k to 120k stars in months; fathom has not been published to
  that audience.
- Languages: peers claim 35 to 250 or more through tree-sitter; fathom is deepest in Go.
- Live updates: CodeGraph, GitNexus and codebase-memory update as files change; fathom refreshes on
  commits and on request.
- Calls between services: several peers link HTTP routes to handlers and match API contracts.
- Search by meaning: fathom has none, by choice; for free-form notes most memory tools do.

## 6. What to adopt, and where each idea landed

Grouped by the source's own timing. The verdicts are the owner's, in vision 38 section 6.

| When | Idea | Verdict | Landed in |
|---|---|---|---|
| Now | One-command plugin (Claude Code marketplace, Copilot Agent Plugins) | accept; publication stays the owner's Q4 call (F2) | RH- rows, G16 |
| Now | A small default tool set (Task Master offers 7, 15 or 36; CodeGraph exposes one explore tool and found agents choose better) | accept | ADR-053 decision 1; RH-1 |
| Now | Be the proof layer under Spec Kit and OpenSpec | accept | SI- rows, G19 |
| Now | Read every client's rules files with off, warn and error levels | accept | GL-1 to GL-6, G21 |
| Now | Serve rules through every client's hooks | accept | GL-7 to GL-9 |
| Next | Rules that learn from feedback (Bugbot promotes after positive signals and retires after repeated negative ones) | accept | GL-10 to GL-14 |
| Next | Keep invalidated facts with dates | accept (extends SU-19) | GL-15 to GL-17 |
| Next | Feed findings to reviewers as SARIF or a check | accept (F14 output format only) | GL-20, GL-21 |
| Next | Calls between services | accept | ADR-050; SG- rows, G17 |
| Next | A stronger proof tier: tests tied to requirements (Kiro generates property tests from requirements) | accept as `certified_by:` | SI- rows, ADR-043 |
| Next | Split big diffs into layers (CodeRabbit's change stack) by blast radius, no model | accept | GL-22 |
| Next | Show the cost of every answer (claude-mem shows tokens per result) | accept | PE-20, PE-21 |
| Next | Score impact against what changed together in git history | accept | ADR-050 decision 6 |
| Later | Live updates as files change, with an incremental-equals-full check | accept under budget | SG-28, SG-29 |
| Later | Full-text search over docs and notes (SQLite FTS5) | accept, pulled to wave 2 | ADR-049 decision 8; KR- rows |
| Later | A shared team index snapshot | defer | F27 |
| Later | Answer events for background agents | defer | F28 |

## 7. What not to copy

- Do not let a model decide what is done; the popular spec tools do, and fathom's reason to exist
  is that it does not (ADR-049 decision 12).
- Do not capture everything automatically without a budget; claude-mem's biggest complaint is
  runaway token use.
- Do not add heavy servers: graph databases, vector databases and Redis each make a local tool a
  heavier guest. Keep one embedded store.
- Free-only tools struggle, so the public-release call (Q4, F2) is a business question and not only
  a licensing one; the licence decision is ADR-052.
- Stars follow packaging: the fastest growth came from one-command installs and marketplace
  listings, not from the deepest engineering. That is the case for reach before depth (ADR-049
  decision 3).

## 8. What it drove

| Decision | Where |
|---|---|
| The kitchen-sink gate, pillars, reach first, views never trackers, no Rust rewrite | ADR-049; vision 38 |
| The system graph and dependency APIs from local caches | ADR-050 |
| The joins only fathom can do (test intelligence, work intelligence, change and compliance receipts) | ADR-051 |
| Goals G16 to G25 | `docs/intent/scope.md` |
| Deferred: shared snapshot, answer events, Rust wasm module | F27, F28, F29 |

## 9. What this does not claim

That any tool is better or worse than another on a task (no hands-on test was run); that stars,
valuations or usage figures are precise; that vendor benchmarks are true; that the matrix is current
after 2026-09-27; that fathom's position is stable (the lane is contested and the fastest risers are
copying its idea).

## Sources (public, read 2026-09-27)

JetBrains developer survey 2026 (blog.jetbrains.com research); Claude Code hooks and memory
documentation (code.claude.com); Cursor search documentation and the Bugbot learning post
(cursor.com); GitHub Copilot Memory documentation and the agentic code review changelog; Spec Kit
(github/spec-kit), OpenSpec (Fission-AI/OpenSpec); Kiro correctness documentation (kiro.dev);
CodeGraph (colbymchenry/codegraph), GitNexus, codebase-memory-mcp, Graphify (Graphify-Labs),
Serena (oraios/serena); Augment Context Engine MCP post; Sourcegraph MCP documentation; claude-mem,
Graphiti (getzep/graphiti), Claude OS (brobertsaz/claude-os); CodeRabbit agentic change
management post.
