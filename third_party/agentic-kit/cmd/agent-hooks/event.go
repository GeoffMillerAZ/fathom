package main

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"unicode/utf8"
)

// Hook event names shared by both hosts.
const (
	eventPreToolUse   = "PreToolUse"
	eventPostToolUse  = "PostToolUse"
	eventSubagentStop = "SubagentStop"
)

// hookInput is the subset of the host hook payload this binary reads. Both
// hosts use the same field names for these.
type hookInput struct {
	Event        string
	SessionID    string
	CWD          string
	Model        string // Codex: the session's model
	ToolName     string
	ToolUseID    string
	ToolInput    json.RawMessage
	ToolResponse json.RawMessage
	AgentID      string // set inside subagents (Claude Code), on SubagentStop (both)
	AgentType    string
	LastMessage  string
	// AgentTranscriptPath is the finished subagent's transcript (SubagentStop).
	AgentTranscriptPath string
}

// decodeHookInput reads the payload member by member. A field this binary
// uses that has an unexpected type reads as empty; a field it does not use is
// never decoded at all. So a host upgrade that adds fields or retypes one it
// does not need cannot make a spawn unreadable and slip past the gate.
func decodeHookInput(raw []byte) (hookInput, error) {
	m, err := rawObject(raw)
	if err != nil {
		return hookInput{}, err
	}
	return hookInput{
		Event:               rawString(m, "hook_event_name"),
		SessionID:           rawString(m, "session_id"),
		CWD:                 rawString(m, "cwd"),
		Model:               rawString(m, "model"),
		ToolName:            rawString(m, "tool_name"),
		ToolUseID:           rawString(m, "tool_use_id"),
		ToolInput:           m["tool_input"],
		ToolResponse:        m["tool_response"],
		AgentID:             rawString(m, "agent_id"),
		AgentType:           rawString(m, "agent_type"),
		LastMessage:         rawString(m, "last_assistant_message"),
		AgentTranscriptPath: rawString(m, "agent_transcript_path"),
	}, nil
}

// spawnRequest is a subagent spawn normalized across hosts.
type spawnRequest struct {
	Host      string
	Model     string
	Effort    string
	AgentType string
	// DefinedModel is the model the named agent's own definition pins, when
	// the hook could read one. It makes a no-model deny name the right model.
	DefinedModel string
	// Fork is true for a Claude Code fork, which inherits the session model
	// and cannot take a model. Codex forks (fork_context, fork_turns) carry
	// context but still take a model, so they set ContextFork instead.
	Fork        bool
	ContextFork bool
	// Brief is the text the classifier and markers read: the prompt and its
	// description (Claude) or the task name, message and text items (Codex).
	Brief string
}

// codexSpawnTools are the names Codex gives its spawn tool in hook payloads.
// The documentation says spawn_agent; Codex 0.163 actually sends the
// namespaced collaborationspawn_agent (observed in a live session,
// 2026-10-19). The hook registration's matcher must name both.
var codexSpawnTools = []string{"spawn_agent", "collaborationspawn_agent"}

// isSpawnTool reports whether a tool name is the host's subagent spawn.
func isSpawnTool(host, tool string) bool {
	if host == hostCodex {
		return contains(codexSpawnTools, tool)
	}
	return tool == "Agent" || tool == "Task" // Task: the pre-2.1.63 name
}

// isShellTool reports whether a tool runs shell commands. Codex reports its
// shell tools under several names; its "Bash" hook matcher covers them all.
func isShellTool(host, tool string) bool {
	if host == hostCodex {
		switch tool {
		case "Bash", "shell", "exec_command", "local_shell", "unified_exec":
			return true
		}
		return false
	}
	return tool == "Bash"
}

// parseSpawn normalizes a spawn tool input. ok is false when the input is not
// a JSON object; callers then fail open. Members are read leniently, like the
// payload itself.
func parseSpawn(host string, raw json.RawMessage) (spawnRequest, bool) {
	m, err := rawObject(raw)
	if err != nil {
		return spawnRequest{}, false
	}
	if host == hostCodex {
		brief := []string{rawString(m, "task_name"), rawString(m, "message")}
		var items []json.RawMessage
		_ = json.Unmarshal(m["items"], &items)
		for _, item := range items {
			if im, err := rawObject(item); err == nil {
				brief = append(brief, rawString(im, "text"))
			}
		}
		var forkContext bool
		_ = json.Unmarshal(m["fork_context"], &forkContext)
		return spawnRequest{
			Host:        host,
			Model:       strings.TrimSpace(rawString(m, "model")),
			Effort:      strings.TrimSpace(rawString(m, "reasoning_effort")),
			AgentType:   rawString(m, "agent_type"),
			ContextFork: forkContext || forksTurns(m["fork_turns"]),
			Brief:       strings.Join(brief, "\n"),
		}, true
	}
	agentType := rawString(m, "subagent_type")
	return spawnRequest{
		Host:      host,
		Model:     strings.TrimSpace(rawString(m, "model")),
		Effort:    strings.TrimSpace(rawString(m, "effort")),
		AgentType: agentType,
		Fork:      agentType == "fork",
		Brief:     rawString(m, "description") + "\n" + rawString(m, "prompt"),
	}, true
}

// forksTurns reports whether a Codex v2 fork_turns value carries parent
// turns. "none", empty, null and 0 do not.
func forksTurns(raw json.RawMessage) bool {
	var s string
	if json.Unmarshal(raw, &s) == nil {
		s = strings.ToLower(strings.TrimSpace(s))
		return s != "" && s != "none" && s != "0"
	}
	var n float64
	if json.Unmarshal(raw, &n) == nil {
		return n > 0
	}
	return false
}

// agentResult is the subset of Claude Code's Agent tool_response measured in
// the ledger. Codex's spawn_agent returns only a handle; its report arrives
// at SubagentStop.
type agentResult struct {
	Status          string          `json:"status"`
	AgentID         string          `json:"agentId"`
	ResolvedModel   string          `json:"resolvedModel"`
	TotalTokens     int64           `json:"totalTokens"`
	TotalDurationMs int64           `json:"totalDurationMs"`
	ToolUseCount    int64           `json:"totalToolUseCount"`
	Content         json.RawMessage `json:"content"`
}

// textChars counts the characters in every string leaf of a JSON value, a
// shape-independent measure of how much text a tool result put in context.
// Malformed JSON counts as its raw length.
func textChars(raw json.RawMessage) int {
	if len(raw) == 0 {
		return 0
	}
	var v any
	if json.Unmarshal(raw, &v) != nil {
		return utf8.RuneCount(raw)
	}
	return countChars(v, 0)
}

func countChars(v any, depth int) int {
	if depth > 64 {
		return 0
	}
	switch t := v.(type) {
	case string:
		return utf8.RuneCountInString(t)
	case []any:
		n := 0
		for _, e := range t {
			n += countChars(e, depth+1)
		}
		return n
	case map[string]any:
		n := 0
		for _, e := range t {
			n += countChars(e, depth+1)
		}
		return n
	}
	return 0
}

// commandHead labels a shell command by its program, never by its arguments,
// so the ledger can rank output sinks without storing command text (which
// may hold secrets). Words are split quote-aware, so an assignment such as
// PASSWORD='a b c' is one word and is skipped whole; a leading "cd dir &&",
// wrappers such as sudo and env, and their flags are skipped too.
func commandHead(raw json.RawMessage) string {
	m, err := rawObject(raw)
	if err != nil {
		return ""
	}
	cmd := rawString(m, "command")
	if cmd == "" {
		var argv []string // some hosts send argv
		if json.Unmarshal(m["command"], &argv) != nil || len(argv) == 0 {
			return ""
		}
		return programName(argv[0])
	}
	words := shellWords(cmd, 64)
	for i := 0; i < len(words); i++ {
		w := words[i]
		switch {
		case isAssignment(w):
			continue
		case w == "cd" || w == "pushd":
			for i < len(words) && words[i] != "&&" && words[i] != ";" {
				i++
			}
			continue
		case w == "&&" || w == ";" || w == "sudo" || w == "env" || w == "time" || w == "command" || w == "exec" || w == "nice":
			continue
		case strings.HasPrefix(w, "-"):
			continue // a wrapper's flag, e.g. sudo -n
		}
		return programName(w)
	}
	return ""
}

// isAssignment reports whether a shell word is NAME=value.
func isAssignment(w string) bool {
	name, _, ok := strings.Cut(w, "=")
	if !ok || name == "" {
		return false
	}
	for i, r := range name {
		if !(r == '_' || r >= 'A' && r <= 'Z' || r >= 'a' && r <= 'z' || i > 0 && r >= '0' && r <= '9') {
			return false
		}
	}
	return true
}

// programName is the base name of a program word, capped.
func programName(w string) string {
	head := filepath.Base(w)
	if len(head) > 32 {
		head = head[:32]
	}
	return head
}
