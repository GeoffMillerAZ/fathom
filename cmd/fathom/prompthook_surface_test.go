package main

// prompthook_surface_test.go — O615: the prompt hook's issue-shaped route
// line names a tool the session's registered fathom surface serves. localize
// is on /rpc/extended (and its /rpc/companion alias) only; the default
// /rpc/agent surface gets its own opening move, and an unknown registration
// gets that move plus where localize lives.

import (
	"bytes"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/harness"
)

const surfaceTestTrace = "panic: runtime error: invalid memory address or nil pointer dereference\n\ngoroutine 1 [running]:\nmain.main()\n\t/app/main.go:42 +0x1a"

// writeMCPRegistration writes a Claude Code MCP config file holding one
// "fathom" entry at the top level, or under projects[project] when project
// is set (the local scope).
func writeMCPRegistration(t *testing.T, path, project string, entry map[string]any) {
	t.Helper()
	doc := map[string]any{"mcpServers": map[string]any{"fathom": entry}}
	if project != "" {
		doc = map[string]any{"projects": map[string]any{project: doc}}
	}
	b, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, b, 0o644); err != nil {
		t.Fatal(err)
	}
}

func httpEntry(path string) map[string]any {
	return map[string]any{"type": "http", "url": "http://fathom.test:9" + path}
}

func TestPromptHookRoutesAnIssuePromptToAToolTheSurfaceServes(t *testing.T) {
	const localizeLine = "localize is the opening move"
	type want int
	const (
		wantLocalize want = iota // the extended surface: localize, as before
		wantDefault              // /rpc/agent: the default-surface move, no localize
		wantUnknown              // the default-surface move, plus where localize lives
	)
	cases := []struct {
		name  string
		setup func(t *testing.T, repo, cfg string)
		spent bool
		want  want
	}{
		{"project .mcp.json at /rpc/extended", func(t *testing.T, repo, _ string) {
			writeMCPRegistration(t, filepath.Join(repo, ".mcp.json"), "", httpEntry("/rpc/extended"))
		}, false, wantLocalize},
		{"local scope at /rpc/companion/", func(t *testing.T, repo, cfg string) {
			writeMCPRegistration(t, filepath.Join(cfg, ".claude.json"), repo, httpEntry("/rpc/companion/"))
		}, false, wantLocalize},
		{"user scope at /rpc/agent", func(t *testing.T, _, cfg string) {
			writeMCPRegistration(t, filepath.Join(cfg, ".claude.json"), "", httpEntry("/rpc/agent"))
		}, false, wantDefault},
		{"project stdio bridge to /rpc/agent", func(t *testing.T, repo, _ string) {
			writeMCPRegistration(t, filepath.Join(repo, ".mcp.json"), "", map[string]any{
				"command": "fathom", "args": []string{"serve-mcp", "--url", "http://fathom.test:9/rpc/agent"},
			})
		}, false, wantDefault},
		{"no registration", func(*testing.T, string, string) {}, false, wantUnknown},
		{"unparsable .mcp.json", func(t *testing.T, repo, _ string) {
			writeRepoFile(t, repo, ".mcp.json", "{not json")
		}, false, wantUnknown},
		{"user config over the size cap", func(t *testing.T, _, cfg string) {
			p := filepath.Join(cfg, ".claude.json")
			writeMCPRegistration(t, p, "", httpEntry("/rpc/agent"))
			b, _ := os.ReadFile(p)
			pad := `{"pad":"` + strings.Repeat("x", promptUserConfigCap) + `",` + string(b[1:])
			if err := os.WriteFile(p, []byte(pad), 0o644); err != nil {
				t.Fatal(err)
			}
		}, false, wantUnknown},
		{"deadline spent", func(t *testing.T, repo, _ string) {
			writeMCPRegistration(t, filepath.Join(repo, ".mcp.json"), "", httpEntry("/rpc/agent"))
		}, true, wantUnknown},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Setenv("HOME", t.TempDir())
			cfg := filepath.Join(t.TempDir(), "claude-config")
			t.Setenv("CLAUDE_CONFIG_DIR", cfg)
			repo := t.TempDir()
			c.setup(t, repo, cfg)
			deadline := time.Now().Add(5 * time.Second)
			if c.spent {
				deadline = time.Now().Add(-time.Second)
			}
			got := routeLineFor(&http.Client{Timeout: time.Second}, "http://127.0.0.1:1", repo, surfaceTestTrace, deadline)
			if strings.Contains(got, "\n") || len(got) > harness.CodexAdditionalContextCap || len(strings.Fields(got)) > 45 {
				t.Errorf("route line %q is not one line inside the hook's budget", got)
			}
			defaultMove := strings.Contains(got, "search") && strings.Contains(got, "symbol_context")
			switch c.want {
			case wantLocalize:
				if !strings.Contains(got, localizeLine) {
					t.Errorf("route line = %q, want %q (the extended surface serves localize)", got, localizeLine)
				}
			case wantDefault:
				if !defaultMove || strings.Contains(got, "localize") {
					t.Errorf("route line = %q, want search then symbol_context and no localize (/rpc/agent does not serve it)", got)
				}
			case wantUnknown:
				if !defaultMove || strings.Contains(got, localizeLine) || !strings.Contains(got, "localize") || !strings.Contains(got, "/rpc/extended") {
					t.Errorf("route line = %q, want the default-surface move plus that localize does it on /rpc/extended", got)
				}
			}
		})
	}

	t.Run("the hook reads the session's project registration", func(t *testing.T) {
		dir := t.TempDir()
		t.Setenv("CLAUDE_PROJECT_DIR", dir)
		t.Setenv("CLAUDE_CONFIG_DIR", filepath.Join(t.TempDir(), "claude-config"))
		writeMCPRegistration(t, filepath.Join(dir, ".mcp.json"), "", httpEntry("/rpc/agent"))
		srv := fakePromptDaemon(t, nil, nil)
		defer srv.Close()
		promptHookStdin(t, dir, surfaceTestTrace)
		var out, errBuf bytes.Buffer
		if code := runWithStdin([]string{"prompt-hook", "--url=" + srv.URL}, nil, &out, &errBuf); code != 0 {
			t.Fatalf("prompt-hook exit = %d; stderr=%s", code, errBuf.String())
		}
		if !strings.Contains(out.String(), "symbol_context") || strings.Contains(out.String(), "localize") {
			t.Errorf("prompt-hook output = %q, want the /rpc/agent move without localize", out.String())
		}
	})
}
