RO-V1 reading: on Claude Code 2.1.302 both Stop and SubagentStop inputs carry the final assistant message (`last_assistant_message`), SubagentStop and every hook fired inside a subagent carry `agent_id`, SubagentStop carries the subagent's own transcript path, and `additionalContext` from either event reaches the model, which takes another turn at once (the subagent before it returns, never the main agent), so the row's facts are read and H2 can build on them with a re-entry guard.

# RO-V1: the Stop and SubagentStop facts H2 needs

Taken 2026-10-18 on Claude Code 2.1.302 (`claude --version`), one headless session with the installed login, no
API key variable. No fathom code ran: the scratch project's own hooks captured the inputs.

## What it means for H2

1. **The claim check needs no transcript read for the message.** Stop and SubagentStop both hand the hook the final
   assistant message in `last_assistant_message`. RO-39 reads that field and spends none of its 200 ms on the
   transcript tail.
2. **A subagent's rows can be told apart.** Every hook fired inside the subagent (PreToolUse, PostToolUse, and
   SubagentStop) carries `agent_id` and `agent_type`. The main agent's hooks carry neither, and both carry the
   parent's `session_id`. So "the agent's own rows" (RO-39) is answerable once the test-run row records the input's
   `agent_id`, which TR-7's row does not do today (`logTestRunRow` writes no agent key). PR-V3 reads yes.
3. **Context from Stop is not a quiet note for the next prompt: it re-opens the turn.** The text arrives as a
   `<system-reminder>` ("Stop hook additional context: ...") in a new model request, the model answers it at once,
   and Stop fires again with `stop_hook_active: true`. A hook that emits on every Stop loops until the turn cap. Here
   7 of the first prompt's 9 turns answered the marker. So every advisory line H2 prints costs one model turn,
   and RO-39 must print at most once per chain: silent when `stop_hook_active` is true, which the stop hook already
   does (`cmd/fathom/stophook.go`, "no re-entry loops").
4. **SubagentStop's context lands in the subagent, before it returns.** The subagent took another turn per
   SubagentStop (9 fires, `stop_hook_active` true from the second). The main agent got no hook context of its own.
   It saw the marker only because the subagent's report repeated it. A claim corrected at SubagentStop is corrected
   by the subagent, in the report the main agent reads.
5. **The person sees it too.** The stream-json output carries each `hook_response` with the hook's stdout, and the
   transcript records a `hook_success` and a `hook_additional_context` attachment plus a `stop_hook_summary` system
   line. How the interactive terminal draws it was not read.

## What was read

| Question (row text) | Stop | SubagentStop |
|---|---|---|
| carries the final assistant message | yes, `last_assistant_message` | yes, `last_assistant_message` (the subagent's) |
| `agent_id` | absent | present, with `agent_type` |
| the subagent's own transcript path | n/a | `agent_transcript_path`: `<project transcripts>/<session>/subagents/agent-<agent_id>.jsonl` |
| `additionalContext` reaches the model | yes: a new request carries it and the main agent answers it at once | yes: the subagent's next request carries it and the subagent answers before it returns |
| reaches the main agent | yes | no, except through the subagent's report text |
| re-entry flag | `stop_hook_active`: false first, true on each re-fire | same |

Hooks inside the subagent: the PreToolUse and PostToolUse inputs for the subagent's `ls` carried `agent_id` (the same
value SubagentStart and SubagentStop carried) and `agent_type: general-purpose`. The main agent's PreToolUse for the
Agent call carried neither.

Redacted SubagentStop input (values cut, paths and ids replaced):

```json
{"session_id": "<session>", "transcript_path": "<project transcripts>/<session>.jsonl", "cwd": "<scratch>",
 "prompt_id": "<id>", "permission_mode": "default", "agent_id": "<agent_id>", "agent_type": "general-purpose",
 "hook_event_name": "SubagentStop", "stop_hook_active": false,
 "agent_transcript_path": "<project transcripts>/<session>/subagents/agent-<agent_id>.jsonl",
 "last_assistant_message": "`ls` didn't list any files, ...", "background_tasks": [], "session_crons": []}
```

The Stop input has the same keys without `agent_id`, `agent_type` and `agent_transcript_path`.

## Method

One scratch project under the wave's own scratch directory, no git repository above it. Its `.claude/settings.json`
registered command hooks on Stop, SubagentStop, SubagentStart, PreToolUse (`*`) and PostToolUse (`*`), each running
one script that copied stdin to a file. Stop returned `hookSpecificOutput.additionalContext` "Stop hook marker:
VELLUMQUARTZ", SubagentStop "SubagentStop hook marker: BRINDLECOMET", the rest returned nothing. The session ran
`claude -p --input-format stream-json --output-format stream-json --include-hook-events --setting-sources project
--strict-mcp-config --max-turns 8`, with a scrubbed environment (no API key or parent-session variables), allowed tools
Agent and `Bash(ls)`, and two user turns: spawn one general-purpose subagent that runs `ls`, then quote any hook
context. The markers were traced through both transcripts: VELLUMQUARTZ in the main transcript's
`hook_additional_context` attachments and the next model request, BRINDLECOMET in the subagent transcript's and the
subagent's next request, and in the main transcript only inside the Agent tool result. Both prompts ended at
`error_max_turns`, which is finding 3. Raw captures stay out of the repository.
