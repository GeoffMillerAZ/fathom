package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/geoffmilleraz/fathom/internal/extractor/tsengine"
)

// provenanceDoc is PROVENANCE.md's path under the repository root.
var provenanceDoc = filepath.Join("internal", "extractor", "tsengine", "grammars", "PROVENANCE.md")

// cmdProvenance is `indexbench provenance > <PROVENANCE.md>`: the committed
// document with its generated block re-rendered from grammars.lock.json.
func cmdProvenance(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("provenance", flag.ContinueOnError)
	fs.SetOutput(stderr)
	root := fs.String("root", ".", "repository root")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	lock, err := tsengine.LoadGrammarLock()
	if err != nil {
		fmt.Fprintln(stderr, "provenance:", err)
		return 1
	}
	doc, err := os.ReadFile(filepath.Join(*root, provenanceDoc))
	if err != nil {
		fmt.Fprintln(stderr, "provenance:", err)
		return 1
	}
	out, err := tsengine.ReplaceProvenanceBlock(string(doc), tsengine.RenderProvenanceTables(lock))
	if err != nil {
		fmt.Fprintln(stderr, "provenance:", err)
		return 1
	}
	fmt.Fprint(stdout, out)
	return 0
}
