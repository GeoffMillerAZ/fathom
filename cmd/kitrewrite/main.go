// Command kitrewrite keeps fathom one self-contained Go module (ADR-070): it
// rewrites the kit mirrors under third_party/ between their upstream form
// (their own go.mod, imports of github.com/geoffmilleraz/<kit>) and the
// in-module form fathom builds (go.mod kept as go.mod.upstream, imports of
// github.com/geoffmilleraz/fathom/third_party/<kit>). See internal/kitrewrite.
//
//	go run ./cmd/kitrewrite [-reverse] [ROOT]   both mirrors and ROOT's own Go/templ files
//	go run ./cmd/kitrewrite [-reverse] -kit DIR one kit copy (task sync-kit, task vendor-check)
//	go run ./cmd/kitrewrite -check [ROOT]       exit 1 naming each way ROOT is not self-contained
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/geoffmilleraz/fathom/internal/kitrewrite"
)

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	fs := flag.NewFlagSet("kitrewrite", flag.ContinueOnError)
	reverse := fs.Bool("reverse", false, "restore the kits' upstream form")
	check := fs.Bool("check", false, "report, change nothing: exit 1 when ROOT is not one self-contained module")
	kit := fs.String("kit", "", "rewrite this one kit copy (its base name is the mirror's name) instead of the repository")
	module := fs.String("module", "", "with -kit: the consumer module path (default: ROOT's go.mod module)")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	root := "."
	if fs.NArg() > 1 {
		fmt.Fprintln(os.Stderr, "kitrewrite: at most one ROOT")
		return 2
	} else if fs.NArg() == 1 {
		root = fs.Arg(0)
	}
	dir := kitrewrite.Forward
	if *reverse {
		dir = kitrewrite.Reverse
	}

	if *check {
		vs, err := kitrewrite.Check(root)
		if err != nil {
			fmt.Fprintf(os.Stderr, "kitrewrite: %v\n", err)
			return 2
		}
		for _, v := range vs {
			fmt.Println(v)
		}
		if len(vs) > 0 {
			fmt.Fprintf(os.Stderr, "kitrewrite: %d violation(s): not one self-contained module (ADR-070); `go run ./cmd/kitrewrite` rewrites the mirrors\n", len(vs))
			return 1
		}
		fmt.Println("kitrewrite: self-contained: no replace, no exclude, no private require, no upstream kit import")
		return 0
	}

	var (
		rep    kitrewrite.Report
		err    error
		target = root
	)
	if *kit != "" {
		target = *kit
		m := *module
		if m == "" {
			if m, err = kitrewrite.ModulePath(root); err != nil {
				fmt.Fprintf(os.Stderr, "kitrewrite: -module unset and %v\n", err)
				return 2
			}
		}
		rep, err = kitrewrite.RewriteKit(filepath.Clean(*kit), m, dir)
	} else {
		rep, err = kitrewrite.RewriteRepo(root, dir)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "kitrewrite: %v\n", err)
		return 1
	}
	fmt.Printf("kitrewrite: %s %s: %d file(s), %d import(s) rewritten, %d module file(s) moved\n", dir, target, rep.Files, rep.Imports, rep.Moved)
	return 0
}
