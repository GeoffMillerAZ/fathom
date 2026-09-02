package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/contextbench/publicloc"
)

// cmdImport derives the public localization corpus from pinned local
// snapshots (docs/plans/bench-public.md PB-14; runbook docs/bench/
// public-loc.md). It never touches the network: the operator fetches the
// two JSONL snapshots and the per-instance base_commit checkouts first,
// and this command turns them into testdata/contextbench/tasks-public-go.json
// deterministically — the same inputs reproduce the same corpus_sha256.
//
//	contextbenchscore import [-multi-swe <jsonl> [-multi-swe-revision <rev>]]
//	                         [-swe-explore <jsonl> [-swe-explore-revision <rev>]]
//	                         -checkouts <dir> [-callers-dir <dir>] -out <path>
//	                         [-n 35] [-unregistered-n]
//
// -checkouts/<instance_id>/ is a checkout of the subject at base_commit
// (function_localization is read from it with go/parser).
// -callers-dir/<instance_id>.json is cmd/loctruth's output for that
// checkout (find_callers truth); without it no find_callers tasks are
// derived and the omission is recorded per instance, never silent.
// An -n outside the pre-registered 30–40 window is refused unless
// -unregistered-n is passed, and the corpus then says pre_registered=false.
func cmdImport(args []string) int {
	fs := flag.NewFlagSet("import", flag.ContinueOnError)
	multiPath := fs.String("multi-swe", "", "pinned Multi-SWE-bench Go JSONL snapshot (CC0-1.0)")
	multiRev := fs.String("multi-swe-revision", "", "upstream revision the Multi-SWE-bench snapshot was fetched at")
	explorePath := fs.String("swe-explore", "", "pinned SWE-Explore-Bench Go JSONL snapshot (MIT)")
	exploreRev := fs.String("swe-explore-revision", "", "upstream revision the SWE-Explore snapshot was fetched at")
	checkouts := fs.String("checkouts", "", "directory of per-instance base_commit checkouts (<dir>/<instance_id>/)")
	callersDir := fs.String("callers-dir", "", "directory of cmd/loctruth outputs (<dir>/<instance_id>.json)")
	outPath := fs.String("out", "", "corpus path to write (testdata/contextbench/tasks-public-go.json)")
	n := fs.Int("n", publicloc.RegisteredRule.N, "first N eligible instances by instance_id")
	unregistered := fs.Bool("unregistered-n", false, "allow -n outside the pre-registered 30–40 window (fixtures, explorations)")
	selectOnly := fs.Bool("select-only", false, "apply the selection rule and write the selected instances (id, repo_url, base_commit, checkout dir) to -out; no checkouts needed")
	classes := fs.String("classes", "", "comma-separated task classes to keep (default all: file_localization,function_localization,find_callers); recorded in selection.classes")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *multiPath == "" && *explorePath == "" {
		fmt.Fprintln(os.Stderr, "import: at least one of -multi-swe or -swe-explore is required")
		return 2
	}
	if *outPath == "" || (*checkouts == "" && !*selectOnly) {
		fmt.Fprintln(os.Stderr, "import: -out is required, and -checkouts unless -select-only")
		return 2
	}
	if (*n < publicloc.RegisteredNMin || *n > publicloc.RegisteredNMax) && !*unregistered {
		fmt.Fprintf(os.Stderr, "import: -n %d is outside the pre-registered %d–%d window; pass -unregistered-n to derive an unregistered corpus\n", *n, publicloc.RegisteredNMin, publicloc.RegisteredNMax)
		return 2
	}

	var instances []publicloc.Instance
	var pins []publicloc.Pin
	read := func(source, path, rev string, fn func([]byte) ([]publicloc.Instance, error)) error {
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		insts, err := fn(raw)
		if err != nil {
			return err
		}
		instances = append(instances, insts...)
		pins = append(pins, publicloc.Pin{Source: source, Revision: rev, File: filepath.Base(path), SHA256: publicloc.SHA256Hex(raw), Rows: len(insts)})
		return nil
	}
	if *multiPath != "" {
		if err := read(publicloc.SourceMultiSWEBench, *multiPath, *multiRev, func(b []byte) ([]publicloc.Instance, error) {
			return publicloc.ReadMultiSWEBench(bytes.NewReader(b))
		}); err != nil {
			fmt.Fprintf(os.Stderr, "import: %v\n", err)
			return 1
		}
	}
	if *explorePath != "" {
		if err := read(publicloc.SourceSWEExplore, *explorePath, *exploreRev, func(b []byte) ([]publicloc.Instance, error) {
			return publicloc.ReadSWEExplore(bytes.NewReader(b))
		}); err != nil {
			fmt.Fprintf(os.Stderr, "import: %v\n", err)
			return 1
		}
	}

	safeID := map[string]string{}
	for _, in := range instances {
		safeID[in.InstanceID] = in.SafeID()
	}
	env := publicloc.Env{
		ReadFile: func(id, rel string) ([]byte, error) {
			dir := filepath.Join(*checkouts, safeID[id])
			if st, err := os.Stat(dir); err != nil || !st.IsDir() {
				return nil, fmt.Errorf("no checkout at %s (pin the subject at base_commit first — docs/bench/public-loc.md)", dir)
			}
			return os.ReadFile(filepath.Join(dir, filepath.FromSlash(rel)))
		},
	}
	if *callersDir != "" {
		env.Callers = func(id string) (publicloc.CallersFile, error) {
			path := filepath.Join(*callersDir, safeID[id]+".json")
			raw, err := os.ReadFile(path)
			if err != nil {
				return publicloc.CallersFile{}, fmt.Errorf("no cmd/loctruth output at %s", path)
			}
			var cf publicloc.CallersFile
			if err := json.Unmarshal(raw, &cf); err != nil {
				return publicloc.CallersFile{}, fmt.Errorf("%s: %w", path, err)
			}
			return cf, nil
		}
	}

	rule := publicloc.RegisteredRule
	rule.N = *n
	if *selectOnly {
		selected, report := publicloc.Select(instances, rule)
		type row struct {
			InstanceID string `json:"instance_id"`
			Source     string `json:"source"`
			RepoURL    string `json:"repo_url"`
			BaseCommit string `json:"base_commit"`
			Checkout   string `json:"checkout"`
			Service    string `json:"service"`
		}
		rows := make([]row, 0, len(selected))
		for _, in := range selected {
			rows = append(rows, row{in.InstanceID, in.Source, in.RepoURL(), in.BaseCommit, in.SafeID(), in.Service()})
		}
		raw, err := json.MarshalIndent(struct {
			Selection publicloc.SelectionReport `json:"selection"`
			Instances []row                     `json:"instances"`
		}{report, rows}, "", "  ")
		if err != nil {
			fmt.Fprintf(os.Stderr, "import: %v\n", err)
			return 1
		}
		if err := os.WriteFile(*outPath, append(raw, '\n'), 0o644); err != nil {
			fmt.Fprintf(os.Stderr, "import: writing %s: %v\n", *outPath, err)
			return 1
		}
		fmt.Printf("import: selected %d of %d candidates -> %s (pin each checkout at base_commit, then run cmd/loctruth and import again)\n", report.Selected, report.Candidates, *outPath)
		return 0
	}
	var keep []string
	if *classes != "" {
		for _, c := range strings.Split(*classes, ",") {
			c = strings.TrimSpace(c)
			switch c {
			case "file_localization", "function_localization", "find_callers":
				keep = append(keep, c)
			default:
				fmt.Fprintf(os.Stderr, "import: unknown class %q in -classes\n", c)
				return 2
			}
		}
	}
	corpus, err := publicloc.BuildCorpus(instances, pins, rule, env, keep...)
	if err != nil {
		fmt.Fprintf(os.Stderr, "import: %v\n", err)
		return 1
	}
	raw, err := publicloc.MarshalCanonical(corpus)
	if err != nil {
		fmt.Fprintf(os.Stderr, "import: %v\n", err)
		return 1
	}
	if err := os.WriteFile(*outPath, raw, 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "import: writing %s: %v\n", *outPath, err)
		return 1
	}

	sel := corpus.Selection
	fmt.Printf("import: %d tasks from %d instances -> %s\n", len(corpus.Tasks), sel.Selected, *outPath)
	fmt.Printf("  selection: %d candidates; excluded no-gold=%d too-many=%d duplicate=%d beyond-n=%d; pre_registered=%v\n",
		sel.Candidates, sel.ExcludedNoGoldFiles, sel.ExcludedTooManyFiles, sel.ExcludedDuplicate, sel.ExcludedBeyondN, sel.PreRegistered)
	for _, p := range corpus.Sources {
		fmt.Printf("  source %s (%s) %s rows=%d selected=%d sha256=%s\n", p.Source, p.License, p.File, p.Rows, p.Selected, p.SHA256)
	}
	omitted := 0
	for _, in := range corpus.Instances {
		omitted += len(in.Omitted)
	}
	if omitted > 0 {
		fmt.Printf("  %d derivation omission(s) recorded under instances[].omitted\n", omitted)
	}
	fmt.Printf("  corpus_sha256=%s\n", corpus.CorpusSHA256)
	return 0
}
