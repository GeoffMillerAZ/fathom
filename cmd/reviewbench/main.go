// Command reviewbench reproduces VB-24/M15's historical-PR review-recall
// bench offline and deterministically (docs/bench/readings/
// vb24-review-historical-prs.md; docs/plans/verbs.md M15/VB-24). For each
// pair in a historical-prs.json corpus it fetches the two upstream commits
// it needs (A's base and merge), ingests A's base into a throwaway store,
// and runs the review engine in-process over A's diff — then scores
// whether the result named a (file, function) B's later fix touched,
// against the SAME strict rule the 2026-09-17 re-score applied (a hit must
// be a caller outside A's own changed files, or a pre-existing test — not
// one of A's own changed-file symbols recited back).
//
// It spends no LLM tokens, opens no daemon connection, and writes only to
// the scratch directories it is given. Network access is required (git
// fetch against the real upstream repos named in the corpus).
//
// Usage:
//
//	reviewbench -corpus <historical-prs.json> -workdir <scratch dir> -out <results.json> [-limit N] [-repo <owner/name>]
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"sort"

	"github.com/geoffmilleraz/fathom/internal/reviewbench"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("reviewbench", flag.ContinueOnError)
	corpusPath := fs.String("corpus", "internal/analysis/review/testdata/historical-prs.json", "path to a historical-prs.json corpus")
	workdir := fs.String("workdir", "", "scratch directory for repo checkouts and throwaway stores (required)")
	outPath := fs.String("out", "", "write the per-pair results as JSON here (optional; written once, at the end)")
	jsonlPath := fs.String("jsonl", "", "append one JSON-encoded PairScore per pair here as it completes (optional; survives an interrupted run, unlike -out)")
	limit := fs.Int("limit", 0, "stop after N pairs (0 = all)")
	repoFilter := fs.String("repo", "", "only run pairs from this repo (owner/name)")
	keepStores := fs.Bool("keep-stores", false, "do not delete each pair's throwaway store after scoring (debugging)")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *workdir == "" {
		fmt.Fprintln(stderr, "error: -workdir is required")
		return 2
	}

	corpus, err := reviewbench.LoadCorpus(*corpusPath)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}

	reposDir := *workdir + "/repos"
	storesDir := *workdir + "/stores"
	if err := os.MkdirAll(reposDir, 0o755); err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}
	if err := os.MkdirAll(storesDir, 0o755); err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}

	runner := reviewbench.Runner{Git: reviewbench.ExecGit, ReposDir: reposDir, StoresDir: storesDir}
	ctx := context.Background()

	var jsonlEnc *json.Encoder
	if *jsonlPath != "" {
		f, err := os.Create(*jsonlPath)
		if err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			return 1
		}
		defer f.Close()
		jsonlEnc = json.NewEncoder(f)
	}

	var scores []reviewbench.PairScore
	ran := 0
	for _, p := range corpus.Pairs {
		if *repoFilter != "" && p.Repo != *repoFilter {
			continue
		}
		if *limit > 0 && ran >= *limit {
			break
		}
		ran++
		fmt.Fprintf(stdout, "== %s #%d -> #%d ==\n", p.Repo, p.PRA.Number, p.PRB.Number)
		result, storePath, err := runner.Run(ctx, p)
		// Removed IMMEDIATELY after this pair, not deferred to the end of
		// the run — a `defer` inside a for loop only fires when the
		// ENCLOSING FUNCTION returns, so every pair's throwaway store
		// accumulated on disk for the whole 42-pair run until this fix
		// (2026-09-18: caught mid-run when the shared host disk hit <1GB
		// free — see the reading's "Deviations" section).
		if !*keepStores {
			os.Remove(storePath)
			os.Remove(storePath + "-wal")
			os.Remove(storePath + "-shm")
		}
		if err != nil {
			fmt.Fprintf(stderr, "  error: %v\n", err)
			continue
		}
		score := reviewbench.ScorePair(p, result.ChangedFiles, result.ChangedSymbols, result.TestGap)
		scores = append(scores, score)
		fmt.Fprintf(stdout, "  nodes=%d typed_edges=%d changed_files=%d gold=%d strict_new_hit=%v\n",
			result.NodeCount, result.TypedEdgeCount, len(result.ChangedFiles), len(score.Gold), score.StrictNewHit())
		// Streamed per-pair output (-jsonl): survives a kill/crash partway
		// through the corpus — the same lesson as the store-cleanup fix
		// above, learned the same run.
		if jsonlEnc != nil {
			if err := jsonlEnc.Encode(score); err != nil {
				fmt.Fprintf(stderr, "error: writing -jsonl: %v\n", err)
				return 1
			}
		}
	}

	summary := summarize(scores)
	fmt.Fprintf(stdout, "\n%s\n", summary.String())

	if *outPath != "" {
		f, err := os.Create(*outPath)
		if err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			return 1
		}
		defer f.Close()
		enc := json.NewEncoder(f)
		enc.SetIndent("", "  ")
		if err := enc.Encode(map[string]any{"summary": summary, "pairs": scores}); err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			return 1
		}
	}
	return 0
}

// Summary is the aggregate recall/precision table reviewbench prints and
// (optionally) writes to -out.
type Summary struct {
	Pairs         int            `json:"pairs"`
	StrictNewHits int            `json:"strict_new_hits"`
	OwnSymbolOnly int            `json:"own_symbol_only"`
	PureMisses    int            `json:"pure_misses"`
	ByRepo        map[string]int `json:"strict_new_hits_by_repo"`
}

func (s Summary) String() string {
	rate := 0.0
	if s.Pairs > 0 {
		rate = 100 * float64(s.StrictNewHits) / float64(s.Pairs)
	}
	repos := make([]string, 0, len(s.ByRepo))
	for r := range s.ByRepo {
		repos = append(repos, r)
	}
	sort.Strings(repos)
	line := fmt.Sprintf("pairs=%d strict_new_hits=%d (%.1f%%) own_symbol_only=%d pure_misses=%d", s.Pairs, s.StrictNewHits, rate, s.OwnSymbolOnly, s.PureMisses)
	for _, r := range repos {
		line += fmt.Sprintf(" %s=%d", r, s.ByRepo[r])
	}
	return line
}

func summarize(scores []reviewbench.PairScore) Summary {
	s := Summary{Pairs: len(scores), ByRepo: map[string]int{}}
	for _, sc := range scores {
		if sc.StrictNewHit() {
			s.StrictNewHits++
			s.ByRepo[sc.Pair.Repo]++
		} else if sc.AnyHit() {
			s.OwnSymbolOnly++
		} else {
			s.PureMisses++
		}
	}
	return s
}
