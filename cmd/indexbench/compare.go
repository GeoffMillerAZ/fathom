package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime/debug"
	"sort"
	"strings"
	"time"

	"github.com/geoffmilleraz/fathom/internal/contextbench/publicloc"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/indexbench"
	"github.com/geoffmilleraz/fathom/pkg/client"
)

// defaultLedgerPath is docs/bench/competitors-ledger.jsonl, relative to the
// caller's working directory — the same convention -out uses in run().
const defaultLedgerPath = "docs/bench/competitors-ledger.jsonl"

// cmdCompare implements `indexbench compare` (SB-31, docs/plans/
// strengths-bench.md wave 4): fathom's own L0 row, from the same
// IndexOracleReport path the self run uses, beside a competitor's — a
// graphify graph.json's call edges and/or a competitor MCP server's
// reference answers, run exactly as docs/bench/competitors.md documents.
// Every scored row is appended to the ledger at -ledger.
func cmdCompare(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("indexbench compare", flag.ContinueOnError)
	fs.SetOutput(stderr)
	repo := fs.String("repo", "", "module root to score (must contain go.mod) — required")
	service := fs.String("service", "compare-self", "service id to ingest the tree under")
	storePath := fs.String("store", "", "probe store path (default: in-memory; never the shared fleet store)")
	maxQ := fs.Int("max-queries", 200, "bound fathom's own oracle to this many deterministic stride-sampled questions")
	graphifyPath := fs.String("graphify", "", "graphify graph.json to score against the L0 truth (SB-29)")
	mcpConfigPath := fs.String("mcp", "", "MCPServerConfig JSON file naming a stdio MCP server to probe (SB-30)")
	tool := fs.String("tool", "", "MCP tool name to call per symbol — required with -mcp")
	arm := fs.String("arm", "", "ledger arm label for the -mcp probe, e.g. \"serena\" or \"serena@v1.7.0\" — required with -mcp")
	mcpSymbols := fs.Int("mcp-symbols", 20, "bound how many truth symbols are probed over MCP — one subprocess round trip each")
	version := fs.String("version", "", "version label recorded on the ledger row for whichever competitor ran (overrides an \"arm@version\" suffix on -arm)")
	ledgerPath := fs.String("ledger", defaultLedgerPath, "ledger file to append rows to")
	now := fs.String("now", "", "date recorded on ledger rows, YYYY-MM-DD (default: today, UTC)")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *repo == "" {
		fmt.Fprintln(stderr, "indexbench compare: -repo is required")
		return 2
	}
	mcpGiven := *mcpConfigPath != "" || *tool != "" || *arm != ""
	if mcpGiven && (*mcpConfigPath == "" || *tool == "" || *arm == "") {
		fmt.Fprintln(stderr, "indexbench compare: -mcp, -tool and -arm must be given together")
		return 2
	}

	abs, err := filepath.Abs(*repo)
	if err != nil {
		fmt.Fprintf(stderr, "indexbench compare: %v\n", err)
		return 2
	}
	date := *now
	if date == "" {
		date = time.Now().UTC().Format("2006-01-02")
	}
	commit := gitCommitAt(abs)

	ctx := context.Background()
	c, err := client.NewConstantClient(client.ModeLocal, *storePath)
	if err != nil {
		fmt.Fprintf(stderr, "indexbench compare: open probe store: %v\n", err)
		return 1
	}
	defer func() { _ = c.Close() }()

	if _, err := c.Ingest(ctx, core.ServiceRef{ID: *service, Path: abs}); err != nil {
		fmt.Fprintf(stderr, "indexbench compare: ingest %s: %v\n", abs, err)
		return 1
	}

	rep, err := indexbench.IndexOracleReport(ctx, c, []indexbench.Subject{
		{Service: *service, Root: abs, MaxQueries: *maxQ},
	})
	if err != nil {
		fmt.Fprintf(stderr, "indexbench compare: %v\n", err)
		return 1
	}
	if len(rep.Services) != 1 {
		fmt.Fprintf(stderr, "indexbench compare: expected one scored service, got %d\n", len(rep.Services))
		return 1
	}
	var fathomCalls, fathomRefs indexbench.ToolScore
	for _, t := range rep.Services[0].Tools {
		switch t.Tool {
		case indexbench.ToolFindCallers:
			fathomCalls = t
		case indexbench.ToolReferences:
			fathomRefs = t
		}
	}

	truth, err := publicloc.LoadGoTruth(abs)
	if err != nil {
		fmt.Fprintf(stderr, "indexbench compare: go/types truth: %v\n", err)
		return 1
	}

	fmt.Fprintf(stdout, "%s @ %s (%d go packages)\n", *service, abs, truth.Packages)

	var ledgerRows []indexbench.CompetitorLedgerRow
	ranAny := false

	if *graphifyPath != "" {
		edges, err := indexbench.LoadGraphifyEdges(*graphifyPath)
		if err != nil {
			fmt.Fprintf(stderr, "indexbench compare: %v\n", err)
			return 1
		}
		cmp := indexbench.CompareEdges(edges.Edges, truth)
		printCompareRow(stdout, "fathom (find_callers)", fathomCalls.Precision, fathomCalls.Recall, fathomCalls.Queries, nil)
		printCompareRow(stdout, "graphify", cmp.Precision, cmp.Recall, cmp.Queries, edges.DroppedKinds)
		ledgerRows = append(
			ledgerRows,
			indexbench.CompetitorLedgerRow{
				Arm: "fathom", Oracle: "find_callers", Version: fathomVersion(), Repo: abs, Commit: commit,
				Precision: fathomCalls.Precision, Recall: fathomCalls.Recall, Adjudicable: fathomCalls.Queries, Date: date,
			},
			indexbench.CompetitorLedgerRow{
				Arm: "graphify", Oracle: "find_callers", Version: *version, Repo: abs, Commit: commit,
				Precision: cmp.Precision, Recall: cmp.Recall, Adjudicable: cmp.Queries, DroppedKinds: edges.DroppedKinds, Date: date,
			},
		)
		ranAny = true
	}

	if mcpGiven {
		cfgData, err := os.ReadFile(*mcpConfigPath)
		if err != nil {
			fmt.Fprintf(stderr, "indexbench compare: read -mcp config: %v\n", err)
			return 1
		}
		var cfg indexbench.MCPServerConfig
		if err := json.Unmarshal(cfgData, &cfg); err != nil {
			fmt.Fprintf(stderr, "indexbench compare: parse -mcp config %s: %v\n", *mcpConfigPath, err)
			return 1
		}
		names := sampleDeclaredNames(truth, *mcpSymbols)
		symbols := indexbench.TruthSymbolsForMCP(truth, names)
		edges, err := indexbench.ProbeReferencesViaMCP(ctx, cfg, *tool, symbols)
		if err != nil {
			fmt.Fprintf(stderr, "indexbench compare: %v\n", err)
			return 1
		}
		cmp := indexbench.CompareEdges(edges.Edges, truth)
		armName, armVersion := splitArmVersion(*arm)
		if *version != "" {
			armVersion = *version
		}
		printCompareRow(stdout, "fathom (references)", fathomRefs.Precision, fathomRefs.Recall, fathomRefs.Queries, nil)
		printCompareRow(stdout, armName, cmp.Precision, cmp.Recall, cmp.Queries, edges.DroppedKinds)
		ledgerRows = append(
			ledgerRows,
			indexbench.CompetitorLedgerRow{
				Arm: "fathom", Oracle: "references", Version: fathomVersion(), Repo: abs, Commit: commit,
				Precision: fathomRefs.Precision, Recall: fathomRefs.Recall, Adjudicable: fathomRefs.Queries, Date: date,
			},
			indexbench.CompetitorLedgerRow{
				Arm: armName, Oracle: "references", Version: armVersion, Repo: abs, Commit: commit,
				Precision: cmp.Precision, Recall: cmp.Recall, Adjudicable: cmp.Queries, DroppedKinds: edges.DroppedKinds, Date: date,
			},
		)
		ranAny = true
	}

	if !ranAny {
		printCompareRow(stdout, "fathom (find_callers)", fathomCalls.Precision, fathomCalls.Recall, fathomCalls.Queries, nil)
		printCompareRow(stdout, "fathom (references)", fathomRefs.Precision, fathomRefs.Recall, fathomRefs.Queries, nil)
		ledgerRows = append(
			ledgerRows,
			indexbench.CompetitorLedgerRow{
				Arm: "fathom", Oracle: "find_callers", Version: fathomVersion(), Repo: abs, Commit: commit,
				Precision: fathomCalls.Precision, Recall: fathomCalls.Recall, Adjudicable: fathomCalls.Queries, Date: date,
			},
			indexbench.CompetitorLedgerRow{
				Arm: "fathom", Oracle: "references", Version: fathomVersion(), Repo: abs, Commit: commit,
				Precision: fathomRefs.Precision, Recall: fathomRefs.Recall, Adjudicable: fathomRefs.Queries, Date: date,
			},
		)
	}

	if err := indexbench.AppendCompetitorLedger(*ledgerPath, ledgerRows); err != nil {
		fmt.Fprintf(stderr, "indexbench compare: %v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "\nledger: %s (%d row(s) appended)\n", *ledgerPath, len(ledgerRows))
	return 0
}

func printCompareRow(w io.Writer, label string, precision, recall float64, adjudicable int, dropped map[string]int) {
	fmt.Fprintf(w, "  %-24s precision %.4f  recall %.4f  adjudicable %d\n", label, precision, recall, adjudicable)
	if len(dropped) > 0 {
		fmt.Fprintf(w, "  %-24s dropped: %v\n", "", dropped)
	}
}

// sampleDeclaredNames mirrors oracle.go's deterministic stride sample
// (unexported there) over every qualified name truth declares, so an MCP
// probe's cost — one subprocess round trip per name — is bounded the same
// way fathom's own oracle bounds itself.
func sampleDeclaredNames(truth publicloc.GoTruth, max int) []string {
	set := map[string]bool{}
	for _, names := range truth.DeclsByFile {
		for _, n := range names {
			set[n] = true
		}
	}
	all := make([]string, 0, len(set))
	for n := range set {
		all = append(all, n)
	}
	sort.Strings(all)
	if max <= 0 || len(all) <= max {
		return all
	}
	out := make([]string, 0, max)
	for i := 0; i < max; i++ {
		out = append(out, all[i*len(all)/max])
	}
	return out
}

// splitArmVersion splits "name@version" into its two halves; an arm with no
// "@" returns an empty version.
func splitArmVersion(arm string) (name, version string) {
	if i := strings.LastIndex(arm, "@"); i > 0 {
		return arm[:i], arm[i+1:]
	}
	return arm, ""
}

// fathomVersion is the version label recorded on fathom's own ledger rows:
// the module version embedded at build time, or "dev" for a `go run`/`go
// test` build that carries none.
func fathomVersion() string {
	if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "" && info.Main.Version != "(devel)" {
		return info.Main.Version
	}
	return "dev"
}

// gitCommitAt returns dir's HEAD commit, or "" when dir is not a git
// checkout (or git is unavailable) — a best-effort ledger field, never a
// hard failure.
func gitCommitAt(dir string) string {
	out, err := exec.Command("git", "-C", dir, "rev-parse", "HEAD").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}
