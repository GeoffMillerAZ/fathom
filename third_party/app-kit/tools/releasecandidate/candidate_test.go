package main

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/mod/sumdb/dirhash"
)

func TestPrepareReproducibleProxyAndInventory(t *testing.T) {
	repo := fixtureRepo(t)
	firstOut := filepath.Join(t.TempDir(), "first")
	secondOut := filepath.Join(t.TempDir(), "second")
	first, err := prepare(context.Background(), repo, firstOut)
	if err != nil {
		t.Fatal(err)
	}
	second, err := prepare(context.Background(), repo, secondOut)
	if err != nil {
		t.Fatal(err)
	}
	if first.TreeSHA256 != second.TreeSHA256 || first.Version != second.Version || first.ZipSHA256 != second.ZipSHA256 || first.ModuleSum != second.ModuleSum || first.GoModSum != second.GoModSum {
		t.Fatalf("candidate was not reproducible:\n%+v\n%+v", first, second)
	}
	if !strings.HasSuffix(first.Version, first.TreeSHA256) || len(first.TreeSHA256) != 64 {
		t.Fatalf("version does not bind full tree hash: %q", first.Version)
	}
	zipPath := proxyArtifact(t, firstOut, first.Version, ".zip")
	if info, err := os.Stat(firstOut); err != nil || info.Mode().Perm() != 0700 {
		t.Fatalf("output mode = %v, err = %v", info, err)
	}
	for _, path := range []string{zipPath, proxyArtifact(t, firstOut, first.Version, ".mod"), proxyArtifact(t, firstOut, first.Version, ".info"), filepath.Join(firstOut, "receipt.json")} {
		if info, err := os.Stat(path); err != nil || info.Mode().Perm() != 0600 {
			t.Fatalf("artifact %s mode = %v, err = %v", path, info, err)
		}
	}
	gotSum, err := dirhash.HashZip(zipPath, dirhash.Hash1)
	if err != nil {
		t.Fatal(err)
	}
	if gotSum != first.ModuleSum {
		t.Fatalf("module sum = %q, HashZip = %q", first.ModuleSum, gotSum)
	}
	wantMod, err := os.ReadFile(filepath.Join(repo, "go.mod"))
	if err != nil {
		t.Fatal(err)
	}
	gotMod := readArtifact(t, firstOut, first.Version, ".mod")
	if !bytes.Equal(gotMod, wantMod) {
		t.Fatal("proxy .mod bytes differ from selected root go.mod")
	}
	wantGoModSum, err := dirhash.Hash1([]string{"go.mod"}, func(string) (io.ReadCloser, error) {
		return io.NopCloser(bytes.NewReader(wantMod)), nil
	})
	if err != nil || first.GoModSum != wantGoModSum {
		t.Fatalf("go.mod sum = %q, independent = %q, err = %v", first.GoModSum, wantGoModSum, err)
	}
	zr, err := zip.OpenReader(zipPath)
	if err != nil {
		t.Fatal(err)
	}
	defer zr.Close()
	if len(zr.File) != len(first.Files) {
		t.Fatalf("ZIP files = %d, receipt files = %d", len(zr.File), len(first.Files))
	}
	prefix := candidateModule + "@" + first.Version + "/"
	for i, f := range zr.File {
		if f.Name != prefix+first.Files[i].Path {
			t.Fatalf("ZIP entry %d = %q, want %q", i, f.Name, prefix+first.Files[i].Path)
		}
		reader, err := f.Open()
		if err != nil {
			t.Fatal(err)
		}
		body, readErr := io.ReadAll(reader)
		closeErr := reader.Close()
		if readErr != nil || closeErr != nil {
			t.Fatalf("read ZIP entry %q: read=%v close=%v", f.Name, readErr, closeErr)
		}
		record := first.Files[i]
		if int64(len(body)) != record.Bytes || hashBytes(body) != record.SHA256 {
			t.Fatalf("ZIP entry %q differs from receipt: bytes=%d/%d sha=%s/%s", f.Name, len(body), record.Bytes, hashBytes(body), record.SHA256)
		}
	}
	var saved Receipt
	body, err := os.ReadFile(filepath.Join(firstOut, "receipt.json"))
	if err != nil || json.Unmarshal(body, &saved) != nil || saved.ZipSHA256 != first.ZipSHA256 {
		t.Fatalf("invalid saved receipt: %v", err)
	}
	info := readArtifact(t, firstOut, first.Version, ".info")
	if !bytes.Contains(info, []byte("1970-01-01T00:00:00Z")) {
		t.Fatalf("info does not use fixed epoch: %s", info)
	}
}

func TestPrepareRecordsStableTrackedDeletion(t *testing.T) {
	repo := fixtureRepo(t)
	deletedPath := "apps/keep.txt"
	if err := os.Remove(filepath.Join(repo, deletedPath)); err != nil {
		t.Fatal(err)
	}
	receipt, err := prepare(context.Background(), repo, filepath.Join(t.TempDir(), "candidate"))
	if err != nil {
		t.Fatal(err)
	}
	for _, file := range receipt.Files {
		if file.Path == deletedPath {
			t.Fatalf("tracked deletion was archived: %s", deletedPath)
		}
	}
	for _, exclusion := range receipt.Exclusions {
		if exclusion.Path == deletedPath && exclusion.Reason == "tracked deletion in working tree" {
			return
		}
	}
	t.Fatalf("tracked deletion not recorded: %+v", receipt.Exclusions)
}

func TestReadSnapshotFileStillRefusesUnrecordedAbsence(t *testing.T) {
	repo := fixtureRepo(t)
	if _, err := readSnapshotFile(repo, "apps/not-present.txt"); err == nil || !strings.Contains(err.Error(), "inspect source file") {
		t.Fatalf("missing selected-file error = %v", err)
	}
}

func TestPrepareRefusesCollisionAndInsideSource(t *testing.T) {
	repo := fixtureRepo(t)
	out := filepath.Join(t.TempDir(), "candidate")
	if err := os.Mkdir(out, 0700); err != nil {
		t.Fatal(err)
	}
	if _, err := prepare(context.Background(), repo, out); err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("collision error = %v", err)
	}
	if _, err := prepare(context.Background(), repo, filepath.Join(repo, "output")); err == nil || !strings.Contains(err.Error(), "outside") {
		t.Fatalf("inside-source error = %v", err)
	}
}

func TestPrepareExcludesPrivateAndNestedModuleFiles(t *testing.T) {
	repo := fixtureRepo(t)
	writeFixture(t, repo, "data/private.txt", "private")
	writeFixture(t, repo, "apps/nested/go.mod", "module example.com/nested\n")
	writeFixture(t, repo, "apps/nested/secret.txt", "nested")
	git(t, repo, "add", "apps/nested/go.mod", "apps/nested/secret.txt")
	receipt, err := prepare(context.Background(), repo, filepath.Join(t.TempDir(), "candidate"))
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range receipt.Files {
		if strings.HasPrefix(f.Path, "data/") || strings.HasPrefix(f.Path, "apps/nested/") {
			t.Fatalf("private or nested file selected: %s", f.Path)
		}
	}
	if len(receipt.Exclusions) != 2 || receipt.Exclusions[0].Reason != "nested Go module apps/nested" {
		t.Fatalf("nested exclusions = %+v", receipt.Exclusions)
	}
}

func TestPrepareRefusesSymlinksAndEnvironmentFiles(t *testing.T) {
	t.Run("source root", func(t *testing.T) {
		repo := fixtureRepo(t)
		link := filepath.Join(t.TempDir(), "repo-link")
		if err := os.Symlink(repo, link); err != nil {
			t.Fatal(err)
		}
		if _, err := prepare(context.Background(), link, filepath.Join(t.TempDir(), "out")); err == nil || !strings.Contains(err.Error(), "symlink") {
			t.Fatalf("root symlink error = %v", err)
		}
	})
	t.Run("selected leaf", func(t *testing.T) {
		repo := fixtureRepo(t)
		if err := os.Symlink(filepath.Join(repo, "README.md"), filepath.Join(repo, "apps", "linked")); err != nil {
			t.Fatal(err)
		}
		git(t, repo, "add", "apps/linked")
		if _, err := prepare(context.Background(), repo, filepath.Join(t.TempDir(), "out")); err == nil || !strings.Contains(err.Error(), "symlink or special") {
			t.Fatalf("leaf symlink error = %v", err)
		}
	})
	t.Run("environment prefix", func(t *testing.T) {
		repo := fixtureRepo(t)
		writeFixture(t, repo, "apps/.env.production", "no")
		if _, err := prepare(context.Background(), repo, filepath.Join(t.TempDir(), "out")); err == nil || !strings.Contains(err.Error(), "environment-prefixed") {
			t.Fatalf("environment error = %v", err)
		}
	})
}

func TestPrepareRefusesInvalidSource(t *testing.T) {
	tests := []struct {
		name string
		edit func(*testing.T, string)
		want string
	}{
		{"wrong module", func(t *testing.T, repo string) { writeFixture(t, repo, "go.mod", "module example.com/wrong\n") }, "module must be exactly"},
		{"replace", func(t *testing.T, repo string) {
			writeFixture(t, repo, "go.mod", "module "+candidateModule+"\n\nreplace example.com/a => ../a\n")
		}, "replacements are forbidden"},
		{"oversized", func(t *testing.T, repo string) {
			f, err := os.OpenFile(filepath.Join(repo, "apps", "huge"), os.O_CREATE|os.O_WRONLY, 0600)
			if err != nil {
				t.Fatal(err)
			}
			if err := f.Truncate(maxFileBytes + 1); err != nil {
				t.Fatal(err)
			}
			if err := f.Close(); err != nil {
				t.Fatal(err)
			}
		}, "exceeds"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			repo := fixtureRepo(t)
			tc.edit(t, repo)
			if _, err := prepare(context.Background(), repo, filepath.Join(t.TempDir(), "out")); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %v, want %q", err, tc.want)
			}
		})
	}
}

func fixtureRepo(t *testing.T) string {
	t.Helper()
	repo := t.TempDir()
	for _, dir := range sourceDirs {
		if err := os.MkdirAll(filepath.Join(repo, dir), 0700); err != nil {
			t.Fatal(err)
		}
		writeFixture(t, repo, filepath.Join(dir, "keep.txt"), dir)
	}
	for _, file := range sourceRootFiles {
		body := file + "\n"
		if file == "go.mod" {
			body = "module " + candidateModule + "\n\ngo 1.26\n"
		}
		writeFixture(t, repo, file, body)
	}
	git(t, repo, "init", "-q")
	git(t, repo, "add", ".")
	git(t, repo, "-c", "user.name=Test", "-c", "user.email=test@example.invalid", "commit", "-qm", "fixture")
	return repo
}

func writeFixture(t *testing.T, root, path, body string) {
	t.Helper()
	full := filepath.Join(root, path)
	if err := os.MkdirAll(filepath.Dir(full), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(body), 0600); err != nil {
		t.Fatal(err)
	}
}

func git(t *testing.T, root string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
	cmd.Env = append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL="+os.DevNull)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

func proxyArtifact(t *testing.T, out, version, suffix string) string {
	t.Helper()
	return filepath.Join(out, "proxy", filepath.FromSlash(candidateModule), "@v", version+suffix)
}

func readArtifact(t *testing.T, out, version, suffix string) []byte {
	t.Helper()
	body, err := os.ReadFile(proxyArtifact(t, out, version, suffix))
	if err != nil {
		t.Fatal(err)
	}
	return body
}
