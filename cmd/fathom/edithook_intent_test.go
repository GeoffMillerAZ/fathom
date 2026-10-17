package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/semantic"
)

// cappedRulesServer answers obligations with exactly as many rules as the
// caller asked for (6 when it named no cap) and records the cap asked for.
func cappedRulesServer(t *testing.T, asked *int32) func() string {
	t.Helper()
	srv, _ := countingServer(t, func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Params struct {
				Name      string `json:"name"`
				Arguments struct {
					File     string `json:"file"`
					MaxRules int    `json:"max_rules"`
				} `json:"arguments"`
			} `json:"params"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		inner := `{}`
		switch req.Params.Name {
		case "list_services":
			inner = `{"services":[]}`
		case "obligations":
			n := req.Params.Arguments.MaxRules
			atomic.StoreInt32(asked, int32(n))
			if n <= 0 || n > semantic.MaxObligations {
				n = semantic.MaxObligations
			}
			rules := []any{}
			for i := 0; i < n; i++ {
				rules = append(rules, map[string]any{"rule": fmt.Sprintf("Rule %d keeps the store pure.", i+1), "src": "AGENTS.md#L1-L2", "tier": "doc-claimed", "layer": "obligations"})
			}
			b, _ := json.Marshal(map[string]any{"service": "svc", "file": req.Params.Arguments.File, "rules": rules})
			inner = string(b)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": 1, "result": map[string]any{"content": []any{map[string]any{"type": "text", "text": inner}}}})
	})
	t.Cleanup(srv.Close)
	return func() string { return srv.URL }
}

// IV-3b (ADR-074 decision 6): an edit under a generated entry is served
// the invariant ids the router row names, each with its rung, in at most 2
// of I35's 6 slots — even when a hand-written guard matches the file first.
// The governing rules get the remaining slots, so the count cap stands; no
// stamp or provenance noise reaches the agent; an invariant no surface row
// names (repo-wide) never reaches an edit.
func TestRenderPreEdit_IntentIdsAndRungInAtMostTwoSlots(t *testing.T) {
	guardStampsIsolated(t)
	t.Setenv("HOME", t.TempDir())
	root := t.TempDir()
	gitInitTempRepo(t, root)
	writeCorpusFixture(t, root)
	// A hand guard that matches the same file FIRST and asks for all 6 slots.
	writeGuardCue(t, root, `guards: [
	{
		scopes: ["internal/**"]
		note:   "hand note: internal code stays deterministic"
		max_rules: 6
	},
]
`)
	if code, _, stderr := runIntentExport(t, "--root", root, "--guard", "--write"); code != 0 {
		t.Fatalf("--write: exit %d: %s", code, stderr)
	}
	file := filepath.Join(root, "internal", "store", "store.go")
	if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file, []byte("package store\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	var asked int32
	url := cappedRulesServer(t, &asked)
	res, ok := renderPreEdit(url(), root, file, "s-intent-slots")
	if !ok {
		t.Fatal("a guarded file under a generated entry must be served")
	}
	text := res.text

	if !strings.Contains(text, "hand note: internal code stays deterministic") {
		t.Errorf("the hand guard still speaks:\n%s", text)
	}
	if !strings.Contains(text, "I1 [test]") {
		t.Fatalf("the edit must carry the row's invariant id with its rung:\n%s", text)
	}
	if strings.Contains(text, "I2") {
		t.Fatalf("a repo-wide invariant (no surface row names it) must never reach an edit:\n%s", text)
	}
	if strings.Contains(text, "@ ") || strings.Contains(text, "(fathom intent audit)") {
		t.Fatalf("provenance stamps are for the audit, not the agent:\n%s", text)
	}

	var intentSlots, ruleSlots int
	for _, line := range strings.Split(text, "\n") {
		switch {
		case strings.HasPrefix(line, "• intent"), strings.HasPrefix(line, "• done needs"):
			intentSlots++
			if n := len(strings.Fields(strings.TrimPrefix(line, "• "))); n > intentSlotMaxWords {
				t.Errorf("intent slot is %d words, over the %d-word rule cap: %q", n, intentSlotMaxWords, line)
			}
		case strings.HasPrefix(line, "• "):
			ruleSlots++
		}
	}
	if intentSlots == 0 || intentSlots > 2 {
		t.Fatalf("intent occupies %d slots, want 1..2:\n%s", intentSlots, text)
	}
	if intentSlots+ruleSlots > semantic.MaxObligations {
		t.Fatalf("%d intent + %d rule slots exceed I35's %d:\n%s", intentSlots, ruleSlots, semantic.MaxObligations, text)
	}
	if got := int(atomic.LoadInt32(&asked)); got != semantic.MaxObligations-intentSlots {
		t.Fatalf("asked the daemon for %d rules, want %d (6 minus the intent slots)", got, semantic.MaxObligations-intentSlots)
	}

	// I35's count, word and payload caps are unchanged.
	if semantic.MaxObligations != 6 || intentSlotMaxWords != 28 || specLineTokenBudget != 320 {
		t.Fatalf("I35's caps moved: count %d, words %d, payload %d tokens", semantic.MaxObligations, intentSlotMaxWords, specLineTokenBudget)
	}
}

// A file only a generated entry covers (no hand guard) gets the intent
// slots and the rest of the six as rules.
func TestRenderPreEdit_GeneratedEntryAloneServesIntentAndRules(t *testing.T) {
	guardStampsIsolated(t)
	t.Setenv("HOME", t.TempDir())
	root := t.TempDir()
	gitInitTempRepo(t, root)
	writeCorpusFixture(t, root)
	if code, _, stderr := runIntentExport(t, "--root", root, "--guard", "--write"); code != 0 {
		t.Fatalf("--write: exit %d: %s", code, stderr)
	}
	file := filepath.Join(root, "internal", "store", "store.go")
	_ = os.MkdirAll(filepath.Dir(file), 0o755)
	_ = os.WriteFile(file, []byte("package store\n"), 0o644)
	var asked int32
	url := cappedRulesServer(t, &asked)
	res, ok := renderPreEdit(url(), root, file, "s-intent-alone")
	if !ok || !strings.Contains(res.text, "• intent") || !strings.Contains(res.text, "I1 [test]") {
		t.Fatalf("ok=%v text:\n%s", ok, res.text)
	}
	if strings.Contains(res.text, "intent: internal/store/**") {
		t.Fatalf("the raw generated note must not be printed as the guard note:\n%s", res.text)
	}
	if !strings.Contains(res.text, "done needs: TestStorePurity") {
		t.Fatalf("the row's Done-needs tests ride the second slot:\n%s", res.text)
	}
}
