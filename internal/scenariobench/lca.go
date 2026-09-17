// lca.go is VB-25 (docs/vision/29-verbs-not-lookups.md Wave G; plan
// docs/plans/verbs.md VB-25): a loader for Long Code Arena's
// bug-localization track and its conversion into scenariobench Tasks
// scored by the "files" oracle (localization.go) rather than a test run —
// "which files does the fix need", never "make these tests pass".
//
// The operator downloads the dataset (`-dataset <path.jsonl>` on
// cmd/scenariobench's run/ledger subcommands, or LoadLCARecords directly)
// — this package never fetches it itself, mirroring derive.go's own rule
// for the public Go corpus's pinned snapshots. Every field this loader
// reads is named exactly as the upstream dataset card names it
// (huggingface.co/datasets/JetBrains-Research/lca-bug-localization,
// verified 2026-09-13 — docs/bench/scenarios.md "External corpus (LCA bug
// localization)" documents the exact mapping and the dataset's own
// URL/license):
//
//	repo_owner, repo_name  -> Subject.RepoURL ("https://github.com/<owner>/<name>")
//	base_sha                -> Subject.BaseCommit
//	repo_language           -> Subject.Language (informational only)
//	issue_title, issue_body -> Task.Prompt (issue_body may be empty)
//	changed_files           -> Task.Gold.Files (the gold answer set)
//	text_id                 -> Task.ID (falls back to
//	                           "<owner>__<name>-<line index>" when empty)
package scenariobench

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// LCARecord is one Long Code Arena bug-localization record, decoded from
// the upstream dataset's own column names (this file's own doc comment) —
// only the columns this loader actually uses; the dataset card lists many
// more (token/line counts, repo stars, …) this bench has no use for.
type LCARecord struct {
	TextID       string   `json:"text_id"`
	ID           int64    `json:"id"`
	RepoOwner    string   `json:"repo_owner"`
	RepoName     string   `json:"repo_name"`
	RepoLanguage string   `json:"repo_language"`
	BaseSHA      string   `json:"base_sha"`
	IssueTitle   string   `json:"issue_title"`
	IssueBody    string   `json:"issue_body"`
	ChangedFiles []string `json:"changed_files"`
}

// LoadLCARecords reads path (a JSONL file the operator downloaded — this
// package never fetches it itself) and returns every record, in file
// order. A blank line is skipped; a malformed line fails loudly, naming
// its 1-based line number, so a corrupted download is refused at load
// time rather than silently dropping rows.
func LoadLCARecords(path string) ([]LCARecord, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("loading LCA dataset %s: %w", path, err)
	}
	defer f.Close()

	var out []LCARecord
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 16*1024*1024)
	line := 0
	for sc.Scan() {
		line++
		raw := strings.TrimSpace(sc.Text())
		if raw == "" {
			continue
		}
		var rec LCARecord
		if err := json.Unmarshal([]byte(raw), &rec); err != nil {
			return nil, fmt.Errorf("loading LCA dataset %s: line %d: %w", path, line, err)
		}
		out = append(out, rec)
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("loading LCA dataset %s: %w", path, err)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("loading LCA dataset %s: no records", path)
	}
	return out, nil
}

// LCAArmFathom / LCAArmControl are the two arms BuildLCACorpus registers —
// vision 29 Wave G's own ask ("the same model with and without the agent
// surface"): fathom wired (the promoted surface, including the localize
// verb, VB-26) versus a plain control with Read/Grep/Bash only.
const (
	LCAArmFathom  = "fathom"
	LCAArmControl = "control"
)

// DefaultLCAMaxTurns bounds a bug-localization session: a pure
// read-and-answer task (change-shaped=false, no plan-first — see
// isChangeShaped), so the budget is small — R10's own SHERLOC finding is
// that agents spend about half their tool budget merely locating; this
// task measures exactly that half, not a fix.
const DefaultLCAMaxTurns = 15

// lcaPredictionFathom / lcaPredictionControl are the pre-registered
// predictions every task in a derived corpus repeats verbatim (SB-10):
// vision 29 Wave G's own bet, registered before a token is spent, never
// narrated after the fact.
const (
	lcaPredictionFathom = "The localize verb's graph-proximity, recency and " +
		"obligations ranking (VB-26) beats a cold grep-and-guess on " +
		"file-level accuracy@1, mirroring LocAgent's own reported lift " +
		"over a syntax-only graph (docs/vision/29-verbs-not-lookups.md R10)."
	lcaPredictionControl = "Without a localize verb the agent falls back to " +
		"grep and a stack-trace read, matching R10's SHERLOC finding that " +
		"roughly half the tool budget goes to locating rather than fixing."
)

// LCACriterionID is VB-27's pre-registered criterion (M12/M16): "the
// external corpus shows no lift with a confidence interval that includes
// zero: the verdict publishes the null and BC-15 closes on it" (vision 29
// §5). MinTrials mirrors VB-27's own "n >= 3" floor; the paired-statistics
// interval itself is computed by the escalation/VB-27 tooling over the
// rows this criterion's own Evaluate merely floors — a bare
// Criterion.Evaluate pass/fail is not that confidence interval.
const LCACriterionID = "lca-bug-localization-m12-m16"

// lcaCorpusDescription documents this corpus's own shape for a reader of
// the committed JSONL/JSON, mirroring derive.go's Description constant.
const lcaCorpusDescription = "Long Code Arena bug-localization track (docs/bench/scenarios.md \"External corpus (LCA bug localization)\"), vision 29 Wave G (VB-25): each task is one upstream issue at a pinned base commit, scored by the files oracle (Oracle.Localization) against the upstream's own changed-file set — never an edit. Two arms: fathom (wired, the promoted agent surface including localize) and control (Read/Grep/Bash only), registered predictions per VB-27 (M12/M16)."

// LCATaskFromRecord converts one LCARecord to a Task: change-shaped=false
// (no plan-first), the issue text as the whole prompt, Gold.Files the
// upstream's own changed-file set, and Oracle.Localization set so
// RunOracle scores the LAST session's own final answer against Gold.Files
// (ScoreLocalization) instead of running a test command. derivation names
// path and idx (the record's own position) so a stranger can re-derive
// the same task from the same file.
func LCATaskFromRecord(rec LCARecord, path string, idx int) (Task, error) {
	if rec.RepoOwner == "" || rec.RepoName == "" {
		return Task{}, fmt.Errorf("LCA record %d (%s): missing repo_owner/repo_name", idx, rec.TextID)
	}
	if rec.BaseSHA == "" {
		return Task{}, fmt.Errorf("LCA record %d (%s): missing base_sha", idx, rec.TextID)
	}
	if len(rec.ChangedFiles) == 0 {
		return Task{}, fmt.Errorf("LCA record %d (%s): missing changed_files — nothing for the files oracle to score against", idx, rec.TextID)
	}
	id := rec.TextID
	if id == "" {
		id = fmt.Sprintf("%s__%s-%d", rec.RepoOwner, rec.RepoName, idx)
	}

	prompt := "The following issue was filed against " + rec.RepoOwner + "/" + rec.RepoName +
		" at commit " + rec.BaseSHA + ". Name every file you believe the fix will need to " +
		"change, most likely first. Do not edit anything — this task is answered by naming " +
		"files, never by a patch.\n\nIssue title: " + rec.IssueTitle
	if rec.IssueBody != "" {
		prompt += "\n\n" + rec.IssueBody
	}

	gold := dedupSorted(rec.ChangedFiles)

	return Task{
		ID:       id,
		Category: CategoryLocalization,
		Prompt:   prompt,
		Subject: Subject{
			RepoURL:    "https://github.com/" + rec.RepoOwner + "/" + rec.RepoName,
			BaseCommit: rec.BaseSHA,
			Service:    "lca-" + rec.RepoOwner + "-" + rec.RepoName,
			Language:   rec.RepoLanguage,
		},
		Sessions: Sessions{Mode: ModeSingle, Count: 1, MaxTurns: DefaultLCAMaxTurns},
		Oracle:   Oracle{Localization: true},
		Gold:     Gold{Files: gold},
		Predictions: map[string]string{
			LCAArmFathom:  lcaPredictionFathom,
			LCAArmControl: lcaPredictionControl,
		},
		Derivation: fmt.Sprintf("Long Code Arena bug-localization track, record %d of %s (text_id %q) — docs/bench/scenarios.md \"External corpus (LCA bug localization)\".", idx, path, id),
	}, nil
}

// BuildLCACorpus converts every record (in order) to a Task
// (LCATaskFromRecord) and assembles the Corpus BuildLCACorpus's own two
// arms (LCAArmFathom, LCAArmControl) and criterion (LCACriterionID) —
// validated exactly as a committed corpus file is (validateCorpus), so a
// malformed dataset is refused before a cell is planned, not discovered
// mid-run.
func BuildLCACorpus(records []LCARecord, path string) (Corpus, error) {
	if len(records) == 0 {
		return Corpus{}, fmt.Errorf("building LCA corpus from %s: no records", path)
	}
	tasks := make([]Task, 0, len(records))
	seen := make(map[string]bool, len(records))
	for i, rec := range records {
		task, err := LCATaskFromRecord(rec, path, i)
		if err != nil {
			return Corpus{}, fmt.Errorf("building LCA corpus from %s: %w", path, err)
		}
		if seen[task.ID] {
			return Corpus{}, fmt.Errorf("building LCA corpus from %s: duplicate task id %s (record %d)", path, task.ID, i)
		}
		seen[task.ID] = true
		tasks = append(tasks, task)
	}

	c := Corpus{
		Version:     1,
		Description: lcaCorpusDescription,
		Arms: map[string]Arm{
			LCAArmFathom: {
				Wired:        true,
				AllowedTools: []string{"Read", "Grep", "Bash"},
				Instructions: "stanza",
				Surface:      "fathom",
				Prediction:   lcaPredictionFathom,
			},
			LCAArmControl: {
				Wired:        false,
				AllowedTools: []string{"Read", "Grep", "Bash"},
				Instructions: "plain",
				Surface:      "",
				Prediction:   lcaPredictionControl,
			},
		},
		Tasks: tasks,
		Criterion: Criterion{
			ID:         LCACriterionID,
			Treatment:  LCAArmFathom,
			Control:    LCAArmControl,
			MinTrials:  3,
			Registered: "docs/vision/29-verbs-not-lookups.md §5 (VB-27, M12/M16)",
		},
		Sources: []SourcePin{{
			Name:    "lca-bug-localization",
			URL:     lcaDatasetURL,
			License: lcaDatasetLicense,
		}},
	}
	if err := validateCorpus(c); err != nil {
		return Corpus{}, fmt.Errorf("building LCA corpus from %s: %w", path, err)
	}
	return c, nil
}

// lcaDatasetURL / lcaDatasetLicense are recorded on every derived
// corpus's own Sources (SourcePin) so a stranger can re-derive without
// reading this file. Verified 2026-09-13 against the dataset card
// (docs/bench/scenarios.md "External corpus (LCA bug localization)" names
// how); re-verify before relying on this at a later date — a license tag
// on a dataset card can change.
const (
	lcaDatasetURL     = "https://huggingface.co/datasets/JetBrains-Research/lca-bug-localization"
	lcaDatasetLicense = "Apache-2.0 (verified 2026-09-13; see docs/bench/scenarios.md)"
)

// ExternalCorpusCriteria returns the registered Criterion for every
// external corpus this package builds directly from an operator-supplied
// dataset (currently only Long Code Arena bug localization, VB-25) —
// keyed by the criterion's own ID (LCACriterionID), never by a corpus
// SHA256, because that sha exists only once the operator's own -dataset
// file is loaded (CanonicalCorpusSHA256) and cmd/scenariobench's own
// criteriaBySHA256 glob (testdata/scenariobench/*.json) can never
// discover it — an external corpus is never committed as a corpus JSON
// file there.
//
// wave N15 item 1's own fix: without this, a filed LCA ledger row (which
// DOES carry the correct CriterionID, stamped at filing time) rendered
// "(no registered criterion for this corpus)" in the verdict block —
// verdict.go's criterionByID scans criteria's VALUES for a matching ID,
// so merging this map's values into whatever criteriaBySHA256 already
// loaded (any key works; only the ID inside each Criterion is ever
// compared) is enough to resolve it.
func ExternalCorpusCriteria() map[string]Criterion {
	return map[string]Criterion{
		LCACriterionID: {
			ID:         LCACriterionID,
			Treatment:  LCAArmFathom,
			Control:    LCAArmControl,
			MinTrials:  3,
			Registered: "docs/vision/29-verbs-not-lookups.md §5 (VB-27, M12/M16)",
		},
	}
}

// CanonicalCorpusSHA256 hashes c's own canonical JSON encoding
// (canonicalMarshal, derive.go's two-space-indent/no-HTML-escaping
// convention) — the identity a corpus built in memory from an operator's
// dataset (never a single committed file CorpusSHA256 could hash) needs
// for the SAME cell-key/ledger-row purpose CorpusSHA256 serves a
// committed corpus file.
func CanonicalCorpusSHA256(c Corpus) (string, error) {
	raw, err := canonicalMarshal(c)
	if err != nil {
		return "", fmt.Errorf("hashing LCA corpus: %w", err)
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:]), nil
}
