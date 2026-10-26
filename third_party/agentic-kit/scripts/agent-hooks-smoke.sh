#!/usr/bin/env bash
# Live smoke test of cmd/agent-hooks against the real Claude Code and Codex
# CLIs. It spends real tokens (a few cents on the cheapest models), so it is
# not part of `task validate`; run it after installing or upgrading the
# binary or either host.
#
# Each scenario runs in a scratch project with its own state directory and
# asserts on the ledger the hooks wrote, so a pass proves the hosts actually
# delivered the events, not just that the binary works in isolation.
#
#   scripts/agent-hooks-smoke.sh [--bin PATH] [--claude-model M] [--codex-model M]
#                                [--codex-policy FILE] [--skip-claude] [--skip-codex]
#
# Claude Code runs with --setting-sources project, so only the scratch
# project's hooks load. Codex receives the hooks as session-layer -c
# overrides with --dangerously-bypass-hook-trust for that one invocation:
# the hooks are this repository's reviewed bytes, and nothing is written to
# the user's Codex trust state.
set -euo pipefail

bin="${HOME}/.local/bin/agent-hooks"
claude_model="haiku"
codex_model=""
codex_policy=""
run_claude=1
run_codex=1
while (($#)); do
	case "$1" in
	--bin) bin="$2"; shift 2 ;;
	--claude-model) claude_model="$2"; shift 2 ;;
	--codex-model) codex_model="$2"; shift 2 ;;
	--codex-policy) codex_policy="$2"; shift 2 ;;
	--skip-claude) run_claude=0; shift ;;
	--skip-codex) run_codex=0; shift ;;
	*) echo "unknown argument: $1" >&2; exit 2 ;;
	esac
done

[[ -x "$bin" ]] || { echo "agent-hooks binary not found at $bin (--bin)" >&2; exit 2; }
work="$(mktemp -d "${TMPDIR:-/tmp}/agent-hooks-smoke.XXXXXX")"
failures=0
trap 'echo "scratch: $work"' EXIT

pass() { printf 'PASS %s\n' "$1"; }
fail() { printf 'FAIL %s\n' "$1"; failures=$((failures + 1)); }

# expect NAME LEDGER PATTERN...: every pattern must match a ledger line.
expect() {
	local name="$1" ledger="$2"
	shift 2
	for pattern in "$@"; do
		if ! grep -q -- "$pattern" "$ledger" 2>/dev/null; then
			fail "$name: no ledger line matches $pattern"
			return
		fi
	done
	pass "$name"
}

claude_run() {
	local name="$1" prompt="$2"
	local dir="$work/claude"
	export AGENT_HOOKS_STATE="$work/state-$name" AGENT_HOOKS_POLICY="$work/none.json"
	(cd "$dir" && claude -p "$prompt" --model "$claude_model" --setting-sources project \
		--strict-mcp-config --no-session-persistence --max-budget-usd 0.50 \
		--output-format stream-json --verbose --allowedTools "Bash(seq:*)" \
		>"$work/$name.jsonl" 2>"$work/$name.err") || true
	echo "$AGENT_HOOKS_STATE/ledger.jsonl"
}

if ((run_claude)); then
	command -v claude >/dev/null || { echo "claude CLI not found" >&2; exit 2; }
	mkdir -p "$work/claude/.claude"
	git -C "$work/claude" init -q
	"$bin" print-hooks --host claude --bin "$bin" >"$work/claude/.claude/settings.json"

	l=$(claude_run c-unset 'Use the Agent tool once to spawn a general-purpose subagent with the prompt "Reply with the word pong." Do NOT pass a model on the first attempt. If blocked, follow the reason and retry once.')
	expect "claude: unset model is denied, the retry is allowed" "$l" '"decision":"deny","codes":\["no_model"\]' '"decision":"allow"'

	l=$(claude_run c-ceiling 'Use the Agent tool to spawn a general-purpose subagent with model "opus" and the prompt "Reply with the word ok." Do not add any bracketed tag. Report what happened.')
	expect "claude: a worker on the deep tier is denied" "$l" '"codes":\["tier_ceiling"\]'

	l=$(claude_run c-fork 'Use the Agent tool with subagent_type "fork" and the prompt "Say hi." Do not retry if blocked.')
	expect "claude: an unmarked fork is denied" "$l" '"codes":\["fork_unmarked"\]'

	l=$(claude_run c-contain 'Run exactly this bash command: seq 1 4000 — then say the first and last number you see.')
	expect "claude: large main-thread Bash output is contained" "$l" '"contained":true'
	# The ledger says the hook replaced the output; the transcript must show the
	# model actually received the replacement, saved-file pointer included.
	if grep -q 'main-thread output contained' "$work/c-contain.jsonl" && grep -q 'Full output: ' "$work/c-contain.jsonl"; then
		pass "claude: the model received the contained output and its saved-file path"
	else
		fail "claude: the contained output never reached the model"
	fi

	l=$(claude_run c-result 'Spawn one general-purpose subagent with model "haiku" and the prompt "Reply with the word done." Wait for it, then reply finished.')
	expect "claude: spawn results are measured" "$l" '"kind":"spawn_result"' '"resolved_model":'
fi

if ((run_codex)); then
	command -v codex >/dev/null || { echo "codex CLI not found" >&2; exit 2; }
	[[ -n "$codex_model" ]] || { echo "--codex-model is required for the Codex scenarios" >&2; exit 2; }
	mkdir -p "$work/codex"
	git -C "$work/codex" init -q
	overrides=()
	while IFS= read -r line; do overrides+=(-c "$line"); done < <("$bin" print-hooks --host codex --bin "$bin" | python3 -I -c '
import json, sys
def toml(v):
    if isinstance(v, dict): return "{" + ",".join(f"{k}={toml(x)}" for k, x in v.items()) + "}"
    if isinstance(v, list): return "[" + ",".join(toml(x) for x in v) + "]"
    return json.dumps(v) if isinstance(v, str) else str(v)
for event, groups in json.load(sys.stdin)["hooks"].items():
    print(f"hooks.{event}={toml(groups)}")')
	export AGENT_HOOKS_STATE="$work/state-x-unset" AGENT_HOOKS_POLICY="${codex_policy:-$work/none.json}"
	(cd "$work/codex" && codex exec --json --ephemeral --dangerously-bypass-hook-trust "${overrides[@]}" \
		-m "$codex_model" -c 'model_reasoning_effort="low"' \
		'Use spawn_agent to start exactly one subagent whose message is "Reply with the word pong." Do not fork context. On the first attempt do not set model or reasoning_effort. If blocked, follow the reason and retry once, then wait for the result. Also run the shell command `seq 1 50` once.' \
		>"$work/x-unset.jsonl" 2>"$work/x-unset.err" </dev/null) || true
	l="$AGENT_HOOKS_STATE/ledger.jsonl"
	expect "codex: unset model and effort are denied together" "$l" '"codes":\["no_model","no_effort"\]'
	expect "codex: the corrected spawn is allowed" "$l" '"decision":"allow"'
	expect "codex: subagent stops and shell output are measured" "$l" '"kind":"subagent_stop"' '"kind":"tool_output"'
fi

if ((failures)); then
	echo "$failures scenario(s) failed"
	exit 1
fi
echo "all scenarios passed"
