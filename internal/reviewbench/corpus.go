// Package reviewbench reproduces VB-24/M15's historical-PR review-recall
// bench offline and deterministically: for each "regression PR A, fix PR B"
// pair in internal/analysis/review/testdata/historical-prs.json, check out
// A's base commit of the ORIGINAL upstream repo (caddyserver/caddy,
// golangci/golangci-lint — fetched fresh into a scratch git checkout, never
// this repo), ingest it into a throwaway store, run the SAME
// internal/analysis/review engine `fathom review --format github` uses over
// A's diff, and score whether the result names a (file, function) B's fix
// later touched.
//
// This package spends no LLM tokens and talks to no daemon — git fetch of
// two commits per pair plus an in-process ingest/review, exactly the
// docs/bench/readings/vb24-review-historical-prs.md "Setup, per pair"
// method, but scored against the review engine's own structured Go data
// (ChangedSymbolReviews, impact.TestGap) instead of re-parsing rendered
// Markdown — the 2026-09-17 "strict re-score" found the original bench's
// CALLER_RE regex never matched a single real nested-caller citation, so
// scoring off structured data instead of rendered text closes that
// regression class permanently.
package reviewbench

import (
	"encoding/json"
	"fmt"
	"os"
	"time"
)

// PRFile is one file GitHub's `pulls/{n}/files` reported changed, with its
// unified-diff patch text (nil for a file GitHub omits a patch for — binary,
// or too large).
type PRFile struct {
	Filename string  `json:"filename"`
	Patch    *string `json:"patch"`
}

// PRRef is one pull request's identity: number, title, the two commits that
// matter (its merge commit and the commit it merged onto), and the files it
// changed.
type PRRef struct {
	Number       int      `json:"number"`
	Title        string   `json:"title"`
	MergeCommit  string   `json:"merge_commit"`
	BaseCommit   string   `json:"base_commit"`
	MergedAt     string   `json:"merged_at"`
	FilesChanged []string `json:"files_changed,omitempty"`
}

// PRBRef is PR B: same identity shape as PRRef, but FilesChanged carries
// each file's patch text (the gold source — ExtractGold reads it).
type PRBRef struct {
	Number       int      `json:"number"`
	Title        string   `json:"title"`
	MergeCommit  string   `json:"merge_commit"`
	BaseCommit   string   `json:"base_commit"`
	MergedAt     string   `json:"merged_at"`
	FilesChanged []PRFile `json:"files_changed"`
}

// Pair is one "regression PR A, fix PR B" pair mined from a public Go repo.
type Pair struct {
	Repo     string `json:"repo"` // "owner/name", e.g. "caddyserver/caddy"
	Language string `json:"language"`
	FoundVia string `json:"found_via"`
	PRA      struct {
		PRRef
		FilesChanged []string `json:"files_changed"`
	} `json:"pr_a"`
	PRB PRBRef `json:"pr_b"`
}

// Key is a short, stable, filesystem-safe identifier for the pair — used as
// the scratch store filename and the ingested service ID.
func (p Pair) Key() string {
	repo := p.Repo
	for i := 0; i < len(repo); i++ {
		if repo[i] == '/' {
			repo = repo[:i] + "-" + repo[i+1:]
		}
	}
	return fmt.Sprintf("%s-%d-%d", repo, p.PRA.Number, p.PRB.Number)
}

// MergedAtTimes parses PRA/PRB's merged_at timestamps (RFC3339). Errors are
// swallowed to zero time — corpus well-formedness is
// TestHistoricalPRCorpusLoads's job (internal/analysis/review), not this
// package's; reviewbench only reads what that test already guarantees.
func (p Pair) MergedAtTimes() (a, b time.Time) {
	a, _ = time.Parse(time.RFC3339, p.PRA.MergedAt)
	b, _ = time.Parse(time.RFC3339, p.PRB.MergedAt)
	return a, b
}

// Corpus is the top-level testdata/historical-prs.json shape.
type Corpus struct {
	Pairs []Pair `json:"pairs"`
}

// LoadCorpus reads and parses a historical-prs.json corpus file.
func LoadCorpus(path string) (Corpus, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Corpus{}, fmt.Errorf("read %s: %w", path, err)
	}
	var c Corpus
	if err := json.Unmarshal(data, &c); err != nil {
		return Corpus{}, fmt.Errorf("unmarshal %s: %w", path, err)
	}
	return c, nil
}
