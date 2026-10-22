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
	"unicode/utf8"

	"github.com/geoffmilleraz/fathom/internal/harness"
)

func ad12Spawn(t *testing.T, repo, tool, harnessID string) string {
	t.Helper()
	raw, _ := json.Marshal(map[string]any{"session_id": "sess-parent", "cwd": repo, "tool_name": tool, "tool_input": map[string]any{"prompt": "Find the handler", "subagent_type": "explorer", "custom": map[string]any{"keep": true}}})
	restore := pipeStdin(t, string(raw))
	defer restore()
	var out bytes.Buffer
	if code := cmdEditHook([]string{"--pre", "--harness", harnessID}, &out, io.Discard); code != 0 {
		t.Fatalf("hook exited %d", code)
	}
	return out.String()
}

func TestAD12SpawnAppendsFactsPreservingInputAndPermission(t *testing.T) {
	repo := hookMapRepo(t)
	oldNow := nowFunc
	nowFunc = func() time.Time { return time.Date(2026, 10, 22, 12, 1, 0, 0, time.UTC) }
	t.Cleanup(func() { nowFunc = oldNow })
	now := time.Date(2026, 10, 22, 12, 0, 0, 0, time.UTC)
	fileMapEntry(t, repo, "live", "g1.go#L3-L3", "the first handler", "sess-parent", now.Add(-time.Minute))
	fileMapEntry(t, repo, "live", "g2.go#L5-L5", "the newest handler", "sess-parent", now)
	for _, tool := range []string{"Agent", "Task"} {
		got := ad12Spawn(t, repo, tool, harness.ClaudeCode)
		var out struct {
			Specific struct {
				Updated    map[string]json.RawMessage `json:"updatedInput"`
				Permission json.RawMessage            `json:"permissionDecision"`
			} `json:"hookSpecificOutput"`
		}
		if err := json.Unmarshal([]byte(got), &out); err != nil {
			t.Fatalf("%s output %q: %v", tool, got, err)
		}
		var prompt string
		_ = json.Unmarshal(out.Specific.Updated["prompt"], &prompt)
		if !strings.HasPrefix(prompt, "Find the handler\n") || !strings.Contains(prompt, "func G() {}") || !strings.Contains(prompt, "why: the newest handler") {
			t.Errorf("%s missing facts: %q", tool, prompt)
		}
		if strings.Index(prompt, "g2.go") > strings.Index(prompt, "g1.go") {
			t.Error("entries must be newest first")
		}
		if len(prompt)-len("Find the handler\n\n") > 1500 {
			t.Errorf("block over byte cap: %d", len(prompt))
		}
		if string(out.Specific.Updated["subagent_type"]) != `"explorer"` || string(out.Specific.Updated["custom"]) != `{"keep":true}` || len(out.Specific.Permission) != 0 {
			t.Errorf("input or permission changed: %s", got)
		}
	}
	var out, errs bytes.Buffer
	if code := cmdMapShow([]string{"live", "--brief", "--repo", repo}, &out, &errs); code != 0 {
		t.Fatalf("brief: %s", errs.String())
	}
	if !strings.Contains(out.String(), "func G() {}") || !strings.Contains(out.String(), "why: the newest handler") || len(out.String()) > 1500 {
		t.Errorf("CLI must share facts and cap: %q", out.String())
	}
}

func TestAD12SpawnNoMapAndUnsupportedHarnessLeaveInputUntouched(t *testing.T) {
	repo := hookMapRepo(t)
	oldNow := nowFunc
	nowFunc = func() time.Time { return time.Date(2026, 10, 22, 12, 1, 0, 0, time.UTC) }
	t.Cleanup(func() { nowFunc = oldNow })
	if got := ad12Spawn(t, repo, "Agent", harness.ClaudeCode); got != "" {
		t.Errorf("no map rewrote input: %q", got)
	}
	now := time.Date(2026, 10, 22, 12, 0, 0, 0, time.UTC)
	fileMapEntry(t, repo, "live", "g1.go#L3-L3", "handler", "sess-parent", now)
	if got := ad12Spawn(t, repo, "Agent", "codex"); got != "" {
		t.Errorf("unsupported harness rewrote input: %q", got)
	}
	fileMapEntry(t, repo, "other", "g2.go#L5-L5", "foreign session", "different-parent", now)
	t.Setenv("CLAUDE_CODE_SESSION_ID", "sess-parent")
	if got := ad12Spawn(t, repo, "Agent", harness.ClaudeCode); strings.Contains(got, "foreign session") {
		t.Errorf("other session leaked: %s", got)
	}
}

func TestAD12ResumedParentDoesNotCloseNewMaps(t *testing.T) {
	repo := hookMapRepo(t)
	oldNow := nowFunc
	nowFunc = func() time.Time { return time.Date(2026, 10, 22, 12, 1, 0, 0, time.UTC) }
	t.Cleanup(func() { nowFunc = oldNow })
	now := time.Date(2026, 10, 22, 12, 0, 0, 0, time.UTC)
	fileMapEntry(t, repo, "before", "g1.go#L3-L3", "before end", "sess-parent", now.Add(-2*time.Minute))
	if err := appendThreadItem(threadItem{SchemaVersion: threadSchemaVersion, ID: "end-parent", Kind: "session-end", FiledAt: now.Add(-time.Minute).Format(time.RFC3339), Pointers: map[string]string{"session_id": "sess-parent"}}); err != nil {
		t.Fatal(err)
	}
	fileMapEntry(t, repo, "resumed", "g2.go#L5-L5", "after resume", "sess-parent", now)
	set := loadMapSet(repo, now)
	if set.maps["before"].EndedAt.IsZero() {
		t.Error("prior map must close")
	}
	if !set.maps["resumed"].EndedAt.IsZero() {
		t.Error("earlier end note closed resumed parent's new map")
	}
	if got := ad12Spawn(t, repo, "Agent", harness.ClaudeCode); !strings.Contains(got, "after resume") || strings.Contains(got, "before end") {
		t.Errorf("wrong live map delivered: %s", got)
	}
}

func TestAD12SubagentStartKeepsLiveParentMapWithoutDiscoveries(t *testing.T) {
	repo := hookMapRepo(t)
	now := time.Now().UTC()
	writeSessionStamp(sessionStamp{SessionID: "sess-parent", Cwd: repo, Identity: &identityResult{Schema: 1, Service: "fathom", Symbols: 10}})
	fileMapEntry(t, repo, "before", "g1.go#L3-L3", "before end", "sess-parent", now.Add(-2*time.Minute))
	if err := appendThreadItem(threadItem{SchemaVersion: threadSchemaVersion, ID: "end-parent", Kind: "session-end", FiledAt: now.Add(-time.Minute).Format(time.RFC3339), Pointers: map[string]string{"session_id": "sess-parent"}}); err != nil {
		t.Fatal(err)
	}
	fileMapEntry(t, repo, "resumed", "g2.go#L5-L5", "after resume", "sess-parent", now)
	fileMapEntry(t, repo, "foreign", "g3.go#L3-L3", "another parent", "sess-other", now)
	var out, errb bytes.Buffer
	payload, err := json.Marshal(map[string]string{"session_id": "sess-parent", "cwd": repo, "agent_id": "agent-7"})
	if err != nil {
		t.Fatal(err)
	}
	if code := cmdSessionHookEvent("subagent-start", []string{"--url", "http://127.0.0.1:1", "--dir", repo}, bytes.NewReader(payload), &out, &errb); code != 0 {
		t.Fatalf("exit %d: %s", code, errb.String())
	}
	primer := hookContext(t, out.String())
	if len(primer) > subagentDiscoveriesPrimerBudget || strings.Count(primer, subagentMapLead) != 1 {
		t.Fatalf("want one map inside the primer cap, got %d bytes: %s", len(primer), primer)
	}
	for _, want := range []string{"fathom map show resumed --budget 1200", "--agent agent-7"} {
		if !strings.Contains(primer, want) {
			t.Errorf("primer lacks %q: %s", want, primer)
		}
	}
	for _, unwanted := range []string{"exploration map before", "exploration map foreign", subagentBriefLead} {
		if strings.Contains(primer, unwanted) {
			t.Errorf("primer contains %q: %s", unwanted, primer)
		}
	}
}

func TestAD12MapBriefClipsUTF8AndDropsChangedRanges(t *testing.T) {
	repo := hookMapRepo(t)
	oldNow := nowFunc
	now := time.Date(2026, 10, 22, 12, 0, 0, 0, time.UTC)
	nowFunc = func() time.Time { return now }
	t.Cleanup(func() { nowFunc = oldNow })
	body := "// " + strings.Repeat("界", 2000) + "\n"
	if err := os.WriteFile(filepath.Join(repo, "long.go"), []byte(body), 0600); err != nil {
		t.Fatal(err)
	}
	fileMapEntry(t, repo, "long", "long.go#L1-L1", strings.Repeat("why ", 100), "sess-parent", now)
	got := parentMapBrief(repo, "sess-parent", now)
	if len(got) > 1500 || !utf8.ValidString(got) || !strings.Contains(got, "[excerpt clipped]") || !strings.Contains(got, "why:") {
		t.Errorf("invalid capped facts (%d): %q", len(got), got)
	}
	if err := os.WriteFile(filepath.Join(repo, "long.go"), []byte("changed\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if got := parentMapBrief(repo, "sess-parent", now); got != "" {
		t.Errorf("changed range served: %q", got)
	}
}

func TestAD12MalformedSpawnLeavesInputUntouchedAndLogsReason(t *testing.T) {
	repo := hookMapRepo(t)
	raw, _ := json.Marshal(map[string]any{"session_id": "sess-parent", "cwd": repo, "tool_name": "Agent", "tool_input": "invalid"})
	restore := pipeStdin(t, string(raw))
	defer restore()
	var out bytes.Buffer
	if code := cmdEditHook([]string{"--pre"}, &out, io.Discard); code != 0 || out.Len() != 0 {
		t.Errorf("malformed spawn changed input: exit %d, %q", code, out.String())
	}
	data, _ := os.ReadFile(os.Getenv("FATHOM_HOOKS_LOG"))
	if !bytes.Contains(data, []byte("subagent-map")) || !bytes.Contains(data, []byte("invalid-input")) {
		t.Fatalf("missing skip reason: %s", data)
	}
}
