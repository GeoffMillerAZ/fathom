// ClaudeHarness (SB-14) shells out to `claude -p`, exactly as
// scripts/context-bench.sh does (--output-format stream-json --verbose,
// --mcp-config, --strict-mcp-config, --allowedTools/--disallowedTools,
// --max-turns), and hands the captured transcript to ParseSessionStream.
// Running a real session spends real tokens — this file's tests exercise
// buildClaudeArgs and relativizeSessionPaths only; RunSession itself is
// exercised by FakeHarness in TestRunSessions_FakeHarnessReplays (SB-15).
package scenariobench

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

// ClaudeHarness runs one session via the real `claude` CLI.
type ClaudeHarness struct {
	// Bin is the `claude` executable; "claude" (resolved via PATH) when
	// empty.
	Bin string
}

// Name implements Harness.
func (h ClaudeHarness) Name() string { return "claude-code" }

// Version implements Harness: `claude --version`'s first line, trimmed.
// Called on demand — never at construction — so a caller that never runs a
// session never needs the binary installed.
func (h ClaudeHarness) Version() string {
	bin := h.bin()
	out, err := exec.Command(bin, "--version").Output()
	if err != nil {
		return ""
	}
	line := strings.TrimSpace(string(out))
	if i := strings.IndexByte(line, '\n'); i >= 0 {
		line = line[:i]
	}
	return line
}

func (h ClaudeHarness) bin() string {
	if h.Bin != "" {
		return h.Bin
	}
	return "claude"
}

// RunSession implements Harness: writes req.Arm's MCP config to req.RawDir,
// runs `claude -p` in req.Worktree capturing the stream-json transcript,
// and parses it with ParseSessionStream. A session that ran but produced
// no parseable result is reported as data (SessionResult.Error), not a Go
// error — one bad session should not abort a multi-session run; only a
// failure to even start the process is returned as an error.
func (h ClaudeHarness) RunSession(req SessionRequest) (SessionResult, error) {
	if err := os.MkdirAll(req.RawDir, 0o755); err != nil {
		return SessionResult{}, fmt.Errorf("claude harness: creating raw dir %s: %w", req.RawDir, err)
	}
	mcpPath := filepath.Join(req.RawDir, fmt.Sprintf("session-%d-mcp.json", req.Index))
	if req.MCPConfigPath != "" {
		// The RENDERED config (RenderArm's own normalization — alwaysLoad
		// defaulted true, a wired arm's merged-in fathom entry) is what
		// must reach the agent; copying it into RawDir (rather than
		// pointing --mcp-config at it directly) keeps the existing
		// provenance filename and worktree-independent raw artifact.
		raw, err := os.ReadFile(req.MCPConfigPath)
		if err != nil {
			return SessionResult{}, fmt.Errorf("claude harness: reading rendered mcp config %s: %w", req.MCPConfigPath, err)
		}
		if err := os.WriteFile(mcpPath, raw, 0o644); err != nil {
			return SessionResult{}, fmt.Errorf("claude harness: writing mcp config copy %s: %w", mcpPath, err)
		}
	} else {
		// No rendered config (a SessionRequest built by hand, e.g. a
		// test): fall back to writing req.Arm.MCP verbatim, as before.
		mcpServers := req.Arm.MCP
		if mcpServers == nil {
			mcpServers = map[string]any{}
		}
		if err := writeJSONFile(mcpPath, map[string]any{"mcpServers": mcpServers}); err != nil {
			return SessionResult{}, fmt.Errorf("claude harness: %w", err)
		}
	}

	streamPath := filepath.Join(req.RawDir, fmt.Sprintf("session-%d-%s.stream.jsonl", req.Index, safeRole(req.Role)))
	args := buildClaudeArgs(req, mcpPath)

	cmd := exec.Command(h.bin(), args...)
	cmd.Dir = req.Worktree
	cmd.Env = SubjectEnv(req.Worktree, req.Env)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	runErr := cmd.Run()
	if err := os.WriteFile(streamPath, stdout.Bytes(), 0o644); err != nil {
		if errors.Is(err, os.ErrPermission) {
			// A wedged external-volume TCC verdict (EPERM on a path that
			// stat can see) is an instrument fault of this machine, not of
			// the cell: report it as an instrument error so the cell is
			// LOST and the run continues (vs-pilot-2 aborted 33 cells in on
			// one, 2026-09-13); a rerun fills the gap.
			return SessionResult{
				Index: req.Index, Role: req.Role, RowID: req.RowID,
				InstrumentError: fmt.Sprintf("instrument: writing transcript %s: %v", streamPath, err),
				Error:           fmt.Sprintf("instrument: writing transcript %s: %v", streamPath, err),
			}, nil
		}
		return SessionResult{}, fmt.Errorf("claude harness: writing transcript %s: %w", streamPath, err)
	}
	if runErr != nil {
		_ = os.WriteFile(streamPath+".stderr", stderr.Bytes(), 0o644)
	}

	result := parseSessionStreamBytes(stdout.Bytes())
	result.Index = req.Index
	result.Role = req.Role
	result.RowID = req.RowID
	result.RawPath = streamPath
	result.FilesRead = relativizeSessionPaths(req.Worktree, result.FilesRead)
	result.FilesEdited = relativizeSessionPaths(req.Worktree, result.FilesEdited)
	// claude -p exits 1 whenever its result carries is_error — including
	// error_max_turns, the budget hand-off. A transcript with a terminal
	// result is a reading whatever the exit code; only a run with no result
	// event at all is an error (guards pilot 3 lost four cells to this,
	// 2026-09-13).
	if runErr != nil && result.Error == "" && !result.BudgetHit && result.Turns == 0 {
		result.Error = fmt.Sprintf("claude -p exited non-zero: %v", runErr)
	}
	return result, nil
}

// defaultRestrictedTools is what `--tools` names under `--restricted`
// (RC-6) when the arm declared no AllowedTools of its own: the built-in
// set a session needs to actually work, since `--restricted` otherwise
// strips every command/code-running tool.
var defaultRestrictedTools = []string{"Bash", "Read", "Edit", "Write", "MultiEdit", "Grep", "Glob", "NotebookEdit", "TodoWrite"}

// restrictedToolsList is what `--tools` should name for req: allowed with
// every "mcp__<server>" entry removed (an MCP server's own tools are
// gated by --mcp-config/--strict-mcp-config, not --tools) when allowed is
// non-empty, else defaultRestrictedTools.
func restrictedToolsList(allowed []string) []string {
	if len(allowed) == 0 {
		return append([]string(nil), defaultRestrictedTools...)
	}
	out := make([]string, 0, len(allowed))
	for _, t := range allowed {
		if !strings.HasPrefix(t, "mcp__") {
			out = append(out, t)
		}
	}
	if len(out) == 0 {
		return append([]string(nil), defaultRestrictedTools...)
	}
	sort.Strings(out)
	return out
}

// buildClaudeArgs is the pure part of RunSession: the CLI flags for one
// session, given the request and a resolved MCP config path. Kept
// separate from process execution so it is unit-testable with no spend.
//
// The clean room (RC-6, vision 28 §2 F3/F4) is `--restricted` plus
// `--disable-slash-commands`: `--restricted` ignores the operator's user,
// project and local settings files (hooks, plugins) while still honoring
// `--settings <file>` (the arm's own rendered settings.json) and
// `--mcp-config`/`--strict-mcp-config` (already passed below), and
// confines the file tools to the working directory; `--tools` is exactly
// what the arm allows (restrictedToolsList). An allow-list additionally
// gets `--permission-mode default`, since "auto" (Claude Code's default)
// approves outside an allow-list too — 35 writes from
// allowedTools:[Read,Grep,Bash] arms in the second reading.
// sessionAllowList is what --allowedTools names for req: the arm's own
// allow-list when it has one (already carrying mcp__<server> entries from
// RenderArm), else the restricted tool list plus mcp__<server> for each of
// the arm's servers — sorted, so the argv is stable.
func sessionAllowList(req SessionRequest) []string {
	if len(req.AllowedTools) > 0 {
		return sortedCopy(req.AllowedTools)
	}
	out := restrictedToolsList(nil)
	for _, name := range req.MCPServerNames {
		out = append(out, "mcp__"+name)
	}
	sort.Strings(out)
	return out
}

func buildClaudeArgs(req SessionRequest, mcpConfigPath string) []string {
	args := []string{
		"-p", req.Prompt,
		"--output-format", "stream-json",
		"--verbose",
		"--mcp-config", mcpConfigPath,
		"--strict-mcp-config",
		"--restricted",
		"--disable-slash-commands",
	}
	if req.SettingsPath != "" {
		args = append(args, "--settings", req.SettingsPath)
	}
	args = append(args, "--tools", strings.Join(restrictedToolsList(req.AllowedTools), ","))
	if req.SystemAppend != "" {
		args = append(args, "--append-system-prompt", req.SystemAppend)
	}
	// Every session runs under --permission-mode default with an explicit
	// allow-list: an arm's own list when it has one, else every tool the
	// session was given plus mcp__<server> for each of its servers. Under
	// --restricted a session with no allow-list had every Bash call answered
	// "This command requires approval" (-p cannot approve), and without
	// --add-dir even mkdir inside the worktree was refused (reuse-pilot-2,
	// 2026-09-13) — the clean room must deny the operator's hooks, never the
	// agent's own hands.
	args = append(args, "--allowedTools", strings.Join(sessionAllowList(req), " "))
	args = append(args, "--permission-mode", "default")
	if req.Worktree != "" {
		args = append(args, "--add-dir", req.Worktree)
	}
	if len(req.DisallowedTools) > 0 {
		disallowed := sortedCopy(req.DisallowedTools)
		args = append(args, "--disallowedTools", strings.Join(disallowed, " "))
	}
	if req.MaxTurns > 0 {
		args = append(args, "--max-turns", fmt.Sprintf("%d", req.MaxTurns))
	}
	if req.Model != "" {
		args = append(args, "--model", req.Model)
	}
	return args
}

// relativizeSessionPaths converts each path to root-relative when it is
// under root (an absolute path a tool reported), sorts, and dedupes — the
// "sorted, unique, repo-relative" contract types.go documents for
// FilesRead/FilesEdited. A path already relative, or outside root, is kept
// as-is: read_source's paths are already repo-relative and a path a tool
// reported outside the worktree (rare, but not impossible) should not be
// silently discarded.
func relativizeSessionPaths(root string, paths []string) []string {
	if len(paths) == 0 {
		return nil
	}
	seen := map[string]bool{}
	out := make([]string, 0, len(paths))
	for _, p := range paths {
		rel := p
		if filepath.IsAbs(p) {
			if r, err := filepath.Rel(root, p); err == nil && !strings.HasPrefix(r, "..") {
				rel = r
			}
		}
		if !seen[rel] {
			seen[rel] = true
			out = append(out, rel)
		}
	}
	sort.Strings(out)
	return out
}

func safeRole(role string) string {
	if role == "" {
		return "session"
	}
	return role
}

var _ Harness = ClaudeHarness{}
