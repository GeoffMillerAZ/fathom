//go:build !windows

package main

import (
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

func TestFIFODoesNotHang(t *testing.T) {
	root, s, p, q := fixture(t)
	if e := syscall.Mkfifo(filepath.Join(root, "pipe"), 0600); e != nil {
		t.Skip(e)
	}
	q.Files[0].Path = "pipe"
	done := make(chan report, 1)
	go func() { done <- run(root, s, p, q, "json") }()
	select {
	case r := <-done:
		if r.Status != "incomplete" {
			t.Fatal(r)
		}
	case <-time.After(time.Second):
		t.Fatal("FIFO open hung")
	}
}

func TestControlFIFODoesNotHang(t *testing.T) {
	name := filepath.Join(t.TempDir(), "control")
	if e := syscall.Mkfifo(name, 0600); e != nil {
		t.Skip(e)
	}
	done := make(chan error, 1)
	go func() { var s scope; done <- strictFile(name, &s) }()
	select {
	case e := <-done:
		if e == nil {
			t.Fatal("accepted FIFO")
		}
	case <-time.After(time.Second):
		t.Fatal("control FIFO open hung")
	}
}
