package main

import (
	"os"
	"path/filepath"
	"testing"
)

// ─── channels_env / ident_cache_env ────────────────────────────────────────

func TestCheckChannelsEnv_Unset(t *testing.T) {
	t.Setenv("FATHOM_CHANNELS", "")
	got := checkChannelsEnv()
	if got.Status != doctorPass {
		t.Errorf("Status = %v, want pass", got.Status)
	}
}

func TestCheckChannelsEnv_Off(t *testing.T) {
	t.Setenv("FATHOM_CHANNELS", "off")
	got := checkChannelsEnv()
	if got.Status != doctorWarn {
		t.Errorf("Status = %v, want warn", got.Status)
	}
	if got.Remedy == "" {
		t.Error("expected a remedy")
	}
}

func TestCheckIdentCacheEnv_Unset(t *testing.T) {
	t.Setenv("FATHOM_IDENT_NO_CACHE", "")
	got := checkIdentCacheEnv()
	if got.Status != doctorPass {
		t.Errorf("Status = %v, want pass", got.Status)
	}
}

func TestCheckIdentCacheEnv_Disabled(t *testing.T) {
	t.Setenv("FATHOM_IDENT_NO_CACHE", "1")
	got := checkIdentCacheEnv()
	if got.Status != doctorWarn {
		t.Errorf("Status = %v, want warn", got.Status)
	}
}

// ─── user_scope_ignore ──────────────────────────────────────────────────────

func TestCheckUserScopeIgnore_NoHomeFile_Pass(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	got := checkUserScopeIgnore(t.TempDir())
	if got.Status != doctorPass {
		t.Errorf("Status = %v, Reason = %q", got.Status, got.Reason)
	}
}

func TestCheckUserScopeIgnore_ConfiguredNoHiddenFiles_Pass(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	writeUserIgnoreTest(t, home, ".DS_Store\n")

	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "main.go"), []byte("package main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got := checkUserScopeIgnore(root)
	if got.Status != doctorPass {
		t.Errorf("Status = %v, Reason = %q", got.Status, got.Reason)
	}
}

func TestCheckUserScopeIgnore_HidingIndexableFile_Warn(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	writeUserIgnoreTest(t, home, "*.go\n")

	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "main.go"), []byte("package main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got := checkUserScopeIgnore(root)
	if got.Status != doctorWarn {
		t.Fatalf("Status = %v, Reason = %q, want warn", got.Status, got.Reason)
	}
	if got.Remedy == "" {
		t.Error("expected a remedy")
	}
}

func TestCheckUserScopeIgnore_ProjectAlreadyExcludes_NotBlamedOnUser(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	writeUserIgnoreTest(t, home, "*.go\n")

	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, ".fathomignore"), []byte("*.go\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "main.go"), []byte("package main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got := checkUserScopeIgnore(root)
	if got.Status != doctorPass {
		t.Errorf("Status = %v, Reason = %q — project already excludes *.go, so the user-scope pattern isn't the cause", got.Status, got.Reason)
	}
}

func TestCheckUserScopeIgnore_Reinclude_Warn(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	writeUserIgnoreTest(t, home, "!oops\n")

	got := checkUserScopeIgnore(t.TempDir())
	if got.Status != doctorWarn {
		t.Fatalf("Status = %v, Reason = %q, want warn", got.Status, got.Reason)
	}
}

func writeUserIgnoreTest(t *testing.T, home, content string) {
	t.Helper()
	dir := filepath.Join(home, ".fathom")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "ignore"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// ─── user_scope_guard ───────────────────────────────────────────────────────

func TestCheckUserScopeGuard_Absent_Pass(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	got := checkUserScopeGuard()
	if got.Status != doctorPass {
		t.Errorf("Status = %v, Reason = %q", got.Status, got.Reason)
	}
}

func TestCheckUserScopeGuard_Valid_Pass(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	writeUserGuardCue(t, home, `guards: [
	{
		note: "double-check"
		command_scopes: ["terraform apply*"]
	},
]`)
	got := checkUserScopeGuard()
	if got.Status != doctorPass {
		t.Errorf("Status = %v, Reason = %q", got.Status, got.Reason)
	}
}

func TestCheckUserScopeGuard_Invalid_Warn(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	writeUserGuardCue(t, home, `guards: [
	{
		scopes: ["internal/store/**"]
	},
]`)
	got := checkUserScopeGuard()
	if got.Status != doctorWarn {
		t.Fatalf("Status = %v, Reason = %q, want warn", got.Status, got.Reason)
	}
}
