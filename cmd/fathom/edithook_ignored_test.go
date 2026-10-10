package main

// edithook_ignored_test.go — EW-55/EW-56: an edit to a gitignored file gets no
// rule injection and costs no daemon traffic, on the Post, Pre and
// Bash-write paths (on Pre only when no guard names the file); tracked and negated files are served as before.

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

func guardStampsIsolated(t *testing.T) {
	t.Helper()
	orig := guardStampDir
	guardStampDir = filepath.Join(t.TempDir(), "stamps")
	t.Cleanup(func() { guardStampDir = orig })
}

// R-b (owner ruling, 2026-10-10): an explicit guard scoped to a path is
// signal, so its note fires on --pre even when git ignores the file.
func TestRenderPreEdit_GuardedGitignoredPathStillPrints(t *testing.T) {
	guardStampsIsolated(t)
	dir := ignoredFixture(t)
	writeGuardCue(t, dir, `guards: [{scopes: ["workspace/**"], note: "workspace is scratch, keep it small"}]`)
	srv, calls := rulesServer(t)
	defer srv.Close()
	res, ok := renderPreEdit(srv.URL, dir, filepath.Join(dir, "workspace", "x.go"), "s-pre-g")
	if !ok || !strings.Contains(res.text, "workspace is scratch, keep it small") {
		t.Fatalf("guarded ignored path: ok=%v text=%q, want the guard note", ok, res.text)
	}
	if atomic.LoadInt32(calls) == 0 {
		t.Error("guarded ignored path never asked the daemon for rules")
	}
}

// The generic PostToolUse injection for the same guarded, ignored file stays silent.
func TestRenderPostEdit_GuardedGitignoredPathStaysSilent(t *testing.T) {
	dir := ignoredFixture(t)
	writeGuardCue(t, dir, `guards: [{scopes: ["workspace/**"], note: "workspace is scratch, keep it small"}]`)
	srv, calls := rulesServer(t)
	defer srv.Close()
	text, _, ok := renderPostEdit(srv.URL, dir, filepath.Join(dir, "workspace", "x.go"), "s-post-g", "")
	if ok || text != "" {
		t.Fatalf("post-edit injected %q for a guarded ignored path, want silence", text)
	}
	if n := atomic.LoadInt32(calls); n != 0 {
		t.Fatalf("post-edit sent %d request(s), want 0", n)
	}
}

// An ignored file no guard names stays silent with zero daemon requests.
func TestRenderPreEdit_GitignoredPathWithoutGuardIsSilentWithZeroDaemonRequests(t *testing.T) {
	guardStampsIsolated(t)
	dir := ignoredFixture(t)
	writeGuardCue(t, dir, `guards: [{scopes: ["pkg/**"], note: "pkg is guarded"}]`)
	srv, calls := rulesServer(t)
	defer srv.Close()
	if _, ok := renderPreEdit(srv.URL, dir, filepath.Join(dir, "workspace", "x.go"), "s-pre"); ok {
		t.Error("ignored path with no matching guard rendered")
	}
	if n := atomic.LoadInt32(calls); n != 0 {
		t.Fatalf("pre-edit sent %d request(s) for an unguarded ignored path, want 0", n)
	}
	// Control: the guard does fire (and asks the daemon) for a tracked file.
	renderPreEdit(srv.URL, dir, filepath.Join(dir, "pkg", "a.go"), "s-pre")
	if atomic.LoadInt32(calls) == 0 {
		t.Fatal("control: a tracked guarded file never reached the daemon")
	}
}

// A `**` guard matches an ignored file too: the ruling draws no line by glob shape.
func TestRenderPreEdit_StarStarGuardPrintsOnGitignoredPath(t *testing.T) {
	guardStampsIsolated(t)
	dir := ignoredFixture(t)
	writeGuardCue(t, dir, `guards: [{scopes: ["**"], note: "everything is guarded"}]`)
	srv, _ := rulesServer(t)
	defer srv.Close()
	res, ok := renderPreEdit(srv.URL, dir, filepath.Join(dir, "workspace", "x.go"), "s-pre-ss")
	if !ok || !strings.Contains(res.text, "everything is guarded") {
		t.Fatalf("** guard on ignored path: ok=%v text=%q", ok, res.text)
	}
}

// An ignored file is not in the index, so the daemon reads it index_stale;
// the per-file refresh (SU-6) and its stale line are skipped for it.
func TestRenderPreEdit_GuardedGitignoredPathSkipsRefresh(t *testing.T) {
	guardStampsIsolated(t)
	dir := ignoredFixture(t)
	writeGuardCue(t, dir, `guards: [{scopes: ["workspace/**"], note: "workspace is scratch"}]`)
	var refreshes int32
	mux := http.NewServeMux()
	mux.HandleFunc("/rpc", func(w http.ResponseWriter, r *http.Request) {
		if rpcRequestName(r) != "obligations" {
			_ = json.NewEncoder(w).Encode(rpcTextResult(map[string]any{"services": []any{}}))
			return
		}
		_ = json.NewEncoder(w).Encode(rpcTextResult(map[string]any{"service": "svc", "file": "workspace/x.go", "rules": []any{}, "index_stale": true}))
	})
	mux.HandleFunc("/refresh-file", func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&refreshes, 1)
		_ = json.NewEncoder(w).Encode(refreshFileResponse{Refreshed: true})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	res, ok := renderPreEdit(srv.URL, dir, filepath.Join(dir, "workspace", "x.go"), "s-pre-r")
	if !ok || !strings.Contains(res.text, "workspace is scratch") {
		t.Fatalf("ok=%v text=%q, want the guard note", ok, res.text)
	}
	if n := atomic.LoadInt32(&refreshes); n != 0 {
		t.Errorf("refresh-file called %d time(s) for an ignored file, want 0", n)
	}
	if strings.Contains(res.text, "index") {
		t.Errorf("stale line printed for an ignored file: %q", res.text)
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

// TestBashWriteAndEditShareOneGitignoreSkip is GL-25 (G3, 2026-10-10): a sed
// into a gitignored workspace/ file still got six ADR rules and "33 more".
// The Bash path ran EW-55's skip only on the targets it parsed, and resolved a
// relative target against the PROJECT root rather than the directory the
// command ran in: `sed -i … NOTES.md` from workspace/notes/ was asked about
// a root-level NOTES.md, which git does not ignore. Every spelling of a write
// into an ignored file is now one silence through editTargetIgnored, the
// predicate the Edit path takes, with no obligations request, and a tracked
// file written from its own directory is asked about by its real path.
func TestBashWriteAndEditShareOneGitignoreSkip(t *testing.T) {
	repo, tracked, ignored := deployedFixture(t)
	home := deployedHome(t)
	link := filepath.Join(t.TempDir(), "linked")
	if err := os.Symlink(repo, link); err != nil {
		t.Fatal(err)
	}
	srv, asked := deployedDaemon(t)
	env := deployedEnv(home, repo, srv.URL)
	hooks := deployedEditHooks(t)
	postBash, post := hooks["PostToolUse Bash"], hooks["PostToolUse Edit|Write|MultiEdit"]
	notesDir := filepath.Dir(ignored)

	cases := []struct{ name, cwd, command string }{
		{"relative from the ignored directory", notesDir, "sed -i '' 's/a/b/' NOTES.md"},
		{"absolute", repo, "sed -i '' 's/a/b/' " + ignored},
		{"through a symlink to the repository", repo, "sed -i '' 's/a/b/' " + filepath.Join(link, "workspace", "notes", "NOTES.md")},
		{"after a cd", repo, "cd workspace/notes && sed -i '' 's/a/b/' NOTES.md"},
		{"no parsed target, from the ignored directory", notesDir, `F=NOTES.md; sed -i "" "s/a/b/" $F`},
	}
	for _, c := range cases {
		if out := runDeployedHook(t, env, postBash, c.cwd, map[string]any{"command": c.command}, "Bash"); out != "" {
			t.Errorf("%s: a write into an ignored file was answered: %q", c.name, out)
		}
	}
	// The Edit path, for the same file: the same skip.
	if out := runDeployedHook(t, env, post, repo, map[string]any{"file_path": ignored, "old_string": "a", "new_string": "b"}, "Edit"); out != "" {
		t.Errorf("Edit of an ignored file was answered: %q", out)
	}
	if got := asked(); len(got) != 0 {
		t.Fatalf("obligations asked for %v; an ignored target costs no daemon request", got)
	}
	n := 0
	for _, b := range readDeployedBeats(t, filepath.Join(home, ".fathom", "hooks.jsonl")) {
		if b.Kind != "answered" {
			continue
		}
		if b.Meta["reason"] != silenceGitignored {
			t.Errorf("answer %v: reason %q, want %q (never foreign-target for an ignored file)", b.Meta, b.Meta["reason"], silenceGitignored)
		}
		n++
	}
	if n != len(cases)+1 {
		t.Errorf("%d answered beats, want %d", n, len(cases)+1)
	}

	// Control: a tracked file written from its own directory is asked about
	// by its repository path, not as a root-level file of the same name.
	out := runDeployedHook(t, env, postBash, filepath.Dir(tracked), map[string]any{"command": "sed -i '' 's/a/b/' widget.go"}, "Bash")
	if !strings.Contains(out, "Determinism is a contract") {
		t.Fatalf("a tracked write was not served: %q", out)
	}
	if got := asked(); strings.Join(got, ",") != "internal/widget/widget.go" {
		t.Fatalf("obligations asked for %v, want [internal/widget/widget.go]", got)
	}
}
