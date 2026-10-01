package main

// edithook_pin_test.go — the commit pin line (ADR-057 decision 4,
// docs/design/served-not-kept.md section 3, plan docs/plans/served-not-kept.md
// PR-3 to PR-6) over real git repositories in t.TempDir.

import (
	"bytes"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/config"
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
