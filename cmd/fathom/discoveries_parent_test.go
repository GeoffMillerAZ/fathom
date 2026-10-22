package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/discoveryptr"
	"github.com/geoffmilleraz/fathom/internal/harness"
)

func TestAD11DiscoveriesParentFilter(t *testing.T) {
	pinNoEnclosingRepo(t)
	threadEnv(t)
	root := t.TempDir()
	t.Setenv("FATHOM_SESSIONS_LOG", filepath.Join(t.TempDir(), "sessions.jsonl"))
	t.Setenv("FATHOM_OBSERVED_DISCOVERIES_LOG", filepath.Join(t.TempDir(), "observed.jsonl"))
	now := time.Date(2026, 10, 22, 12, 0, 0, 0, time.UTC)
	old := nowFunc
	nowFunc = func() time.Time { return now }
	t.Cleanup(func() { nowFunc = old })
	recordSessionWindow(root, sessionWindowRow{SessionID: "parent", Started: now.Add(-time.Hour).Format(time.RFC3339)})
	for _, c := range []struct{ file, session, agent string }{
		{"own.go", "parent", ""}, {"child.go", "parent", "child"}, {"other.go", "other", ""}, {"AGENTS.md", "parent", ""},
	} {
		if err := os.WriteFile(filepath.Join(root, c.file), []byte("package sample\n"), 0600); err != nil {
			t.Fatal(err)
		}
		recordObservedDiscoveryIfAny(harness.HookInput{Cwd: root, SessionID: c.session, AgentID: c.agent, Tool: harness.ToolShell, Command: "sed -n '1,1p' " + c.file})
	}
	var out, errs bytes.Buffer
	if code := cmdDiscoveries([]string{"--root", root, "--service", "svc", "--parent-session", "parent"}, &out, &errs); code != 0 {
		t.Fatalf("exit %d: %s", code, errs.String())
	}
	if !strings.Contains(out.String(), "own.go#L1-L1") {
		t.Errorf("missing own range: %s", out.String())
	}
	for _, unwanted := range []string{"child.go", "other.go", "AGENTS.md", "unreachable", "stale"} {
		if strings.Contains(out.String(), unwanted) {
			t.Errorf("filtered expansion leaked %q: %s", unwanted, out.String())
		}
	}
	brief := subagentBrief("", root, "svc", "parent", 900, now)
	if brief != out.String()[:len(out.String())-1] {
		t.Errorf("expansion differs from filtered brief: %q / %q", out.String(), brief)
	}
}

func TestAD11BriefIgnoresDaemonRangeSnapshots(t *testing.T) {
	pinNoEnclosingRepo(t)
	threadEnv(t)
	root := t.TempDir()
	t.Setenv("FATHOM_STATE_DIR", t.TempDir())
	t.Setenv("FATHOM_SESSIONS_LOG", filepath.Join(t.TempDir(), "sessions.jsonl"))
	t.Setenv("FATHOM_OBSERVED_DISCOVERIES_LOG", filepath.Join(t.TempDir(), "observed.jsonl"))
	now := time.Date(2026, 10, 22, 12, 0, 0, 0, time.UTC)
	recordSessionWindow(root, sessionWindowRow{SessionID: "parent", Started: now.Add(-time.Hour).Format(time.RFC3339)})
	if err := os.WriteFile(filepath.Join(root, "sample.go"), []byte("package sample\n"), 0600); err != nil {
		t.Fatal(err)
	}
	p, _, _ := discoveryptr.Read(root, "sample.go", 1, 1)
	raw, _ := json.Marshal(map[string]any{"ts": now.Format(time.RFC3339Nano), "service": "svc", "caller": map[string]string{"session": "parent", "agent_id": ""}, "range_pointers": []discoveryptr.Pointer{p}})
	writeDiscoCallLogFixture(t, callLogPathFor(root), []string{string(raw)})
	if got := subagentBrief("", root, "svc", "parent", 900, now); got != "" {
		t.Errorf("daemon snapshot entered brief: %s", got)
	}
}
