package main

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/harness"
)

func TestInitTargetsFor(t *testing.T) {
	now := time.Date(2026, 9, 2, 0, 0, 0, 0, time.UTC)

	def, err := initTargetsFor("", now)
	if err != nil {
		t.Fatal(err)
	}
	if def.ID != harness.ClaudeCode || !def.Hooks || !reflect.DeepEqual(def.StanzaFiles, []string{"CLAUDE.md", "AGENTS.md"}) || def.HooksNote != "" {
		t.Errorf("default targets = %+v", def)
	}

	codex, err := initTargetsFor(harness.Codex, now)
	if err != nil {
		t.Fatal(err)
	}
	if codex.Hooks || !reflect.DeepEqual(codex.StanzaFiles, []string{"AGENTS.md"}) || !strings.Contains(codex.HooksNote, "not wired (wave 2)") || !strings.Contains(codex.MCPAddLine, "codex mcp add") {
		t.Errorf("codex targets = %+v", codex)
	}

	gemini, err := initTargetsFor(harness.GeminiCLI, now)
	if err != nil {
		t.Fatal(err)
	}
	if gemini.Hooks || !reflect.DeepEqual(gemini.StanzaFiles, []string{"GEMINI.md"}) {
		t.Errorf("gemini targets = %+v", gemini)
	}

	agents, err := initTargetsFor(harness.Agents, now)
	if err != nil {
		t.Fatal(err)
	}
	if agents.Hooks || !reflect.DeepEqual(agents.StanzaFiles, []string{"AGENTS.md"}) || !strings.Contains(agents.HooksNote, "no hook mechanism") {
		t.Errorf("agents targets = %+v", agents)
	}

	none, err := initTargetsFor(initHarnessNone, now)
	if err != nil {
		t.Fatal(err)
	}
	if none.Hooks || none.StanzaID != harness.Agents || !reflect.DeepEqual(none.StanzaFiles, []string{"CLAUDE.md", "AGENTS.md"}) || none.MCPAddLine != "" {
		t.Errorf("none targets = %+v", none)
	}

	if _, err := initTargetsFor("bogus", now); err == nil || !strings.Contains(err.Error(), "unknown --harness") {
		t.Errorf("unknown id: err = %v", err)
	}

	// HP-16: a descriptor past its demotion clock is refused with the reason.
	demoted := harness.Descriptor{
		ID: "x", Display: "X", Codec: "x", CodecLanded: "2026-01-01", VerifiedAgainst: "unverified", Tier: "A",
		Instructions: []harness.InstructionFile{{Path: "AGENTS.md", Mode: "native"}},
	}
	if _, err := initTargetsForDescriptor(demoted, now); err == nil || !strings.Contains(err.Error(), "refusing --harness=x") || !strings.Contains(err.Error(), "tier D") {
		t.Errorf("demoted descriptor: err = %v", err)
	}
	if _, err := initTargetsForDescriptor(demoted, time.Date(2026, 1, 10, 0, 0, 0, 0, time.UTC)); err != nil {
		t.Errorf("inside the window: %v", err)
	}
}

// TestCmdInitHarnessCodex pins HP-13 end to end: `--harness=codex` writes
// AGENTS.md (with the pull-tier hook sentences) and .fathom/ only, never
// CLAUDE.md or .claude/settings.json, and prints the descriptor's own
// `mcp add` line plus the honest hooks line.
func TestCmdInitHarnessCodex(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	dir := t.TempDir()
	svc := strings.ToLower(filepath.Base(dir))
	srv := fakeInitDaemon(t, svc, 7)
	defer srv.Close()

	var out, errb bytes.Buffer
	code := cmdInit([]string{"--harness", "codex", "--path", dir, "--url", srv.URL, "--bin", testBinPath}, &out, &errb)
	if code != 0 {
		t.Fatalf("exit = %d; stdout=%s stderr=%s", code, out.String(), errb.String())
	}
	agents, err := os.ReadFile(filepath.Join(dir, "AGENTS.md"))
	if err != nil {
		t.Fatalf("AGENTS.md: %v", err)
	}
	body := string(agents)
	if !strings.Contains(body, fathomStanzaBeginPrefix) || !strings.Contains(body, svc) {
		t.Errorf("AGENTS.md lacks the stanza:\n%s", body)
	}
	if strings.Contains(body, "PostToolUse hook") || strings.Contains(body, "(Codex does not)") {
		t.Errorf("the codex stanza must not promise a Claude Code hook:\n%s", body)
	}
	if !strings.Contains(body, "Codex CLI") || !strings.Contains(body, "`obligations` MCP tool") || !strings.Contains(body, "fathom prime") {
		t.Errorf("the codex stanza must name the pull tier:\n%s", body)
	}
	for _, absent := range []string{"CLAUDE.md", filepath.Join(".claude", "settings.json"), "GEMINI.md"} {
		if _, err := os.Stat(filepath.Join(dir, absent)); err == nil {
			t.Errorf("%s must not be written for --harness=codex", absent)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, ".fathom", "guard.cue")); err != nil {
		t.Errorf(".fathom/ scaffolds must still be written: %v", err)
	}
	stdout := out.String()
	for _, want := range []string{
		"codex mcp add fathom --url " + srv.URL + "/rpc/agent",
		"hooks: not wired (wave 2)",
		"fathom harness capabilities --harness codex",
		"MANUAL: add the verify gate to lefthook.yml's pre-push",
	} {
		if !strings.Contains(stdout, want) {
			t.Errorf("stdout missing %q:\n%s", want, stdout)
		}
	}
	if strings.Contains(stdout, "claude mcp add") {
		t.Errorf("--harness=codex must not print the Claude Code registration:\n%s", stdout)
	}
}

func TestCmdInitHarnessNoneAndUnknown(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	dir := t.TempDir()
	svc := strings.ToLower(filepath.Base(dir))
	srv := fakeInitDaemon(t, svc, 7)
	defer srv.Close()

	var out, errb bytes.Buffer
	if code := cmdInit([]string{"--harness", "none", "--path", dir, "--url", srv.URL, "--bin", testBinPath}, &out, &errb); code != 0 {
		t.Fatalf("exit = %d; stdout=%s stderr=%s", code, out.String(), errb.String())
	}
	for _, present := range []string{"CLAUDE.md", "AGENTS.md", filepath.Join(".fathom", "guard.cue")} {
		if _, err := os.Stat(filepath.Join(dir, present)); err != nil {
			t.Errorf("--harness=none must write %s: %v", present, err)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, ".claude", "settings.json")); err == nil {
		t.Error("--harness=none must not write .claude/settings.json")
	}
	if !strings.Contains(out.String(), "hooks: none written") {
		t.Errorf("stdout must say no hooks were written:\n%s", out.String())
	}

	out.Reset()
	errb.Reset()
	if code := cmdInit([]string{"--harness", "bogus", "--path", dir, "--url", srv.URL, "--bin", testBinPath}, &out, &errb); code != 1 || !strings.Contains(errb.String(), "unknown --harness") {
		t.Errorf("unknown harness: exit %d stderr %q", code, errb.String())
	}
}
