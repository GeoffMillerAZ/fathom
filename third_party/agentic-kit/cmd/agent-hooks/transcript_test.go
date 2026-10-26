package main

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func writeFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestClaudeTranscriptUsageCountsEachMessageOnce(t *testing.T) {
	path := writeFile(t, t.TempDir(), "agent.jsonl", strings.Join([]string{
		`{"type":"user","message":{"role":"user","content":"go"}}`,
		`{"type":"assistant","message":{"id":"m1","model":"claude-haiku-5-5","usage":{"input_tokens":10,"cache_read_input_tokens":100,"output_tokens":5}}}`,
		`{"type":"assistant","message":{"id":"m1","model":"claude-haiku-5-5","usage":{"input_tokens":10,"cache_read_input_tokens":100,"output_tokens":20}}}`,
		`not json`,
		`{"type":"assistant","message":{"id":"m2","model":"claude-haiku-5-5","usage":{"input_tokens":1,"cache_creation_input_tokens":50,"output_tokens":9}}}`,
	}, "\n"))
	u, ok := readTranscriptUsage(hostClaude, path)
	if !ok || u.Model != "claude-haiku-5-5" || u.Tokens != 130+60 {
		t.Fatalf("usage = %+v ok=%v, want 190 tokens (m1 counted once at its largest)", u, ok)
	}
}

func TestCodexTranscriptUsageTakesTheLastCumulativeTotal(t *testing.T) {
	path := writeFile(t, t.TempDir(), "rollout.jsonl", strings.Join([]string{
		`{"type":"session_meta","payload":{}}`,
		`{"type":"turn_context","payload":{"model":"cx-small","effort":"low"}}`,
		`{"type":"event_msg","payload":{"type":"token_count","info":{"total_token_usage":{"total_tokens":100}}}}`,
		`{"type":"event_msg","payload":{"type":"token_count","info":null}}`,
		`{"type":"event_msg","payload":{"type":"token_count","info":{"total_token_usage":{"total_tokens":450}}}}`,
	}, "\n"))
	u, ok := readTranscriptUsage(hostCodex, path)
	if !ok || u.Model != "cx-small" || u.Tokens != 450 {
		t.Fatalf("usage = %+v ok=%v", u, ok)
	}
}

func TestTranscriptUsageRefusesUnsafePaths(t *testing.T) {
	dir := t.TempDir()
	for name, path := range map[string]string{
		"empty":     "",
		"relative":  "agent.jsonl",
		"missing":   filepath.Join(dir, "absent.jsonl"),
		"directory": dir,
	} {
		if _, ok := readTranscriptUsage(hostClaude, path); ok {
			t.Errorf("%s: read usage from %q", name, path)
		}
	}
}

func TestSubagentStopDefersTokenReadingToReport(t *testing.T) {
	ta := newTestApp(t)
	path := filepath.Join(ta.dir, "agent.jsonl")
	ta.hook(t, hostClaude, map[string]any{
		"hook_event_name": "SubagentStop", "session_id": "s", "agent_id": "a1", "agent_type": "cavecrew-investigator",
		"agent_transcript_path": path, "last_assistant_message": "src/x.go:1",
	})
	r := ta.ledger(t)[0]
	if r.Kind != kindSubagentStop || r.Transcript != path || r.TotalTokens != 0 || r.ReportChars != 10 {
		t.Fatalf("ledger = %+v (the hook must store the path, not read a file the host is still writing)", r)
	}
	ta.out.Reset()
	ta.run([]string{"report"})
	if !strings.Contains(ta.out.String(), "unknown spend (transcript gone or unreadable): 1") {
		t.Fatalf("a missing transcript must count as unknown spend:\n%s", ta.out.String())
	}
	writeFile(t, ta.dir, "agent.jsonl", `{"type":"assistant","message":{"id":"m1","model":"claude-haiku-5-5","usage":{"input_tokens":3,"output_tokens":4}}}`)
	ta.out.Reset()
	ta.run([]string{"report"})
	if !regexp.MustCompile(`claude-haiku-5-5\s+7`).MatchString(ta.out.String()) {
		t.Fatalf("report did not read the finished transcript:\n%s", ta.out.String())
	}
}
