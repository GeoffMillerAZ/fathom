package scenariobench

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// lookupsVSPath is testdata/scenariobench/lookups-vs.json (SB-32), resolved
// relative to the repo root rather than this package directory since the
// corpus is committed under the repo-wide testdata/scenariobench/, not
// internal/scenariobench/testdata/.
func lookupsVSPath(t *testing.T) string {
	t.Helper()
	return filepath.Join("..", "..", "testdata", "scenariobench", "lookups-vs.json")
}

// TestLookupsVSCorpus_ParsesAgainstContract proves the SB-32 corpus is valid
// data against the Corpus/Task/Arm contract this file must not edit
// (types.go) — the anchor a future LoadScenarioCorpus (SB-10, wave 2) will
// build on.
func TestLookupsVSCorpus_ParsesAgainstContract(t *testing.T) {
	data, err := os.ReadFile(lookupsVSPath(t))
	if err != nil {
		t.Fatalf("read lookups-vs.json: %v", err)
	}
	var c Corpus
	if err := json.Unmarshal(data, &c); err != nil {
		t.Fatalf("unmarshal lookups-vs.json against Corpus: %v", err)
	}

	wantArms := []string{"full", "graphify", "serena", "gopls", "grep"}
	for _, a := range wantArms {
		if _, ok := c.Arms[a]; !ok {
			t.Errorf("arm %q missing", a)
		}
	}
	if len(c.Arms) != len(wantArms) {
		t.Errorf("got %d arms, want %d: %v", len(c.Arms), len(wantArms), c.Arms)
	}

	full := c.Arms["full"]
	if !full.Wired || full.Instructions != "stanza" || full.Surface != "fathom" {
		t.Errorf("full arm = %+v, want wired=true instructions=stanza surface=fathom", full)
	}
	for _, name := range []string{"graphify", "serena", "gopls", "grep"} {
		if c.Arms[name].Instructions != "plain" {
			t.Errorf("arm %q instructions = %q, want plain", name, c.Arms[name].Instructions)
		}
		if c.Arms[name].Wired {
			t.Errorf("arm %q must not be wired", name)
		}
	}
	for _, name := range wantArms {
		got := c.Arms[name].AllowedTools
		if len(got) != 3 {
			t.Errorf("arm %q allowed_tools = %v, want [Read Grep Bash]", name, got)
		}
	}
	if c.Arms["graphify"].Surface != "graphify@v0.9.61" {
		t.Errorf("graphify surface = %q, want graphify@v0.9.61", c.Arms["graphify"].Surface)
	}
	if c.Arms["serena"].Surface != "serena@v1.7.0" {
		t.Errorf("serena surface = %q, want serena@v1.7.0", c.Arms["serena"].Surface)
	}
	if c.Arms["gopls"].Surface != "gopls@v0.20+" {
		t.Errorf("gopls surface = %q, want gopls@v0.20+", c.Arms["gopls"].Surface)
	}
	if c.Arms["grep"].Surface != "" {
		t.Errorf("grep surface = %q, want empty", c.Arms["grep"].Surface)
	}
	if len(c.Arms["graphify"].MCP) == 0 || len(c.Arms["serena"].MCP) == 0 || len(c.Arms["gopls"].MCP) == 0 {
		t.Error("graphify/serena/gopls arms must carry an mcp block")
	}
	if len(c.Arms["grep"].MCP) != 0 {
		t.Error("grep arm must carry no mcp block")
	}

	wantIDs := []string{
		"H1-T01", "H1-T02", "H1-T03", "H1-T04", "H1-T05", "H1-T06", "H1-T07",
		"H1-T08", "H1-T09", "H1-T10", "H1-T11", "H1-T12", "H1-T13", "FRESH-M7-01",
	}
	if len(c.Tasks) != len(wantIDs) {
		t.Fatalf("got %d tasks, want %d", len(c.Tasks), len(wantIDs))
	}
	byID := map[string]Task{}
	for _, task := range c.Tasks {
		byID[task.ID] = task
	}
	for _, id := range wantIDs {
		task, ok := byID[id]
		if !ok {
			t.Errorf("task %q missing", id)
			continue
		}
		if task.Prompt == "" {
			t.Errorf("task %q has no prompt", id)
		}
		if task.Derivation == "" {
			t.Errorf("task %q has no derivation", id)
		}
		for _, arm := range wantArms {
			if task.Predictions[arm] == "" {
				t.Errorf("task %q missing a prediction for arm %q", id, arm)
			}
		}
	}

	fresh, ok := byID["FRESH-M7-01"]
	if !ok {
		t.Fatal("FRESH-M7-01 missing")
	}
	if fresh.Category != CategoryFreshness {
		t.Errorf("FRESH-M7-01 category = %q, want %q", fresh.Category, CategoryFreshness)
	}
	if len(fresh.Subject.Setup) == 0 {
		t.Error("FRESH-M7-01 must carry a subject.setup step that edits a file")
	}

	if c.Criterion.ID != "M6-1" || c.Criterion.Treatment != "full" || c.Criterion.Control != "grep" {
		t.Errorf("criterion = %+v, want id=M6-1 treatment=full control=grep", c.Criterion)
	}
	if c.Criterion.MinTrials != 2 || !c.Criterion.RequireSuccessAtLeastEqual || c.Criterion.MaxTokensPerSuccessRatio != 1.0 {
		t.Errorf("criterion bar = %+v, want min_trials=2 require_success_at_least_equal=true max_tokens_per_success_ratio=1.0", c.Criterion)
	}
	if c.Criterion.Registered != "2026-09-13" {
		t.Errorf("criterion registered = %q, want 2026-09-13", c.Criterion.Registered)
	}

	if len(c.Sources) != 3 {
		t.Fatalf("got %d sources, want 3 (graphify, serena, gopls)", len(c.Sources))
	}
	seen := map[string]bool{}
	for _, s := range c.Sources {
		seen[s.Name] = true
		if s.URL == "" || s.Revision == "" || s.License == "" {
			t.Errorf("source %+v missing a field", s)
		}
	}
	for _, name := range []string{"graphify", "serena", "gopls"} {
		if !seen[name] {
			t.Errorf("source pin %q missing", name)
		}
	}

	gold, ok := c.Notes["gold"].(map[string]any)
	if !ok {
		t.Fatal("notes.gold missing or not an object")
	}
	for _, id := range wantIDs {
		if _, ok := gold[id]; !ok {
			t.Errorf("notes.gold missing an entry for %q", id)
		}
	}
}

// TestLookupsVSCorpus_LoadScenarioCorpusAccepts proves LoadScenarioCorpus
// (SB-10's gate) no longer refuses lookups-vs.json's answer-only tasks:
// before oracle.answer_all_of existed, every task's oracle was `{}` and
// hasOracle refused it ("no oracle ... nothing would decide success").
func TestLookupsVSCorpus_LoadScenarioCorpusAccepts(t *testing.T) {
	c, err := LoadScenarioCorpus(lookupsVSPath(t))
	if err != nil {
		t.Fatalf("LoadScenarioCorpus: %v", err)
	}
	for _, task := range c.Tasks {
		if len(task.Oracle.AnswerAllOf) == 0 {
			t.Errorf("task %q: Oracle.AnswerAllOf is empty", task.ID)
		}
		if !AnswerOnly(task) {
			t.Errorf("task %q: want AnswerOnly(task)=true", task.ID)
		}
	}
}

// TestLookupsVSCorpus_ContainsGraphifyLiteral pins the same lexical fact
// fathom verify's oracle checks for this file's plan row (SB-32: pattern
// "\"graphify\"").
func TestLookupsVSCorpus_ContainsGraphifyLiteral(t *testing.T) {
	data, err := os.ReadFile(lookupsVSPath(t))
	if err != nil {
		t.Fatalf("read lookups-vs.json: %v", err)
	}
	if !bytes.Contains(data, []byte(`"graphify"`)) {
		t.Error(`lookups-vs.json must contain the literal string "graphify"`)
	}
}
