package rulereplay

import (
	"archive/tar"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// CommitInfo identifies one commit in a replay range.
type CommitInfo struct {
	SHA     string
	Date    time.Time
	Subject string
	// Parents is the commit's parent SHAs in git's own order (empty for a
	// root commit, length 2+ for a merge). Only Parents[0] — the first
	// parent — is used for new-vs-inherited classification; see the
	// package doc comment.
	Parents []string
}

// sinceSpec is Options.Since, classified into how git should be asked for
// the commit list.
type sinceSpec struct {
	// display is a human-readable rendering of the range, used for
	// Report.RangeSpec — never fed back to git.
	display string
	// logArgs are appended to `git log --reverse --topo-order --format=…`
	// (before a trailing "--").
	logArgs []string
}

// parseSince classifies since: a value already containing ".." is a
// literal two-dot (or three-dot) range, passed to git verbatim; a bare
// positive integer N means "the last N commits" via `git log -n N HEAD`
// — NOT the range "HEAD~N..HEAD", which errors outright on a repository
// with fewer than N+1 commits (every synthetic fixture in this package's
// own tests) and, on a repository with merge commits, can silently name a
// different — usually larger — set than "the last N commits" actually
// means (a side-branch commit merged in during that span is an ancestor
// of HEAD but not of HEAD~N's first-parent-only walk). `-n` limits git's
// own newest-first traversal before --reverse flips it, so it always
// means exactly "the last N commits, oldest first", however small the
// repository or however many merges it contains. Anything else is a git
// ref, meaning every commit reachable from HEAD but not from it
// ("<ref>..HEAD").
func parseSince(since string) sinceSpec {
	s := strings.TrimSpace(since)
	if strings.Contains(s, "..") {
		return sinceSpec{display: s, logArgs: []string{s}}
	}
	if n, err := strconv.Atoi(s); err == nil && n > 0 {
		return sinceSpec{
			display: fmt.Sprintf("last %d commits", n),
			logArgs: []string{"-n", strconv.Itoa(n), "HEAD"},
		}
	}
	rangeSpec := s + "..HEAD"
	return sinceSpec{display: rangeSpec, logArgs: []string{rangeSpec}}
}

// ResolveSince returns since's human-readable range description (
// Report.RangeSpec) — see parseSince for the full classification.
func ResolveSince(since string) string {
	return parseSince(since).display
}

// gitFieldSep separates fields within one `git log --format` line. \x1f
// (unit separator) never appears in a commit subject in practice, and
// unlike a printable character needs no escaping scheme.
const gitFieldSep = "\x1f"

const gitLogFormat = "%H" + gitFieldSep + "%cI" + gitFieldSep + "%P" + gitFieldSep + "%s"

// listCommits returns every commit since selects, oldest → newest
// (--reverse over git's default topological order, so a commit always
// follows every one of its ancestors already in the list).
func listCommits(ctx context.Context, repoRoot, since string) ([]CommitInfo, error) {
	spec := parseSince(since)
	args := append([]string{"-C", repoRoot, "log", "--reverse", "--topo-order", "--format=" + gitLogFormat}, spec.logArgs...)
	args = append(args, "--")
	cmd := exec.CommandContext(ctx, "git", args...)
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git log (%s): %w", spec.display, exitErr(err))
	}
	return parseGitLog(out)
}

func parseGitLog(out []byte) ([]CommitInfo, error) {
	text := strings.TrimRight(string(out), "\n")
	if text == "" {
		return nil, nil
	}
	lines := strings.Split(text, "\n")
	commits := make([]CommitInfo, 0, len(lines))
	for _, line := range lines {
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, gitFieldSep, 4)
		if len(parts) != 4 {
			return nil, fmt.Errorf("git log: malformed line %q", line)
		}
		date, err := time.Parse(time.RFC3339, parts[1])
		if err != nil {
			return nil, fmt.Errorf("git log: parsing date %q: %w", parts[1], err)
		}
		var parents []string
		if p := strings.TrimSpace(parts[2]); p != "" {
			parents = strings.Fields(p)
		}
		commits = append(commits, CommitInfo{SHA: parts[0], Date: date, Parents: parents, Subject: parts[3]})
	}
	return commits, nil
}

// exitErr appends an *exec.ExitError's captured stderr when present, so a
// caller sees git's own complaint instead of just "exit status 128".
func exitErr(err error) error {
	if ee, ok := err.(*exec.ExitError); ok && len(ee.Stderr) > 0 {
		return fmt.Errorf("%w: %s", err, strings.TrimSpace(string(ee.Stderr)))
	}
	return err
}

// archiveCommit checks out sha into a fresh temp directory via `git
// archive`, never the repository's own working tree, and returns a
// cleanup func that removes it. Mirrors internal/receiptverify's
// archiveCommit (VB-20) — same two-step contract (verify the repo and
// commit exist, then stream-extract) — kept as a separate, unexported copy
// rather than a shared dependency between the two packages.
func archiveCommit(ctx context.Context, repoRoot, sha string) (dir string, cleanup func(), err error) {
	if err := exec.CommandContext(ctx, "git", "-C", repoRoot, "rev-parse", "--git-dir").Run(); err != nil {
		return "", nil, fmt.Errorf("%s is not a git repository: %w", repoRoot, err)
	}

	dir, err = os.MkdirTemp("", "fathom-rulereplay-*")
	if err != nil {
		return "", nil, fmt.Errorf("temp dir: %w", err)
	}
	cleanup = func() { _ = os.RemoveAll(dir) }

	cmd := exec.CommandContext(ctx, "git", "-C", repoRoot, "archive", "--format=tar", sha)
	stderr := &strings.Builder{}
	cmd.Stderr = stderr
	pipe, err := cmd.StdoutPipe()
	if err != nil {
		cleanup()
		return "", nil, fmt.Errorf("git archive %s: %w", sha, err)
	}
	if err := cmd.Start(); err != nil {
		cleanup()
		return "", nil, fmt.Errorf("git archive %s: %w", sha, err)
	}
	extractErr := extractTar(pipe, dir)
	waitErr := cmd.Wait()
	if waitErr != nil {
		cleanup()
		msg := strings.TrimSpace(stderr.String())
		if msg != "" {
			return "", nil, fmt.Errorf("git archive %s: %w: %s", sha, waitErr, msg)
		}
		return "", nil, fmt.Errorf("git archive %s: %w", sha, waitErr)
	}
	if extractErr != nil {
		cleanup()
		return "", nil, fmt.Errorf("extract archive of %s: %w", sha, extractErr)
	}
	return dir, cleanup, nil
}

// extractTar extracts a tar stream (as `git archive --format=tar` emits)
// into dir. Every entry is confined under dir; an entry that would escape
// it is refused rather than followed, though a commit git itself produced
// never carries one.
func extractTar(r io.Reader, dir string) error {
	tr := tar.NewReader(r)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		target := filepath.Join(dir, hdr.Name) //nolint:gosec — Clean+containment check below
		if !strings.HasPrefix(target, filepath.Clean(dir)+string(os.PathSeparator)) && target != filepath.Clean(dir) {
			return fmt.Errorf("archive entry %q escapes the checkout directory", hdr.Name)
		}
		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return err
			}
			f, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(hdr.Mode&0o777)) //nolint:gosec — target is containment-checked above
			if err != nil {
				return err
			}
			if _, err := io.CopyN(f, tr, hdr.Size); err != nil && err != io.EOF { //nolint:gosec — hdr.Size comes from git's own archive, not attacker input
				f.Close()
				return err
			}
			if err := f.Close(); err != nil {
				return err
			}
		default:
			// Symlinks and other special entries: a Go module tree never
			// legitimately needs one for this purpose — skip rather than
			// fail the whole extraction.
		}
	}
}
