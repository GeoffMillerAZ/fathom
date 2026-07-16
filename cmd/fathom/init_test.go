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

func TestCmdInit_DaemonDown(t *testing.T) {
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
