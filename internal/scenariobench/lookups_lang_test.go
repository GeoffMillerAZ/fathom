// SU-23 (docs/plans/substrate.md section F, ADR-041 decision 9): the two
// non-Go language lookups corpora — testdata/scenariobench/lookups-ts.json
// (nest, TypeScript) and testdata/scenariobench/lookups-py.json (fastapi,
// Python), both calibration/corpus.yaml subjects at their pinned commit.
// TestLanguageLookupsCorporaParse is this row's own symbol (the plan row's
// {symbol: TestLanguageLookupsCorporaParse} contract — the row's symbol IS
// the test, skip_test: true): it loads both through LoadScenarioCorpus (the
// same load-and-validate path every committed corpus goes through, so a
// missing prediction/oracle/derivation already fails there) and then checks
// the properties specific to this pair of corpora that LoadScenarioCorpus
// does not: exactly the two arms this design calls for (full, grep), the
// registered criterion shape, and — the truth-at-head / prompt-names-anchor
// rule lookups-vs.json's own H1 tasks follow by construction — that no
// task's prompt already names the very file its own oracle answer cites.
package scenariobench

import (
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// languageLookupsCorpora is the pair SU-23 adds, path relative to this
// package directory (the repo-wide testdata/scenariobench/, not a
// package-local testdata/).
func languageLookupsCorpora() []string {
	return []string{
		filepath.Join("..", "..", "testdata", "scenariobench", "lookups-ts.json"),
		filepath.Join("..", "..", "testdata", "scenariobench", "lookups-py.json"),
	}
}

// citationFileRe matches an oracle answer token shaped like a file citation
// — "<path>#L<n>" — the same "path#Lnn" form citation.go parses. The
// captured group is the file path alone, without the line anchor.
var citationFileRe = regexp.MustCompile(`^([\w./-]+)#L\d+$`)

func TestLanguageLookupsCorporaParse(t *testing.T) {
	for _, path := range languageLookupsCorpora() {
		path := path
		t.Run(filepath.Base(path), func(t *testing.T) {
			c, err := LoadScenarioCorpus(path)
			if err != nil {
				t.Fatalf("LoadScenarioCorpus(%s): %v", path, err)
			}

			// Exactly two arms: full (fathom, wired) and grep (no MCP at
			// all) — the Go corpus's graphify/serena/gopls competitors are
			// Go/LSP-specific and do not apply to a TypeScript or Python
			// subject (docs/bench/competitors.md).
			wantArms := []string{"full", "grep"}
			if len(c.Arms) != len(wantArms) {
				t.Errorf("%s: got %d arms, want %d: %v", path, len(c.Arms), len(wantArms), c.Arms)
			}
			for _, name := range wantArms {
				if _, ok := c.Arms[name]; !ok {
					t.Errorf("%s: arm %q missing", path, name)
				}
			}
			full := c.Arms["full"]
			if !full.Wired || full.Instructions != "stanza" || full.Surface != "fathom" {
				t.Errorf("%s: full arm = %+v, want wired=true instructions=stanza surface=fathom", path, full)
			}
			grep := c.Arms["grep"]
			if grep.Wired || grep.Instructions != "plain" || grep.Surface != "" {
				t.Errorf("%s: grep arm = %+v, want wired=false instructions=plain surface=\"\"", path, grep)
			}

			// Criterion: fathom vs grep, min_trials 2 (ADR-041 decision 9 /
			// SU-23's own DELIVER text), registered before any spend.
			if c.Criterion.Treatment != "full" || c.Criterion.Control != "grep" {
				t.Errorf("%s: criterion treatment/control = %q/%q, want full/grep", path, c.Criterion.Treatment, c.Criterion.Control)
			}
			if c.Criterion.MinTrials < 2 {
				t.Errorf("%s: criterion.min_trials = %d, want >= 2", path, c.Criterion.MinTrials)
			}
			if c.Criterion.Registered == "" {
				t.Errorf("%s: criterion.registered is empty", path)
			}
			if len(c.Tasks) < 6 {
				t.Errorf("%s: got %d tasks, want >= 6", path, len(c.Tasks))
			}

			for _, task := range c.Tasks {
				// LoadScenarioCorpus already refused a task with a missing
				// prediction for a declared arm; re-assert here so this
				// test's own failure message names the corpus/task
				// directly rather than relying on the load error above.
				for _, arm := range wantArms {
					if strings.TrimSpace(task.Predictions[arm]) == "" {
						t.Errorf("%s: task %s has no prediction for arm %q", path, task.ID, arm)
					}
				}

				// Truth-at-head / prompt-names-anchor rule: collect every
				// file citation ("path#Lnn") this task's own oracle names
				// as a required answer, and refuse a prompt that already
				// names that same file path — the gold citation's file
				// must be something the agent locates, never something
				// the prompt hands it.
				for _, gold := range citationFilesInOracle(task.Oracle) {
					if strings.Contains(task.Prompt, gold) {
						t.Errorf("%s: task %s prompt names its own gold file %q — the prompt must never pre-name the file its oracle cites", path, task.ID, gold)
					}
				}
			}
		})
	}
}

// citationFilesInOracle returns the distinct file paths named by every
// "path#Lnn"-shaped token in o's AnswerAllOf and AnswerAnyOf groups.
func citationFilesInOracle(o Oracle) []string {
	seen := map[string]bool{}
	var files []string
	add := func(tok string) {
		m := citationFileRe.FindStringSubmatch(tok)
		if m == nil {
			return
		}
		if !seen[m[1]] {
			seen[m[1]] = true
			files = append(files, m[1])
		}
	}
	for _, tok := range o.AnswerAllOf {
		add(tok)
	}
	for _, group := range o.AnswerAnyOf {
		for _, tok := range group {
			add(tok)
		}
	}
	return files
}
