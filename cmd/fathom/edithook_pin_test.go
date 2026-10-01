package main

// edithook_pin_test.go — the commit pin line (ADR-057 decision 4,
// docs/design/served-not-kept.md section 3, plan docs/plans/served-not-kept.md
// PR-3 to PR-6) over real git repositories in t.TempDir.

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/config"
	"github.com/geoffmilleraz/fathom/internal/conserve"
	"github.com/geoffmilleraz/fathom/internal/harness"
)

// pinRuleText and pinBodyText are markers no line and no beat may ever
// repeat: the invariant's own statement (the rule text) and a test body.
const (
	pinRuleText = "RULE-TEXT-ROUNDS-HALF-UP"
	pinBodyText = "SECRET-PIN-BODY-7"
)

const pinInvariantsBase = "# Invariants\n\n| # | Invariant | Enforced by | Source |\n|---|---|---|---|\n" +
	"| I1 | Round rounds half up. " + pinRuleText + " | test `TestRoundHalfUp`, `TestRoundDown` | ADR-1 |\n"

const pinRoundGo = "package money\n\nfunc Round(x int) int { return (x + 5) / 10 * 10 }\n"

const pinHalfUpTest = "func TestRoundHalfUp(t *testing.T) {\n\tif Round(15) != 20 {\n\t\tt.Fatal(\"" + pinBodyText + "\")\n\t}\n}\n"

const pinDownTest = "func TestRoundDown(t *testing.T) {\n\tif Round(14) != 10 {\n\t\tt.Fatal(\"down\")\n\t}\n}\n"

const pinTestHeader = "package money\n\nimport \"testing\"\n\n"

// pinGit runs git in repo with a fixed identity and no user or system
// config, and returns its standard output (git's commit summary line goes
// there).
func pinGit(t *testing.T, repo string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", repo}, args...)...)
	cmd.Env = append(os.Environ(),
		"GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=/dev/null",
		"GIT_AUTHOR_NAME=pin", "GIT_AUTHOR_EMAIL=pin@example.com",
		"GIT_COMMITTER_NAME=pin", "GIT_COMMITTER_EMAIL=pin@example.com")
	var out, errb bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errb
	if err := cmd.Run(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, errb.String())
	}
	return out.String()
}

// pinCommitAll stages everything and commits it, returning git's stdout.
func pinCommitAll(t *testing.T, repo, msg string, extra ...string) string {
	t.Helper()
	pinGit(t, repo, "add", "-A")
	return pinGit(t, repo, append([]string{"commit", "-m", msg}, extra...)...)
}

// pinRepo is a git repository whose base commit has an invariant naming
// TestRoundHalfUp and TestRoundDown, both in money/round_test.go. HOME and
// the hooks log are private to the test; mode, when not "", is written as
// the project layer's [hooks] pin_check.
func pinRepo(t *testing.T, mode string) string {
	t.Helper()
	isolateHookRun(t)
	t.Setenv("HOME", t.TempDir())
	t.Setenv("FATHOM_HOOKS_LOG", filepath.Join(t.TempDir(), "hooks.jsonl"))
	t.Setenv("CLAUDE_PROJECT_DIR", "")
	repo, _ := filepath.EvalSymlinks(t.TempDir())
	gitInitTempRepo(t, repo)
	writeRepoFile(t, repo, "docs/intent/invariants.md", pinInvariantsBase)
	writeRepoFile(t, repo, "money/round.go", pinRoundGo)
	writeRepoFile(t, repo, "money/round_test.go", pinTestHeader+pinHalfUpTest+"\n"+pinDownTest)
	if mode != "" {
		writeRepoFile(t, repo, ".fathom/config.toml", "[hooks]\npin_check = \""+mode+"\"\n")
	}
	pinCommitAll(t, repo, "base", "--quiet")
	return repo
}

func pinHook(repo, command, output string) harness.HookInput {
	return harness.HookInput{
		Event: harness.EventPostTool, SessionID: "pin-session", Cwd: repo,
		ToolName: "Bash", Tool: harness.ToolShell, Command: command, ToolOutput: output,
	}
}

func pinClaudeCodec(t *testing.T) harness.Codec {
	t.Helper()
	c, ok := harness.CodecFor(harness.ClaudeCode)
	if !ok {
		t.Fatal("no claude-code codec")
	}
	return c
}

// pinBeats returns every `pin` beat in the test's hooks log, kind first.
func pinBeats(t *testing.T) []map[string]string {
	t.Helper()
	beats, _ := readBeatLog(os.Getenv("FATHOM_HOOKS_LOG"))
	var out []map[string]string
	for _, b := range beats {
		if b.Channel == "edit-hook" && b.Kind == pinBeatKind {
			m := map[string]string{}
			for k, v := range b.Meta {
				m[k] = v
			}
			out = append(out, m)
		}
	}
	return out
}

var pinSummaryShaRe = regexp.MustCompile(`(?m)^\[[^\]]* ([0-9a-f]{4,})\]`)

func pinSha(t *testing.T, gitOut string) string {
	t.Helper()
	m := pinSummaryShaRe.FindStringSubmatch(gitOut)
	if m == nil {
		t.Fatalf("no summary line in %q", gitOut)
	}
	return m[1]
}

// TestCommitPinLineNamesSourceAndIsSilentOnKeep pins PR-3: on a git commit
// seen by PostToolUse(Bash), the sha comes from git's summary line, the
// check runs parent to commit, and each lost pin is one line naming the
// test, its naming source, the loss kind and a head location — under 240
// bytes, never the rule text, never waiver or bypass text, and nothing at
// all when the commit keeps every pin (a named test moved unchanged to
// another file is kept).
func TestCommitPinLineNamesSourceAndIsSilentOnKeep(t *testing.T) {
	repo := pinRepo(t, config.PinCheckFeedback)
	codec := pinClaudeCodec(t)

	// Kept: TestRoundHalfUp moves, body untouched, to its own file.
	writeRepoFile(t, repo, "money/round_test.go", pinTestHeader+pinDownTest)
	writeRepoFile(t, repo, "money/half_test.go", pinTestHeader+pinHalfUpTest)
	out := pinCommitAll(t, repo, "move the half-up test")
	if line := commitPinLine(codec, pinHook(repo, `git add -A && git commit -m "move the half-up test"`, out), false); line != "" {
		t.Fatalf("a commit that keeps every pin printed %q, want silence", line)
	}

	// Lost, renamed: same body, new name.
	writeRepoFile(t, repo, "money/half_test.go", pinTestHeader+strings.Replace(pinHalfUpTest, "TestRoundHalfUp(", "TestRoundHalfUpAgain(", 1))
	out = pinCommitAll(t, repo, "rename")
	sha := pinSha(t, out)
	line := commitPinLine(codec, pinHook(repo, "git commit -am rename", out), false)
	for _, want := range []string{"fathom pin renamed", "TestRoundHalfUp ", "named by", "docs/intent/invariants.md:5", "TestRoundHalfUpAgain", "money/half_test.go:5", sha[:7]} {
		if !strings.Contains(line, want) {
			t.Errorf("renamed line %q does not carry %q", line, want)
		}
	}
	if strings.Contains(line, "source was edited") {
		t.Errorf("renamed line %q says its source was edited; invariants.md was not touched", line)
	}
	assertPinLineShape(t, line, 1)

	// Lost, deleted, while the same commit edits the source that names it:
	// the retirement stays visible and says so.
	writeRepoFile(t, repo, "money/round_test.go", pinTestHeader+"func TestRoundTruncates(t *testing.T) {}\n")
	writeRepoFile(t, repo, "docs/intent/invariants.md", strings.Replace(pinInvariantsBase, ", `TestRoundDown`", "", 1))
	out = pinCommitAll(t, repo, "drop the down test")
	sha = pinSha(t, out)
	line = commitPinLine(codec, pinHook(repo, "cd money && git commit -am 'drop the down test'", out), false)
	for _, want := range []string{"fathom pin deleted", "TestRoundDown", "docs/intent/invariants.md:5", "TestRoundTruncates", "money/round_test.go:5", sha[:7], "(its source was edited in this commit)"} {
		if !strings.Contains(line, want) {
			t.Errorf("deleted line %q does not carry %q", line, want)
		}
	}
	assertPinLineShape(t, line, 1)

	// The same check through `fathom edit-hook`: the PostToolUse(Bash)
	// envelope's stdout is where git printed the summary line.
	repo = pinRepo(t, config.PinCheckFeedback)
	writeRepoFile(t, repo, "money/round_test.go", pinTestHeader+"func TestRoundHalfUp(t *testing.T) {}\n\n"+pinDownTest)
	out = pinCommitAll(t, repo, "weaken")
	stdout := runPinEditHook(t, repo, `git commit -m "weaken"`, out)
	if !strings.Contains(stdout, "additionalContext") || !strings.Contains(stdout, "fathom pin weakened · TestRoundHalfUp ") {
		t.Fatalf("edit-hook stdout %q carries no weakened pin line", stdout)
	}

	// No summary line (a `git commit -q`): no check, no line.
	writeRepoFile(t, repo, "money/round.go", pinRoundGo+"\n// touched\n")
	out = pinCommitAll(t, repo, "quiet", "-q")
	if out != "" {
		t.Fatalf("git commit -q printed %q", out)
	}
	if stdout := runPinEditHook(t, repo, "git commit -qam quiet", out); stdout != "" {
		t.Fatalf("a commit with no summary line printed %q", stdout)
	}
}

// assertPinLineShape checks the line discipline: at most max loss lines,
// each under 240 bytes, no rule text, no source text, no waiver or bypass
// text and no description of the detector.
func assertPinLineShape(t *testing.T, text string, max int) {
	t.Helper()
	lines := strings.Split(text, "\n")
	if len(lines) > max {
		t.Errorf("%d lines, want at most %d: %q", len(lines), max, text)
	}
	for _, l := range lines {
		if len(l) >= 240 {
			t.Errorf("line is %d bytes, want under 240: %q", len(l), l)
		}
	}
	low := strings.ToLower(text)
	for _, banned := range []string{strings.ToLower(pinRuleText), strings.ToLower(pinBodyText), "waive", "bypass", "--no-verify", "ignore", "normalized body", "integrity"} {
		if strings.Contains(low, banned) {
			t.Errorf("line %q carries %q", text, banned)
		}
	}
}

// runPinEditHook runs `fathom edit-hook` in process on a PostToolUse(Bash)
// envelope whose tool_response.stdout is output, and returns its stdout.
func runPinEditHook(t *testing.T, repo, command, output string) string {
	t.Helper()
	srv, _ := countingDaemon(t)
	restore := pipeStdin(t, testRunPostPayload(repo, command, output))
	defer restore()
	var out bytes.Buffer
	if rc := cmdEditHook([]string{"--url", srv.URL}, &out, io.Discard); rc != 0 {
		t.Fatalf("edit-hook exited %d", rc)
	}
	return out.String()
}

// TestCommitPinSkipsSilentlyOverBudget pins PR-4: the stage runs inside a
// 250 ms sub-budget within the hook guard deadline (I53). Over budget, past
// the deadline, or on a read error it prints nothing and its beat says
// skipped with a reason, never kept. CPU-safe: the budget is injected, and
// nothing waits on a wall-clock interval it could lose under -race.
func TestCommitPinSkipsSilentlyOverBudget(t *testing.T) {
	lost := []conserve.PinLoss{{Test: "TestRoundHalfUp", Kind: "deleted"}}
	worked := func(called *bool, losses []conserve.PinLoss, err error) func(context.Context) ([]conserve.PinLoss, error) {
		return func(context.Context) ([]conserve.PinLoss, error) {
			*called = true
			return losses, err
		}
	}
	check := func(t *testing.T, got pinOutcome, status, reason string) {
		t.Helper()
		if got.status != status || got.reason != reason {
			t.Errorf("outcome = %s/%s, want %s/%s", got.status, got.reason, status, reason)
		}
		if status != pinLost && len(got.losses) != 0 {
			t.Errorf("a %s outcome carries %d losses; a skipped check renders nothing", status, len(got.losses))
		}
	}

	t.Run("no budget left: never starts", func(t *testing.T) {
		called := false
		check(t, runCommitPinWithDeadline(context.Background(), 0, worked(&called, lost, nil)), pinSkipped, "budget")
		if called {
			t.Error("the check started with no budget left")
		}
	})
	t.Run("the hook deadline already passed", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		called := false
		check(t, runCommitPinWithDeadline(ctx, time.Hour, worked(&called, lost, nil)), pinSkipped, "deadline")
		if called {
			t.Error("the check started past the hook deadline")
		}
	})
	t.Run("an answer that arrives after the budget is dropped", func(t *testing.T) {
		got := runCommitPinWithDeadline(context.Background(), time.Nanosecond, func(ctx context.Context) ([]conserve.PinLoss, error) {
			<-ctx.Done() // the sub-budget ends; the answer is late
			return lost, nil
		})
		check(t, got, pinSkipped, "budget")
	})
	t.Run("the hook deadline passes mid-check", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		got := runCommitPinWithDeadline(ctx, time.Hour, func(context.Context) ([]conserve.PinLoss, error) {
			cancel()
			return lost, nil
		})
		check(t, got, pinSkipped, "deadline")
	})
	t.Run("a read error", func(t *testing.T) {
		called := false
		check(t, runCommitPinWithDeadline(context.Background(), time.Hour, worked(&called, nil, errors.New("cat-file: broken pipe"))), pinSkipped, "read_error")
	})
	t.Run("a root commit", func(t *testing.T) {
		called := false
		check(t, runCommitPinWithDeadline(context.Background(), time.Hour, worked(&called, nil, pinSkipRootCommit)), pinSkipped, "root_commit")
	})
	t.Run("in time", func(t *testing.T) {
		called := false
		check(t, runCommitPinWithDeadline(context.Background(), time.Hour, worked(&called, lost, nil)), pinLost, "")
		check(t, runCommitPinWithDeadline(context.Background(), time.Hour, worked(&called, nil, nil)), pinKept, "")
	})

	// Through `fathom edit-hook` with the budget spent: a commit that loses a
	// pin prints nothing, and its one beat says skipped, never kept.
	t.Run("edit-hook over budget", func(t *testing.T) {
		repo := pinRepo(t, config.PinCheckFeedback)
		prev := pinStageBudget
		pinStageBudget = 0
		t.Cleanup(func() { pinStageBudget = prev })
		writeRepoFile(t, repo, "money/round_test.go", pinTestHeader+pinDownTest)
		out := pinCommitAll(t, repo, "drop half-up")
		if stdout := runPinEditHook(t, repo, `git commit -am "drop half-up"`, out); stdout != "" {
			t.Fatalf("an over-budget check printed %q", stdout)
		}
		beats := pinBeats(t)
		if len(beats) != 1 || beats[0]["status"] != pinSkipped || beats[0]["reason"] != "budget" {
			t.Fatalf("pin beats = %v, want one skipped/budget", beats)
		}
		if beats[0]["commit"] == "" || !strings.HasPrefix(beats[0]["commit"], pinSha(t, out)) {
			t.Errorf("the skipped beat names commit %q, want %s", beats[0]["commit"], pinSha(t, out))
		}
	})
}

// TestPinBeatCarriesNoSourceText pins PR-5: exactly one `pin` beat per
// checked commit goes into hooks.jsonl (channel edit-hook) with the
// session, the commit, the status, the loss kinds, the test ids, the source
// cites (path:line) and the stage's ms, and never any source text: not the
// rule's words, not a test body. A git commit with no summary line writes
// one skipped beat (reason no_summary) so PR-M1 can name the skip rate; a
// Bash call that commits nothing writes none.
func TestPinBeatCarriesNoSourceText(t *testing.T) {
	repo := pinRepo(t, "") // no [hooks] pin_check anywhere: the default, record

	// Lost: the half-up test renamed, its body (and its secret) unchanged.
	writeRepoFile(t, repo, "money/round_test.go", pinTestHeader+strings.Replace(pinHalfUpTest, "TestRoundHalfUp(", "TestRoundHalfUpRenamed(", 1)+"\n"+pinDownTest)
	out := pinCommitAll(t, repo, "rename half-up")
	sha := pinSha(t, out)
	if stdout := runPinEditHook(t, repo, "git commit -am rename", out); stdout != "" {
		t.Fatalf("record mode printed %q; the payload must not change", stdout)
	}
	beats := pinBeats(t)
	if len(beats) != 1 {
		t.Fatalf("pin beats = %v, want exactly one for the one checked commit", beats)
	}
	b := beats[0]
	want := map[string]string{
		"session": "tr-session", "status": pinLost, "mode": config.PinCheckRecord, "emitted": "false",
		"kinds": "renamed", "tests": "TestRoundHalfUp", "sources": "docs/intent/invariants.md:5", "lost": "1",
	}
	for k, v := range want {
		if b[k] != v {
			t.Errorf("beat %s = %q, want %q (beat %v)", k, b[k], v, b)
		}
	}
	if len(b["commit"]) != 40 || !strings.HasPrefix(b["commit"], sha) {
		t.Errorf("beat commit = %q, want the full sha of %s", b["commit"], sha)
	}
	if _, err := strconv.Atoi(b["ms"]); err != nil {
		t.Errorf("beat ms = %q, want the stage's milliseconds", b["ms"])
	}

	// Kept: one beat, no kinds or tests.
	writeRepoFile(t, repo, "money/round.go", pinRoundGo+"\n// a comment\n")
	out = pinCommitAll(t, repo, "comment")
	runPinEditHook(t, repo, "git commit -am comment", out)
	beats = pinBeats(t)
	if len(beats) != 2 || beats[1]["status"] != pinKept || beats[1]["kinds"] != "" || beats[1]["tests"] != "" {
		t.Fatalf("after a keeping commit, pin beats = %v, want a second, kept, with no kinds or tests", beats)
	}

	// No summary line: one skipped beat naming why, and no commit.
	writeRepoFile(t, repo, "money/round.go", pinRoundGo)
	out = pinCommitAll(t, repo, "quiet", "-q")
	runPinEditHook(t, repo, "git commit -qam quiet", out)
	beats = pinBeats(t)
	if len(beats) != 3 || beats[2]["status"] != pinSkipped || beats[2]["reason"] != "no_summary" || beats[2]["commit"] != "" {
		t.Fatalf("after a git commit -q, pin beats = %v, want a third, skipped/no_summary, with no commit", beats)
	}

	// Not a commit: no beat.
	runPinEditHook(t, repo, "git log --oneline -3", "")
	runPinEditHook(t, repo, "ls -la", "")
	if n := len(pinBeats(t)); n != 3 {
		t.Fatalf("a Bash call that commits nothing wrote a pin beat (%d beats)", n)
	}

	raw, err := os.ReadFile(os.Getenv("FATHOM_HOOKS_LOG"))
	if err != nil {
		t.Fatal(err)
	}
	for _, secret := range []string{pinRuleText, pinBodyText, "Round rounds half up", "Round(15)"} {
		if bytes.Contains(raw, []byte(secret)) {
			t.Errorf("hooks.jsonl carries source text %q", secret)
		}
	}
}

// TestPinCheckOffAndRecordLeaveStdoutByteIdentical pins PR-6's second
// clause: for the same commit, `[hooks] pin_check = "off"` (no check, no
// beat) and "record" (check and beat) print byte-identical stdout, here a
// bash-write payload the same command earned; "feedback" puts the lost-pin
// line in that same envelope. Under an OFF arm stdout is empty and the beat
// is stamped arm off; on a harness other than Claude Code feedback stays
// record-only (PR-V1).
func TestPinCheckOffAndRecordLeaveStdoutByteIdentical(t *testing.T) {
	repo := pinRepo(t, config.PinCheckOff)
	isolateBashDoubt(t)
	t.Setenv("FATHOM_HOOKS_LOG", filepath.Join(t.TempDir(), "hooks.jsonl"))
	srv := fakePromptDaemon(t, map[string]any{
		"NOTES.md": map[string]any{
			"service": strings.ToLower(filepath.Base(repo)), "file": "NOTES.md",
			"rules": []any{map[string]any{"rule": "Notes stay short.", "src": "AGENTS.md#L1-L1", "tier": "doc-claimed", "layer": "obligations"}},
		},
	}, nil)
	defer srv.Close()

	// One commit that loses TestRoundHalfUp, made by a command that also
	// writes NOTES.md (so the bash-write payload is in play).
	writeRepoFile(t, repo, "money/round_test.go", pinTestHeader+pinDownTest)
	writeRepoFile(t, repo, "NOTES.md", "note\n")
	out := pinCommitAll(t, repo, "notes")
	command := `echo note > NOTES.md && git add -A && git commit -m notes`
	run := func(mode string) string {
		t.Helper()
		writeRepoFile(t, repo, ".fathom/config.toml", "[hooks]\npin_check = \""+mode+"\"\n")
		restore := pipeStdin(t, testRunPostPayload(repo, command, out))
		defer restore()
		var stdout bytes.Buffer
		if rc := cmdEditHook([]string{"--url", srv.URL}, &stdout, io.Discard); rc != 0 {
			t.Fatalf("edit-hook exited %d", rc)
		}
		return stdout.String()
	}

	off := run(config.PinCheckOff)
	if n := len(pinBeats(t)); n != 0 {
		t.Fatalf("pin_check off wrote %d pin beat(s); off is no check and no beat", n)
	}
	if !strings.Contains(off, "Notes stay short.") {
		t.Fatalf("the control is blind: the bash-write payload is missing under off: %q", off)
	}
	record := run(config.PinCheckRecord)
	if record != off {
		t.Fatalf("record changed the payload:\noff:    %q\nrecord: %q", off, record)
	}
	if beats := pinBeats(t); len(beats) != 1 || beats[0]["status"] != pinLost {
		t.Fatalf("record wrote pin beats %v, want one lost", beats)
	}

	feedback := run(config.PinCheckFeedback)
	if strings.Count(feedback, "\n") != 1 || !strings.Contains(feedback, "Notes stay short.") || !strings.Contains(feedback, "fathom pin deleted · TestRoundHalfUp ") {
		t.Fatalf("feedback is not one envelope carrying the bash-write rule and the pin line: %q", feedback)
	}
	if beats := pinBeats(t); len(beats) != 2 || beats[1]["emitted"] != "true" {
		t.Fatalf("feedback pin beats = %v, want a second, emitted", beats)
	}

	// Another harness: feedback stays record-only.
	codex, ok := harness.CodecFor(harness.Codex)
	if !ok {
		t.Fatal("no codex codec")
	}
	if line := commitPinLine(codex, pinHook(repo, command, out), false); line != "" {
		t.Errorf("feedback on codex returned %q; other harnesses are record-only", line)
	}
	if beats := pinBeats(t); len(beats) != 3 || beats[2]["status"] != pinLost || beats[2]["emitted"] != "false" {
		t.Errorf("codex pin beats = %v, want a third, lost, not emitted", beats)
	}

	// Under an OFF arm: nothing printed, the beat stamped arm off.
	_, _ = armOffEnv(t, 3)
	if armed := run(config.PinCheckFeedback); armed != "" {
		t.Fatalf("an OFF arm printed %q", armed)
	}
	if beats := pinBeats(t); len(beats) != 4 || beats[3]["arm"] != "off" || beats[3]["emitted"] != "false" {
		t.Fatalf("OFF-arm pin beats = %v, want a fourth stamped arm off", beats)
	}
}
