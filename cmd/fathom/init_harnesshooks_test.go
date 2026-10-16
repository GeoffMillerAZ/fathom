package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/harness"
	"github.com/geoffmilleraz/fathom/internal/wiring"
)

// TestCodexHookKeyReusesJSONHookEditor pins HP-18 as it stands after
// HP-34's measurement: the SHAPE is still Claude's nesting (KindJSONHook,
// no new editor kind), but codex-cli 0.153.1 does not load
// <repo>/.codex/hooks.json at all, so the descriptor names no writable
// project hook file and the key builder refuses rather than producing a
// key for a file the harness ignores. Gemini CLI, which does honor its
// project settings.json, keeps exercising the builder end to end.
func TestCodexHookKeyReusesJSONHookEditor(t *testing.T) {
	specs := hookSpecsFor(harness.Codex)
	if len(specs) == 0 {
		t.Fatal("codex has no hook specs")
	}
	if k, ok := codexHookKey(specs[0], "/opt/fathom session-hook --harness codex"); ok {
		t.Errorf("codexHookKey must refuse: codex-cli 0.153.1 ignores the project hook file, so writing %s would be wiring that cannot fire (%+v)", k.File, k)
	}
	// The shape itself is unchanged — proven against the user-scope file
	// the harness DOES load, which doctor reads and init never writes.
	d, _ := harness.Lookup(harness.Codex)
	var userFile harness.HookFile
	for _, f := range d.HookFiles {
		if f.Scope == "user" && f.Kind != "" {
			userFile = f
		}
	}
	if userFile.Path != "~/.codex/hooks.json" || userFile.Kind != "json-hook" || userFile.TimeoutUnit != "s" {
		t.Fatalf("codex user hook file = %+v", userFile)
	}
	k := harnessHookKeyFor(userFile, specs[0], "/opt/fathom session-hook --harness codex")
	if k.Kind != wiring.KindJSONHook || k.Marker != "fathom session-hook" || k.Timeout != 10 {
		t.Errorf("key = %+v", k)
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
	// Codex names no writable project hook file (HP-34): nothing is
	// planned, and the note says why rather than failing silently.
	if keys, notes := harnessHookKeys(root, harness.Codex, "/opt/fathom"); len(keys) != 0 || len(notes) != 1 || !strings.Contains(notes[0], "no repo-scoped hook file") {
		t.Errorf("codex must plan nothing: %d keys, notes %v", len(keys), notes)
	}
	// Gemini CLI does honor its project settings.json, so it carries the
	// add-only contract this test exists for.
	const gem = harness.GeminiCLI
	keys, notes := harnessHookKeys(root, gem, "/opt/fathom")
	if len(keys) != len(hookSpecsFor(gem)) || len(notes) != 0 {
		t.Fatalf("fresh root: %d keys, notes %v", len(keys), notes)
	}
	for _, k := range keys {
		if strings.HasPrefix(k.File, "~") || !strings.HasPrefix(k.Body, "command -v /opt/fathom ") || !strings.Contains(k.Body, "; /opt/fathom ") || !strings.Contains(k.Body, "--harness "+gem) {
			t.Errorf("key %+v", k)
		}
	}
	// Pre-existing entry for the first spec, spelled by a human: skipped.
	first := hookSpecsFor(gem)[0]
	existing := map[string]any{"hooks": map[string]any{first.event: []any{map[string]any{
		"matcher": first.matcher,
		"hooks":   []any{map[string]any{"type": "command", "command": first.marker + " --harness " + gem}},
	}}}}
	raw, _ := json.Marshal(existing)
	if err := os.MkdirAll(filepath.Join(root, ".gemini"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".gemini", "settings.json"), raw, 0o644); err != nil {
		t.Fatal(err)
	}
	keys, _ = harnessHookKeys(root, gem, "/opt/fathom")
	if len(keys) != len(hookSpecsFor(gem))-1 {
		t.Errorf("present entry must be skipped: %d keys", len(keys))
	}
	for _, k := range keys {
		if k.Path[0] == first.event && k.Path[1] == first.matcher {
			t.Errorf("the present spec was re-planned: %+v", k)
		}
	}
	// GL-7: Cursor's hook file is flat — init plans nothing and points at
	// the verb that prints the entries.
	if keys, notes := harnessHookKeys(root, "cursor", "/opt/fathom"); len(keys) != 0 || len(notes) != 1 || !strings.Contains(notes[0], "fathom harness hooks --harness cursor") {
		t.Errorf("a flat hook file plans nothing and names the printing verb: %v %v", keys, notes)
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
	// ADR-059 (AG-6): the stanza goes into AGENTS.md, and Gemini CLI reads
	// it through context.fileName — no stanza in GEMINI.md.
	if _, err := os.Stat(filepath.Join(dir, "AGENTS.md")); err != nil {
		t.Errorf("AGENTS.md must be written: %v", err)
	}
	if ctx, _ := settings["context"].(map[string]any); ctx == nil || fmt.Sprint(ctx["fileName"]) != "[AGENTS.md GEMINI.md]" {
		t.Errorf("context.fileName = %v, want [AGENTS.md GEMINI.md]:\n%s", settings["context"], raw)
	}
	for _, absent := range []string{"GEMINI.md", "CLAUDE.md", filepath.Join(".claude", "settings.json"), filepath.Join(".codex", "hooks.json")} {
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

// TestCheckHarnessHookFilesNeverPasses pins the harness_hooks doctor row
// for an UNVERIFIED descriptor with a writable project hook file — the
// state the row exists for. Present entries read as `skipped: unverified`
// (never PASS), missing ones are named with init as the remedy, an
// unreadable file is named rather than counted absent (I38), and a data
// row, a verified descriptor and Claude Code are each skipped with a
// pointer at the row that DOES cover them.
//
// Gemini CLI is the subject now, not Codex: HP-34 verified Codex against a
// real session, so its hook files moved to hooks_installed/hooks_firing —
// which is the whole point of this wave (an unverified descriptor must be
// able to BECOME verified), and leaving the test on Codex would have
// pinned the circular state instead.
func TestCheckHarnessHookFilesNeverPasses(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	dir := t.TempDir()
	svc := strings.ToLower(filepath.Base(dir))
	srv := fakeInitDaemon(t, svc, 7)
	defer srv.Close()

	const gem = harness.GeminiCLI
	n := len(hookSpecsFor(gem))
	c := checkHarnessHookFiles(dir, gem)
	if c.Name != "harness_hooks" || c.Status != doctorSkipped ||
		!strings.Contains(c.Reason, fmt.Sprintf("0 of %d", n)) || !strings.Contains(c.Remedy, "fathom init --harness="+gem) {
		t.Errorf("empty root = %+v", c)
	}
	var out, errb bytes.Buffer
	if code := cmdInit([]string{"--harness", gem, "--path", dir, "--url", srv.URL, "--bin", testBinPath}, &out, &errb); code != 0 {
		t.Fatalf("init: %d %s", code, errb.String())
	}
	c = checkHarnessHookFiles(dir, gem)
	if c.Status != doctorSkipped || !strings.HasPrefix(c.Reason, "skipped: unverified") ||
		!strings.Contains(c.Reason, fmt.Sprintf("%d of %d", n, n)) || strings.Contains(c.Reason, "missing") {
		t.Errorf("after init = %+v", c)
	}
	// Unreadable: named, never "0 of n".
	if err := os.WriteFile(filepath.Join(dir, ".gemini", "settings.json"), []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	c = checkHarnessHookFiles(dir, gem)
	if c.Status != doctorSkipped || !strings.Contains(c.Reason, "did not parse") || !strings.Contains(c.Reason, "invisible") {
		t.Errorf("unparsable = %+v", c)
	}
	if cu := checkHarnessHookFiles(dir, "cursor"); cu.Status != doctorSkipped || !strings.Contains(cu.Reason, "0 of 7") || !strings.Contains(cu.Remedy, "fathom harness hooks --harness cursor") {
		t.Errorf("cursor = %+v", cu)
	}
	if cl := checkHarnessHookFiles(dir, "claude"); cl.Status != doctorSkipped || !strings.Contains(cl.Reason, "hooks_installed") {
		t.Errorf("claude = %+v", cl)
	}
	// Verified: the row steps aside for hooks_installed/hooks_firing.
	if cx := checkHarnessHookFiles(dir, "codex"); cx.Status != doctorSkipped || !strings.Contains(cx.Reason, "hooks_installed") || !strings.Contains(cx.Reason, "verified") {
		t.Errorf("codex (verified) = %+v", cx)
	}
	if b := checkHarnessHookFiles(dir, "bogus"); b.Status != doctorSkipped {
		t.Errorf("bogus = %+v", b)
	}
}

// TestCodexDoctorRowsUseTheMeasuredEvidence is HP-34's unit half: for the
// now-verified Codex descriptor, hooks_installed reads the file the
// harness actually loads (resolved through CODEX_HOME, not $HOME), and
// hooks_firing reads the beat log's `harness` stamp — with a missing
// wiring reported as FAIL and a remedy that does NOT prescribe `init`,
// because init cannot write that file.
func TestCodexDoctorRowsUseTheMeasuredEvidence(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	codexHome := filepath.Join(home, "codexhome")
	if err := os.MkdirAll(codexHome, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("CODEX_HOME", codexHome)
	beatLog := filepath.Join(t.TempDir(), "hooks.jsonl")
	t.Setenv("FATHOM_HOOKS_LOG", beatLog)

	d, _ := harness.Lookup(harness.Codex)
	root := t.TempDir()

	// Nothing wired anywhere: a real FAIL, and the remedy is the printing
	// verb, never `fathom init` (which cannot write a user-global file).
	c := checkHarnessHooksInstalled(root, d)
	if c.Status != doctorFail || !strings.Contains(c.Remedy, "fathom harness hooks --harness codex") {
		t.Errorf("unwired = %+v", c)
	}
	if strings.Contains(c.Remedy, "fathom init") {
		t.Errorf("the remedy must not prescribe init for a file init may not write: %q", c.Remedy)
	}

	// Wire every spec into the CODEX_HOME file. $HOME/.codex is left empty
	// on purpose: a reader that ignored CODEX_HOME would read that instead
	// and report the wrong answer with full confidence.
	specs := hookSpecsFor(harness.Codex)
	events := map[string][]any{}
	for _, sp := range specs {
		g := map[string]any{"hooks": []any{map[string]any{"type": "command", "command": "/opt/fathom " + sp.sub}}}
		if sp.matcher != "" {
			g["matcher"] = sp.matcher
		}
		events[sp.event] = append(events[sp.event], g)
	}
	raw, _ := json.Marshal(map[string]any{"hooks": events})
	if err := os.WriteFile(filepath.Join(codexHome, "hooks.json"), raw, 0o644); err != nil {
		t.Fatal(err)
	}
	if c := checkHarnessHooksInstalled(root, d); c.Status != doctorPass || !strings.Contains(c.Reason, "~/.codex/hooks.json") {
		t.Errorf("wired under CODEX_HOME = %+v", c)
	}

	// hooks_firing: no beat for this harness is a FAIL, and a beat stamped
	// for ANOTHER harness must not be mistaken for one.
	if c := checkHarnessHooksFiring(d); c.Status != doctorFail || !strings.Contains(c.Reason, "harness=codex") {
		t.Errorf("no beats = %+v", c)
	}
	beatHarness = harness.ClaudeCode
	emitBeat("session-hook", "fired", nil)
	if c := checkHarnessHooksFiring(d); c.Status != doctorFail {
		t.Errorf("a claude beat is not codex evidence: %+v", c)
	}
	beatHarness = harness.Codex
	emitBeat("session-hook", "fired", nil)
	beatHarness = ""
	c = checkHarnessHooksFiring(d)
	if c.Status != doctorPass || !strings.Contains(c.Reason, "session-hook×1") {
		t.Errorf("codex beats = %+v", c)
	}
}

func TestCmdHarnessLadder(t *testing.T) {
	var out, errb bytes.Buffer
	if code := run([]string{"harness", "ladder", "--url", "http://127.0.0.1:7581"}, &out, &errb); code != 0 {
		t.Fatalf("exit %d: %s", code, errb.String())
	}
	if !strings.Contains(out.String(), "no injection here") || !strings.Contains(out.String(), "http://127.0.0.1:7581/rpc/agent") || strings.Contains(out.String(), "{{url}}") {
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
