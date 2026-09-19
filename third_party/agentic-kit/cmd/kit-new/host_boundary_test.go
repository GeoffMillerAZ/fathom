package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestHostGuardProtectsPolicyAndAliases(t *testing.T) {
	t.Parallel()
	f := newHostFixture(t, "codex")
	f.policy.DenyPaths = append(f.policy.DenyPaths, "private/key")
	f.install(t)
	f.write(t, "private/key", []byte("synthetic"))
	if err := os.Link(filepath.Join(f.dir, "private/key"), filepath.Join(f.dir, "public-alias")); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"PRIVATE/new", "public-alias", ".agentic-kit/new", ".codex/hooks.json", ".codex/config.toml", ".claude/settings.local.json", "AGENTS.md", "claude.md"} {
		t.Run(path, func(t *testing.T) {
			input, _ := json.Marshal(map[string]string{"file_path": path})
			if code, _ := f.guard(t, hostHookInput{Event: "PreToolUse", CWD: f.dir, Tool: "Edit", Input: input}); code != 2 {
				t.Fatalf("protected path escaped: %s", path)
			}
		})
	}
}

func TestHostJSONRejectsAmbiguousCaseAndInvalidUTF8(t *testing.T) {
	t.Parallel()
	for _, raw := range [][]byte{[]byte(`{"tool_name":"Read","TOOL_NAME":"Edit"}`), {'{', '"', 'x', '"', ':', '"', 0xff, '"', '}'}} {
		if hostValidateJSON(raw) == nil {
			t.Fatal("ambiguous native JSON accepted")
		}
	}
}

func TestHostGuardRejectsRawParentTraversal(t *testing.T) {
	t.Parallel()
	for _, host := range []string{"claude", "codex"} {
		t.Run(host, func(t *testing.T) {
			f := newHostFixture(t, host)
			f.install(t)
			f.write(t, "private/secret", []byte("synthetic"))
			if err := f.root.MkdirAll("private/sub", 0700); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(filepath.Join(f.dir, "private/sub"), filepath.Join(f.dir, "alias")); err != nil {
				t.Fatal(err)
			}
			// Deliberately preserve the raw path instead of filepath.Join.
			path := f.dir + "/alias/../secret"
			if raw, err := os.ReadFile(path); err != nil || string(raw) != "synthetic" {
				t.Fatalf("physical traversal fixture: %q %v", raw, err)
			}
			for _, tc := range []struct{ cwd, path string }{
				{f.dir, path}, {f.dir, "alias/../secret"}, {f.dir + "/alias/..", "secret"},
			} {
				input, _ := json.Marshal(map[string]string{"file_path": tc.path})
				if code, out := f.guard(t, hostHookInput{Event: "PreToolUse", CWD: tc.cwd, Tool: "Read", Input: input}); code != 2 {
					t.Fatalf("raw traversal passed: %+v: %d %s", tc, code, out)
				}
			}
		})
	}
}

func TestHostGuardProtectsExecutableAndHardlinks(t *testing.T) {
	t.Parallel()
	for _, host := range []string{"claude", "codex"} {
		for _, local := range []bool{false, true} {
			f := newHostFixture(t, host)
			if local {
				f.write(t, "bin/kit-new", []byte("synthetic executable"))
				f.binary = filepath.Join(f.dir, "bin/kit-new")
				if err := os.Chmod(f.binary, 0700); err != nil {
					t.Fatal(err)
				}
			}
			f.install(t)
			if err := os.Link(f.binary, filepath.Join(f.dir, "binary-alias")); err != nil {
				t.Fatal(err)
			}
			for _, path := range []string{f.binary, "binary-alias"} {
				input, _ := json.Marshal(map[string]string{"file_path": path})
				if code, out := f.guard(t, hostHookInput{Event: "PreToolUse", CWD: f.dir, Tool: "Write", Input: input}); code != 2 {
					t.Fatalf("guard replacement allowed: %s local=%v %s: %d %s", host, local, path, code, out)
				}
			}
		}
	}
}

func TestHostCanonicalProjectAndPortableDeniedPaths(t *testing.T) {
	t.Parallel()
	f := newHostFixture(t, "codex")
	for _, path := range []string{"prívate", "private\x00", "private\tkey"} {
		p := f.policy
		p.DenyPaths = []string{path}
		if validateHostPolicy(p) == nil {
			t.Fatal("normalization-sensitive path accepted")
		}
	}
	f.write(t, "policy.json", hostJSON(f.policy))
	alias := filepath.Join(t.TempDir(), "alias")
	if err := os.Symlink(f.dir, alias); err != nil {
		t.Fatal(err)
	}
	var out, errOut bytes.Buffer
	code := runHost([]string{"plan", "-host", "codex", "-dir", alias, "-binary", f.binary, "-policy", filepath.Join(f.dir, "policy.json")}, bytes.NewReader(nil), &out, &errOut, fakeHostDeps("codex"))
	if code != 0 {
		t.Fatalf("plan: %d %s", code, errOut.String())
	}
	var plan hostPlan
	if err := json.Unmarshal(out.Bytes(), &plan); err != nil {
		t.Fatal(err)
	}
	physical, err := filepath.EvalSymlinks(f.dir)
	if err != nil || plan.Root != physical {
		t.Fatalf("noncanonical project: %s %v", plan.Root, err)
	}
}
