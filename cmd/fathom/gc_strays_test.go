package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"
)

// elfHeader is enough of an ELF file for the binaries class to recognise.
var elfHeader = []byte{0x7f, 'E', 'L', 'F', 2, 1, 1, 0, 0, 0, 0, 0, 0, 0, 0, 0}

// machoHeader is a 64-bit little-endian Mach-O magic.
var machoHeader = []byte{0xcf, 0xfa, 0xed, 0xfe, 7, 0, 0, 1}

// SV-4 (ADR-060 decision 4, I63): `fathom gc` reports untracked,
// non-ignored files outside the declared homes and executable binaries in
// the tree, and --apply never removes them: they are not fathom's.
func TestGCStraysReportOnlyNeverApplied(t *testing.T) {
	repo := filepath.Join(t.TempDir(), "repo")
	if err := os.MkdirAll(repo, 0o755); err != nil {
		t.Fatal(err)
	}
	linesGit(t, repo, "init", "-q")
	linesWrite(t, repo, ".gitignore", "/build/\n")
	linesWrite(t, repo, "main.go", "package main\n")
	linesWrite(t, repo, "run.sh", "#!/bin/sh\necho hi\n")
	if err := os.Chmod(filepath.Join(repo, "run.sh"), 0o755); err != nil {
		t.Fatal(err)
	}
	// A committed executable binary.
	if err := os.MkdirAll(filepath.Join(repo, "tools"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "tools", "blob"), append(append([]byte{}, elfHeader...), make([]byte, 64)...), 0o755); err != nil {
		t.Fatal(err)
	}
	linesGit(t, repo, "add", "-A")
	linesGit(t, repo, "commit", "-q", "-m", "base")

	// Untracked: a stray note, a stray directory, an untracked binary at
	// the root, files inside the declared homes, and an ignored build dir.
	linesWrite(t, repo, "notes.txt", "scratch\n")
	linesWrite(t, repo, "scratch/a.txt", "a\n")
	linesWrite(t, repo, "scratch/b.txt", "bb\n")
	if err := os.WriteFile(filepath.Join(repo, "fathom"), append(append([]byte{}, machoHeader...), make([]byte, 32)...), 0o755); err != nil {
		t.Fatal(err)
	}
	linesWrite(t, repo, "workspace/logs/run.log", "log\n")
	cacheRoot := filepath.Join(repo, ".cache-root")
	linesWrite(t, cacheRoot, "go-build/x", "cached\n")
	linesWrite(t, repo, "build/out.bin", "ignored\n")
	// A non-executable file with an ELF header is not an executable binary.
	if err := os.WriteFile(filepath.Join(repo, "fixture.elf"), elfHeader, 0o644); err != nil {
		t.Fatal(err)
	}

	before := gcTreeDigest(t, repo)
	home, tmp := t.TempDir(), t.TempDir()
	rep := runGC(gcOptions{
		Home: home, TmpDir: tmp, Apply: true, Now: time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC),
		Repo: repo, CacheRoots: []string{cacheRoot},
	})
	after := gcTreeDigest(t, repo)
	if before != after {
		t.Fatalf("gc --apply changed the repository tree:\nbefore:\n%s\nafter:\n%s", before, after)
	}

	got := map[string][]string{}
	for _, it := range rep.Items {
		if it.Class != gcClassStrays && it.Class != gcClassBinaries {
			continue
		}
		if it.Action != "report" || it.Done || it.Error != "" {
			t.Errorf("%s %s: action %q done %v error %q; want report, never applied", it.Class, it.Path, it.Action, it.Done, it.Error)
		}
		rel, err := filepath.Rel(repo, it.Path)
		if err != nil {
			t.Fatal(err)
		}
		got[it.Class] = append(got[it.Class], filepath.ToSlash(rel))
		if it.Reason == "" {
			t.Errorf("%s %s has no reason", it.Class, rel)
		}
	}
	for c := range got {
		sort.Strings(got[c])
	}
	wantStrays := []string{"fixture.elf", "notes.txt", "scratch"}
	wantBins := []string{"fathom", "tools/blob"}
	if strings.Join(got[gcClassStrays], ",") != strings.Join(wantStrays, ",") {
		t.Errorf("strays = %v, want %v (workspace/, the cache root, ignored and tracked files are not strays; a binary is reported once, as a binary)", got[gcClassStrays], wantStrays)
	}
	if strings.Join(got[gcClassBinaries], ",") != strings.Join(wantBins, ",") {
		t.Errorf("binaries = %v, want %v (a script and a non-executable ELF are not)", got[gcClassBinaries], wantBins)
	}
	if _, ok := rep.Reclaimable[gcClassStrays]; ok {
		t.Errorf("strays counted as reclaimable: %v", rep.Reclaimable)
	}
	if _, ok := rep.Reclaimable[gcClassBinaries]; ok {
		t.Errorf("binaries counted as reclaimable: %v", rep.Reclaimable)
	}

	// Without a repository (the daemon's daily pass, doctor) the classes are
	// not planned at all.
	bare := runGC(gcOptions{Home: home, TmpDir: tmp})
	for _, it := range bare.Items {
		if it.Class == gcClassStrays || it.Class == gcClassBinaries {
			t.Fatalf("strays planned without a repository: %+v", it)
		}
	}

	// The verb reaches the classes: a dry run over the fixture repository,
	// with the cache root declared the way ADR-039 declares it.
	t.Setenv("FATHOM_CACHE_ROOT", cacheRoot)
	var out, errb bytes.Buffer
	code := cmdGC([]string{"--json", "--repo", repo, "--workspace", filepath.Join(repo, "workspace"), "--call-log", filepath.Join(tmp, "calls.jsonl"), "--url", "http://127.0.0.1:1"}, &out, &errb)
	if code != 0 && code != 1 {
		t.Fatalf("gc exit %d: %s", code, errb.String())
	}
	var vrep gcReport
	if err := json.Unmarshal(out.Bytes(), &vrep); err != nil {
		t.Fatalf("gc --json: %v\n%s", err, out.String())
	}
	n := 0
	for _, it := range vrep.Items {
		if it.Class == gcClassStrays || it.Class == gcClassBinaries {
			n++
		}
	}
	if n != len(wantStrays)+len(wantBins) {
		t.Errorf("fathom gc --repo reported %d strays and binaries, want %d:\n%s", n, len(wantStrays)+len(wantBins), out.String())
	}
	if gcTreeDigest(t, repo) != before {
		t.Fatal("fathom gc changed the repository tree")
	}
}
