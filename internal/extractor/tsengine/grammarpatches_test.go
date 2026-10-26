package tsengine

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
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
	for _, p := range []string{lockDir, filepath.Join(lockDir, "patches"), src, filepath.Join(src, "src")} {
		if err := os.MkdirAll(p, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(src, "src/parser.c"), []byte("first\nsecond\n"), 0o644); err != nil {
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
	b, err := os.ReadFile(lock)
	if err != nil {
		return err
	}
	var parsed struct {
		Grammars []json.RawMessage `json:"grammars"`
	}
	if err := json.Unmarshal(b, &parsed); err != nil {
		return err
	}
	if len(parsed.Grammars) != 1 {
		return fmt.Errorf("expected one fixture row, got %d", len(parsed.Grammars))
	}
	c := exec.Command("bash", "grammars/apply-patches.sh", lock, string(parsed.Grammars[0]), src)
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
	p1 := "diff --git a/src/parser.c b/src/parser.c\n--- a/src/parser.c\n+++ b/src/parser.c\n@@ -1,2 +1,2 @@\n-first\n+one\n second\n"
	p2 := "diff --git a/src/parser.c b/src/parser.c\n--- a/src/parser.c\n+++ b/src/parser.c\n@@ -1,2 +1,2 @@\n one\n-second\n+two\n"
	lock, src, _ := patchFixture(t, []string{p1, p2})
	if err := runPatchScript(lock, src); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(src, "src/parser.c"))
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
	p := "diff --git a/src/parser.c b/src/parser.c\n--- a/src/parser.c\n+++ b/src/parser.c\n@@ -1,2 +1,2 @@\n-first\n+one\n second\n"
	lock, src, _ := patchFixture(t, []string{p})
	ancestor := filepath.Dir(filepath.Dir(lock))
	if out, err := exec.Command("git", "-C", ancestor, "init", "-q").CombinedOutput(); err != nil {
		t.Fatalf("git init: %v: %s", err, out)
	}
	parentFile := filepath.Join(ancestor, "src/parser.c")
	if err := os.MkdirAll(filepath.Dir(parentFile), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(parentFile, []byte("first\nsecond\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := runPatchScript(lock, src); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(filepath.Join(src, "src/parser.c"))
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
	if err := checkBuildPatchHook(string(b)); err != nil {
		t.Fatal(err)
	}
	for _, mutation := range []struct{ name, old, replacement string }{
		{"commented", `bash "$HERE/apply-patches.sh"`, `# bash "$HERE/apply-patches.sh"`},
		{"guarded", `bash "$HERE/apply-patches.sh" "$LOCK" "$ROW" "$WORK/grammar"`, `bash "$HERE/apply-patches.sh" "$LOCK" "$ROW" "$WORK/grammar" || true`},
		{"pre licence removed", "check_licence \"$WORK/grammar\" \"$(field .licence)\"\nbash", "bash"},
		{"post licence removed", "check_licence \"$WORK/grammar\" \"$(field .licence)\"\n\nSUBDIRS", "\nSUBDIRS"},
	} {
		t.Run(mutation.name, func(t *testing.T) {
			changed := strings.Replace(string(b), mutation.old, mutation.replacement, 1)
			if changed == string(b) {
				t.Fatal("mutation did not apply")
			}
			if err := checkBuildPatchHook(changed); err == nil {
				t.Fatal("mutation survived")
			}
		})
	}
}

func checkBuildPatchHook(s string) error {
	lines := strings.Split(s, "\n")
	hook := `bash "$HERE/apply-patches.sh" "$LOCK" "$ROW" "$WORK/grammar"`
	for i, line := range lines {
		if line != hook {
			continue
		}
		if i < 1 || i+1 >= len(lines) || lines[i-1] != `check_licence "$WORK/grammar" "$(field .licence)"` || lines[i+1] != `check_licence "$WORK/grammar" "$(field .licence)"` {
			return fmt.Errorf("patch hook lacks adjacent licence checks")
		}
		if strings.Index(s, hook) > strings.Index(s, `if [ "$(field .generate)" = "true" ]`) {
			return fmt.Errorf("patch hook after generation")
		}
		return nil
	}
	return fmt.Errorf("exact unguarded patch hook missing")
}

func TestPatchIgnoresHostileGitConfig(t *testing.T) {
	p := "diff --git a/src/parser.c b/src/parser.c\n--- a/src/parser.c\n+++ b/src/parser.c\n@@ -1,2 +1,2 @@\n-first\n+one   \n second\n"
	lock, src, _ := patchFixture(t, []string{p})
	config := filepath.Join(t.TempDir(), "gitconfig")
	if err := os.WriteFile(config, []byte("[apply]\n\twhitespace = fix\n[core]\n\tautocrlf = true\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GIT_CONFIG_GLOBAL", config)
	t.Setenv("GIT_CONFIG_SYSTEM", config)
	t.Setenv("GIT_CONFIG_COUNT", "2")
	t.Setenv("GIT_CONFIG_KEY_0", "apply.whitespace")
	t.Setenv("GIT_CONFIG_VALUE_0", "fix")
	t.Setenv("GIT_CONFIG_KEY_1", "core.autocrlf")
	t.Setenv("GIT_CONFIG_VALUE_1", "true")
	t.Setenv("GIT_CONFIG_PARAMETERS", "'apply.whitespace=fix' 'core.autocrlf=true'")
	if err := runPatchScript(lock, src); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(src, "src/parser.c"))
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != "one   \nsecond\n" {
		t.Fatalf("host config changed source bytes: %q", b)
	}
}

func TestPatchRefusesUnsafeTargets(t *testing.T) {
	for _, tc := range []struct {
		name, patch string
		generate    bool
	}{
		{"licence edit", "diff --git a/LICENSE b/LICENSE\n--- a/LICENSE\n+++ b/LICENSE\n@@ -1 +1 @@\n-MIT\n+MIT changed\n", false},
		{"licence addition", "diff --git a/LICENSE.extra b/LICENSE.extra\nnew file mode 100644\n--- /dev/null\n+++ b/LICENSE.extra\n@@ -0,0 +1 @@\n+MIT\n", false},
		{"notice addition", "diff --git a/NOTICE.txt b/NOTICE.txt\nnew file mode 100644\n--- /dev/null\n+++ b/NOTICE.txt\n@@ -0,0 +1 @@\n+attribution\n", false},
		{"copyright addition", "diff --git a/COPYRIGHT b/COPYRIGHT\nnew file mode 100644\n--- /dev/null\n+++ b/COPYRIGHT\n@@ -0,0 +1 @@\n+attribution\n", false},
		{"uncompiled README", "diff --git a/README.md b/README.md\nnew file mode 100644\n--- /dev/null\n+++ b/README.md\n@@ -0,0 +1 @@\n+changed\n", false},
		{"case alias grammar", "diff --git a/GRAMMAR.JS b/GRAMMAR.JS\nnew file mode 100644\n--- /dev/null\n+++ b/GRAMMAR.JS\n@@ -0,0 +1 @@\n+module.exports = {};\n", false},
		{"case alias generated", "diff --git a/src/PARSER.C b/src/PARSER.C\nnew file mode 100644\n--- /dev/null\n+++ b/src/PARSER.C\n@@ -0,0 +1 @@\n+int x;\n", true},
		{"symlink", "diff --git a/link b/link\nnew file mode 120000\n--- /dev/null\n+++ b/link\n@@ -0,0 +1 @@\n+../outside\n", false},
		{"mode change", "diff --git a/src/parser.c b/src/parser.c\nold mode 100644\nnew mode 100755\n", false},
		{"rename", "diff --git a/src/parser.c b/moved.txt\nsimilarity index 100%\nrename from src/parser.c\nrename to moved.txt\n", false},
		{"copy", "diff --git a/src/parser.c b/copied.txt\nsimilarity index 100%\ncopy from src/parser.c\ncopy to copied.txt\n", false},
		{"binary", "diff --git a/src/parser.c b/src/parser.c\nindex 1111111..2222222 100644\nBinary files a/src/parser.c and b/src/parser.c differ\n", false},
		{"generated parser", "diff --git a/src/parser.c b/src/parser.c\nnew file mode 100644\n--- /dev/null\n+++ b/src/parser.c\n@@ -0,0 +1 @@\n+int x;\n", true},
		{"generated grammar json", "diff --git a/src/grammar.json b/src/grammar.json\nnew file mode 100644\n--- /dev/null\n+++ b/src/grammar.json\n@@ -0,0 +1 @@\n+{}\n", true},
		{"generated node types", "diff --git a/src/node-types.json b/src/node-types.json\nnew file mode 100644\n--- /dev/null\n+++ b/src/node-types.json\n@@ -0,0 +1 @@\n+[]\n", true},
		{"uncompiled grammar", "diff --git a/grammar.js b/grammar.js\nnew file mode 100644\n--- /dev/null\n+++ b/grammar.js\n@@ -0,0 +1 @@\n+module.exports = {};\n", false},
		{"outside", "diff --git a/../outside b/../outside\n--- a/../outside\n+++ b/../outside\n@@ -1 +1 @@\n-old\n+new\n", false},
		{"noncanonical", "diff --git a/./src/parser.c b/./src/parser.c\n--- a/./src/parser.c\n+++ b/./src/parser.c\n@@ -1,2 +1,2 @@\n-first\n+one\n second\n", false},
		{"same bytes", "diff --git a/src/parser.c b/src/parser.c\n--- a/src/parser.c\n+++ b/src/parser.c\n@@ -1,2 +1,2 @@\n-first\n+first\n second\n", false},
		{"empty", "", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			lock, src, _ := patchFixture(t, []string{tc.patch})
			if err := os.WriteFile(filepath.Join(src, "LICENSE"), []byte("MIT\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			if tc.generate {
				b, _ := os.ReadFile(lock)
				b = []byte(strings.Replace(string(b), `"licence":"MIT"`, `"generate":true,"licence":"MIT"`, 1))
				if err := os.WriteFile(lock, b, 0o644); err != nil {
					t.Fatal(err)
				}
			}
			if err := runPatchScript(lock, src); err == nil {
				t.Fatal("unsafe patch accepted")
			}
		})
	}
}

func TestPatchRefusesExistingTargetSymlink(t *testing.T) {
	p := "diff --git a/link b/link\n--- a/link\n+++ b/link\n@@ -1 +1 @@\n-first\n+one\n"
	lock, src, _ := patchFixture(t, []string{p})
	outside := filepath.Join(t.TempDir(), "outside")
	if err := os.WriteFile(outside, []byte("first\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(src, "link")); err != nil {
		t.Fatal(err)
	}
	if err := runPatchScript(lock, src); err == nil {
		t.Fatal("symlink target accepted")
	}
}

func TestPatchRefusesAncestorSymlinkAndSourceGitConfig(t *testing.T) {
	p := "diff --git a/deps/helper.js b/deps/helper.js\nnew file mode 100644\n--- /dev/null\n+++ b/deps/helper.js\n@@ -0,0 +1 @@\n+module.exports = 1;\n"
	for _, tc := range []string{"ancestor symlink", "source git config"} {
		t.Run(tc, func(t *testing.T) {
			lock, src, _ := patchFixture(t, []string{p})
			if tc == "ancestor symlink" {
				outside := t.TempDir()
				if err := os.Symlink(outside, filepath.Join(src, "deps")); err != nil {
					t.Fatal(err)
				}
			} else {
				if err := os.Mkdir(filepath.Join(src, ".git"), 0o755); err != nil {
					t.Fatal(err)
				}
			}
			if err := runPatchScript(lock, src); err == nil {
				t.Fatal("unsafe source accepted")
			}
		})
	}
}

func TestPatchAllowsRegularJSDependency(t *testing.T) {
	p := "diff --git a/node_modules/tree-sitter-c/helper.js b/node_modules/tree-sitter-c/helper.js\nnew file mode 100644\n--- /dev/null\n+++ b/node_modules/tree-sitter-c/helper.js\n@@ -0,0 +1 @@\n+module.exports = 1;\n"
	lock, src, _ := patchFixture(t, []string{p})
	b, err := os.ReadFile(lock)
	if err != nil {
		t.Fatal(err)
	}
	b = []byte(strings.Replace(string(b), `"licence":"MIT"`, `"generate":true,"licence":"MIT"`, 1))
	if err := os.WriteFile(lock, b, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := runPatchScript(lock, src); err != nil {
		t.Fatal(err)
	}
	b, err = os.ReadFile(filepath.Join(src, "node_modules/tree-sitter-c/helper.js"))
	if err != nil || string(b) != "module.exports = 1;\n" {
		t.Fatalf("dependency: %q %v", b, err)
	}
}

func TestPatchUsesSelectedRowAfterLockChanges(t *testing.T) {
	p := "diff --git a/src/parser.c b/src/parser.c\n--- a/src/parser.c\n+++ b/src/parser.c\n@@ -1,2 +1,2 @@\n-first\n+one\n second\n"
	lock, src, _ := patchFixture(t, []string{p})
	b, err := os.ReadFile(lock)
	if err != nil {
		t.Fatal(err)
	}
	var parsed struct {
		Grammars []json.RawMessage `json:"grammars"`
	}
	if err := json.Unmarshal(b, &parsed); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(lock, []byte(`{"grammars":[]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	c := exec.Command("bash", "grammars/apply-patches.sh", lock, string(parsed.Grammars[0]), src)
	if out, err := c.CombinedOutput(); err != nil {
		t.Fatalf("selected row lost: %v: %s", err, out)
	}
}

func TestPatchAppliesStagedVerifiedBytes(t *testing.T) {
	p := "diff --git a/src/parser.c b/src/parser.c\n--- a/src/parser.c\n+++ b/src/parser.c\n@@ -1,2 +1,2 @@\n-first\n+one\n second\n"
	other := strings.Replace(p, "+one\n", "+evil\n", 1)
	lock, src, lockDir := patchFixture(t, []string{p})
	tools := t.TempDir()
	otherPath := filepath.Join(tools, "other.patch")
	if err := os.WriteFile(otherPath, []byte(other), 0o644); err != nil {
		t.Fatal(err)
	}
	realShasum, err := exec.LookPath("shasum")
	if err != nil {
		t.Fatal(err)
	}
	stub := "#!/bin/sh\n" + realShasum + " \"$@\"\n" +
		"if [ ! -e \"$MUTATE_MARK\" ]; then : > \"$MUTATE_MARK\"; cp \"$MUTATE_PATCH\" \"$MUTATE_ORIGINAL\"; fi\n"
	if err := os.WriteFile(filepath.Join(tools, "shasum"), []byte(stub), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("MUTATE_MARK", filepath.Join(tools, "mutated"))
	t.Setenv("MUTATE_PATCH", otherPath)
	t.Setenv("MUTATE_ORIGINAL", filepath.Join(lockDir, "patches/p1.patch"))
	t.Setenv("PATH", tools+string(os.PathListSeparator)+os.Getenv("PATH"))
	if err := runPatchScript(lock, src); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(src, "src/parser.c"))
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != "one\nsecond\n" {
		t.Fatalf("applied bytes changed after verified read: %q", b)
	}
}
