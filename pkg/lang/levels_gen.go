//go:build ignore

// levels_gen regenerates levels.json from docs/bench/readings/lang. Run it
// from this directory with `go generate`; a drift test fails when the two
// differ.
package main

import (
	"fmt"
	"os"

	"github.com/geoffmilleraz/fathom/internal/langlevelsgen"
)

func main() {
	b, err := langlevelsgen.Generate("../../docs/bench/readings/lang")
	if err != nil {
		fmt.Fprintln(os.Stderr, "levels_gen:", err)
		os.Exit(1)
	}
	if err := os.WriteFile("levels.json", b, 0o644); err != nil {
		fmt.Fprintln(os.Stderr, "levels_gen:", err)
		os.Exit(1)
	}
}
