package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/config"
)

// fakeInitDaemon serves ingest + list_services shaped like the real MCP
// endpoint (mirrors fakeStopDaemon in stophook_test.go): ingest always
// succeeds, and the service always reports symbolCount symbols thereafter.
func fakeInitDaemon(t *testing.T, service string, symbolCount int) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Params struct {
				Name      string          `json:"name"`
				Arguments json.RawMessage `json:"arguments"`
			} `json:"params"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		var inner string
		switch req.Params.Name {
		case "ingest":
			inner = fmt.Sprintf(`{"service":%q,"ok":true}`, service)
		case "list_services":
			inner = fmt.Sprintf(`{"services":[{"service":%q,"symbol_count":%d,"languages":["go"]}]}`, service, symbolCount)
		default:
			http.Error(w, "unknown tool", 400)
			return
		}
		env := map[string]any{"result": map[string]any{"content": []map[string]any{{"type": "text", "text": inner}}}}
		_ = json.NewEncoder(w).Encode(env)
	}))
}

const testBinPath = "/opt/fathom/bin/fathom"

func TestCmdInit_WiresRepoEndToEnd(t *testing.T) {
	t.Setenv("HOME", t.TempDir()) // CF-8: init now scaffolds ~/.fathom/config.toml — never touch the real home in tests
	dir := t.TempDir()
	svc := strings.ToLower(filepath.Base(dir))
	srv := fakeInitDaemon(t, svc, 123)
	defer srv.Close()

	var out, errBuf bytes.Buffer
	code := cmdInit([]string{"--path", dir, "--url", srv.URL, "--bin", testBinPath}, &out, &errBuf)
	if code != 0 {
		t.Fatalf("exit = %d, want 0; stdout=%s stderr=%s", code, out.String(), errBuf.String())
	}

	// .fathomignore
	ignore, err := os.ReadFile(filepath.Join(dir, ".fathomignore"))
	if err != nil {
		t.Fatalf("reading .fathomignore: %v", err)
	}
	for _, want := range defaultFathomIgnore {
		if !strings.Contains(string(ignore), want) {
			t.Errorf(".fathomignore missing %q:\n%s", want, ignore)
		}
	}

	// .claude/settings.json
	settingsRaw, err := os.ReadFile(filepath.Join(dir, ".claude", "settings.json"))
	if err != nil {
		t.Fatalf("reading settings.json: %v", err)
	}
	var settings map[string]any
	if err := json.Unmarshal(settingsRaw, &settings); err != nil {
		t.Fatalf("settings.json not valid JSON: %v\n%s", err, settingsRaw)
	}
	settingsStr := string(settingsRaw)
	for _, want := range []string{
		testBinPath + " session-hook",
		testBinPath + " edit-hook",
		testBinPath + " stop-hook",
		testBinPath + " edit-hook --pre",
		"startup|resume|clear|compact",
		"Edit|Write|MultiEdit",
	} {
		if !strings.Contains(settingsStr, want) {
			t.Errorf("settings.json missing %q:\n%s", want, settingsStr)
		}
	}

	// Guarded edits (docs/design/guarded-edits.md) + the discovery observer
	// (docs/design/production-tokenomics.md §4): PreToolUse wires THREE
	// matcher groups (Edit|Write|MultiEdit, Bash, and Grep|Glob|Read) with
	// the SAME command — all three must actually be present, not just the
	// marker substring once.
	var settingsTree map[string]any
	if err := json.Unmarshal(settingsRaw, &settingsTree); err != nil {
		t.Fatalf("settings.json not valid JSON: %v", err)
	}
	hooksObj, _ := settingsTree["hooks"].(map[string]any)
	preArr, _ := hooksObj["PreToolUse"].([]any)
	if len(preArr) != 3 {
		t.Fatalf("PreToolUse groups = %d, want 3 (Edit|Write|MultiEdit + Bash + Grep|Glob|Read): %v", len(preArr), preArr)
	}
	gotMatchers := map[string]bool{}
	for _, g := range preArr {
		gm, _ := g.(map[string]any)
		matcher, _ := gm["matcher"].(string)
		gotMatchers[matcher] = true
		hs, _ := gm["hooks"].([]any)
		if len(hs) != 1 {
			t.Errorf("PreToolUse matcher %q hooks = %d, want 1: %v", matcher, len(hs), hs)
		}
	}
	if !gotMatchers["Edit|Write|MultiEdit"] || !gotMatchers["Bash"] {
		t.Errorf("PreToolUse matchers = %v, want both Edit|Write|MultiEdit and Bash", gotMatchers)
	}

	// .fathom/guard.cue + modules.cue: scaffolded, commented (inert).
	for _, name := range []string{"guard.cue", "modules.cue"} {
		raw, err := os.ReadFile(filepath.Join(dir, ".fathom", name))
		if err != nil {
			t.Fatalf("reading .fathom/%s: %v", name, err)
		}
		s := string(raw)
		for _, line := range strings.Split(strings.TrimSpace(s), "\n") {
			if line == "" {
				continue
			}
			if !strings.HasPrefix(strings.TrimSpace(line), "//") {
				t.Errorf(".fathom/%s has an active (non-comment) line: %q\nfull:\n%s", name, line, s)
			}
		}
	}

	// .fathom/config.toml (FL-25): scaffolded, every line commented (`#`,
	// TOML's own comment syntax — guard.cue/modules.cue above are CUE and
	// use `//`), so the findings-loop [findings] block it documents is
	// inert until a human uncomments it.
	{
		raw, err := os.ReadFile(filepath.Join(dir, ".fathom", "config.toml"))
		if err != nil {
			t.Fatalf("reading .fathom/config.toml: %v", err)
		}
		s := string(raw)
		if !strings.Contains(s, "[findings]") {
			t.Errorf(".fathom/config.toml missing a commented [findings] block:\n%s", s)
		}
		for _, want := range []string{"subject", "outbox", "producers"} {
			if !strings.Contains(s, want) {
				t.Errorf(".fathom/config.toml missing %q:\n%s", want, s)
			}
		}
		// DP-1 (docs/vision/32-decision-port.md): a commented [decide]
		// block sits right after [findings], same discipline.
		if !strings.Contains(s, "[decide]") {
			t.Errorf(".fathom/config.toml missing a commented [decide] block:\n%s", s)
		}
		for _, want := range []string{"provider", "profile", "compile_threshold", "route_confidence", "route_timeout_ms", "rank_within_tier"} {
			if !strings.Contains(s, want) {
				t.Errorf(".fathom/config.toml missing %q:\n%s", want, s)
			}
		}
		if strings.Index(s, "[findings]") > strings.Index(s, "[decide]") {
			t.Errorf(".fathom/config.toml: [decide] must come after [findings]:\n%s", s)
		}
		for _, line := range strings.Split(strings.TrimSpace(s), "\n") {
			if line == "" {
				continue
			}
			if !strings.HasPrefix(strings.TrimSpace(line), "#") {
				t.Errorf(".fathom/config.toml has an active (non-comment) line: %q\nfull:\n%s", line, s)
			}
		}
	}

	// CLAUDE.md + AGENTS.md carry the same stanza.
	for _, name := range []string{"CLAUDE.md", "AGENTS.md"} {
		raw, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Fatalf("reading %s: %v", name, err)
		}
		s := string(raw)
		if !strings.Contains(s, fathomStanzaBeginPrefix) || !strings.Contains(s, fathomStanzaEnd) {
			t.Errorf("%s missing markers:\n%s", name, s)
		}
		if !strings.Contains(s, svc) {
			t.Errorf("%s missing service %q:\n%s", name, svc, s)
		}
	}

	stdout := out.String()
	for _, want := range []string{
		"created .fathomignore",
		"created .claude/settings.json",
		"created CLAUDE.md",
		"created AGENTS.md",
		"ok ingest",
		"MANUAL:",
		"/rpc/agent",
	} {
		if !strings.Contains(stdout, want) {
			t.Errorf("stdout missing %q:\n%s", want, stdout)
		}
	}
}

func TestCmdInit_Idempotent(t *testing.T) {
	t.Setenv("HOME", t.TempDir()) // CF-8: see TestCmdInit_WiresRepoEndToEnd
	dir := t.TempDir()
	svc := strings.ToLower(filepath.Base(dir))
	srv := fakeInitDaemon(t, svc, 42)
	defer srv.Close()

	var out1, err1 bytes.Buffer
	if code := cmdInit([]string{"--path", dir, "--url", srv.URL, "--bin", testBinPath}, &out1, &err1); code != 0 {
		t.Fatalf("first run exit = %d; stdout=%s stderr=%s", code, out1.String(), err1.String())
	}

	files := []string{".fathomignore", filepath.Join(".claude", "settings.json"), "CLAUDE.md", "AGENTS.md", filepath.Join(".fathom", "guard.cue"), filepath.Join(".fathom", "modules.cue"), filepath.Join(".fathom", "config.toml")}
	before := map[string][]byte{}
	for _, f := range files {
		raw, err := os.ReadFile(filepath.Join(dir, f))
		if err != nil {
			t.Fatalf("reading %s after first run: %v", f, err)
		}
		before[f] = raw
	}

	var out2, err2 bytes.Buffer
	if code := cmdInit([]string{"--path", dir, "--url", srv.URL, "--bin", testBinPath}, &out2, &err2); code != 0 {
		t.Fatalf("second run exit = %d; stdout=%s stderr=%s", code, out2.String(), err2.String())
	}

	for _, f := range files {
		raw, err := os.ReadFile(filepath.Join(dir, f))
		if err != nil {
			t.Fatalf("reading %s after second run: %v", f, err)
		}
		if !bytes.Equal(raw, before[f]) {
			t.Errorf("%s changed on second (idempotent) run:\n--- before ---\n%s\n--- after ---\n%s", f, before[f], raw)
		}
	}

	out2Str := out2.String()
	for _, want := range []string{
		"ok .fathomignore (already present)",
		"ok .claude/settings.json (already present)",
		"ok CLAUDE.md (already present)",
		"ok AGENTS.md (already present)",
		"ok .fathom/guard.cue (already present)",
		"ok .fathom/modules.cue (already present)",
		"ok .fathom/config.toml (already present)",
	} {
		if !strings.Contains(out2Str, want) {
			t.Errorf("second run stdout missing %q:\n%s", want, out2Str)
		}
	}
}

func TestMergeHookSettings_PreservesExisting(t *testing.T) {
	t.Setenv("HOME", t.TempDir()) // mergeHookSettings now reads user-scope ~/.claude/settings.json — must not see the real one
	existing := []byte(`{
  "customTopLevelKey": "keep-me",
  "hooks": {
    "PostToolUse": [
      {
        "matcher": "Edit|Write|MultiEdit",
        "hooks": [
          {"type": "command", "command": "/some/other/lint.sh", "timeout": 30}
        ]
      }
    ]
  }
}`)

	merged, changed, _, err := mergeHookSettings(existing, testBinPath)
	if err != nil {
		t.Fatalf("mergeHookSettings: %v", err)
	}
	if !changed {
		t.Fatal("expected changed=true on first merge")
	}

	var got map[string]any
	if err := json.Unmarshal(merged, &got); err != nil {
		t.Fatalf("merged output not valid JSON: %v\n%s", err, merged)
	}
	if got["customTopLevelKey"] != "keep-me" {
		t.Errorf("unrelated top-level key lost: %v", got)
	}

	mergedStr := string(merged)
	if !strings.Contains(mergedStr, "/some/other/lint.sh") {
		t.Errorf("pre-existing PostToolUse hook lost:\n%s", mergedStr)
	}
	for _, want := range []string{
		testBinPath + " session-hook",
		testBinPath + " edit-hook",
		testBinPath + " stop-hook",
		"startup|resume|clear|compact",
	} {
		if !strings.Contains(mergedStr, want) {
			t.Errorf("merged settings missing %q:\n%s", want, mergedStr)
		}
	}

	// The pre-existing PostToolUse group (same matcher) must gain the fathom
	// hook alongside the unrelated one — never split into a duplicate
	// sibling. O57 item 4/5(b) added a SECOND PostToolUse matcher group
	// (Bash, the write-detecting edit-hook), so there are now two groups —
	// exactly the shape PreToolUse's three matcher groups already prove
	// ensureHookEntry's per-group marker scoping handles correctly.
	hooksObj, _ := got["hooks"].(map[string]any)
	ptu, _ := hooksObj["PostToolUse"].([]any)
	if len(ptu) != 2 {
		t.Fatalf("PostToolUse groups = %d, want 2 (Edit|Write|MultiEdit merged into the existing matcher group, Bash a new sibling): %v", len(ptu), ptu)
	}
	group, _ := ptu[0].(map[string]any)
	if group["matcher"] != "Edit|Write|MultiEdit" {
		t.Fatalf("PostToolUse group 0 matcher = %v, want Edit|Write|MultiEdit", group["matcher"])
	}
	hooks, _ := group["hooks"].([]any)
	if len(hooks) != 2 {
		t.Fatalf("PostToolUse Edit|Write|MultiEdit hooks = %d, want 2 (existing + fathom): %v", len(hooks), hooks)
	}
	bashGroup, _ := ptu[1].(map[string]any)
	if bashGroup["matcher"] != "Bash" {
		t.Fatalf("PostToolUse group 1 matcher = %v, want Bash", bashGroup["matcher"])
	}
	bashHooks, _ := bashGroup["hooks"].([]any)
	if len(bashHooks) != 1 {
		t.Fatalf("PostToolUse Bash hooks = %d, want 1 (fathom only): %v", len(bashHooks), bashHooks)
	}

	// Re-running against the merged output is a byte-identical no-op.
	merged2, changed2, _, err := mergeHookSettings(merged, testBinPath)
	if err != nil {
		t.Fatalf("second mergeHookSettings: %v", err)
	}
	if changed2 {
		t.Error("second merge reported changed=true, want false (idempotent)")
	}
	if !bytes.Equal(merged, merged2) {
		t.Errorf("second merge produced different bytes:\n--- first ---\n%s\n--- second ---\n%s", merged, merged2)
	}
}

// TestMergeHookSettings_SkipsUserScopeWiring covers the fix for the live
// dogfood bug: `fathom init` wrote project-scope SessionStart/PostToolUse/
// Stop hooks that DUPLICATED identical ones already wired globally at user
// scope (~/.claude/settings.json), so the post-hook fired twice per edit
// (4596 logged calls in the reproduction). mergeHookSettings now reads user
// scope first and skips any fathom hook already wired there.
func TestMergeHookSettings_SkipsUserScopeWiring(t *testing.T) {
	t.Run("user scope has the pre-phase-O three — project gets only the PreToolUse guard groups", func(t *testing.T) {
		home := t.TempDir()
		t.Setenv("HOME", home)
		userSettings := `{"hooks":{
			"SessionStart":[{"matcher":"startup|resume|clear|compact","hooks":[{"type":"command","command":"` + testBinPath + ` session-hook","timeout":10}]}],
			"PostToolUse":[{"matcher":"Edit|Write|MultiEdit","hooks":[{"type":"command","command":"` + testBinPath + ` edit-hook","timeout":5}]}],
			"Stop":[{"hooks":[{"type":"command","command":"` + testBinPath + ` stop-hook","timeout":10}]}]
		}}`
		if err := os.MkdirAll(filepath.Join(home, ".claude"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(home, ".claude", "settings.json"), []byte(userSettings), 0o644); err != nil {
			t.Fatal(err)
		}

		merged, changed, skipped, err := mergeHookSettings(nil, testBinPath)
		if err != nil {
			t.Fatalf("mergeHookSettings: %v", err)
		}
		if !changed {
			t.Fatal("expected changed=true — the PreToolUse guard groups are still genuinely missing")
		}
		if len(skipped) != 3 {
			t.Fatalf("skipped = %v, want 3 (session-hook, edit-hook post, stop-hook)", skipped)
		}

		var got map[string]any
		if err := json.Unmarshal(merged, &got); err != nil {
			t.Fatalf("merged output not valid JSON: %v\n%s", err, merged)
		}
		hooksObj, _ := got["hooks"].(map[string]any)
		if _, ok := hooksObj["SessionStart"]; ok {
			t.Errorf("SessionStart must not be written at project scope — already wired at user scope: %v", hooksObj)
		}
		if _, ok := hooksObj["Stop"]; ok {
			t.Errorf("Stop must not be written at project scope — already wired at user scope: %v", hooksObj)
		}
		// The Edit|Write|MultiEdit group is already wired at user scope and
		// must not be duplicated at project scope; the Bash group (O57 item
		// 4/5b) is genuinely missing from the user-scope fixture above (only
		// the pre-phase-O three are wired there) and so DOES get written —
		// its own matcher, not a dupe of Edit|Write|MultiEdit's.
		ptu, _ := hooksObj["PostToolUse"].([]any)
		if len(ptu) != 1 {
			t.Fatalf("PostToolUse groups = %d, want 1 (only Bash, genuinely missing at user scope): %v", len(ptu), ptu)
		}
		group, _ := ptu[0].(map[string]any)
		if group["matcher"] != "Bash" {
			t.Errorf("PostToolUse group matcher = %v, want Bash (Edit|Write|MultiEdit is already wired at user scope)", group["matcher"])
		}
		pre, _ := hooksObj["PreToolUse"].([]any)
		if len(pre) != 3 {
			t.Fatalf("PreToolUse groups = %d, want 3 (all three guard/discovery groups, genuinely missing)", len(pre))
		}
		mergedStr := string(merged)
		if !strings.Contains(mergedStr, testBinPath+" edit-hook --pre") {
			t.Errorf("PreToolUse guard hooks missing from merged output:\n%s", mergedStr)
		}
	})

	t.Run("user scope empty — project gets everything", func(t *testing.T) {
		t.Setenv("HOME", t.TempDir()) // fresh HOME, no ~/.claude/settings.json at all
		merged, changed, skipped, err := mergeHookSettings(nil, testBinPath)
		if err != nil {
			t.Fatalf("mergeHookSettings: %v", err)
		}
		if !changed {
			t.Fatal("expected changed=true")
		}
		if len(skipped) != 0 {
			t.Fatalf("skipped = %v, want none (no user-scope wiring exists)", skipped)
		}
		var got map[string]any
		if err := json.Unmarshal(merged, &got); err != nil {
			t.Fatalf("merged output not valid JSON: %v\n%s", err, merged)
		}
		hooksObj, _ := got["hooks"].(map[string]any)
		for _, event := range []string{"SessionStart", "PostToolUse", "PreToolUse", "Stop"} {
			if _, ok := hooksObj[event]; !ok {
				t.Errorf("%s missing from project settings — user scope is empty, everything should install", event)
			}
		}
	})

	t.Run("user scope wires a DIFFERENT command for the same event — fathom's own still installs", func(t *testing.T) {
		home := t.TempDir()
		t.Setenv("HOME", home)
		userSettings := `{"hooks":{"PostToolUse":[{"matcher":"Edit|Write|MultiEdit","hooks":[{"type":"command","command":"/some/other/lint.sh","timeout":30}]}]}}`
		if err := os.MkdirAll(filepath.Join(home, ".claude"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(home, ".claude", "settings.json"), []byte(userSettings), 0o644); err != nil {
			t.Fatal(err)
		}

		merged, changed, skipped, err := mergeHookSettings(nil, testBinPath)
		if err != nil {
			t.Fatalf("mergeHookSettings: %v", err)
		}
		if !changed {
			t.Fatal("expected changed=true — fathom's own edit-hook must still install")
		}
		if len(skipped) != 0 {
			t.Fatalf("skipped = %v, want none — the user-scope hook on this event/matcher is someone else's, not fathom's", skipped)
		}
		mergedStr := string(merged)
		if !strings.Contains(mergedStr, testBinPath+" edit-hook") {
			t.Errorf("fathom's own edit-hook must still be installed at project scope:\n%s", mergedStr)
		}
	})

	t.Run("re-running after a skip stays idempotent — never adds the skipped hook later", func(t *testing.T) {
		home := t.TempDir()
		t.Setenv("HOME", home)
		userSettings := `{"hooks":{"Stop":[{"hooks":[{"type":"command","command":"` + testBinPath + ` stop-hook","timeout":10}]}]}}`
		if err := os.MkdirAll(filepath.Join(home, ".claude"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(home, ".claude", "settings.json"), []byte(userSettings), 0o644); err != nil {
			t.Fatal(err)
		}

		merged, _, skipped, err := mergeHookSettings(nil, testBinPath)
		if err != nil {
			t.Fatalf("mergeHookSettings: %v", err)
		}
		if len(skipped) != 1 {
			t.Fatalf("skipped = %v, want 1 (stop-hook)", skipped)
		}
		merged2, changed2, skipped2, err := mergeHookSettings(merged, testBinPath)
		if err != nil {
			t.Fatalf("second mergeHookSettings: %v", err)
		}
		if changed2 {
			t.Error("second merge reported changed=true — stop-hook must stay skipped, not get added on re-run")
		}
		if len(skipped2) != 1 {
			t.Fatalf("second run skipped = %v, want 1 (stop-hook still skipped)", skipped2)
		}
		if !bytes.Equal(merged, merged2) {
			t.Errorf("second merge produced different bytes:\n--- first ---\n%s\n--- second ---\n%s", merged, merged2)
		}
	})
}

func TestFathomStanza_MarkerReplace(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "CLAUDE.md")
	original := "# My Project\n\nSome intro text.\n\n" +
		fathomStanzaBegin + "\nSTALE CONTENT — service \"old-svc\"\n" + fathomStanzaEnd +
		"\n\n## Later section\n\nMore content below.\n"
	if err := os.WriteFile(path, []byte(original), 0o644); err != nil {
		t.Fatal(err)
	}

	var out bytes.Buffer
	newStanza := fathomStanza("new-svc", false)
	if err := applyStanza(path, newStanza, &out); err != nil {
		t.Fatalf("applyStanza: %v", err)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	gotStr := string(got)

	for _, want := range []string{"# My Project", "Some intro text.", "## Later section", "More content below."} {
		if !strings.Contains(gotStr, want) {
			t.Errorf("surrounding content lost, missing %q:\n%s", want, gotStr)
		}
	}
	if strings.Contains(gotStr, "STALE CONTENT") || strings.Contains(gotStr, "old-svc") {
		t.Errorf("stale stanza content survived replacement:\n%s", gotStr)
	}
	if !strings.Contains(gotStr, "new-svc") {
		t.Errorf("new stanza content missing:\n%s", gotStr)
	}
	if !strings.Contains(out.String(), "updated CLAUDE.md") {
		t.Errorf("expected an 'updated' line, got: %s", out.String())
	}

	// Re-applying the identical stanza is a byte-identical no-op.
	out.Reset()
	if err := applyStanza(path, newStanza, &out); err != nil {
		t.Fatalf("second applyStanza: %v", err)
	}
	got2, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, got2) {
		t.Error("second applyStanza changed file bytes")
	}
	if !strings.Contains(out.String(), "already present") {
		t.Errorf("second applyStanza should report already present: %s", out.String())
	}
}

// TestFathomStanza_AppendsToExistingFileWithoutMarkers is AK-3's
// "append-to-existing" case: a repo's own CLAUDE.md that predates fathom
// entirely (real content, no fathom:begin/end markers anywhere) must gain
// the stanza appended, with every byte of the original content preserved.
func TestFathomStanza_AppendsToExistingFileWithoutMarkers(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "CLAUDE.md")
	original := "# My Project\n\nSome pre-existing operating notes, no fathom here yet.\n"
	if err := os.WriteFile(path, []byte(original), 0o644); err != nil {
		t.Fatal(err)
	}

	var out bytes.Buffer
	stanza := fathomStanza("my-svc", false)
	if err := applyStanza(path, stanza, &out); err != nil {
		t.Fatalf("applyStanza: %v", err)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	gotStr := string(got)
	if !strings.Contains(gotStr, "Some pre-existing operating notes") {
		t.Errorf("original content lost:\n%s", gotStr)
	}
	if !strings.Contains(gotStr, fathomStanzaBeginPrefix) || !strings.Contains(gotStr, fathomStanzaEnd) {
		t.Errorf("stanza markers missing after append:\n%s", gotStr)
	}
	if !strings.Contains(gotStr, "my-svc") {
		t.Errorf("stanza body missing after append:\n%s", gotStr)
	}
	if !strings.Contains(out.String(), "updated CLAUDE.md") {
		t.Errorf("expected an 'updated' line, got: %s", out.String())
	}

	// Re-applying is a no-op once the markers exist (replace path, not append).
	out.Reset()
	if err := applyStanza(path, stanza, &out); err != nil {
		t.Fatalf("second applyStanza: %v", err)
	}
	got2, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, got2) {
		t.Error("second applyStanza changed file bytes")
	}
	if !strings.Contains(out.String(), "already present") {
		t.Errorf("second applyStanza should report already present: %s", out.String())
	}
}

// TestFathomStanza_CorruptedMarkers_Refuses is AK-3's "marker-corruption
// refusal" case: a hand-edit that leaves only one marker (or reorders them)
// must not be silently papered over by appending a second block — applyStanza
// must return an error and leave the file byte-for-byte unchanged.
func TestFathomStanza_CorruptedMarkers_Refuses(t *testing.T) {
	cases := map[string]string{
		"begin_only":       "# Project\n\n" + fathomStanzaBegin + "\nstuff, no closing marker\n\nmore content\n",
		"end_only":         "# Project\n\nstuff\n" + fathomStanzaEnd + "\n\nmore content\n",
		"end_before_begin": "# Project\n\n" + fathomStanzaEnd + "\nstuff\n" + fathomStanzaBegin + "\nmore\n",
		"end_equals_begin": "# Project\n\n" + fathomStanzaEnd + fathomStanzaBegin + "\nmore\n",
	}
	for name, original := range cases {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "CLAUDE.md")
			if err := os.WriteFile(path, []byte(original), 0o644); err != nil {
				t.Fatal(err)
			}

			var out bytes.Buffer
			err := applyStanza(path, fathomStanza("svc", false), &out)
			if err == nil {
				t.Fatal("expected applyStanza to refuse on corrupted markers, got nil error")
			}
			if !strings.Contains(err.Error(), "corrupted") {
				t.Errorf("error doesn't mention corruption: %v", err)
			}

			got, rerr := os.ReadFile(path)
			if rerr != nil {
				t.Fatal(rerr)
			}
			if string(got) != original {
				t.Errorf("file was modified despite refusal:\nwant:\n%s\ngot:\n%s", original, got)
			}
			if out.String() != "" {
				t.Errorf("expected no stdout on refusal, got: %s", out.String())
			}
		})
	}
}

// ─── e. ~/.fathom/config.toml scaffold (CF-8) ─────────────────────────────

func TestWriteStarterConfig_CreatesFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".fathom", "config.toml")

	var out bytes.Buffer
	if err := writeStarterConfig(path, &out); err != nil {
		t.Fatalf("writeStarterConfig: %v", err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading scaffolded config: %v", err)
	}
	for _, want := range []string{"claude-cli", "anthropic", "bedrock", "azure-openai", "us-gov-west-1", "azure.us", "api_key_env", "api_key_cmd"} {
		if !strings.Contains(string(raw), want) {
			t.Errorf("scaffolded config missing %q:\n%s", want, raw)
		}
	}
	if strings.Contains(strings.ToLower(string(raw)), "\napi_key =") {
		t.Error("scaffolded config must never contain a literal api_key assignment")
	}
	if !strings.Contains(out.String(), "created") {
		t.Errorf("stdout = %s, want a created-file message", out.String())
	}
}

func TestWriteStarterConfig_WriteIfAbsent_NeverOverwrites(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".fathom", "config.toml")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	custom := `default_profile = "mine"` + "\n"
	if err := os.WriteFile(path, []byte(custom), 0o644); err != nil {
		t.Fatal(err)
	}

	var out bytes.Buffer
	if err := writeStarterConfig(path, &out); err != nil {
		t.Fatalf("writeStarterConfig: %v", err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) != custom {
		t.Errorf("writeStarterConfig overwrote an existing file:\n%s", raw)
	}
	if !strings.Contains(out.String(), "already present") {
		t.Errorf("stdout = %s, want an already-present message", out.String())
	}
}

func TestWriteStarterConfig_EmptyPath_NoOp(t *testing.T) {
	var out bytes.Buffer
	if err := writeStarterConfig("", &out); err != nil {
		t.Fatalf("writeStarterConfig(\"\"): %v", err)
	}
	if out.String() != "" {
		t.Errorf("expected silent no-op for an empty path, got %q", out.String())
	}
}

// TestWriteStarterConfig_UncommentedProfilesAreValid proves every commented
// profile block is genuinely well-formed once uncommented — not just
// plausible-looking prose — by stripping the leading "# " from each
// candidate block and round-tripping it through config.Load's real TOML+CUE
// validation.
func TestWriteStarterConfig_UncommentedProfilesAreValid(t *testing.T) {
	blocks := [][]string{
		{"default_profile = \"personal\"", "[profiles.personal]", "provider = \"claude-cli\""},
		{"[profiles.api]", "provider = \"anthropic\"", "api_key_env = \"ANTHROPIC_API_KEY\""},
		{"[profiles.gov-bedrock]", "provider = \"bedrock\"", "region = \"us-gov-west-1\"", "model = \"anthropic.claude-3-5-sonnet-20241022-v2:0\""},
		{"[profiles.gov-azure]", "provider = \"azure-openai\"", "endpoint = \"https://example-resource.openai.azure.us\"", "deployment = \"dep\"", "api_key_env = \"AZURE_OPENAI_API_KEY\""},
		{"[daemon]", "store = \"workspace/db/fleet.sqlite\"", "addr = \"127.0.0.1:7575\"", "auto_refresh = true", "default_profile = \"personal\""},
	}
	for i, lines := range blocks {
		doc := strings.Join(lines, "\n") + "\n"
		dir := t.TempDir()
		path := filepath.Join(dir, "config.toml")
		if err := os.WriteFile(path, []byte(doc), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := config.Load(path, ""); err != nil {
			t.Errorf("block %d failed to validate once uncommented:\n%s\nerror: %v", i, doc, err)
		}
	}
}

func TestCmdInit_ScaffoldsConfig(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	dir := t.TempDir()
	svc := strings.ToLower(filepath.Base(dir))
	srv := fakeInitDaemon(t, svc, 7)
	defer srv.Close()

	var out, errBuf bytes.Buffer
	code := cmdInit([]string{"--path", dir, "--url", srv.URL, "--bin", testBinPath}, &out, &errBuf)
	if code != 0 {
		t.Fatalf("exit = %d; stdout=%s stderr=%s", code, out.String(), errBuf.String())
	}
	if _, err := os.Stat(filepath.Join(home, ".fathom", "config.toml")); err != nil {
		t.Errorf("~/.fathom/config.toml not scaffolded: %v", err)
	}
	if !strings.Contains(out.String(), "config.toml") {
		t.Errorf("stdout = %s, want it to mention the config scaffold", out.String())
	}
}

func TestCmdInit_DaemonDown(t *testing.T) {
	t.Setenv("HOME", t.TempDir()) // CF-8: see TestCmdInit_WiresRepoEndToEnd
	dir := t.TempDir()
	var out, errBuf bytes.Buffer
	// Port 1 is never listening — the connection fails fast (mirrors the
	// daemon-down convention used by sessionhook_test.go / stophook_test.go).
	code := cmdInit([]string{"--path", dir, "--url", "http://127.0.0.1:1", "--bin", testBinPath}, &out, &errBuf)
	if code != 2 {
		t.Fatalf("exit = %d, want 2; stdout=%s stderr=%s", code, out.String(), errBuf.String())
	}

	// Wiring must still be written even though the daemon is unreachable.
	for _, f := range []string{".fathomignore", filepath.Join(".claude", "settings.json"), "CLAUDE.md", "AGENTS.md"} {
		if _, err := os.Stat(filepath.Join(dir, f)); err != nil {
			t.Errorf("%s not written despite daemon-down: %v", f, err)
		}
	}

	stdout := out.String()
	if !strings.Contains(stdout, "MANUAL:") {
		t.Errorf("expected MANUAL lines on daemon-down: %s", stdout)
	}
	if !strings.Contains(stdout, "unreachable") {
		t.Errorf("expected an unreachable-daemon message: %s", stdout)
	}
}

func TestResolveHooksScope(t *testing.T) {
	t.Run("default (empty) is auto", func(t *testing.T) {
		got, err := resolveHooksScope("")
		if err != nil || got != hooksScopeAuto {
			t.Fatalf("resolveHooksScope(\"\") = %q, %v; want %q, nil", got, err, hooksScopeAuto)
		}
	})
	t.Run("explicit project", func(t *testing.T) {
		got, err := resolveHooksScope("project")
		if err != nil || got != hooksScopeProject {
			t.Fatalf("resolveHooksScope(\"project\") = %q, %v; want %q, nil", got, err, hooksScopeProject)
		}
	})
	t.Run("explicit auto", func(t *testing.T) {
		got, err := resolveHooksScope("auto")
		if err != nil || got != hooksScopeAuto {
			t.Fatalf("resolveHooksScope(\"auto\") = %q, %v; want %q, nil", got, err, hooksScopeAuto)
		}
	})
	t.Run("invalid value refused", func(t *testing.T) {
		if _, err := resolveHooksScope("bogus"); err == nil {
			t.Fatal("resolveHooksScope(\"bogus\") = nil error, want a refusal")
		}
	})
	t.Run("FATHOM_HOOKS_SCOPE=project forces project when flag left at auto", func(t *testing.T) {
		t.Setenv("FATHOM_HOOKS_SCOPE", "project")
		got, err := resolveHooksScope("auto")
		if err != nil || got != hooksScopeProject {
			t.Fatalf("resolveHooksScope(\"auto\") with env = %q, %v; want %q, nil", got, err, hooksScopeProject)
		}
	})
	t.Run("explicit --hooks-scope=project always wins regardless of env", func(t *testing.T) {
		t.Setenv("FATHOM_HOOKS_SCOPE", "project")
		got, err := resolveHooksScope("project")
		if err != nil || got != hooksScopeProject {
			t.Fatalf("resolveHooksScope(\"project\") with env = %q, %v; want %q, nil", got, err, hooksScopeProject)
		}
	})
}

// TestCmdInit_HooksScopeProject_OverridesUserScope is the regression test
// for the clean-room/CI defect: on a machine where fathom's hooks are
// already wired at Claude Code USER scope (~/.claude/settings.json — the
// operator's own machine, or any harness that seeded one), `fathom init`'s
// default `auto` scope skips writing them at PROJECT scope, so a harness
// that only ever loads project settings (`claude --restricted --settings
// <path>`, docs/bench/scenarios.md "The clean room") sees NO fathom hooks
// at all. `--hooks-scope project` must write SessionStart, PostToolUse
// (the post edit-hook) and Stop at project scope regardless.
func TestCmdInit_HooksScopeProject_OverridesUserScope(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	userSettings := `{"hooks":{
		"SessionStart":[{"matcher":"startup|resume|clear|compact","hooks":[{"type":"command","command":"` + testBinPath + ` session-hook","timeout":10}]}],
		"PostToolUse":[{"matcher":"Edit|Write|MultiEdit","hooks":[{"type":"command","command":"` + testBinPath + ` edit-hook","timeout":5}]}],
		"Stop":[{"hooks":[{"type":"command","command":"` + testBinPath + ` stop-hook","timeout":10}]}]
	}}`
	if err := os.MkdirAll(filepath.Join(home, ".claude"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, ".claude", "settings.json"), []byte(userSettings), 0o644); err != nil {
		t.Fatal(err)
	}

	dir := t.TempDir()
	svc := strings.ToLower(filepath.Base(dir))
	srv := fakeInitDaemon(t, svc, 7)
	defer srv.Close()

	var out, errBuf bytes.Buffer
	code := cmdInit([]string{"--path", dir, "--url", srv.URL, "--bin", testBinPath, "--hooks-scope", "project"}, &out, &errBuf)
	if code != 0 {
		t.Fatalf("exit = %d, want 0; stdout=%s stderr=%s", code, out.String(), errBuf.String())
	}

	settingsRaw, err := os.ReadFile(filepath.Join(dir, ".claude", "settings.json"))
	if err != nil {
		t.Fatalf("reading settings.json: %v", err)
	}
	var settings map[string]any
	if err := json.Unmarshal(settingsRaw, &settings); err != nil {
		t.Fatalf("settings.json not valid JSON: %v\n%s", err, settingsRaw)
	}
	hooksObj, _ := settings["hooks"].(map[string]any)
	if sessionStart, _ := hooksObj["SessionStart"].([]any); len(sessionStart) == 0 {
		t.Errorf("SessionStart missing at project scope under --hooks-scope project (user scope already had it wired): %v", hooksObj)
	}
	if postToolUse, _ := hooksObj["PostToolUse"].([]any); len(postToolUse) == 0 {
		t.Errorf("PostToolUse missing at project scope under --hooks-scope project: %v", hooksObj)
	}
	if stop, _ := hooksObj["Stop"].([]any); len(stop) == 0 {
		t.Errorf("Stop missing at project scope under --hooks-scope project: %v", hooksObj)
	}

	settingsStr := string(settingsRaw)
	for _, want := range []string{
		testBinPath + " session-hook",
		testBinPath + " edit-hook",
		testBinPath + " stop-hook",
	} {
		if !strings.Contains(settingsStr, want) {
			t.Errorf("settings.json missing %q even though --hooks-scope project should force it:\n%s", want, settingsStr)
		}
	}

	if !strings.Contains(out.String(), "wired at project scope (--hooks-scope project)") {
		t.Errorf("stdout missing the --hooks-scope project note:\n%s", out.String())
	}
}
