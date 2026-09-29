# XM-V1: what the SubagentStart hook carries, and what RC-28's 591-byte rows are

XM-V1 reading: the SubagentStart input does NOT carry the sub-agent's prompt (Claude Code 2.1.284 builds it as the common fields plus `agent_id` and `agent_type`, and the hooks reference documents the same), so a hook cannot choose map entries by relevance to the task; and the 591-byte RC-28 rows ARE boilerplate: every emitted row of 600 bytes or less (549 of 874) is exactly the constant base primer (identity line plus opening-moves table) with no discoveries brief appended, and 591 is that primer's length for service "fathom" under the pre-SR-34 opening-moves wording.

Plain summary: both guesses in the design were right. The hook knows which session spawned the sub-agent and what kind of agent it is, never what the sub-agent was asked to do, so a handoff map id has to come from state the parent filed (its thread or its map), not from the prompt, and `map show --for '<task>'` (XM-D3) stays deferred because its trigger names this fact. The 591-byte rows carried no parent knowledge at all: only 325 of 874 emitted SubagentStart payloads (37 percent) held a discoveries brief.

Date: 2026-09-29. Claude Code version read: 2.1.284 (the installed `@anthropic-ai/claude-code` package, `bin/claude.exe`). Method: the hook-input construction read from the shipped bundle and cross-checked against the hooks reference (`https://code.claude.com/docs/en/hooks.md`, fetched the same day) and this repository's codec fixture; the payload sizes counted from the operator's beat log (`~/.fathom/hooks.jsonl` and its one rotated generation, channel `subagent-hook`, statistics only, no payload text is logged or stored). No model was called.

## Fact 1: the SubagentStart input has no prompt

The bundle builds the input as `{...common, hook_event_name: "SubagentStart", agent_id, agent_type}`, and the only caller passes the session, the new agent's id, its type and an abort signal: nothing that holds the task text. The common builder returns `{session_id, transcript_path, cwd, scratchpad_dir?, prompt_id?, permission_mode, agent_id, agent_type, effort?}`. The bundle's own input schema for the event is `hook_event_name: "SubagentStart", agent_id, agent_type` over the common fields. The hooks reference's SubagentStart section shows the same six keys and says the returned `additionalContext` is added "before its first prompt".

The near misses, each checked:

- `prompt_id` is a UUID naming the user prompt the session is processing (the parent's), for joining hook output to telemetry. It is an id, not the text, and it names the parent's prompt, not the sub-agent's.
- `transcript_path` is the parent session's transcript. The reference says it is written asynchronously and may lag the in-memory conversation, so the Agent tool call that carries the prompt may not be on disk when the hook fires; and a workflow-spawned sub-agent has no Agent tool call in the parent transcript at all (its prompt is computed by the script). Reading the prompt from there would be both unreliable and a read of transcript content, which the thread store refuses by construction.
- `agent_transcript_path` exists only on SubagentStop.
- The sub-agent's Bash environment carries the parent's `CLAUDE_CODE_SESSION_ID` and `CLAUDE_CODE_CHILD_SESSION=1`; no variable in the bundle exports the agent id to a subprocess. A sub-agent that should append with its agent id as provenance (XM-14) has to be told the id, by the brief or by the SubagentStart payload, which is the only moment fathom sees it.

This repository agrees: the Claude codec reads exactly `agent_id` and `agent_type` on SubagentStart (`internal/harness/codec.go`), and its fixture (`internal/harness/harness_test.go`) decodes `{"session_id","cwd","agent_id","agent_type"}`. SubagentStart fires for workflow agents as well as Agent-tool agents: this reading's own worker received the primer at start.

Consequence for the plan: XM-D3 (`map show --for '<task>'`) stays deferred, its trigger unmet. XM-8 carries one map id chosen from the parent's filed state (keyed by the parent session id the input does carry), and a sub-agent that needs a different slice asks for it by id.

## Fact 2: the 591-byte rows are the constant primer

`subagentPrimer` (`cmd/fathom/sessionhook_events.go`) renders a fixed identity-and-routing line, the opening-moves table, and then appends the parent's discoveries brief only when the parent established something; without a brief the payload stays under the 600-byte base cap, with one it may grow to 1,200. The base length depends only on the service name, the symbol count's digits and the binary's opening-moves wording, so it can be recomputed exactly:

| Bytes | Rows | What it is |
|---|---|---|
| 591 | 306 | service "fathom", five-digit symbol count, the O102 table (bug report routes to localize), the wording before SR-34 (68b90b1e) |
| 567 | 180 | service "fathom", the table before O102 (1eda1139) |
| 575 | 49 | service "fathom", the SR-34 table (the wording today) |
| 592, 593 | 12 | service "app-kit", the O102 table, five- and six-digit counts |
| 572 | 2 | service "agentic-kit", the table before O102 |

All 549 rows at or under 600 bytes carry the base primer and nothing else. The 591-byte rows fall on binaries built before SR-34 (291) or on a branch beside it (15), never on one after it, which is what a wording change predicts.

Window: 2026-09-14 08:30 to 2026-09-29 12:57 UTC, 1,075 SubagentStart firings: 201 silent (not an indexed root, or the daemon down), 874 emitted, 549 base-only, 325 with a discoveries brief (median 1,003 bytes, 90th percentile 1,177, smallest 615). 28 payloads were cut at exactly 1,200 bytes and 52 were 1,100 or more. Repositories: this one 732, app-kit 135, agentic-kit 7. The design's 283 of 510 was a shorter window of the same log.

Consequence for the plan: RC-28's pointer injection reached a sub-agent in about a third of spawns, so the re-derivation XM-M1 measured cannot by itself say that pointers failed where they were delivered: two thirds of spawns received none (this log has no per-spawn join to the transcripts, so the split is not measured here). And the 1,200-byte cap is tight where it matters: a map id appended after the discoveries brief would have been cut on 28 of 325 briefed spawns (9 percent), so XM-8 should place the id ahead of the brief and let the brief yield bytes, never the id.

## Reproduce

- The bundle: search `bin/claude.exe` for `hook_event_name:"SubagentStart"` (the builder and its caller) and for `transcript_path:vf(` (the common fields). Minified names are those of 2.1.284 and change between releases.
- The documentation: the SubagentStart input and the common input fields sections of the hooks reference.
- The sizes: a standard-library Python scan of `~/.fathom/hooks.jsonl` and `hooks.jsonl.1` for `channel == "subagent-hook"`, counting `meta.emitted` and `meta.bytes`, cut at 2026-09-29T13:00:00Z; the base lengths recomputed from the primer's format string with each historical `openingMovesTable` (`git show <commit>^:cmd/fathom/init.go`). Scripts kept outside the tree (`<scratch>/wb/core-scratch/rc28*.py`, `primer*.py`).
