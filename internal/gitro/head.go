package gitro

// head.go — HEAD without a process (RT-115, docs/design/responsible-tenant.md
// §14). The console asked `git rev-parse HEAD` of every registered service
// on every model build and every plan read: one fork per service per page.
// Head reads the same answer from the repository's files — .git (a
// directory, or a linked worktree's "gitdir:" file), HEAD, the loose ref and
// packed-refs in the common directory — so a page costs file reads, never a
// child process. A repository layout it does not read (the reftable
// format) is an error the caller names, never a guess.

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Head returns the commit HEAD names for the work tree containing dir.
func Head(dir string) (string, error) {
	gitDir, err := findGitDir(dir)
	if err != nil {
		return "", err
	}
	common := gitDir
	if raw, err := os.ReadFile(filepath.Join(gitDir, "commondir")); err == nil {
		c := strings.TrimSpace(string(raw))
		if !filepath.IsAbs(c) {
			c = filepath.Join(gitDir, c)
		}
		common = filepath.Clean(c)
	}
	raw, err := os.ReadFile(filepath.Join(gitDir, "HEAD"))
	if err != nil {
		return "", fmt.Errorf("read HEAD in %s: %w", gitDir, err)
	}
	v := strings.TrimSpace(string(raw))
	for depth := 0; depth < 8; depth++ {
		ref, symbolic := strings.CutPrefix(v, "ref: ")
		if !symbolic {
			if isHexSHA(v) {
				return v, nil
			}
			return "", fmt.Errorf("HEAD in %s is neither a ref nor a commit: %q", gitDir, v)
		}
		ref = strings.TrimSpace(ref)
		next, err := readRef(gitDir, common, ref)
		if err != nil {
			return "", err
		}
		v = next
	}
	return "", fmt.Errorf("HEAD in %s: symbolic refs nest too deep", gitDir)
}

// findGitDir walks up from dir to the repository's git directory.
func findGitDir(dir string) (string, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return "", err
	}
	for d := abs; ; {
		p := filepath.Join(d, ".git")
		st, err := os.Stat(p)
		switch {
		case err == nil && st.IsDir():
			return p, nil
		case err == nil:
			raw, rerr := os.ReadFile(p)
			if rerr != nil {
				return "", rerr
			}
			g, ok := strings.CutPrefix(strings.TrimSpace(string(raw)), "gitdir: ")
			if !ok {
				return "", fmt.Errorf("%s is not a gitdir file", p)
			}
			if !filepath.IsAbs(g) {
				g = filepath.Join(d, g)
			}
			return filepath.Clean(g), nil
		case !errors.Is(err, os.ErrNotExist):
			return "", err
		}
		parent := filepath.Dir(d)
		if parent == d {
			return "", fmt.Errorf("%s is not inside a git repository", dir)
		}
		d = parent
	}
}

// readRef resolves one ref: per-worktree refs (HEAD-like) in gitDir, the
// rest loose in the common directory, then packed-refs.
func readRef(gitDir, common, ref string) (string, error) {
	if _, err := os.Stat(filepath.Join(common, "reftable")); err == nil {
		return "", fmt.Errorf("%s uses the reftable format, which is not read here", common)
	}
	for _, base := range []string{gitDir, common} {
		raw, err := os.ReadFile(filepath.Join(base, filepath.FromSlash(ref)))
		if err == nil {
			return strings.TrimSpace(string(raw)), nil
		}
		if !errors.Is(err, os.ErrNotExist) {
			return "", err
		}
	}
	fh, err := os.Open(filepath.Join(common, "packed-refs"))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", fmt.Errorf("ref %s not found in %s (an unborn branch?)", ref, common)
		}
		return "", err
	}
	defer fh.Close()
	sc := bufio.NewScanner(fh)
	sc.Buffer(make([]byte, 0, 64*1024), 1<<20)
	want := []byte(ref)
	for sc.Scan() {
		line := sc.Bytes()
		if len(line) == 0 || line[0] == '#' || line[0] == '^' {
			continue
		}
		sha, name, ok := bytes.Cut(line, []byte(" "))
		if ok && bytes.Equal(bytes.TrimSpace(name), want) {
			return string(sha), nil
		}
	}
	if err := sc.Err(); err != nil {
		return "", err
	}
	return "", fmt.Errorf("ref %s not found in %s (an unborn branch?)", ref, common)
}

func isHexSHA(s string) bool {
	if len(s) != 40 && len(s) != 64 {
		return false
	}
	for _, r := range s {
		if !(r >= '0' && r <= '9' || r >= 'a' && r <= 'f') {
			return false
		}
	}
	return true
}
