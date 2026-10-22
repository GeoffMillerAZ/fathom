package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/harness"
)

func TestAD11BriefOnlyParentFreshRanges(t *testing.T) {
	pinNoEnclosingRepo(t)
	threadEnv(t)
	root := t.TempDir()
	t.Setenv("FATHOM_STATE_DIR", t.TempDir())
	t.Setenv("FATHOM_SESSIONS_LOG", filepath.Join(t.TempDir(), "sessions.jsonl"))
	t.Setenv("FATHOM_OBSERVED_DISCOVERIES_LOG", filepath.Join(t.TempDir(), "observed.jsonl"))
	now := time.Date(2026, 10, 22, 12, 0, 0, 0, time.UTC)
	recordSessionWindow(root, sessionWindowRow{SessionID: "parent", Started: now.Add(-time.Hour).Format(time.RFC3339)})
	body := []byte("package sample\nfunc F() {}\n")
	hash, _ := core.HashLines(body, 2, 2)
	for _, name := range []string{"own.go", "hook.go", "console.go", "other.go", "child.go", "expired.go", "changed.go", "AGENTS.md", "AGENTS.override.md", "CLAUDE.md", "outside.go"} {
		if err := os.WriteFile(filepath.Join(root, name), body, 0600); err != nil {
			t.Fatal(err)
		}
	}
	type rowCase struct {
		path, origin, session, agent, hash string
		age                                time.Duration
	}
	cases := []rowCase{
		{"own.go", "", "parent", "", hash, -time.Minute},
		{"hook.go", "edit-hook", "parent", "", hash, -time.Minute},
		{"console.go", "console", "parent", "", hash, -time.Minute},
		{"other.go", "", "other", "", hash, -time.Minute},
		{"child.go", "", "parent", "child", hash, -time.Minute},
		{"expired.go", "", "parent", "", hash, -2 * time.Hour},
		{"changed.go", "", "parent", "", "wrong", -time.Minute},
		{"AGENTS.md", "", "parent", "", hash, -time.Minute},
		{"AGENTS.override.md", "", "parent", "", hash, -time.Minute},
		{"CLAUDE.md", "", "parent", "", hash, -time.Minute},
		{"../outside.go", "", "parent", "", hash, -time.Minute},
	}
	var lines []string
	for _, c := range cases {
		raw, err := json.Marshal(map[string]any{"ts": now.Add(c.age).Format(time.RFC3339Nano), "service": "svc", "origin": c.origin, "caller": map[string]string{"session": c.session, "agent_id": c.agent}, "pointers": []string{c.path + "#L2-L2"}, "range_pointers": []map[string]any{{"path": c.path, "start": 2, "end": 2, "hash": c.hash}}})
		if err != nil {
			t.Fatal(err)
		}
		lines = append(lines, string(raw))
	}
	writeDiscoCallLogFixture(t, callLogPathFor(root), lines)
	got := subagentBrief("http://127.0.0.1:1", root, "svc", "parent", 900, now)
	if !strings.Contains(got, "own.go#L2-L2") {
		t.Errorf("missing own checked range: %q", got)
	}
	for _, c := range cases[1:] {
		if strings.Contains(got, c.path) {
			t.Errorf("leaked %s: %q", c.path, got)
		}
	}
	if strings.Contains(got, "unreachable:") || strings.Contains(got, "stale") {
		t.Errorf("error or stale text leaked: %q", got)
	}
	if err := os.WriteFile(filepath.Join(root, "own.go"), []byte("changed\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if got := subagentBrief("http://127.0.0.1:1", root, "svc", "parent", 900, now); got != "" {
		t.Errorf("empty must have no section: %q", got)
	}
}

func TestAD11ObserverRejectsShellTokens(t *testing.T) {
	pinNoEnclosingRepo(t)
	root := t.TempDir()
	for _, token := range []string{",", "}", "0", "x", "2>/dev/null", ".pane:hover", "s/a/b/"} {
		in := harness.HookInput{Tool: harness.ToolShell, Command: "cat '" + token + "'", Cwd: root}
		if got := observedDiscoveryPointers(in); len(got) != 0 {
			t.Errorf("token %q became pointer %v", token, got)
		}
	}
}

func TestAD11ObserverRecordsRangeHashAndChildIdentity(t *testing.T) {
	pinNoEnclosingRepo(t)
	root := t.TempDir()
	log := filepath.Join(t.TempDir(), "observed.jsonl")
	t.Setenv("FATHOM_OBSERVED_DISCOVERIES_LOG", log)
	p := filepath.Join(root, "a.go")
	if err := os.WriteFile(p, []byte("package sample\nfunc F() {}\n"), 0600); err != nil {
		t.Fatal(err)
	}
	recordObservedDiscoveryIfAny(harness.HookInput{Tool: harness.ToolShell, Command: "sed -n '2,2p' '" + p + "'", Cwd: root, SessionID: "parent", AgentID: "child"})
	data, err := os.ReadFile(log)
	if err != nil {
		t.Fatal(err)
	}
	var row struct {
		Agent  string `json:"agent_id"`
		Ranges []struct {
			Path, Hash string
			Start, End int
		} `json:"range_pointers"`
	}
	if err := json.Unmarshal(data, &row); err != nil {
		t.Fatal(err)
	}
	if row.Agent != "child" || len(row.Ranges) != 1 || row.Ranges[0].Hash == "" || row.Ranges[0].Start != 2 || row.Ranges[0].End != 2 {
		t.Fatalf("unscoped or unhashed observation: %s", data)
	}
}

func TestAD11ShellFragmentsNeverPersist(t *testing.T) {
	pinNoEnclosingRepo(t)
	root := t.TempDir()
	log := filepath.Join(t.TempDir(), "observed.jsonl")
	t.Setenv("FATHOM_OBSERVED_DISCOVERIES_LOG", log)
	for _, fragment := range []string{"s/p/q", "x/y/z", "missing.go"} {
		recordObservedDiscoveryIfAny(harness.HookInput{Tool: harness.ToolShell, Command: "cat '" + fragment + "'", Cwd: root, SessionID: "parent"})
	}
	if data, err := os.ReadFile(log); err == nil {
		t.Fatalf("non-path shell arguments persisted: %s", data)
	}
}

func TestAD11ObservedRangeUsesRepositoryRoot(t *testing.T) {
	pinNoEnclosingRepo(t)
	root := t.TempDir()
	sub := filepath.Join(root, "src")
	for _, dir := range []string{filepath.Join(root, ".git"), sub} {
		if err := os.MkdirAll(dir, 0700); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(root, "a.go"), []byte("wrong root\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sub, "a.go"), []byte("package sample\n"), 0600); err != nil {
		t.Fatal(err)
	}
	log := filepath.Join(t.TempDir(), "observed.jsonl")
	t.Setenv("FATHOM_OBSERVED_DISCOVERIES_LOG", log)
	recordObservedDiscoveryIfAny(harness.HookInput{Tool: harness.ToolShell, Command: "sed -n '1,1p' a.go", Cwd: sub, SessionID: "parent"})
	data, err := os.ReadFile(log)
	if err != nil {
		t.Fatal(err)
	}
	var row observedDiscoveryRow
	if err := json.Unmarshal(data, &row); err != nil {
		t.Fatal(err)
	}
	want, _ := core.HashLines([]byte("package sample\n"), 1, 1)
	if len(row.RangePointers) != 1 || row.RangePointers[0].Path != "src/a.go" || row.RangePointers[0].Hash != want {
		t.Fatalf("range attributed to wrong root: %s", data)
	}
}
