package main

// edithook_fold_test.go — XM-37 (docs/plans/exploration-maps.md Wave 2d): a
// target in the `clone` or `worktree` class that is tracked in its own work
// tree is folded to its repository-relative key, and the indexed
// repository's rules for that key are served instead of a silent
// `foreign-target`. An untracked target there stays silent. O821's accident
// (a relative Bash target named as the same path in the owner's checkout)
// stays removed: a path the indexed repository tracks and the clone does not
// is never folded.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEditHookFoldsCloneAndWorktreeToRepoRelative(t *testing.T) {
	f := newDestFixture(t)
	const key = "internal/widget/widget.go"
	in := func(root string, parts ...string) string {
		return filepath.Join(append([]string{root, "internal", "widget"}, parts...)...)
	}
	write := func(p, body string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	// O821's shape: a file the indexed repository commits after the clone was
	// made, so the clone holds the same path untracked; and a brand-new file
	// in the clone.
	write(in(f.repo, "later.go"), "package widget\n")
	gitT(t, f.repo, "add", "internal/widget/later.go")
	gitT(t, f.repo, "commit", "-qm", "later")
	cloneLater, cloneNew := in(f.pathClone, "later.go"), in(f.pathClone, "brand_new.go")
	write(cloneLater, "package widget\n")
	write(cloneNew, "package widget\n")

	// The fold, against the project the hook serves (the indexed repo).
	for _, c := range []struct {
		target, class string
		tracked       bool
	}{
		{in(f.pathClone, "widget.go"), destClone, true},
		{in(f.worktree, "widget.go"), destWorktree, true},
		{cloneNew, destClone, false},
		{cloneLater, destClone, false},
		{filepath.Join(f.urlClone, "x.go"), destClone, false},
		{f.tracked, "", false},
		{f.outside, "", false},
	} {
		got := foldCloneTarget(f.repo, c.target)
		if got.class != c.class || got.tracked != c.tracked {
			t.Errorf("foldCloneTarget(%s) = %+v, want class %q tracked %v", c.target, got, c.class, c.tracked)
			continue
		}
		if c.tracked && (got.key != key || got.path != filepath.Join(f.repo, filepath.FromSlash(key))) {
			t.Errorf("foldCloneTarget(%s) folded to %q (%s), want %q under the indexed repo", c.target, got.key, got.path, key)
		}
		if !c.tracked && (got.key != "" || got.path != "") {
			t.Errorf("foldCloneTarget(%s) = %+v: an untracked target must not fold", c.target, got)
		}
	}

	// As deployed: an absolute Edit and a relative Bash write from the work
	// tree's own directory, into a clone and into a linked worktree.
	home := deployedHome(t)
	srv, asked := deployedDaemon(t)
	env := deployedEnv(home, f.repo, srv.URL)
	hooks := deployedEditHooks(t)
	post, postBash := hooks["PostToolUse Edit|Write|MultiEdit"], hooks["PostToolUse Bash"]
	edit := func(target string) string {
		return runDeployedHook(t, env, post, f.repo, map[string]any{"file_path": target, "old_string": "a", "new_string": "b"}, "Edit")
	}
	sed := func(cwd, name string) string {
		return runDeployedHook(t, env, postBash, cwd, map[string]any{"command": "sed -i '' 's/a/b/' " + name}, "Bash")
	}
	served := []struct{ label, out string }{
		{"clone Edit", edit(in(f.pathClone, "widget.go"))},
		{"clone sed", sed(in(f.pathClone), "widget.go")},
		{"worktree Edit", edit(in(f.worktree, "widget.go"))},
		{"worktree sed", sed(in(f.worktree), "widget.go")},
	}
	for _, s := range served {
		if !strings.Contains(s.out, "Determinism is a contract") {
			t.Errorf("%s: the indexed repository's rules were not served: %q", s.label, s.out)
		}
		if !strings.Contains(s.out, foldedRulesNote) {
			t.Errorf("%s: the payload does not say the rules are the indexed repository's for this path: %q", s.label, s.out)
		}
	}
	for _, silent := range []struct{ label, out string }{
		{"untracked clone Edit", edit(cloneNew)},
		{"O821: relative write of a path only the indexed repo tracks", sed(in(f.pathClone), "later.go")},
	} {
		if silent.out != "" {
			t.Errorf("%s printed %q; an untracked clone target stays silent", silent.label, silent.out)
		}
	}
	for _, file := range asked() {
		if file != key {
			t.Errorf("obligations was asked for %q; only the folded key %q may reach the daemon", file, key)
		}
	}
	if n := len(asked()); n != len(served) {
		t.Errorf("obligations asked %d time(s), want %d (one per folded answer)", n, len(served))
	}

	// The beats keep the destination class and record the fold.
	type answer struct{ class, emitted, reason, fold, foldKey string }
	got := map[string]answer{} // phase + dest_path
	for _, b := range readDeployedBeats(t, filepath.Join(home, ".fathom", "hooks.jsonl")) {
		if b.Channel != "edit-hook" || b.Kind != "answered" {
			continue
		}
		got[b.Meta["phase"]+" "+b.Meta["dest_path"]] = answer{b.Meta["dest_class"], b.Meta["emitted"], b.Meta["reason"], b.Meta["fold"], b.Meta["fold_key"]}
	}
	folded := func(class string) answer { return answer{class, "true", "", foldTracked, key} }
	untracked := answer{destClone, "false", string(silenceForeignTarget), foldUntracked, ""}
	for k, want := range map[string]answer{
		"post " + in(f.pathClone, "widget.go"):      folded(destClone),
		"post-bash " + in(f.pathClone, "widget.go"): folded(destClone),
		"post " + in(f.worktree, "widget.go"):       folded(destWorktree),
		"post-bash " + in(f.worktree, "widget.go"):  folded(destWorktree),
		"post " + cloneNew:                          untracked,
		"post-bash " + cloneLater:                   untracked,
	} {
		if g, ok := got[k]; !ok || g != want {
			t.Errorf("beat for %s = %+v, want %+v (beats %v)", k, g, want, got)
		}
	}

	// The in-process dispatch folds the same way (the static edge to the code
	// the hook process ran).
	if out := runInProcessHook(t, home, f.repo, srv.URL, post, f.repo, map[string]any{"file_path": in(f.pathClone, "widget.go"), "old_string": "a", "new_string": "b"}, "Edit"); !strings.Contains(out, foldedRulesNote) {
		t.Errorf("in-process clone Edit did not fold: %q", out)
	}
}
