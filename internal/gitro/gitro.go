// Package gitro runs git reads that never write the repository they read.
//
// `git status` and a `git diff` against the working tree refresh the
// index's stat cache as a side effect and, when they can take index.lock,
// write the index back. A read that happens inside a repository somebody is
// working in — the console computing a conservation report on a page view,
// in the registered live checkout — therefore rewrote .git/index there, and
// an agent's concurrent `git add` failed with "Unable to create
// '.git/index.lock': File exists" (console wave C review, defect 11).
// GIT_OPTIONAL_LOCKS=0 stops `git status` from writing but not `git diff`.
//
// So every command here runs with GIT_OPTIONAL_LOCKS=0, and one that
// refreshes the index (status, diff) reads a private copy of it through
// GIT_INDEX_FILE, removed once the command has run: the answer is the
// repository's own, and its index is never locked or written. The copy is
// taken whole (git replaces the index by renaming a finished file, so a
// read sees one complete version) and keeps the index's modification time
// (git's racy-entry check compares against it), and core.splitIndex is off
// for the command, so no shared-index file is written into .git either.
package gitro

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// Command returns the git command for args run in dir, and a cleanup to
// call once it has finished (it removes the private index copy, if any).
func Command(ctx context.Context, dir string, args ...string) (*exec.Cmd, func(), error) {
	env := append(os.Environ(), "GIT_OPTIONAL_LOCKS=0")
	cleanup := func() {}
	if refreshesIndex(args) {
		idx, done, err := privateIndex(ctx, dir)
		if err != nil {
			return nil, cleanup, err
		}
		cleanup = done
		env = append(env, "GIT_INDEX_FILE="+idx)
		args = append([]string{"-c", "core.splitIndex=false"}, args...)
	}
	cmd := exec.CommandContext(ctx, "git", args...)
	// A git killed at its context's end may leave a child holding the
	// output pipe; the caller is released after WaitDelay regardless.
	cmd.WaitDelay = 2 * time.Second
	cmd.Dir = dir
	cmd.Env = env
	return cmd, cleanup, nil
}

// Output runs git with args in dir without writing the repository and
// returns its standard output, as exec.Cmd.Output does.
func Output(ctx context.Context, dir string, args ...string) ([]byte, error) {
	cmd, cleanup, err := Command(ctx, dir, args...)
	defer cleanup()
	if err != nil {
		return nil, err
	}
	return cmd.Output()
}

// CombinedOutput is Output with standard error interleaved.
func CombinedOutput(ctx context.Context, dir string, args ...string) ([]byte, error) {
	cmd, cleanup, err := Command(ctx, dir, args...)
	defer cleanup()
	if err != nil {
		return nil, err
	}
	return cmd.CombinedOutput()
}

// refreshesIndex reports whether the git subcommand in args refreshes the
// index as a side effect: status, and diff (which compares with the working
// tree unless given --cached; with it there is nothing to refresh, and the
// private copy costs one small file).
func refreshesIndex(args []string) bool {
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "-c" || a == "-C":
			i++ // the option's value
		case strings.HasPrefix(a, "-"):
		default:
			return a == "status" || a == "diff"
		}
	}
	return false
}

// privateIndex copies dir's index to a new temporary directory and returns
// the copy's path and its cleanup. A repository with no index yet (nothing
// ever added) gets a path that does not exist, which git reads as an empty
// index — as it reads the missing real one.
func privateIndex(ctx context.Context, dir string) (string, func(), error) {
	src := os.Getenv("GIT_INDEX_FILE")
	if src == "" {
		cmd := exec.CommandContext(ctx, "git", "rev-parse", "--git-path", "index")
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), "GIT_OPTIONAL_LOCKS=0")
		out, err := cmd.Output()
		if err != nil {
			return "", func() {}, err
		}
		src = strings.TrimSpace(string(out))
	}
	if !filepath.IsAbs(src) {
		src = filepath.Join(dir, src)
	}
	tmp, err := os.MkdirTemp("", "fathom-gitro-")
	if err != nil {
		return "", func() {}, err
	}
	cleanup := func() { _ = os.RemoveAll(tmp) }
	dst := filepath.Join(tmp, "index")
	st, err := os.Stat(src)
	if errors.Is(err, fs.ErrNotExist) {
		return dst, cleanup, nil
	}
	var raw []byte
	if err == nil {
		raw, err = os.ReadFile(src)
	}
	if err != nil {
		cleanup()
		return "", func() {}, err
	}
	if err := os.WriteFile(dst, raw, 0o600); err != nil {
		cleanup()
		return "", func() {}, err
	}
	// The copy keeps the index's modification time: git trusts an entry's
	// stat only when the entry is older than the index file ("racy git"),
	// so a copy stamped now would read a same-size edit made in the index's
	// own second as no change.
	if err := os.Chtimes(dst, st.ModTime(), st.ModTime()); err != nil {
		cleanup()
		return "", func() {}, err
	}
	return dst, cleanup, nil
}
