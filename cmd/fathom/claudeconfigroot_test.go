package main

// claudeconfigroot_test.go — O1194: one resolver names Claude Code's
// per-user config root ($CLAUDE_CONFIG_DIR, else ~/.claude), and every
// reader of that tree follows it.

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/custodian"
	"github.com/geoffmilleraz/fathom/internal/harness"
)

func TestClaudeConfigRootHonoursClaudeConfigDir(t *testing.T) {
	home := t.TempDir()
	repo := t.TempDir()
	t.Setenv("HOME", home)
	under := func(t *testing.T, got, root string) {
		t.Helper()
		if !strings.HasPrefix(got, root+string(filepath.Separator)) {
			t.Errorf("%q is not under %q", got, root)
		}
	}

	t.Run("unset: ~/.claude", func(t *testing.T) {
		t.Setenv("CLAUDE_CONFIG_DIR", "")
		root := filepath.Join(home, ".claude")
		if got := harness.ClaudeConfigRoot(home); got != root {
			t.Errorf("ClaudeConfigRoot = %q, want %q", got, root)
		}
		if got := harness.ClaudeUserConfigFile(home); got != filepath.Join(home, ".claude.json") {
			t.Errorf("ClaudeUserConfigFile = %q, want ~/.claude.json", got)
		}
		under(t, custodian.ClaudeCodeMemoryDir(repo, home), filepath.Join(root, "projects"))
		if got := defaultReadoutProjectsDir(); got != filepath.Join(root, "projects") {
			t.Errorf("defaultReadoutProjectsDir = %q, want %q", got, filepath.Join(root, "projects"))
		}
		if got := harness.ClaudeConfigRoot(""); got != "" {
			t.Errorf("ClaudeConfigRoot with no home and no env = %q, want empty", got)
		}
	})

	t.Run("set: $CLAUDE_CONFIG_DIR", func(t *testing.T) {
		cfg := filepath.Join(t.TempDir(), "claude-config")
		t.Setenv("CLAUDE_CONFIG_DIR", cfg)
		for _, h := range []string{home, ""} {
			if got := harness.ClaudeConfigRoot(h); got != cfg {
				t.Errorf("ClaudeConfigRoot(%q) = %q, want %q", h, got, cfg)
			}
		}
		if got := harness.ClaudeUserConfigFile(home); got != filepath.Join(cfg, ".claude.json") {
			t.Errorf("ClaudeUserConfigFile = %q, want $CLAUDE_CONFIG_DIR/.claude.json", got)
		}
		under(t, custodian.ClaudeCodeMemoryDir(repo, home), filepath.Join(cfg, "projects"))
		if got := defaultReadoutProjectsDir(); got != filepath.Join(cfg, "projects") {
			t.Errorf("defaultReadoutProjectsDir = %q, want %q", got, filepath.Join(cfg, "projects"))
		}
	})
}
