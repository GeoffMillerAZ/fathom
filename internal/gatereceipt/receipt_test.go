package gatereceipt

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const passingLog = "fathom gate: validate START 2026-10-22T01:00:00Z\nchecks passed\nfathom gate: validate PASS 2026-10-22T02:00:00Z\n"

func gitTest(t *testing.T, root string, args ...string) string {
	t.Helper()
	c := exec.CommandContext(context.Background(), "git", args...)
	c.Dir = root
	c.Env = append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL="+os.DevNull, "GIT_AUTHOR_NAME=Fixture", "GIT_AUTHOR_EMAIL=fixture@example.test", "GIT_COMMITTER_NAME=Fixture", "GIT_COMMITTER_EMAIL=fixture@example.test")
	b, err := c.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v: %s", args, err, b)
	}
	return strings.TrimSpace(string(b))
}

func put(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0600); err != nil {
		t.Fatal(err)
	}
}

func fixture(t *testing.T) Options {
	t.Helper()
	root := t.TempDir()
	gitTest(t, root, "init", "-q")
	put(t, filepath.Join(root, "Taskfile.yml"), "version: '3'\n")
	put(t, filepath.Join(root, "lefthook.yml"), "pre-push: {}\n")
	gitTest(t, root, "add", ".")
	gitTest(t, root, "-c", "core.hooksPath="+os.DevNull, "commit", "-qm", "fixture")
	log := filepath.Join(t.TempDir(), "gate.log")
	put(t, log, passingLog)
	return Options{Root: root, Dir: t.TempDir(), Log: log, Command: "validate", GoVersion: "go version fixture", FathomVersion: "fixture", OS: "fixture-os", Arch: "fixture-arch"}
}

func TestWriteDeterministicAndCopiesLog(t *testing.T) {
	o := fixture(t)
	r, id, err := Write(context.Background(), o)
	if err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(o.Dir, id+".json"))
	if err != nil {
		t.Fatal(err)
	}
	_, id2, err := Write(context.Background(), o)
	if err != nil || id2 != id {
		t.Fatalf("non deterministic: %s %s %v", id, id2, err)
	}
	b2, _ := os.ReadFile(filepath.Join(o.Dir, id2+".json"))
	if !bytes.Equal(b, b2) {
		t.Fatal("receipt changed")
	}
	log, err := os.ReadFile(filepath.Join(o.Dir, id+".log"))
	if err != nil || string(log) != passingLog {
		t.Fatalf("log: %q %v", log, err)
	}
	if r.Tree != gitTest(t, o.Root, "rev-parse", "HEAD^{tree}") || r.Commit != gitTest(t, o.Root, "rev-parse", "HEAD") || r.HostLabel != "unnamed" || r.Exit != 0 || r.Dirty || r.TimeSource != "log" {
		t.Fatalf("receipt: %+v", r)
	}
	if r.Start != "2026-10-22T01:00:00Z" || r.End != "2026-10-22T02:00:00Z" || r.LogBytes != int64(len(passingLog)) {
		t.Fatalf("metadata: %+v", r)
	}
}

func TestWriteRefusesDirtyTrees(t *testing.T) {
	for _, kind := range []string{"tracked", "untracked", "staged", "rename"} {
		t.Run(kind, func(t *testing.T) {
			o := fixture(t)
			switch kind {
			case "tracked":
				put(t, filepath.Join(o.Root, "Taskfile.yml"), "changed\n")
			case "untracked":
				put(t, filepath.Join(o.Root, "new file"), "new\n")
			case "staged":
				put(t, filepath.Join(o.Root, "Taskfile.yml"), "changed\n")
				gitTest(t, o.Root, "add", ".")
			case "rename":
				gitTest(t, o.Root, "mv", "Taskfile.yml", "renamed.yml")
			}
			_, _, err := Write(context.Background(), o)
			if err == nil || !strings.Contains(err.Error(), "dirty (1 path") {
				t.Fatalf("want dirty count, got %v", err)
			}
			entries, _ := os.ReadDir(o.Dir)
			if len(entries) != 0 {
				t.Fatal("wrote evidence for dirty tree")
			}
		})
	}
}

func TestWriteFailureAndMtimeFallback(t *testing.T) {
	o := fixture(t)
	put(t, o.Log, "checks failed\nfathom gate: validate FAIL\n")
	stamp := time.Date(2026, 10, 22, 3, 0, 0, 0, time.UTC)
	if err := os.Chtimes(o.Log, stamp, stamp); err != nil {
		t.Fatal(err)
	}
	o.HostLabel = "helper"
	r, _, err := Write(context.Background(), o)
	if err != nil {
		t.Fatal(err)
	}
	if r.Exit != 1 || r.TimeSource != "file-mtime" || r.Start != stamp.Format(time.RFC3339Nano) || r.End != r.Start || r.HostLabel != "helper" {
		t.Fatalf("receipt: %+v", r)
	}
	put(t, o.Log, "ok\n")
	if _, _, err := Write(context.Background(), o); err == nil {
		t.Fatal("accepted an unfinished log")
	}
}

func TestWriteRecordsFailureExitStatus(t *testing.T) {
	o := fixture(t)
	put(t, o.Log, "fathom gate: validate START 2026-10-22T01:00:00Z\nfathom gate: validate FAIL exit=7 2026-10-22T02:00:00Z\n")
	r, _, err := Write(context.Background(), o)
	if err != nil || r.Exit != 7 {
		t.Fatalf("exit: %+v %v", r, err)
	}
}

func TestWriteFinishedTaskFailureLog(t *testing.T) {
	o := fixture(t)
	put(t, o.Log, "fathom gate: validate FAIL exit=7 2026-10-22T02:00:00Z\ntask: Failed to run task \"validate\": exit status 7\n")
	r, _, err := Write(context.Background(), o)
	if err != nil || r.Exit != 7 {
		t.Fatalf("task failure: %+v %v", r, err)
	}
}
