package connector

import (
	"embed"
	"io/fs"
)

// fauxtownFS is the fauxtown fixture fleet compiled into the binary, for
// `fathom init --demo` (RH-12, docs/design/reach.md §5.1): a stranger with
// a prebuilt binary has no checkout to copy it from, and the demo never
// downloads. Two parts of the fixture are left out by embed's own rules:
// dot-files (the planted .claude/worktrees tree, a discovery-exclusion
// case) and polyglot-service, whose go.mod makes it a module of its own.
// The demo copy is the other nine services.
//
//go:embed testdata/fauxtown
var fauxtownFS embed.FS

// Fauxtown returns the fauxtown fixture fleet, rooted at the directory
// that holds its services.
func Fauxtown() (fs.FS, error) {
	return fs.Sub(fauxtownFS, "testdata/fauxtown")
}
