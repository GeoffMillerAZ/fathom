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
	lock.Toolchain.ZigSHA256 = auditZigSHA256
	write()
	if _, err := loadAuditCandidate(config{CandidateLock: lockFile, CandidateModule: module}); err == nil || err.Error() != "audit module identity mismatch" {
		t.Fatalf("valid lock must reach module identity check: %v", err)
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
	outside := filepath.Join(t.TempDir(), "outside.json")
	if err := os.WriteFile(outside, []byte("valid"), 0600); err != nil {
		t.Fatal(err)
	}
	if info, err := os.Stat(outside); err != nil || !info.Mode().IsRegular() {
		t.Fatalf("outside fixture must be a real regular file: %v", err)
	}
	if _, err := auditRead(outside, 5); err == nil || err.Error() != "audit input outside ignored workspace" {
		t.Fatalf("existing regular file outside workspace must be refused by containment: %v", err)
	}
}

func TestKnownAuditMetadataAcceptsOnlyPinnedCandidates(t *testing.T) {
	base := auditLock{Schema: "fathom-cpp-external-audit-v1", BuilderSHA256: strings.Repeat("a", 64), BudgetSHA256: strings.Repeat("b", 64)}
	base.Module.Blob, base.Module.EntryPoint, base.Module.ABI = "cpp", "tree_sitter_cpp", 15
	base.Runtime = auditSource{Crate: "tree-sitter/tree-sitter", Version: "0.26.11", ArchiveSHA256: "fc2a4d846c36f7d19714085bbad76a68c5ff9d0fe7b1e2a6a62a22446efe966e", Licence: "MIT", LicenceSHA256: "c5cfb43042b6b72045f4ba997834d0a7786d2793d91680868b5815b39f14fc78", ArchiveBytes: 920896, Entries: 673, Files: 600, ExtractedBytes: 3922532}
	base.Toolchain.ZigVersion, base.Toolchain.ZigSHA256 = "0.16.0", auditZigSHA256
	base.Toolchain.WasmOptVersion, base.Toolchain.WasmOptSHA256 = "129", auditWasmOptSHA256
	for _, c := range []struct {
		name, pin, archive, licence, module string
		archiveBytes, moduleBytes           int
	}{
		{"tree-sitter/tree-sitter-cpp", "c009222808634c1014f82438d4883753516a2c24", "6c6efaf024f5c02fee0403e4e2373346eeadc40828587bcfad9351df2f0a7986", "2e0110e07abef7c2548b26ec9d6969775617ca539a0dc8dbeeb14d6452c711d1", "fde7035227a488422e95cc1a408fecbbc5b3440dceebf109e42d483d31270ed2", 1740811, 2790800},
		{"taku25/tree-sitter-unreal-cpp", "920e9810cc64a47fa05cfcf30c7caecb85b5f2c3", "285ad7c2969a03254ed7a77068e0effca26cb8f014bc5b9d300bd4415ec0b17b", "ef02f1077281d63262946e50ec461ba72efda41e51084c887379b59397b96a6d", "613af35868891df920a1e5c12fb67b2c2e83eb6a7c9cab6d3a1247acd20d22df", 2628614, 3975018},
	} {
		lock := base
		lock.Module.SHA256, lock.Module.Bytes = c.module, c.moduleBytes
		lock.Grammar = auditSource{Crate: c.name, Version: c.pin, ArchiveSHA256: c.archive, Licence: "MIT", LicenceSHA256: c.licence, ArchiveBytes: c.archiveBytes, Entries: 110, Files: 100, ExtractedBytes: 40000000}
		if !knownAuditMetadata(lock) {
			t.Fatalf("pinned candidate refused: %s", c.name)
		}
		lock.Grammar.ArchiveSHA256 = strings.Repeat("0", 64)
		if knownAuditMetadata(lock) {
			t.Fatalf("changed source accepted: %s", c.name)
		}
	}
}
