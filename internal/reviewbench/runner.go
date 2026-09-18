package reviewbench

import (
	"context"
	"fmt"
	"os"

	"github.com/geoffmilleraz/fathom/internal/analysis/impact"
	"github.com/geoffmilleraz/fathom/internal/analysis/review"
	"github.com/geoffmilleraz/fathom/pkg/client"
)

// Runner runs one pair end-to-end: shallow-fetch A's base and merge commits
// of the upstream repo into a reused per-repo checkout, ingest A's base
// state into a throwaway store, check out A's merge commit, resolve the
// changed files, and run the SAME review.ChangedSymbolReviews /
// impact.TestGap composition `fathom review --format github` renders —
// in-process library calls, never the built CLI binary (STEP 1's brief).
type Runner struct {
	Git       GitRunner
	ReposDir  string // one subdirectory per upstream repo, reused across pairs
	StoresDir string // one throwaway .db per pair, safe to delete after scoring
}

// RunResult is one pair's resolved review inputs and outputs — everything
// ScorePair needs, plus enough to debug a pair by hand.
type RunResult struct {
	Pair           Pair
	ChangedFiles   []string
	ChangedSymbols []review.ChangedSymbolReview
	TestGap        impact.TestGapResult
	NodeCount      int
	TypedEdgeCount int
}

// Run executes one pair. storePath is returned so the caller can remove it
// once scoring is done (kept on disk during the call only).
func (r Runner) Run(ctx context.Context, p Pair) (RunResult, string, error) {
	repoDir := r.ReposDir + "/" + sanitizeRepo(p.Repo)
	if err := EnsureRepo(r.Git, repoDir, p.Repo); err != nil {
		return RunResult{}, "", fmt.Errorf("ensure repo %s: %w", p.Repo, err)
	}
	if err := FetchCommit(r.Git, repoDir, p.PRA.BaseCommit); err != nil {
		return RunResult{}, "", fmt.Errorf("fetch base %s: %w", p.PRA.BaseCommit, err)
	}
	if err := FetchCommit(r.Git, repoDir, p.PRA.MergeCommit); err != nil {
		return RunResult{}, "", fmt.Errorf("fetch merge %s: %w", p.PRA.MergeCommit, err)
	}
	if err := CheckoutCommit(r.Git, repoDir, p.PRA.BaseCommit); err != nil {
		return RunResult{}, "", fmt.Errorf("checkout base %s: %w", p.PRA.BaseCommit, err)
	}

	storePath := r.StoresDir + "/" + p.Key() + ".db"
	os.Remove(storePath)
	c, err := client.NewConstantClient(client.ModeLocal, storePath)
	if err != nil {
		return RunResult{}, storePath, fmt.Errorf("open store: %w", err)
	}
	defer c.Close()

	rep, err := c.Ingest(ctx, client.ServiceRef{ID: p.Key(), Path: repoDir})
	if err != nil {
		return RunResult{}, storePath, fmt.Errorf("ingest %s at %s: %w", p.Key(), p.PRA.BaseCommit, err)
	}
	if os.Getenv("REVIEWBENCH_DEBUG") != "" {
		st, known := c.TypedEdgeStatus(p.Key())
		fmt.Printf("  [debug] symbols=%d typed_pkgs=%d typed_pkgs_failed=%d typed_direct=%d typed_iface=%d typed_unresolved=%d status_known=%v failed=%v reason=%q fallback=%q\n",
			rep.Symbols, rep.TypedPackages, rep.TypedPackagesFailed, rep.TypedEdgesDirect, rep.TypedEdgesIfaceDispatch, rep.TypedEdgesUnresolved, known, st.Failed, st.Reason, st.Fallback)
	}

	if err := CheckoutCommit(r.Git, repoDir, p.PRA.MergeCommit); err != nil {
		return RunResult{}, storePath, fmt.Errorf("checkout merge %s: %w", p.PRA.MergeCommit, err)
	}
	changedFiles, err := ChangedFilesSince(r.Git, repoDir, p.PRA.BaseCommit)
	if err != nil {
		return RunResult{}, storePath, fmt.Errorf("diff since %s: %w", p.PRA.BaseCommit, err)
	}
	if len(changedFiles) == 0 {
		return RunResult{}, storePath, fmt.Errorf("no changed files resolved between %s and %s", p.PRA.BaseCommit, p.PRA.MergeCommit)
	}

	nodes, err := c.ListAllSymbols(ctx, p.Key())
	if err != nil {
		return RunResult{}, storePath, fmt.Errorf("list symbols: %w", err)
	}
	typed, _ := c.TypedEdges(ctx, p.Key())

	res := RunResult{
		Pair:           p,
		ChangedFiles:   changedFiles,
		ChangedSymbols: review.ChangedSymbolReviews(nodes, typed, changedFiles),
		TestGap:        impact.TestGap(nodes, changedFiles, typed),
		NodeCount:      len(nodes),
		TypedEdgeCount: len(typed),
	}
	return res, storePath, nil
}

func sanitizeRepo(repo string) string {
	out := []byte(repo)
	for i, b := range out {
		if b == '/' {
			out[i] = '-'
		}
	}
	return string(out)
}
