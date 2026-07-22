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
		"startup|resume|clear|compact",
		"Edit|Write|MultiEdit",
	} {
		if !strings.Contains(settingsStr, want) {
			t.Errorf("settings.json missing %q:\n%s", want, settingsStr)
		}
	}

	// CLAUDE.md + AGENTS.md carry the same stanza.
	for _, name := range []string{"CLAUDE.md", "AGENTS.md"} {
		raw, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Fatalf("reading %s: %v", name, err)
		}
		s := string(raw)
		if !strings.Contains(s, fathomStanzaBegin) || !strings.Contains(s, fathomStanzaEnd) {
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

	files := []string{".fathomignore", filepath.Join(".claude", "settings.json"), "CLAUDE.md", "AGENTS.md"}
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
	} {
		if !strings.Contains(out2Str, want) {
			t.Errorf("second run stdout missing %q:\n%s", want, out2Str)
		}
	}
}

func TestMergeHookSettings_PreservesExisting(t *testing.T) {
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

	merged, changed, err := mergeHookSettings(existing, testBinPath)
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
	// hook alongside the unrelated one — never split into a duplicate sibling.
	hooksObj, _ := got["hooks"].(map[string]any)
	ptu, _ := hooksObj["PostToolUse"].([]any)
	if len(ptu) != 1 {
		t.Fatalf("PostToolUse groups = %d, want 1 (merged into the existing matcher group): %v", len(ptu), ptu)
	}
	group, _ := ptu[0].(map[string]any)
	hooks, _ := group["hooks"].([]any)
	if len(hooks) != 2 {
		t.Fatalf("PostToolUse hooks = %d, want 2 (existing + fathom): %v", len(hooks), hooks)
	}

	// Re-running against the merged output is a byte-identical no-op.
	merged2, changed2, err := mergeHookSettings(merged, testBinPath)
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
	newStanza := fathomStanza("new-svc")
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
