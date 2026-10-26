package main

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestParseSpawnClaude(t *testing.T) {
	r, ok := parseSpawn(hostClaude, json.RawMessage(`{"description":"find","prompt":"the loader","subagent_type":"fork","model":" haiku ","effort":"low","future_field":1}`))
	if !ok || r.Model != "haiku" || r.Effort != "low" || !r.Fork || r.AgentType != "fork" || r.Brief != "find\nthe loader" {
		t.Fatalf("parsed = %+v ok=%v", r, ok)
	}
	if _, ok := parseSpawn(hostClaude, json.RawMessage(`"not an object"`)); ok {
		t.Fatal("a non-object input must not parse")
	}
}

func TestParseSpawnCodex(t *testing.T) {
	tests := []struct {
		input       string
		wantFork    bool
		wantBriefIn string
	}{
		{`{"task_name":"vault","message":"find it","fork_turns":"none","model":"m","reasoning_effort":"low"}`, false, "vault\nfind it"},
		{`{"message":"x","fork_turns":"all"}`, true, "x"},
		{`{"message":"x","fork_turns":3}`, true, "x"},
		{`{"message":"x","fork_turns":0}`, false, "x"},
		{`{"message":"x","fork_context":true}`, true, "x"},
		{`{"items":[{"type":"text","text":"from items"},{"type":"image","url":"u"}]}`, false, "from items"},
	}
	for _, tt := range tests {
		r, ok := parseSpawn(hostCodex, json.RawMessage(tt.input))
		if !ok || r.ContextFork != tt.wantFork || r.Fork || !strings.Contains(r.Brief, tt.wantBriefIn) {
			t.Errorf("%s: parsed = %+v ok=%v", tt.input, r, ok)
		}
	}
	r, _ := parseSpawn(hostCodex, json.RawMessage(`{"message":"x","model":"m","reasoning_effort":"high","agent_type":"explorer"}`))
	if r.Model != "m" || r.Effort != "high" || r.AgentType != "explorer" {
		t.Fatalf("codex fields = %+v", r)
	}
}

func TestIsSpawnAndShellTool(t *testing.T) {
	for _, c := range []struct {
		host, tool   string
		spawn, shell bool
	}{
		{hostClaude, "Agent", true, false},
		{hostClaude, "Task", true, false},
		{hostClaude, "Bash", false, true},
		{hostClaude, "spawn_agent", false, false},
		{hostCodex, "spawn_agent", true, false},
		{hostCodex, "collaborationspawn_agent", true, false},
		{hostCodex, "collaborationwait_agent", false, false},
		{hostCodex, "Agent", false, false},
		{hostCodex, "exec_command", false, true},
		{hostCodex, "apply_patch", false, false},
	} {
		if got := isSpawnTool(c.host, c.tool); got != c.spawn {
			t.Errorf("isSpawnTool(%s, %s) = %v", c.host, c.tool, got)
		}
		if got := isShellTool(c.host, c.tool); got != c.shell {
			t.Errorf("isShellTool(%s, %s) = %v", c.host, c.tool, got)
		}
	}
}

func TestCommandHead(t *testing.T) {
	for in, want := range map[string]string{
		`{"command":"go test ./..."}`:                 "go",
		`{"command":"FOO=1 BAR=2 /usr/bin/make -j8"}`: "make",
		`{"command":"cd /repo && npm test"}`:          "npm",
		`{"command":"sudo -n true"}`:                  "true",
		`{"command":["bash","-lc","ls"]}`:             "bash",
		`{"command":""}`:                              "",
		`{"other":1}`:                                 "",
		`not json`:                                    "",
	} {
		if got := commandHead(json.RawMessage(in)); got != want {
			t.Errorf("commandHead(%s) = %q, want %q", in, got, want)
		}
	}
}

func TestTextChars(t *testing.T) {
	for in, want := range map[string]int{
		`"héllo"`:                        5,
		`{"a":"xy","b":["z",{"c":"w"}]}`: 4,
		`123`:                            0,
		``:                               0,
		`{bad`:                           4,
	} {
		if got := textChars(json.RawMessage(in)); got != want {
			t.Errorf("textChars(%s) = %d, want %d", in, got, want)
		}
	}
}
