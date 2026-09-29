package gitro

// head.go — RT-101 (docs/design/responsible-tenant.md §7.1): the HEAD
// commit of a work tree, read from .git/HEAD and the ref files without a
// fork. Auto-refresh asked `git rev-parse HEAD` on every per-service read
// (the audit's autorefresh-git-fork-per-read hazard): one process per
// query, on the request path. Head reads HEAD, follows a symbolic ref to its
// loose file or packed-refs (a linked worktree's `gitdir:` file and its
// commondir handled), and caches the answer per root until one of the files
// it read changes mtime. Only a layout it cannot read (reftable, an unborn
// branch) runs git, under a 2 s deadline; a fallback that does not answer
// in time is ErrFreshnessUnknown — the caller discloses freshness as
// unknown instead of waiting on git. The fallback's answer is cached too,
// while the files it was asked about stand and for at most failureCacheTTL,
// so such a repository forks once per TTL, not once per read. A branch git
// confirms has no commit yet is ErrUnbornHead: a work tree with no HEAD
// commit, whose freshness is its content (the caller's manifest case).

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// ErrNotRepository is Head's answer for a root inside no git work tree.
var ErrNotRepository = errors.New("gitro: not inside a git work tree")

// ErrFreshnessUnknown is Head's answer when HEAD could not be read from the
// files and the git fallback failed or exceeded its deadline.
var ErrFreshnessUnknown = errors.New("gitro: HEAD unreadable — freshness unknown")

// ErrUnbornHead is Head's answer when HEAD names a branch with no commit
// yet (a fresh `git init`), confirmed by git: there is no commit to compare,
// so the connector treats the tree as a non-git corpus (content manifest).
// It also matches ErrFreshnessUnknown: a service whose recorded watermark
// is a commit still has no HEAD to compare it with.
var ErrUnbornHead = errors.New("gitro: HEAD names a branch with no commit yet")

// errUnresolvedRef is readHead's error for a symbolic ref found in no loose
// file and not in packed-refs.
var errUnresolvedRef = errors.New("ref is neither a loose ref nor in packed-refs")

// failureCacheTTL bounds how long a fallback's answer is reused (a var for
// tests).
var failureCacheTTL = 30 * time.Second

// headNow is the cache's clock (a var for tests).
var headNow = time.Now

// fallbackDeadline bounds the git fallback (a var for tests).
var fallbackDeadline = 2 * time.Second

// headFallback runs `git rev-parse HEAD` in root under ctx (a var for tests).
var headFallback = func(ctx context.Context, root string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", "-C", root, "rev-parse", "HEAD")
	cmd.Env = Env()
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// headCacheMax bounds the per-root cache; roots are registered services, so
// the bound is never reached in practice and a reset is harmless.
const headCacheMax = 256

// fileStamp is one file's identity for cache validation.
type fileStamp struct {
	path  string
	mtime int64
	size  int64
	ok    bool
}

func stampOf(path string) fileStamp {
	fi, err := os.Stat(path)
	if err != nil {
		return fileStamp{path: path}
	}
	return fileStamp{path: path, mtime: fi.ModTime().UnixNano(), size: fi.Size(), ok: true}
}

type headEntry struct {
	sha    string
	err    error       // a cached fallback failure
	stamps []fileStamp // every file the answer was read from (or found absent)
	// expires bounds a fallback answer (zero for one read from the files).
	expires time.Time
}

var (
	headMu    sync.Mutex
	headCache = map[string]headEntry{}
)

// ResetHeadCache empties the per-root cache (tests).
func ResetHeadCache() {
	headMu.Lock()
	headCache = map[string]headEntry{}
	headMu.Unlock()
}

// Head returns the commit root's work tree has checked out. It reads the
// repository's files — never forks for a readable layout — and caches the
// answer per root until a file it read moves. "" with ErrNotRepository when
// root is in no work tree; "" with ErrFreshnessUnknown when HEAD could not
// be read and the 2 s git fallback did not answer.
func Head(ctx context.Context, root string) (string, error) {
	if root == "" {
		return "", ErrNotRepository
	}
	headMu.Lock()
	e, hit := headCache[root]
	headMu.Unlock()
	if hit && stampsCurrent(e.stamps) && (e.expires.IsZero() || headNow().Before(e.expires)) {
		return e.sha, e.err
	}
	sha, stamps, err := readHead(root)
	switch {
	case err == nil:
		storeHead(root, headEntry{sha: sha, stamps: stamps})
		return sha, nil
	case errors.Is(err, ErrNotRepository):
		return "", err
	}
	fctx, cancel := context.WithTimeout(ctx, fallbackDeadline)
	defer cancel()
	sha, ferr := headFallback(fctx, root)
	if ferr == nil && isObjectName(sha) {
		storeHead(root, headEntry{sha: sha, stamps: stamps, expires: headNow().Add(failureCacheTTL)})
		return sha, nil
	}
	if ctx.Err() != nil {
		// The caller gave up: nothing was learned about the repository.
		return "", fmt.Errorf("%w (%s: %v)", ErrFreshnessUnknown, root, ctx.Err())
	}
	var out error
	switch {
	case ferr != nil && fctx.Err() == nil && errors.Is(err, errUnresolvedRef) && !hasReftable(stamps):
		// The files name a branch with no ref and git, answering in time,
		// cannot resolve HEAD either: an unborn branch.
		// Still freshness unknown to a caller comparing a recorded commit
		// (a SHA watermark whose branch vanished); a caller without one
		// (the connector) takes it as a tree with no commit yet.
		out = fmt.Errorf("%w — %w (%s: %v; git fallback: %v)", ErrUnbornHead, ErrFreshnessUnknown, root, err, ferr)
	default:
		if ferr == nil {
			ferr = fmt.Errorf("unexpected output %q", sha)
		}
		out = fmt.Errorf("%w (%s: %v; git fallback: %v)", ErrFreshnessUnknown, root, err, ferr)
	}
	storeHead(root, headEntry{err: out, stamps: stamps, expires: headNow().Add(failureCacheTTL)})
	return "", out
}

func storeHead(root string, e headEntry) {
	headMu.Lock()
	if len(headCache) >= headCacheMax {
		headCache = map[string]headEntry{}
	}
	headCache[root] = e
	headMu.Unlock()
}

// reftableList is the file a reftable repository's refs move through.
func reftableList(commonDir string) string {
	return filepath.Join(commonDir, "reftable", "tables.list")
}

// hasReftable reports whether stamps saw a reftable table list.
func hasReftable(stamps []fileStamp) bool {
	for _, s := range stamps {
		if s.ok && filepath.Base(s.path) == "tables.list" && filepath.Base(filepath.Dir(s.path)) == "reftable" {
			return true
		}
	}
	return false
}

func stampsCurrent(stamps []fileStamp) bool {
	for _, s := range stamps {
		if stampOf(s.path) != s {
			return false
		}
	}
	return true
}

// readHead resolves root's HEAD from the files alone. On failure inside a
// work tree the stamps of every file consulted are still returned, so a
// fallback's answer can be cached against them.
func readHead(root string) (string, []fileStamp, error) {
	gitDir, commonDir, stamps, err := findGitDirs(root)
	if err != nil {
		return "", nil, err
	}
	stamps = append(stamps, stampOf(reftableList(commonDir)))
	headPath := filepath.Join(gitDir, "HEAD")
	stamps = append(stamps, stampOf(headPath))
	raw, err := os.ReadFile(headPath)
	if err != nil {
		return "", stamps, err
	}
	val := strings.TrimSpace(string(raw))
	for depth := 0; depth < 5; depth++ {
		if !strings.HasPrefix(val, "ref:") {
			if isObjectName(val) {
				return val, stamps, nil
			}
			return "", stamps, fmt.Errorf("HEAD holds %q, not an object name", val)
		}
		ref := strings.TrimSpace(strings.TrimPrefix(val, "ref:"))
		if ref == "" || strings.Contains(ref, "..") {
			return "", stamps, fmt.Errorf("HEAD names an unreadable ref %q", ref)
		}
		next, found, refStamps := readRef(gitDir, commonDir, ref)
		stamps = append(stamps, refStamps...)
		if !found {
			return "", stamps, fmt.Errorf("ref %s: %w", ref, errUnresolvedRef)
		}
		val = next
	}
	return "", stamps, errors.New("symbolic refs nest too deeply")
}

// readRef reads ref as a loose file (per-worktree gitDir first, then the
// common dir) or from packed-refs, returning every file it consulted.
func readRef(gitDir, commonDir, ref string) (string, bool, []fileStamp) {
	var stamps []fileStamp
	dirs := []string{gitDir}
	if commonDir != gitDir {
		dirs = append(dirs, commonDir)
	}
	for _, d := range dirs {
		p := filepath.Join(d, filepath.FromSlash(ref))
		st := stampOf(p)
		stamps = append(stamps, st)
		if !st.ok {
			continue
		}
		raw, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		return strings.TrimSpace(string(raw)), true, stamps
	}
	packed := filepath.Join(commonDir, "packed-refs")
	stamps = append(stamps, stampOf(packed))
	raw, err := os.ReadFile(packed)
	if err != nil {
		return "", false, stamps
	}
	sc := bufio.NewScanner(bytes.NewReader(raw))
	sc.Buffer(make([]byte, 64<<10), 1<<20)
	for sc.Scan() {
		line := sc.Text()
		if line == "" || line[0] == '#' || line[0] == '^' {
			continue
		}
		sha, name, ok := strings.Cut(line, " ")
		if ok && name == ref && isObjectName(sha) {
			return sha, true, stamps
		}
	}
	return "", false, stamps
}

// findGitDirs walks up from root to the first .git entry: a directory (a
// main work tree) or a file naming the gitdir (a linked worktree or a
// submodule), whose commondir file, when present, locates the shared refs.
func findGitDirs(root string) (gitDir, commonDir string, stamps []fileStamp, err error) {
	dir, aerr := filepath.Abs(root)
	if aerr != nil {
		return "", "", nil, aerr
	}
	// A root that does not exist is in no work tree — as `git -C root`
	// fails there — never the enclosing directory's repository.
	if fi, serr := os.Stat(dir); serr != nil || !fi.IsDir() {
		return "", "", nil, ErrNotRepository
	}
	for {
		dotGit := filepath.Join(dir, ".git")
		fi, serr := os.Stat(dotGit)
		if serr == nil {
			if fi.IsDir() {
				return dotGit, dotGit, []fileStamp{stampOf(dotGit)}, nil
			}
			raw, rerr := os.ReadFile(dotGit)
			if rerr != nil {
				return "", "", nil, rerr
			}
			line := strings.TrimSpace(string(raw))
			if !strings.HasPrefix(line, "gitdir:") {
				return "", "", nil, fmt.Errorf("%s is not a gitdir file", dotGit)
			}
			gd := strings.TrimSpace(strings.TrimPrefix(line, "gitdir:"))
			if !filepath.IsAbs(gd) {
				gd = filepath.Join(dir, gd)
			}
			cd := gd
			if c, cerr := os.ReadFile(filepath.Join(gd, "commondir")); cerr == nil {
				cd = strings.TrimSpace(string(c))
				if !filepath.IsAbs(cd) {
					cd = filepath.Join(gd, cd)
				}
			}
			return filepath.Clean(gd), filepath.Clean(cd), []fileStamp{stampOf(dotGit)}, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", "", nil, ErrNotRepository
		}
		dir = parent
	}
}

// isObjectName reports whether s is a full SHA-1 or SHA-256 object name.
func isObjectName(s string) bool {
	if len(s) != 40 && len(s) != 64 {
		return false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f') {
			return false
		}
	}
	return true
}

// IsObjectName reports whether s is a full git object name (40 or 64
// lowercase hex characters) — how a stored watermark says it is a commit.
func IsObjectName(s string) bool { return isObjectName(s) }
