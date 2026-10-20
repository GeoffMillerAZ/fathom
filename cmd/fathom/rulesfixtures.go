package main

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/geoffmilleraz/fathom/internal/readout"
)

// cmdRulesFixtures prints the fixture reading of a tech pack directory, the
// harness a kit runs in its own CI. Exit 0: every present section passes.
// 1: a section is withheld. 2: the pack is unreadable or invalid.
func cmdRulesFixtures(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("rules fixtures", stderr)
	jsonOut := fs.Bool("json", false, "emit the reading as JSON")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	rest := fs.Args()
	if len(rest) > 0 {
		if err := fs.Parse(rest[1:]); err != nil {
			return 2
		}
	}
	if len(rest) == 0 || len(fs.Args()) != 0 {
		fmt.Fprintln(stderr, "error: usage: fathom rules fixtures <pack dir> [--json]")
		return 2
	}
	r, err := readout.RunPackFixtures(rest[0])
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 2
	}
	if *jsonOut {
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(r); err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			return 2
		}
	} else {
		for _, l := range r.Lines() {
			fmt.Fprintln(stdout, l)
		}
	}
	if !r.OK() {
		return 1
	}
	return 0
}
