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
	mcpServers := req.Arm.MCP
	if mcpServers == nil {
		mcpServers = map[string]any{}
	}
	if err := writeJSONFile(mcpPath, map[string]any{"mcpServers": mcpServers}); err != nil {
		return SessionResult{}, fmt.Errorf("claude harness: %w", err)
	}

	streamPath := filepath.Join(req.RawDir, fmt.Sprintf("session-%d-%s.stream.jsonl", req.Index, safeRole(req.Role)))
	args := buildClaudeArgs(req, mcpPath)

	cmd := exec.Command(h.bin(), args...)
	cmd.Dir = req.Worktree
	cmd.Env = sessionEnv(req.Env)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	runErr := cmd.Run()
	if err := os.WriteFile(streamPath, stdout.Bytes(), 0o644); err != nil {
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
	if runErr != nil && result.Error == "" {
		result.Error = fmt.Sprintf("claude -p exited non-zero: %v", runErr)
	}
	return result, nil
}

// buildClaudeArgs is the pure part of RunSession: the CLI flags for one
// session, given the request and a resolved MCP config path. Kept
// separate from process execution so it is unit-testable with no spend.
func buildClaudeArgs(req SessionRequest, mcpConfigPath string) []string {
	args := []string{
		"-p", req.Prompt,
		"--output-format", "stream-json",
		"--verbose",
		"--mcp-config", mcpConfigPath,
		"--strict-mcp-config",
	}
	if req.SystemAppend != "" {
		args = append(args, "--append-system-prompt", req.SystemAppend)
	}
	if len(req.Arm.AllowedTools) > 0 {
		allowed := sortedCopy(req.Arm.AllowedTools)
		args = append(args, "--allowedTools", strings.Join(allowed, " "))
	}
	if len(req.Arm.DisallowedTools) > 0 {
		disallowed := sortedCopy(req.Arm.DisallowedTools)
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

// sessionEnv merges extra into the current environment, sorted so the
// child process's environment is deterministic given the same input map
// (map iteration order otherwise would not be).
func sessionEnv(extra map[string]string) []string {
	env := os.Environ()
	if len(extra) == 0 {
		return env
	}
	keys := make([]string, 0, len(extra))
	for k := range extra {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		env = append(env, k+"="+extra[k])
	}
	return env
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
