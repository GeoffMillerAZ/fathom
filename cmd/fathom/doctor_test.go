package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	skillsdist "github.com/geoffmilleraz/fathom/dist/agents/skills"
)

// ─── test helpers ───────────────────────────────────────────────────────────

func requireGit(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
}

// gitRepo initializes a git repo at dir with one commit and returns its HEAD sha.
func gitRepo(t *testing.T, dir string) string {
	t.Helper()
	requireGit(t)
	for _, args := range [][]string{
		{"init", "-q"},
		{"add", "-A"},
		{"-c", "user.email=t@t", "-c", "user.name=t", "commit", "-qm", "init", "--allow-empty"},
	} {
		cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	head, err := doctorGitHead(dir)
	if err != nil || head == "" {
		t.Fatalf("resolving HEAD: %v", err)
	}
	return head
}

// fakeDaemonCfg configures fakeDoctorDaemon: which services list_services
// reports, the obligations response's rule count (0 => "never compiled"
// shape), how many of those rules carry status:"stale", the compile
// watermark, and the server_stats surface_traffic "agent" call count.
type fakeDaemonCfg struct {
	services   []string
	obligRules int
	obligStale int
	watermark  string
	agentCalls int

	// Phase O additions (doctor_phaseo_test.go): modules is list_modules'
	// {module, path} rows; sources is list_sources' file list. Zero value
	// (both nil) preserves every pre-existing test's behavior exactly —
	// list_modules/list_sources simply weren't callable before these
	// checks existed, so no prior test exercises them.
	modules []fakeModuleRow
	sources []string
}

type fakeModuleRow struct {
	module, path string
}

// fakeDoctorDaemon serves healthz + list_services + obligations +
// server_stats shaped like the real MCP endpoint.
func fakeDoctorDaemon(t *testing.T, cfg fakeDaemonCfg) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "ok")
	})
	mux.HandleFunc("/rpc", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Params struct {
				Name string `json:"name"`
			} `json:"params"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		var inner string
		switch req.Params.Name {
		case "list_services":
			rows := make([]string, len(cfg.services))
			for i, s := range cfg.services {
				rows[i] = fmt.Sprintf(`{"service":%q,"symbol_count":10,"languages":["go"]}`, s)
			}
			inner = `{"services":[` + strings.Join(rows, ",") + `]}`
		case "obligations":
			rules := "[]"
			if cfg.obligRules > 0 {
				rs := make([]string, cfg.obligRules)
				for i := range rs {
					if i < cfg.obligStale {
						rs[i] = `{"rule":"r","status":"stale"}`
					} else {
						rs[i] = `{"rule":"r"}`
					}
				}
				rules = "[" + strings.Join(rs, ",") + "]"
			}
			wm := ""
			if cfg.watermark != "" {
				wm = fmt.Sprintf(`,"compiled_at_watermark":%q`, cfg.watermark)
			}
			inner = fmt.Sprintf(`{"service":"svc","file":"CLAUDE.md","rules":%s%s}`, rules, wm)
		case "server_stats":
			inner = fmt.Sprintf(`{"uptime_seconds":1,"surface_traffic":{"agent":{"calls":%d,"last_ts":"2026-01-01T00:00:00Z"}}}`, cfg.agentCalls)
		case "list_modules":
			rows := make([]string, len(cfg.modules))
			for i, m := range cfg.modules {
				rows[i] = fmt.Sprintf(`{"module":%q,"path":%q}`, m.module, m.path)
			}
			inner = `{"modules":[` + strings.Join(rows, ",") + `]}`
		case "list_sources":
			rows := make([]string, len(cfg.sources))
			for i, s := range cfg.sources {
				rows[i] = fmt.Sprintf("%q", s)
			}
			inner = `{"sources":[` + strings.Join(rows, ",") + `]}`
		default:
			http.Error(w, "unknown tool", 400)
			return
		}
		env := map[string]any{"result": map[string]any{"content": []map[string]any{{"type": "text", "text": inner}}}}
		_ = json.NewEncoder(w).Encode(env)
	})
	return httptest.NewServer(mux)
}

// writeAllPassFixture builds a fully-wired repo at root: CLAUDE.md/AGENTS.md
// stanza, .claude/settings.json hooks, lefthook.yml, every shipped skill
// current, and a call log with edit-hook/verify-cli traffic. Returns the
// repo's git HEAD.
func writeAllPassFixture(t *testing.T, root, svc string) string {
	t.Helper()
	if err := os.WriteFile(filepath.Join(root, "CLAUDE.md"), []byte(fathomStanza(svc)+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "AGENTS.md"), []byte(fathomStanza(svc)+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	merged, _, err := mergeHookSettings(nil, "/opt/fathom/bin/fathom")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, ".claude"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".claude", "settings.json"), merged, 0o644); err != nil {
		t.Fatal(err)
	}

	lefthook := "pre-push:\n  commands:\n    verify:\n      run: bin/fathom verify --service " + svc + "\n    review:\n      run: bin/fathom review --service " + svc + "\n"
	if err := os.WriteFile(filepath.Join(root, "lefthook.yml"), []byte(lefthook), 0o644); err != nil {
		t.Fatal(err)
	}

	for _, skillName := range skillsdist.Names {
		content, err := skillsdist.FS.ReadFile(skillName + "/SKILL.md")
		if err != nil {
			t.Fatal(err)
		}
		dest := filepath.Join(root, ".claude", "skills", skillName, "SKILL.md")
		if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(dest, content, 0o644); err != nil {
			t.Fatal(err)
		}
	}

	logDir := filepath.Join(root, "workspace", "logs")
	if err := os.MkdirAll(logDir, 0o755); err != nil {
		t.Fatal(err)
	}
	logLines := `{"ts":"2026-01-01T00:00:00Z","tool":"obligations","origin":"edit-hook","latency_ms":1}
{"ts":"2026-01-01T00:00:01Z","tool":"verify_plan","origin":"verify-cli","latency_ms":1}
`
	if err := os.WriteFile(filepath.Join(logDir, "mcp-calls.jsonl"), []byte(logLines), 0o644); err != nil {
		t.Fatal(err)
	}

	return gitRepo(t, root)
}

// writeClaudeRegistration writes ~/.claude.json (HOME must already be
// t.Setenv'd to a temp dir) registering fathom at baseURL/rpc/agent.
func writeClaudeRegistration(t *testing.T, home, baseURL string) {
	t.Helper()
	doc := fmt.Sprintf(`{"mcpServers":{"fathom":{"type":"http","url":%q}}}`, baseURL+"/rpc/agent")
	if err := os.WriteFile(filepath.Join(home, ".claude.json"), []byte(doc), 0o644); err != nil {
		t.Fatal(err)
	}
}

// ─── mcp_registration ───────────────────────────────────────────────────────

func TestCheckClaudeMCPRegistration(t *testing.T) {
	t.Run("unreadable home is skipped, never a guessed pass", func(t *testing.T) {
		t.Setenv("HOME", filepath.Join(t.TempDir(), "does-not-exist"))
		got := checkClaudeMCPRegistration("http://localhost:7575")
		if got.Status != doctorSkipped {
			t.Fatalf("status = %v, want skipped", got.Status)
		}
	})

	t.Run("missing entry fails with the register remedy", func(t *testing.T) {
		home := t.TempDir()
		t.Setenv("HOME", home)
		if err := os.WriteFile(filepath.Join(home, ".claude.json"), []byte(`{"mcpServers":{}}`), 0o644); err != nil {
			t.Fatal(err)
		}
		got := checkClaudeMCPRegistration("http://localhost:7575")
		if got.Status != doctorFail || got.Remedy == "" {
			t.Fatalf("got %+v, want fail with a remedy", got)
		}
	})

	t.Run("wrong URL fails", func(t *testing.T) {
		home := t.TempDir()
		t.Setenv("HOME", home)
		writeClaudeRegistration(t, home, "http://localhost:7575")
		// Overwrite with a non-/rpc/agent URL.
		doc := `{"mcpServers":{"fathom":{"type":"http","url":"http://localhost:7575/rpc"}}}`
		if err := os.WriteFile(filepath.Join(home, ".claude.json"), []byte(doc), 0o644); err != nil {
			t.Fatal(err)
		}
		got := checkClaudeMCPRegistration("http://localhost:7575")
		if got.Status != doctorFail {
			t.Fatalf("status = %v, want fail", got.Status)
		}
	})

	t.Run("registered at /rpc/agent passes", func(t *testing.T) {
		home := t.TempDir()
		t.Setenv("HOME", home)
		writeClaudeRegistration(t, home, "http://localhost:7575")
		got := checkClaudeMCPRegistration("http://localhost:7575")
		if got.Status != doctorPass {
			t.Fatalf("got %+v, want pass", got)
		}
	})

	t.Run("companion alias also passes", func(t *testing.T) {
		home := t.TempDir()
		t.Setenv("HOME", home)
		doc := `{"mcpServers":{"fathom":{"type":"http","url":"http://localhost:7575/rpc/companion"}}}`
		if err := os.WriteFile(filepath.Join(home, ".claude.json"), []byte(doc), 0o644); err != nil {
			t.Fatal(err)
		}
		got := checkClaudeMCPRegistration("http://localhost:7575")
		if got.Status != doctorPass {
			t.Fatalf("got %+v, want pass", got)
		}
	})
}

func TestCheckCodexMCPRegistration(t *testing.T) {
	t.Run("no readable config is skipped", func(t *testing.T) {
		t.Setenv("HOME", t.TempDir())
		got := checkCodexMCPRegistration(t.TempDir())
		if got.Status != doctorSkipped {
			t.Fatalf("status = %v, want skipped", got.Status)
		}
	})

	t.Run("http url at /rpc/agent passes", func(t *testing.T) {
		home := t.TempDir()
		t.Setenv("HOME", home)
		if err := os.MkdirAll(filepath.Join(home, ".codex"), 0o755); err != nil {
			t.Fatal(err)
		}
		toml := "[mcp_servers.fathom]\nurl = \"http://127.0.0.1:7575/rpc/agent\"\n"
		if err := os.WriteFile(filepath.Join(home, ".codex", "config.toml"), []byte(toml), 0o644); err != nil {
			t.Fatal(err)
		}
		got := checkCodexMCPRegistration(t.TempDir())
		if got.Status != doctorPass {
			t.Fatalf("got %+v, want pass", got)
		}
	})

	t.Run("stdio transport passes without a URL", func(t *testing.T) {
		home := t.TempDir()
		t.Setenv("HOME", home)
		if err := os.MkdirAll(filepath.Join(home, ".codex"), 0o755); err != nil {
			t.Fatal(err)
		}
		toml := "[mcp_servers.fathom]\ncommand = \"/abs/path/bin/fathom\"\n"
		if err := os.WriteFile(filepath.Join(home, ".codex", "config.toml"), []byte(toml), 0o644); err != nil {
			t.Fatal(err)
		}
		got := checkCodexMCPRegistration(t.TempDir())
		if got.Status != doctorPass {
			t.Fatalf("got %+v, want pass", got)
		}
	})

	t.Run("no fathom entry fails", func(t *testing.T) {
		home := t.TempDir()
		t.Setenv("HOME", home)
		if err := os.MkdirAll(filepath.Join(home, ".codex"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(home, ".codex", "config.toml"), []byte("[mcp_servers.other]\nurl = \"http://x\"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		got := checkCodexMCPRegistration(t.TempDir())
		if got.Status != doctorFail {
			t.Fatalf("status = %v, want fail", got.Status)
		}
	})
}

// ─── daemon_reachable ───────────────────────────────────────────────────────

func TestCheckDaemonReachable(t *testing.T) {
	t.Run("dead daemon fails", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {}))
		deadURL := srv.URL
		srv.Close() // now unreachable

		up, got := checkDaemonReachable(http.DefaultClient, deadURL)
		if up {
			t.Fatal("up = true for a dead daemon")
		}
		if got.Status != doctorFail || got.Remedy == "" {
			t.Fatalf("got %+v, want fail with a remedy", got)
		}
	})

	t.Run("live daemon passes", func(t *testing.T) {
		srv := fakeDoctorDaemon(t, fakeDaemonCfg{services: []string{"svc"}, obligRules: 1, watermark: "wm", agentCalls: 1})
		defer srv.Close()
		up, got := checkDaemonReachable(http.DefaultClient, srv.URL)
		if !up || got.Status != doctorPass {
			t.Fatalf("got up=%v check=%+v, want pass", up, got)
		}
	})
}

// ─── binary_fresh ───────────────────────────────────────────────────────────

func TestCheckBinaryFresh(t *testing.T) {
	old := doctorBinaryRevision
	defer func() { doctorBinaryRevision = old }()

	t.Run("matching revision passes, stating the local-only limit", func(t *testing.T) {
		dir := t.TempDir()
		head := gitRepo(t, dir)
		doctorBinaryRevision = func() (string, bool, bool) { return head, false, true }

		got := checkBinaryFresh(dir)
		if got.Status != doctorPass {
			t.Fatalf("got %+v, want pass", got)
		}
		if !strings.Contains(got.Reason, "daemon's installed copy is unverified") {
			t.Errorf("pass must state the local-only limit: %s", got.Reason)
		}
		if strings.Contains(got.Reason, "vcs.modified") || strings.Contains(got.Reason, "nested") {
			t.Errorf("clean build in a plain repo must carry neither the dirty nor the poisoning note: %s", got.Reason)
		}
	})

	t.Run("dirty build passes with the vcs.modified note surfaced", func(t *testing.T) {
		dir := t.TempDir()
		head := gitRepo(t, dir)
		doctorBinaryRevision = func() (string, bool, bool) { return head, true, true }

		got := checkBinaryFresh(dir)
		if got.Status != doctorPass {
			t.Fatalf("got %+v, want pass", got)
		}
		if !strings.Contains(got.Reason, "vcs.modified=true") {
			t.Errorf("dirty build must surface vcs.modified in the reason: %s", got.Reason)
		}
	})

	t.Run("nested linked worktrees add the stamp-poisoning note to a matching pass", func(t *testing.T) {
		requireGit(t)
		dir := t.TempDir()
		head := gitRepo(t, dir)
		nested := filepath.Join(dir, ".claude", "worktrees", "agent-x")
		if err := os.MkdirAll(filepath.Dir(nested), 0o755); err != nil {
			t.Fatal(err)
		}
		if out, err := exec.Command("git", "-C", dir, "worktree", "add", "--detach", nested, "HEAD").CombinedOutput(); err != nil {
			t.Fatalf("git worktree add: %v\n%s", err, out)
		}
		doctorBinaryRevision = func() (string, bool, bool) { return head, false, true }

		got := checkBinaryFresh(dir)
		if got.Status != doctorPass {
			t.Fatalf("got %+v, want pass", got)
		}
		if !strings.Contains(got.Reason, "linked worktree") || !strings.Contains(got.Reason, "does not prove") {
			t.Errorf("matching stamp with nested worktrees must carry the poisoning note: %s", got.Reason)
		}
	})

	t.Run("stale binary fails with a deploy remedy", func(t *testing.T) {
		dir := t.TempDir()
		gitRepo(t, dir)
		doctorBinaryRevision = func() (string, bool, bool) { return "deadbeefcafe0000deadbeefcafe0000deadbeef", false, true }

		got := checkBinaryFresh(dir)
		if got.Status != doctorFail || got.Remedy == "" {
			t.Fatalf("got %+v, want fail with a remedy", got)
		}
		if strings.Contains(got.Reason, "worktree") {
			t.Fatalf("plain repo should not carry the worktree caveat: %s", got.Reason)
		}
	})

	t.Run("no embedded revision is skipped, never a guessed pass or fail", func(t *testing.T) {
		dir := t.TempDir()
		gitRepo(t, dir)
		doctorBinaryRevision = func() (string, bool, bool) { return "", false, false }

		got := checkBinaryFresh(dir)
		if got.Status != doctorSkipped {
			t.Fatalf("status = %v, want skipped", got.Status)
		}
	})

	t.Run("stale binary in a linked worktree carries the caveat, only when verified", func(t *testing.T) {
		requireGit(t)
		main := t.TempDir()
		gitRepo(t, main)

		wt := filepath.Join(t.TempDir(), "linked-wt")
		if out, err := exec.Command("git", "-C", main, "worktree", "add", "--detach", wt, "HEAD").CombinedOutput(); err != nil {
			t.Fatalf("git worktree add: %v\n%s", err, out)
		}
		// Advance the worktree's own HEAD so it diverges from the pinned
		// (fake) binary revision below.
		if err := os.WriteFile(filepath.Join(wt, "f.txt"), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
		for _, args := range [][]string{
			{"add", "-A"},
			{"-c", "user.email=t@t", "-c", "user.name=t", "commit", "-qm", "wt commit"},
		} {
			cmd := exec.Command("git", append([]string{"-C", wt}, args...)...)
			if out, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("git %v: %v\n%s", args, err, out)
			}
		}

		if !isGitWorktree(wt) {
			t.Fatalf("expected %s to be detected as a linked worktree", wt)
		}
		if isGitWorktree(main) {
			t.Fatalf("main repo %s must NOT be detected as a linked worktree", main)
		}

		doctorBinaryRevision = func() (string, bool, bool) { return "deadbeefcafe0000deadbeefcafe0000deadbeef", false, true }
		got := checkBinaryFresh(wt)
		if got.Status != doctorFail {
			t.Fatalf("got %+v, want fail", got)
		}
		if !strings.Contains(got.Reason, "worktree") {
			t.Fatalf("expected the worktree caveat in the reason: %s", got.Reason)
		}
	})
}

// ─── hooks_installed / lefthook_gates / hooks_firing ────────────────────────

func TestCheckHooksInstalled(t *testing.T) {
	// Isolate from the developer's real user-scope ~/.claude/settings.json:
	// the check now reads it by design, so every subtest pins HOME.
	t.Setenv("HOME", t.TempDir())

	t.Run("codex is skipped — no hook mechanism", func(t *testing.T) {
		got := checkHooksInstalled(t.TempDir(), "codex")
		if got.Status != doctorSkipped {
			t.Fatalf("status = %v, want skipped", got.Status)
		}
	})

	t.Run("no settings in any scope fails with the init remedy", func(t *testing.T) {
		got := checkHooksInstalled(t.TempDir(), "claude")
		if got.Status != doctorFail {
			t.Fatalf("status = %v, want fail", got.Status)
		}
		if !strings.Contains(got.Remedy, "fathom init") {
			t.Errorf("no-wiring-no-traffic case should carry the init remedy: %+v", got)
		}
	})

	t.Run("fully wired project settings.json passes", func(t *testing.T) {
		dir := t.TempDir()
		merged, _, err := mergeHookSettings(nil, "/opt/fathom/bin/fathom")
		if err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(filepath.Join(dir, ".claude"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, ".claude", "settings.json"), merged, 0o644); err != nil {
			t.Fatal(err)
		}
		got := checkHooksInstalled(dir, "claude")
		if got.Status != doctorPass {
			t.Fatalf("got %+v, want pass", got)
		}
		if !strings.Contains(got.Reason, "project") {
			t.Errorf("pass should name the wiring scope: %s", got.Reason)
		}
	})

	t.Run("user-scope wiring passes with a scope note — never a project-scope FAIL", func(t *testing.T) {
		home := t.TempDir()
		t.Setenv("HOME", home)
		merged, _, err := mergeHookSettings(nil, "/opt/fathom/bin/fathom")
		if err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(filepath.Join(home, ".claude"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(home, ".claude", "settings.json"), merged, 0o644); err != nil {
			t.Fatal(err)
		}
		got := checkHooksInstalled(t.TempDir(), "claude") // project scope empty
		if got.Status != doctorPass {
			t.Fatalf("got %+v, want pass (user-scope wiring is a valid wiring)", got)
		}
		if !strings.Contains(got.Reason, "user") {
			t.Errorf("pass should name the user scope: %s", got.Reason)
		}
	})

	t.Run("project-local settings.local.json wiring passes", func(t *testing.T) {
		dir := t.TempDir()
		merged, _, err := mergeHookSettings(nil, "/opt/fathom/bin/fathom")
		if err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(filepath.Join(dir, ".claude"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, ".claude", "settings.local.json"), merged, 0o644); err != nil {
			t.Fatal(err)
		}
		got := checkHooksInstalled(dir, "claude")
		if got.Status != doctorPass {
			t.Fatalf("got %+v, want pass", got)
		}
	})

	t.Run("partial wiring fails naming the missing hook", func(t *testing.T) {
		dir := t.TempDir()
		if err := os.MkdirAll(filepath.Join(dir, ".claude"), 0o755); err != nil {
			t.Fatal(err)
		}
		partial := `{"hooks":{"SessionStart":[{"matcher":"startup|resume|clear|compact","hooks":[{"type":"command","command":"/x fathom session-hook"}]}]}}`
		if err := os.WriteFile(filepath.Join(dir, ".claude", "settings.json"), []byte(partial), 0o644); err != nil {
			t.Fatal(err)
		}
		got := checkHooksInstalled(dir, "claude")
		if got.Status != doctorFail || !strings.Contains(got.Reason, "edit-hook") {
			t.Fatalf("got %+v, want fail naming edit-hook/stop-hook", got)
		}
	})

	t.Run("no visible wiring but live edit-hook traffic names the contradiction, not the duplicate-installing remedy", func(t *testing.T) {
		dir := t.TempDir()
		logDir := filepath.Join(dir, "workspace", "logs")
		if err := os.MkdirAll(logDir, 0o755); err != nil {
			t.Fatal(err)
		}
		line := `{"ts":"2026-01-01T00:00:00Z","tool":"obligations","origin":"edit-hook","latency_ms":1}` + "\n"
		if err := os.WriteFile(filepath.Join(logDir, "mcp-calls.jsonl"), []byte(line), 0o644); err != nil {
			t.Fatal(err)
		}
		got := checkHooksInstalled(dir, "claude")
		if got.Status != doctorFail {
			t.Fatalf("status = %v, want fail", got.Status)
		}
		if !strings.Contains(got.Reason, "DUPLICATES") {
			t.Errorf("contradiction case must warn about duplicate installation: %s", got.Reason)
		}
		if strings.Contains(got.Remedy, "fathom init") && !strings.Contains(got.Remedy, "do not run") {
			t.Errorf("contradiction case must not prescribe the duplicate-installing remedy: %s", got.Remedy)
		}
	})
}

func TestCheckLefthookGates(t *testing.T) {
	t.Run("missing file fails", func(t *testing.T) {
		got := checkLefthookGates(t.TempDir())
		if got.Status != doctorFail {
			t.Fatalf("status = %v, want fail", got.Status)
		}
	})

	t.Run("verify-only passes with a note", func(t *testing.T) {
		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, "lefthook.yml"), []byte("pre-push:\n  commands:\n    verify:\n      run: bin/fathom verify --service x\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		got := checkLefthookGates(dir)
		if got.Status != doctorPass {
			t.Fatalf("got %+v, want pass", got)
		}
	})

	t.Run("no verify step fails", func(t *testing.T) {
		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, "lefthook.yml"), []byte("pre-commit:\n  commands:\n    fmt:\n      run: gofmt -l .\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		got := checkLefthookGates(dir)
		if got.Status != doctorFail {
			t.Fatalf("status = %v, want fail", got.Status)
		}
	})
}

func TestCheckHooksFiring(t *testing.T) {
	t.Setenv("HOME", t.TempDir()) // resolveCallLogPath reads config.toml; keep it isolated

	t.Run("codex is skipped", func(t *testing.T) {
		got := checkHooksFiring(t.TempDir(), "codex")
		if got.Status != doctorSkipped {
			t.Fatalf("status = %v, want skipped", got.Status)
		}
	})

	t.Run("no call log fails — configured is not the same as firing", func(t *testing.T) {
		got := checkHooksFiring(t.TempDir(), "claude")
		if got.Status != doctorFail {
			t.Fatalf("status = %v, want fail", got.Status)
		}
	})

	t.Run("call log with no edit-hook origin still fails", func(t *testing.T) {
		dir := t.TempDir()
		logDir := filepath.Join(dir, "workspace", "logs")
		if err := os.MkdirAll(logDir, 0o755); err != nil {
			t.Fatal(err)
		}
		line := `{"ts":"2026-01-01T00:00:00Z","tool":"list_services","origin":"session-hook","latency_ms":1}` + "\n"
		if err := os.WriteFile(filepath.Join(logDir, "mcp-calls.jsonl"), []byte(line), 0o644); err != nil {
			t.Fatal(err)
		}
		got := checkHooksFiring(dir, "claude")
		if got.Status != doctorFail {
			t.Fatalf("status = %v, want fail", got.Status)
		}
	})

	t.Run("edit-hook traffic passes", func(t *testing.T) {
		dir := t.TempDir()
		logDir := filepath.Join(dir, "workspace", "logs")
		if err := os.MkdirAll(logDir, 0o755); err != nil {
			t.Fatal(err)
		}
		line := `{"ts":"2026-01-01T00:00:00Z","tool":"obligations","origin":"edit-hook","latency_ms":1}` + "\n"
		if err := os.WriteFile(filepath.Join(logDir, "mcp-calls.jsonl"), []byte(line), 0o644); err != nil {
			t.Fatal(err)
		}
		got := checkHooksFiring(dir, "claude")
		if got.Status != doctorPass {
			t.Fatalf("got %+v, want pass", got)
		}
	})
}

// ─── stanza_present ─────────────────────────────────────────────────────────

func TestCheckStanzaPresent(t *testing.T) {
	t.Run("missing files fail", func(t *testing.T) {
		got := checkStanzaPresent(t.TempDir())
		if got.Status != doctorFail {
			t.Fatalf("status = %v, want fail", got.Status)
		}
	})

	t.Run("well-formed passes", func(t *testing.T) {
		dir := t.TempDir()
		stanza := fathomStanza("svc") + "\n"
		if err := os.WriteFile(filepath.Join(dir, "CLAUDE.md"), []byte(stanza), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "AGENTS.md"), []byte(stanza), 0o644); err != nil {
			t.Fatal(err)
		}
		got := checkStanzaPresent(dir)
		if got.Status != doctorPass {
			t.Fatalf("got %+v, want pass", got)
		}
	})

	t.Run("corrupted marker pair fails, naming the file", func(t *testing.T) {
		dir := t.TempDir()
		// Only the BEGIN marker present — a hand-edit that deleted END.
		corrupted := fathomStanzaBegin + "\nsome content, no end marker\n"
		if err := os.WriteFile(filepath.Join(dir, "CLAUDE.md"), []byte(corrupted), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "AGENTS.md"), []byte(fathomStanza("svc")+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		got := checkStanzaPresent(dir)
		if got.Status != doctorFail || !strings.Contains(got.Reason, "corrupted") || !strings.Contains(got.Reason, "CLAUDE.md") {
			t.Fatalf("got %+v, want fail naming CLAUDE.md as corrupted", got)
		}
	})
}

// ─── skills_installed ───────────────────────────────────────────────────────

func TestCheckSkillsInstalled(t *testing.T) {
	t.Run("none installed fails", func(t *testing.T) {
		got := checkSkillsInstalled(t.TempDir(), "claude")
		if got.Status != doctorFail {
			t.Fatalf("status = %v, want fail", got.Status)
		}
	})

	t.Run("all current passes", func(t *testing.T) {
		dir := t.TempDir()
		for _, skillName := range skillsdist.Names {
			content, err := skillsdist.FS.ReadFile(skillName + "/SKILL.md")
			if err != nil {
				t.Fatal(err)
			}
			dest := filepath.Join(dir, ".claude", "skills", skillName, "SKILL.md")
			if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(dest, content, 0o644); err != nil {
				t.Fatal(err)
			}
		}
		got := checkSkillsInstalled(dir, "claude")
		if got.Status != doctorPass {
			t.Fatalf("got %+v, want pass", got)
		}
	})

	t.Run("stale skill fails", func(t *testing.T) {
		dir := t.TempDir()
		for _, skillName := range skillsdist.Names {
			content, err := skillsdist.FS.ReadFile(skillName + "/SKILL.md")
			if err != nil {
				t.Fatal(err)
			}
			dest := filepath.Join(dir, ".claude", "skills", skillName, "SKILL.md")
			if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
				t.Fatal(err)
			}
			body := content
			if skillName == skillsdist.Names[0] {
				body = append([]byte(nil), content...)
				body = append(body, []byte("\n<!-- fathom:owned -->\nstale edit\n")...)
			}
			if err := os.WriteFile(dest, body, 0o644); err != nil {
				t.Fatal(err)
			}
		}
		got := checkSkillsInstalled(dir, "claude")
		if got.Status != doctorFail || !strings.Contains(got.Reason, "stale") {
			t.Fatalf("got %+v, want fail mentioning stale", got)
		}
	})

	t.Run("non-owned file at a skill's path fails, never silently skipped", func(t *testing.T) {
		dir := t.TempDir()
		dest := filepath.Join(dir, ".claude", "skills", skillsdist.Names[0], "SKILL.md")
		if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(dest, []byte("my own skill, not fathom's"), 0o644); err != nil {
			t.Fatal(err)
		}
		got := checkSkillsInstalled(dir, "claude")
		if got.Status != doctorFail || !strings.Contains(got.Reason, "not fathom-owned") {
			t.Fatalf("got %+v, want fail mentioning not fathom-owned", got)
		}
	})

	t.Run("codex prompts checked under HOME", func(t *testing.T) {
		home := t.TempDir()
		t.Setenv("HOME", home)
		for _, skillName := range skillsdist.Names {
			content, err := skillsdist.FS.ReadFile(skillName + "/codex-prompt.md")
			if err != nil {
				t.Fatal(err)
			}
			dest := filepath.Join(home, ".codex", "prompts", skillName+".md")
			if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(dest, content, 0o644); err != nil {
				t.Fatal(err)
			}
		}
		got := checkSkillsInstalled(t.TempDir(), "codex")
		if got.Status != doctorPass {
			t.Fatalf("got %+v, want pass", got)
		}
	})
}

// ─── config_valid ───────────────────────────────────────────────────────────

func TestCheckConfigValid(t *testing.T) {
	t.Run("absent config is valid", func(t *testing.T) {
		t.Setenv("HOME", t.TempDir())
		got := checkConfigValid(t.TempDir())
		if got.Status != doctorPass {
			t.Fatalf("got %+v, want pass", got)
		}
	})

	t.Run("bad project config fails naming the file", func(t *testing.T) {
		t.Setenv("HOME", t.TempDir())
		dir := t.TempDir()
		if err := os.MkdirAll(filepath.Join(dir, ".fathom"), 0o755); err != nil {
			t.Fatal(err)
		}
		bad := "[profiles.x]\napi_key = \"literal-secret-not-allowed\"\n"
		if err := os.WriteFile(filepath.Join(dir, ".fathom", "config.toml"), []byte(bad), 0o644); err != nil {
			t.Fatal(err)
		}
		got := checkConfigValid(dir)
		if got.Status != doctorFail {
			t.Fatalf("status = %v, want fail", got.Status)
		}
	})
}

// ─── obligations_served / surface_traffic ──────────────────────────────────

// namedDir creates <tmp>/<name> and returns it — a root whose
// serviceCandidates first candidate is a KNOWN name, unlike t.TempDir()'s
// random basename.
func namedDir(t *testing.T, name string) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestCheckObligationsServed(t *testing.T) {
	t.Run("daemon down is skipped, not failed", func(t *testing.T) {
		got := checkObligationsServed(http.DefaultClient, "http://127.0.0.1:1", t.TempDir(), "svc", false)
		if got.Status != doctorSkipped {
			t.Fatalf("status = %v, want skipped", got.Status)
		}
	})

	t.Run("service not in the index is the distinct not-indexed verdict, remedy never names a ghost compile", func(t *testing.T) {
		srv := fakeDoctorDaemon(t, fakeDaemonCfg{services: []string{"other"}})
		defer srv.Close()
		root := namedDir(t, "alpha")
		got := checkObligationsServed(http.DefaultClient, srv.URL, root, "", true)
		if got.Status != doctorFail {
			t.Fatalf("status = %v, want fail", got.Status)
		}
		if !strings.Contains(got.Reason, "alpha") || !strings.Contains(got.Reason, "other") {
			t.Errorf("not-indexed reason must name the candidates and the indexed services: %s", got.Reason)
		}
		if !strings.Contains(got.Reason, "injection cannot fire") {
			t.Errorf("not-indexed reason must state the hook consequence: %s", got.Reason)
		}
		if strings.Contains(got.Remedy, "fathom compile") {
			t.Errorf("remedy must never be a compile for a service not in the index: %s", got.Remedy)
		}
		if !strings.Contains(got.Remedy, "--service") {
			t.Errorf("remedy must offer the --service escape hatch: %s", got.Remedy)
		}
	})

	t.Run("second candidate resolves like the hooks do", func(t *testing.T) {
		requireGit(t)
		repo := namedDir(t, "beta")
		gitRepo(t, repo)
		sub := filepath.Join(repo, "deploy")
		if err := os.MkdirAll(sub, 0o755); err != nil {
			t.Fatal(err)
		}
		// candidates for sub: [deploy, beta]; only beta is indexed.
		srv := fakeDoctorDaemon(t, fakeDaemonCfg{services: []string{"beta"}, obligRules: 1, watermark: "wm"})
		defer srv.Close()
		got := checkObligationsServed(http.DefaultClient, srv.URL, sub, "", true)
		if got.Status != doctorPass {
			t.Fatalf("got %+v, want pass via the second candidate", got)
		}
		if !strings.Contains(got.Reason, "beta") {
			t.Errorf("reason must name the resolved service: %s", got.Reason)
		}
	})

	t.Run("zero rules and no watermark fails with the compile remedy naming the resolved service", func(t *testing.T) {
		srv := fakeDoctorDaemon(t, fakeDaemonCfg{services: []string{"alpha"}})
		defer srv.Close()
		root := namedDir(t, "alpha")
		got := checkObligationsServed(http.DefaultClient, srv.URL, root, "", true)
		if got.Status != doctorFail || !strings.Contains(got.Remedy, "fathom compile --service alpha") {
			t.Fatalf("got %+v, want fail with a compile remedy for the resolved service", got)
		}
	})

	t.Run("served rules pass", func(t *testing.T) {
		srv := fakeDoctorDaemon(t, fakeDaemonCfg{services: []string{"alpha"}, obligRules: 2, watermark: "2026-01-01T00:00:00Z"})
		defer srv.Close()
		root := namedDir(t, "alpha")
		got := checkObligationsServed(http.DefaultClient, srv.URL, root, "", true)
		if got.Status != doctorPass {
			t.Fatalf("got %+v, want pass", got)
		}
		if strings.Contains(got.Reason, "drift status") {
			t.Errorf("fresh rules must not carry the drift note: %s", got.Reason)
		}
	})

	t.Run("stale-labeled rules pass with the drift count surfaced, never a clean-shaped pass", func(t *testing.T) {
		srv := fakeDoctorDaemon(t, fakeDaemonCfg{services: []string{"alpha"}, obligRules: 3, obligStale: 2, watermark: "wm"})
		defer srv.Close()
		root := namedDir(t, "alpha")
		got := checkObligationsServed(http.DefaultClient, srv.URL, root, "", true)
		if got.Status != doctorPass {
			t.Fatalf("got %+v, want pass", got)
		}
		if !strings.Contains(got.Reason, "2 of 3 rule(s) carry a drift status") {
			t.Errorf("stale rules must be counted into the reason: %s", got.Reason)
		}
	})

	t.Run("explicit --service outside the hook candidates passes with the injection-mismatch note", func(t *testing.T) {
		srv := fakeDoctorDaemon(t, fakeDaemonCfg{services: []string{"fathom"}, obligRules: 1, watermark: "wm"})
		defer srv.Close()
		root := namedDir(t, "sextant")
		got := checkObligationsServed(http.DefaultClient, srv.URL, root, "fathom", true)
		if got.Status != doctorPass {
			t.Fatalf("got %+v, want pass", got)
		}
		if !strings.Contains(got.Reason, "NOT among the hook candidate IDs") {
			t.Errorf("explicit service the hooks cannot resolve must carry the mismatch note: %s", got.Reason)
		}
	})
}

func TestCheckSurfaceTraffic(t *testing.T) {
	t.Run("daemon down is skipped", func(t *testing.T) {
		got := checkSurfaceTraffic(http.DefaultClient, "http://127.0.0.1:1", false)
		if got.Status != doctorSkipped {
			t.Fatalf("status = %v, want skipped", got.Status)
		}
	})

	t.Run("zero calls fails, scoped to this daemon process's lifetime", func(t *testing.T) {
		srv := fakeDoctorDaemon(t, fakeDaemonCfg{services: []string{"svc"}, agentCalls: 0})
		defer srv.Close()
		got := checkSurfaceTraffic(http.DefaultClient, srv.URL, true)
		if got.Status != doctorFail {
			t.Fatalf("status = %v, want fail", got.Status)
		}
		if !strings.Contains(got.Reason, "since this daemon process started") {
			t.Errorf("zero-calls reason must state the process-lifetime scope: %s", got.Reason)
		}
	})

	t.Run("nonzero calls pass", func(t *testing.T) {
		srv := fakeDoctorDaemon(t, fakeDaemonCfg{services: []string{"svc"}, agentCalls: 3})
		defer srv.Close()
		got := checkSurfaceTraffic(http.DefaultClient, srv.URL, true)
		if got.Status != doctorPass {
			t.Fatalf("got %+v, want pass", got)
		}
	})
}

// ─── resolveDoctorHarness / exit code / rendering ──────────────────────────

func TestResolveDoctorHarness(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	if got := resolveDoctorHarness("claude", t.TempDir()); got != "claude" {
		t.Errorf("explicit claude = %q", got)
	}
	if got := resolveDoctorHarness("codex", t.TempDir()); got != "codex" {
		t.Errorf("explicit codex = %q", got)
	}

	t.Run("auto detects claude from .claude/settings.json", func(t *testing.T) {
		dir := t.TempDir()
		if err := os.MkdirAll(filepath.Join(dir, ".claude"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, ".claude", "settings.json"), []byte("{}"), 0o644); err != nil {
			t.Fatal(err)
		}
		if got := resolveDoctorHarness("auto", dir); got != "claude" {
			t.Errorf("got %q, want claude", got)
		}
	})

	t.Run("auto detects codex from .codex/config.toml", func(t *testing.T) {
		dir := t.TempDir()
		if err := os.MkdirAll(filepath.Join(dir, ".codex"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, ".codex", "config.toml"), []byte(""), 0o644); err != nil {
			t.Fatal(err)
		}
		if got := resolveDoctorHarness("auto", dir); got != "codex" {
			t.Errorf("got %q, want codex", got)
		}
	})

	t.Run("auto defaults to claude with no evidence either way", func(t *testing.T) {
		if got := resolveDoctorHarness("auto", t.TempDir()); got != "claude" {
			t.Errorf("got %q, want claude default", got)
		}
	})
}

func TestDoctorExitCode(t *testing.T) {
	cases := []struct {
		name   string
		checks []doctorCheck
		want   int
	}{
		{"all pass", []doctorCheck{{Status: doctorPass}, {Status: doctorSkipped}}, 0},
		{"one fail", []doctorCheck{{Status: doctorPass}, {Status: doctorFail}}, 1},
		{"all skipped", []doctorCheck{{Status: doctorSkipped}, {Status: doctorSkipped}}, 0},
	}
	for _, c := range cases {
		if got := doctorExitCode(c.checks); got != c.want {
			t.Errorf("%s: doctorExitCode = %d, want %d", c.name, got, c.want)
		}
	}
}

// ─── cmdDoctor integration: all-pass and dead-daemon fixtures ──────────────

func TestCmdDoctor_AllPass(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	dir := t.TempDir()
	svc := strings.ToLower(filepath.Base(dir))
	head := writeAllPassFixture(t, dir, svc)

	srv := fakeDoctorDaemon(t, fakeDaemonCfg{services: []string{svc}, obligRules: 3, watermark: "2026-01-01T00:00:00Z", agentCalls: 5})
	defer srv.Close()
	writeClaudeRegistration(t, home, srv.URL)

	old := doctorBinaryRevision
	defer func() { doctorBinaryRevision = old }()
	doctorBinaryRevision = func() (string, bool, bool) { return head, false, true }

	var out, errOut bytes.Buffer
	code := cmdDoctor([]string{"--path", dir, "--service", svc, "--url", srv.URL, "--harness", "claude"}, &out, &errOut)
	if code != 0 {
		t.Fatalf("exit = %d, want 0\nstdout:\n%s\nstderr:\n%s", code, out.String(), errOut.String())
	}
	if !strings.Contains(out.String(), "0 fail") {
		t.Errorf("table does not report zero failures:\n%s", out.String())
	}
}

func TestCmdDoctor_DeadDaemon(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	dir := t.TempDir()
	svc := strings.ToLower(filepath.Base(dir))
	head := writeAllPassFixture(t, dir, svc)

	// Daemon that answers once (to get a URL) then is closed — dead by the
	// time doctor queries it.
	srv := fakeDoctorDaemon(t, fakeDaemonCfg{services: []string{svc}})
	deadURL := srv.URL
	srv.Close()
	writeClaudeRegistration(t, home, deadURL)

	old := doctorBinaryRevision
	defer func() { doctorBinaryRevision = old }()
	doctorBinaryRevision = func() (string, bool, bool) { return head, false, true }

	var out, errOut bytes.Buffer
	code := cmdDoctor([]string{"--path", dir, "--service", svc, "--url", deadURL, "--harness", "claude"}, &out, &errOut)
	if code != 1 {
		t.Fatalf("exit = %d, want 1 (dead daemon must fail the gate)\nstdout:\n%s", code, out.String())
	}
	if !strings.Contains(out.String(), "daemon_reachable") {
		t.Errorf("table missing daemon_reachable row:\n%s", out.String())
	}
}

func TestCmdDoctor_JSON(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	dir := t.TempDir()
	svc := strings.ToLower(filepath.Base(dir))
	head := writeAllPassFixture(t, dir, svc)

	srv := fakeDoctorDaemon(t, fakeDaemonCfg{services: []string{svc}, obligRules: 3, watermark: "2026-01-01T00:00:00Z", agentCalls: 5})
	defer srv.Close()
	writeClaudeRegistration(t, home, srv.URL)

	old := doctorBinaryRevision
	defer func() { doctorBinaryRevision = old }()
	doctorBinaryRevision = func() (string, bool, bool) { return head, false, true }

	var out, errOut bytes.Buffer
	code := cmdDoctor([]string{"--path", dir, "--service", svc, "--url", srv.URL, "--harness", "claude", "--json"}, &out, &errOut)
	if code != 0 {
		t.Fatalf("exit = %d, want 0\n%s", code, out.String())
	}
	var report struct {
		Service string        `json:"service"`
		Harness string        `json:"harness"`
		Checks  []doctorCheck `json:"checks"`
		Summary struct {
			Total, Pass, Fail, Skipped int
		} `json:"summary"`
	}
	if err := json.Unmarshal(out.Bytes(), &report); err != nil {
		t.Fatalf("output is not valid JSON: %v\n%s", err, out.String())
	}
	if report.Service != svc || report.Harness != "claude" {
		t.Errorf("service/harness = %q/%q", report.Service, report.Harness)
	}
	if report.Summary.Fail != 0 || len(report.Checks) == 0 {
		t.Errorf("summary = %+v, checks = %d", report.Summary, len(report.Checks))
	}
}

func TestCmdDoctor_BadHarnessFlag(t *testing.T) {
	var out, errOut bytes.Buffer
	code := cmdDoctor([]string{"--harness", "bogus"}, &out, &errOut)
	if code != 2 {
		t.Fatalf("exit = %d, want 2 for an invalid --harness", code)
	}
}
