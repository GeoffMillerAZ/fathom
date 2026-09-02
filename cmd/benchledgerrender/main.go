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
	if len(rows) == 0 {
		// An empty ledger renders as a sentence, not an empty table: a bare
		// header would read as "measured, nothing to report".
		fmt.Println("_No runs recorded yet. `task bench-home` produces the first row (it spends real tokens)._")
		return
	}
	fmt.Print(contextbench.RenderLedgerMarkdown(rows))
}
