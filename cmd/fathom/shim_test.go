package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestInstallShim pins IO-20: one stable PATH name, deployed-binary
// preferred, never clobbering a foreign real file.
func TestInstallShim(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	t.Run("falls_back_to_running_executable", func(t *testing.T) {
		msg, err := installShim(nil)
		if err != nil {
			t.Fatalf("installShim: %v", err)
		}
		if !strings.Contains(msg, "installed") {
			t.Errorf("msg = %q", msg)
		}
		target, err := os.Readlink(filepath.Join(home, ".local", "bin", "fathom"))
		if err != nil || target == "" {
			t.Fatalf("shim not a symlink: %v", err)
		}
	})

	t.Run("idempotent", func(t *testing.T) {
		msg, err := installShim(nil)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(msg, "current") {
			t.Errorf("second install should be a no-op, got %q", msg)
		}
	})

	t.Run("prefers_deployed_binary", func(t *testing.T) {
		deployed := filepath.Join(home, ".local", "libexec", "fathom")
		os.MkdirAll(deployed, 0o755)
		os.WriteFile(filepath.Join(deployed, "fathom"), []byte("#!/bin/sh\n"), 0o755)
		if _, err := installShim(nil); err != nil {
			t.Fatal(err)
		}
		target, _ := os.Readlink(filepath.Join(home, ".local", "bin", "fathom"))
		if target != filepath.Join(deployed, "fathom") {
			t.Errorf("target = %q, want the deployed binary", target)
		}
	})

	t.Run("refuses_to_clobber_a_real_file", func(t *testing.T) {
		shim := filepath.Join(home, ".local", "bin", "fathom")
		os.Remove(shim)
		os.WriteFile(shim, []byte("someone else's binary"), 0o755)
		if _, err := installShim(nil); err == nil {
			t.Fatal("must refuse to clobber a non-symlink")
		}
	})
}
