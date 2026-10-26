package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

var testLimits = limits{MaxFiles: 16, MaxFileBytes: 1024, MaxTotalBytes: 4096}

func fixtureGit(t *testing.T, dir string, extra []string, args ...string) string {
	t.Helper()
	cmd := fixtureGitCommand(dir, extra, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("fixture git %v failed: %v: %s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

func fixtureGitCommand(dir string, extra []string, args ...string) *exec.Cmd {
	cmd := exec.Command("git", append([]string{"-C", dir, "-c", "core.hooksPath=/dev/null", "-c", "commit.gpgsign=false"}, args...)...)
	env := make([]string, 0, len(os.Environ())+len(extra)+2)
	for _, e := range os.Environ() {
		if !strings.HasPrefix(strings.ToUpper(strings.SplitN(e, "=", 2)[0]), "GIT_") {
			env = append(env, e)
		}
	}
	cmd.Env = append(env, extra...)
	return cmd
}

func fixtureRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	fixtureGit(t, dir, nil, "init", "-q")
	fixtureGit(t, dir, nil, "config", "user.name", "Test")
	fixtureGit(t, dir, nil, "config", "user.email", "test@example.invalid")
	return dir
}

func fixtureWrite(t *testing.T, dir, name, data string) {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(data), 0644); err != nil {
		t.Fatal(err)
	}
}

func fixtureCommit(t *testing.T, dir string) string {
	t.Helper()
	fixtureGit(t, dir, nil, "add", "-A")
	fixtureGit(t, dir, nil, "commit", "-qm", "fixture")
	return fixtureGit(t, dir, nil, "rev-parse", "HEAD")
}

func TestStagedSnapshotsObjectsAndDrift(t *testing.T) {
	dir := fixtureRepo(t)
	fixtureWrite(t, dir, "a.txt", "old")
	base := fixtureCommit(t, dir)
	fixtureWrite(t, dir, "a.txt", "staged")
	fixtureWrite(t, dir, "space name.txt", "space")
	fixtureWrite(t, dir, "line\nbreak.txt", "newline")
	fixtureGit(t, dir, nil, "add", "--", "a.txt", "space name.txt", "line\nbreak.txt")
	fixtureWrite(t, dir, "a.txt", "unstaged")
	s, err := (gitSource{Dir: dir}).staged(context.Background(), testLimits)
	if err != nil {
		t.Fatal(err)
	}
	if len(s.Changes) != 3 {
		t.Fatalf("changes = %d, want 3", len(s.Changes))
	}
	got := map[string]change{}
	for _, c := range s.Changes {
		got[c.Path] = c
	}
	if string(got["a.txt"].Before) != "old" || string(got["a.txt"].After) != "staged" ||
		string(got["space name.txt"].After) != "space" || string(got["line\nbreak.txt"].After) != "newline" {
		t.Fatalf("captured Git blobs differ from index: %+v", got)
	}
	if err := (gitSource{Dir: dir}).unchanged(context.Background(), s, true); err != nil {
		t.Fatalf("unstaged edit changed index snapshot: %v", err)
	}
	fixtureWrite(t, dir, "a.txt", "later")
	fixtureGit(t, dir, nil, "add", "a.txt")
	if err := (gitSource{Dir: dir}).unchanged(context.Background(), s, true); err == nil {
		t.Fatal("index drift accepted")
	}
	r, err := (gitSource{Dir: dir}).revision(context.Background(), base, base, testLimits)
	if err != nil || len(r.Changes) != 0 {
		t.Fatalf("same revision: %v %+v", err, r)
	}
}

func TestRevisionFullNewRefDeleteAndRefMove(t *testing.T) {
	dir := fixtureRepo(t)
	fixtureWrite(t, dir, "kept", "before")
	fixtureWrite(t, dir, "gone", "remove")
	base := fixtureCommit(t, dir)
	if err := os.Remove(filepath.Join(dir, "gone")); err != nil {
		t.Fatal(err)
	}
	fixtureWrite(t, dir, "kept", "after")
	rev := fixtureCommit(t, dir)
	g := gitSource{Dir: dir}
	s, err := g.revision(context.Background(), base, "HEAD", testLimits)
	if err != nil {
		t.Fatal(err)
	}
	if s.Revision != rev || s.Ref != "HEAD" || len(s.Changes) != 2 {
		t.Fatalf("revision snapshot = %+v", s)
	}
	byPath := map[string]change{}
	for _, c := range s.Changes {
		byPath[c.Path] = c
	}
	if string(byPath["gone"].Before) != "remove" || byPath["gone"].AfterOID != "" ||
		string(byPath["kept"].After) != "after" {
		t.Fatalf("bad change capture: %+v", byPath)
	}
	if err := g.unchanged(context.Background(), s, false); err != nil {
		t.Fatal(err)
	}
	full, err := g.revision(context.Background(), "", rev, testLimits)
	if err != nil || len(full.Changes) != 1 || full.Ref != "" {
		t.Fatalf("full new ref snapshot: %v %+v", err, full)
	}
	fixtureWrite(t, dir, "new", "move")
	fixtureCommit(t, dir)
	if err := g.unchanged(context.Background(), s, false); err == nil {
		t.Fatal("moved ref accepted")
	}
	if err := g.unchanged(context.Background(), full, false); err != nil {
		t.Fatalf("immutable OID rejected: %v", err)
	}
	if _, err := g.revision(context.Background(), "absent-ref", "HEAD", testLimits); err == nil {
		t.Fatal("missing base accepted")
	}
}

func TestUnbornIndexAndAlternateIndex(t *testing.T) {
	dir := fixtureRepo(t)
	fixtureWrite(t, dir, "a", "default")
	fixtureGit(t, dir, nil, "add", "a")
	g := gitSource{Dir: dir}
	s, err := g.staged(context.Background(), testLimits)
	if err != nil || len(s.Changes) != 1 || string(s.Changes[0].After) != "default" {
		t.Fatalf("unborn index: %v %+v", err, s)
	}
	alt := "alternate-index"
	fixtureGit(t, dir, []string{"GIT_INDEX_FILE=" + filepath.Join(dir, alt)}, "read-tree", "--empty")
	fixtureWrite(t, dir, "b", "alternate")
	fixtureGit(t, dir, []string{"GIT_INDEX_FILE=" + filepath.Join(dir, alt)}, "add", "b")
	gs := gitSource{Dir: dir, IndexFile: alt}
	sa, err := gs.staged(context.Background(), testLimits)
	if err != nil || len(sa.Changes) != 1 || sa.Changes[0].Path != "b" {
		t.Fatalf("alternate index: %v %+v", err, sa)
	}
	if err := gs.unchanged(context.Background(), sa, true); err != nil {
		t.Fatal(err)
	}
}

func TestGitEnvironmentAndLinkedWorktree(t *testing.T) {
	dir := fixtureRepo(t)
	fixtureWrite(t, dir, "tracked", "safe")
	fixtureCommit(t, dir)
	linked := filepath.Join(t.TempDir(), "linked")
	fixtureGit(t, dir, nil, "worktree", "add", "-qb", "test-linked", linked)
	fixtureWrite(t, linked, "tracked", "linked")
	fixtureGit(t, linked, nil, "add", "tracked")
	other := fixtureRepo(t)
	t.Setenv("GIT_DIR", filepath.Join(other, ".git"))
	t.Setenv("GIT_WORK_TREE", other)
	t.Setenv("GIT_INDEX_FILE", filepath.Join(other, "index"))
	t.Setenv("GIT_OBJECT_DIRECTORY", filepath.Join(other, "objects"))
	t.Setenv("GIT_ALTERNATE_OBJECT_DIRECTORIES", filepath.Join(other, "alt"))
	t.Setenv("GIT_CONFIG_COUNT", "1")
	t.Setenv("GIT_CONFIG_KEY_0", "core.fsmonitor")
	t.Setenv("GIT_CONFIG_VALUE_0", "/bin/false")
	s, err := (gitSource{Dir: linked}).staged(context.Background(), testLimits)
	if err != nil || len(s.Changes) != 1 || string(s.Changes[0].After) != "linked" {
		t.Fatalf("poisoned environment affected linked worktree: %v %+v", err, s)
	}
}

func TestPolicyBlobRegularAndBounded(t *testing.T) {
	dir := fixtureRepo(t)
	fixtureWrite(t, dir, "policy.json", "{}")
	if err := os.Symlink("outside", filepath.Join(dir, "link")); err != nil {
		t.Fatal(err)
	}
	commit := fixtureCommit(t, dir)
	g := gitSource{Dir: dir}
	data, gotCommit, err := g.policyBlob(context.Background(), "HEAD", "policy.json")
	if err != nil || string(data) != "{}" || gotCommit != commit {
		t.Fatalf("policy blob: %v %q %s", err, data, gotCommit)
	}
	if _, _, err := g.policyBlob(context.Background(), "HEAD", "link"); err == nil {
		t.Fatal("symlink accepted as policy")
	}
	if _, _, err := g.policyBlob(context.Background(), "HEAD", "../outside"); err == nil {
		t.Fatal("parent path accepted")
	}
	fixtureWrite(t, dir, "empty-policy.json", "")
	fixtureCommit(t, dir)
	empty, _, err := g.policyBlob(context.Background(), "HEAD", "empty-policy.json")
	if err != nil || empty == nil || len(empty) != 0 {
		t.Fatalf("empty regular blob lost: %v %#v", err, empty)
	}
	fixtureWrite(t, dir, "big", strings.Repeat("x", policyBlobCap+1))
	fixtureCommit(t, dir)
	if _, _, err := g.policyBlob(context.Background(), "HEAD", "big"); err == nil {
		t.Fatal("oversized policy accepted")
	}
}

func TestSnapshotBoundsSymlinkAndUnmergedIndex(t *testing.T) {
	dir := fixtureRepo(t)
	fixtureWrite(t, dir, "a", "old")
	fixtureCommit(t, dir)
	if err := os.Symlink("../outside", filepath.Join(dir, "link")); err != nil {
		t.Fatal(err)
	}
	fixtureWrite(t, dir, "a", strings.Repeat("x", 20))
	fixtureGit(t, dir, nil, "add", "a", "link")
	g := gitSource{Dir: dir}
	s, err := g.staged(context.Background(), testLimits)
	if err != nil || len(s.Changes) != 2 {
		t.Fatalf("symlink snapshot: %v %+v", err, s)
	}
	for _, c := range s.Changes {
		if c.Path == "link" && (c.AfterMode != "120000" || string(c.After) != "../outside") {
			t.Fatalf("symlink followed or lost: %+v", c)
		}
	}
	tooSmall := testLimits
	tooSmall.MaxFileBytes = 10
	if _, err := g.staged(context.Background(), tooSmall); err == nil {
		t.Fatal("oversized blob accepted")
	}
	tooSmall = testLimits
	tooSmall.MaxFiles = 1
	if _, err := g.staged(context.Background(), tooSmall); err == nil {
		t.Fatal("excess files accepted")
	}
	tooSmall = testLimits
	tooSmall.MaxTotalBytes = 25
	if _, err := g.staged(context.Background(), tooSmall); err == nil {
		t.Fatal("aggregate blob budget exceeded")
	}
	baseOID := fixtureGit(t, dir, nil, "rev-parse", "HEAD:a")
	t.Run("foreign-context-isolated", func(t *testing.T) {
		// Poison only a disposable foreign repository, never the caller's real index.
		foreign := fixtureRepo(t)
		fixtureWrite(t, foreign, "sentinel", "must remain unchanged")
		fixtureCommit(t, foreign)
		foreignIndex := filepath.Join(foreign, ".git", "index")
		before, err := os.ReadFile(foreignIndex)
		if err != nil {
			t.Fatal(err)
		}
		t.Setenv("GIT_DIR", filepath.Join(foreign, ".git"))
		cmd := fixtureGitCommand(dir, nil, "update-index", "--index-info")
		cmd.Stdin = strings.NewReader(fmt.Sprintf("0 %s\ta\n100644 %s 2\ta\n100644 %s 3\ta\n", strings.Repeat("0", len(baseOID)), baseOID, baseOID))
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("create unmerged index: %v %s", err, out)
		}
		if after, err := os.ReadFile(foreignIndex); err != nil || !bytes.Equal(before, after) {
			t.Fatalf("fixture command changed the foreign index: %v", err)
		}
	})
	// The poison is restored before this control check. The earlier successful
	// staged snapshot differs only by the fixture's independently observed conflict.
	unmerged := fixtureGit(t, dir, nil, "ls-files", "--unmerged")
	if lines := strings.Split(unmerged, "\n"); len(lines) != 2 || !strings.Contains(lines[0], " 2\ta") || !strings.Contains(lines[1], " 3\ta") {
		t.Fatalf("fixture did not get the intended conflict: %q", unmerged)
	}
	if _, err := g.staged(context.Background(), testLimits); err == nil {
		t.Fatal("unmerged index accepted")
	}
}

func TestGitlinkCapturesModeWithoutReadingSubmodule(t *testing.T) {
	dir := fixtureRepo(t)
	fixtureWrite(t, dir, "root", "ok")
	commit := fixtureCommit(t, dir)
	fixtureGit(t, dir, nil, "update-index", "--add", "--cacheinfo", "160000,"+commit+",submodule")
	s, err := (gitSource{Dir: dir}).staged(context.Background(), testLimits)
	if err != nil || len(s.Changes) != 1 {
		t.Fatalf("gitlink snapshot: %v %+v", err, s)
	}
	c := s.Changes[0]
	if c.Path != "submodule" || c.AfterMode != "160000" || c.AfterOID != commit || c.After != nil {
		t.Fatalf("gitlink read as file: %+v", c)
	}
}

func TestParsePushUpdates(t *testing.T) {
	forty := strings.Repeat("a", 40)
	zeros := strings.Repeat("0", 40)
	valid := fmt.Sprintf("refs/heads/main %s refs/heads/main %s\nrefs/heads/new %s refs/heads/new %s\n(delete) %s refs/heads/old %s\n", forty, forty, forty, zeros, zeros, forty)
	updates, err := parsePushUpdates(strings.NewReader(valid))
	if err != nil || len(updates) != 3 {
		t.Fatalf("valid updates: %v %+v", err, updates)
	}
	for _, size := range []int{40, 64} {
		oid := strings.Repeat("a", size)
		for _, localRef := range []string{oid, strings.ToUpper(oid)} {
			input := fmt.Sprintf("%s %s refs/heads/main %s\n", localRef, oid, strings.Repeat("0", size))
			if got, err := parsePushUpdates(strings.NewReader(input)); err != nil || len(got) != 1 {
				t.Fatalf("raw OID push row rejected: %v %+v", err, got)
			}
		}
		input := fmt.Sprintf("%s %s refs/heads/main %s\n", strings.Repeat("b", size), oid, strings.Repeat("0", size))
		if _, err := parsePushUpdates(strings.NewReader(input)); err == nil {
			t.Fatal("mismatched raw OID accepted")
		}
	}
	for _, input := range []string{
		"refs/heads/main -x refs/heads/main " + forty + "\n",
		"refs/heads/main " + forty + " -bad " + forty + "\n",
		"refs/heads/main " + forty + " refs/heads/a..b " + forty + "\n",
		"refs/heads/main " + forty + " refs/heads/main " + forty,
		strings.Repeat(valid, 11),
		strings.Repeat("x", pushInputCap+1),
	} {
		if _, err := parsePushUpdates(strings.NewReader(input)); err == nil {
			t.Fatalf("malformed input accepted: %.80q", input)
		}
	}
}

func TestGitExecutableCannotComeFromCandidate(t *testing.T) {
	dir := fixtureRepo(t)
	trusted, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	trusted, err = filepath.Abs(trusted)
	if err != nil {
		t.Fatal(err)
	}
	bin := filepath.Join(dir, "bin")
	if err := os.MkdirAll(bin, 0755); err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(dir, "executed")
	shim := filepath.Join(bin, "git")
	if err := os.WriteFile(shim, []byte("#!/bin/sh\ntouch \""+marker+"\"\n"), 0755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	nested := filepath.Join(dir, "nested")
	if err := os.Mkdir(nested, 0755); err != nil {
		t.Fatal(err)
	}
	alias := filepath.Join(t.TempDir(), "alias")
	if err := os.Symlink(nested, alias); err != nil {
		t.Fatal(err)
	}
	for _, repo := range []string{dir, nested, alias} {
		if _, err := (gitSource{Dir: repo}).git(context.Background(), 128, nil, "rev-parse", "--show-toplevel"); err == nil {
			t.Fatalf("candidate Git executable accepted for %s", repo)
		}
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("candidate Git executable ran: %v", err)
	}
	if _, err := (gitSource{Dir: nested, GitBin: "git"}).git(context.Background(), 128, nil, "rev-parse", "--show-toplevel"); err == nil {
		t.Fatal("relative explicit Git executable accepted")
	}
	if _, err := (gitSource{Dir: nested, GitBin: shim}).git(context.Background(), 128, nil, "rev-parse", "--show-toplevel"); err == nil {
		t.Fatal("explicit candidate Git executable accepted")
	}
	shimAlias := filepath.Join(t.TempDir(), "git")
	if err := os.Symlink(shim, shimAlias); err != nil {
		t.Fatal(err)
	}
	if _, err := (gitSource{Dir: nested, GitBin: shimAlias}).git(context.Background(), 128, nil, "rev-parse", "--show-toplevel"); err == nil {
		t.Fatal("symlink to candidate Git executable accepted")
	}
	out, err := (gitSource{Dir: nested, GitBin: trusted}).git(context.Background(), 128, nil, "rev-parse", "--show-toplevel")
	canonicalDir, canonicalErr := filepath.EvalSymlinks(dir)
	if err != nil || canonicalErr != nil || strings.TrimSpace(string(out)) != canonicalDir {
		t.Fatalf("trusted absolute Git failed: %v %q", err, out)
	}
}

func TestPromisedBlobNeverFetched(t *testing.T) {
	source := fixtureRepo(t)
	fixtureWrite(t, source, "blob.txt", "promised content")
	fixtureCommit(t, source)
	fixtureGit(t, source, nil, "config", "uploadpack.allowFilter", "true")
	blobOID := fixtureGit(t, source, nil, "rev-parse", "HEAD:blob.txt")
	clone := filepath.Join(t.TempDir(), "partial")
	fixtureGit(t, source, nil, "clone", "--quiet", "--no-checkout", "--filter=blob:none", "file://"+source, clone)
	g := gitSource{Dir: clone}
	if _, err := g.git(context.Background(), 128, nil, "cat-file", "-e", blobOID); err == nil {
		t.Fatal("fixture blob was present; partial clone did not promise it")
	}
	if _, err := g.revision(context.Background(), "", "HEAD", testLimits); err == nil {
		t.Fatal("missing promised blob was accepted or fetched")
	}
	if _, err := g.git(context.Background(), 128, nil, "cat-file", "-e", blobOID); err == nil {
		t.Fatal("quality-gate fetched a promised blob")
	}
}

func TestSchemaRuntimeParityCalibratedAndCrossKind(t *testing.T) {
	if _, err := exec.LookPath("cue"); err != nil {
		t.Skip("CUE CLI not installed")
	}
	schema := "schema.cue"
	valid := validTestPolicy()
	valid.Model = modelPolicy{Provider: "openrouter", ModelEnv: "Q_MODEL", ExpectedModelEnv: "Q_EXPECTED", APIKeyEnv: "Q_KEY", MaxInputBytes: 8192, TimeoutSeconds: 5, Sites: []modelSite{}}
	site := modelSite{ID: "review", Version: "v1", Question: "Does this change raise a concern?", Boundaries: []string{"ci"}, Select: selector{Prefixes: []string{"src/"}}, Mode: "hold", MinConfidence: .7}
	site.Calibration = &calibration{Site: "repo/ci/review", QuestionSHA256: siteDigest(site), Provider: "typesafe-openrouter", ResolvedModel: "pinned", ValidUntil: time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC), MinPerClass: 1, MinN: 3, MinAccuracy: .8, MinWilsonLo: .5, Cases: []labeledCase{{ID: "case1", Expected: "clear", Actual: "clear", Confidence: .9, Provenance: "human"}}}
	valid.Model.Sites = append(valid.Model.Sites, site)
	if err := validatePolicy(valid); err != nil {
		t.Fatalf("runtime rejected calibrated fixture: %v", err)
	}
	vet := func(p policy) error {
		t.Helper()
		data, err := json.Marshal(p)
		if err != nil {
			return err
		}
		path := filepath.Join(t.TempDir(), "policy.json")
		if err := os.WriteFile(path, data, 0600); err != nil {
			return err
		}
		cmd := exec.Command("cue", "vet", "-d", "#Policy", schema, path)
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("%v: %s", err, out)
		}
		return nil
	}
	if err := vet(valid); err != nil {
		t.Fatalf("CUE rejected calibrated fixture: %v", err)
	}
	for _, mutate := range []struct {
		name string
		edit func(*policy)
	}{
		{"stray max bytes", func(p *policy) { p.Rules[0].MaxBytes = 10 }},
		{"stray companion", func(p *policy) { p.Rules[0].Companion = &selector{Prefixes: []string{"tests/"}} }},
		{"duplicate IDs", func(p *policy) { p.Model.Sites[0].ID = p.Rules[0].ID }},
		{"duplicate cases", func(p *policy) { c := p.Model.Sites[0].Calibration; c.Cases = append(c.Cases, c.Cases[0]) }},
		{"long prefix", func(p *policy) { p.Rules[0].Select.Prefixes = []string{strings.Repeat("x", 1025)} }},
		{"long suffix", func(p *policy) { p.Rules[0].Select.Suffixes = []string{strings.Repeat("x", 1025)} }},
		{"remote model env", func(p *policy) { p.Model.ModelEnv = "" }},
		{"calibration provider", func(p *policy) { p.Model.Sites[0].Calibration.Provider = "openrouter" }},
		{"no checks", func(p *policy) { p.Rules = []rule{}; p.Model.Sites = []modelSite{} }},
	} {
		t.Run(mutate.name, func(t *testing.T) {
			p, err := decodePolicy(encodedPolicy(t, valid))
			if err != nil {
				t.Fatal(err)
			}
			mutate.edit(&p)
			if err := validatePolicy(p); err == nil {
				t.Fatal("runtime accepted cross-kind option")
			}
			if err := vet(p); err == nil {
				t.Fatal("CUE accepted cross-kind option")
			}
		})
	}
}
