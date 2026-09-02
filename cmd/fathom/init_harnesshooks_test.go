package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/harness"
	"github.com/geoffmilleraz/fathom/internal/wiring"
)

// TestCodexHookKeyReusesJSONHookEditor pins HP-18: the verified Codex
// hooks.json shape is Claude's nesting, so the key is a KindJSONHook
// addressed at .codex/hooks.json with the (event, matcher, marker)
// identity and a seconds timeout — no new editor kind.
func TestCodexHookKeyReusesJSONHookEditor(t *testing.T) {
	specs := hookSpecsFor(harness.Codex)
	if len(specs) == 0 {
		t.Fatal("codex has no hook specs")
	}
	k := codexHookKey(specs[0], "/opt/fathom session-hook --harness codex")
	if k.File != ".codex/hooks.json" || k.Kind != wiring.KindJSONHook || k.Marker != "fathom session-hook" || k.Timeout != 10 {
		t.Errorf("codexHookKey = %+v", k)
	}
	if len(k.Path) != 2 || k.Path[0] != "SessionStart" || k.Path[1] != "startup|resume|clear|compact" {
		t.Errorf("path = %v", k.Path)
	}
	if k.ID() != "SessionStart|startup|resume|clear|compact|fathom session-hook" {
		t.Errorf("ID = %q", k.ID())
	}
	// Gemini's settings.json takes milliseconds: the descriptor says so and
	// the key builder converts.
	g, _ := harness.Lookup(harness.GeminiCLI)
	gf, ok := g.InitHookFile()
	if !ok || gf.Path != ".gemini/settings.json" || gf.TimeoutUnit != "ms" {
		t.Fatalf("gemini init hook file = %+v, %v", gf, ok)
	}
	gk := harnessHookKeyFor(gf, hookSpecsFor(harness.GeminiCLI)[0], "/opt/fathom session-hook --harness gemini-cli")
	if gk.Timeout != 10000 || gk.File != ".gemini/settings.json" || gk.Kind != wiring.KindJSONHook {
		t.Errorf("gemini key = %+v", gk)
	}
	if gk.Path[1] != "" {
		t.Errorf("Gemini lifecycle matchers are exact strings, so SessionStart must match every source: %v", gk.Path)
	}
}

// TestHarnessHookKeysAddOnlyAndNeverUserGlobal pins the planning rules:
// one key per binding into the descriptor's project file, a present entry
// (by marker) is skipped, a data row plans nothing, and no key ever names
// a "~/" path.
func TestHarnessHookKeysAddOnlyAndNeverUserGlobal(t *testing.T) {
	root := t.TempDir()
	keys, notes := harnessHookKeys(root, harness.Codex, "/opt/fathom")
	if len(keys) != len(hookSpecsFor(harness.Codex)) || len(notes) != 0 {
		t.Fatalf("fresh root: %d keys, notes %v", len(keys), notes)
	}
	for _, k := range keys {
		if strings.HasPrefix(k.File, "~") || !strings.HasPrefix(k.Body, "/opt/fathom ") || !strings.Contains(k.Body, "--harness codex") {
			t.Errorf("key %+v", k)
		}
	}
	// Pre-existing entry for the first spec, spelled by a human: skipped.
	first := hookSpecsFor(harness.Codex)[0]
	existing := map[string]any{"hooks": map[string]any{first.event: []any{map[string]any{
		"matcher": first.matcher,
		"hooks":   []any{map[string]any{"type": "command", "command": "fathom session-hook --harness codex"}},
	}}}}
	raw, _ := json.Marshal(existing)
	if err := os.MkdirAll(filepath.Join(root, ".codex"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".codex", "hooks.json"), raw, 0o644); err != nil {
		t.Fatal(err)
	}
	keys, _ = harnessHookKeys(root, harness.Codex, "/opt/fathom")
	if len(keys) != len(hookSpecsFor(harness.Codex))-1 {
		t.Errorf("present entry must be skipped: %d keys", len(keys))
	}
	for _, k := range keys {
		if k.Path[0] == first.event && k.Path[1] == first.matcher {
			t.Errorf("the present spec was re-planned: %+v", k)
		}
	}
	if keys, notes := harnessHookKeys(root, "cursor", "/opt/fathom"); len(keys) != 0 || len(notes) != 0 {
		t.Errorf("a data row plans nothing: %v %v", keys, notes)
	}
	if keys, _ := harnessHookKeys(root, harness.ClaudeCode, "/opt/fathom"); len(keys) == 0 {
		t.Error("claude-code is wired too (init.go keeps its own path, but the builder must not refuse it)")
	}
}

// TestUnverifiedHookSentencesStayHonest pins HP-11 rule 3 for the
// wired-but-unverified case: the stanza names the hook file, the trust
// step, the fallback and the harness — and never the Claude Code literal.
func TestUnverifiedHookSentencesStayHonest(t *testing.T) {
	for _, id := range []string{harness.Codex, harness.GeminiCLI} {
		d, _ := harness.Lookup(id)
		f, _ := d.InitHookFile()
		for _, intentOn := range []bool{false, true} {
			o, c := stanzaHookSentences(id, intentOn)
			for _, want := range []string{f.Path, "unverified here", "`obligations` MCP tool", d.Display, "Treat them as binding, not advisory."} {
				if !strings.Contains(o, want) {
					t.Errorf("%s obligations lacks %q:\n%s", id, want, o)
				}
			}
			for _, want := range []string{f.Path, "fathom prime", "fathom thread note"} {
				if !strings.Contains(c, want) {
					t.Errorf("%s continuity lacks %q:\n%s", id, want, c)
				}
			}
			if o == obligationsPlain || o == obligationsWithIntent || c == continuityClaude {
				t.Errorf("%s must not reuse the Claude Code literals", id)
			}
			if intentOn && !strings.Contains(o, "docs/intent/") {
				t.Errorf("%s: intent wording missing", id)
			}
		}
	}
	// A verified descriptor keeps the Claude Code literals byte for byte.
	if o, c := stanzaHookSentences(harness.ClaudeCode, false); o != obligationsPlain || c != continuityClaude {
		t.Error("claude-code must render the pre-seam literals")
	}
}

// TestCmdInitHarnessGemini pins HP-19's init half: `--harness=gemini-cli`
// writes GEMINI.md and the hooks block of .gemini/settings.json (ms
// timeouts) beside keys already there, prints the httpUrl snippet, and
// touches no user-global file.
func TestCmdInitHarnessGemini(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	dir := t.TempDir()
	svc := strings.ToLower(filepath.Base(dir))
	srv := fakeInitDaemon(t, svc, 7)
	defer srv.Close()

	// An adopter's existing settings (MCP + context) must survive.
	prior := `{"mcpServers":{"other":{"httpUrl":"http://x/mcp"}},"context":{"fileName":["AGENTS.md","GEMINI.md"]}}`
	if err := os.MkdirAll(filepath.Join(dir, ".gemini"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".gemini", "settings.json"), []byte(prior), 0o644); err != nil {
		t.Fatal(err)
	}

	var out, errb bytes.Buffer
	if code := cmdInit([]string{"--harness", "gemini-cli", "--path", dir, "--url", srv.URL, "--bin", testBinPath}, &out, &errb); code != 0 {
		t.Fatalf("exit = %d; stdout=%s stderr=%s", code, out.String(), errb.String())
	}
	raw, err := os.ReadFile(filepath.Join(dir, ".gemini", "settings.json"))
	if err != nil {
		t.Fatal(err)
	}
	var settings map[string]any
	if err := json.Unmarshal(raw, &settings); err != nil {
		t.Fatalf("settings.json: %v\n%s", err, raw)
	}
	if _, ok := settings["mcpServers"].(map[string]any)["other"]; !ok {
		t.Errorf("pre-existing mcpServers key lost:\n%s", raw)
	}
	if _, ok := settings["context"]; !ok {
		t.Errorf("pre-existing context key lost:\n%s", raw)
	}
	hooksObj, _ := settings["hooks"].(map[string]any)
	entries := 0
	for ev, groups := range hooksObj {
		for _, g := range groups.([]any) {
			for _, h := range g.(map[string]any)["hooks"].([]any) {
				hm := h.(map[string]any)
				entries++
				if to, _ := hm["timeout"].(float64); to < 1000 {
					t.Errorf("%s: timeout %v must be milliseconds", ev, to)
				}
				if cmd, _ := hm["command"].(string); !strings.Contains(cmd, "--harness gemini-cli") {
					t.Errorf("%s: %q must spell --harness gemini-cli", ev, cmd)
				}
			}
		}
	}
	if want := len(hookSpecsFor(harness.GeminiCLI)); entries != want {
		t.Errorf("%d hook entries, want %d:\n%s", entries, want, raw)
	}
	if _, ok := hooksObj["AfterTool"]; !ok {
		t.Errorf("AfterTool group missing:\n%s", raw)
	}
	if _, err := os.Stat(filepath.Join(dir, "GEMINI.md")); err != nil {
		t.Errorf("GEMINI.md must be written: %v", err)
	}
	for _, absent := range []string{"CLAUDE.md", filepath.Join(".claude", "settings.json"), filepath.Join(".codex", "hooks.json")} {
		if _, err := os.Stat(filepath.Join(dir, absent)); err == nil {
			t.Errorf("%s must not be written for --harness=gemini-cli", absent)
		}
	}
	// init's harness-neutral user-scope writes (the write-if-absent
	// ~/.fathom/config.toml, the PATH shim) are unchanged; the HARNESS's
	// user-global files are never touched.
	if _, err := os.Stat(filepath.Join(home, ".gemini")); err == nil {
		t.Error("init must never write ~/.gemini (user-global is ask-first)")
	}
	stdout := out.String()
	for _, want := range []string{"httpUrl", srv.URL + "/rpc/agent", "entries written to .gemini/settings.json", "unverified on this machine", "context.fileName"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("stdout missing %q:\n%s", want, stdout)
		}
	}
}

// TestCheckHarnessHookFilesNeverPasses pins the harness_hooks doctor row:
// present entries read as `skipped: unverified` (never PASS), missing ones
// are named with init as the remedy, an unreadable file is named rather
// than counted absent (I38), and a data row / Claude Code are skipped with
// a pointer.
func TestCheckHarnessHookFilesNeverPasses(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	dir := t.TempDir()
	svc := strings.ToLower(filepath.Base(dir))
	srv := fakeInitDaemon(t, svc, 7)
	defer srv.Close()

	c := checkHarnessHookFiles(dir, "codex")
	if c.Name != "harness_hooks" || c.Status != doctorSkipped || !strings.Contains(c.Reason, "0 of 7") || !strings.Contains(c.Remedy, "fathom init --harness=codex") {
		t.Errorf("empty root = %+v", c)
	}
	var out, errb bytes.Buffer
	if code := cmdInit([]string{"--harness", "codex", "--path", dir, "--url", srv.URL, "--bin", testBinPath}, &out, &errb); code != 0 {
		t.Fatalf("init: %d %s", code, errb.String())
	}
	c = checkHarnessHookFiles(dir, "codex")
	if c.Status != doctorSkipped || !strings.HasPrefix(c.Reason, "skipped: unverified") || !strings.Contains(c.Reason, "7 of 7") || strings.Contains(c.Reason, "missing") {
		t.Errorf("after init = %+v", c)
	}
	if !strings.Contains(c.Remedy, "/hooks") {
		t.Errorf("a fully wired file's remedy is the trust step: %+v", c)
	}
	// Unreadable: named, never "0 of 7".
	if err := os.WriteFile(filepath.Join(dir, ".codex", "hooks.json"), []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	c = checkHarnessHookFiles(dir, "codex")
	if c.Status != doctorSkipped || !strings.Contains(c.Reason, "did not parse") || !strings.Contains(c.Reason, "invisible") {
		t.Errorf("unparsable = %+v", c)
	}
	if cu := checkHarnessHookFiles(dir, "cursor"); cu.Status != doctorSkipped || !strings.Contains(cu.Reason, "not wire") {
		t.Errorf("cursor = %+v", cu)
	}
	if cl := checkHarnessHookFiles(dir, "claude"); cl.Status != doctorSkipped || !strings.Contains(cl.Reason, "hooks_installed") {
		t.Errorf("claude = %+v", cl)
	}
	if b := checkHarnessHookFiles(dir, "bogus"); b.Status != doctorSkipped {
		t.Errorf("bogus = %+v", b)
	}
}

func TestCmdHarnessLadder(t *testing.T) {
	var out, errb bytes.Buffer
	if code := run([]string{"harness", "ladder", "--url", "http://127.0.0.1:7575"}, &out, &errb); code != 0 {
		t.Fatalf("exit %d: %s", code, errb.String())
	}
	if !strings.Contains(out.String(), "no injection here") || !strings.Contains(out.String(), "http://127.0.0.1:7575/rpc/agent") || strings.Contains(out.String(), "{{url}}") {
		t.Errorf("ladder table:\n%s", out.String())
	}
	out.Reset()
	if code := run([]string{"harness", "ladder", "--json"}, &out, &errb); code != 0 {
		t.Fatalf("exit %d: %s", code, errb.String())
	}
	var rows []harness.LadderRow
	if err := json.Unmarshal(out.Bytes(), &rows); err != nil || len(rows) != len(harness.LadderRows()) {
		t.Errorf("--json: %v, %d rows", err, len(rows))
	}
}
