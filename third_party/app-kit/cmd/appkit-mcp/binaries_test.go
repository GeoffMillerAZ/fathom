package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// execMagics are the leading bytes of the executable formats a Go build can
// emit. A committed file starting with one of these is a stale build output,
// never source — legitimately-tracked binaries in this repo (PNGs, icons) do
// not start with any of them.
var execMagics = [][]byte{
	{0x7f, 'E', 'L', 'F'},    // ELF (Linux)
	{0xcf, 0xfa, 0xed, 0xfe}, // Mach-O 64-bit little-endian (macOS)
	{0xce, 0xfa, 0xed, 0xfe}, // Mach-O 32-bit little-endian
	{0xca, 0xfe, 0xba, 0xbe}, // Mach-O universal ("fat"), big-endian
	{0xbe, 0xba, 0xfe, 0xca}, // Mach-O universal, little-endian
	{'M', 'Z'},               // PE/DOS (Windows .exe)
}

// TestNoCommittedBinaries walks every git-tracked file in the repo and fails on
// any that is a compiled executable. The appkit-mcp binary building to the repo
// root and getting committed is the concrete failure this guards: a stale
// committed binary is what makes the live MCP server serve pre-contribution
// guidance until someone notices (see docs/plans/motion-provenance.md F-4/F-9).
// It reads git-tracked paths, not the working tree, so a gitignored build
// output sitting on disk (the normal case) is correctly ignored.
func TestNoCommittedBinaries(t *testing.T) {
	const kitRootRel = "../.."

	out, err := exec.Command("git", "-C", kitRootRel, "ls-files", "-z").Output()
	if err != nil {
		t.Skipf("git ls-files unavailable (%v); cannot enumerate tracked files", err)
	}

	buf := make([]byte, 4)
	for _, rel := range bytes.Split(out, []byte{0}) {
		if len(rel) == 0 {
			continue
		}
		path := filepath.Join(kitRootRel, string(rel))
		f, err := os.Open(path)
		if err != nil {
			continue // tracked but absent (e.g. a submodule path) — not our concern
		}
		n, _ := f.Read(buf)
		_ = f.Close()
		head := buf[:n]
		for _, magic := range execMagics {
			if bytes.HasPrefix(head, magic) {
				t.Errorf("tracked compiled binary %q — build outputs must be gitignored, not committed", string(rel))
				break
			}
		}
	}
}
