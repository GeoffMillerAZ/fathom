package main

import (
	"fmt"
	"os"
	"path/filepath"
)

// useLedgerFlag is PE-26's `--ledger <path>`: it names the one ledger file
// this run appends to and reads, ahead of both the per-file variable
// (envVar) and FATHOM_HOME, so a scratch or test run names its own file and
// the operator's ledger is never touched (O121). The run's ledger readers
// and writers all resolve through the variable, so setting it for this
// process is what makes the flag reach every one of them, hook children
// included. An empty path is no flag.
func useLedgerFlag(envVar, path string) error {
	if path == "" {
		return nil
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("--ledger %q: %w", path, err)
	}
	return os.Setenv(envVar, abs)
}
