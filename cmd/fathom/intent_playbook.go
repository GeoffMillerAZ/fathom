package main

// intent_playbook.go — IV-1 (ADR-074 decision 1): the root INTENT.md and
// `fathom intent render --playbook` are retired. Nothing read the file; the
// command stays one minor release as a deprecation notice that writes
// nothing, then IV-10 removes this file.

import (
	"fmt"
	"io"
)

// cmdIntentRender implements the deprecated `fathom intent render`: it names
// ADR-074 on stderr, writes nothing and exits 0 whatever the flags.
func cmdIntentRender(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("intent render", stderr)
	fs.String("root", ".", "ignored (retired verb)")
	fs.Bool("playbook", false, "retired: prints a deprecation notice and writes nothing")
	fs.Bool("dry-run", false, "ignored (retired verb)")
	fs.Bool("check", false, "ignored (retired verb)")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	fmt.Fprintln(stderr, "deprecated: `fathom intent render --playbook` is retired (ADR-074): INTENT.md is no longer generated, checked or read; delete any INTENT.md and read docs/intent/README.md. The command is removed in the next minor release.")
	return 0
}
