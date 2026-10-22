package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/harness"
)

func TestInactiveFeaturesShareAttentionBudget(t *testing.T) {
	root, now := readinessEnv(t)
	if err := os.WriteFile(filepath.Join(os.Getenv("FATHOM_HOME"), "config.toml"), []byte("[readouts]\nmode = 'rich'\n[hooks]\nclaim_check = true\nread_outline_hint = true\n"), 0600); err != nil {
		t.Fatal(err)
	}
	fragment := inactiveFeaturesLine(root, harness.ClaudeCode, now)
	if !strings.Contains(fragment, "3 features inactive:") || !strings.Contains(fragment, "+1 more") || strings.Contains(fragment, "readouts") {
		t.Fatalf("unbounded feature list: %q", fragment)
	}
	primer, _, attention := composePrimer("identity", root, now, unreadRoots, fragment)
	if len(attention) > attentionLineBudget || strings.Count(primer, "attention:") != 1 || !strings.Contains(attention, fragment) || strings.Count(primer, fragment) != 1 {
		t.Fatalf("features must share the attention line: attention=%q primer=%q", attention, primer)
	}
}

func TestPrimerNamesCutWhenEveryBriefLineDrops(t *testing.T) {
	root, now := readinessEnv(t)
	row := map[string]any{"schema_version": 1, "id": "TH-1", "thread": "handoff", "kind": "note", "text": strings.Repeat("detail ", 100), "filed_at": now.Format("2006-01-02T15:04:05Z07:00"), "pointers": map[string]string{"repo_root": root}}
	raw, _ := json.Marshal(row)
	if err := os.WriteFile(threadsPath(), append(raw, '\n'), 0600); err != nil {
		t.Fatal(err)
	}
	identity := strings.Repeat("i", sessionPrimerBudget-220)
	primer, _, _ := composePrimer(identity, root, now, unreadRoots)
	if len(primer) > sessionPrimerBudget || !strings.Contains(primer, "brief line(s) cut to fit") || strings.Contains(primer, "detail detail") {
		t.Fatalf("all-brief cut must remain named: %q", primer)
	}
}
