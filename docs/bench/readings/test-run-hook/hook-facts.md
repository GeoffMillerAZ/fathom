# TR-V1: what the Claude Code hooks carry when a test command runs

TR-V1 reading: PostToolUse(Bash) carries NO exit code (its `tool_response` is `{stdout, stderr, interrupted, isImage, noOutputExpected, returnCodeInterpretation?}`); a failing test command arrives as PostToolUseFailure only when the shell's exit status is non-zero, with the exit code as the first line of `error` (`Exit code N`), and as PostToolUse otherwise, which is 99 percent of real failing runs because agents pipe the output (536 of 541 failing runs in the scan). Register on both; parse the output; use the exit code only from PostToolUseFailure's `error`.

Plain summary: the design's guess was right in outcome and wrong in reason. Failing test runs mostly do arrive as PostToolUse results, but not because Claude Code routes failures there: an unpiped `go test` that exits 1 is a tool error and fires PostToolUseFailure. They arrive as PostToolUse because agents almost always pipe test output through `tail` or `grep`, so the shell's exit status is the filter's (0). The hook must therefore read the output on both events, and it can never assume it saw the whole run.

Date: 2026-09-29. Claude Code version read: 2.1.284 (the installed `@anthropic-ai/claude-code` package, `bin/claude.exe`). Method: the hook-input construction read from the shipped bundle (not a model and not the documentation's prose), cross-checked against real transcript tool results from this repository's sessions (statistics only; no command text or output is stored here).

## Fact 1: the PostToolUse(Bash) input

The bundle builds the PostToolUse input as `{...common, hook_event_name: "PostToolUse", tool_name, tool_input, tool_response, tool_use_id, duration_ms}`, where `tool_response` is the Bash tool's own result object. That object is built as `{stdout, stderr, interrupted, isImage, returnCodeInterpretation, noOutputExpected, backgroundTaskId, ..., persistedOutputPath, persistedOutputSize, ...}`. The exit code exists inside the tool (`Ne.code`) and is sent only to Claude Code's own telemetry (`tengu_bash_tool_command_executed.exit_code`); it is never placed in `tool_response`.

Transcript cross-check: 3,036 successful test-shaped Bash results (`go test`, `task validate|test`, `make test`, `npm test`, `pytest`, `cargo test`) over the newest 400 session files. Their result objects carried exactly `interrupted, isImage, noOutputExpected, stderr, stdout` (3,027) or those plus `returnCodeInterpretation` (9). None carried an exit code field.

Consequences: on PostToolUse the parse has only the text, which is capped at 30,000 characters by default (`BASH_MAX_OUTPUT_LENGTH`; larger output is persisted and named by `persistedOutputPath`). The stdout is present, which also answers the Claude Code half of PR-V1's first question (the context half and Codex remain open there).

## Fact 2: when a failing command fires PostToolUseFailure

The Bash tool interprets the exit status per command (`jWe`): for the last pipeline segment's first word, `grep`, `rg`, `find`, `diff`, `test` and `[` treat 1 as not an error; every other command (the default, which covers `go`, `task`, `make`, `npm`, `pytest`) is an error when the code is not 0. On an error the tool throws its `ShellError` (`code`, merged output), and the dispatcher fires PostToolUseFailure with `{...common, hook_event_name: "PostToolUseFailure", tool_name, tool_input, tool_use_id, error, is_interrupt, duration_ms}`. `error` is the formatted failure: `Exit code N`, then the command's output (stderr then stdout, joined by newlines). The same text is the tool result the model sees, which is why a failed Bash call in a transcript reads `Exit code 1` on its first line.

Transcript cross-check (same scan): 3,072 test-shaped calls; 36 were tool errors (PostToolUseFailure), 30 of them opening with `Exit code N` (codes 1, 2 and 128). Of the calls whose output showed a failing Go test (`--- FAIL`, `FAIL<tab>pkg`, a bare `FAIL`, or `panic:`), 536 arrived as successful results (PostToolUse), 528 of them piped, and 5 arrived as errors. 2,760 of the 3,072 test-shaped commands were piped.

## Fact 3: a field fathom reads wrong today

Fathom's Claude codec reads PostToolUseFailure's message from `tool_error`, a key Claude Code does not send; the key is `error`. HP-27's remedy line therefore never quoted the failure text. TR-2 fixes the codec to read `error` (keeping `tool_error` as a fallback) and names the key in the descriptor's input map.

## What the build does with these facts

- Register on PostToolUse(Bash) and PostToolUseFailure(Bash) (TR-2): dropping either loses real failing runs.
- The exit code is used only when the hook input carries it, which in Claude Code means PostToolUseFailure's leading `Exit code N` (TR-4). On PostToolUse the result is pass only when the runner's own summary covers every package and the output was not piped through a filter; otherwise unknown.
- The PostToolUseFailure matcher that exists today covers only fathom's MCP tools, so its firing count (19 in 4 days, design section 3) never measured Bash failures; it is not evidence either way.

## Reproduce

- The bundle strings: search `bin/claude.exe` for `hook_event_name:"PostToolUse"`, `hook_event_name:"PostToolUseFailure"`, `returnCodeInterpretation` and `class x1 extends Error` (the minified names are those of 2.1.284 and change between releases).
- The transcript statistics: a standard-library Python scan over `~/.claude/projects/<repo>/*.jsonl` and their `subagents/` files, pairing each test-shaped Bash `tool_use` with its `tool_result` and counting `is_error`, the leading `Exit code N`, the result object's keys, pipes and Go failure markers. Script kept outside the tree (`<scratch>/ew/trv1_scan.py`); nothing it reads is printed.
