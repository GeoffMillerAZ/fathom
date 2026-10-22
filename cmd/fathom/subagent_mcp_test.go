package main

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Real MCP callers need not carry harness session/agent headers. The completed
// tool hook does, so record the result there rather than guessing its owner.
func TestAD11CompletedMCPLookupsKeepParentIdentity(t *testing.T) {
	for _, tool := range []string{"symbol_context", "find_symbol", "read_source"} {
		t.Run(tool, func(t *testing.T) {
			pinNoEnclosingRepo(t)
			threadEnv(t)
			root := t.TempDir()
			t.Setenv("FATHOM_STATE_DIR", t.TempDir())
			t.Setenv("FATHOM_SESSIONS_LOG", filepath.Join(t.TempDir(), "sessions.jsonl"))
			t.Setenv("FATHOM_OBSERVED_DISCOVERIES_LOG", filepath.Join(t.TempDir(), "observed.jsonl"))
			now := time.Date(2026, 10, 22, 12, 0, 0, 0, time.UTC)
			oldNow := nowFunc
			nowFunc = func() time.Time { return now }
			t.Cleanup(func() { nowFunc = oldNow })
			recordSessionWindow(root, sessionWindowRow{SessionID: "parent", Started: now.Add(-time.Hour).Format(time.RFC3339)})
			payloads := map[string]string{
				"symbol_context": `{"def":{"file_path":"%s","line":2},"snippet":[{"line":2,"text":"func F() {}"}]}`,
				"find_symbol":    `{"symbols":[{"file_path":"%s","line":2,"end_line":2}]}`,
				"read_source":    `{"file":"%s","lines":[{"line":2,"text":"func F() {}"}]}`,
			}
			for _, c := range []struct{ file, session, agent, service, prefix string }{
				{"own.go", "parent", "", "svc", "mcp__fathom__"},
				{"plugin.go", "parent", "", "svc", "mcp__plugin_example_fathom__"},
				{"child.go", "parent", "child", "svc", "mcp__fathom__"},
				{"other.go", "other", "", "svc", "mcp__fathom__"},
				{"foreign.go", "parent", "", "other", "mcp__fathom__"},
				{"AGENTS.md", "parent", "", "svc", "mcp__fathom__"},
			} {
				if err := os.WriteFile(filepath.Join(root, c.file), []byte("package sample\nfunc F() {}\n"), 0600); err != nil {
					t.Fatal(err)
				}
				payload := strings.ReplaceAll(payloads[tool], "%s", c.file)
				raw, _ := json.Marshal(map[string]any{
					"session_id": c.session, "agent_id": c.agent, "cwd": root,
					"tool_name": c.prefix + tool, "tool_input": map[string]string{"service": c.service},
					"tool_response": map[string]any{"content": []any{map[string]string{"type": "text", "text": payload}}},
				})
				restore := pipeStdin(t, string(raw))
				var out bytes.Buffer
				code := cmdEditHook(nil, &out, io.Discard)
				restore()
				if code != 0 || out.Len() != 0 {
					t.Fatalf("observation must be silent: exit %d, %q", code, out.String())
				}
			}
			got := subagentBrief("http://127.0.0.1:1", root, "svc", "parent", 900, now)
			for _, file := range []string{"own.go", "plugin.go"} {
				if !strings.Contains(got, file+"#L2-L2") {
					t.Errorf("parent %s result lost: %q", tool, got)
				}
			}
			for _, file := range []string{"child.go", "other.go", "foreign.go", "AGENTS.md"} {
				if strings.Contains(got, file) {
					t.Errorf("unowned result leaked: %q", got)
				}
			}
			for _, file := range []string{"own.go", "plugin.go"} {
				if err := os.WriteFile(filepath.Join(root, file), []byte("changed\n"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			if got := subagentBrief("http://127.0.0.1:1", root, "svc", "parent", 900, now); got != "" {
				t.Errorf("changed ranges survived: %q", got)
			}
		})
	}
}
