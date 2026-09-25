package main

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"golang.org/x/mod/modfile"
	"golang.org/x/mod/module"
	"golang.org/x/mod/sumdb/dirhash"
	modzip "golang.org/x/mod/zip"
)

const candidateModule, maxFiles, maxFileBytes, maxTotalBytes = "github.com/geoffmilleraz/app-kit", 4000, 8 << 20, 64 << 20

var sourceDirs = []string{"appkit", "apps", "cmd", "internal", "cue", "tools", "docs", "fathom-pack"}
var sourceRootFiles = []string{
	"go.mod", "go.sum", "LICENSE", "README.md", "CHANGELOG.md", "AGENTS.md", "CLAUDE.md",
	"VISION.md", "ROADMAP.md", "PRODUCT.md", "Taskfile.yml",
}

type FileRecord struct {
	Path   string `json:"path"`
	Bytes  int64  `json:"bytes"`
	SHA256 string `json:"sha256"`
}
type Exclusion struct {
	Path   string `json:"path"`
	Reason string `json:"reason"`
}
type Receipt struct {
	Kind        string       `json:"kind"`
	Module      string       `json:"module"`
	Version     string       `json:"version"`
	SourceHEAD  string       `json:"source_head"`
	TreeSHA256  string       `json:"tree_sha256"`
	ZipSHA256   string       `json:"zip_sha256"`
	ModuleSum   string       `json:"module_sum"`
	GoModSum    string       `json:"go_mod_sum"`
	Files       []FileRecord `json:"files"`
	TotalBytes  int64        `json:"total_bytes"`
	Exclusions  []Exclusion  `json:"exclusions,omitempty"`
	Limitations []string     `json:"limitations"`
	Output      string       `json:"output"`
}
type snapshotFile struct {
	path string
	body []byte
	info os.FileInfo
}

func (f snapshotFile) Path() string                { return f.path }
func (f snapshotFile) Lstat() (os.FileInfo, error) { return f.info, nil }
func (f snapshotFile) Open() (io.ReadCloser, error) {
	return io.NopCloser(bytes.NewReader(f.body)), nil
}

type snapshot struct {
	files      []snapshotFile
	records    []FileRecord
	exclusions []Exclusion
	total      int64
	treeHash   string
}

func prepare(ctx context.Context, root, out string) (Receipt, error) {
	if ctx == nil {
		return Receipt{}, errors.New("context is required")
	}
	root, out, err := validateLocations(root, out)
	if err != nil {
		return Receipt{}, err
	}
	if err := ctx.Err(); err != nil {
		return Receipt{}, err
	}
	headBeforeBytes, err := gitOutputBytes(ctx, root, "rev-parse", "--verify", "HEAD")
	if err != nil {
		return Receipt{}, fmt.Errorf("read source HEAD: %w", err)
	}
	first, err := takeSnapshot(ctx, root)
	if err != nil {
		return Receipt{}, err
	}
	second, err := takeSnapshot(ctx, root)
	if err != nil {
		return Receipt{}, err
	}
	headAfterBytes, err := gitOutputBytes(ctx, root, "rev-parse", "--verify", "HEAD")
	if err != nil {
		return Receipt{}, fmt.Errorf("re-read source HEAD: %w", err)
	}
	headBefore, headAfter := strings.TrimSpace(string(headBeforeBytes)), strings.TrimSpace(string(headAfterBytes))
	if headBefore == "" || headBefore != headAfter || !snapshotsEqual(first, second) {
		return Receipt{}, errors.New("source changed during candidate snapshot")
	}
	modBytes, err := moduleFile(first)
	if err != nil {
		return Receipt{}, err
	}
	version := "v0.0.0-rehearsal.h" + first.treeHash
	mv := module.Version{Path: candidateModule, Version: version}
	zipBytes, err := createZipTwice(mv, first.files)
	if err != nil {
		return Receipt{}, err
	}
	moduleSum, err := memoryModuleSum(zipBytes)
	if err != nil {
		return Receipt{}, fmt.Errorf("compute module sum: %w", err)
	}
	goModSum, err := memoryFileSum("go.mod", modBytes)
	if err != nil {
		return Receipt{}, fmt.Errorf("compute go.mod sum: %w", err)
	}
	receipt := Receipt{
		Kind: "maintainer-private-unpublished-rehearsal", Module: candidateModule, Version: version,
		SourceHEAD: headBefore, TreeSHA256: first.treeHash, ZipSHA256: hashBytes(zipBytes),
		ModuleSum: moduleSum, GoModSum: goModSum, Files: first.records, TotalBytes: first.total,
		Exclusions: first.exclusions, Output: out,
		Limitations: []string{
			"maintainer checkout rehearsal only; not a published release",
			"selection is two compared Git-visible path-and-byte snapshots, not an adversarial atomic filesystem capture",
			"allowlisting and .env refusal are not a secret or license scan",
			"proxy info time is the fixed Unix epoch, not source commit time",
		},
	}
	if err := writeCandidate(ctx, out, mv, modBytes, zipBytes, receipt); err != nil {
		return Receipt{}, err
	}
	return receipt, nil
}
func validateLocations(root, out string) (string, string, error) {
	if root == "" || out == "" {
		return "", "", errors.New("source root and output directory are required")
	}
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return "", "", fmt.Errorf("resolve source root: %w", err)
	}
	rootInfo, err := os.Lstat(rootAbs)
	if err != nil {
		return "", "", fmt.Errorf("inspect source root: %w", err)
	}
	if rootInfo.Mode()&os.ModeSymlink != 0 || !rootInfo.IsDir() {
		return "", "", errors.New("source root must be a real directory, not a symlink")
	}
	rootReal, err := filepath.EvalSymlinks(rootAbs)
	if err != nil {
		return "", "", fmt.Errorf("canonicalize source root: %w", err)
	}
	outAbs, err := filepath.Abs(out)
	if err != nil {
		return "", "", fmt.Errorf("resolve output directory: %w", err)
	}
	if _, err := os.Lstat(outAbs); err == nil {
		return "", "", errors.New("output directory already exists")
	} else if !os.IsNotExist(err) {
		return "", "", fmt.Errorf("inspect output directory: %w", err)
	}
	parentReal, err := filepath.EvalSymlinks(filepath.Dir(outAbs))
	if err != nil {
		return "", "", fmt.Errorf("canonicalize output parent: %w", err)
	}
	outReal := filepath.Join(parentReal, filepath.Base(outAbs))
	if within(rootReal, outReal) {
		return "", "", errors.New("output directory must be outside the source root")
	}
	return rootReal, outReal, nil
}
func within(root, path string) bool {
	rel, err := filepath.Rel(root, path)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}
func takeSnapshot(ctx context.Context, root string) (snapshot, error) {
	pathspecs := append(append([]string(nil), sourceDirs...), sourceRootFiles...)
	deletedRaw, err := gitOutputBytes(ctx, root, append([]string{"ls-files", "--deleted", "-z", "--"}, pathspecs...)...)
	if err != nil {
		return snapshot{}, fmt.Errorf("select tracked deletions: %w", err)
	}
	deleted := make(map[string]bool)
	for _, path := range splitGitPaths(deletedRaw) {
		if !validSelectedPath(path) {
			return snapshot{}, fmt.Errorf("Git returned unsafe or unallowlisted deleted path %q", path)
		}
		deleted[path] = true
	}
	args := []string{"ls-files", "--cached", "--others", "--exclude-standard", "-z", "--"}
	args = append(args, sourceDirs...)
	args = append(args, sourceRootFiles...)
	raw, err := gitOutputBytes(ctx, root, args...)
	if err != nil {
		return snapshot{}, fmt.Errorf("select Git-visible source files: %w", err)
	}
	paths := splitGitPaths(raw)
	sort.Strings(paths)
	result := snapshot{exclusions: make([]Exclusion, 0, len(deleted))}
	for path := range deleted {
		result.exclusions = append(result.exclusions, Exclusion{Path: path, Reason: "tracked deletion in working tree"})
	}
	seen := ""
	for _, path := range paths {
		if path == "" || path == seen {
			continue
		}
		seen = path
		if err := ctx.Err(); err != nil {
			return snapshot{}, err
		}
		if !validSelectedPath(path) {
			return snapshot{}, fmt.Errorf("Git returned unsafe or unallowlisted path %q", path)
		}
		if strings.HasPrefix(filepath.Base(path), ".env") {
			return snapshot{}, fmt.Errorf("environment-prefixed file refused: %s", path)
		}
		if deleted[path] {
			continue
		}
		if nestedModule, err := nestedModuleFor(root, path); err != nil {
			return snapshot{}, err
		} else if nestedModule != "" {
			result.exclusions = append(result.exclusions, Exclusion{Path: path, Reason: "nested Go module " + nestedModule})
			continue
		}
		f, err := readSnapshotFile(root, path)
		if err != nil {
			return snapshot{}, err
		}
		if len(result.files) >= maxFiles {
			return snapshot{}, fmt.Errorf("selected file count exceeds %d", maxFiles)
		}
		result.total += int64(len(f.body))
		if result.total > maxTotalBytes {
			return snapshot{}, fmt.Errorf("selected byte count exceeds %d", maxTotalBytes)
		}
		result.files = append(result.files, f)
		result.records = append(result.records, FileRecord{Path: path, Bytes: int64(len(f.body)), SHA256: hashBytes(f.body)})
	}
	if len(result.files) == 0 {
		return snapshot{}, errors.New("source selection is empty")
	}
	sort.Slice(result.exclusions, func(i, j int) bool { return result.exclusions[i].Path < result.exclusions[j].Path })
	result.treeHash = treeHash(result.files)
	return result, nil
}

func splitGitPaths(raw []byte) []string {
	if len(raw) == 0 {
		return nil
	}
	return strings.Split(strings.TrimSuffix(string(raw), "\x00"), "\x00")
}
func validSelectedPath(path string) bool {
	if !filepath.IsLocal(path) || filepath.ToSlash(filepath.Clean(path)) != path {
		return false
	}
	for _, file := range sourceRootFiles {
		if path == file {
			return true
		}
	}
	for _, dir := range sourceDirs {
		if strings.HasPrefix(path, dir+"/") {
			return true
		}
	}
	return false
}
func nestedModuleFor(root, path string) (string, error) {
	for dir := filepath.Dir(path); dir != "."; dir = filepath.Dir(dir) {
		_, err := os.Lstat(filepath.Join(root, filepath.FromSlash(dir), "go.mod"))
		if err == nil {
			return filepath.ToSlash(dir), nil
		}
		if !os.IsNotExist(err) {
			return "", fmt.Errorf("inspect nested module %s: %w", dir, err)
		}
	}
	return "", nil
}
func readSnapshotFile(root, path string) (snapshotFile, error) {
	for dir := filepath.Dir(path); dir != "."; dir = filepath.Dir(dir) {
		info, err := os.Lstat(filepath.Join(root, filepath.FromSlash(dir)))
		if err != nil {
			return snapshotFile{}, fmt.Errorf("inspect source ancestor %s: %w", dir, err)
		}
		if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
			return snapshotFile{}, fmt.Errorf("source ancestor is not a real directory: %s", dir)
		}
	}
	full := filepath.Join(root, filepath.FromSlash(path))
	info, err := os.Lstat(full)
	if err != nil {
		return snapshotFile{}, fmt.Errorf("inspect source file %s: %w", path, err)
	}
	if !info.Mode().IsRegular() {
		return snapshotFile{}, fmt.Errorf("source file is symlink or special: %s", path)
	}
	if info.Size() > maxFileBytes {
		return snapshotFile{}, fmt.Errorf("source file exceeds %d bytes: %s", maxFileBytes, path)
	}
	file, err := os.Open(full)
	if err != nil {
		return snapshotFile{}, fmt.Errorf("open source file %s: %w", path, err)
	}
	body, err := io.ReadAll(io.LimitReader(file, maxFileBytes+1))
	closeErr := file.Close()
	if err != nil {
		return snapshotFile{}, fmt.Errorf("read source file %s: %w", path, err)
	}
	if closeErr != nil {
		return snapshotFile{}, fmt.Errorf("close source file %s: %w", path, closeErr)
	}
	if len(body) > maxFileBytes {
		return snapshotFile{}, fmt.Errorf("source file exceeds %d bytes: %s", maxFileBytes, path)
	}
	after, err := os.Lstat(full)
	if err != nil || !os.SameFile(info, after) || after.Size() != int64(len(body)) || after.Mode() != info.Mode() {
		return snapshotFile{}, fmt.Errorf("source file changed while reading: %s", path)
	}
	return snapshotFile{path: path, body: body, info: info}, nil
}
func moduleFile(s snapshot) ([]byte, error) {
	var body []byte
	for _, f := range s.files {
		if f.path == "go.mod" {
			body = f.body
			break
		}
	}
	if body == nil {
		return nil, errors.New("selected source has no go.mod")
	}
	mf, err := modfile.Parse("go.mod", body, nil)
	if err != nil {
		return nil, fmt.Errorf("parse go.mod: %w", err)
	}
	if mf.Module == nil || mf.Module.Mod.Path != candidateModule {
		return nil, fmt.Errorf("go.mod module must be exactly %s", candidateModule)
	}
	if len(mf.Replace) != 0 {
		return nil, errors.New("go.mod replacements are forbidden in a release candidate")
	}
	return body, nil
}
func createZipTwice(m module.Version, files []snapshotFile) ([]byte, error) {
	zfiles := make([]modzip.File, len(files))
	for i := range files {
		zfiles[i] = files[i]
	}
	checked, err := modzip.CheckFiles(zfiles)
	if err != nil {
		return nil, fmt.Errorf("check module files: %w", err)
	}
	if len(checked.Omitted) != 0 {
		return nil, fmt.Errorf("unexpected module ZIP omissions: %v", checked.Omitted)
	}
	var first, second bytes.Buffer
	if err := modzip.Create(&first, m, zfiles); err != nil {
		return nil, fmt.Errorf("create module ZIP: %w", err)
	}
	if err := modzip.Create(&second, m, zfiles); err != nil {
		return nil, fmt.Errorf("recreate module ZIP: %w", err)
	}
	if !bytes.Equal(first.Bytes(), second.Bytes()) {
		return nil, errors.New("module ZIP is not reproducible from the same snapshot")
	}
	zr, err := zip.NewReader(bytes.NewReader(first.Bytes()), int64(first.Len()))
	if err != nil || len(zr.File) != len(files) {
		return nil, errors.New("module ZIP inventory differs from selected files")
	}
	return first.Bytes(), nil
}
func memoryModuleSum(body []byte) (string, error) {
	zr, err := zip.NewReader(bytes.NewReader(body), int64(len(body)))
	if err != nil {
		return "", err
	}
	byPath := make(map[string]*zip.File, len(zr.File))
	paths := make([]string, len(zr.File))
	for i, f := range zr.File {
		paths[i], byPath[f.Name] = f.Name, f
	}
	return dirhash.Hash1(paths, func(path string) (io.ReadCloser, error) {
		file, ok := byPath[path]
		if !ok {
			return nil, os.ErrNotExist
		}
		return file.Open()
	})
}

func memoryFileSum(path string, body []byte) (string, error) {
	return dirhash.Hash1([]string{path}, func(name string) (io.ReadCloser, error) {
		if name != path {
			return nil, os.ErrNotExist
		}
		return io.NopCloser(bytes.NewReader(body)), nil
	})
}
func writeCandidate(ctx context.Context, out string, m module.Version, modBytes, zipBytes []byte, receipt Receipt) error {
	if err := os.Mkdir(out, 0700); err != nil {
		return fmt.Errorf("create output directory: %w", err)
	}
	escaped, err := module.EscapePath(m.Path)
	if err != nil {
		return fmt.Errorf("escape module path: %w", err)
	}
	version, err := module.EscapeVersion(m.Version)
	if err != nil {
		return fmt.Errorf("escape module version: %w", err)
	}
	dir := filepath.Join(out, "proxy", filepath.FromSlash(escaped), "@v")
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("create proxy directories: %w", err)
	}
	info := []byte(`{"Version":"` + m.Version + `","Time":"1970-01-01T00:00:00Z"}`)
	receiptBytes, err := json.MarshalIndent(receipt, "", "  ")
	if err != nil {
		return err
	}
	artifacts := []struct {
		path string
		body []byte
	}{
		{filepath.Join(dir, version+".zip"), zipBytes},
		{filepath.Join(dir, version+".mod"), modBytes},
		{filepath.Join(dir, version+".info"), info},
		{filepath.Join(dir, "list"), []byte(m.Version + "\n")},
		{filepath.Join(out, "receipt.json"), append(receiptBytes, '\n')},
	}
	for _, artifact := range artifacts {
		if err := ctx.Err(); err != nil {
			return fmt.Errorf("write candidate: %w", err)
		}
		if err := writeExclusive(artifact.path, artifact.body); err != nil {
			return err
		}
	}
	return nil
}
func writeExclusive(path string, body []byte) error {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return fmt.Errorf("create %s: %w", path, err)
	}
	if _, err := f.Write(body); err != nil {
		_ = f.Close()
		return fmt.Errorf("write %s: %w", path, err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("close %s: %w", path, err)
	}
	return nil
}
func snapshotsEqual(a, b snapshot) bool {
	if a.treeHash != b.treeHash || a.total != b.total || len(a.files) != len(b.files) || len(a.exclusions) != len(b.exclusions) {
		return false
	}
	for i := range a.files {
		if a.files[i].path != b.files[i].path || a.files[i].info.Mode() != b.files[i].info.Mode() || !bytes.Equal(a.files[i].body, b.files[i].body) {
			return false
		}
	}
	for i := range a.exclusions {
		if a.exclusions[i] != b.exclusions[i] {
			return false
		}
	}
	return true
}
func treeHash(files []snapshotFile) string {
	// A structured inventory avoids delimiter ambiguity in binary file bodies.
	records := make([]FileRecord, len(files))
	for i, f := range files {
		records[i] = FileRecord{Path: f.path, Bytes: int64(len(f.body)), SHA256: hashBytes(f.body)}
	}
	body, _ := json.Marshal(records) // only strings and integers; cannot fail
	return hashBytes(body)
}
func hashBytes(body []byte) string { return fmt.Sprintf("%x", sha256.Sum256(body)) }

func gitOutputBytes(ctx context.Context, root string, args ...string) ([]byte, error) {
	command := exec.CommandContext(ctx, "git", append([]string{"-C", root}, args...)...)
	command.Env = append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL="+os.DevNull)
	body, err := command.Output()
	if err != nil {
		return nil, err
	}
	return body, nil
}
