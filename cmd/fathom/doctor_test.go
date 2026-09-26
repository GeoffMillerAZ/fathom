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
	"time"

	skillsdist "github.com/geoffmilleraz/fathom/dist/agents/skills"
	"github.com/geoffmilleraz/fathom/internal/statedir"
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

	// O23 additions (doctor_test.go): servicePaths is list_services' optional
	// "path" per service — checkHooksFiring's daemon-side call-log
	// resolution needs a real "fathom" service path to join against.
	// obligFileRules, when non-nil, overrides obligations' rule count PER
	// REQUESTED FILE (files absent from the map serve zero, matching a
	// scope miss) — nil (the zero value) preserves every pre-existing
	// test's file-blind obligRules/obligStale/watermark behavior exactly.
	// obligLayerCompiled controls staleness_report's layers["obligations"]
	// presence — checkObligationsServed's "compiled but no probe matches"
	// distinction (default false: "never compiled").
	servicePaths       map[string]string
	obligFileRules     map[string]int
	obligLayerCompiled bool

	// obligLayerCoverage (O54), when non-nil, is rendered as the
	// obligations response's layer_coverage field on every probe that
	// otherwise serves real rules (n > 0) — checkObligationsServed's
	// partial-layer WARN downgrade. Nil (the zero value) preserves every
	// pre-existing test's coverage-free obligations payload exactly.
	obligLayerCoverage *fakeLayerCoverage

	// obligLayerCoverageUnknown (O60), when true, renders layer_coverage
	// as {"status":"unknown", note:...} instead of real numbers — the
	// shape a real store answers with when it has no compile_runs row for
	// the service (semantic.ObligationsLayerCoverage's found:false).
	// Mutually exclusive with obligLayerCoverage; false preserves every
	// pre-existing test's behavior exactly.
	obligLayerCoverageUnknown bool
}

// fakeLayerCoverage mirrors semantic.LayerCoverage's JSON shape for
// fakeDoctorDaemon's obligations responses. CorpusGrew (wave O4, item 3)
// defaults to 0 (omitted from the wire payload, matching
// semantic.LayerCoverage's own corpus_grew,omitempty tag) — every
// pre-existing test that does not set it keeps its exact prior payload.
type fakeLayerCoverage struct {
	SectionsTotal    int
	SectionsCompiled int
	BudgetStopped    bool
	CorpusGrew       int
	UnreadCorpus     []string
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
				Name      string         `json:"name"`
				Arguments map[string]any `json:"arguments"`
			} `json:"params"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		var inner string
		switch req.Params.Name {
		case "list_services":
			rows := make([]string, len(cfg.services))
			for i, s := range cfg.services {
				pathField := ""
				if p := cfg.servicePaths[s]; p != "" {
					pathField = fmt.Sprintf(`,"path":%q`, p)
				}
				rows[i] = fmt.Sprintf(`{"service":%q,"symbol_count":10,"languages":["go"]%s}`, s, pathField)
			}
			inner = `{"services":[` + strings.Join(rows, ",") + `]}`
		case "obligations":
			file, _ := req.Params.Arguments["file"].(string)
			n, wm := cfg.obligRules, cfg.watermark
			if cfg.obligFileRules != nil {
				n = cfg.obligFileRules[file] // absent file: 0, matching a scope miss
				if n == 0 {
					wm = ""
				}
			}
			rules := "[]"
			if n > 0 {
				rs := make([]string, n)
				for i := range rs {
					if i < cfg.obligStale {
						rs[i] = `{"rule":"r","status":"stale"}`
					} else {
						rs[i] = `{"rule":"r"}`
					}
				}
				rules = "[" + strings.Join(rs, ",") + "]"
			}
			wmField := ""
			if wm != "" {
				wmField = fmt.Sprintf(`,"compiled_at_watermark":%q`, wm)
			}
			covField := ""
			if cfg.obligLayerCoverage != nil {
				c := cfg.obligLayerCoverage
				corpusGrewField := ""
				if c.CorpusGrew > 0 {
					corpusGrewField = fmt.Sprintf(`,"corpus_grew":%d`, c.CorpusGrew)
				}
				if len(c.UnreadCorpus) > 0 {
					corpusGrewField += `,"unread_corpus":` + mustJSON(c.UnreadCorpus)
				}
				covField = fmt.Sprintf(`,"layer_coverage":{"sections_total":%d,"sections_compiled":%d,"budget_stopped":%v%s}`,
					c.SectionsTotal, c.SectionsCompiled, c.BudgetStopped, corpusGrewField)
			} else if cfg.obligLayerCoverageUnknown {
				covField = `,"layer_coverage":{"status":"unknown","note":"layer compiled before coverage was recorded; recompile to record coverage"}`
			}
			inner = fmt.Sprintf(`{"service":"svc","file":%q,"rules":%s%s%s}`, file, rules, wmField, covField)
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
		case "staleness_report":
			layers := "{}"
			if cfg.obligLayerCompiled {
				layers = `{"obligations":{"fresh":1}}`
			}
			inner = fmt.Sprintf(`{"layers":%s}`, layers)
		default:
			http.Error(w, "unknown tool", 400)
			return
		}
		env := map[string]any{"result": map[string]any{"content": []map[string]any{{"type": "text", "text": inner}}}}
		_ = json.NewEncoder(w).Encode(env)
	})
	return httptest.NewServer(mux)
}

// writeFathomGoMod writes a go.mod declaring the fathom module at dir —
// the O23 self-hosting gate (isFathomSourceTree) requires this before
// gitRepo commits it, so binary_fresh's/hooks_firing's git- and
// call-log-comparison logic can still be exercised against a plain tempdir
// exactly as before that gate existed.
func writeFathomGoMod(t *testing.T, dir string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module "+fathomModulePath+"\n\ngo 1.26\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}

// writeAllPassFixture builds a fully-wired repo at root: CLAUDE.md/AGENTS.md
// stanza, .claude/settings.json hooks, lefthook.yml, every shipped skill
// current, and a call log with edit-hook/verify-cli traffic. Returns the
// repo's git HEAD.
func writeAllPassFixture(t *testing.T, root, svc string) string {
	t.Helper()
	if err := os.WriteFile(filepath.Join(root, "CLAUDE.md"), []byte(fathomStanza(svc, false)+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "AGENTS.md"), []byte(fathomStanza(svc, false)+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	merged, _, _, err := mergeHookSettings(nil, "/opt/fathom/bin/fathom")
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

	logDir := filepath.Join(statedir.Resolve(root), "logs")
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

	// O23 (docs/dogfood-observations.md): binary_fresh is only meaningful
	// from the fathom source tree itself — every subtest below marks its
	// tempdir as that tree (writeFathomGoMod) so the pre-existing
	// git-comparison logic still gets exercised exactly as before; this
	// first subtest is the NEW gate itself, on a repo that is NOT fathom.
	t.Run("not the fathom source tree is skipped, never a guessed fail (O23)", func(t *testing.T) {
		dir := t.TempDir()
		gitRepo(t, dir) // a plain repo, no go.mod at all
		doctorBinaryRevision = func() (string, bool, bool) { return "deadbeefcafe0000deadbeefcafe0000deadbeef", false, true }

		got := checkBinaryFresh(dir)
		if got.Status != doctorSkipped {
			t.Fatalf("got %+v, want skipped — this repo's HEAD has nothing to do with the fathom binary", got)
		}
		if !strings.Contains(got.Reason, "fathom source tree") {
			t.Errorf("skip reason must explain the limit: %s", got.Reason)
		}
	})

	t.Run("a different repo's go.mod is skipped too, even if its HEAD happens to match", func(t *testing.T) {
		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module github.com/someone/else\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		head := gitRepo(t, dir)
		doctorBinaryRevision = func() (string, bool, bool) { return head, false, true } // would PASS under the old logic

		got := checkBinaryFresh(dir)
		if got.Status != doctorSkipped {
			t.Fatalf("got %+v, want skipped — a coincidentally-matching HEAD in an unrelated module proves nothing", got)
		}
	})

	t.Run("matching revision passes, stating the local-only limit", func(t *testing.T) {
		dir := t.TempDir()
		writeFathomGoMod(t, dir)
		head := gitRepo(t, dir)
		doctorBinaryRevision = func() (string, bool, bool) { return head, false, true }

		got := checkBinaryFresh(dir)
		if got.Status != doctorPass {
			t.Fatalf("got %+v, want pass", got)
		}
		if !strings.Contains(got.Reason, "knows only the local binary") || !strings.Contains(got.Reason, "daemon_fresh") {
			t.Errorf("pass must state the local-only limit: %s", got.Reason)
		}
		if strings.Contains(got.Reason, "vcs.modified") || strings.Contains(got.Reason, "nested") {
			t.Errorf("clean build in a plain repo must carry neither the dirty nor the poisoning note: %s", got.Reason)
		}
	})

	t.Run("dirty build passes with the vcs.modified note surfaced", func(t *testing.T) {
		dir := t.TempDir()
		writeFathomGoMod(t, dir)
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
		writeFathomGoMod(t, dir)
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
		writeFathomGoMod(t, dir)
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
		writeFathomGoMod(t, dir)
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
		writeFathomGoMod(t, main)
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

	// DG-13: a bare inequality FAIL cannot tell "binary is stale, deploy
	// catches it up" from "this checkout hasn't pulled the commits the
	// running binary was built from — deploying now would rebuild from
	// the older tree and reinstall a regression". Both directions must
	// name themselves distinctly and pick the right remedy.

	t.Run("names direction when the local checkout is behind the running binary", func(t *testing.T) {
		requireGit(t)
		dir := t.TempDir()
		writeFathomGoMod(t, dir)
		older := gitRepo(t, dir)
		gitCommit(t, dir, "second.txt", "advance past the pinned binary revision")
		newer, err := doctorGitHead(dir)
		if err != nil || newer == "" {
			t.Fatalf("resolving advanced HEAD: %v", err)
		}
		// Roll the checkout's own HEAD back to the older commit — as if
		// the running binary (pinned at `newer` below) was built from a
		// tree this checkout has not yet pulled.
		gitCheckout(t, dir, older)

		doctorBinaryRevision = func() (string, bool, bool) { return newer, false, true }
		got := checkBinaryFresh(dir)
		if got.Status != doctorFail {
			t.Fatalf("got %+v, want fail", got)
		}
		if !strings.Contains(got.Reason, "checkout") || !strings.Contains(got.Reason, "behind the running binary") {
			t.Errorf("reason must name the checkout as the behind side: %s", got.Reason)
		}
		if !strings.Contains(got.Reason, "1 commit") {
			t.Errorf("reason should count the exact commit gap: %s", got.Reason)
		}
		if !strings.Contains(got.Reason, "do not deploy") {
			t.Errorf("reason must warn against deploying from a behind checkout: %s", got.Reason)
		}
		if !strings.Contains(got.Remedy, "do NOT run task deploy") {
			t.Errorf("remedy must warn against deploying when it would regress: %s", got.Remedy)
		}
		if !strings.Contains(got.Remedy, "pull") {
			t.Errorf("remedy should tell the operator to pull first: %s", got.Remedy)
		}
	})

	t.Run("names direction when the binary is behind HEAD", func(t *testing.T) {
		requireGit(t)
		dir := t.TempDir()
		writeFathomGoMod(t, dir)
		older := gitRepo(t, dir)
		gitCommit(t, dir, "second.txt", "advance HEAD past the pinned (stale) binary revision")

		doctorBinaryRevision = func() (string, bool, bool) { return older, false, true }
		got := checkBinaryFresh(dir)
		if got.Status != doctorFail {
			t.Fatalf("got %+v, want fail", got)
		}
		if !strings.Contains(got.Reason, "binary") || !strings.Contains(got.Reason, "behind HEAD") {
			t.Errorf("reason must name the binary as the behind side: %s", got.Reason)
		}
		if !strings.Contains(got.Reason, "1 commit") {
			t.Errorf("reason should count the exact commit gap: %s", got.Reason)
		}
		if !strings.Contains(got.Reason, "deploy to catch up") {
			t.Errorf("reason should say deploy catches the binary up: %s", got.Reason)
		}
		if !strings.Contains(got.Remedy, "task deploy") {
			t.Errorf("remedy should still be task deploy in this direction: %s", got.Remedy)
		}
	})
}

// gitCommit adds one file with the given content and commits it, advancing
// dir's HEAD by one commit.
func gitCommit(t *testing.T, dir, file, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, file), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{
		{"add", "-A"},
		{"-c", "user.email=t@t", "-c", "user.name=t", "commit", "-qm", "advance"},
	} {
		cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
}

// gitCheckout detaches dir's HEAD at rev, without touching the working tree
// contents doctor's checks read (go.mod, etc. are unaffected since the test
// fixtures don't vary those files across commits).
func gitCheckout(t *testing.T, dir, rev string) {
	t.Helper()
	cmd := exec.Command("git", "-C", dir, "checkout", "-q", "--detach", rev)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git checkout %s: %v\n%s", rev, err, out)
	}
}

// ─── hooks_installed / lefthook_gates / hooks_firing ────────────────────────

func TestCheckHooksInstalled(t *testing.T) {
	// Isolate from the developer's real user-scope ~/.claude/settings.json:
	// the check now reads it by design, so every subtest pins HOME.
	t.Setenv("HOME", t.TempDir())

	// HP-34: Codex's hooks are wired AND verified, so this row is a real
	// verdict for it now. A harness whose codec has not landed (cursor)
	// keeps the skip — that is what the skip was ever for.
	t.Run("codex gets a real verdict, not a skip", func(t *testing.T) {
		t.Setenv("CODEX_HOME", t.TempDir())
		got := checkHooksInstalled(t.TempDir(), "codex")
		if got.Status != doctorFail {
			t.Fatalf("status = %v, want fail (nothing wired in this CODEX_HOME)", got.Status)
		}
	})

	t.Run("a harness with no codec is skipped", func(t *testing.T) {
		got := checkHooksInstalled(t.TempDir(), "cursor")
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
		merged, _, _, err := mergeHookSettings(nil, "/opt/fathom/bin/fathom")
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
		merged, _, _, err := mergeHookSettings(nil, "/opt/fathom/bin/fathom")
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
		merged, _, _, err := mergeHookSettings(nil, "/opt/fathom/bin/fathom")
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

	// Regression for the live dogfood bug (2026-07): only the two phase-O
	// PreToolUse "edit-hook --pre" groups were genuinely missing; the
	// PostToolUse edit-hook was wired and firing (4596 logged calls). The
	// old check used that POST traffic (origin "edit-hook" in the call
	// log) as "evidence" for the MISSING pre-hooks too, because pre and
	// post share the same call-log origin — and told the operator not to
	// run `fathom init`, when running it was exactly the fix. The call log
	// alone can never distinguish pre from post; only the beat log's phase
	// meta can (see hookVariantSeenEvidence).
	t.Run("post-channel call-log traffic is never mistaken for --pre evidence", func(t *testing.T) {
		dir := t.TempDir()
		if err := os.MkdirAll(filepath.Join(dir, ".claude"), 0o755); err != nil {
			t.Fatal(err)
		}
		// The pre-phase-O three are wired; both PreToolUse guard groups are
		// genuinely absent.
		merged, _, _, err := mergeHookSettings(nil, "/opt/fathom/bin/fathom")
		if err != nil {
			t.Fatal(err)
		}
		var parsed map[string]any
		if err := json.Unmarshal(merged, &parsed); err != nil {
			t.Fatal(err)
		}
		hooksObj := parsed["hooks"].(map[string]any)
		delete(hooksObj, "PreToolUse")
		partial, err := json.Marshal(parsed)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, ".claude", "settings.json"), partial, 0o644); err != nil {
			t.Fatal(err)
		}

		// Heavy POST-channel call-log traffic — exactly what the live repro
		// showed (thousands of obligations calls with origin "edit-hook").
		logDir := filepath.Join(statedir.Resolve(dir), "logs")
		if err := os.MkdirAll(logDir, 0o755); err != nil {
			t.Fatal(err)
		}
		line := `{"ts":"2026-01-01T00:00:00Z","tool":"obligations","origin":"edit-hook","latency_ms":1}` + "\n"
		if err := os.WriteFile(filepath.Join(logDir, "mcp-calls.jsonl"), []byte(strings.Repeat(line, 10)), 0o644); err != nil {
			t.Fatal(err)
		}

		got := checkHooksInstalled(dir, "claude")
		if got.Status != doctorFail {
			t.Fatalf("status = %v, want fail (the two --pre groups are genuinely missing)", got.Status)
		}
		if strings.Contains(got.Reason, "DUPLICATES") {
			t.Errorf("post-channel traffic must never be read as evidence for the missing --pre hooks: %s", got.Reason)
		}
		if !strings.Contains(got.Remedy, "fathom init") {
			t.Errorf("genuinely-missing --pre hooks must carry the init remedy, not a duplicate-avoidance warning: %s", got.Remedy)
		}
		if strings.Contains(got.Remedy, "do not run") {
			t.Errorf("must not tell the operator not to run init when the missing hooks have zero firing evidence: %s", got.Remedy)
		}
	})

	// The mirror case: the beat log DOES carry phase=pre evidence for the
	// missing --pre hooks (wired at a scope doctor cannot read, e.g.
	// managed settings) — this must still block the duplicate-installing
	// remedy, exactly as before, just sourced from the beat log now.
	t.Run("beat-log pre-phase evidence still blocks the duplicate-installing remedy", func(t *testing.T) {
		dir := t.TempDir()
		if err := os.MkdirAll(filepath.Join(dir, ".claude"), 0o755); err != nil {
			t.Fatal(err)
		}
		merged, _, _, err := mergeHookSettings(nil, "/opt/fathom/bin/fathom")
		if err != nil {
			t.Fatal(err)
		}
		var parsed map[string]any
		if err := json.Unmarshal(merged, &parsed); err != nil {
			t.Fatal(err)
		}
		hooksObj := parsed["hooks"].(map[string]any)
		delete(hooksObj, "PreToolUse")
		partial, err := json.Marshal(parsed)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, ".claude", "settings.json"), partial, 0o644); err != nil {
			t.Fatal(err)
		}

		beatsPath := filepath.Join(t.TempDir(), "hooks.jsonl")
		t.Setenv("FATHOM_HOOKS_LOG", beatsPath)
		beat := `{"channel":"edit-hook","kind":"seen","at":"2026-01-01T00:00:00Z","meta":{"phase":"pre","target":"file"}}` + "\n" +
			`{"channel":"edit-hook","kind":"seen","at":"2026-01-01T00:00:01Z","meta":{"phase":"pre","target":"command"}}` + "\n"
		if err := os.WriteFile(beatsPath, []byte(beat), 0o644); err != nil {
			t.Fatal(err)
		}

		got := checkHooksInstalled(dir, "claude")
		if got.Status != doctorFail {
			t.Fatalf("status = %v, want fail", got.Status)
		}
		if !strings.Contains(got.Reason, "DUPLICATES") {
			t.Errorf("beat-log evidence for BOTH --pre groups should read as a pure contradiction case: %s", got.Reason)
		}
		if strings.Contains(got.Remedy, "fathom init") && !strings.Contains(got.Remedy, "do not run") {
			t.Errorf("contradiction case must not prescribe the duplicate-installing remedy: %s", got.Remedy)
		}
	})

	// Only ONE of the two PreToolUse matcher groups has beat-log evidence —
	// the fix for defect (b): the two groups must be tracked and reported
	// independently, never conflated into a single "edit-hook --pre" verdict.
	t.Run("the two PreToolUse matcher groups are evidenced independently", func(t *testing.T) {
		dir := t.TempDir()
		if err := os.MkdirAll(filepath.Join(dir, ".claude"), 0o755); err != nil {
			t.Fatal(err)
		}
		merged, _, _, err := mergeHookSettings(nil, "/opt/fathom/bin/fathom")
		if err != nil {
			t.Fatal(err)
		}
		var parsed map[string]any
		if err := json.Unmarshal(merged, &parsed); err != nil {
			t.Fatal(err)
		}
		hooksObj := parsed["hooks"].(map[string]any)
		delete(hooksObj, "PreToolUse")
		partial, err := json.Marshal(parsed)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, ".claude", "settings.json"), partial, 0o644); err != nil {
			t.Fatal(err)
		}

		beatsPath := filepath.Join(t.TempDir(), "hooks.jsonl")
		t.Setenv("FATHOM_HOOKS_LOG", beatsPath)
		// ONLY the file-target (Edit|Write|MultiEdit) group has evidence;
		// the command-target (Bash) group has none.
		beat := `{"channel":"edit-hook","kind":"seen","at":"2026-01-01T00:00:00Z","meta":{"phase":"pre","target":"file"}}` + "\n"
		if err := os.WriteFile(beatsPath, []byte(beat), 0o644); err != nil {
			t.Fatal(err)
		}

		got := checkHooksInstalled(dir, "claude")
		if got.Status != doctorFail {
			t.Fatalf("status = %v, want fail", got.Status)
		}
		// Mixed case: one group evidenced (do-not-duplicate), the other
		// genuinely missing (init is safe/correct for it).
		if !strings.Contains(got.Reason, "matcher Bash") || !strings.Contains(got.Reason, "matcher Edit|Write|MultiEdit") {
			t.Errorf("reason must name BOTH matcher groups distinctly, not conflate them: %s", got.Reason)
		}
		if !strings.Contains(got.Remedy, "fathom init") {
			t.Errorf("mixed case must still offer the init remedy for the genuinely-missing group: %s", got.Remedy)
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

	// deadHC/deadURL: a daemon that is unreachable — these subtests exercise
	// the self-hosting candidate (root IS the fathom tree) without needing a
	// live daemon for the "fathom" service-path candidate too.
	deadHC := http.DefaultClient
	const deadURL = "http://127.0.0.1:1"

	// HP-34: the beat log's `harness` stamp is real per-harness firing
	// evidence, so Codex gets a verdict; an empty log is a FAIL naming
	// what is missing, never a skip that hides it.
	t.Run("codex reads the beat log, not the call log", func(t *testing.T) {
		t.Setenv("FATHOM_HOOKS_LOG", filepath.Join(t.TempDir(), "hooks.jsonl"))
		got := checkHooksFiring(deadHC, deadURL, false, t.TempDir(), "codex")
		if got.Status != doctorFail || !strings.Contains(got.Reason, "harness=codex") {
			t.Fatalf("got %+v, want fail naming the missing harness stamp", got)
		}
	})

	t.Run("a harness with no codec is skipped", func(t *testing.T) {
		got := checkHooksFiring(deadHC, deadURL, false, t.TempDir(), "cursor")
		if got.Status != doctorSkipped {
			t.Fatalf("status = %v, want skipped", got.Status)
		}
	})

	t.Run("not the fathom tree and daemon down: skipped, never a guessed fail (O23)", func(t *testing.T) {
		got := checkHooksFiring(deadHC, deadURL, false, t.TempDir(), "claude")
		if got.Status != doctorSkipped {
			t.Fatalf("status = %v, want skipped — this repo is not the fathom source tree and the daemon cannot be asked for one", got.Status)
		}
	})

	t.Run("fathom source tree, no call log fails — configured is not the same as firing", func(t *testing.T) {
		dir := t.TempDir()
		writeFathomGoMod(t, dir)
		got := checkHooksFiring(deadHC, deadURL, false, dir, "claude")
		if got.Status != doctorFail {
			t.Fatalf("status = %v, want fail", got.Status)
		}
	})

	t.Run("call log with no edit-hook origin still fails", func(t *testing.T) {
		dir := t.TempDir()
		writeFathomGoMod(t, dir)
		logDir := filepath.Join(statedir.Resolve(dir), "logs")
		if err := os.MkdirAll(logDir, 0o755); err != nil {
			t.Fatal(err)
		}
		line := `{"ts":"2026-01-01T00:00:00Z","tool":"list_services","origin":"session-hook","latency_ms":1}` + "\n"
		if err := os.WriteFile(filepath.Join(logDir, "mcp-calls.jsonl"), []byte(line), 0o644); err != nil {
			t.Fatal(err)
		}
		got := checkHooksFiring(deadHC, deadURL, false, dir, "claude")
		if got.Status != doctorFail {
			t.Fatalf("status = %v, want fail", got.Status)
		}
	})

	t.Run("edit-hook traffic passes", func(t *testing.T) {
		dir := t.TempDir()
		writeFathomGoMod(t, dir)
		logDir := filepath.Join(statedir.Resolve(dir), "logs")
		if err := os.MkdirAll(logDir, 0o755); err != nil {
			t.Fatal(err)
		}
		line := `{"ts":"2026-01-01T00:00:00Z","tool":"obligations","origin":"edit-hook","latency_ms":1}` + "\n"
		if err := os.WriteFile(filepath.Join(logDir, "mcp-calls.jsonl"), []byte(line), 0o644); err != nil {
			t.Fatal(err)
		}
		got := checkHooksFiring(deadHC, deadURL, false, dir, "claude")
		if got.Status != doctorPass {
			t.Fatalf("got %+v, want pass", got)
		}
	})

	// O23's own reproduction: root is a normal adopter repo (NOT the fathom
	// tree), the call log genuinely lives at the DAEMON's own workspace
	// (here, a different tempdir standing in for the fathom checkout the
	// daemon was launched from), and the daemon indexes itself as service
	// "fathom" (the fleet-dogfood convention) — checkHooksFiring must find
	// it there and PASS, never FAIL against the adopter root.
	t.Run("O23: adopter repo resolves the daemon's own call log via the fathom service path", func(t *testing.T) {
		adopter := t.TempDir()
		fathomTree := t.TempDir()
		logDir := filepath.Join(statedir.Resolve(fathomTree), "logs")
		if err := os.MkdirAll(logDir, 0o755); err != nil {
			t.Fatal(err)
		}
		line := `{"ts":"2026-01-01T00:00:00Z","tool":"obligations","origin":"edit-hook","latency_ms":1}` + "\n"
		if err := os.WriteFile(filepath.Join(logDir, "mcp-calls.jsonl"), []byte(line), 0o644); err != nil {
			t.Fatal(err)
		}
		srv := fakeDoctorDaemon(t, fakeDaemonCfg{services: []string{"adopter", "fathom"}, servicePaths: map[string]string{"fathom": fathomTree}})
		defer srv.Close()

		got := checkHooksFiring(http.DefaultClient, srv.URL, true, adopter, "claude")
		if got.Status != doctorPass {
			t.Fatalf("got %+v, want pass — the call log lives at the daemon's own \"fathom\" service path, not the adopter root", got)
		}
		// O63 item 5: the resolved call-log path is state-dir-based, not a
		// literal join under fathomTree — it must no longer leak the
		// repo's own path (that's the point), so the reason is checked
		// against callLogPathFor(fathomTree), the SAME resolution
		// resolveCallLogCandidates used to find it.
		if want := callLogPathFor(fathomTree); !strings.Contains(got.Reason, want) {
			t.Errorf("reason should name the actual (daemon-side) call-log path %q: %s", want, got.Reason)
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
		stanza := fathomStanza("svc", false) + "\n"
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
		if err := os.WriteFile(filepath.Join(dir, "AGENTS.md"), []byte(fathomStanza("svc", false)+"\n"), 0o644); err != nil {
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

	// O54 deliverable 5: a PARTIALLY compiled obligations layer still
	// serves real rules — a real, observable PASS — but must WARN, never
	// FAIL, naming the resume command, when a compile_runs row shows
	// sections_compiled < sections_total.
	t.Run("partially compiled layer warns with the resume command, never fails", func(t *testing.T) {
		srv := fakeDoctorDaemon(t, fakeDaemonCfg{
			services: []string{"alpha"}, obligRules: 2, watermark: "wm",
			obligLayerCoverage: &fakeLayerCoverage{SectionsTotal: 60, SectionsCompiled: 18, BudgetStopped: true},
		})
		defer srv.Close()
		root := namedDir(t, "alpha")
		got := checkObligationsServed(http.DefaultClient, srv.URL, root, "", true)
		if got.Status != doctorWarn {
			t.Fatalf("got %+v, want warn (never fail) for a partially compiled layer", got)
		}
		if !strings.Contains(got.Reason, "18/60 sections") {
			t.Errorf("reason must name the section count: %s", got.Reason)
		}
		if !strings.Contains(got.Reason, "fathom compile --service alpha --layer obligations --dry-run=false --continue") {
			t.Errorf("reason must carry the exact resume command: %s", got.Reason)
		}
	})

	// wave O4, item 3 (ADR-037 addendum): the operator's own daemon was
	// observed at 42/66 compiled with a real corpus_grew:14 — the WARN
	// reason named the section split but silently dropped corpus_grew
	// entirely (doctorLayerCoverage's own JSON mirror had no field to
	// decode it into), understating how much of the gap is "the corpus
	// outgrew the last compile" versus "this run's own budget stopped it
	// early". Pinned with the operator's own exact numbers.
	t.Run("partially compiled layer names corpus_grew when the corpus has outgrown the last compile", func(t *testing.T) {
		srv := fakeDoctorDaemon(t, fakeDaemonCfg{
			services: []string{"alpha"}, obligRules: 2, watermark: "wm",
			obligLayerCoverage: &fakeLayerCoverage{SectionsTotal: 66, SectionsCompiled: 42, BudgetStopped: true, CorpusGrew: 14},
		})
		defer srv.Close()
		root := namedDir(t, "alpha")
		got := checkObligationsServed(http.DefaultClient, srv.URL, root, "", true)
		if got.Status != doctorWarn {
			t.Fatalf("got %+v, want warn (never fail) for a partially compiled layer", got)
		}
		if !strings.Contains(got.Reason, "42/66 sections") {
			t.Errorf("reason must name the exact section split: %s", got.Reason)
		}
		if !strings.Contains(got.Reason, "corpus_grew=14") {
			t.Errorf("reason must name corpus_grew when the corpus has outgrown the last compile: %s", got.Reason)
		}
		if !strings.Contains(got.Reason, "fathom compile --service alpha --layer obligations --dry-run=false --continue") {
			t.Errorf("reason must carry the exact resume command: %s", got.Reason)
		}
	})

	// The sibling case: budget_stopped with NO corpus growth must not
	// fabricate a corpus_grew mention — omitted from the wire payload
	// entirely (CorpusGrew: 0, matching the omitempty tag), the reason
	// must say nothing about it either.
	t.Run("partially compiled layer omits corpus_grew when the corpus has not grown", func(t *testing.T) {
		srv := fakeDoctorDaemon(t, fakeDaemonCfg{
			services: []string{"alpha"}, obligRules: 2, watermark: "wm",
			obligLayerCoverage: &fakeLayerCoverage{SectionsTotal: 60, SectionsCompiled: 18, BudgetStopped: true},
		})
		defer srv.Close()
		root := namedDir(t, "alpha")
		got := checkObligationsServed(http.DefaultClient, srv.URL, root, "", true)
		if strings.Contains(got.Reason, "corpus_grew") {
			t.Errorf("reason must not mention corpus_grew when it is zero: %s", got.Reason)
		}
	})

	// RT-143 / I38: a coverage computed without an unread guide is a lower
	// bound. "4/4 compiled" with ADR.md unread must not PASS as complete,
	// and the reason must name the unread guide.
	t.Run("unread corpus guide makes full coverage a named lower bound", func(t *testing.T) {
		srv := fakeDoctorDaemon(t, fakeDaemonCfg{
			services: []string{"alpha"}, obligRules: 2, watermark: "wm",
			obligLayerCoverage: &fakeLayerCoverage{SectionsTotal: 4, SectionsCompiled: 4, UnreadCorpus: []string{"ADR.md: file is 3.0 MiB, over the 2.0 MiB read cap"}},
		})
		defer srv.Close()
		root := namedDir(t, "alpha")
		got := checkObligationsServed(http.DefaultClient, srv.URL, root, "", true)
		if got.Status != doctorWarn {
			t.Errorf("status = %v, want warn: coverage is only a lower bound", got.Status)
		}
		if !strings.Contains(got.Reason, "ADR.md: file is 3.0 MiB") || !strings.Contains(got.Reason, "lower bound") {
			t.Errorf("reason must name the unread guide as a lower bound: %s", got.Reason)
		}
	})

	t.Run("fully compiled layer coverage stays a plain pass", func(t *testing.T) {
		srv := fakeDoctorDaemon(t, fakeDaemonCfg{
			services: []string{"alpha"}, obligRules: 2, watermark: "wm",
			obligLayerCoverage: &fakeLayerCoverage{SectionsTotal: 60, SectionsCompiled: 60, BudgetStopped: false},
		})
		defer srv.Close()
		root := namedDir(t, "alpha")
		got := checkObligationsServed(http.DefaultClient, srv.URL, root, "", true)
		if got.Status != doctorPass {
			t.Fatalf("got %+v, want pass — sections_compiled == sections_total", got)
		}
		if strings.Contains(got.Reason, "PARTIALLY compiled") {
			t.Errorf("a fully compiled layer must not carry the partial-coverage note: %s", got.Reason)
		}
	})

	// O60: the store has no compile_runs row for this service (a layer
	// compiled before coverage tracking existed is the live case) — the
	// envelope now carries layer_coverage:{status:"unknown"} instead of
	// omitting the key entirely, and this row must say so rather than
	// silently reading it as "nothing to report".
	t.Run("unrecorded coverage is named, never silently read as complete", func(t *testing.T) {
		srv := fakeDoctorDaemon(t, fakeDaemonCfg{
			services: []string{"alpha"}, obligRules: 2, watermark: "wm",
			obligLayerCoverageUnknown: true,
		})
		defer srv.Close()
		root := namedDir(t, "alpha")
		got := checkObligationsServed(http.DefaultClient, srv.URL, root, "", true)
		if got.Status != doctorPass {
			t.Fatalf("got %+v, want pass — unrecorded coverage is not itself a problem", got)
		}
		if !strings.Contains(got.Reason, "coverage unrecorded") {
			t.Errorf("reason must name the coverage as unrecorded, not silently read as complete: %s", got.Reason)
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

	// O23's own reproduction: CLAUDE.md alone answers zero because this
	// service's compiled rules are code-scoped, but the SAME daemon serves
	// real rules on a real indexed Go source file — checkObligationsServed
	// must probe that file and PASS naming it, never FAIL for CLAUDE.md
	// specifically. Live evidence: 6 rules served on cmd/conjure/main.go.
	t.Run("O23: dominant-language source-file probe passes when CLAUDE.md alone would fail", func(t *testing.T) {
		srv := fakeDoctorDaemon(t, fakeDaemonCfg{
			services:       []string{"alpha"},
			sources:        []string{"cmd/conjure/main.go"},
			obligFileRules: map[string]int{"cmd/conjure/main.go": 6},
			watermark:      "wm",
		})
		defer srv.Close()
		root := namedDir(t, "alpha")
		got := checkObligationsServed(http.DefaultClient, srv.URL, root, "", true)
		if got.Status != doctorPass {
			t.Fatalf("got %+v, want pass via the source-file probe", got)
		}
		if !strings.Contains(got.Reason, "cmd/conjure/main.go") {
			t.Errorf("pass must name the probe that actually answered: %s", got.Reason)
		}
		if strings.Contains(got.Reason, "file CLAUDE.md") {
			t.Errorf("must not claim CLAUDE.md answered when the source-file probe is what served: %s", got.Reason)
		}
	})

	// The third outcome O23 asks for: every probe (source file + CLAUDE.md +
	// AGENTS.md) serves zero rules, but the obligations layer HAS been
	// compiled for this service (staleness_report's layers block proves
	// it) — a distinct, honest WARN, never conflated with "never compiled".
	t.Run("O23: compiled but no probe matches scope is a distinct warn, not a never-compiled fail", func(t *testing.T) {
		srv := fakeDoctorDaemon(t, fakeDaemonCfg{
			services:           []string{"alpha"},
			sources:            []string{"cmd/conjure/main.go"},
			obligFileRules:     map[string]int{}, // every probed file serves zero
			obligLayerCompiled: true,
		})
		defer srv.Close()
		root := namedDir(t, "alpha")
		got := checkObligationsServed(http.DefaultClient, srv.URL, root, "", true)
		if got.Status != doctorWarn {
			t.Fatalf("got %+v, want warn (compiled, but no probe in scope) — never fail or a silent pass", got)
		}
		if !strings.Contains(got.Reason, "IS compiled") {
			t.Errorf("warn reason must say the layer IS compiled: %s", got.Reason)
		}
		if !strings.Contains(got.Reason, "cmd/conjure/main.go") || !strings.Contains(got.Reason, "CLAUDE.md") || !strings.Contains(got.Reason, "AGENTS.md") {
			t.Errorf("warn reason must name every probe tried: %s", got.Reason)
		}
	})

	// Genuinely never compiled, now proven independently of any single
	// probe file via obligationsLayerCompiled (not just "CLAUDE.md served
	// zero") — still a real FAIL, the never-silent discipline preserved.
	t.Run("O23: never compiled (proven via staleness_report) still fails, distinct from the warn case", func(t *testing.T) {
		srv := fakeDoctorDaemon(t, fakeDaemonCfg{
			services: []string{"alpha"},
			sources:  []string{"cmd/conjure/main.go"},
			// obligFileRules unset (nil): falls back to the file-blind
			// obligRules/watermark, both zero by default — "never compiled".
		})
		defer srv.Close()
		root := namedDir(t, "alpha")
		got := checkObligationsServed(http.DefaultClient, srv.URL, root, "", true)
		if got.Status != doctorFail {
			t.Fatalf("got %+v, want fail", got)
		}
		if !strings.Contains(got.Remedy, "fathom compile --service alpha") {
			t.Errorf("never-compiled fail must carry the compile remedy: %s", got.Remedy)
		}
		if !strings.Contains(got.Reason, "never been compiled") {
			t.Errorf("reason must say never compiled, not the ambiguous old wording: %s", got.Reason)
		}
	})
}

func TestCheckSurfaceTraffic(t *testing.T) {
	t.Run("daemon down is skipped", func(t *testing.T) {
		got := checkSurfaceTraffic(http.DefaultClient, "http://127.0.0.1:1", false, t.TempDir())
		if got.Status != doctorSkipped {
			t.Fatalf("status = %v, want skipped", got.Status)
		}
	})

	t.Run("zero calls fails, scoped to this daemon process's lifetime", func(t *testing.T) {
		srv := fakeDoctorDaemon(t, fakeDaemonCfg{services: []string{"svc"}, agentCalls: 0})
		defer srv.Close()
		got := checkSurfaceTraffic(http.DefaultClient, srv.URL, true, t.TempDir())
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
		got := checkSurfaceTraffic(http.DefaultClient, srv.URL, true, t.TempDir())
		if got.Status != doctorPass {
			t.Fatalf("got %+v, want pass", got)
		}
	})
}

// TestSurfaceTrafficFallsBackToPersistentLog pins IT-4: server_stats'
// surface_traffic counter is PROCESS-LIFETIME (resets on every daemon
// restart, doctor.go's own documented caveat), so a zero there is not
// proof nothing ever happened — checkSurfaceTraffic must consult the
// persistent call log (the same path resolution hooks_firing uses) before
// failing, and only fail when THAT shows no agent-surface row either.
func TestSurfaceTrafficFallsBackToPersistentLog(t *testing.T) {
	t.Run("live counter zero, but a recent agent row in the persistent log passes", func(t *testing.T) {
		logPath := filepath.Join(t.TempDir(), "mcp-calls.jsonl")
		row := fmt.Sprintf(`{"ts":%q,"tool":"find_symbol","surface":"agent"}`, time.Now().UTC().Format(time.RFC3339))
		if err := os.WriteFile(logPath, []byte(row+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		t.Setenv("FATHOM_CALL_LOG", logPath)

		srv := fakeDoctorDaemon(t, fakeDaemonCfg{services: []string{"svc"}, agentCalls: 0})
		defer srv.Close()
		got := checkSurfaceTraffic(http.DefaultClient, srv.URL, true, t.TempDir())
		if got.Status != doctorPass {
			t.Fatalf("status = %v, want pass — the persistent log shows a recent agent call: %+v", got.Status, got)
		}
		if !strings.Contains(got.Reason, "live counter reset by restart") {
			t.Errorf("reason must explain the fallback: %s", got.Reason)
		}
		if !strings.Contains(got.Reason, "1 agent call") {
			t.Errorf("reason must name the count: %s", got.Reason)
		}
	})

	t.Run("live counter zero and no agent row ever still fails", func(t *testing.T) {
		logPath := filepath.Join(t.TempDir(), "mcp-calls.jsonl")
		if err := os.WriteFile(logPath, []byte(`{"ts":"2020-01-01T00:00:00Z","tool":"find_symbol","surface":"cli"}`+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		t.Setenv("FATHOM_CALL_LOG", logPath)

		srv := fakeDoctorDaemon(t, fakeDaemonCfg{services: []string{"svc"}, agentCalls: 0})
		defer srv.Close()
		got := checkSurfaceTraffic(http.DefaultClient, srv.URL, true, t.TempDir())
		if got.Status != doctorFail {
			t.Fatalf("status = %v, want fail — no agent-surface row exists anywhere", got.Status)
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

// TestCmdDoctor_AdopterRepoIsNotFathom is O23's primary regression test
// (docs/dogfood-observations.md): doctor was written and exercised only
// inside fathom's own repo, where "the repo being doctored" and "the repo
// the binary/daemon came from" happen to be the SAME directory. Every prior
// doctor test (including TestCmdDoctor_AllPass above) makes that same
// mistake — its fixture puts the call log inside the doctored root and
// pins the fake binary revision to that root's OWN HEAD, which is exactly
// the self-hosting shape that let three bugs ship unnoticed. This test
// deliberately makes the two directories DIFFERENT: a scratch adopter repo
// (t.TempDir, git-init'd, no go.mod at all — nothing fathom about it) is
// doctored against a daemon that — like the real one — is fathom's own
// source tree, indexed under service "fathom" (CLAUDE.md's fleet-dogfood
// convention), with the running binary's embedded revision set to that
// OTHER tree's HEAD. Before the O23 fix this reproduced all three named
// bugs: binary_fresh FAILed comparing the binary to the adopter's HEAD;
// hooks_firing FAILed looking for the call log under the adopter root
// instead of the daemon's own workspace; obligations_served FAILed
// probing CLAUDE.md alone when this service's compiled rules are
// code-scoped. The assertion: a correctly-wired adopter reports ZERO
// FAILs, and each of the three checks individually is never the FAIL.
func TestCmdDoctor_AdopterRepoIsNotFathom(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	adopter := t.TempDir()
	svc := strings.ToLower(filepath.Base(adopter))
	gitRepo(t, adopter) // a plain repo — no go.mod, nothing fathom about it

	// Wire the adopter like a real `fathom init` would: stanza, hooks,
	// lefthook, skills — everything EXCEPT a call log at its own root (O23:
	// that belongs to the daemon, never the doctored repo).
	if err := os.WriteFile(filepath.Join(adopter, "CLAUDE.md"), []byte(fathomStanza(svc, false)+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(adopter, "AGENTS.md"), []byte(fathomStanza(svc, false)+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	merged, _, _, err := mergeHookSettings(nil, "/opt/fathom/bin/fathom")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(adopter, ".claude"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(adopter, ".claude", "settings.json"), merged, 0o644); err != nil {
		t.Fatal(err)
	}
	lefthook := "pre-push:\n  commands:\n    verify:\n      run: bin/fathom verify --service " + svc + "\n"
	if err := os.WriteFile(filepath.Join(adopter, "lefthook.yml"), []byte(lefthook), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, skillName := range skillsdist.Names {
		content, err := skillsdist.FS.ReadFile(skillName + "/SKILL.md")
		if err != nil {
			t.Fatal(err)
		}
		dest := filepath.Join(adopter, ".claude", "skills", skillName, "SKILL.md")
		if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(dest, content, 0o644); err != nil {
			t.Fatal(err)
		}
	}

	// The fathom tree stand-in: a DIFFERENT directory — the one the daemon
	// (and the doctor binary) actually came from — with its own HEAD and
	// the REAL call log carrying edit-hook traffic. Resolved through
	// symlinks up front (macOS: /var -> /private/var) so it matches what
	// gitToplevel(fathomTree) returns once gitRepo below makes it a real
	// repo — callLogPathFor resolves through gitToplevel too, and the
	// state dir's repo-id must be computed from the SAME string on both
	// the write (here) and read (checkHooksFiring) sides.
	fathomTree := t.TempDir()
	fathomTree, _ = filepath.EvalSymlinks(fathomTree)
	writeFathomGoMod(t, fathomTree)
	fathomHead := gitRepo(t, fathomTree)
	logDir := filepath.Join(statedir.Resolve(fathomTree), "logs")
	if err := os.MkdirAll(logDir, 0o755); err != nil {
		t.Fatal(err)
	}
	logLines := `{"ts":"2026-01-01T00:00:00Z","tool":"obligations","origin":"edit-hook","latency_ms":1}` + "\n"
	if err := os.WriteFile(filepath.Join(logDir, "mcp-calls.jsonl"), []byte(logLines), 0o644); err != nil {
		t.Fatal(err)
	}

	// The daemon indexes BOTH the adopter and itself (service "fathom",
	// path = fathomTree). This adopter's compiled obligations are
	// code-scoped: a real Go source file serves rules, CLAUDE.md alone
	// serves none — the live O23 repro (6 rules on cmd/conjure/main.go,
	// zero on CLAUDE.md).
	srv := fakeDoctorDaemon(t, fakeDaemonCfg{
		services:       []string{svc, "fathom"},
		servicePaths:   map[string]string{"fathom": fathomTree},
		sources:        []string{"cmd/adopter/main.go"},
		obligFileRules: map[string]int{"cmd/adopter/main.go": 4},
		watermark:      "2026-01-01T00:00:00Z",
		agentCalls:     3,
	})
	defer srv.Close()
	writeClaudeRegistration(t, home, srv.URL)

	// The RUNNING binary's own revision is the FATHOM tree's HEAD, not the
	// adopter's — exactly the real deployment shape, and exactly what made
	// the pre-fix binary_fresh FAIL comparing it to the adopter's HEAD.
	old := doctorBinaryRevision
	defer func() { doctorBinaryRevision = old }()
	doctorBinaryRevision = func() (string, bool, bool) { return fathomHead, false, true }

	var out, errOut bytes.Buffer
	code := cmdDoctor([]string{"--path", adopter, "--service", svc, "--url", srv.URL, "--harness", "claude", "--json"}, &out, &errOut)
	if code != 0 {
		t.Fatalf("exit = %d, want 0 (zero false FAILs)\nstdout:\n%s\nstderr:\n%s", code, out.String(), errOut.String())
	}

	var report struct {
		Checks  []doctorCheck `json:"checks"`
		Summary struct {
			Fail int `json:"fail"`
		} `json:"summary"`
	}
	if err := json.Unmarshal(out.Bytes(), &report); err != nil {
		t.Fatalf("output is not valid JSON: %v\n%s", err, out.String())
	}
	if report.Summary.Fail != 0 {
		var failing []string
		for _, c := range report.Checks {
			if c.Status == doctorFail {
				failing = append(failing, fmt.Sprintf("%s: %s", c.Name, c.Reason))
			}
		}
		t.Fatalf("a correctly-wired adopter repo must show ZERO fails, got %d: %s", report.Summary.Fail, strings.Join(failing, " | "))
	}

	// Pin each of the three named bugs individually, by name — a future
	// regression in exactly one of them must still be caught here even if
	// some other check happens to also go red and the aggregate count
	// still reads oddly.
	byName := map[string]doctorCheck{}
	for _, c := range report.Checks {
		byName[c.Name] = c
	}
	if c, ok := byName["binary_fresh"]; ok && c.Status == doctorFail {
		t.Errorf("binary_fresh must never FAIL comparing the binary to a DIFFERENT repo's HEAD: %+v", c)
	}
	if c, ok := byName["hooks_firing"]; !ok || c.Status != doctorPass {
		t.Errorf("hooks_firing must PASS by finding the daemon's real call log via the \"fathom\" service path, not FAIL against the adopter root: %+v", c)
	}
	if c, ok := byName["obligations_served"]; !ok || c.Status != doctorPass {
		t.Errorf("obligations_served must PASS by probing the real code-scoped source file, not FAIL on CLAUDE.md alone: %+v", c)
	} else if !strings.Contains(c.Reason, "cmd/adopter/main.go") {
		t.Errorf("obligations_served pass must name the source-file probe that actually answered: %s", c.Reason)
	}
}

func TestCmdDoctor_BadHarnessFlag(t *testing.T) {
	var out, errOut bytes.Buffer
	code := cmdDoctor([]string{"--harness", "bogus"}, &out, &errOut)
	if code != 2 {
		t.Fatalf("exit = %d, want 2 for an invalid --harness", code)
	}
}

// TestLefthookGates_VariableIndirection pins the audit defect: a pre-push
// step spelled FATHOM=/path/bin/fathom; "$FATHOM" verify ... is a real
// fathom-verify gate, and the literal-substring check called it absent —
// a false negative that told a correctly-gated repo to add what it had.
func TestLefthookGates_VariableIndirection(t *testing.T) {
	cases := []struct {
		name, yml string
		wantPass  bool
	}{
		{"dollar_var_quoted", "pre-push:\n  commands:\n    fathom-verify:\n      run: |\n        FATHOM=/Volumes/x/sextant/bin/fathom\n        \"$FATHOM\" verify --service s --plan docs/plans/p.md\n", true},
		{"dollar_var_bare", "pre-push:\n  commands:\n    gate:\n      run: $FATHOM verify --service s\n", true},
		{"braced_var", "pre-push:\n  commands:\n    gate:\n      run: ${FATHOM} verify --service s\n", true},
		{"absolute_path", "pre-push:\n  commands:\n    gate:\n      run: /usr/local/bin/fathom verify --service s\n", true},
		{"literal_still_works", "pre-push:\n  commands:\n    gate:\n      run: bin/fathom verify --service s\n", true},
		{"no_gate_at_all", "pre-push:\n  commands:\n    lint:\n      run: golangci-lint run\n", false},
		{"verify_without_fathom", "pre-push:\n  commands:\n    gate:\n      run: npm run verify\n", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			if err := os.WriteFile(filepath.Join(root, "lefthook.yml"), []byte(tc.yml), 0o644); err != nil {
				t.Fatal(err)
			}
			got := checkLefthookGates(root)
			gotPass := got.Status == doctorPass
			if gotPass != tc.wantPass {
				t.Errorf("status=%s reason=%q, wantPass=%v", got.Status, got.Reason, tc.wantPass)
			}
		})
	}
}
