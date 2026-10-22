package main

import (
	"testing"

	"github.com/geoffmilleraz/fathom/internal/readout"
	"github.com/geoffmilleraz/fathom/internal/shellcmd"
)

func TestCompileReadoutMixedCommandEndsHeaderBlock(t *testing.T) {
	for _, tc := range []struct {
		name, output string
	}{
		{"blank line", "# example.com/m/cart\ncart.go:10:20: undefined: Add\n\ncart.go:20:3: more linter output\n"},
		{"whitespace line", "# example.com/m/cart\ncart.go:10:20: undefined: Add\n \t\ncart.go:20:3: more linter output\n"},
		{"linter suffix", "# example.com/m/cart\ncart.go:10:20: undefined: Add\ncart.go:20:3: invalid regexp (SA1000)\ncart.go:21:3: more linter output\n"},
		{"named linter suffix", "# example.com/m/cart\ncart.go:10:20: undefined: Add\ncart.go:20:3: unchecked error (errcheck)\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := shellcmd.ParseCommand("go build ./...; staticcheck ./...")
			filtered := compileReadoutText(&c, tc.output)
			recs := readout.CompileErrors(filtered, nil)
			if len(recs) != 1 || recs[0].Line != 2 {
				t.Fatalf("compile records = %+v, want only the compiler error at line 10; filtered=%q", recs, filtered)
			}
		})
	}
}
