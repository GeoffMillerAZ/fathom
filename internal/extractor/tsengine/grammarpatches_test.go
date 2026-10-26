package tsengine

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func patchFixture(t *testing.T, patches []string) (string, string, string) {
	t.Helper()
	dir := t.TempDir()
	lockDir := filepath.Join(dir, "grammars")
	src := filepath.Join(dir, "source")
	for _, p := range []string{lockDir, filepath.Join(lockDir, "patches"), src} {
		if err := os.MkdirAll(p, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(src, "grammar.txt"), []byte("first\nsecond\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	var entries []LockPatch
	for i, p := range patches {
		name := "patches/p" + string(rune('1'+i)) + ".patch"
		if err := os.WriteFile(filepath.Join(lockDir, name), []byte(p), 0o644); err != nil {
			t.Fatal(err)
		}
		sum := sha256.Sum256([]byte(p))
		entries = append(entries, LockPatch{Path: name, SHA256: hex.EncodeToString(sum[:])})
	}
	lock := map[string]any{"grammars": []map[string]any{{"blob": "demo", "licence": "MIT", "patches": entries}}}
	b, err := json.Marshal(lock)
	if err != nil {
		t.Fatal(err)
	}
	lockPath := filepath.Join(lockDir, "grammars.lock.json")
	if err := os.WriteFile(lockPath, b, 0o644); err != nil {
		t.Fatal(err)
	}
	return lockPath, src, lockDir
}

func runPatchScript(lock, src string) error {
	c := exec.Command("bash", "grammars/apply-patches.sh", lock, "demo", src)
	c.Env = append(os.Environ(), "GIT_DIR=", "GIT_WORK_TREE=")
	out, err := c.CombinedOutput()
	if err != nil {
		return &patchError{err, string(out)}
	}
	return nil
}

type patchError struct {
	err    error
	output string
}

func (e *patchError) Error() string { return e.err.Error() + ": " + e.output }

func TestApplyPatchesOrderedAndStrict(t *testing.T) {
	p1 := "diff --git a/grammar.txt b/grammar.txt\n--- a/grammar.txt\n+++ b/grammar.txt\n@@ -1,2 +1,2 @@\n-first\n+one\n second\n"
	p2 := "diff --git a/grammar.txt b/grammar.txt\n--- a/grammar.txt\n+++ b/grammar.txt\n@@ -1,2 +1,2 @@\n one\n-second\n+two\n"
	lock, src, _ := patchFixture(t, []string{p1, p2})
	if err := runPatchScript(lock, src); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(src, "grammar.txt"))
	if err != nil || string(b) != "one\ntwo\n" {
		t.Fatalf("ordered patch: %q %v", b, err)
	}
	if err := runPatchScript(lock, src); err == nil {
		t.Fatal("reapplying must fail")
	}

	for _, tc := range []struct {
		name   string
		change func(string, string, string) error
	}{
		{"wrong hash", func(lock, _, _ string) error {
			b, e := os.ReadFile(lock)
			if e != nil {
				return e
			}
			h := sha256.Sum256([]byte(p1))
			return os.WriteFile(lock, []byte(strings.Replace(string(b), hex.EncodeToString(h[:]), strings.Repeat("0", 64), 1)), 0o644)
		}},
		{"missing", func(_, _, dir string) error { return os.Remove(filepath.Join(dir, "patches/p1.patch")) }},
		{"symlink", func(_, _, dir string) error {
			p := filepath.Join(dir, "patches/p1.patch")
			if e := os.Remove(p); e != nil {
				return e
			}
			return os.Symlink(filepath.Join(dir, "grammars.lock.json"), p)
		}},
		{"symlink directory", func(_, _, dir string) error {
			p := filepath.Join(dir, "patches")
			if err := os.Rename(p, p+"-real"); err != nil {
				return err
			}
			return os.Symlink(p+"-real", p)
		}},
		{"MPL", func(lock, _, _ string) error {
			b, e := os.ReadFile(lock)
			if e != nil {
				return e
			}
			return os.WriteFile(lock, []byte(strings.Replace(string(b), `"licence":"MIT"`, `"licence":"MPL-2.0"`, 1)), 0o644)
		}},
		{"malformed patches object", func(lock, _, _ string) error {
			return os.WriteFile(lock, []byte(`{"grammars":[{"blob":"demo","licence":"MIT","patches":{}}]}`), 0o644)
		}},
		{"false patches", func(lock, _, _ string) error {
			return os.WriteFile(lock, []byte(`{"grammars":[{"blob":"demo","licence":"MIT","patches":false}]}`), 0o644)
		}},
		{"duplicate blob rows", func(lock, _, _ string) error {
			return os.WriteFile(lock, []byte(`{"grammars":[{"blob":"demo","licence":"MIT"},{"blob":"demo","licence":"MIT"}]}`), 0o644)
		}},
		{"malformed patch entry", func(lock, _, _ string) error {
			return os.WriteFile(lock, []byte(`{"grammars":[{"blob":"demo","licence":"MIT","patches":[{}]}]}`), 0o644)
		}},
		{"traversal", func(lock, _, _ string) error {
			b, e := os.ReadFile(lock)
			if e != nil {
				return e
			}
			return os.WriteFile(lock, []byte(strings.Replace(string(b), "patches/p1.patch", "patches/../p1.patch", 1)), 0o644)
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			l, s, d := patchFixture(t, []string{p1})
			if err := tc.change(l, s, d); err != nil {
				t.Fatal(err)
			}
			if err := runPatchScript(l, s); err == nil {
				t.Fatal("expected rejection")
			}
		})
	}
	for _, p := range []string{p2, strings.Replace(p1, "-first\n+one", "-one\n+first", 1)} {
		l, s, _ := patchFixture(t, []string{p})
		if err := runPatchScript(l, s); err == nil {
			t.Fatalf("nonapplicable patch accepted: %q", p)
		}
	}
}

func TestPatchSourceInsideAncestorGitRepo(t *testing.T) {
	p := "diff --git a/grammar.txt b/grammar.txt\n--- a/grammar.txt\n+++ b/grammar.txt\n@@ -1,2 +1,2 @@\n-first\n+one\n second\n"
	lock, src, _ := patchFixture(t, []string{p})
	ancestor := filepath.Dir(filepath.Dir(lock))
	if out, err := exec.Command("git", "-C", ancestor, "init", "-q").CombinedOutput(); err != nil {
		t.Fatalf("git init: %v: %s", err, out)
	}
	parentFile := filepath.Join(ancestor, "grammar.txt")
	if err := os.WriteFile(parentFile, []byte("first\nsecond\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := runPatchScript(lock, src); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(filepath.Join(src, "grammar.txt"))
	if err != nil || string(got) != "one\nsecond\n" {
		t.Fatalf("source: %q %v", got, err)
	}
	parent, err := os.ReadFile(parentFile)
	if err != nil || string(parent) != "first\nsecond\n" {
		t.Fatalf("parent changed: %q %v", parent, err)
	}
}

func TestBuildPatchHookOrder(t *testing.T) {
	b, err := os.ReadFile("grammars/build.sh")
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	a := strings.Index(s, "check_licence \"$WORK/grammar\"")
	p := strings.Index(s, "apply-patches.sh")
	g := strings.Index(s, "if [ \"$(field .generate)\" = \"true\" ]")
	if a < 0 || p < a || g < p {
		t.Fatalf("patch hook outside verified-fetch / generate interval: %d %d %d", a, p, g)
	}
}
