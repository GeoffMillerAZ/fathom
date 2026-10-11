// Command linguistgen regenerates internal/repofacts/linguist.json, the
// linguist basis' pinned data (RF-3), from a github-linguist/linguist
// checkout at a 40-hex revision:
//
//	go run ./internal/repofacts/linguistgen -linguist <checkout> -rev <40-hex> [-out internal/repofacts/linguist.json]
//
// It reads lib/linguist/vendor.yml, lib/linguist/documentation.yml,
// lib/linguist/generated.rb and LICENSE, and writes the data the way
// LB-F48's `indexbench lang-linguist` writes the language data. It never
// fetches: the checkout is the caller's.
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/geoffmilleraz/fathom/internal/repofacts"
)

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	fs := flag.NewFlagSet("linguistgen", flag.ContinueOnError)
	dir := fs.String("linguist", "", "github-linguist/linguist checkout")
	rev := fs.String("rev", "", "the checkout's commit (40 hex)")
	out := fs.String("out", filepath.Join("internal", "repofacts", "linguist.json"), "data file to write")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *dir == "" || *rev == "" {
		fmt.Fprintln(os.Stderr, "linguistgen: -linguist and -rev are required")
		return 2
	}
	read := func(rel string) []byte {
		b, err := os.ReadFile(filepath.Join(*dir, rel))
		if err != nil {
			fmt.Fprintln(os.Stderr, "linguistgen:", err)
			os.Exit(2)
		}
		return b
	}
	d, err := repofacts.ImportLinguist(read("lib/linguist/vendor.yml"), read("lib/linguist/documentation.yml"),
		read("lib/linguist/generated.rb"), read("LICENSE"), *rev)
	if err != nil {
		fmt.Fprintln(os.Stderr, "linguistgen:", err)
		return 1
	}
	b, err := repofacts.MarshalLinguist(d)
	if err != nil {
		fmt.Fprintln(os.Stderr, "linguistgen:", err)
		return 1
	}
	if err := os.WriteFile(*out, b, 0o644); err != nil {
		fmt.Fprintln(os.Stderr, "linguistgen:", err)
		return 1
	}
	fmt.Printf("linguistgen: %s at %s: vendor %d, documentation %d, generated %d, dropped %d\n",
		repofacts.LinguistSource, d.Revision, len(d.Vendor), len(d.Documentation), len(d.Generated), len(d.Dropped))
	return 0
}
