package main

// edithook_dest_test.go — XM-36 (docs/plans/exploration-maps.md Wave 2d):
// the edit hook's beat records a destination class and the target path for
// every file-target answer, post-bash writes included.

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// knownSilenceReason reports whether r, a beat's reason, is one of the
// silenceReason constants: the vocabulary a test reads back from hooks.jsonl.
func knownSilenceReason(r string) bool {
	switch silenceReason(r) {
	case silenceArmOff, silenceCapabilityOff, silenceGitignored, silenceForeignTarget,
		silenceDoubtAlreadyServed, silenceNoRules, silenceNoGuard, silenceGuardSilent,
		silenceUnresolved, silenceNoService, silenceDaemonError, silenceEncodeFailed:
		return true
	}
	return false
}

// gitT runs git in dir, failing the test on error.
func gitT(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir, "-c", "user.name=t", "-c", "user.email=t@t", "-c", "commit.gpgsign=false"}, args...)...)
	cmd.Env = append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

// destFixture is the indexed repository (with an origin), a clone of it by
// path, a separate checkout naming the same origin, a linked worktree, a
// scratch directory and a path outside all of them.
type destFixture struct {
	repo, tracked, ignored        string
	pathClone, urlClone, worktree string
	scratch, outside              string
}

func newDestFixture(t *testing.T) destFixture {
	t.Helper()
	repo, tracked, ignored := deployedFixture(t)
	gitT(t, repo, "remote", "add", "origin", "https://forge.example/team/widgetco.git")
	gitT(t, repo, "add", "-A")
	gitT(t, repo, "commit", "-qm", "init")
	base := t.TempDir()
	f := destFixture{repo: repo, tracked: tracked, ignored: ignored}
	f.pathClone = filepath.Join(base, "clone-by-path")
	gitT(t, base, "clone", "-q", repo, f.pathClone)
	f.urlClone = filepath.Join(base, "clone-by-url")
	if err := os.MkdirAll(f.urlClone, 0o755); err != nil {
		t.Fatal(err)
	}
	gitT(t, f.urlClone, "init", "-q")
	gitT(t, f.urlClone, "remote", "add", "origin", "git@forge.example:team/widgetco")
	f.worktree = filepath.Join(base, "wt")
	gitT(t, repo, "worktree", "add", "-q", f.worktree)
	f.scratch = filepath.Join(os.TempDir(), "session-scratchpad-xm36", "draft.sh")
	f.outside = "/xm36-nonexistent/elsewhere/notes.md"
	return f
}

func TestBeatRecordsDestinationClassAndPath(t *testing.T) {
	f := newDestFixture(t)
	rel := func(root string) string { return filepath.Join(root, "internal", "widget", "widget.go") }

	// The classifier, against the project the hook serves (the indexed repo).
	for _, c := range []struct{ target, want string }{
		{f.tracked, destIndexed},
		{f.ignored, destGitignored},
		{rel(f.pathClone), destClone},
		{filepath.Join(f.urlClone, "x.go"), destClone},
		{rel(f.worktree), destWorktree},
		{f.scratch, destScratch},
		{f.outside, destOutside},
	} {
		if got := destinationClass(f.repo, c.target); got != c.want {
			t.Errorf("destinationClass(%s) = %q, want %q", c.target, got, c.want)
		}
	}

	// The beats, as deployed: an Edit and a sed into each class.
	home := deployedHome(t)
	srv, _ := deployedDaemon(t)
	env := deployedEnv(home, f.repo, srv.URL)
	hooks := deployedEditHooks(t)
	post, postBash := hooks["PostToolUse Edit|Write|MultiEdit"], hooks["PostToolUse Bash"]
	edits := map[string]string{
		f.tracked:        destIndexed,
		rel(f.pathClone): destClone,
		rel(f.worktree):  destWorktree,
		f.outside:        destOutside,
	}
	for target := range edits {
		runDeployedHook(t, env, post, f.repo, map[string]any{"file_path": target, "old_string": "a", "new_string": "b"}, "Edit")
	}
	seds := map[string]string{ // cwd → the class of the file the sed wrote there
		filepath.Dir(f.tracked):        destIndexed,
		filepath.Dir(rel(f.pathClone)): destClone,
		filepath.Dir(rel(f.worktree)):  destWorktree,
		filepath.Dir(f.ignored):        destGitignored,
	}
	for cwd := range seds {
		name := "widget.go"
		if seds[cwd] == destGitignored {
			name = "NOTES.md"
		}
		runDeployedHook(t, env, postBash, cwd, map[string]any{"command": "sed -i '' 's/a/b/' " + name}, "Bash")
	}

	got := map[string]string{} // phase + path → class
	for _, b := range readDeployedBeats(t, filepath.Join(home, ".fathom", "hooks.jsonl")) {
		if b.Channel != "edit-hook" || b.Kind != "answered" {
			continue
		}
		path, class := b.Meta["dest_path"], b.Meta["dest_class"]
		if path == "" || class == "" {
			t.Errorf("a file-target answer without a destination: %v", b.Meta)
			continue
		}
		got[b.Meta["phase"]+" "+path] = class
	}
	for target, want := range edits {
		if g := got["post "+target]; g != want {
			t.Errorf("post answer for %s: dest_class %q, want %q", target, g, want)
		}
	}
	for cwd, want := range seds {
		var found string
		for k, v := range got {
			if strings.HasPrefix(k, "post-bash "+cwd+string(filepath.Separator)) {
				found = v
			}
		}
		if found != want {
			t.Errorf("post-bash answer for a sed in %s: dest_class %q, want %q (beats %v)", cwd, found, want, got)
		}
	}
}
