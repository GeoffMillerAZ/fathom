package scenariobench

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func writeHomeFixture(t *testing.T, homeDir string, claudeJSON, settingsJSON map[string]any) {
	t.Helper()
	if claudeJSON != nil {
		raw, err := json.Marshal(claudeJSON)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(homeDir, ".claude.json"), raw, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if settingsJSON != nil {
		if err := os.MkdirAll(filepath.Join(homeDir, ".claude"), 0o755); err != nil {
			t.Fatal(err)
		}
		raw, err := json.Marshal(settingsJSON)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(homeDir, ".claude", "settings.json"), raw, 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func TestCleanRoomEnv_CopiesClaudeJSONAndStripsSettingsHooksAndPlugins(t *testing.T) {
	runDir := t.TempDir()
	cellDir := filepath.Join(t.TempDir(), "cell")
	homeDir := t.TempDir()

	writeHomeFixture(
		t, homeDir,
		map[string]any{
			"mcpServers":             map[string]any{"fathom": map[string]any{"type": "http"}},
			"projects":               map[string]any{"/some/path": map[string]any{}},
			"oauthAccount":           map[string]any{"emailAddress": "someone@example.invalid"},
			"hasCompletedOnboarding": true,
		},
		map[string]any{
			"hooks":          map[string]any{"PostToolUse": []any{}},
			"enabledPlugins": map[string]any{"some-plugin": true},
			"model":          "sonnet",
		},
	)

	env, err := cleanRoomEnv(runDir, cellDir, homeDir)
	if err != nil {
		t.Fatalf("cleanRoomEnv: %v", err)
	}

	configDir := filepath.Join(runDir, "claude-config")
	if env["CLAUDE_CONFIG_DIR"] != configDir {
		t.Errorf("CLAUDE_CONFIG_DIR = %q, want %q", env["CLAUDE_CONFIG_DIR"], configDir)
	}
	wantThreads := filepath.Join(cellDir, "threads.jsonl")
	if env["FATHOM_THREADS"] != wantThreads {
		t.Errorf("FATHOM_THREADS = %q, want %q", env["FATHOM_THREADS"], wantThreads)
	}

	// .claude.json: mcpServers/projects/auth/onboarding all survive —
	// --strict-mcp-config already neutralizes mcpServers for every real
	// session (harness_claude.go), so stripping it here only broke
	// authentication (RC-6's own smoke test) for no clean-room benefit.
	claudeJSON := readJSONFile(t, filepath.Join(configDir, ".claude.json"))
	for _, kept := range []string{"mcpServers", "projects", "oauthAccount", "hasCompletedOnboarding"} {
		if _, ok := claudeJSON[kept]; !ok {
			t.Errorf("expected %q kept in the clean room's .claude.json, but it is missing", kept)
		}
	}

	// settings.json: hooks/enabledPlugins are exactly what the clean room
	// exists to keep out of a cell (F4) — stripped; everything else (e.g.
	// "model") survives.
	settings := readJSONFile(t, filepath.Join(configDir, "settings.json"))
	for _, stripped := range []string{"hooks", "enabledPlugins"} {
		if _, ok := settings[stripped]; ok {
			t.Errorf("expected %q stripped from the clean room's settings.json, but it is present", stripped)
		}
	}
	if settings["model"] != "sonnet" {
		t.Errorf("expected non-hook/plugin settings kept, got %v", settings)
	}

	if _, err := os.Stat(cellDir); err != nil {
		t.Errorf("expected cellDir to be created: %v", err)
	}
}

func readJSONFile(t *testing.T, path string) map[string]any {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("parsing %s: %v", path, err)
	}
	return doc
}

func TestCleanRoomEnv_NoHomeSourcesIsNotAnError(t *testing.T) {
	runDir := t.TempDir()
	cellDir := filepath.Join(t.TempDir(), "cell")
	homeDir := t.TempDir() // no .claude.json, no .claude/settings.json

	env, err := cleanRoomEnv(runDir, cellDir, homeDir)
	if err != nil {
		t.Fatalf("cleanRoomEnv: %v", err)
	}
	if env["CLAUDE_CONFIG_DIR"] == "" || env["FATHOM_THREADS"] == "" {
		t.Errorf("expected both env vars set regardless of missing home sources, got %v", env)
	}
	if _, err := os.Stat(filepath.Join(runDir, "claude-config", ".claude.json")); !os.IsNotExist(err) {
		t.Errorf("expected no .claude.json written when the source does not exist, err=%v", err)
	}
	if _, err := os.Stat(filepath.Join(runDir, "claude-config", "settings.json")); !os.IsNotExist(err) {
		t.Errorf("expected no settings.json written when the source does not exist, err=%v", err)
	}
}

func TestSessionEnv_EmptyRunDirSkipsCleanRoom(t *testing.T) {
	wt := Worktree{CellDir: t.TempDir()}
	opts := RunOptions{Env: map[string]string{"FOO": "bar"}}
	env, err := sessionEnv(wt, opts)
	if err != nil {
		t.Fatalf("sessionEnv: %v", err)
	}
	if len(env) != 1 || env["FOO"] != "bar" {
		t.Errorf("expected opts.Env unchanged when RunDir is empty, got %v", env)
	}
}

func TestSessionEnv_MergesCleanRoomUnderExplicitEnv(t *testing.T) {
	wt := Worktree{CellDir: filepath.Join(t.TempDir(), "cell")}
	opts := RunOptions{
		RunDir:  t.TempDir(),
		HomeDir: t.TempDir(),
		Env:     map[string]string{"CLAUDE_CONFIG_DIR": "explicit-wins", "OTHER": "kept"},
	}
	env, err := sessionEnv(wt, opts)
	if err != nil {
		t.Fatalf("sessionEnv: %v", err)
	}
	if env["CLAUDE_CONFIG_DIR"] != "explicit-wins" {
		t.Errorf("expected opts.Env to win on collision, got %q", env["CLAUDE_CONFIG_DIR"])
	}
	if env["OTHER"] != "kept" {
		t.Errorf("expected opts.Env's other keys preserved, got %v", env)
	}
	if env["FATHOM_THREADS"] == "" {
		t.Errorf("expected the clean room's FATHOM_THREADS merged in, got %v", env)
	}
}
