package scenariobench

import (
	"os"
	"path/filepath"
	"strings"
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

// TestLoadScenarioCorpus_RefusesLocalizationCombinedWithAnswerAllOf pins
// the doc's own rule (types.go, Oracle.Localization): the two answer-only
// oracles ask different questions and never combine on one task.
func TestLoadScenarioCorpus_RefusesLocalizationCombinedWithAnswerAllOf(t *testing.T) {
	dir := t.TempDir()
	bad := `{
  "version": 1,
  "description": "test corpus",
  "arms": {
    "full": {"wired": true, "instructions": "stanza", "surface": "fathom"},
    "control": {"wired": false, "instructions": "none", "surface": ""}
  },
  "tasks": [
    {
      "id": "T1",
      "category": "localization",
      "prompt": "name the files",
      "derivation": "hand-authored",
      "subject": {"repo_url": "https://example.invalid/repo.git", "base_commit": "deadbeef"},
      "sessions": {"mode": "single", "count": 1, "max_turns": 10},
      "oracle": {"localization": true, "answer_all_of": ["tok1"]},
      "gold": {"files": ["a.go"]},
      "predictions": {"full": "wins", "control": "loses"}
    }
  ],
  "criterion": {"id": "C1", "treatment": "full", "control": "control", "min_trials": 1, "require_success_at_least_equal": true, "max_tokens_per_success_ratio": 1.0, "registered": "2026-09-01"}
}`
	path := writeCorpusFixture(t, dir, "corpus.json", bad)
	if _, err := LoadScenarioCorpus(path); err == nil {
		t.Fatal("expected an error for a task carrying both localization and answer_all_of")
	}
}

// TestLoadScenarioCorpus_RefusesHomeCorpusDerivationWithoutExcludePaths is
// O53 defect 3's loader-refusal proof: a task whose derivation names the
// home corpus (testdata/contextbench/tasks-home.json) but declares no
// subject.exclude_paths must be refused, naming what is missing — the
// answer-key contamination that reached 41 scored cells in pilot
// vs-pilot-3 must never load silently again.
func TestLoadScenarioCorpus_RefusesHomeCorpusDerivationWithoutExcludePaths(t *testing.T) {
	dir := t.TempDir()
	bad := `{
  "arms": {"full": {"wired": true, "instructions": "stanza"}},
  "tasks": [{
    "id": "T1", "prompt": "p",
    "derivation": "Copied verbatim from testdata/contextbench/tasks-home.json id H1-T01.",
    "subject": {"repo_url": "u", "base_commit": "c"},
    "sessions": {"mode": "single", "count": 1, "max_turns": 5},
    "oracle": {"answer_all_of": ["tok"]},
    "predictions": {"full": "wins"}
  }],
  "criterion": {"id": "C1", "treatment": "full", "control": "control", "min_trials": 1}
}`
	path := writeCorpusFixture(t, dir, "corpus.json", bad)
	_, err := LoadScenarioCorpus(path)
	if err == nil {
		t.Fatal("expected an error for a home-corpus-derived task with no exclude_paths")
	}
	if !strings.Contains(err.Error(), "exclude_paths") {
		t.Errorf("error %q does not name exclude_paths", err)
	}
}

// TestLoadScenarioCorpus_RefusesHomeCorpusDerivationWithPartialExcludePaths
// covers the "cover" requirement, not just "declare something": all three
// globs are required, not just one.
func TestLoadScenarioCorpus_RefusesHomeCorpusDerivationWithPartialExcludePaths(t *testing.T) {
	dir := t.TempDir()
	bad := `{
  "arms": {"full": {"wired": true, "instructions": "stanza"}},
  "tasks": [{
    "id": "T1", "prompt": "p",
    "derivation": "Copied verbatim from testdata/contextbench/tasks-home.json id H1-T01.",
    "subject": {"repo_url": "u", "base_commit": "c", "exclude_paths": ["testdata/contextbench/**"]},
    "sessions": {"mode": "single", "count": 1, "max_turns": 5},
    "oracle": {"answer_all_of": ["tok"]},
    "predictions": {"full": "wins"}
  }],
  "criterion": {"id": "C1", "treatment": "full", "control": "control", "min_trials": 1}
}`
	path := writeCorpusFixture(t, dir, "corpus.json", bad)
	if _, err := LoadScenarioCorpus(path); err == nil {
		t.Fatal("expected an error: exclude_paths covers only one of the three required globs")
	}
}

// TestLoadScenarioCorpus_AcceptsHomeCorpusDerivationWithExcludePaths is the
// converse: all three required globs present, task loads clean.
func TestLoadScenarioCorpus_AcceptsHomeCorpusDerivationWithExcludePaths(t *testing.T) {
	dir := t.TempDir()
	good := `{
  "arms": {"full": {"wired": true, "instructions": "stanza"}},
  "tasks": [{
    "id": "T1", "prompt": "p",
    "derivation": "Copied verbatim from testdata/contextbench/tasks-home.json id H1-T01.",
    "subject": {"repo_url": "u", "base_commit": "c", "exclude_paths": ["testdata/contextbench/**", "testdata/scenariobench/**", "docs/bench/**"]},
    "sessions": {"mode": "single", "count": 1, "max_turns": 5},
    "oracle": {"answer_all_of": ["tok"]},
    "predictions": {"full": "wins"}
  }],
  "criterion": {"id": "C1", "treatment": "full", "control": "control", "min_trials": 1}
}`
	path := writeCorpusFixture(t, dir, "corpus.json", good)
	c, err := LoadScenarioCorpus(path)
	if err != nil {
		t.Fatalf("LoadScenarioCorpus: %v", err)
	}
	if len(c.Tasks) != 1 {
		t.Fatalf("unexpected tasks: %+v", c.Tasks)
	}
}

// TestLoadScenarioCorpus_TaskNotDerivedFromHomeCorpusNeedsNoExcludePaths
// proves the requirement is scoped to home-corpus derivations only — an
// ordinary task with no exclude_paths at all still loads.
func TestLoadScenarioCorpus_TaskNotDerivedFromHomeCorpusNeedsNoExcludePaths(t *testing.T) {
	dir := t.TempDir()
	path := writeCorpusFixture(t, dir, "corpus.json", validCorpusJSON())
	if _, err := LoadScenarioCorpus(path); err != nil {
		t.Fatalf("LoadScenarioCorpus: %v", err)
	}
}

// TestLoadScenarioCorpus_AnswerAnyOfAlone proves a task whose ONLY oracle
// is answer_any_of (no answer_all_of at all) loads without error — hasOracle
// must accept it, mirroring answer_all_of's own acceptance (O53 defect 5a).
func TestLoadScenarioCorpus_AnswerAnyOfAlone(t *testing.T) {
	dir := t.TempDir()
	good := `{
  "arms": {"full": {"wired": true, "instructions": "stanza"}},
  "tasks": [{
    "id": "T1", "prompt": "p", "derivation": "d",
    "subject": {"repo_url": "u", "base_commit": "c"},
    "sessions": {"mode": "single", "count": 1, "max_turns": 5},
    "oracle": {"answer_any_of": [["core.Edge", "Edge struct"]]},
    "predictions": {"full": "wins"}
  }],
  "criterion": {"id": "C1", "treatment": "full", "control": "control", "min_trials": 1}
}`
	path := writeCorpusFixture(t, dir, "corpus.json", good)
	c, err := LoadScenarioCorpus(path)
	if err != nil {
		t.Fatalf("LoadScenarioCorpus: %v", err)
	}
	if len(c.Tasks) != 1 || len(c.Tasks[0].Oracle.AnswerAnyOf) != 1 {
		t.Fatalf("unexpected tasks: %+v", c.Tasks)
	}
}

// TestLoadScenarioCorpus_RefusesEmptyAnswerAnyOfGroup covers the
// structural check: a group with zero alternatives, or containing an
// empty string, can never be satisfied and is refused at load time.
func TestLoadScenarioCorpus_RefusesEmptyAnswerAnyOfGroup(t *testing.T) {
	cases := []string{
		`{"answer_any_of": [[]]}`,
		`{"answer_any_of": [[""]]}`,
	}
	for _, oracle := range cases {
		dir := t.TempDir()
		bad := `{
  "arms": {"full": {"wired": true, "instructions": "stanza"}},
  "tasks": [{
    "id": "T1", "prompt": "p", "derivation": "d",
    "subject": {"repo_url": "u", "base_commit": "c"},
    "sessions": {"mode": "single", "count": 1, "max_turns": 5},
    "oracle": ` + oracle + `,
    "predictions": {"full": "wins"}
  }],
  "criterion": {"id": "C1", "treatment": "full", "control": "control", "min_trials": 1}
}`
		path := writeCorpusFixture(t, dir, "corpus.json", bad)
		if _, err := LoadScenarioCorpus(path); err == nil {
			t.Errorf("oracle %s: expected an error for an unsatisfiable answer_any_of group", oracle)
		}
	}
}

// TestLoadScenarioCorpus_RefusesLocalizationCombinedWithAnswerAnyOf mirrors
// the existing AnswerAllOf/Localization refusal for the new group form.
func TestLoadScenarioCorpus_RefusesLocalizationCombinedWithAnswerAnyOf(t *testing.T) {
	dir := t.TempDir()
	bad := `{
  "arms": {"full": {"wired": true, "instructions": "stanza"}},
  "tasks": [{
    "id": "T1", "prompt": "p", "derivation": "d",
    "subject": {"repo_url": "u", "base_commit": "c"},
    "sessions": {"mode": "single", "count": 1, "max_turns": 5},
    "oracle": {"localization": true, "answer_any_of": [["a", "b"]]},
    "gold": {"files": ["a.go"]},
    "predictions": {"full": "wins"}
  }],
  "criterion": {"id": "C1", "treatment": "full", "control": "control", "min_trials": 1}
}`
	path := writeCorpusFixture(t, dir, "corpus.json", bad)
	if _, err := LoadScenarioCorpus(path); err == nil {
		t.Fatal("expected an error for a task carrying both localization and answer_any_of")
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
