// Command benchledgerrender prints the committed context-bench ledger as a
// Markdown table. It is the read-only half of the bench: rendering the
// history costs nothing, while producing a new row costs real LLM tokens
// (scripts/context-bench.sh). Kept separate from contextbenchscore so that
// `task bench-ledger` cannot be confused for a command that runs anything.
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/geoffmilleraz/fathom/internal/contextbench"
)

func main() {
	ledger := flag.String("ledger", "docs/bench/ledger.jsonl", "path to the committed JSONL bench ledger")
	flag.Parse()

	rows, err := contextbench.LoadLedger(*ledger)
	if err != nil {
		fmt.Fprintf(os.Stderr, "benchledgerrender: %v\n", err)
		os.Exit(1)
	}
	// RC-35: the "Where fathom stands" verdict block renders ABOVE the
	// table — RenderVerdict already covers the empty-ledger sentence, so
	// an empty ledger needs no special case here.
	fmt.Print(contextbench.RenderVerdict(rows))
	fmt.Println()
	if len(rows) == 0 {
		return
	}
	fmt.Print(contextbench.RenderLedgerMarkdown(rows))
}
