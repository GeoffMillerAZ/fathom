package worktree

// Immutable git facts requested by clone resolution. These process-local
// memos are keyed by immutable object ids, never working-tree contents.
import (
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

	"github.com/geoffmilleraz/fathom/internal/gitro"
)

type gitFactKey struct{ root, oid string }
type gitFact struct {
	ready chan struct{}
	data  []byte
	err   error
}

var repositoryRoots sync.Map

func gitFactOutput(ctx context.Context, memo *sync.Map, key gitFactKey, dir string, args ...string) ([]byte, error) {
	entry := &gitFact{ready: make(chan struct{})}
	actual, loaded := memo.LoadOrStore(key, entry)
	if loaded {
		entry = actual.(*gitFact)
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-entry.ready:
			return entry.data, entry.err
		}
	}
	entry.data, entry.err = boundedGitOutput(ctx, dir, args...)
	close(entry.ready)
	if entry.err != nil {
		memo.CompareAndDelete(key, entry)
	}
	return entry.data, entry.err
}

// Bound subprocess duration and output independently of repository size.
func boundedGitOutput(ctx context.Context, dir string, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	cmd, cleanup, err := gitro.Command(ctx, dir, args...)
	defer cleanup()
	if err != nil {
		return nil, err
	}
	var out gitOutputBuffer
	cmd.Stdout = &out
	err = cmd.Run()
	return out.buffer.Bytes(), err
}

type gitOutputBuffer struct{ buffer bytes.Buffer }

func (b *gitOutputBuffer) Write(p []byte) (int, error) {
	if b.buffer.Len()+len(p) > 32<<20 {
		return 0, fmt.Errorf("git output exceeds clone identity ceiling")
	}
	return b.buffer.Write(p)
}

type indexedBlob struct{ mode, oid string }
type commitTree struct {
	ready chan struct{}
	blobs map[string]indexedBlob
	err   error
}

var parsedTrees sync.Map

func indexedTree(ctx context.Context, root, commit string) (map[string]indexedBlob, error) {
	key := gitFactKey{canonical(root), commit}
	entry := &commitTree{ready: make(chan struct{})}
	actual, loaded := parsedTrees.LoadOrStore(key, entry)
	if loaded {
		entry = actual.(*commitTree)
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-entry.ready:
			return entry.blobs, entry.err
		}
	}
	data, err := boundedGitOutput(ctx, key.root, "ls-tree", "-r", "-z", commit)
	entry.err = err
	if err == nil {
		entry.blobs = map[string]indexedBlob{}
		for _, record := range bytes.Split(data, []byte{0}) {
			header, path, ok := strings.Cut(string(record), "\t")
			if !ok {
				continue
			}
			fields := strings.Fields(header)
			if len(fields) == 3 && fields[1] == "blob" {
				entry.blobs[path] = indexedBlob{fields[0], fields[2]}
			}
		}
	}
	close(entry.ready)
	if entry.err != nil {
		parsedTrees.CompareAndDelete(key, entry)
	}
	return entry.blobs, entry.err
}

// VisiblePaths is called once by the request owner. Only tracked paths in
// the service subtree are enumerated, expressed relative to its root.
func (r Resolution) VisiblePaths(ctx context.Context) (map[string]bool, error) {
	prefix, err := filepath.Rel(r.Checkout, r.Root)
	if err != nil || escapes(prefix) {
		return nil, fmt.Errorf("service root escapes resolved checkout")
	}
	pathspec := ":(top,literal)"
	if prefix != "." {
		pathspec = ":(top,literal)" + filepath.ToSlash(prefix) + "/"
	}
	data, err := boundedGitOutput(ctx, r.Checkout, "ls-files", "-z", "--cached", "--", pathspec)
	if err != nil {
		return nil, err
	}
	out := map[string]bool{}
	for _, path := range bytes.Split(data, []byte{0}) {
		if len(path) == 0 {
			continue
		}
		full := filepath.Join(r.Checkout, filepath.FromSlash(string(path)))
		if !within(r.Root, full) {
			continue
		}
		rel, err := filepath.Rel(r.Root, full)
		if err == nil {
			out[filepath.ToSlash(rel)] = true
		}
	}
	return out, nil
}

// UntrackedPath qualifies a path absent from VisiblePaths without reading
// its bytes. Ignored files and symlinks retain their existing refusals;
// errors never turn an unlisted path into a readable one.
func (r Resolution) UntrackedPath(ctx context.Context, rel string) bool {
	if filepath.IsAbs(rel) || escapes(filepath.Clean(rel)) {
		return false
	}
	full := filepath.Join(r.Root, filepath.FromSlash(rel))
	info, err := os.Lstat(full)
	if err != nil || !info.Mode().IsRegular() {
		return false
	}
	real, err := filepath.EvalSymlinks(full)
	if err != nil || !within(canonical(r.Root), real) {
		return false
	}
	_, err = boundedGitOutput(ctx, r.Root, "check-ignore", "--quiet", "--", rel)
	var exit *exec.ExitError
	return errors.As(err, &exit) && exit.ExitCode() == 1
}
