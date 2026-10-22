//go:build darwin || linux

package conserve

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
)

func TestReadWorktreeFileRefusesFIFOWithoutOpening(t *testing.T) {
	root := t.TempDir()
	fifo := filepath.Join(root, "pipe")
	if err := syscall.Mkfifo(fifo, 0o600); err != nil {
		t.Fatal(err)
	}
	if b, ok, err := readWorktreeFile(context.Background(), root, "pipe"); err == nil || ok || b != nil || !strings.Contains(err.Error(), "non-regular") {
		t.Fatalf("FIFO read = %d bytes, ok=%v, err=%v", len(b), ok, err)
	}
	if err := os.Symlink("pipe", filepath.Join(root, "pipe-link")); err != nil {
		t.Fatal(err)
	}
	if b, ok, err := readWorktreeFile(context.Background(), root, "pipe-link"); err == nil || ok || b != nil || !strings.Contains(err.Error(), "non-regular") {
		t.Fatalf("FIFO symlink read = %d bytes, ok=%v, err=%v", len(b), ok, err)
	}
	if b, ok, err := readWorktreeFile(context.Background(), root, "."); err != nil || ok || b != nil {
		t.Fatalf("directory read = %d bytes, ok=%v, err=%v", len(b), ok, err)
	}
	if err := os.WriteFile(filepath.Join(root, "regular"), []byte("content"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("regular", filepath.Join(root, "regular-link")); err != nil {
		t.Fatal(err)
	}
	if b, ok, err := readWorktreeFile(context.Background(), root, "regular-link"); err != nil || !ok || string(b) != "content" {
		t.Fatalf("regular symlink read = %q, ok=%v, err=%v", b, ok, err)
	}
}

func TestReadWorktreeGeneratedRegularGuardAndCancellation(t *testing.T) {
	root := t.TempDir()
	if err := syscall.Mkfifo(filepath.Join(root, "pipe"), 0o600); err != nil {
		t.Fatal(err)
	}
	if generated, ok, err := readWorktreeGenerated(context.Background(), root, "pipe"); err == nil || ok || generated || !strings.Contains(err.Error(), "non-regular") {
		t.Fatalf("FIFO classification = %v, %v, %v", generated, ok, err)
	}
	if err := os.WriteFile(filepath.Join(root, "asset.png"), []byte(strings.Repeat("x", 4<<20)), 0o600); err != nil {
		t.Fatal(err)
	}
	if generated, ok, err := readWorktreeGenerated(context.Background(), root, "asset.png"); err != nil || !ok || generated {
		t.Fatalf("asset classification = %v, %v, %v", generated, ok, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if generated, ok, err := readWorktreeGenerated(ctx, root, "asset.png"); err == nil || ok || generated {
		t.Fatalf("canceled classification = %v, %v, %v", generated, ok, err)
	}
}
