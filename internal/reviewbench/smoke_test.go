package reviewbench

import (
	"context"
	"os"
	"testing"
)

// TestSmoke_TwoRealPairs is STEP 1's "fixture-pinned smoke test over 2
// PRs": it runs the REAL harness (git fetch against the real upstream
// repos, in-process ingest, in-process review) over 2 pairs from the
// committed corpus and asserts the exact outcome
// docs/bench/readings/vb24-review-historical-prs.md's strict re-score
// already recorded for them — caddyserver/caddy #1300->#1309 is one of the
// 5 genuine strict-new hits (a caller, createUpstreamRequest, outside A's
// changed files); #2014->#2039 is an own-symbol-only pair (no strict-new
// hit). A regression in ScorePair, ExtractGold, or the review engine's own
// caller resolution would flip either outcome.
//
// Gated behind FATHOM_REVIEWBENCH=1 (network + ~1-2 min of git/ingest
// cost, the same opt-in convention internal/semantic/eval_test.go's
// FATHOM_EVAL_LLM uses) — never runs in `go test ./...`.
func TestSmoke_TwoRealPairs(t *testing.T) {
	if os.Getenv("FATHOM_REVIEWBENCH") != "1" {
		t.Skip("set FATHOM_REVIEWBENCH=1 to run the live 2-pair smoke test (network + git fetch required)")
	}
	workdir := t.TempDir()
	corpus, err := LoadCorpus("testdata/historical-prs-smoke.json")
	if err != nil {
		t.Fatalf("LoadCorpus: %v", err)
	}
	if len(corpus.Pairs) != 2 {
		t.Fatalf("smoke fixture has %d pairs, want 2", len(corpus.Pairs))
	}

	runner := Runner{Git: ExecGit, ReposDir: workdir + "/repos", StoresDir: workdir + "/stores"}
	ctx := context.Background()

	want := map[string]bool{
		"caddyserver-caddy-1300-1309": true,  // genuine strict-new hit (caller: createUpstreamRequest)
		"caddyserver-caddy-2014-2039": false, // own-symbol-only, no strict-new hit
	}
	for _, p := range corpus.Pairs {
		result, storePath, err := runner.Run(ctx, p)
		defer os.Remove(storePath)
		if err != nil {
			t.Fatalf("%s: Run: %v", p.Key(), err)
		}
		score := ScorePair(p, result.ChangedFiles, result.ChangedSymbols, result.TestGap)
		if got, wantHit := score.StrictNewHit(), want[p.Key()]; got != wantHit {
			t.Errorf("%s: StrictNewHit() = %v, want %v (gold=%d matches=%+v)", p.Key(), got, wantHit, len(score.Gold), score.Matches)
		}
	}
}
