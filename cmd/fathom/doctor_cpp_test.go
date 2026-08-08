package main

import (
	"os"
	"path/filepath"
	"testing"
)

// TestCheckCppToolchain pins CT-7 (docs/plans/cpp-toolchain.md): on a CMake
// repo, doctor verifies clang-tooling readiness — compile_commands.json at
// the root or under build/ — and names the exact remedy when absent. On a
// non-CMake repo the check skips honestly. The missing case is doctorWarn,
// never doctorFail: an un-exported compilation database is a quality signal,
// not broken wiring.
func TestCheckCppToolchain(t *testing.T) {
	t.Run("non-CMake repo skips", func(t *testing.T) {
		root := t.TempDir()
		c := checkCppToolchain(root)
		if c.Status != doctorSkipped {
			t.Errorf("status = %q, want skipped (no CMakeLists.txt)", c.Status)
		}
	})

	t.Run("CMake repo without compile_commands warns with the remedy", func(t *testing.T) {
		root := t.TempDir()
		writeFile(t, filepath.Join(root, "CMakeLists.txt"), "project(x)\n")
		c := checkCppToolchain(root)
		if c.Status != doctorWarn {
			t.Errorf("status = %q, want warn", c.Status)
		}
		wantRemedy := "cmake -S . -B build -G Ninja -DCMAKE_EXPORT_COMPILE_COMMANDS=ON"
		if c.Remedy != wantRemedy {
			t.Errorf("remedy = %q, want %q", c.Remedy, wantRemedy)
		}
	})

	t.Run("compile_commands at root passes", func(t *testing.T) {
		root := t.TempDir()
		writeFile(t, filepath.Join(root, "CMakeLists.txt"), "project(x)\n")
		writeFile(t, filepath.Join(root, "compile_commands.json"), "[]\n")
		c := checkCppToolchain(root)
		if c.Status != doctorPass {
			t.Errorf("status = %q, want pass; reason=%q", c.Status, c.Reason)
		}
	})

	t.Run("compile_commands under build/ passes", func(t *testing.T) {
		root := t.TempDir()
		writeFile(t, filepath.Join(root, "CMakeLists.txt"), "project(x)\n")
		writeFile(t, filepath.Join(root, "build", "compile_commands.json"), "[]\n")
		c := checkCppToolchain(root)
		if c.Status != doctorPass {
			t.Errorf("status = %q, want pass; reason=%q", c.Status, c.Reason)
		}
	})
}

// writeFile creates path (and parents) with content, failing the test on error.
func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
