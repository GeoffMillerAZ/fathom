# Token economics evidence, 2026-10-19

Companion record: [tools-assessment.md](tools-assessment.md) (24 tools rated, with evidence).

Evidence behind [decision 0022](../../../intent/decisions/0022-token-economics-is-enforced-at-the-spawn.md)
and [docs/tokenomics.md](../../../tokenomics.md). Hosts: Claude Code 2.1.304, Codex CLI
0.163.0, macOS. Everything here is a dated observation of one owner's machine: re-measure
before relying on a number, and treat vendor claims as claims.

## 1. Method

- **History.** `claude_history.py` over 5,052 Claude Code transcripts (2026-09-02 to
  10-19; nothing older survives locally) and `codex_history.py` over 792 Codex rollouts
  (2025-10-08 to 2026-10-18). Both read local files only and emit aggregates, never prompt
  text. Tokens are deduplicated per message (Claude) or response (Codex).
- **Host contracts.** Official documentation fetched on the day (code.claude.com hooks,
  sub-agents, plugins, env-vars; developers.openai.com/codex hooks, subagents, plugins,
  config reference), then verified in live sessions (section 4).
- **External practice.** Web research on proven token-saving hooks, orchestrator context
  hygiene, memory systems and adjacent tooling (section 5), weighted toward independent
  measurement over adoption, and adoption over vendor claims.

## 2. What the previous configuration did

Claude Code ran a bash PreToolUse hook on `Agent` (adopted 2026-07-16): deny worker
spawns with no model, Opus or an unknown model; allow critics to inherit; nudge
"/caveman" on worker spawns; warn critics carrying it.

| Measure (Claude Code, 463 spawns) | Value |
|---|---|
| Spawns by model argument | sonnet 307, unset 130, opus 25, haiku 1 |
| Spawns by type | general-purpose 158, implementer 128, **fork 118**, unset 43 |
| Denies | 35 (no model 32, Opus 3) |
| Denied spawns retried with a model in the same session | 8 of 34 (23.5%) |
| "/caveman" nudges / worker spawns carrying it afterwards | 110 / 6% |
| Notes on allowed critic spawns | 98, plus 31 critic-caveman warnings |
| cavecrew agent spawns | 0 (the skill named agents that were never installed) |
| Subagent share of all tokens / of output | **91.2% / 84.7%** |
| Opus-class subagents' share of all output | 46.9% |
| Report length, median (p90) chars, with / without "/caveman" | 2,509 (4,255) / 2,617 (9,428), confounded |
| Hook-injected context (fathom primer and obligations) | median 1.8k chars per session, about 0.007% of input |

Reading: the model gate changed spawns but was routed around (forks cannot take a model;
most denied spawns were dropped or re-issued as forks). The nudge did not change
behavior. Notes on allowed calls were noise. Hook injection is not a cost problem.

| Measure (Codex) | Value |
|---|---|
| `spawn_agent` calls carrying model and reasoning_effort | 188 of 209 (90%), always together |
| Subagent share of input tokens | about 30% (parents about 70%) |
| Cached share of input | about 97%; output is about 0.4% of input |
| Heaviest parent settings, 2026-09 | gpt-6-astra at ultra (780 turns), gpt-5.6-sol at medium (1,103) |
| Largest tool-output sinks | `exec` 1.0B chars (p90 28k); computer-use calls 200-700k chars each |

Codex's spend lever is the parent's model and effort, not subagent discipline.

## 3. Prices and catalog on the day

Anthropic first-party, per million tokens (input / output): Haiku 5.5 $0.10 / $0.50
(prompts up to 100k), Sonnet 5.5 $2 / $10, Opus 5.5 $4 / $20, Fable 5.1 $10 / $50. The
July policy assumed Opus at about 10x Sonnet; it is now 2x, and Haiku is a twentieth of
Sonnet. Codex's local catalog carries no prices; its priority rank orders gpt-6.1-sol,
gpt-6-astra, gpt-6-sol, gpt-6-luna, gpt-5.6-sol, gpt-5.6-terra, gpt-5.6-luna.

## 4. Host contracts verified live

Each was observed in a real `claude -p` or `codex exec` session, not only read in docs.

| Finding | Consequence |
|---|---|
| Claude Code shows a JSON PreToolUse deny as "PreToolUse:Agent hook error: …" | Not a failure; agents read the reason and retry. Explains the history's "hook error" prefix. |
| Claude Code agents launch async by default; PostToolUse fires at launch with `status: async_launched`, no tokens | Spend must be read at completion. |
| SubagentStop carries `agent_transcript_path` but fires before the final message is flushed | Tokens are read from the transcript at report time, never in the hook. |
| Bash output above about 30k chars is persisted by Claude Code itself (`persistedOutputPath`, a 2 KB preview) | Containment applies only below that; above it the hook stands down. |
| `updatedToolOutput` on a main-thread Bash result is shown to the model | Containment works: an agent read head, tail and the saved-file notice. |
| Codex sends the spawn as **`collaborationspawn_agent`**, not the documented `spawn_agent`, with v2 arguments (task_name, fork_turns, model, reasoning_effort, message) | A matcher of `spawn_agent` alone gates nothing. Both names are matched. |
| Codex loads project hooks only when the project layer is trusted in user config; `-c` session-layer hooks load with `--dangerously-bypass-hook-trust` | The smoke test injects hooks per invocation and never touches trust state. |
| Claude Code tool-event payloads carry `agent_id` only inside subagents (docs); Codex tool events carry none | Main-thread detection exists for Claude Code only. |

## 5. External research, weighed

| Practice or tool | Evidence | Verdict |
|---|---|---|
| Pin cheap models on mechanical subagents | Anthropic cost guidance; price ratios are arithmetic | adopt (the tier policy) |
| Delegate verbose work to subagents | Anthropic cost guidance | adopt (orchestrator guidance) |
| Filter or contain tool output with hooks | Official mechanism; no published measurement | adopt with evidence kept (containment) |
| Caveman-style compression | JetBrains paired run, 86 tasks: 8.5% fewer tokens; about 4% beyond "be concise" | keep inside agent definitions only |
| rtk command rewriting | Vendor 60-90%; independent benchmarks found no cost saving | reject until an A/B here says otherwise |
| context-mode sandboxing | Vendor-only 96-98% | reject until measured here |
| superpowers subagent-driven development | Very wide adoption; skills only | adopt the pattern, not the plugin |
| claude-mem and other memory add-ons | Vendor or self-reported; Letta's own LoCoMo run favored plain files | reject; native memory plus fathom threads |
| ccusage, ccstatusline | Wide adoption, read-only | recommended to consumers |

Sources: code.claude.com/docs/en/costs, /hooks, /sub-agents; developers.openai.com/codex/hooks,
/subagents, /memories; thenewstack.io/caveman-mode-token-savings;
quesma.com/blog/does-rtk-make-ai-coding-cheaper; github.com/rtk-ai/rtk;
github.com/mksglu/context-mode; github.com/obra/superpowers; github.com/thedotmack/claude-mem;
github.com/ccusage/ccusage; github.com/sirmalloc/ccstatusline (all fetched 2026-10-19).

## 6. Live smoke result

`scripts/agent-hooks-smoke.sh` on the day, both hosts, cheapest models: 9 of 9 scenarios
passed on the final build (unset model denied then corrected, worker ceiling, unmarked
fork, main-thread containment, the contained output and its saved-file path reaching the
model, spawn results measured; Codex unset model and effort denied together, corrected
spawn allowed, subagent stop and shell output measured).

An adversarial review of the first build found nine defects before merge, among them a
contained output that could lose its saved-file path (small stdout, large stderr), a
second ledger rotation that could evict a full file, sync spawns counted twice, policies
the runtime accepted but the schema rejected (missing fields, key case, byte versus rune
limits), and a command label that could store a word of a quoted secret. It also raised
the deny-loop risk if a host hides the spawn's model field. Every finding now has a
regression test, and the deny-loop breaker answers the last.

## 7. Limits

No pre-hook baseline survives (transcripts start 2026-09-02). Retry and report-length
measures are heuristic and small. Codex per-token model attribution is approximate. The
Workflow tool's agent() spawns and agent teams were not observed reaching PreToolUse.
