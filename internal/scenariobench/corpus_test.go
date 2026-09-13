package scenariobench

import (
	"os"
	"path/filepath"
	"testing"
)

func validCorpusJSON() string {
	return `{
  "version": 1,
  "description": "test corpus",
  "arms": {
    "full": {"wired": true, "instructions": "stanza", "surface": "fathom"},
    "control": {"wired": false, "instructions": "none", "surface": ""}
  },
  "tasks": [
    {
      "id": "T1",
      "category": "bug_fix",
      "prompt": "fix the bug",
      "derivation": "derived from PR #1",
      "subject": {"repo_url": "https://example.invalid/repo.git", "base_commit": "deadbeef"},
      "sessions": {"mode": "single", "count": 1, "max_turns": 10},
      "oracle": {"fail_to_pass": ["TestFoo"]},
      "predictions": {"full": "wins", "control": "loses"}
    }
  ],
  "criterion": {"id": "C1", "treatment": "full", "control": "control", "min_trials": 1, "require_success_at_least_equal": true, "max_tokens_per_success_ratio": 1.0, "registered": "2026-09-01"}
}`
}

func writeCorpusFixture(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("writing corpus fixture: %v", err)
	}
	return path
}

func TestLoadScenarioCorpus_Valid(t *testing.T) {
	dir := t.TempDir()
	path := writeCorpusFixture(t, dir, "corpus.json", validCorpusJSON())

	c, err := LoadScenarioCorpus(path)
	if err != nil {
		t.Fatalf("LoadScenarioCorpus: %v", err)
	}
	if len(c.Tasks) != 1 || c.Tasks[0].ID != "T1" {
		t.Fatalf("unexpected tasks: %+v", c.Tasks)
	}
	if len(c.Arms) != 2 {
		t.Fatalf("unexpected arms: %+v", c.Arms)
	}
}

func TestLoadScenarioCorpus_RefusesMissingPrediction(t *testing.T) {
	dir := t.TempDir()
	bad := `{
  "arms": {"full": {"wired": true, "instructions": "stanza"}, "control": {"wired": false, "instructions": "none"}},
  "tasks": [{
    "id": "T1", "prompt": "p", "derivation": "d",
    "subject": {"repo_url": "u", "base_commit": "c"},
    "sessions": {"mode": "single", "count": 1, "max_turns": 5},
    "oracle": {"fail_to_pass": ["TestFoo"]},
    "predictions": {"full": "wins"}
  }],
  "criterion": {"id": "C1", "treatment": "full", "control": "control", "min_trials": 1}
}`
	path := writeCorpusFixture(t, dir, "corpus.json", bad)
	if _, err := LoadScenarioCorpus(path); err == nil {
		t.Fatal("expected an error for a task missing the control arm's prediction")
	}
}

func TestLoadScenarioCorpus_RefusesMissingOracle(t *testing.T) {
	dir := t.TempDir()
	bad := `{
  "arms": {"full": {"wired": true, "instructions": "stanza"}},
  "tasks": [{
    "id": "T1", "prompt": "p", "derivation": "d",
    "subject": {"repo_url": "u", "base_commit": "c"},
    "sessions": {"mode": "single", "count": 1, "max_turns": 5},
    "oracle": {},
    "predictions": {"full": "wins"}
  }],
  "criterion": {"id": "C1", "treatment": "full", "control": "control", "min_trials": 1}
}`
	path := writeCorpusFixture(t, dir, "corpus.json", bad)
	if _, err := LoadScenarioCorpus(path); err == nil {
		t.Fatal("expected an error for a task with no oracle")
	}
}

func TestLoadScenarioCorpus_RefusesMissingDerivation(t *testing.T) {
	dir := t.TempDir()
	bad := `{
  "arms": {"full": {"wired": true, "instructions": "stanza"}},
  "tasks": [{
    "id": "T1", "prompt": "p",
    "subject": {"repo_url": "u", "base_commit": "c"},
    "sessions": {"mode": "single", "count": 1, "max_turns": 5},
    "oracle": {"fail_to_pass": ["TestFoo"]},
    "predictions": {"full": "wins"}
  }],
  "criterion": {"id": "C1", "treatment": "full", "control": "control", "min_trials": 1}
}`
	path := writeCorpusFixture(t, dir, "corpus.json", bad)
	if _, err := LoadScenarioCorpus(path); err == nil {
		t.Fatal("expected an error for a task with no derivation")
	}
}

// TestLoadScenarioCorpus_AnswerOnlyOracleIsValid proves a task whose ONLY
// oracle is answer_all_of (SB-32 lookups-vs.json's shape) loads without
// error — hasOracle must accept it.
func TestLoadScenarioCorpus_AnswerOnlyOracleIsValid(t *testing.T) {
	dir := t.TempDir()
	good := `{
  "version": 1,
  "description": "test corpus",
  "arms": {
    "full": {"wired": true, "instructions": "stanza", "surface": "fathom"},
    "control": {"wired": false, "instructions": "none", "surface": ""}
  },
  "tasks": [
    {
      "id": "T1",
      "category": "obligations",
      "prompt": "which rule governs this file?",
      "derivation": "hand-authored lookup task",
      "subject": {"repo_url": "https://example.invalid/repo.git", "base_commit": "deadbeef"},
      "sessions": {"mode": "single", "count": 1, "max_turns": 10},
      "oracle": {"answer_all_of": ["tok1", "tok2"]},
      "predictions": {"full": "wins", "control": "loses"}
    }
  ],
  "criterion": {"id": "C1", "treatment": "full", "control": "control", "min_trials": 1, "require_success_at_least_equal": true, "max_tokens_per_success_ratio": 1.0, "registered": "2026-09-01"}
}`
	path := writeCorpusFixture(t, dir, "corpus.json", good)
	c, err := LoadScenarioCorpus(path)
	if err != nil {
		t.Fatalf("LoadScenarioCorpus: %v", err)
	}
	if len(c.Tasks) != 1 || len(c.Tasks[0].Oracle.AnswerAllOf) != 2 {
		t.Fatalf("unexpected tasks: %+v", c.Tasks)
	}
}

func TestCorpusSHA256_Deterministic(t *testing.T) {
	dir := t.TempDir()
	path := writeCorpusFixture(t, dir, "corpus.json", validCorpusJSON())

	sha1, err := CorpusSHA256(path)
	if err != nil {
		t.Fatalf("CorpusSHA256: %v", err)
	}
	sha2, err := CorpusSHA256(path)
	if err != nil {
		t.Fatalf("CorpusSHA256: %v", err)
	}
	if sha1 != sha2 {
		t.Fatalf("expected a stable hash, got %s then %s", sha1, sha2)
	}
	if len(sha1) != 64 {
		t.Fatalf("expected a hex sha256 (64 chars), got %d: %s", len(sha1), sha1)
	}
}
