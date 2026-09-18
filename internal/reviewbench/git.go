package reviewbench

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// GitRunner runs a git command with the given args inside dir and returns
// its combined stdout — the same injectable-exec shape
// cmd/fathom/review.go's gitRunner uses, so tests can fake it.
type GitRunner func(dir string, args ...string) ([]byte, error)

// ExecGit is the real GitRunner: os/exec against the system git binary.
func ExecGit(dir string, args ...string) ([]byte, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return out, fmt.Errorf("git %s (in %s): %w\n%s", strings.Join(args, " "), dir, err, out)
	}
	return out, nil
}

// EnsureRepo makes sure dir is a git repository with "origin" pointing at
// https://github.com/<repo>.git, initializing it if this is the first call
// for that dir (reviewbench reuses ONE checkout per upstream repo across
// every pair from it — the object store dedups shared blobs/trees across
// the depth-1 fetches, keeping total disk far below cloning per pair).
func EnsureRepo(run GitRunner, dir, repo string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("mkdir %s: %w", dir, err)
	}
	if _, err := os.Stat(filepath.Join(dir, ".git")); err != nil {
		if _, err := run(dir, "init", "-q"); err != nil {
			return err
		}
		if _, err := run(dir, "remote", "add", "origin", "https://github.com/"+repo+".git"); err != nil {
			return err
		}
	}
	return nil
}

// FetchCommit does a shallow (depth 1) fetch of one commit sha into dir's
// repo — GitHub's smart-HTTP server serves any reachable sha this way, no
// full clone needed (the reading's own method).
func FetchCommit(run GitRunner, dir, sha string) error {
	_, err := run(dir, "fetch", "--depth", "1", "origin", sha)
	return err
}

// CheckoutCommit detaches HEAD at sha, forcibly (discards any leftover
// working-tree state from the previous pair's checkout in the same reused
// repo dir).
func CheckoutCommit(run GitRunner, dir, sha string) error {
	_, err := run(dir, "checkout", "-f", sha)
	return err
}

// ChangedFilesSince returns the sorted, deduplicated `git diff --name-only
// since` output — the same changed-file resolution cmd/fathom/review.go's
// runReviewBody performs, run against dir's current checkout (HEAD must
// already be at A's merge commit; since is A's base commit sha).
func ChangedFilesSince(run GitRunner, dir, since string) ([]string, error) {
	out, err := run(dir, "diff", "--name-only", since)
	if err != nil {
		return nil, err
	}
	var files []string
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			files = append(files, line)
		}
	}
	return files, nil
}
