package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/connector"
)

func TestSeedUserIgnoreFile_WritesDefaults(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	var out bytes.Buffer
	if err := seedUserIgnoreFile(nil, &out); err != nil {
		t.Fatal(err)
	}

	path := filepath.Join(home, ".fathom", "ignore")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("expected %s to be written: %v", path, err)
	}
	for _, pattern := range connector.DefaultUserIgnoreSeed {
		if !strings.Contains(string(data), pattern) {
			t.Errorf("expected written file to contain %q", pattern)
		}
		if !strings.Contains(out.String(), pattern) {
			t.Errorf("expected printed output to contain %q (must print exactly what it writes)", pattern)
		}
	}
}

func TestSeedUserIgnoreFile_NeverOverwrites(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	dir := filepath.Join(home, ".fathom")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	custom := "# my own custom ignore\ncustom-pattern/\n"
	if err := os.WriteFile(filepath.Join(dir, "ignore"), []byte(custom), 0o644); err != nil {
		t.Fatal(err)
	}

	var out bytes.Buffer
	if err := seedUserIgnoreFile(nil, &out); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(dir, "ignore"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != custom {
		t.Errorf("expected the existing file to be untouched, got %q", data)
	}
	if !strings.Contains(out.String(), "already present") {
		t.Errorf("expected an 'already present' message, got %q", out.String())
	}
}

// TestCmdInit_SeedUserIgnore_RequiresFlag proves init never touches
// ~/.fathom/ignore unless --seed-user-ignore is passed explicitly.
func TestCmdInit_SeedUserIgnore_RequiresFlag(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	root := t.TempDir()

	var out, errBuf bytes.Buffer
	// init will report exitCode 2 (daemon unreachable) — that's fine, this
	// test only asserts the ignore file itself is untouched.
	run([]string{"init", "--path=" + root, "--url=http://127.0.0.1:1"}, &out, &errBuf)

	if _, err := os.Stat(filepath.Join(home, ".fathom", "ignore")); err == nil {
		t.Fatal("expected ~/.fathom/ignore to NOT be created without --seed-user-ignore")
	}
}

func TestCmdInit_SeedUserIgnore_WritesWhenFlagged(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	root := t.TempDir()

	var out, errBuf bytes.Buffer
	run([]string{"init", "--path=" + root, "--url=http://127.0.0.1:1", "--seed-user-ignore"}, &out, &errBuf)

	if _, err := os.Stat(filepath.Join(home, ".fathom", "ignore")); err != nil {
		t.Fatalf("expected ~/.fathom/ignore to be created with --seed-user-ignore: %v", err)
	}
}
