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

// TestCompilerVersionRelease (round 3 A4): nvcc's first --version line names
// the driver, not its release; the version recorded is the release line.
func TestCompilerVersionRelease(t *testing.T) {
	bin := filepath.Join(t.TempDir(), "nvcc")
	script := "#!/bin/sh\necho 'nvcc: NVIDIA (R) Cuda compiler driver'\necho 'Copyright (c) 2005-2025 NVIDIA Corporation'\necho 'Built on x'\necho 'Cuda compilation tools, release 12.9, V12.9.86'\necho 'Build cuda_12.9.r12.9/compiler.0'\n"
	if err := os.WriteFile(bin, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	if got := compilerVersion(bin); got != "Cuda compilation tools, release 12.9, V12.9.86" {
		t.Errorf("compilerVersion(nvcc) = %q, want the release line", got)
	}
}
