package main

// adopterfootprint_test.go — O63 item 5's aggregate proof: `fathom init`
// plus a full hook session (SessionStart, a guarded Read, an edit, Stop)
// must leave a fresh adopter's repository showing NOTHING new in `git
// status` beyond what init itself created. Before this wave, hook output
// (workspace/logs/sessions.jsonl, workspace/logs/observed-discoveries.jsonl,
// workspace/artifacts/retros/*.json, .fathom/receipts.jsonl, MCP call logs)
// landed inside the subject repository's own tree — a wired bench arm's
// diff carried fourteen to sixteen fathom-owned files, and a real adopter
// would find hook output mixed into their own working tree.
// internal/statedir's package doc and each write site's own test
// (TestStoryRows, TestStopHookLandsRetroArtifact, discoveries_test.go, …)
// already pin EVERY individual write landing under statedir.Resolve
// instead — this test is the repo-tree-wide aggregate, so an adopter's
// "does `fathom init` + normal use ever touch my `git status`?" has one
// answer instead of an inference from N separate unit tests.
import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/statedir"
)

// runGitFootprint runs one git subcommand against root and fails the test
// on error — the same bare pattern gitRepo (doctor_test.go) already uses,
// factored out here since this test needs more than gitRepo's fixed
// init+commit sequence (a SECOND commit, after `fathom init` has run, to
// mark its output as the owned baseline).
func runGitFootprint(t *testing.T, root string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return string(out)
}

// TestAdopterFootprint_InitAndHooksWriteNothingOutsideOwnedRegions is the
// O63 item 5 gate: after `fathom init` (committed as the owned baseline)
// and a full session's worth of real hook invocations, `git status
// --porcelain` in the subject repo must be empty — no session log, no
// observed-discoveries log, no retro artifact, no receipts log, nothing
// the hooks wrote has landed inside the repository at all.
func TestAdopterFootprint_InitAndHooksWriteNothingOutsideOwnedRegions(t *testing.T) {
	requireGit(t)
	t.Setenv("HOME", t.TempDir()) // CF-8: never the real home; also guarantees the XDG state dir default resolves OUTSIDE repo by construction
	repo := t.TempDir()
	// gitToplevel resolves through `git rev-parse --show-toplevel`, which
	// canonicalizes symlinks (macOS: /var -> /private/var) — without this,
	// repo and gitToplevel(repo) are the same directory but different
	// STRINGS, so statedir.Resolve(repo) below would hash a different
	// RepoID than the one sessionsLogPathFor/callLogPathFor actually wrote
	// under (same gotcha doctor_test.go's TestCmdDoctor_AdopterRepoIsNotFathom
	// already works around).
	repo, err := filepath.EvalSymlinks(repo)
	if err != nil {
		t.Fatalf("EvalSymlinks(%q): %v", repo, err)
	}
	runGitFootprint(t, repo, "init", "-q")

	svc := strings.ToLower(filepath.Base(repo))
	daemon := fakeInitDaemon(t, svc, 42)
	defer daemon.Close()

	// 1. `fathom init` — the only step allowed to create anything in the
	// tree; its output becomes the "owned regions" baseline.
	var initOut, initErr bytes.Buffer
	if code := cmdInit([]string{"--path", repo, "--url", daemon.URL, "--bin", testBinPath}, &initOut, &initErr); code != 0 {
		t.Fatalf("cmdInit exit = %d; stdout=%s stderr=%s", code, initOut.String(), initErr.String())
	}
	runGitFootprint(t, repo, "add", "-A")
	runGitFootprint(t, repo, "-c", "user.email=t@t", "-c", "user.name=t", "commit", "-qm", "fathom init")
	if status := runGitFootprint(t, repo, "status", "--porcelain"); strings.TrimSpace(status) != "" {
		t.Fatalf("repo not clean right after committing fathom init's own output: %q", status)
	}

	// 2. A full session: SessionStart, a guarded Read (the discovery
	// observer), an edited file (PostToolUse obligations), Stop — every
	// hook verb that O63 found writing into the repo tree.
	const sessionID = "footprint-session-1"
	claudeMD := filepath.Join(repo, "CLAUDE.md")

	t.Setenv("FATHOM_ARM_STATE", filepath.Join(t.TempDir(), "arm.json"))
	t.Setenv("FATHOM_SESSION_LEDGER", filepath.Join(t.TempDir(), "ledger.jsonl"))
	t.Setenv("FATHOM_NARRATE_STATE", filepath.Join(t.TempDir(), "narrate.json"))
	// The hooks beat log is per-user global by design (beats.go), never
	// repo-scoped — pointed at a tempdir purely so this test never
	// touches the real $HOME/.fathom/hooks.jsonl, not because it could
	// ever land inside repo either way.
	t.Setenv("FATHOM_HOOKS_LOG", filepath.Join(t.TempDir(), "hooks.jsonl"))
	t.Setenv("FATHOM_CHANNELS", "")
	resetObsCache()

	// SessionStart
	sessionRaw, _ := json.Marshal(map[string]any{"session_id": sessionID, "source": "startup", "cwd": repo})
	var sOut, sErr bytes.Buffer
	if code := cmdSessionHook([]string{"--url", daemon.URL, "--dir", repo}, bytes.NewReader(sessionRaw), &sOut, &sErr); code != 0 {
		t.Fatalf("session-hook exit %d: %s", code, sErr.String())
	}

	// PreToolUse guarded Read (RC-56's observed-discoveries write path)
	preRaw, _ := json.Marshal(map[string]any{
		"session_id": sessionID,
		"cwd":        repo,
		"tool_name":  "Read",
		"tool_input": map[string]any{"file_path": claudeMD},
	})
	restorePre := pipeStdin(t, string(preRaw))
	var preOut, preErr bytes.Buffer
	if code := cmdEditHook([]string{"--url", daemon.URL, "--pre"}, &preOut, &preErr); code != 0 {
		restorePre()
		t.Fatalf("edit-hook --pre exit %d: %s", code, preErr.String())
	}
	restorePre()

	// PostToolUse edit (obligations injection attempt against the fake
	// daemon — it answers "unknown tool" for obligations, so this
	// exercises the daemon-error silent path, which is exactly the path
	// most real edits with no compiled layer take)
	postRaw, _ := json.Marshal(map[string]any{
		"session_id": sessionID,
		"cwd":        repo,
		"tool_input": map[string]any{"file_path": claudeMD},
	})
	restorePost := pipeStdin(t, string(postRaw))
	var postOut, postErr bytes.Buffer
	if code := cmdEditHook([]string{"--url", daemon.URL}, &postOut, &postErr); code != 0 {
		restorePost()
		t.Fatalf("edit-hook exit %d: %s", code, postErr.String())
	}
	restorePost()

	// Stop
	stopRaw, _ := json.Marshal(map[string]any{
		"session_id":       sessionID,
		"cwd":              repo,
		"stop_hook_active": false,
		"transcript_path":  filepath.Join(t.TempDir(), "nope.jsonl"),
	})
	restoreStop := pipeStdin(t, string(stopRaw))
	var stopOut, stopErr bytes.Buffer
	if code := cmdStopHook([]string{"--url", daemon.URL}, &stopOut, &stopErr); code != 0 {
		restoreStop()
		t.Fatalf("stop-hook exit %d: %s", code, stopErr.String())
	}
	restoreStop()

	// 3. The gate: git status must still be clean. Any hook that wrote
	// inside repo shows up here as an untracked (or modified) path.
	if status := runGitFootprint(t, repo, "status", "--porcelain"); strings.TrimSpace(status) != "" {
		t.Errorf("git status not clean after a full hook session — hooks left a footprint in the repo tree:\n%s", status)
	}

	// 4. Positive evidence: the writes DID happen — just outside the repo,
	// under the state directory — so a clean git status above is not
	// vacuously true because every hook silently failed to run at all.
	stateDir := statedir.Resolve(repo)
	if strings.HasPrefix(stateDir, repo) {
		t.Fatalf("statedir.Resolve(%q) = %q is INSIDE the repo — the default regressed to the pre-O63 in-tree convention", repo, stateDir)
	}
	sessionsLog := filepath.Join(stateDir, "logs", "sessions.jsonl")
	if data, err := os.ReadFile(sessionsLog); err != nil || len(data) == 0 {
		t.Errorf("expected a session window row at %s: %v", sessionsLog, err)
	}
	observedLog := filepath.Join(stateDir, "logs", "observed-discoveries.jsonl")
	if data, err := os.ReadFile(observedLog); err != nil || len(data) == 0 {
		t.Errorf("expected an observed-discovery row at %s: %v", observedLog, err)
	}
}
