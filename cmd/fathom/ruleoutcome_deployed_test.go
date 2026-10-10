package main

// ruleoutcome_deployed_test.go — GL-24 (docs/plans/governance-learning.md,
// INC-27): the rule-outcome ledger held 67 rows, the last on 2026-10-01, while
// JF-1 and GL-10 were [x] and their unit tests passed. Those tests fed the
// tracker a claim id directly, or a fake daemon whose FIRST rule carried one.
// The deployed daemon serves raw-section rows (no claim id) ahead of the
// compiled ones, and the tracker keyed on rules[0] alone, so every served
// answer wrote nothing; the Bash-write path, 84 of the 90 emissions from
// 2026-10-03 to 2026-10-10, never called the tracker at all.
//
// This test's oracle is the deployed shape, not a unit seam: the hook runs as
// its own process through main() (the binary's entry point, re-exec'd), with
// the command lines Claude Code's settings.json carries (fathomHookSpecs), the
// default ledger and beat paths under a scratch FATHOM_HOME, and a daemon
// answering in the order the real one does (raw rows first).

import (
	"bufio"
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/harness"
)

// deployedCLIEnv makes a re-exec'd test binary BE the fathom binary: init
// below calls main() — prepareProcess, then run(os.Args[1:]) — before the
// test framework parses a flag.
const deployedCLIEnv = "FATHOM_TEST_DEPLOYED_CLI"

func init() {
	if os.Getenv(deployedCLIEnv) == "1" {
		main()
	}
}

// deployedClaim and deployedClaim2 are the compiled rows' claim ids the fake
// daemon serves behind two raw-section rows.
const (
	deployedClaim  = "c0ffee0000000000000000000000000000000000000000000000000000000001"
	deployedClaim2 = "c0ffee0000000000000000000000000000000000000000000000000000000002"
)

// deployedDaemon answers obligations the way the deployed daemon did on
// 2026-10-10 for every sampled file: raw-section rows first (Tier "raw", no
// claim id), then compiled rows that carry one. asked returns the files
// obligations was asked for, in order.
func deployedDaemon(t *testing.T) (srv *httptest.Server, asked func() []string) {
	t.Helper()
	var mu sync.Mutex
	var files []string
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Params struct {
				Name      string `json:"name"`
				Arguments struct {
					Service string `json:"service"`
					File    string `json:"file"`
				} `json:"arguments"`
			} `json:"params"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		inner := map[string]any{}
		switch req.Params.Name {
		case "obligations":
			mu.Lock()
			files = append(files, req.Params.Arguments.File)
			mu.Unlock()
			inner = map[string]any{
				"service": req.Params.Arguments.Service, "file": req.Params.Arguments.File, "displaced": 33,
				"rules": []map[string]any{
					{"rule": "ADR-066: fathom is pure Go", "src": "ADR.md#L4165-L4173", "tier": "raw", "why": "section not compiled"},
					{"rule": "AGENTS.md: workspace/ holds every artifact", "src": "AGENTS.md#L6-L14", "tier": "raw", "why": "section not compiled"},
					{"rule": "Determinism is a contract", "src": "internal/AGENTS.md#L3-L3", "tier": "doc-claimed", "claim_id": deployedClaim},
					{"rule": "Sorted outputs everywhere", "src": "internal/AGENTS.md#L4-L4", "tier": "doc-claimed", "claim_id": deployedClaim2},
				},
			}
		case "list_services":
			inner = map[string]any{"services": []any{}}
		}
		text, _ := json.Marshal(inner)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"jsonrpc": "2.0", "id": 1,
			"result": map[string]any{"content": []any{map[string]any{"type": "text", "text": string(text)}}},
		})
	}))
	t.Cleanup(srv.Close)
	return srv, func() []string {
		mu.Lock()
		defer mu.Unlock()
		return append([]string(nil), files...)
	}
}

// deployedHook is one wired hook: the settings.json event and matcher, and
// the arguments after `fathom` on its command line.
type deployedHook struct{ event, matcher, sub string }

// deployedEditHooks reads the edit-hook command lines `fathom init` writes
// into .claude/settings.json (fathomHookSpecs, from the harness registry).
func deployedEditHooks(t *testing.T) map[string]deployedHook {
	t.Helper()
	out := map[string]deployedHook{}
	for _, s := range hookSpecsFor(harness.ClaudeCode) {
		if !strings.HasPrefix(s.sub, "edit-hook") {
			continue
		}
		key := s.event + " " + s.matcher
		out[key] = deployedHook{event: s.event, matcher: s.matcher, sub: s.sub}
	}
	for _, want := range []string{"PostToolUse Edit|Write|MultiEdit", "PostToolUse Bash", "PreToolUse Edit|Write|MultiEdit", "PreToolUse Bash"} {
		if _, ok := out[want]; !ok {
			t.Fatalf("no wired edit-hook for %s in the claude-code hook table: %+v", want, out)
		}
	}
	return out
}

// deployedEnv is a hook process's environment: the parent's, minus every
// FATHOM_*, CLAUDE_*, HOME and TMPDIR value, plus a scratch home and the
// fake daemon. FATHOM_HOME holds the default ledger and beat log paths; no
// FATHOM_RULE_OUTCOME_LOG or FATHOM_HOOKS_LOG override is set, as deployed.
func deployedEnv(home, projDir, url string) []string {
	var env []string
	for _, kv := range os.Environ() {
		k, _, _ := strings.Cut(kv, "=")
		if strings.HasPrefix(k, "FATHOM_") || strings.HasPrefix(k, "CLAUDE_") || k == "HOME" || k == "TMPDIR" {
			continue
		}
		env = append(env, kv)
	}
	return append(env,
		deployedCLIEnv+"=1",
		"HOME="+home,
		"FATHOM_HOME="+filepath.Join(home, ".fathom"),
		"TMPDIR="+filepath.Join(home, "tmp"),
		"CLAUDE_PROJECT_DIR="+projDir,
		"FATHOM_HTTP="+url,
	)
}

// runDeployedHook runs one wired hook as its own process with a Claude Code
// tool-call envelope on stdin, from cwd.
func runDeployedHook(t *testing.T, env []string, h deployedHook, cwd string, toolInput map[string]any, tool string) string {
	t.Helper()
	envelope, _ := json.Marshal(map[string]any{
		"session_id": "deployed-session", "hook_event_name": h.event, "cwd": cwd,
		"tool_name": tool, "tool_input": toolInput,
	})
	cmd := exec.Command(os.Args[0], strings.Fields(h.sub)...)
	cmd.Env, cmd.Dir, cmd.Stdin = env, cwd, bytes.NewReader(envelope)
	var out, errb bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errb
	if err := cmd.Run(); err != nil {
		t.Fatalf("fathom %s: %v\nstderr: %s", h.sub, err, errb.String())
	}
	return out.String()
}

// runInProcessHook is runDeployedHook through the package's own dispatch
// (runWithStdin, the verb table main() calls) with the environment and the
// temp-directory state the hook process keeps under its TMPDIR: the control
// that the deployed process and the in-process path agree, and the static
// edge from a test to the code the process ran.
func runInProcessHook(t *testing.T, home, repo, url string, h deployedHook, cwd string, toolInput map[string]any, tool string) string {
	t.Helper()
	t.Setenv("HOME", home)
	t.Setenv("FATHOM_HOME", filepath.Join(home, ".fathom"))
	t.Setenv("FATHOM_RULE_OUTCOME_LOG", "")
	t.Setenv("FATHOM_HOOKS_LOG", "")
	t.Setenv("CLAUDE_PROJECT_DIR", repo)
	t.Setenv("FATHOM_HTTP", url)
	tmp := filepath.Join(home, "tmp")
	for v, name := range map[*string]string{
		&ruleOutcomeMemoDir: "fathom-rule-outcome-memos", &guardStampDir: "fathom-guard-stamps",
		&ruleVerdictMemoDir: "fathom-rule-verdict-memos", &bashDoubtDir: "fathom-bash-doubt", &editSetDir: "fathom-edit-set",
	} {
		orig := *v
		*v = filepath.Join(tmp, name)
		t.Cleanup(func() { *v = orig })
	}
	origOrigin, origHarness := rpcOrigin, beatHarness
	t.Cleanup(func() { rpcOrigin, beatHarness = origOrigin, origHarness })
	envelope, _ := json.Marshal(map[string]any{
		"session_id": "deployed-session", "hook_event_name": h.event, "cwd": cwd,
		"tool_name": tool, "tool_input": toolInput,
	})
	hookStdin(t, string(envelope))
	var out, errb bytes.Buffer
	if code := runWithStdin(strings.Fields(h.sub), nil, &out, &errb); code != 0 {
		t.Fatalf("in-process fathom %s: exit %d: %s", h.sub, code, errb.String())
	}
	return out.String()
}

// deployedFixture is a git repository with workspace/ ignored, one tracked
// source file and one file under the ignored directory.
func deployedFixture(t *testing.T) (repo, tracked, ignored string) {
	t.Helper()
	repo = filepath.Join(t.TempDir(), "widgetco")
	tracked = filepath.Join(repo, "internal", "widget", "widget.go")
	ignored = filepath.Join(repo, "workspace", "notes", "NOTES.md")
	for p, body := range map[string]string{
		filepath.Join(repo, ".gitignore"): "workspace/\n",
		tracked:                           "package widget\n",
		ignored:                           "notes\n",
	} {
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	gitInitTempRepo(t, repo)
	return repo, tracked, ignored
}

// deployedHome is a scratch HOME with its own temp directory.
func deployedHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	if err := os.MkdirAll(filepath.Join(home, "tmp"), 0o755); err != nil {
		t.Fatal(err)
	}
	return home
}

func TestDeployedEditHookWritesRuleOutcomeRowsAndSilenceReasons(t *testing.T) {
	repo, tracked, ignored := deployedFixture(t)
	home := deployedHome(t)
	srv, _ := deployedDaemon(t)
	env := deployedEnv(home, repo, srv.URL)
	hooks := deployedEditHooks(t)
	post, postBash := hooks["PostToolUse Edit|Write|MultiEdit"], hooks["PostToolUse Bash"]
	pre, preBash := hooks["PreToolUse Edit|Write|MultiEdit"], hooks["PreToolUse Bash"]
	widgetDir := filepath.Dir(tracked)

	// Served answers: an Edit of a tracked file, twice, and a sed into it
	// from its own directory.
	edit := map[string]any{"file_path": tracked, "old_string": "a", "new_string": "b"}
	runDeployedHook(t, env, pre, repo, edit, "Edit")
	if out := runDeployedHook(t, env, post, repo, edit, "Edit"); !strings.Contains(out, "Determinism is a contract") {
		t.Fatalf("the post-edit hook served nothing for a governed tracked file: %q", out)
	}
	runDeployedHook(t, env, post, repo, edit, "Edit")
	sed := map[string]any{"command": "sed -i '' 's/a/b/' widget.go"}
	runDeployedHook(t, env, preBash, widgetDir, sed, "Bash")
	if out := runDeployedHook(t, env, postBash, widgetDir, sed, "Bash"); !strings.Contains(out, "widget.go") {
		t.Fatalf("the post-bash hook served nothing for a sed into a governed tracked file: %q", out)
	}

	// Silent answers: an ignored file by Edit and by a sed from its own
	// directory, a file outside the repository, a read-only Bash call.
	runDeployedHook(t, env, post, repo, map[string]any{"file_path": ignored, "old_string": "a", "new_string": "b"}, "Edit")
	runDeployedHook(t, env, postBash, filepath.Dir(ignored), map[string]any{"command": "sed -i '' 's/a/b/' NOTES.md"}, "Bash")
	runDeployedHook(t, env, post, repo, map[string]any{"file_path": filepath.Join(t.TempDir(), "elsewhere.go"), "old_string": "a", "new_string": "b"}, "Edit")
	runDeployedHook(t, env, preBash, repo, map[string]any{"command": "git status"}, "Bash")

	// The ledger: rows under the JF-1 names at the default path, keyed by the
	// first served CLAIM, never empty because a raw row topped the set.
	ledger := filepath.Join(home, ".fathom", "rule-outcomes.jsonl")
	rows := readRuleOutcomeRows(t, ledger)
	var got []string
	for _, r := range rows {
		got = append(got, r.RuleID[len(r.RuleID)-1:]+":"+r.Event+":"+r.File)
		switch r.Event {
		case ruleOutcomeServed, ruleOutcomeStillServed, ruleOutcomeTopChanged:
		default:
			t.Errorf("row %+v: event %q is not a JF-1 name", r, r.Event)
		}
		if r.RuleID != deployedClaim {
			t.Errorf("row %+v: keyed by %q, want the first served claim %s (raw rows carry none)", r, r.RuleID, deployedClaim)
		}
	}
	want := []string{
		"1:served:internal/widget/widget.go",                                             // the first edit
		"1:served:internal/widget/widget.go", "1:still_served:internal/widget/widget.go", // the repeat edit
		"1:served:internal/widget/widget.go", "1:still_served:internal/widget/widget.go", // the sed, resolved from its own directory
	}
	if strings.Join(got, " ") != strings.Join(want, " ") {
		t.Fatalf("rule-outcome rows:\n got %v\nwant %v", got, want)
	}
	// The in-process dispatch continues the same session's ledger exactly as
	// a third process would: served, and still_served for the same claim.
	runInProcessHook(t, home, repo, srv.URL, post, repo, edit, "Edit")
	var more []string
	for _, r := range readRuleOutcomeRows(t, ledger)[len(rows):] {
		more = append(more, r.RuleID[len(r.RuleID)-1:]+":"+r.Event)
	}
	if strings.Join(more, " ") != "1:served 1:still_served" {
		t.Fatalf("in-process rows after the deployed ones = %v, want [1:served 1:still_served]", more)
	}

	// The beats: every answer that said nothing names why, from the closed
	// vocabulary; the ones this test drove name the expected reasons.
	reasons := map[string]int{}
	for _, b := range readDeployedBeats(t, filepath.Join(home, ".fathom", "hooks.jsonl")) {
		if b.Channel != "edit-hook" || b.Kind != "answered" {
			continue
		}
		if b.Meta["emitted"] == "false" {
			if !knownSilenceReason(b.Meta["reason"]) {
				t.Errorf("silent answer %v: reason %q is not in the closed vocabulary", b.Meta, b.Meta["reason"])
			}
			reasons[b.Meta["phase"]+"/"+b.Meta["reason"]]++
		}
	}
	for _, k := range []string{"post/gitignored", "post-bash/gitignored", "post/foreign-target", "pre/no-guard"} {
		if reasons[k] == 0 {
			t.Errorf("no silent answer with %s; got %v", k, reasons)
		}
	}
}

// deployedBeat is one beat-log row.
type deployedBeat struct {
	Channel string            `json:"channel"`
	Kind    string            `json:"kind"`
	Meta    map[string]string `json:"meta"`
}

func readDeployedBeats(t *testing.T, path string) []deployedBeat {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("beat log: %v", err)
	}
	defer f.Close()
	var out []deployedBeat
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64<<10), 1<<20)
	for sc.Scan() {
		var b deployedBeat
		if json.Unmarshal(sc.Bytes(), &b) == nil {
			out = append(out, b)
		}
	}
	return out
}

// TestPreServedRuleStillRecordsServed is PR-11 (docs/plans/served-not-kept.md):
// a guard that names the file serves its rules on --pre and stamps them; the
// paired post call consults the stamp. That stamp suppresses the SR-48 ask
// (the pre and post halves of ONE edit are not two turns) and nothing else:
// the pre hook writes no rule-outcome row, so the post call must. Two guarded
// edits of one file: a served row for each, and still_served on the second.
func TestPreServedRuleStillRecordsServed(t *testing.T) {
	repo, tracked, _ := deployedFixture(t)
	writeGuardCue(t, repo, `guards: [{scopes: ["internal/**"], note: "widget code is spec-governed"}]`)
	home := deployedHome(t)
	srv, _ := deployedDaemon(t)
	env := deployedEnv(home, repo, srv.URL)
	hooks := deployedEditHooks(t)
	pre, post := hooks["PreToolUse Edit|Write|MultiEdit"], hooks["PostToolUse Edit|Write|MultiEdit"]
	edit := map[string]any{"file_path": tracked, "old_string": "a", "new_string": "b"}

	var posts []string
	for range 2 {
		if out := runDeployedHook(t, env, pre, repo, edit, "Edit"); !strings.Contains(out, "widget code is spec-governed") {
			t.Fatalf("the guard did not serve on --pre: %q", out)
		}
		posts = append(posts, runDeployedHook(t, env, post, repo, edit, "Edit"))
	}
	for i, out := range posts {
		if !strings.Contains(out, "already served pre-edit") {
			t.Errorf("post call %d did not consult the guard stamp: %q", i+1, out)
		}
		if strings.Contains(out, "served rule ignored") {
			t.Errorf("post call %d asked the SR-48 verdict on its own guarded pair: %q", i+1, out)
		}
	}
	// A third guarded edit through the in-process dispatch: the same rows a
	// process writes.
	runInProcessHook(t, home, repo, srv.URL, pre, repo, edit, "Edit")
	if out := runInProcessHook(t, home, repo, srv.URL, post, repo, edit, "Edit"); !strings.Contains(out, "already served pre-edit") {
		t.Errorf("in-process post call did not consult the guard stamp: %q", out)
	}
	var got []string
	for _, r := range readRuleOutcomeRows(t, filepath.Join(home, ".fathom", "rule-outcomes.jsonl")) {
		got = append(got, r.RuleID[len(r.RuleID)-1:]+":"+r.Event)
	}
	if want := "1:served 1:served 1:still_served 1:served 1:still_served"; strings.Join(got, " ") != want {
		t.Fatalf("rows after three guarded edits = %v, want %s", got, want)
	}
}
