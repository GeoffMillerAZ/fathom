//go:build unix

package adoption

import (
	"context"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

func TestOpenAdoptionFIFOWithoutWriterDoesNotBlock(t *testing.T) {
	directory := t.TempDir()
	name := filepath.Join(directory, "fifo")
	if err := syscall.Mkfifo(name, 0600); err != nil {
		t.Fatal(err)
	}
	root, err := os.OpenRoot(directory)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	type opened struct {
		file *os.File
		err  error
	}
	done := make(chan opened, 1)
	go func() {
		file, err := openAdoptionFile(root, "fifo")
		done <- opened{file, err}
	}()
	var result opened
	select {
	case result = <-done:
	case <-time.After(2 * time.Second):
		// A regressed blocking O_RDONLY open must not leave a goroutine behind
		// after reporting failure. O_RDWR|O_NONBLOCK supplies the missing writer
		// without blocking this cleanup, releasing that pending read-only open.
		release, releaseErr := os.OpenFile(name, os.O_RDWR|syscall.O_NONBLOCK, 0)
		if releaseErr != nil {
			t.Errorf("could not release regressed FIFO open: %v", releaseErr)
		} else {
			defer release.Close()
		}
		select {
		case result = <-done:
			if result.file != nil {
				result.file.Close()
			}
		case <-time.After(2 * time.Second):
			t.Error("FIFO open did not stop after supplying a cleanup writer")
		}
		t.Fatal("opening a FIFO without a writer blocked; nonblocking open is required before descriptor type checks")
	}
	if result.err != nil || result.file == nil {
		t.Fatalf("nonblocking FIFO open: %v", result.err)
	}
	defer result.file.Close()
	info, err := result.file.Stat()
	if err != nil || info.Mode()&os.ModeNamedPipe == 0 || info.Mode().IsRegular() {
		t.Fatalf("opened descriptor is not the expected FIFO: %v", err)
	}
}

func TestCheckStaticFIFORejected(t *testing.T) {
	root, selection := adoptionCheckFixture(t)
	if err := syscall.Mkfifo(filepath.Join(root, "fifo"), 0600); err != nil {
		t.Fatal(err)
	}
	selection.Artifacts = []Artifact{{Path: "fifo", SHA256: adoptionCheckHash(nil)}}
	adoptionCheckSave(t, root, selection)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	report := Check(ctx, root, DefaultManifest)
	adoptionCheckOutcome(t, report, 1, "failed")
	adoptionCheckFinding(t, report, "artifact:1", "fail")
	for _, file := range report.Files {
		if file.Path == "fifo" {
			t.Fatal("FIFO became regular-file evidence")
		}
	}
}
