package review_test

import (
	"encoding/json"
	"os"
	"regexp"
	"testing"
	"time"
)

// TestHistoricalPRCorpusLoads is VB-24's (docs/plans/verbs.md wave F)
// loader test for testdata/historical-prs.json: a corpus of 50-60
// "merged-then-fixed" PR pairs from two public Go repositories
// (golangci/golangci-lint, caddyserver/caddy), mined for
// docs/bench/readings/vb24-review-historical-prs.md's `fathom review
// --format github` vs. a model-reviewer bench. This test only proves the
// corpus is well-formed and internally consistent (every field present,
// shas look like shas, A really merged before B, at least one file overlaps)
// — the SCORING itself (did the review comment name the call site B later
// fixed) happens offline in the bench script, not here; this is a fixture
// sanity check, not a re-run of the bench.
var shaRE = regexp.MustCompile(`^[0-9a-f]{40}$`)

type historicalPR struct {
	Number      int      `json:"number"`
	Title       string   `json:"title"`
	MergeCommit string   `json:"merge_commit"`
	BaseCommit  string   `json:"base_commit"`
	MergedAt    string   `json:"merged_at"`
	FilesA      []string `json:"files_changed,omitempty"`
}

type historicalPRFile struct {
	Filename string  `json:"filename"`
	Patch    *string `json:"patch"`
}

type historicalPairRaw struct {
	Repo     string `json:"repo"`
	Language string `json:"language"`
	FoundVia string `json:"found_via"`
	PRA      struct {
		historicalPR
		FilesChanged []string `json:"files_changed"`
	} `json:"pr_a"`
	PRB struct {
		Number       int                `json:"number"`
		Title        string             `json:"title"`
		MergeCommit  string             `json:"merge_commit"`
		BaseCommit   string             `json:"base_commit"`
		MergedAt     string             `json:"merged_at"`
		FilesChanged []historicalPRFile `json:"files_changed"`
	} `json:"pr_b"`
}

type historicalCorpus struct {
	Pairs []historicalPairRaw `json:"pairs"`
}

func loadHistoricalCorpus(t *testing.T) historicalCorpus {
	t.Helper()
	data, err := os.ReadFile("testdata/historical-prs.json")
	if err != nil {
		t.Fatalf("read testdata/historical-prs.json: %v", err)
	}
	var c historicalCorpus
	if err := json.Unmarshal(data, &c); err != nil {
		t.Fatalf("unmarshal testdata/historical-prs.json: %v", err)
	}
	return c
}

func TestHistoricalPRCorpusLoads(t *testing.T) {
	c := loadHistoricalCorpus(t)
	if len(c.Pairs) < 20 {
		t.Fatalf("expected at least 20 historical PR pairs (VB-24's floor is >=30 total; this is a hard sanity floor), got %d", len(c.Pairs))
	}

	seenPerRepo := map[string]map[int]bool{}
	byRepo := map[string]int{}

	for i, p := range c.Pairs {
		if p.Repo == "" {
			t.Fatalf("pair %d: empty repo", i)
		}
		if p.Language != "go" {
			t.Errorf("pair %d (%s): expected language \"go\", got %q", i, p.Repo, p.Language)
		}
		if p.PRA.Number == 0 || p.PRB.Number == 0 {
			t.Fatalf("pair %d (%s): pr_a/pr_b number must be set", i, p.Repo)
		}
		if p.PRA.Number == p.PRB.Number {
			t.Fatalf("pair %d (%s): pr_a and pr_b are the same PR #%d", i, p.Repo, p.PRA.Number)
		}
		if !shaRE.MatchString(p.PRA.MergeCommit) {
			t.Errorf("pair %d (%s#%d): pr_a.merge_commit %q is not a 40-char sha", i, p.Repo, p.PRA.Number, p.PRA.MergeCommit)
		}
		if !shaRE.MatchString(p.PRA.BaseCommit) {
			t.Errorf("pair %d (%s#%d): pr_a.base_commit %q is not a 40-char sha", i, p.Repo, p.PRA.Number, p.PRA.BaseCommit)
		}
		if !shaRE.MatchString(p.PRB.MergeCommit) {
			t.Errorf("pair %d (%s#%d): pr_b.merge_commit %q is not a 40-char sha", i, p.Repo, p.PRB.Number, p.PRB.MergeCommit)
		}
		if p.PRA.MergeCommit == p.PRB.MergeCommit {
			t.Errorf("pair %d (%s): pr_a and pr_b share the same merge commit", i, p.Repo)
		}

		ta, err := time.Parse(time.RFC3339, p.PRA.MergedAt)
		if err != nil {
			t.Errorf("pair %d (%s#%d): pr_a.merged_at %q: %v", i, p.Repo, p.PRA.Number, p.PRA.MergedAt, err)
		}
		tb, err := time.Parse(time.RFC3339, p.PRB.MergedAt)
		if err != nil {
			t.Errorf("pair %d (%s#%d): pr_b.merged_at %q: %v", i, p.Repo, p.PRB.Number, p.PRB.MergedAt, err)
		}
		if !ta.IsZero() && !tb.IsZero() && !ta.Before(tb) {
			t.Errorf("pair %d (%s): pr_a (#%d, %s) did not merge before pr_b (#%d, %s)",
				i, p.Repo, p.PRA.Number, ta, p.PRB.Number, tb)
		}

		if len(p.PRA.FilesChanged) == 0 {
			t.Errorf("pair %d (%s#%d): pr_a has no files_changed", i, p.Repo, p.PRA.Number)
		}
		if len(p.PRB.FilesChanged) == 0 {
			t.Errorf("pair %d (%s#%d): pr_b has no files_changed", i, p.Repo, p.PRB.Number)
		}

		aFiles := map[string]bool{}
		for _, f := range p.PRA.FilesChanged {
			aFiles[f] = true
		}
		overlap := false
		for _, f := range p.PRB.FilesChanged {
			if f.Filename == "" {
				t.Errorf("pair %d (%s#%d): pr_b has an empty filename entry", i, p.Repo, p.PRB.Number)
				continue
			}
			if aFiles[f.Filename] {
				overlap = true
			}
		}
		if !overlap {
			t.Errorf("pair %d (%s): pr_a#%d and pr_b#%d share no changed file (required overlap)", i, p.Repo, p.PRA.Number, p.PRB.Number)
		}

		if seenPerRepo[p.Repo] == nil {
			seenPerRepo[p.Repo] = map[int]bool{}
		}
		if seenPerRepo[p.Repo][p.PRA.Number] {
			t.Errorf("pair %d (%s): duplicate pr_a #%d", i, p.Repo, p.PRA.Number)
		}
		seenPerRepo[p.Repo][p.PRA.Number] = true
		byRepo[p.Repo]++
	}

	if len(byRepo) < 2 {
		t.Errorf("expected pairs from at least 2 repositories, got %d: %v", len(byRepo), byRepo)
	}
	t.Logf("historical PR corpus: %d pairs across %d repos: %v", len(c.Pairs), len(byRepo), byRepo)
}
