// Command kit-catalog generates docs/catalog.md — a one-line-per-package
// index of every Go package in the module, grouped by area (foundations by
// category, then kits, runtimes, surfaces, toolchains, cmd, examples,
// scenarios), from package doc comments.
//
// Deterministic: output depends only on the source tree (sorted sections,
// sorted packages, no timestamps), so regenerating without source changes
// is a no-op. Skips tmp/, workspace/, testdata/, cue.mod/, and any dot- or
// underscore-prefixed directory (which covers _reference/ scanned sources).
//
// Usage:
//
//	go run ./cmd/kit-catalog [--root .] [--out docs/catalog.md]
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/geoffmilleraz/agentic-kit/cmd/internal/catalog"
)

func main() {
	root := flag.String("root", ".", "module root to scan")
	out := flag.String("out", "docs/catalog.md", "output markdown file")
	flag.Parse()

	entries, err := catalog.Collect(*root)
	if err != nil {
		fmt.Fprintln(os.Stderr, "kit-catalog:", err)
		os.Exit(1)
	}
	if dir := filepath.Dir(*out); dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			fmt.Fprintln(os.Stderr, "kit-catalog:", err)
			os.Exit(1)
		}
	}
	if err := os.WriteFile(*out, []byte(catalog.Render(entries)), 0o644); err != nil {
		fmt.Fprintln(os.Stderr, "kit-catalog:", err)
		os.Exit(1)
	}
	fmt.Printf("kit-catalog: wrote %s (%d packages)\n", *out, len(entries))
}
