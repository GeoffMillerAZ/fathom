//go:build harness_maintenance

package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
	"unicode/utf8"

	"github.com/geoffmilleraz/fathom/third_party/agentic-kit/internal/hookeval"
	"github.com/geoffmilleraz/fathom/third_party/agentic-kit/internal/jsonwire"
)

var errHookInput = errors.New("invalid hook input")

type hookNativeInput struct {
	Subject   hookeval.Subject
	SessionID string
	ToolUseID string
}

func hookString(m map[string]json.RawMessage, key string, required bool, max int) (string, error) {
	raw, ok := m[key]
	if !ok {
		if required {
			return "", errHookInput
		}
		return "", nil
	}
	var s string
	if bytes.Equal(raw, []byte("null")) || json.Unmarshal(raw, &s) != nil || len(s) > max || !utf8.ValidString(s) || (required && strings.TrimSpace(s) == "") {
		return "", errHookInput
	}
	return s, nil
}
func hookBool(m map[string]json.RawMessage, key string) (bool, bool, error) {
	raw, ok := m[key]
	if !ok {
		return false, false, nil
	}
	var b bool
	if bytes.Equal(raw, []byte("null")) || json.Unmarshal(raw, &b) != nil {
		return false, true, errHookInput
	}
	return b, true, nil
}
func hookBrief(parts ...string) (string, error) {
	s := strings.Join(parts, "\n")
	if len(s) > 65536 {
		return "", errHookInput
	}
	return s, nil
}
func hookNormalize(raw []byte, host, root string) (hookNativeInput, error) {
	var out hookNativeInput
	if len(raw) == 0 || len(raw) > 1<<20 {
		return out, errHookInput
	}
	m, e := jsonwire.Object(raw)
	if e != nil {
		return out, errHookInput
	}
	event, e := hookString(m, "hook_event_name", true, 64)
	if e != nil || event != "PreToolUse" {
		return out, errHookInput
	}
	out.SessionID, e = hookString(m, "session_id", true, 256)
	if e != nil {
		return out, e
	}
	out.ToolUseID, e = hookString(m, "tool_use_id", true, 256)
	if e != nil {
		return out, e
	}
	cwd, e := hookString(m, "cwd", true, 4096)
	if e != nil || cwd != root {
		return out, errHookInput
	}
	tool, e := hookString(m, "tool_name", true, 128)
	if e != nil {
		return out, e
	}
	if host == "claude" && !oneOf(tool, "Agent", "Task") || host == "codex" && !oneOf(tool, "spawn_agent", "collaborationspawn_agent") {
		return out, errHookInput
	}
	inputRaw, ok := m["tool_input"]
	if !ok {
		return out, errHookInput
	}
	in, e := jsonwire.Object(inputRaw)
	if e != nil {
		return out, errHookInput
	}
	s := hookeval.Subject{Host: host, Operation: "spawn"}
	if host == "claude" {
		resume, e := hookString(in, "resume", false, 256)
		if e != nil {
			return out, e
		}
		if resume != "" {
			s.Operation = "resume"
		}
		s.Model, e = hookString(in, "model", false, 256)
		if e != nil {
			return out, e
		}
		s.Effort, e = hookString(in, "effort", false, 64)
		if e != nil {
			return out, e
		}
		s.AgentType, e = hookString(in, "subagent_type", false, 128)
		if e != nil {
			return out, e
		}
		description, e := hookString(in, "description", false, 65536)
		if e != nil {
			return out, e
		}
		prompt, e := hookString(in, "prompt", false, 65536)
		if e != nil {
			return out, e
		}
		s.Brief, e = hookBrief(description, prompt)
		if e != nil {
			return out, e
		}
		s.Fork = s.AgentType == "fork"
		if s.Operation == "resume" && s.Fork {
			return out, errHookInput
		}
		if _, present, e := hookBool(in, "fork_context"); e != nil || present {
			return out, errHookInput
		}
	} else {
		s.Model, e = hookString(in, "model", false, 256)
		if e != nil {
			return out, e
		}
		s.Effort, e = hookString(in, "reasoning_effort", false, 64)
		if e != nil {
			return out, e
		}
		s.AgentType, e = hookString(in, "agent_type", false, 128)
		if e != nil {
			return out, e
		}
		task, e := hookString(in, "task_name", false, 65536)
		if e != nil {
			return out, e
		}
		msg, e := hookString(in, "message", false, 65536)
		if e != nil {
			return out, e
		}
		parts := []string{task, msg}
		if itemRaw, present := in["items"]; present {
			var items []json.RawMessage
			if bytes.Equal(itemRaw, []byte("null")) || json.Unmarshal(itemRaw, &items) != nil || items == nil || len(items) > 256 {
				return out, errHookInput
			}
			for _, item := range items {
				im, e := jsonwire.Object(item)
				if e != nil {
					return out, errHookInput
				}
				t, e := hookString(im, "text", false, 65536)
				if e != nil {
					return out, e
				}
				parts = append(parts, t)
			}
		}
		s.Brief, e = hookBrief(parts...)
		if e != nil {
			return out, e
		}
		fork, present, e := hookBool(in, "fork_context")
		if e != nil {
			return out, e
		}
		s.ContextFork = fork
		if turns, presentTurns := in["fork_turns"]; presentTurns {
			var text string
			if bytes.Equal(turns, []byte("null")) {
				return out, errHookInput
			}
			if json.Unmarshal(turns, &text) == nil {
				if !oneOf(text, "none", "all") {
					return out, errHookInput
				}
				if present && fork != (text == "all") {
					return out, errHookInput
				}
				s.ContextFork = text == "all"
			} else {
				var n int
				if json.Unmarshal(turns, &n) != nil || n < 0 || n > 10000 {
					return out, errHookInput
				}
				if present && fork != (n > 0) {
					return out, errHookInput
				}
				s.ContextFork = n > 0
			}
		}
		if _, present := in["resume"]; present {
			return out, errHookInput
		}
	}
	out.Subject = s
	return out, nil
}
