package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/geoffmilleraz/app-kit/internal/twplus/index"
)

const defaultGoldenPath = "data/tailwindplus/golden-queries.json"

// goldenSet is the committed known-item retrieval suite (the G2 quality proof).
type goldenSet struct {
	K       int           `json:"k"`
	Queries []goldenQuery `json:"queries"`
}

type goldenQuery struct {
	Q               string `json:"q"`
	Section         string `json:"section,omitempty"`
	Framework       string `json:"framework,omitempty"`
	TailwindVersion int    `json:"tailwind_version,omitempty"`
	Mode            string `json:"mode,omitempty"`
	Expect          string `json:"expect"`
}

// dataEval runs the golden set against the index and asserts every query ranks
// its expected component within the top-K distinct full_names. Non-zero exit on
// any miss, so it gates like a test.
func dataEval(args []string) error {
	fs := flag.NewFlagSet("appkit data eval", flag.ContinueOnError)
	idxPath := fs.String("index", defaultIndexPath, "path to the SQLite index")
	goldenPath := fs.String("golden", defaultGoldenPath, "golden query set (JSON)")
	kFlag := fs.Int("k", 0, "top-K threshold (0 = use the set's k, else 3)")
	if err := fs.Parse(args); err != nil {
		return err
	}

	raw, err := os.ReadFile(*goldenPath)
	if err != nil {
		return fmt.Errorf("data eval: read golden: %w", err)
	}
	var gs goldenSet
	if err := json.Unmarshal(raw, &gs); err != nil {
		return fmt.Errorf("data eval: parse golden: %w", err)
	}
	k := gs.K
	if *kFlag > 0 {
		k = *kFlag
	}
	if k <= 0 {
		k = 3
	}

	ix, err := index.Open(*idxPath)
	if err != nil {
		return err
	}
	defer ix.Close()

	passed := 0
	for _, q := range gs.Queries {
		res, err := ix.SearchComponents(q.Q, index.Filters{
			Section: q.Section, Framework: q.Framework, TailwindVersion: q.TailwindVersion, Mode: q.Mode,
		}, 50)
		if err != nil {
			return err
		}
		rank := distinctRank(res, q.Expect)
		ok := rank > 0 && rank <= k
		mark, rankStr := "FAIL", "—"
		if ok {
			mark, passed = "ok", passed+1
		}
		if rank > 0 {
			rankStr = fmt.Sprintf("#%d", rank)
		}
		fmt.Printf("[%-4s] %-3s  %-44s → %s\n", mark, rankStr, q.Q, q.Expect)
	}
	total := len(gs.Queries)
	fmt.Printf("\n%d/%d known-items in top-%d\n", passed, total, k)
	if passed < total {
		return fmt.Errorf("data eval: %d/%d golden queries missed top-%d", total-passed, total, k)
	}
	return nil
}

// distinctRank is the 1-based position of fullName among distinct full_names in
// score order, or 0 if absent (variants of one component collapse to one rank).
func distinctRank(res []index.SearchResult, fullName string) int {
	seen := map[string]bool{}
	rank := 0
	for _, r := range res {
		if seen[r.FullName] {
			continue
		}
		seen[r.FullName] = true
		rank++
		if r.FullName == fullName {
			return rank
		}
	}
	return 0
}
