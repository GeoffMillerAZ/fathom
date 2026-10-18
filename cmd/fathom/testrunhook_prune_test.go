package main

// testrunhook_prune_test.go — RO-36 (docs/plans/readouts.md): the saved
// outputs under workspace/tmp/test-run keep at most 50 files and 8 MiB, the
// oldest removed first, only the hook's own <16 hex>.log files (I63), and a
// failed prune never fails the hook (I9).

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func seedSaved(t *testing.T, dir, name string, size int, age time.Duration) {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(strings.Repeat("x", size)), 0o644); err != nil {
		t.Fatal(err)
	}
	at := time.Now().Add(-age)
	if err := os.Chtimes(p, at, at); err != nil {
		t.Fatal(err)
	}
}

func savedNames(t *testing.T, dir string) map[string]bool {
	t.Helper()
	ents, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]bool{}
	for _, e := range ents {
		out[e.Name()] = true
	}
	return out
}

func TestTestRunSavedOutputIsCountAndByteBounded(t *testing.T) {
	t.Run("count: 50 files, oldest removed first", func(t *testing.T) {
		repo := t.TempDir()
		dir := filepath.Join(repo, filepath.FromSlash(testRunSavedDir))
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		for i := 0; i < 60; i++ {
			seedSaved(t, dir, fmt.Sprintf("%016x.log", i), 10, time.Duration(100-i)*time.Hour)
		}
		rel := saveTestRunOutput(repo, "a fresh failing run")
		if rel == "" {
			t.Fatal("saveTestRunOutput returned no path")
		}
		got := savedNames(t, dir)
		if len(got) != 50 {
			t.Errorf("%d files kept, want 50", len(got))
		}
		if !got[filepath.Base(rel)] {
			t.Errorf("the file just written (%s) was pruned", rel)
		}
		for i := 0; i < 11; i++ { // 61 files, 11 oldest go
			if got[fmt.Sprintf("%016x.log", i)] {
				t.Errorf("oldest file %d survived", i)
			}
		}
		if !got[fmt.Sprintf("%016x.log", 11)] || !got[fmt.Sprintf("%016x.log", 59)] {
			t.Errorf("a newer file was removed: %v", got)
		}
	})

	t.Run("bytes: 8 MiB, oldest first, newest kept", func(t *testing.T) {
		repo := t.TempDir()
		dir := filepath.Join(repo, filepath.FromSlash(testRunSavedDir))
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		const mib = 1 << 20
		for i := 0; i < 9; i++ { // 9 MiB
			seedSaved(t, dir, fmt.Sprintf("%016x.log", i), mib, time.Duration(100-i)*time.Hour)
		}
		rel := saveTestRunOutput(repo, "small")
		got := savedNames(t, dir)
		if got[fmt.Sprintf("%016x.log", 0)] {
			t.Errorf("the oldest file survived: %v", got)
		}
		var total int64
		for n := range got {
			fi, _ := os.Stat(filepath.Join(dir, n))
			total += fi.Size()
		}
		if total > 8*mib {
			t.Errorf("%d bytes kept, want at most %d", total, 8*mib)
		}
		if !got[filepath.Base(rel)] || !got[fmt.Sprintf("%016x.log", 8)] {
			t.Errorf("a newer file was removed: %v", got)
		}
	})

	t.Run("only the hook's own pattern is touched", func(t *testing.T) {
		repo := t.TempDir()
		dir := filepath.Join(repo, filepath.FromSlash(testRunSavedDir))
		if err := os.MkdirAll(filepath.Join(dir, "sub"), 0o755); err != nil {
			t.Fatal(err)
		}
		others := []string{"notes.txt", "0123456789abcde.log", "0123456789abcdef0.log", "0123456789ABCDEF.log", "0123456789abcdef.log.bak"}
		for _, n := range others {
			seedSaved(t, dir, n, 9<<20, 1000*time.Hour)
		}
		target := filepath.Join(t.TempDir(), "outside.log")
		if err := os.WriteFile(target, []byte("keep"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(target, filepath.Join(dir, "00000000000000aa.log")); err != nil {
			t.Fatal(err)
		}
		for i := 0; i < 55; i++ {
			seedSaved(t, dir, fmt.Sprintf("%016x.log", 0x1000+i), 10, time.Duration(10+i)*time.Hour)
		}
		saveTestRunOutput(repo, "fresh")
		got := savedNames(t, dir)
		for _, n := range others {
			if !got[n] {
				t.Errorf("%s, not the hook's pattern, was removed", n)
			}
		}
		if !got["sub"] {
			t.Error("a directory was removed")
		}
		if b, err := os.ReadFile(target); err != nil || string(b) != "keep" {
			t.Errorf("a symlink's target was touched: %q %v", b, err)
		}
	})

	t.Run("a failed prune never fails the save", func(t *testing.T) {
		repo := t.TempDir()
		dir := filepath.Join(repo, filepath.FromSlash(testRunSavedDir))
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		for i := 0; i < 60; i++ {
			seedSaved(t, dir, fmt.Sprintf("%016x.log", i), 10, time.Duration(100-i)*time.Hour)
		}
		if err := os.Chmod(dir, 0o555); err != nil { // listable, not deletable from
			t.Fatal(err)
		}
		defer os.Chmod(dir, 0o755)
		if os.Getuid() == 0 {
			t.Skip("root ignores directory modes")
		}
		pruneTestRunSaved(dir, "")                           // must return, not panic
		pruneTestRunSaved(filepath.Join(repo, "absent"), "") // a missing dir too
		if n := len(savedNames(t, dir)); n != 60 {
			t.Errorf("%d files after a prune that could remove none, want 60", n)
		}
	})
}
