package main

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/landing"
	"github.com/geoffmilleraz/fathom/internal/store"
)

func culpritRepository(t *testing.T) (string, []string) {
	t.Helper()
	root := t.TempDir()
	git := func(args ...string) string {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = root
		cmd.Env = append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL="+os.DevNull)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v: %s", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	git("init", "-q")
	git("config", "user.name", "Test Author")
	git("config", "user.email", "test@example.invalid")
	git("config", "commit.gpgsign", "false")
	var hashes []string
	for _, file := range []string{"base.txt", "leaf.go", "caller.go", "unsupported.xyz"} {
		if err := os.WriteFile(filepath.Join(root, file), []byte("fixture\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		git("add", "--", file)
		git("-c", "core.hooksPath="+os.DevNull, "commit", "-qm", "fixture "+file)
		hashes = append(hashes, git("rev-parse", "HEAD"))
	}
	return root, hashes
}

func TestCulpritRanksRangeAndPrintsBisect(t *testing.T) {
	root, hashes := culpritRepository(t)
	var calls int
	source := func(_ context.Context, service, path string) (landing.Evidence, error) {
		calls++
		if service != "svc" || path != "unused.db" {
			t.Fatalf("source service/store = %q, %q", service, path)
		}
		return landing.Evidence{Nodes: []core.SymbolNode{
			{Service: "svc", Package: "pkg", Name: "Leaf", FilePath: "leaf.go"},
			{Service: "svc", Package: "pkg", Name: "Caller", FilePath: "caller.go", Calls: []string{"Leaf"}},
			{Service: "svc", Package: "pkg", Name: "TestRed", FilePath: "red_test.go", Calls: []string{"Caller"}},
		}}, nil
	}
	var out, stderr bytes.Buffer
	code := runCulprit([]string{hashes[0] + ".." + hashes[3], "--failing", "TestRed,TestOther", "--service", "svc", "--store", "unused.db"}, root, source, &out, &stderr)
	if code != 0 || calls != 1 {
		t.Fatalf("code=%d calls=%d stderr=%s", code, calls, &stderr)
	}
	text := out.String()
	first, second, third := strings.Index(text, "1. "+hashes[1]), strings.Index(text, "2. "+hashes[2]), strings.Index(text, "3. "+hashes[3])
	if first < 0 || second < first || third < second || !strings.Contains(text[third:], "no reach evidence") {
		t.Fatalf("ranked output:\n%s", text)
	}
	for _, want := range []string{"git bisect start " + hashes[3] + " " + hashes[0], "git checkout " + hashes[1], "git bisect good", "git bisect bad", "current index", "TestRed", "unsupported.xyz"} {
		if !strings.Contains(text, want) {
			t.Errorf("missing %q in:\n%s", want, text)
		}
	}
	if strings.Count(text, "TestOther") != 1 || !strings.Contains(text, "Unresolved failing tests: TestOther") {
		t.Errorf("unresolved test must be named once: %s", text)
	}
	if !strings.Contains(text, "git checkout "+hashes[1]+"^") {
		t.Errorf("missing parent confirmation: %s", text)
	}
	cmd := exec.Command("git", "rev-parse", "HEAD")
	cmd.Dir = root
	head, err := cmd.Output()
	if err != nil || strings.TrimSpace(string(head)) != hashes[3] {
		t.Fatalf("command changed HEAD: %q, %v", head, err)
	}
	if _, err := os.Stat(filepath.Join(root, ".git", "BISECT_START")); !os.IsNotExist(err) {
		t.Fatalf("command started bisect: %v", err)
	}
}

func culpritGit(t *testing.T, root string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = root
	cmd.Env = append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL="+os.DevNull,
		"GIT_AUTHOR_DATE=2026-01-01T00:00:00Z", "GIT_COMMITTER_DATE=2026-01-01T00:00:00Z")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v: %s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

func culpritWriteCommit(t *testing.T, root, file, body string) string {
	t.Helper()
	if err := os.WriteFile(filepath.Join(root, file), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	culpritGit(t, root, "add", "--", file)
	culpritGit(t, root, "-c", "core.hooksPath="+os.DevNull, "commit", "-qm", "edit "+file)
	return culpritGit(t, root, "rev-parse", "HEAD")
}

func TestCulpritMergeResolutionOnly(t *testing.T) {
	for _, conflict := range []bool{false, true} {
		t.Run(map[bool]string{false: "clean", true: "resolution"}[conflict], func(t *testing.T) {
			root, hashes := culpritRepository(t)
			culpritGit(t, root, "checkout", "-qb", "side", hashes[0])
			file := "side.go"
			if conflict {
				file = "base.txt"
			}
			culpritWriteCommit(t, root, file, "side\n")
			culpritGit(t, root, "checkout", "--detach", hashes[3])
			if conflict {
				culpritWriteCommit(t, root, file, "main\n")
				cmd := exec.Command("git", "-c", "core.hooksPath="+os.DevNull, "merge", "--no-ff", "-m", "merge side", "side")
				cmd.Dir = root
				if out, err := cmd.CombinedOutput(); err == nil || !strings.Contains(string(out), "CONFLICT") {
					t.Fatalf("expected merge conflict: %v: %s", err, out)
				}
				culpritWriteCommit(t, root, file, "resolved\n")
			} else {
				culpritGit(t, root, "-c", "core.hooksPath="+os.DevNull, "merge", "--no-ff", "-m", "merge side", "side")
			}
			bad := culpritGit(t, root, "rev-parse", "HEAD")
			git := func(args ...string) ([]byte, error) { return []byte(culpritGit(t, root, args...)), nil }
			commits, err := culpritCommits(git, hashes[0], bad)
			if err != nil {
				t.Fatal(err)
			}
			merge := commits[len(commits)-1]
			var want []string
			if conflict {
				want = []string{file}
			}
			if !reflect.DeepEqual(merge.Files, want) {
				t.Errorf("merge must include only its own resolution: got %v, want %v", merge.Files, want)
			}
			source := func(context.Context, string, string) (landing.Evidence, error) {
				return landing.Evidence{Nodes: []core.SymbolNode{
					{Service: "svc", Name: "Leaf", FilePath: "leaf.go"},
					{Service: "svc", Name: "TestRed", FilePath: "red_test.go", Calls: []string{"Leaf"}},
				}}, nil
			}
			var out, stderr bytes.Buffer
			if code := runCulprit([]string{hashes[0] + ".." + bad, "--service", "svc", "--failing", "TestRed"}, root, source, &out, &stderr); code != 0 {
				t.Fatalf("code=%d stderr=%s", code, &stderr)
			}
			if !strings.Contains(out.String(), bad+" [merge]") || !strings.Contains(out.String(), "1. "+hashes[1]) {
				t.Errorf("merge inherited reach or lacks label: %s", &out)
			}
		})
	}
}

func TestCulpritStoreProvenanceAndUnresolved(t *testing.T) {
	withIsolatedConfigHome(t)
	root, hashes := culpritRepository(t)
	culpritGit(t, root, "checkout", "--detach", hashes[0])
	other := culpritWriteCommit(t, root, "branch.go", "branch\n")
	for _, tc := range []struct {
		name, indexed, relation string
		empty                   bool
	}{
		{"same", hashes[3], "same as bad", false},
		{"behind", hashes[1], "behind bad by 2", false},
		{"unrelated", other, "not an ancestor of bad", false},
		{"unavailable-commit", strings.Repeat("f", 40), "relation unknown", false},
		{"missing-metadata", "", "indexed commit unknown", false},
		{"empty", hashes[3], "same as bad", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "index.sqlite")
			st, err := store.NewSQLiteStore(path)
			if err != nil {
				t.Fatal(err)
			}
			defer st.Close()
			ctx := context.Background()
			if tc.indexed != "" {
				if err := st.SetServiceMeta(ctx, store.ServiceMeta{ServiceID: "svc", IndexedCommit: tc.indexed, IndexedAt: 1}); err != nil {
					t.Fatal(err)
				}
			}
			if !tc.empty {
				if err := st.PutObservations(ctx, []core.Observation{{ServiceID: "svc", Package: "pkg", Symbol: "TestRed", SourceRef: "leaf.go", SourceKind: core.KindGoFunc}}); err != nil {
					t.Fatal(err)
				}
			}
			writeUserConfig(t, "[daemon]\nstore = "+tomlQuote(path)+"\n")
			var out, stderr bytes.Buffer
			if code := runCulprit([]string{hashes[0] + ".." + hashes[3], "--service", "svc", "--failing", "TestMissing,TestMissing"}, root, loadCulpritEvidence, &out, &stderr); code != 0 {
				t.Fatalf("code=%d stderr=%s", code, &stderr)
			}
			text := out.String()
			for _, want := range []string{path, tc.relation, "Unresolved failing tests: TestMissing", "git checkout " + hashes[3] + "^"} {
				if !strings.Contains(text, want) {
					t.Errorf("missing %q: %s stderr=%s", want, text, &stderr)
				}
			}
			if tc.indexed != "" && !strings.Contains(text, "indexed commit "+tc.indexed) {
				t.Errorf("missing indexed commit: %s", text)
			}
			if strings.Count(text, "TestMissing") != 1 || !strings.Contains(text, "Top candidate is bad itself") {
				t.Errorf("unresolved/top-bad output: %s", text)
			}
			if tc.empty && strings.Count(text, "No index evidence exists for service") != 1 {
				t.Errorf("empty index must have one top-level notice: %s", text)
			}
			if !strings.Contains(text, "blast radius >=0") {
				t.Errorf("missing files must print lower-bound radius: %s", text)
			}
		})
	}
}

func TestCulpritUnavailableEvidenceAndValidation(t *testing.T) {
	root, hashes := culpritRepository(t)
	var out, stderr bytes.Buffer
	source := func(context.Context, string, string) (landing.Evidence, error) {
		return landing.Evidence{}, errors.New("index unavailable")
	}
	if code := runCulprit([]string{"--service", "svc", hashes[0] + ".." + hashes[3]}, root, source, &out, &stderr); code != 0 {
		t.Fatalf("code=%d stderr=%s", code, &stderr)
	}
	if strings.Count(out.String(), "no reach evidence") < 3 || !strings.Contains(stderr.String(), "index unavailable") || !strings.Contains(out.String(), "1. "+hashes[3]) {
		t.Fatalf("missing-evidence output: %s stderr=%s", &out, &stderr)
	}
	for _, bad := range []string{"", "HEAD", "HEAD...HEAD", "--all..HEAD", "HEAD..missing"} {
		out.Reset()
		stderr.Reset()
		if code := runCulprit([]string{bad}, root, source, &out, &stderr); code == 0 || out.Len() != 0 {
			t.Errorf("bad range %q: code=%d stdout=%s stderr=%s", bad, code, &out, &stderr)
		}
	}
	v, ok := lookupVerb("culprit")
	if !ok || reflect.TypeOf(v.Run) != reflect.TypeOf(culpritRunner{}) {
		t.Fatalf("culprit verb not registered: %+v", v)
	}
}

func TestCulpritMissingStoreAndEmptyRange(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing.sqlite")
	if _, err := loadCulpritEvidence(context.Background(), "svc", path); err == nil {
		t.Fatal("missing store must be unavailable evidence")
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("reader created a store: %v", err)
	}
	root, hashes := culpritRepository(t)
	source := func(context.Context, string, string) (landing.Evidence, error) {
		t.Fatal("empty range/help must not read the index")
		return landing.Evidence{}, nil
	}
	var out, stderr bytes.Buffer
	if code := runCulprit([]string{hashes[0] + ".." + hashes[0]}, root, source, &out, &stderr); code != 0 || !strings.Contains(out.String(), "No commits") {
		t.Fatalf("empty range: code=%d stdout=%s stderr=%s", code, &out, &stderr)
	}
	out.Reset()
	stderr.Reset()
	runCulprit([]string{"-h"}, "not-a-repository", source, &out, &stderr)
	if !strings.Contains(stderr.String(), "Usage: fathom culprit") || out.Len() != 0 {
		t.Fatalf("help did not stop before git/index reads: stdout=%s stderr=%s", &out, &stderr)
	}
}
