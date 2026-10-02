// Command indexbench runs the bench pyramid's L0 index oracle
// (docs/design/bench-pyramid.md §3.1, plan bench-pyramid BP-14) over a real
// Go module at HEAD: it ingests the tree into a throwaway probe store, asks
// fathom the same questions the find_callers / references / blast_radius MCP
// tool handlers ask their backend, and scores the answers against an
// independent go/types truth.
//
// It spends no tokens and talks to no daemon. It is slower than the fixture
// oracle in `task validate` (a whole module's type-check plus a whole
// module's ingest), so it runs on demand: `task bench-l0-self`.
//
// Usage:
//
//	indexbench -root <module root> [-service self] [-store <path>] [-out <file>]
//
// The report is the same schema internal/indexbench commits as a golden, so
// a self run and a fixture run are directly comparable.
//
// A "compare" subcommand (SB-31, docs/plans/strengths-bench.md wave 4) puts
// fathom's own L0 row beside a competitor's, from a graphify graph.json
// and/or a competitor's MCP server, and appends
// docs/bench/competitors-ledger.jsonl:
//
//	indexbench compare -repo <dir> [-graphify <graph.json>] [-mcp <config.json> -tool <name> -arm <name>]
//
// The language-breadth instrument (ADR-048, docs/design/language-breadth.md
// §10, §12) adds lang-probe (a file's tree and its dialect's captures),
// lang-oracle (a dialect's reading against independent oracles),
// lang-parity (a port's dialect against the incumbent it retires, §11),
// lang-regrade (a committed reading recomputed under changed harness rules) and
// lang-tracker (docs/languages.md rendered from the queue and readings),
// lang-next (the open queue items, eligible or blocked, §13.0 step 2) and
// lang-disclosure (the connector's unsupported-language table, LB-F25).
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/indexbench"
	"github.com/geoffmilleraz/fathom/pkg/client"
)

func main() {
	os.Exit(mainRun(os.Args[1:], os.Stdout, os.Stderr))
}

// mainRun dispatches to the "compare" (SB-31) or "scip-oracle" (SU-12,
// ADR-041 decision 5) subcommand, or falls back to the original
// flags-only self-run behavior — neither is a valid -root value, so this
// is unambiguous and every existing invocation (`indexbench -root … `,
// the bench-l0-self task) is unaffected.
func mainRun(args []string, stdout, stderr io.Writer) int {
	if len(args) > 0 && args[0] == "compare" {
		return cmdCompare(args[1:], stdout, stderr)
	}
	if len(args) > 0 && args[0] == "scip-oracle" {
		return cmdScipOracle(args[1:], stdout, stderr)
	}
	if len(args) > 0 && args[0] == "lang-probe" {
		return cmdLangProbe(args[1:], stdout, stderr)
	}
	if len(args) > 0 && args[0] == "lang-draft" {
		return cmdLangDraft(args[1:], stdout, stderr)
	}
	if len(args) > 0 && args[0] == "lang-oracle" {
		return cmdLangOracle(args[1:], stdout, stderr)
	}
	if len(args) > 0 && args[0] == "lang-parity" {
		return cmdLangParity(args[1:], stdout, stderr)
	}
	if len(args) > 0 && args[0] == "lang-regrade" {
		return cmdLangRegrade(args[1:], stdout, stderr)
	}
	if len(args) > 0 && args[0] == "lang-tracker" {
		return cmdLangTracker(args[1:], stdout, stderr)
	}
	if len(args) > 0 && args[0] == "lang-linguist" {
		return cmdLangLinguist(args[1:], stdout, stderr)
	}
	if len(args) > 0 && args[0] == "lang-next" {
		return cmdLangNext(args[1:], stdout, stderr)
	}
	if len(args) > 0 && args[0] == "lang-pin" {
		return cmdLangPin(args[1:], stdout, stderr)
	}
	if len(args) > 0 && args[0] == "lang-disclosure" {
		return cmdLangDisclosure(args[1:], stdout, stderr)
	}
	if len(args) > 0 && args[0] == "lang-routes" {
		return cmdLangRoutes(args[1:], stdout, stderr)
	}
	if len(args) > 0 && args[0] == "lang-policies" {
		return cmdLangPolicies(args[1:], stdout, stderr)
	}
	return run(args, stdout, stderr)
}

func run(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("indexbench", flag.ContinueOnError)
	fs.SetOutput(stderr)
	root := fs.String("root", ".", "module root to score (must contain go.mod)")
	service := fs.String("service", "self", "service id to ingest the tree under")
	storePath := fs.String("store", "", "probe store path (default: in-memory; never the shared fleet store)")
	out := fs.String("out", "", "write the report JSON here (default: stdout)")
	maxBlast := fs.Int("max-blast-queries", 20, "bound blast_radius separately — it rebuilds the call graph on every call, so it costs ~10s a query on a ten-thousand-symbol service")
	misses := fs.Bool("misses", true, "record every truth caller the resolved edges did not predict, with its file and whether the syntax pass saw it — the list a recall number is made of")
	blastDepth := fs.Int("blast-depth", 1, "reverse-walk bound blast_radius is scored at; the truth closure uses the same bound (-1 = unbounded, the pre-BP-27 walk)")
	maxQ := fs.Int("max-queries", 200, "bound each tool to this many deterministic stride-sampled questions (0 = every one; a whole module scored exhaustively is minutes, not seconds)")
	referencesExact := fs.Bool("references-exact", false, "score references' SB-45 exact-mode answer (typed-confirmed occurrences only) instead of its pre-SB-45 one")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	abs, err := filepath.Abs(*root)
	if err != nil {
		fmt.Fprintf(stderr, "indexbench: %v\n", err)
		return 2
	}
	ctx := context.Background()

	started := time.Now()
	c, err := client.NewConstantClient(client.ModeLocal, *storePath)
	if err != nil {
		fmt.Fprintf(stderr, "indexbench: open probe store: %v\n", err)
		return 1
	}
	defer func() { _ = c.Close() }()

	report, err := c.Ingest(ctx, core.ServiceRef{ID: *service, Path: abs})
	if err != nil {
		fmt.Fprintf(stderr, "indexbench: ingest %s: %v\n", abs, err)
		return 1
	}
	ingested := time.Since(started)
	fmt.Fprintf(stderr, "indexbench: ingested %d files, %d symbols in %s\n",
		report.Discovered, report.Symbols, ingested.Round(time.Millisecond))

	// The LocalConnector reports paths relative to the service root, which is
	// the same root go/types walks — so no prefix translation is needed.
	rep, err := indexbench.IndexOracleReport(ctx, c, []indexbench.Subject{
		{Service: *service, Root: abs, MaxQueries: *maxQ, MaxBlastQueries: *maxBlast, BlastDepth: *blastDepth, RecordMisses: *misses, ReferencesExact: *referencesExact},
	})
	if err != nil {
		fmt.Fprintf(stderr, "indexbench: %v\n", err)
		return 1
	}
	rep.Corpus = abs

	data, err := marshal(rep)
	if err != nil {
		fmt.Fprintf(stderr, "indexbench: %v\n", err)
		return 1
	}
	if *out == "" {
		if _, err := stdout.Write(data); err != nil {
			fmt.Fprintf(stderr, "indexbench: %v\n", err)
			return 1
		}
	} else {
		if err := os.MkdirAll(filepath.Dir(*out), 0o755); err != nil {
			fmt.Fprintf(stderr, "indexbench: %v\n", err)
			return 1
		}
		if err := os.WriteFile(*out, data, 0o644); err != nil {
			fmt.Fprintf(stderr, "indexbench: %v\n", err)
			return 1
		}
		fmt.Fprintf(stderr, "indexbench: wrote %s\n", *out)
	}

	// Every rate is printed WITH its denominator: a bench that prints
	// "precision 0.98" and hides "of 51" has told the reader a feeling, not
	// a measurement (ADR-031).
	// BP-29: a per-tool VERDICT against the ADR-032 floor, not just a
	// number. ADR-032 holds L4 closed while L0 precision or recall is
	// under 0.95 on a pinned repo, and a reader should not have to do that
	// comparison in their head on every run.
	below := 0
	for _, svc := range rep.Services {
		fmt.Fprintf(stdout, "\n%s — %d go packages (%d with type errors), %d indexed symbols\n",
			svc.Service, svc.GoPackages, svc.PackagesWithErrors, svc.IndexedSymbols)
		for _, t := range svc.Tools {
			fmt.Fprintf(stdout, "  %-13s precision %.4f (%d/%d)  recall %.4f (%d/%d)  over %d of %d queries\n",
				t.Tool, t.Precision, t.Correct, t.Predicted, t.Recall, t.Correct, t.Truth, t.Queries, t.Population)
			detail := time.Duration(t.ElapsedMS) * time.Millisecond
			per := ""
			if t.Queries > 0 {
				per = fmt.Sprintf(", %s/query", (detail / time.Duration(t.Queries)).Round(time.Millisecond))
			}
			bound := ""
			if t.EdgeFilter != "" {
				bound = "edges=" + t.EdgeFilter
			}
			if t.Depth > 0 {
				bound += fmt.Sprintf(" depth=%d", t.Depth)
			}
			fmt.Fprintf(stdout, "  %-13s %s%s  %s\n", "", detail, per, bound)
			if t.Unadjudicated > 0 || t.UnadjudicatedQueries > 0 {
				fmt.Fprintf(stdout, "  %-13s unadjudicated: %d answers, %d queries — outside the go/types truth's competence, in no rate (BP-30)\n",
					"", t.Unadjudicated, t.UnadjudicatedQueries)
			}
			verdict := "PASS"
			if t.Precision < l0Floor || t.Recall < l0Floor {
				verdict = "BELOW FLOOR"
				below++
			}
			fmt.Fprintf(stdout, "  %-13s %s vs the ADR-032 %.2f floor\n", "", verdict, l0Floor)
		}
	}
	fmt.Fprintf(stdout, "\ntotal %s (ingest %s)\n",
		time.Since(started).Round(time.Millisecond), ingested.Round(time.Millisecond))
	if *out != "" {
		fmt.Fprintf(stdout, "report: %s\n", *out)
	}
	if below > 0 {
		fmt.Fprintf(stdout, "%d tool/metric pair(s) BELOW the %.2f floor — ADR-032 keeps L4 closed while that holds.\n", below, l0Floor)
	} else {
		fmt.Fprintf(stdout, "every scored tool is at or above the %.2f floor.\n", l0Floor)
	}
	return 0
}

// l0Floor is ADR-032's gate: L4 must not run while L0 precision or recall
// is under it on a pinned repo. It is printed beside every rate so the
// verdict and the number it came from are never separated.
const l0Floor = 0.95

func marshal(v any) ([]byte, error) {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
}
