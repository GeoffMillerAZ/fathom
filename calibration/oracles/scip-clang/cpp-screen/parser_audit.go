//go:build fathom_grammar_audit

package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/extractor/tsengine"
)

const auditModuleCap = 32 << 20
const auditLockCap = 1 << 20

const (
	auditGrammarArchive = "8c892195d9e44d816b4cc5ffa9f8cf53d2d1c00b3030feb0d35e612ae5f34bcc"
	auditGrammarLicence = "2bb45b424e2a497412ab9eb903554eebb9847e4f70bc81eecd2609817301679f"
	auditRuntimeArchive = "166cf42202927500081763e65e63d3c0bc9273e05ba998496756fff7569de9c5"
	auditRuntimeLicence = "f1e1e62c6e1119d27f5ba38a0cbdae0788fc00f1d0bdf71cb7710ca5d2421d1c"
	auditModuleSHA256   = "e39bd1cf09c15b52b9495b8e19e6b0627f41312f1e60453f4427695baf29e684"
	auditModuleBytes    = 3852285
	auditZigSHA256      = "e6cd688d25664983833aae272f501d4bceeae304875b8f1741209d15fd13a4ec"
	auditWasmOptSHA256  = "2bf3eadf0b156ab57982f580b9603b5034a22878e08a9516310c14932bd533d4"
)

type auditSource struct {
	Crate          string `json:"crate"`
	Version        string `json:"version"`
	ArchiveSHA256  string `json:"archive_sha256"`
	Licence        string `json:"licence"`
	LicenceSHA256  string `json:"licence_sha256"`
	ArchiveBytes   int    `json:"archive_bytes"`
	Entries        int    `json:"entries"`
	Files          int    `json:"files"`
	ExtractedBytes int    `json:"extracted_bytes"`
}
type auditLock struct {
	Schema string `json:"schema"`
	Module struct {
		Blob       string `json:"blob"`
		EntryPoint string `json:"entrypoint"`
		SHA256     string `json:"sha256"`
		Bytes      int    `json:"bytes"`
		ABI        uint32 `json:"abi"`
	} `json:"module"`
	Grammar       auditSource `json:"grammar"`
	Runtime       auditSource `json:"runtime"`
	BuilderSHA256 string      `json:"builder_sha256"`
	BudgetSHA256  string      `json:"budget_sha256"`
	Toolchain     struct {
		ZigVersion     string `json:"zig_version"`
		ZigSHA256      string `json:"zig_sha256"`
		WasmOptVersion string `json:"wasm_opt_version"`
		WasmOptSHA256  string `json:"wasm_opt_sha256"`
	} `json:"toolchain"`
}
type auditCandidate struct {
	Raw                                          []byte
	Spec                                         tsengine.GrammarAuditSpec
	ModuleSHA256, GrammarVersion, RuntimeVersion string
	GrammarArchiveSHA256, RuntimeArchiveSHA256   string
}

func hex64(s string) bool {
	if len(s) != 64 {
		return false
	}
	_, err := hex.DecodeString(s)
	return err == nil && strings.ToLower(s) == s
}

func auditPath(file string) error {
	if file == "" {
		return errors.New("audit path absent")
	}
	abs, err := filepath.Abs(file)
	if err != nil {
		return err
	}
	cur := string(filepath.Separator)
	for _, part := range strings.Split(strings.TrimPrefix(filepath.Clean(abs), cur), cur) {
		cur = filepath.Join(cur, part)
		st, err := os.Lstat(cur)
		if err != nil {
			return err
		}
		if st.Mode()&os.ModeSymlink != 0 {
			return errors.New("audit path symlink")
		}
	}
	repo, err := git(".", "rev-parse", "--show-toplevel")
	if err != nil {
		return err
	}
	common, err := git(repo, "rev-parse", "--git-common-dir")
	if err != nil {
		return err
	}
	if !filepath.IsAbs(common) {
		common = filepath.Join(repo, common)
	}
	common, err = filepath.EvalSymlinks(common)
	if err != nil {
		return err
	}
	for _, root := range []string{repo, filepath.Dir(common)} {
		if err := exec.Command("git", "-C", root, "check-ignore", "-q", "workspace/").Run(); err != nil {
			continue
		}
		wr, err := filepath.Abs(filepath.Join(root, "workspace"))
		if err == nil && under(abs, wr) && abs != wr {
			return nil
		}
	}
	return errors.New("audit input outside ignored workspace")
}

func auditRead(file string, cap int64) ([]byte, error) {
	if err := auditPath(file); err != nil {
		return nil, err
	}
	f, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	before, err := f.Stat()
	if err != nil || !before.Mode().IsRegular() || before.Size() > cap {
		return nil, errors.New("audit input type or cap")
	}
	b, err := io.ReadAll(io.LimitReader(f, cap+1))
	if err != nil {
		return nil, err
	}
	after, err := f.Stat()
	if err != nil || int64(len(b)) != before.Size() || len(b) > int(cap) || !os.SameFile(before, after) || !before.ModTime().Equal(after.ModTime()) {
		return nil, errors.New("audit input changed")
	}
	return b, nil
}

func loadAuditCandidate(c config) (*auditCandidate, error) {
	if filepath.Base(c.CandidateLock) != "grammars.lock.json" || filepath.Ext(c.CandidateModule) != ".wasm" {
		return nil, errors.New("audit input names refused")
	}
	lockBytes, err := auditRead(c.CandidateLock, auditLockCap)
	if err != nil {
		return nil, err
	}
	var lock auditLock
	lockSHA, err := boundedJSON(c.CandidateLock, auditLockCap, -1, &lock)
	if err != nil {
		return nil, err
	}
	sum := sha256.Sum256(lockBytes)
	if lockSHA != hex.EncodeToString(sum[:]) {
		return nil, errors.New("audit lock changed")
	}
	dec := json.NewDecoder(bytes.NewReader(lockBytes))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&lock); err != nil {
		return nil, err
	}
	if _, err := dec.Token(); err != io.EOF {
		return nil, errors.New("audit lock trailing data")
	}
	root, err := git(".", "rev-parse", "--show-toplevel")
	if err != nil {
		return nil, err
	}
	builder := "calibration/oracles/scip-clang/build-willbooster.py"
	if lock.Module.SHA256 == "fde7035227a488422e95cc1a408fecbbc5b3440dceebf109e42d483d31270ed2" || lock.Module.SHA256 == "613af35868891df920a1e5c12fb67b2c2e83eb6a7c9cab6d3a1247acd20d22df" {
		builder = "calibration/oracles/scip-clang/build-cpp-candidates.py"
	}
	for _, source := range []struct{ rel, want string }{
		{builder, lock.BuilderSHA256},
		{"internal/extractor/tsengine/grammars/budget.c", lock.BudgetSHA256},
	} {
		b, err := os.ReadFile(filepath.Join(root, source.rel))
		if err != nil {
			return nil, err
		}
		sum := sha256.Sum256(b)
		if hex.EncodeToString(sum[:]) != source.want {
			return nil, errors.New("audit builder or budget source mismatch")
		}
	}
	if !knownAuditMetadata(lock) {
		return nil, errors.New("audit lock metadata refused")
	}

	raw, err := auditRead(c.CandidateModule, auditModuleCap)
	if err != nil {
		return nil, err
	}
	sum = sha256.Sum256(raw)
	if len(raw) != lock.Module.Bytes || hex.EncodeToString(sum[:]) != lock.Module.SHA256 {
		return nil, errors.New("audit module identity mismatch")
	}
	return &auditCandidate{Raw: raw, Spec: tsengine.GrammarAuditSpec{Blob: "cpp", EntryPoint: "tree_sitter_cpp", SHA256: lock.Module.SHA256, Bytes: lock.Module.Bytes, ABI: lock.Module.ABI, Licence: "MIT"}, ModuleSHA256: lock.Module.SHA256, GrammarVersion: lock.Grammar.Version, RuntimeVersion: lock.Runtime.Version, GrammarArchiveSHA256: lock.Grammar.ArchiveSHA256, RuntimeArchiveSHA256: lock.Runtime.ArchiveSHA256}, nil
}

func knownAuditMetadata(lock auditLock) bool {
	if lock.Schema != "fathom-cpp-external-audit-v1" || lock.Module.Blob != "cpp" || lock.Module.EntryPoint != "tree_sitter_cpp" || !hex64(lock.BuilderSHA256) || !hex64(lock.BudgetSHA256) || lock.Toolchain.ZigSHA256 != auditZigSHA256 || lock.Toolchain.WasmOptSHA256 != auditWasmOptSHA256 || lock.Toolchain.ZigVersion != "0.16.0" || lock.Toolchain.WasmOptVersion != "129" {
		return false
	}
	if lock.Module.SHA256 == auditModuleSHA256 {
		return lock.Module.ABI == 0x57420006 && lock.Module.Bytes == auditModuleBytes &&
			validAuditSource(lock.Grammar, "willbooster-tree-sitter-cpp", "6.0.2", auditGrammarArchive, auditGrammarLicence, 1365727) &&
			validAuditSource(lock.Runtime, "willbooster-tree-sitter", "7.0.5", auditRuntimeArchive, auditRuntimeLicence, 246632)
	}
	if lock.Module.ABI != 15 || !validAuditSource(lock.Runtime, "tree-sitter/tree-sitter", "0.26.11", "fc2a4d846c36f7d19714085bbad76a68c5ff9d0fe7b1e2a6a62a22446efe966e", "c5cfb43042b6b72045f4ba997834d0a7786d2793d91680868b5815b39f14fc78", 920896) {
		return false
	}
	switch lock.Module.SHA256 {
	case "fde7035227a488422e95cc1a408fecbbc5b3440dceebf109e42d483d31270ed2":
		return lock.Module.Bytes == 2790800 && validAuditSource(lock.Grammar, "tree-sitter/tree-sitter-cpp", "c009222808634c1014f82438d4883753516a2c24", "6c6efaf024f5c02fee0403e4e2373346eeadc40828587bcfad9351df2f0a7986", "2e0110e07abef7c2548b26ec9d6969775617ca539a0dc8dbeeb14d6452c711d1", 1740811)
	case "613af35868891df920a1e5c12fb67b2c2e83eb6a7c9cab6d3a1247acd20d22df":
		return lock.Module.Bytes == 3975018 && validAuditSource(lock.Grammar, "taku25/tree-sitter-unreal-cpp", "920e9810cc64a47fa05cfcf30c7caecb85b5f2c3", "285ad7c2969a03254ed7a77068e0effca26cb8f014bc5b9d300bd4415ec0b17b", "ef02f1077281d63262946e50ec461ba72efda41e51084c887379b59397b96a6d", 2628614)
	}
	return false
}

func validAuditSource(s auditSource, crate, version, archive, licence string, archiveBytes int) bool {
	return s.Crate == crate && s.Version == version && s.ArchiveSHA256 == archive &&
		s.Licence == "MIT" && s.LicenceSHA256 == licence && s.ArchiveBytes == archiveBytes &&
		s.Entries > 0 && s.Entries <= 1000 && s.Files > 0 && s.Files <= s.Entries &&
		s.ExtractedBytes > 0 && s.ExtractedBytes <= 128<<20
}

func openAuditParser(ctx context.Context, a *auditCandidate) (*tsengine.Parser, error) {
	return tsengine.NewGrammarAuditParser(ctx, a.Raw, a.Spec)
}
