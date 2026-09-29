package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"
)

type hostHookInput struct {
	Event  string          `json:"hook_event_name"`
	CWD    string          `json:"cwd"`
	Tool   string          `json:"tool_name"`
	Input  json.RawMessage `json:"tool_input"`
	Source string          `json:"source"`
}

func denyHost(stdout, stderr io.Writer, reason string) int {
	// The same explicit PreToolUse deny envelope works in both hosts. Never
	// emit continue:false here: Codex can fail open on that unsupported field.
	_ = json.NewEncoder(stdout).Encode(map[string]any{"hookSpecificOutput": map[string]any{"hookEventName": "PreToolUse", "permissionDecision": "deny", "permissionDecisionReason": reason}})
	fmt.Fprintln(stderr, reason)
	return 2
}

func hostGuard(root *os.Root, rootPath, host, digest string, stdin io.Reader, stdout, stderr io.Writer) int {
	_, _, policyPath, _ := hostPaths(host)
	raw, exists, err := hostRead(root, policyPath)
	if err != nil || !exists || len(digest) != 64 || hostHash(raw) != digest {
		return denyHost(stdout, stderr, "host policy missing, changed or unreadable")
	}
	var policy hostPolicy
	if err = hostDecode(raw, &policy); err == nil {
		err = validateHostPolicy(policy)
	}
	if err != nil {
		return denyHost(stdout, stderr, "invalid host policy")
	}
	input, err := io.ReadAll(io.LimitReader(stdin, hostMaxBytes+1))
	if err != nil || len(input) > hostMaxBytes {
		return denyHost(stdout, stderr, "hook input unreadable or oversized")
	}
	// Native hosts add fields; preserve forward compatibility without silently
	// accepting malformed/duplicate fields in the supported envelope.
	if err := hostValidateJSON(input); err != nil {
		return denyHost(stdout, stderr, "malformed hook JSON")
	}
	var event hostHookInput
	if json.Unmarshal(input, &event) != nil || event.Event == "" {
		return denyHost(stdout, stderr, "missing hook event")
	}
	if event.CWD == "" || hostParentComponent(event.CWD) {
		return denyHost(stdout, stderr, "missing or noncanonical hook cwd")
	}
	working := event.CWD
	if !filepath.IsAbs(working) {
		working = filepath.Join(rootPath, working)
	}
	rel, err := filepath.Rel(rootPath, filepath.Clean(working))
	if err != nil || (!filepath.IsLocal(rel) && rel != ".") {
		return denyHost(stdout, stderr, "hook cwd is outside the installed project")
	}
	switch event.Event {
	case "PreToolUse":
		if event.Tool == "" {
			return denyHost(stdout, stderr, "missing tool name")
		}
		for _, pattern := range policy.DenyTools {
			if matched, _ := filepath.Match(pattern, event.Tool); matched {
				return denyHost(stdout, stderr, "tool denied by project policy: "+event.Tool)
			}
		}
		if reason := hostPathDenial(root, rootPath, host, working, event, policy); reason != "" {
			return denyHost(stdout, stderr, reason)
		}
		fmt.Fprintln(stdout, "{}")
		return 0
	case "SessionStart":
		if event.Source != "startup" && event.Source != "resume" && event.Source != "compact" {
			return denyHost(stdout, stderr, "unsupported session source")
		}
		_ = json.NewEncoder(stdout).Encode(map[string]any{"hookSpecificOutput": map[string]any{"hookEventName": "SessionStart", "additionalContext": "Agentic-kit project policy loaded. Hooks are partial guardrails; run host doctor before relying on required controls. " + policy.Instructions}})
		return 0
	case "PreCompact", "Stop":
		// Observing this lifecycle does not prove the host preserved instructions
		// or verified completion. Do not manufacture a completion guarantee.
		fmt.Fprintln(stdout, "{}")
		return 0
	default:
		return denyHost(stdout, stderr, "unsupported hook event")
	}
}

func hostPathDenial(root *os.Root, rootPath, currentHost, working string, event hostHookInput, p hostPolicy) string {
	if len(p.DenyPaths) == 0 {
		return ""
	}
	// Opting into path enforcement also protects its governing configuration.
	// A configuration-only/advisory install (no DenyPaths) makes no such claim.
	deniedPaths := append([]string{".agentic-kit", ".claude/settings.json", ".claude/settings.local.json", ".codex/hooks.json", ".codex/config.toml", "CLAUDE.md", "AGENTS.md"}, p.DenyPaths...)
	// Protect every installed guard, including in-project executable names and
	// hardlink aliases of external binaries. Ownership is project-controlled;
	// strict path policy already protects its records from delivered file tools.
	var binaries []os.FileInfo
	for _, host := range []string{"claude", "codex"} {
		_, _, _, ownershipPath := hostPaths(host)
		raw, exists, err := hostRead(root, ownershipPath)
		if err != nil {
			return "guard ownership is unreadable"
		}
		if !exists {
			if host == currentHost {
				return "guard ownership is missing"
			}
			continue
		}
		var own hostOwnership
		if hostDecode(raw, &own) != nil || validateHostOwnership(own, host, rootPath) != nil {
			return "guard ownership is invalid"
		}
		info, err := os.Stat(own.Binary)
		if err != nil || !info.Mode().IsRegular() {
			return "installed guard executable is unavailable"
		}
		binaries = append(binaries, info)
		if rel, err := filepath.Rel(rootPath, own.Binary); err == nil && filepath.IsLocal(rel) {
			deniedPaths = append(deniedPaths, filepath.ToSlash(rel))
		}
	}
	var input map[string]json.RawMessage
	if json.Unmarshal(event.Input, &input) != nil || input == nil {
		return "missing or invalid tool input under path policy"
	}
	var paths []string
	switch event.Tool {
	case "Bash", "exec_command", "shell", "shell_command", "write_stdin":
		return "path policy cannot inspect arbitrary shell/stdin effects; use a scoped file tool or native sandbox"
	case "Read", "Edit", "Write", "MultiEdit", "NotebookEdit":
		var path string
		for _, key := range []string{"file_path", "notebook_path", "path"} {
			if value, ok := input[key]; ok {
				if json.Unmarshal(value, &path) != nil || path == "" {
					return "invalid file path"
				}
				break
			}
		}
		if path == "" {
			return "missing file path"
		}
		paths = append(paths, path)
	case "apply_patch":
		var patch string
		if json.Unmarshal(input["command"], &patch) != nil || patch == "" {
			return "missing apply_patch command"
		}
		if !strings.HasPrefix(patch, "*** Begin Patch\n") || !strings.HasSuffix(strings.TrimSpace(patch), "*** End Patch") {
			return "unsupported patch framing"
		}
		for _, line := range strings.Split(patch, "\n") {
			for _, prefix := range []string{"*** Add File: ", "*** Update File: ", "*** Delete File: ", "*** Move to: "} {
				if strings.HasPrefix(line, prefix) {
					paths = append(paths, strings.TrimPrefix(line, prefix))
				}
			}
		}
		if len(paths) == 0 {
			return "patch has no recognized paths"
		}
	default:
		return "path policy has no verified decoder for tool: " + event.Tool
	}
	for _, path := range paths {
		// The OS follows symlink/.. physically; lexical cleaning would hide it.
		if hostParentComponent(path) {
			return "parent path components require a canonical scoped path"
		}
		absolute := path
		if !filepath.IsAbs(absolute) {
			absolute = filepath.Join(working, path)
		}
		rel, err := filepath.Rel(rootPath, filepath.Clean(absolute))
		if err != nil || !hostRelative(filepath.ToSlash(rel)) {
			return "file path escapes the project or names its root"
		}
		// Reject symlink ancestors, including links whose lexical name would
		// hide a protected target. Absent final paths are legitimate writes.
		if _, err := hostRegularPath(root, filepath.ToSlash(rel)); err != nil {
			return "file path is not a verified regular project path"
		}
		if info, err := root.Stat(rel); err == nil {
			for _, binary := range binaries {
				if os.SameFile(info, binary) {
					return "file path names an installed guard executable"
				}
			}
		}
		for _, denied := range deniedPaths {
			if hostDeniedPath(root, filepath.ToSlash(rel), denied) {
				return "file path denied by project policy: " + denied
			}
		}
	}
	return ""
}

func hostParentComponent(path string) bool {
	for _, component := range strings.Split(filepath.ToSlash(path), "/") {
		if component == ".." {
			return true
		}
	}
	return false
}

func hostDeniedPath(root *os.Root, rel, denied string) bool {
	// Conservative case folding also protects case-insensitive filesystems when
	// the target does not exist yet. Existing aliases (including hardlinks and
	// filesystem Unicode normalization) are compared by identity, not spelling.
	if strings.EqualFold(rel, denied) || (len(rel) > len(denied) && rel[len(denied)] == '/' && strings.EqualFold(rel[:len(denied)], denied)) {
		return true
	}
	protected, err := root.Stat(denied)
	if err != nil {
		return false
	}
	for current := rel; current != "."; current = filepath.Dir(current) {
		if info, err := root.Stat(current); err == nil && os.SameFile(protected, info) {
			return true
		}
	}
	return false
}

func hostValidateJSON(raw []byte) error {
	if len(raw) > hostMaxBytes || !utf8.Valid(raw) {
		return errors.New("JSON exceeds cap")
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	if err := hostJSONValue(d, 0); err != nil {
		return err
	}
	if _, err := d.Token(); err != io.EOF {
		return errors.New("trailing JSON value")
	}
	return nil
}

func hostJSONValue(d *json.Decoder, depth int) error {
	if depth > 64 {
		return errors.New("JSON nesting exceeds 64")
	}
	token, err := d.Token()
	if err != nil {
		return err
	}
	delim, ok := token.(json.Delim)
	if !ok {
		return nil
	}
	if delim != '{' && delim != '[' {
		return errors.New("unexpected JSON delimiter")
	}
	seen := map[string]bool{}
	for d.More() {
		if delim == '{' {
			key, err := d.Token()
			if err != nil {
				return err
			}
			name, ok := key.(string)
			name = strings.ToLower(name)
			if !ok || seen[name] {
				return errors.New("duplicate JSON key")
			}
			seen[name] = true
		}
		if err := hostJSONValue(d, depth+1); err != nil {
			return err
		}
	}
	_, err = d.Token()
	return err
}
