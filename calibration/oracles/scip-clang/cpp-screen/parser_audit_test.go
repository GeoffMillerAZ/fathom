//go:build fathom_grammar_audit

package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAuditLockRefusesSelfConsistentModuleAndWrongTool(t *testing.T) {
	root, err := git(".", "rev-parse", "--show-toplevel")
	if err != nil {
		t.Fatal(err)
	}
	dir, err := os.MkdirTemp(filepath.Join(root, "workspace"), "cpp-lock-test-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	module := filepath.Join(dir, "cpp.wasm")
	raw := []byte("forged")
	if err := os.WriteFile(module, raw, 0600); err != nil {
		t.Fatal(err)
	}
	hash := func(b []byte) string { sum := sha256.Sum256(b); return hex.EncodeToString(sum[:]) }
	lock := auditLock{Schema: "fathom-cpp-external-audit-v1"}
	lock.Module.Blob, lock.Module.EntryPoint, lock.Module.ABI = "cpp", "tree_sitter_cpp", 0x57420006
	lock.Module.Bytes, lock.Module.SHA256 = len(raw), hash(raw)
	lock.Grammar = auditSource{Crate: "willbooster-tree-sitter-cpp", Version: "6.0.2", ArchiveSHA256: auditGrammarArchive, Licence: "MIT", LicenceSHA256: auditGrammarLicence, ArchiveBytes: 1365727, Entries: 22, Files: 20, ExtractedBytes: 28000000}
	lock.Runtime = auditSource{Crate: "willbooster-tree-sitter", Version: "7.0.5", ArchiveSHA256: auditRuntimeArchive, Licence: "MIT", LicenceSHA256: auditRuntimeLicence, ArchiveBytes: 246632, Entries: 95, Files: 90, ExtractedBytes: 1000000}
	for _, source := range []struct {
		path string
		dest *string
	}{
		{filepath.Join(root, "calibration/oracles/scip-clang/build-willbooster.py"), &lock.BuilderSHA256},
		{filepath.Join(root, "internal/extractor/tsengine/grammars/budget.c"), &lock.BudgetSHA256},
	} {
		b, err := os.ReadFile(source.path)
		if err != nil {
			t.Fatal(err)
		}
		*source.dest = hash(b)
	}
	lock.Toolchain.ZigVersion, lock.Toolchain.ZigSHA256 = "0.16.0", auditZigSHA256
	lock.Toolchain.WasmOptVersion, lock.Toolchain.WasmOptSHA256 = "129", auditWasmOptSHA256
	lockFile := filepath.Join(dir, "grammars.lock.json")
	write := func() {
		b, err := json.Marshal(lock)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(lockFile, b, 0600); err != nil {
			t.Fatal(err)
		}
	}
	write()
	if _, err := loadAuditCandidate(config{CandidateLock: lockFile, CandidateModule: module}); err == nil || !strings.Contains(err.Error(), "metadata") {
		t.Fatalf("forged self-consistent module accepted or wrong refusal: %v", err)
	}
	lock.Module.Bytes, lock.Module.SHA256 = auditModuleBytes, auditModuleSHA256
	lock.Toolchain.ZigSHA256 = strings.Repeat("0", 64)
	write()
	if _, err := loadAuditCandidate(config{CandidateLock: lockFile, CandidateModule: module}); err == nil || !strings.Contains(err.Error(), "metadata") {
		t.Fatalf("wrong tool accepted or wrong refusal: %v", err)
	}
}

func TestAuditReadWorkspaceCapsAndSymlinks(t *testing.T) {
	root, err := git(".", "rev-parse", "--show-toplevel")
	if err != nil {
		t.Fatal(err)
	}
	dir, err := os.MkdirTemp(filepath.Join(root, "workspace"), "cpp-audit-test-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	file := filepath.Join(dir, "grammars.lock.json")
	if err := os.WriteFile(file, []byte("12345"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := auditRead(file, 5); err != nil {
		t.Fatalf("equality refused: %v", err)
	}
	if _, err := auditRead(file, 4); err == nil {
		t.Fatal("accepted cap + 1")
	}
	link := filepath.Join(dir, "linked.lock.json")
	if err := os.Symlink(file, link); err != nil {
		t.Fatal(err)
	}
	if _, err := auditRead(link, 5); err == nil {
		t.Fatal("accepted symlink")
	}
	if _, err := auditRead(filepath.Join(t.TempDir(), "outside.json"), 5); err == nil {
		t.Fatal("accepted outside workspace")
	}
}
