// Command agent-hooks is the token-economics hook pack for Claude Code and
// Codex CLI: one stdlib binary that both hosts call as a command hook, plus
// the doctor, policy and report commands that prove it is wired.
//
// What it enforces, at the one place delegation cost is decided — the
// subagent spawn (Claude Code's Agent tool, Codex's spawn_agent):
//
//   - Every spawn names its model. An unset model inherits the session model,
//     which is usually the most expensive tier; the deny reason carries the
//     routing table so the retry picks deliberately.
//   - The model resolves to a tier (fast, standard, deep, frontier by
//     default). Worker briefs cap at the standard tier; critic briefs (review,
//     verification, synthesis) at deep. A brief exceeds its class ceiling only
//     with the target tier's marker, such as "[deep]".
//   - Claude Code forks cannot take a model and run at the session's cost, so
//     they need the fork marker by default.
//   - Codex spawns also name reasoning_effort, because an omitted effort
//     inherits the parent's.
//
// Allowed spawns are silent. Advice (policy data) is emitted at most once per
// session per rule. After spawn.deny_loop_limit denies in a row in one
// session, the next spawn is let through with a note, so a session that
// cannot comply is never stuck. Every decision and every spawn result is
// appended to a local JSONL ledger so the policy can be measured, then kept
// or retired.
//
// Failure contract: tokenomics is economic guidance, not a security boundary.
// Any internal fault — unreadable input, a payload that is not a JSON object,
// unwritable state, a panic — exits 0 with no output, so a broken hook never
// blocks the host. Payloads are read member by member, so a field this
// binary does not use can change type without making a spawn unreadable.
// Only a deliberate policy decision denies. An invalid policy file falls back
// to the built-in policy and says so once per session; doctor fails on it.
//
// Concurrency: every hook invocation is its own short-lived process. Shared
// state is limited to O_APPEND ledger lines (one write per record), a
// lock-file-guarded ledger rotation, and O_EXCL marker files for
// once-per-session advice, so concurrent hook processes are safe. Nothing is
// held across invocations.
//
// Non-guarantees: hooks cover only the tool calls a host delivers to them.
// Claude Code Workflow-script agents, agent teams and hosted tools may bypass
// PreToolUse; Codex skips untrusted hooks until they are approved in /hooks.
// Prompt markers are declarations of intent, not proof of the work's class.
//
// Verified against: Claude Code 2.1.304 and Codex CLI 0.163.0 (2026-10-19).
package main
