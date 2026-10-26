package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

const (
	gitOutputCap  = 16 << 20
	policyBlobCap = 1 << 20
	pushInputCap  = 64 << 10
)

type cappedWriter struct {
	buf bytes.Buffer
	max int
}

func (w *cappedWriter) Write(p []byte) (int, error) {
	if len(p) > w.max-w.buf.Len() {
		return 0, errors.New("Git output exceeds limit")
	}
	return w.buf.Write(p)
}

// git runs plumbing with a fixed environment. IndexFile is resolved relative
// to Dir, not the process working directory. Neither Git hooks, local filters,
// external diffs, pagers, fsmonitors nor inherited Git redirection are used.
func (g gitSource) git(ctx context.Context, cap int, input []byte, args ...string) ([]byte, error) {
	root := g.Dir
	if root == "" {
		root = "."
	}
	root, err := filepath.Abs(root)
	if err != nil {
		return nil, errors.New("invalid repository directory")
	}
	root, err = filepath.EvalSymlinks(root)
	if err != nil {
		return nil, errors.New("invalid repository directory")
	}
	gitBin, err := trustedGitPath(root, g.GitBin)
	if err != nil {
		return nil, err
	}
	base := []string{
		"--literal-pathspecs", "-C", root,
		"-c", "core.fsmonitor=false",
		"-c", "core.hooksPath=/dev/null",
		"-c", "core.pager=cat",
		"-c", "diff.external=",
		"-c", "diff.trustExitCode=false",
		"-c", "core.quotePath=false",
	}
	cmd := exec.CommandContext(ctx, gitBin, append(base, args...)...)
	env := make([]string, 0, len(os.Environ())+8)
	for _, entry := range os.Environ() {
		key, _, _ := strings.Cut(entry, "=")
		if strings.HasPrefix(strings.ToUpper(key), "GIT_") {
			continue
		}
		env = append(env, entry)
	}
	env = append(env,
		"GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL="+os.DevNull,
		"GIT_ATTR_NOSYSTEM=1", "GIT_NO_REPLACE_OBJECTS=1",
		"GIT_NO_LAZY_FETCH=1",
		"GIT_OPTIONAL_LOCKS=0", "GIT_TERMINAL_PROMPT=0", "GIT_PAGER=cat",
	)
	if g.IndexFile != "" {
		index := g.IndexFile
		if !filepath.IsAbs(index) {
			index = filepath.Join(root, index)
		}
		env = append(env, "GIT_INDEX_FILE="+index)
	}
	cmd.Env = env
	cmd.Stdin = bytes.NewReader(input)
	if cap <= 0 || cap > gitOutputCap {
		cap = gitOutputCap
	}
	var out cappedWriter
	out.max = cap
	cmd.Stdout = &out
	cmd.Stderr = &cappedWriter{max: 4096}
	if err := cmd.Run(); err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, fmt.Errorf("Git %s failed or exceeded output limit", args[0])
	}
	return out.buf.Bytes(), nil
}

// trustedGitPath resolves the executable before any Git command runs. Git
// inside the candidate checkout is controlled by the code being evaluated.
func trustedGitPath(dir, configured string) (string, error) {
	canonicalDir, err := filepath.Abs(dir)
	if err != nil {
		return "", errors.New("invalid repository directory")
	}
	canonicalDir, err = filepath.EvalSymlinks(canonicalDir)
	if err != nil {
		return "", errors.New("invalid repository directory")
	}
	gitBin := configured
	if gitBin == "" {
		gitBin, err = exec.LookPath("git")
		if err != nil {
			return "", errors.New("trusted Git executable unavailable")
		}
	} else if !filepath.IsAbs(gitBin) {
		return "", errors.New("Git executable must be an absolute path")
	}
	gitBin, err = filepath.Abs(gitBin)
	if err != nil {
		return "", errors.New("invalid Git executable")
	}
	gitBin, err = filepath.EvalSymlinks(gitBin)
	if err != nil {
		return "", errors.New("invalid Git executable")
	}
	if withinPath(canonicalDir, gitBin) {
		return "", errors.New("Git executable is inside candidate repository")
	}
	for ancestor := canonicalDir; ; ancestor = filepath.Dir(ancestor) {
		_, err := os.Lstat(filepath.Join(ancestor, ".git"))
		if err == nil {
			if withinPath(ancestor, gitBin) {
				return "", errors.New("Git executable is inside candidate repository")
			}
			break
		}
		if !errors.Is(err, os.ErrNotExist) {
			return "", errors.New("cannot inspect repository ancestry")
		}
		if parent := filepath.Dir(ancestor); parent == ancestor {
			break
		}
	}
	return gitBin, nil
}

func withinPath(parent, path string) bool {
	rel, err := filepath.Rel(parent, path)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

func (g gitSource) resolveCommit(ctx context.Context, ref string) (string, error) {
	if ref == "" || strings.ContainsAny(ref, "\x00\n\r") {
		return "", errors.New("invalid revision")
	}
	out, err := g.git(ctx, 128, nil, "rev-parse", "--verify", "--end-of-options", ref+"^{commit}")
	if err != nil {
		return "", errors.New("revision does not resolve to a commit")
	}
	oid := strings.TrimSpace(string(out))
	if !validOID(oid, false) {
		return "", errors.New("invalid resolved commit")
	}
	return oid, nil
}

func (g gitSource) commitTree(ctx context.Context, commit string) (string, error) {
	out, err := g.git(ctx, 128, nil, "rev-parse", "--verify", "--end-of-options", commit+"^{tree}")
	if err != nil {
		return "", errors.New("cannot resolve commit tree")
	}
	tree := strings.TrimSpace(string(out))
	if !validOID(tree, false) {
		return "", errors.New("invalid resolved tree")
	}
	return tree, nil
}

func (g gitSource) emptyTree(ctx context.Context) (string, error) {
	out, err := g.git(ctx, 128, nil, "hash-object", "-w", "-t", "tree", "--stdin")
	if err != nil {
		return "", err
	}
	tree := strings.TrimSpace(string(out))
	if !validOID(tree, false) {
		return "", errors.New("invalid empty tree")
	}
	return tree, nil
}

func (g gitSource) headBase(ctx context.Context) (string, error) {
	commit, err := g.resolveCommit(ctx, "HEAD")
	if err == nil {
		return g.commitTree(ctx, commit)
	}
	// An unborn HEAD is valid; a broken or detached HEAD is not.
	if _, symErr := g.git(ctx, 256, nil, "symbolic-ref", "--quiet", "HEAD"); symErr != nil {
		return "", errors.New("HEAD is unavailable")
	}
	return g.emptyTree(ctx)
}

func (g gitSource) indexTree(ctx context.Context) (string, error) {
	out, err := g.git(ctx, 128, nil, "write-tree")
	if err != nil {
		return "", errors.New("cannot snapshot index, possibly unmerged")
	}
	tree := strings.TrimSpace(string(out))
	if !validOID(tree, false) {
		return "", errors.New("invalid index tree")
	}
	return tree, nil
}

func (g gitSource) staged(ctx context.Context, lim limits) (snapshot, error) {
	base, err := g.headBase(ctx)
	if err != nil {
		return snapshot{}, err
	}
	tree, err := g.indexTree(ctx)
	if err != nil {
		return snapshot{}, err
	}
	return g.capture(ctx, base, tree, "", "", lim)
}

func (g gitSource) revision(ctx context.Context, base, rev string, lim limits) (snapshot, error) {
	commit, err := g.resolveCommit(ctx, rev)
	if err != nil {
		return snapshot{}, err
	}
	tree, err := g.commitTree(ctx, commit)
	if err != nil {
		return snapshot{}, err
	}
	baseTree := ""
	if base == "" {
		baseTree, err = g.emptyTree(ctx)
	} else {
		var baseCommit string
		baseCommit, err = g.resolveCommit(ctx, base)
		if err == nil {
			baseTree, err = g.commitTree(ctx, baseCommit)
		}
	}
	if err != nil {
		return snapshot{}, err
	}
	ref := rev
	if validOID(rev, false) {
		ref = ""
	}
	return g.capture(ctx, baseTree, tree, commit, ref, lim)
}

func (g gitSource) capture(ctx context.Context, baseTree, tree, commit, ref string, lim limits) (snapshot, error) {
	s := snapshot{BaseTree: baseTree, Tree: tree, Revision: commit, Ref: ref}
	if lim.MaxFiles <= 0 || lim.MaxFileBytes <= 0 || lim.MaxTotalBytes <= 0 {
		return snapshot{}, errors.New("snapshot limits must be positive")
	}
	// --raw suppresses patch generation. -z makes all paths unambiguous.
	out, err := g.git(ctx, gitOutputCap, nil, "diff-tree", "--raw", "-z", "-r", "--no-commit-id", "--no-renames", "--no-ext-diff", "--no-textconv", "--abbrev=64", baseTree, tree)
	if err != nil {
		return snapshot{}, err
	}
	var total int64
	for len(out) > 0 {
		meta, rest, ok := cutNUL(out)
		if !ok || len(meta) == 0 || meta[0] != ':' {
			return snapshot{}, errors.New("invalid Git change record")
		}
		path, tail, ok := cutNUL(rest)
		if !ok || len(path) == 0 {
			return snapshot{}, errors.New("invalid Git change path")
		}
		out = tail
		if len(s.Changes) >= lim.MaxFiles {
			return snapshot{}, errors.New("snapshot file count exceeds limit")
		}
		fields := strings.Fields(string(meta[1:]))
		if len(fields) != 5 || len(fields[4]) != 1 || !strings.Contains("AMD T", fields[4]) {
			return snapshot{}, errors.New("unsupported Git change record")
		}
		c := change{Path: string(path)}
		if fields[0] != "000000" {
			c.BeforeMode, c.BeforeOID = fields[0], fields[2]
		}
		if fields[1] != "000000" {
			c.AfterMode, c.AfterOID = fields[1], fields[3]
		}
		if !validChangeMode(c.BeforeMode) || !validChangeMode(c.AfterMode) ||
			(c.BeforeOID != "" && !validOID(c.BeforeOID, false)) ||
			(c.AfterOID != "" && !validOID(c.AfterOID, false)) {
			return snapshot{}, errors.New("invalid Git object or mode")
		}
		if c.BeforeMode != "" && c.BeforeMode != "160000" {
			c.Before, err = g.blob(ctx, c.BeforeOID, lim.MaxFileBytes, &total, lim.MaxTotalBytes)
			if err != nil {
				return snapshot{}, err
			}
		}
		if c.AfterMode != "" && c.AfterMode != "160000" {
			c.After, err = g.blob(ctx, c.AfterOID, lim.MaxFileBytes, &total, lim.MaxTotalBytes)
			if err != nil {
				return snapshot{}, err
			}
		}
		s.Changes = append(s.Changes, c)
	}
	return s, nil
}

func cutNUL(data []byte) ([]byte, []byte, bool) {
	i := bytes.IndexByte(data, 0)
	if i < 0 {
		return nil, nil, false
	}
	return data[:i], data[i+1:], true
}

func validChangeMode(mode string) bool {
	switch mode {
	case "", "100644", "100755", "120000", "160000":
		return true
	}
	return false
}

func (g gitSource) blob(ctx context.Context, oid string, max int, total *int64, maxTotal int) ([]byte, error) {
	out, err := g.git(ctx, 128, nil, "cat-file", "-s", oid)
	if err != nil {
		return nil, errors.New("cannot size Git blob")
	}
	size, err := strconv.ParseInt(strings.TrimSpace(string(out)), 10, 64)
	if err != nil || size < 0 {
		return nil, errors.New("invalid Git blob size")
	}
	if size > int64(max) || size > int64(maxTotal)-*total {
		return nil, errors.New("snapshot blob bytes exceed limit")
	}
	data, err := g.git(ctx, int(size)+1, nil, "cat-file", "blob", oid)
	if err != nil || int64(len(data)) != size {
		return nil, errors.New("cannot read Git blob")
	}
	if data == nil {
		data = []byte{}
	} // A present empty blob is not missing content.
	*total += size
	return data, nil
}

// policyBlob reads one regular tracked blob from a committed revision. Its
// second return value is the resolved policy commit, not a moving ref name.
func (g gitSource) policyBlob(ctx context.Context, ref, path string) ([]byte, string, error) {
	if path == "" || filepath.IsAbs(path) || strings.ContainsAny(path, "\x00\n\r") ||
		path == ".." || strings.HasPrefix(path, "../") || strings.Contains(path, "/../") || strings.HasPrefix(path, "./") {
		return nil, "", errors.New("invalid policy path")
	}
	commit, err := g.resolveCommit(ctx, ref)
	if err != nil {
		return nil, "", err
	}
	out, err := g.git(ctx, 4096, nil, "ls-tree", "-z", "--full-tree", commit, "--", path)
	if err != nil {
		return nil, "", errors.New("cannot locate policy blob")
	}
	record, tail, ok := cutNUL(out)
	if !ok || len(tail) != 0 {
		return nil, "", errors.New("policy path is not one tracked file")
	}
	meta, foundPath, ok := bytes.Cut(record, []byte{'\t'})
	if !ok || string(foundPath) != path {
		return nil, "", errors.New("policy path is not one tracked file")
	}
	fields := strings.Fields(string(meta))
	if len(fields) != 3 || (fields[0] != "100644" && fields[0] != "100755") || fields[1] != "blob" || !validOID(fields[2], false) {
		return nil, "", errors.New("policy path is not a regular blob")
	}
	var total int64
	data, err := g.blob(ctx, fields[2], policyBlobCap, &total, policyBlobCap)
	if err != nil {
		return nil, "", err
	}
	return data, commit, nil
}

func (g gitSource) unchanged(ctx context.Context, s snapshot, staged bool) error {
	if staged {
		base, err := g.headBase(ctx)
		if err != nil {
			return err
		}
		tree, err := g.indexTree(ctx)
		if err != nil {
			return err
		}
		if base != s.BaseTree || tree != s.Tree {
			return errors.New("staged snapshot changed")
		}
		return nil
	}
	if s.Ref != "" {
		commit, err := g.resolveCommit(ctx, s.Ref)
		if err != nil || commit != s.Revision {
			return errors.New("revision changed")
		}
	}
	return nil
}

func parsePushUpdates(r io.Reader) ([]pushUpdate, error) {
	data, err := io.ReadAll(io.LimitReader(r, pushInputCap+1))
	if err != nil {
		return nil, errors.New("cannot read push updates")
	}
	if len(data) > pushInputCap {
		return nil, errors.New("push updates exceed size limit")
	}
	if len(data) == 0 {
		return nil, nil
	}
	if data[len(data)-1] != '\n' {
		return nil, errors.New("unterminated push update")
	}
	lines := bytes.Split(data[:len(data)-1], []byte{'\n'})
	if len(lines) > 32 {
		return nil, errors.New("too many push updates")
	}
	updates := make([]pushUpdate, 0, len(lines))
	for _, line := range lines {
		parts := bytes.Split(line, []byte{' '})
		if len(parts) != 4 {
			return nil, errors.New("invalid push update fields")
		}
		u := pushUpdate{string(parts[0]), string(parts[1]), string(parts[2]), string(parts[3])}
		if !validOID(u.LocalOID, true) || !validOID(u.RemoteOID, true) || len(u.LocalOID) != len(u.RemoteOID) ||
			!validPushRef(u.RemoteRef) || !(validPushRef(u.LocalRef) || u.LocalRef == "(delete)" ||
			(validOID(u.LocalRef, false) && strings.EqualFold(u.LocalRef, u.LocalOID))) ||
			(isZeroOID(u.LocalOID) != (u.LocalRef == "(delete)")) {
			return nil, errors.New("invalid push update")
		}
		updates = append(updates, u)
	}
	return updates, nil
}

func validOID(s string, allowZero bool) bool {
	if len(s) != 40 && len(s) != 64 {
		return false
	}
	for _, c := range s {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
			return false
		}
	}
	return allowZero || !isZeroOID(s)
}

func isZeroOID(s string) bool {
	for _, c := range s {
		if c != '0' {
			return false
		}
	}
	return true
}

func validPushRef(s string) bool {
	if s == "HEAD" {
		return true
	}
	if !strings.HasPrefix(s, "refs/") || strings.HasSuffix(s, "/") || strings.HasSuffix(s, ".") ||
		strings.Contains(s, "//") || strings.Contains(s, "..") || strings.Contains(s, "@{") {
		return false
	}
	for _, component := range strings.Split(s, "/") {
		if component == "" || strings.HasPrefix(component, ".") || strings.HasSuffix(component, ".lock") || strings.HasPrefix(component, "-") {
			return false
		}
	}
	for _, c := range s {
		if c <= 0x20 || c == 0x7f || strings.ContainsRune("~^:?*[\\", c) {
			return false
		}
	}
	return true
}
