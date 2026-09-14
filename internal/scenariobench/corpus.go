// The scenario corpus gate (SB-10, vision 27 §2). A committed scenario
// corpus (testdata/scenariobench/*.json) is DATA: a task names a change to
// a pinned subject, an oracle fathom does not own, and — critically — a
// prediction per arm registered before a token is spent. A corpus missing
// any of the three is refused at load time, not discovered mid-run: a task
// with no oracle cannot decide success, a task with no derivation cannot be
// re-checked by a stranger, and a task missing an arm's prediction lets that
// arm's result be narrated after the fact as whatever it turned out to be.
package scenariobench

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"sort"
)

// LoadScenarioCorpus reads and validates the scenario corpus at path
// (Corpus, the types.go contract). Every task is checked against every
// declared arm; the first invalid task's error is returned so a malformed
// corpus fails loudly rather than running a broken bench silently.
func LoadScenarioCorpus(path string) (Corpus, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return Corpus{}, fmt.Errorf("reading scenario corpus %s: %w", path, err)
	}
	var c Corpus
	if err := json.Unmarshal(raw, &c); err != nil {
		return Corpus{}, fmt.Errorf("parsing scenario corpus %s: %w", path, err)
	}
	if err := validateCorpus(c); err != nil {
		return Corpus{}, fmt.Errorf("scenario corpus %s: %w", path, err)
	}
	return c, nil
}

// validateCorpus checks the corpus-level shape and every task in it.
func validateCorpus(c Corpus) error {
	if len(c.Tasks) == 0 {
		return fmt.Errorf("no tasks")
	}
	if len(c.Arms) == 0 {
		return fmt.Errorf("no arms")
	}
	armNames := make([]string, 0, len(c.Arms))
	for name := range c.Arms {
		armNames = append(armNames, name)
	}
	sort.Strings(armNames)
	seen := make(map[string]bool, len(c.Tasks))
	for _, t := range c.Tasks {
		if err := validateScenarioTask(t, armNames); err != nil {
			return err
		}
		if seen[t.ID] {
			return fmt.Errorf("task %s: duplicate id", t.ID)
		}
		seen[t.ID] = true
	}
	return nil
}

// validateScenarioTask refuses a task lacking a prediction per arm, an
// oracle, or a derivation — the three things a corpus must register before
// any spend (SB-10).
func validateScenarioTask(t Task, armNames []string) error {
	if t.ID == "" {
		return fmt.Errorf("task has no id")
	}
	if t.Prompt == "" {
		return fmt.Errorf("task %s: empty prompt", t.ID)
	}
	if t.Derivation == "" {
		return fmt.Errorf("task %s: no derivation — a task's provenance must be re-checkable by a stranger", t.ID)
	}
	if !hasOracle(t.Oracle) {
		return fmt.Errorf("task %s: no oracle (fail_to_pass, pass_to_pass, or detectors) — nothing would decide success", t.ID)
	}
	if t.Oracle.Localization && (len(t.Oracle.AnswerAllOf) > 0 || len(t.Oracle.AnswerAnyOf) > 0) {
		return fmt.Errorf("task %s: oracle carries both localization and answer_all_of/answer_any_of — the two answer-only oracles ask different questions and never combine on one task", t.ID)
	}
	for i, group := range t.Oracle.AnswerAnyOf {
		if len(group) == 0 {
			return fmt.Errorf("task %s: oracle.answer_any_of[%d] is empty", t.ID, i)
		}
		for _, alt := range group {
			if alt == "" {
				return fmt.Errorf("task %s: oracle.answer_any_of[%d] contains an empty string", t.ID, i)
			}
		}
	}
	if t.Oracle.Refusal != nil && len(t.Oracle.Refusal.MustCite) == 0 {
		return fmt.Errorf("task %s: oracle.refusal needs at least one must_cite token — a refusal with nothing to cite could never be told apart from an agent that just did nothing", t.ID)
	}
	for _, arm := range armNames {
		if t.Predictions[arm] == "" {
			return fmt.Errorf("task %s: missing a registered prediction for arm %q", t.ID, arm)
		}
	}
	if t.Subject.RepoURL == "" {
		return fmt.Errorf("task %s: subject has no repo_url", t.ID)
	}
	if t.Subject.BaseCommit == "" {
		return fmt.Errorf("task %s: subject has no base_commit", t.ID)
	}
	// O53 defect 3: a task copied from the home corpus must exclude the
	// home corpus (and its scenariobench/docs siblings) from its own
	// worktree, or the answer key it was derived from sits inside the
	// very worktree the agent can Read/Grep/Bash.
	if derivesFromHomeCorpus(t) {
		if missing := excludePathsCoverHomeCorpus(t.Subject.ExcludePaths); len(missing) > 0 {
			return fmt.Errorf("task %s: derivation names the home corpus (%s) but subject.exclude_paths is missing %v — the answer key it was derived from must never sit inside this task's own worktree", t.ID, homeCorpusMarker, missing)
		}
	}
	if t.Sessions.MaxTurns <= 0 {
		return fmt.Errorf("task %s: sessions.max_turns must be > 0", t.ID)
	}
	switch t.Sessions.Mode {
	case ModeSingle:
		if t.Sessions.Count <= 0 {
			return fmt.Errorf("task %s: mode single needs sessions.count > 0", t.ID)
		}
	case ModePlannerWorkers:
		if t.Sessions.Workers <= 0 {
			return fmt.Errorf("task %s: mode planner-workers needs sessions.workers > 0", t.ID)
		}
	default:
		return fmt.Errorf("task %s: unknown sessions.mode %q (want single or planner-workers)", t.ID, t.Sessions.Mode)
	}
	return nil
}

// hasOracle reports whether o could decide anything at all.
func hasOracle(o Oracle) bool {
	return len(o.FailToPass) > 0 || len(o.PassToPass) > 0 || len(o.Detectors) > 0 ||
		len(o.AnswerAllOf) > 0 || len(o.AnswerAnyOf) > 0 || o.Localization
}

// CorpusSHA256 returns the hex SHA-256 of the corpus file's bytes — the
// identity a scored cell and a ledger row record so a later reader can
// tell whether two runs measured the same questions (mirrors
// contextbench.CorpusSHA256; `shasum -a 256 <corpus>` reproduces it with no
// tooling).
func CorpusSHA256(path string) (string, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("hashing scenario corpus %s: %w", path, err)
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:]), nil
}
