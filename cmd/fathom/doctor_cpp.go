package main

import (
	"fmt"
	"os"
	"path/filepath"
)

// checkCppToolchain verifies clang-tooling readiness on a CMake repo
// (docs/plans/cpp-toolchain.md CT-7): compile_commands.json — the
// compilation database every clang/LLVM tool keys off — exists at the repo
// root or under build/. Absence is doctorWarn with the exact export command,
// never doctorFail: an un-exported database is a quality signal, not broken
// wiring. A repo with no CMakeLists.txt skips honestly.
func checkCppToolchain(root string) doctorCheck {
	const name = "cpp_toolchain"
	if !fileExists(filepath.Join(root, "CMakeLists.txt")) {
		return doctorCheck{Name: name, Status: doctorSkipped, Reason: "not a CMake repo (no CMakeLists.txt at root) — cpp toolchain check does not apply"}
	}
	for _, rel := range []string{"compile_commands.json", filepath.Join("build", "compile_commands.json")} {
		p := filepath.Join(root, rel)
		if fileExists(p) {
			return doctorCheck{Name: name, Status: doctorPass, Reason: fmt.Sprintf("compile_commands.json present (%s) — clang/LLVM tooling ready", rel)}
		}
	}
	// A symlinked database (CMake convention: ln -s build/compile_commands.json .)
	// is covered by fileExists via Stat; a BROKEN symlink at root is worth
	// naming distinctly because the fix differs.
	if _, err := os.Lstat(filepath.Join(root, "compile_commands.json")); err == nil {
		return doctorCheck{
			Name:   name,
			Status: doctorWarn,
			Reason: "compile_commands.json at root is a broken symlink — its build directory is gone or was moved",
			Remedy: "cmake -S . -B build -G Ninja -DCMAKE_EXPORT_COMPILE_COMMANDS=ON",
		}
	}
	return doctorCheck{
		Name:   name,
		Status: doctorWarn,
		Reason: "CMake repo without compile_commands.json (root or build/) — clang-based tools (clangd, clang-tidy, future precise extraction) have no compilation database",
		Remedy: "cmake -S . -B build -G Ninja -DCMAKE_EXPORT_COMPILE_COMMANDS=ON",
	}
}
