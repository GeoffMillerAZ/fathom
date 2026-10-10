package main

// edithook_ignored_test.go — EW-55: an edit to a gitignored file gets no
// rule injection and costs no daemon traffic, on the Post, Pre and
// Bash-write paths; tracked and negated files are served as before.

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
)

// ignoredFixture is a git repo whose .gitignore lists workspace/ and *.gen.go
// (with !keep.gen.go negated), plus a force-added file under workspace/.
func ignoredFixture(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	gitInitTempRepo(t, dir)
	write := func(rel, body string) {
		p := filepath.Join(dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write(".gitignore", "workspace/\n*.gen.go\n!keep.gen.go\n")
	write("pkg/a.go", "package pkg\n")
	write("workspace/x.go", "package ws\n")
	write("workspace/tracked.go", "package ws\n")
	write("pkg/zz.gen.go", "package pkg\n")
	write("keep.gen.go", "package main\n")
	cmd := exec.Command("git", "-C", dir, "add", "-f", "workspace/tracked.go")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git add -f: %v\n%s", err, out)
	}
	return dir
}

// rulesServer answers every obligations call with one rule and counts all
// requests.
func rulesServer(t *testing.T) (*httptest.Server, *int32) {
	t.Helper()
	return countingServer(t, func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Params struct {
				Name      string `json:"name"`
				Arguments struct {
					File string `json:"file"`
				} `json:"arguments"`
			} `json:"params"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		inner := `{}`
		switch req.Params.Name {
		case "list_services":
			inner = `{"services":[]}`
		case "obligations":
			b, _ := json.Marshal(map[string]any{"service": "svc", "file": req.Params.Arguments.File, "rules": []any{
				map[string]any{"rule": "Keep the registry sorted.", "src": "CLAUDE.md#L1-L1", "tier": "doc-claimed", "layer": "obligations"},
			}})
			inner = string(b)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": 1, "result": map[string]any{"content": []any{map[string]any{"type": "text", "text": inner}}}})
	})
}

func TestRenderPostEdit_GitignoredPathIsSilentWithZeroDaemonRequests(t *testing.T) {
	dir := ignoredFixture(t)
	srv, calls := rulesServer(t)
	defer srv.Close()
	for _, rel := range []string{"workspace/x.go", "pkg/zz.gen.go", "workspace/new/deep.go"} {
		text, _, ok := renderPostEdit(srv.URL, dir, filepath.Join(dir, filepath.FromSlash(rel)), "s-ign", "")
		if ok || text != "" {
			t.Errorf("%s: injected %q, want silence", rel, text)
		}
	}
	if n := atomic.LoadInt32(calls); n != 0 {
		t.Fatalf("daemon got %d request(s) for ignored paths, want 0", n)
	}
}

func TestRenderPostEdit_TrackedAndNegatedPathsStillGetRules(t *testing.T) {
	dir := ignoredFixture(t)
	for _, rel := range []string{"pkg/a.go", "keep.gen.go", "workspace/tracked.go"} {
		srv, calls := rulesServer(t)
		text, _, ok := renderPostEdit(srv.URL, dir, filepath.Join(dir, filepath.FromSlash(rel)), "s-trk", "")
		srv.Close()
		if !ok || !strings.Contains(text, "Keep the registry sorted.") {
			t.Errorf("%s: ok=%v text=%q, want its rule", rel, ok, text)
		}
		if atomic.LoadInt32(calls) == 0 {
			t.Errorf("%s: no daemon request", rel)
		}
	}
}

func TestRenderPreEdit_GitignoredPathIsSilentWithZeroDaemonRequests(t *testing.T) {
	orig := guardStampDir
	guardStampDir = filepath.Join(t.TempDir(), "stamps")
	t.Cleanup(func() { guardStampDir = orig })
	dir := ignoredFixture(t)
	writeGuardCue(t, dir, `guards: [{scopes: ["**"], note: "everything is guarded"}]`)
	srv, calls := rulesServer(t)
	defer srv.Close()
	if _, ok := renderPreEdit(srv.URL, dir, filepath.Join(dir, "workspace", "x.go"), "s-pre"); ok {
		t.Error("ignored path rendered a guard")
	}
	if n := atomic.LoadInt32(calls); n != 0 {
		t.Fatalf("pre-edit sent %d request(s) for an ignored path, want 0", n)
	}
	// Control: the guard does fire (and asks the daemon) for a tracked file.
	renderPreEdit(srv.URL, dir, filepath.Join(dir, "pkg", "a.go"), "s-pre")
	if atomic.LoadInt32(calls) == 0 {
		t.Fatal("control: a tracked guarded file never reached the daemon")
	}
}

func TestEditHookBashWrite_GitignoredTargetIsSilent(t *testing.T) {
	dir := ignoredFixture(t)
	srv, calls := rulesServer(t)
	defer srv.Close()
	got := runBashWriteHook(t, srv.URL, "s-bash", dir, "cp /tmp/draft.go workspace/x.go")
	if got != "" {
		t.Fatalf("ignored write target injected %q", got)
	}
	// Control: a tracked target is served.
	got = runBashWriteHook(t, srv.URL, "s-bash2", dir, "cp /tmp/draft.go pkg/a.go")
	if !strings.Contains(got, "Keep the registry sorted.") {
		t.Fatalf("tracked target not served: %q", got)
	}
	_ = calls
}

func TestGitIgnoredMatcher(t *testing.T) {
	dir := t.TempDir()
	gitInitTempRepo(t, dir)
	files := map[string]string{
		".gitignore":        "/rootonly.txt\nbuild/\n*.log\n!important.log\ndocs/**/tmp\n# comment\n\\#hash\n",
		"sub/.gitignore":    "local.txt\n!*.log\n",
		".git/info/exclude": "excluded-by-info.txt\n",
		"a/b/.gitignore":    "/anchored.txt\n",
	}
	for rel, body := range files {
		p := filepath.Join(dir, filepath.FromSlash(rel))
		_ = os.MkdirAll(filepath.Dir(p), 0o755)
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	cases := map[string]bool{
		"rootonly.txt":         true,
		"sub/rootonly.txt":     false,
		"build/out.o":          true,
		"x/build/out.o":        true,
		"x/build":              false, // a file named build: dir-only pattern
		"run.log":              true,
		"important.log":        false,
		"sub/run.log":          false, // re-included by sub/.gitignore
		"sub/local.txt":        true,
		"local.txt":            false,
		"docs/a/b/tmp/f":       true,
		"docs/tmp":             true,
		"excluded-by-info.txt": true,
		"a/b/anchored.txt":     true,
		"a/b/c/anchored.txt":   false,
		"#hash":                true,
		"pkg/a.go":             false,
	}
	for rel, want := range cases {
		if got := gitignoredInProcess(dir, rel); got != want {
			t.Errorf("gitignoredInProcess(%q) = %v, want %v", rel, got, want)
		}
	}
}

// BenchmarkGitIgnoreDecision reports both decision costs EW-55 weighed: the
// in-process matcher (what the hook runs on every edit) and the
// `git check-ignore -q` shell-out (which it runs only to confirm a positive).
func BenchmarkGitIgnoreDecision(b *testing.B) {
	dir := b.TempDir()
	if out, err := exec.Command("git", "init", "-q", dir).CombinedOutput(); err != nil {
		b.Fatalf("git init: %v\n%s", err, out)
	}
	_ = os.WriteFile(filepath.Join(dir, ".gitignore"), []byte("workspace/\n*.log\nref/\ntmp/\n"), 0o644)
	b.Run("in-process/ignored", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			if !gitignoredInProcess(dir, "workspace/x.go") {
				b.Fatal("not ignored")
			}
		}
	})
	b.Run("in-process/tracked", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			if gitignoredInProcess(dir, "internal/pkg/a.go") {
				b.Fatal("ignored")
			}
		}
	})
	b.Run("git-check-ignore", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			if err := exec.Command("git", "-C", dir, "check-ignore", "-q", "--", "workspace/x.go").Run(); err != nil {
				b.Fatal(fmt.Sprint(err))
			}
		}
	})
}

var _ = bytes.NewBuffer
