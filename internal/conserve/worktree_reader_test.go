package conserve

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

type cancelAfterChecks struct{ checks, limit int }

func (*cancelAfterChecks) Deadline() (time.Time, bool) { return time.Time{}, false }
func (*cancelAfterChecks) Done() <-chan struct{}       { return nil }
func (c *cancelAfterChecks) Err() error {
	c.checks++
	if c.checks >= c.limit {
		return context.Canceled
	}
	return nil
}
func (*cancelAfterChecks) Value(any) any { return nil }

func TestReadWorktreeFileStopsBetweenChunks(t *testing.T) {
	root := t.TempDir()
	body := bytes.Repeat([]byte("x"), 1024*1024)
	if err := os.WriteFile(filepath.Join(root, "huge.txt"), body, 0o644); err != nil {
		t.Fatal(err)
	}
	ctx := &cancelAfterChecks{limit: 3}
	if b, ok, err := readWorktreeFile(ctx, root, "huge.txt"); !errors.Is(err, context.Canceled) || ok || b != nil {
		t.Fatalf("canceled read = %d bytes, ok=%v, err=%v", len(b), ok, err)
	}
	if got, ok, err := worktreeFile(root, "huge.txt"); err != nil || !ok || !bytes.Equal(got, body) {
		t.Fatalf("legacy read = %d bytes, ok=%v, err=%v", len(got), ok, err)
	}
}

func TestNewInputHeadReaderUsesRequestContext(t *testing.T) {
	repo := newInputTestRepo(t)
	repo.write("a.go", "package a\n")
	repo.commitAll("base")
	ctx, cancel := context.WithCancel(context.Background())
	in, err := NewInput(ctx, repo.root, repo.headSHA(), nil, "svc", nil)
	if err != nil {
		t.Fatal(err)
	}
	cancel()
	if b, ok, err := in.HeadFile("a.go"); !errors.Is(err, context.Canceled) || ok || b != nil {
		t.Fatalf("HeadFile after cancellation: %d bytes, ok=%v, err=%v", len(b), ok, err)
	}
}

// Legacy test helper. Production readers always carry the request context.
func worktreeFile(root, path string) ([]byte, bool, error) {
	return readWorktreeFile(context.Background(), root, path)
}
