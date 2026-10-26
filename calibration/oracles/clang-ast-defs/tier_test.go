package main

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestTierRequired (round 3 A3, F3): -tier has no default: a run that names
// none is a usage error (exit 2), never an output read as tier A.
func TestTierRequired(t *testing.T) {
	if os.Getenv("ORACLE_MAIN") == "1" {
		os.Args = append([]string{"oracle"}, strings.Fields(os.Getenv("ORACLE_ARGS"))...)
		main()
		return
	}
	dir := t.TempDir()
	db := filepath.Join(dir, "compile_commands.json")
	if err := os.WriteFile(db, []byte("[]"), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(os.Args[0], "-test.run=^TestTierRequired$")
	cmd.Env = append(os.Environ(), "ORACLE_MAIN=1", "ORACLE_ARGS=-subject "+dir+" -compdb "+db)
	err := cmd.Run()
	var exit *exec.ExitError
	if !errors.As(err, &exit) || exit.ExitCode() != 2 {
		t.Fatalf("no -tier: %v, want exit 2 (usage)", err)
	}
}
