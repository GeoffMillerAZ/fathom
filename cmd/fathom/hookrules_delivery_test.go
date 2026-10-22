package main

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/harness"
)

type ruleDeliveryTransport struct{ rules []any }

func (s *ruleDeliveryTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	if r.Body == nil {
		return &http.Response{StatusCode: 404, Body: io.NopCloser(strings.NewReader("unknown route")), Header: make(http.Header)}, nil
	}
	var req struct {
		Params struct {
			Name string `json:"name"`
		} `json:"params"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)
	var result any
	switch req.Params.Name {
	case "obligations":
		result = map[string]any{"rules": s.rules}
	case "list_services":
		result = map[string]any{"services": []any{}}
	default:
		return &http.Response{StatusCode: 404, Body: io.NopCloser(strings.NewReader("unknown tool")), Header: make(http.Header)}, nil
	}
	inner, _ := json.Marshal(result)
	body, _ := json.Marshal(map[string]any{"result": map[string]any{"content": []any{map[string]string{"type": "text", "text": string(inner)}}}})
	return &http.Response{StatusCode: 200, Body: io.NopCloser(bytes.NewReader(body)), Header: make(http.Header)}, nil
}

func ruleDeliveryEnv(t *testing.T) (string, *ruleDeliveryTransport) {
	t.Helper()
	root := cbHookEnv(t)
	transport := &ruleDeliveryTransport{rules: []any{cbRule("Sort outputs.")}}
	previous := http.DefaultTransport
	http.DefaultTransport = transport
	t.Cleanup(func() { http.DefaultTransport = previous })
	return root, transport
}

func ruleDeliveryPost(t *testing.T, id, root, file, agent, transcript, command string) string {
	t.Helper()
	input := map[string]any{"session_id": "session-a", "cwd": root, "agent_id": agent, "transcript_path": transcript, "tool_name": "Edit", "tool_input": map[string]any{"file_path": file}}
	if id == harness.Codex {
		input["tool_name"] = "apply_patch"
		input["tool_input"] = map[string]any{"command": "*** Begin Patch\n*** Update File: " + file + "\n@@\n-package pkg\n+package pkg\n*** End Patch"}
	}
	if command != "" {
		input["tool_name"], input["tool_input"] = "Bash", map[string]any{"command": command}
	}
	raw, _ := json.Marshal(input)
	restore := pipeStdin(t, string(raw))
	defer restore()
	var out, stderr bytes.Buffer
	if code := cmdEditHook([]string{"--url", "http://127.0.0.1:1", "--harness", id}, &out, &stderr); code != 0 {
		t.Fatalf("edit exit %d: %s", code, stderr.String())
	}
	if out.Len() == 0 {
		return ""
	}
	var decoded struct {
		AdditionalContext      string `json:"additionalContext"`
		AdditionalContextSnake string `json:"additional_context"`
		HookSpecificOutput     struct {
			AdditionalContext string `json:"additionalContext"`
		} `json:"hookSpecificOutput"`
	}
	if err := json.Unmarshal(out.Bytes(), &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.AdditionalContext != "" {
		return decoded.AdditionalContext
	}
	if decoded.AdditionalContextSnake != "" {
		return decoded.AdditionalContextSnake
	}
	return decoded.HookSpecificOutput.AdditionalContext
}

func TestRuleContextDeliveryAcrossFilesAndChannels(t *testing.T) {
	for _, id := range []string{harness.ClaudeCode, harness.Codex} {
		t.Run(id, func(t *testing.T) {
			root, transport := ruleDeliveryEnv(t)
			firstFile := writeEditedFile(t, root, "pkg/a.go", "package pkg\n")
			secondFile := writeEditedFile(t, root, "pkg/b.go", "package pkg\n")
			if got := ruleDeliveryPost(t, id, root, firstFile, "", "", ""); !strings.Contains(got, "Sort outputs.") {
				t.Fatalf("first delivery: %q", got)
			}
			transport.rules = append(transport.rules, cbRule("Inject clocks."))
			got := ruleDeliveryPost(t, id, root, secondFile, "", "", "")
			if strings.Contains(got, "Sort outputs.") || !strings.Contains(got, "Inject clocks.") || !strings.HasSuffix(got, "(1 rule already shown in this session)") {
				t.Fatalf("per-rule delta: %q", got)
			}
			got = ruleDeliveryPost(t, id, root, secondFile, "", "", "sed -i 's/pkg/pkg/' pkg/b.go")
			if strings.Contains(got, "Sort outputs.") || strings.Contains(got, "Inject clocks.") || !strings.HasSuffix(got, "(2 rules already shown in this session)") {
				t.Fatalf("post-bash repeated rules: %q", got)
			}
		})
	}
}

func TestRuleContextDeliveryIdentityAndReset(t *testing.T) {
	for _, id := range []string{harness.ClaudeCode, harness.Codex} {
		t.Run(id, func(t *testing.T) {
			root, _ := ruleDeliveryEnv(t)
			file := writeEditedFile(t, root, "pkg/a.go", "package pkg\n")
			for _, child := range []string{"", "child-a", "child-b"} {
				agent, transcript := child, ""
				if id == harness.Codex {
					agent, transcript = "", child
				}
				if got := ruleDeliveryPost(t, id, root, file, agent, transcript, ""); !strings.Contains(got, "Sort outputs.") {
					t.Fatalf("new context %q: %q", child, got)
				}
				if got := ruleDeliveryPost(t, id, root, file, agent, transcript, ""); strings.Contains(got, "Sort outputs.") {
					t.Fatalf("repeated context %q: %q", child, got)
				}
			}
			restore := pipeStdin(t, `{"session_id":"session-a","trigger":"auto","cwd":"."}`)
			cmdPreCompactHook([]string{"--harness", id}, io.Discard, io.Discard)
			restore()
			if got := ruleDeliveryPost(t, id, root, file, "", "", ""); !strings.Contains(got, "Sort outputs.") {
				t.Fatalf("compaction did not reset: %q", got)
			}
		})
	}
}

func TestRuleContextDeliveryDisabledAndOtherHarnesses(t *testing.T) {
	for _, id := range []string{harness.ClaudeCode, harness.Codex, harness.GeminiCLI, "copilot", "cursor"} {
		t.Run(id, func(t *testing.T) {
			root, _ := ruleDeliveryEnv(t)
			file := writeEditedFile(t, root, "pkg/a.go", "package pkg\n")
			if id == harness.ClaudeCode || id == harness.Codex {
				writeEditedFile(t, root, ".fathom/config.toml", "[hooks]\ndedupe = false\n")
			}
			for i := 0; i < 2; i++ {
				if got := ruleDeliveryPost(t, id, root, file, "", "", ""); !strings.Contains(got, "Sort outputs.") || strings.Contains(got, "already shown") {
					t.Fatalf("full delivery required: %q", got)
				}
			}
		})
	}
}

func TestRuleContextDeliveryFaultsAndFailedWrite(t *testing.T) {
	root, _ := ruleDeliveryEnv(t)
	file := writeEditedFile(t, root, "pkg/a.go", "package pkg\n")
	cbEdit(t, "http://127.0.0.1:1", root, file, "session-a", harness.ClaudeCode, cbFailedWriter{})
	if got := ruleDeliveryPost(t, harness.ClaudeCode, root, file, "", "", ""); !strings.Contains(got, "Sort outputs.") {
		t.Fatalf("failed stdout recorded delivery: %q", got)
	}
	f, _, err := openHookDedupe("session-a")
	if err != nil {
		t.Fatal(err)
	}
	if got := ruleDeliveryPost(t, harness.ClaudeCode, root, file, "", "", ""); !strings.Contains(got, "Sort outputs.") {
		t.Fatalf("lock contention withheld rules: %q", got)
	}
	f.Close()
	if err := os.WriteFile(cbDedupePath("session-a"), []byte("invalid state"), 0o600); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if got := ruleDeliveryPost(t, harness.ClaudeCode, root, file, "", "", ""); !strings.Contains(got, "Sort outputs.") {
			t.Fatalf("state fault withheld rules: %q", got)
		}
	}
}

func TestRuleContextDeliveryStableClaimAndRawIdentity(t *testing.T) {
	root, transport := ruleDeliveryEnv(t)
	file := writeEditedFile(t, root, "pkg/a.go", "package pkg\n")
	now := time.Date(2026, 10, 22, 12, 0, 0, 0, time.UTC)
	previous := nowFunc
	nowFunc = func() time.Time { return now }
	t.Cleanup(func() { nowFunc = previous })
	claim := cbRule("Sort outputs.")
	claim["claim_id"] = "rule-one"
	transport.rules = []any{claim, cbRule("Inject clocks.")}
	if got := ruleDeliveryPost(t, harness.ClaudeCode, root, file, "", "", ""); !strings.Contains(got, "Sort outputs.") || !strings.Contains(got, "Inject clocks.") {
		t.Fatalf("first rules: %q", got)
	}
	claim["rule"] = "Always sort outputs."
	transport.rules = []any{claim, cbRule("Always inject clocks.")}
	got := ruleDeliveryPost(t, harness.ClaudeCode, root, file, "", "", "")
	if !strings.Contains(got, "Always sort outputs.") || !strings.Contains(got, "Always inject clocks.") {
		t.Fatalf("claim/raw identity: %q", got)
	}
	// ADR-080 d3 keeps CB-1's refresh backstop for each rule identity.
	now = now.Add(30 * time.Minute)
	if got := ruleDeliveryPost(t, harness.ClaudeCode, root, file, "", "", ""); !strings.Contains(got, "Always sort outputs.") || !strings.Contains(got, "Always inject clocks.") || strings.Contains(got, "already shown") {
		t.Fatalf("rule identities did not refresh: %q", got)
	}
}

func TestRuleContextDeliveryUnchangedPayload(t *testing.T) {
	for _, id := range []string{harness.ClaudeCode, harness.Codex} {
		for _, channel := range []string{"post-edit", "post-bash"} {
			t.Run(id+"/"+channel, func(t *testing.T) {
				root, _ := ruleDeliveryEnv(t)
				file := writeEditedFile(t, root, "pkg/a.go", "package pkg\n")
				command := ""
				if channel == "post-bash" {
					command = "sed -i 's/pkg/pkg/' pkg/a.go"
				}
				first := ruleDeliveryPost(t, id, root, file, "", "", command)
				if !strings.Contains(first, "Sort outputs.") {
					t.Fatalf("first delivery: %q", first)
				}
				if got := ruleDeliveryPost(t, id, root, file, "", "", command); got != "" {
					t.Fatalf("CB-1 unchanged payload repeated: %q", got)
				}
			})
		}
	}
}

func TestRuleContextDeliveryRefreshBackstops(t *testing.T) {
	for _, id := range []string{harness.ClaudeCode, harness.Codex} {
		for _, backstop := range []string{"30-minutes", "10-withholdings"} {
			t.Run(id+"/"+backstop, func(t *testing.T) {
				root, transport := ruleDeliveryEnv(t)
				firstFile := writeEditedFile(t, root, "pkg/a.go", "package pkg\n")
				secondFile := writeEditedFile(t, root, "pkg/b.go", "package pkg\n")
				now := time.Date(2026, 10, 22, 12, 0, 0, 0, time.UTC)
				previous := nowFunc
				nowFunc = func() time.Time { return now }
				t.Cleanup(func() { nowFunc = previous })
				emit := func(file, command string) string {
					return ruleDeliveryPost(t, id, root, file, "", "", command)
				}
				if got := emit(firstFile, ""); !strings.Contains(got, "Sort outputs.") {
					t.Fatalf("first rule: %q", got)
				}
				// A later payload delivers one new rule without refreshing the old one.
				now = now.Add(15 * time.Minute)
				transport.rules = append(transport.rules, cbRule("Inject clocks."))
				got := emit(secondFile, "")
				if strings.Contains(got, "Sort outputs.") || !strings.Contains(got, "Inject clocks.") || !strings.HasSuffix(got, "(1 rule already shown in this session)") {
					t.Fatalf("different-file delta: %q", got)
				}
				if backstop == "30-minutes" {
					now = now.Add(15*time.Minute - time.Nanosecond)
					if got := emit(secondFile, ""); got != "" {
						t.Fatalf("whole payload repeated before refresh: %q", got)
					}
					now = now.Add(time.Nanosecond)
				} else {
					// Count both whole-payload suppressions and cross-channel deltas.
					for i := 2; i <= 10; i++ {
						command := ""
						if i%2 == 0 {
							command = "sed -i 's/pkg/pkg/' pkg/b.go"
						}
						got := emit(secondFile, command)
						if strings.Contains(got, "Sort outputs.") || strings.Contains(got, "Inject clocks.") || (got != "" && !strings.HasSuffix(got, "(2 rules already shown in this session)")) {
							t.Fatalf("withholding %d: %q", i, got)
						}
					}
				}
				got = emit(secondFile, "")
				if !strings.Contains(got, "Sort outputs.") || strings.Contains(got, "Inject clocks.") || !strings.HasSuffix(got, "(1 rule already shown in this session)") {
					t.Fatalf("per-rule backstop: %q", got)
				}
				if backstop == "10-withholdings" {
					got = emit(firstFile, "")
					if strings.Contains(got, "Sort outputs.") || !strings.Contains(got, "Inject clocks.") || !strings.HasSuffix(got, "(1 rule already shown in this session)") {
						t.Fatalf("independent rule counter: %q", got)
					}
				}
			})
		}
	}
}

func TestRuleContextRecordedCodexShape(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "internal", "harness", "testdata", "codecs", "codex", "post-tool.live.in.json"))
	if err != nil {
		t.Fatal(err)
	}
	in, err := harness.CodexCodec().DecodeInput(harness.EventPostTool, raw)
	if err != nil {
		t.Fatal(err)
	}
	if in.SessionID == "" || in.TranscriptPath == "" || in.AgentID != "" {
		t.Fatalf("recorded Codex context fields: %+v", in)
	}
	descriptor, _ := harness.Lookup(harness.Codex)
	binding, _ := descriptor.Binding(harness.EventPostTool)
	if binding.Input["session_id"] == "" || binding.Input["transcript_path"] == "" || binding.Input["agent_id"] != "" {
		t.Fatalf("Codex descriptor context: %+v", binding.Input)
	}
	for _, field := range []string{`null`, `""`} {
		in, err := harness.CodexCodec().DecodeInput(harness.EventPostTool, []byte(`{"session_id":"session-a","transcript_path":`+field+`}`))
		if err != nil || in.TranscriptPath != "" {
			t.Fatalf("nullable transcript: %+v, %v", in, err)
		}
	}
}
