package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDoctorClaudeProjectRegistration(t *testing.T) {
	const base = "http://127.0.0.1:7581"
	const valid = `{"mcpServers":{"fathom":{"type":"http","url":"http://127.0.0.1:7581/rpc/agent"}}}`
	for _, tc := range []struct {
		name, project, user string
		want                doctorStatus
	}{
		{"project without user config", valid, "", doctorPass},
		{"project with unrelated user state", valid, `{"mcpServers":{}}`, doctorPass},
		{"project overrides wrong user endpoint", valid, strings.ReplaceAll(valid, ":7581", ":9999"), doctorPass},
		{"user remains supported", "", valid, doctorPass},
		{"unrelated project server preserves user", `{"mcpServers":{"other":{"type":"http","url":"http://other/rpc/agent"}}}`, valid, doctorPass},
		{"wrong name", strings.ReplaceAll(valid, `"fathom"`, `"other"`), "", doctorFail},
		{"wrong transport", strings.ReplaceAll(valid, `"http"`, `"sse"`), valid, doctorFail},
		{"wrong host", strings.ReplaceAll(valid, "127.0.0.1", "other.invalid"), valid, doctorFail},
		{"wrong port", strings.ReplaceAll(valid, ":7581", ":9999"), valid, doctorFail},
		{"wrong surface", strings.ReplaceAll(valid, "/rpc/agent", "/rpc"), valid, doctorFail},
		{"query is a different endpoint", strings.ReplaceAll(valid, "/rpc/agent", "/rpc/agent?token=DO_NOT_PRINT"), valid, doctorFail},
		{"credential URL refused without echo", strings.ReplaceAll(valid, "http://127", "http://DO_NOT_PRINT@127"), valid, doctorFail},
		{"companion alias", strings.ReplaceAll(valid, "/rpc/agent", "/rpc/companion/"), "", doctorPass},
		{"malformed project cannot fall back", `{`, valid, doctorSkipped},
		{"nonobject project", `null`, valid, doctorSkipped},
		{"malformed user could hide local override", valid, `{`, doctorSkipped},
		{"unexpanded variable is unknown", strings.ReplaceAll(valid, "127.0.0.1:7581", "${FATHOM_ADDR}"), "", doctorSkipped},
	} {
		t.Run(tc.name, func(t *testing.T) {
			home, root := t.TempDir(), t.TempDir()
			t.Setenv("HOME", home)
			before := map[string]string{}
			for path, body := range map[string]string{filepath.Join(home, ".claude.json"): tc.user, filepath.Join(root, ".mcp.json"): tc.project} {
				if body != "" {
					if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
						t.Fatal(err)
					}
					before[path] = body
				}
			}
			got := checkMCPRegistration(root, "claude", base)
			if got.Status != tc.want {
				t.Fatalf("got %+v, want %v", got, tc.want)
			}
			if strings.Contains(got.Reason+got.Remedy, "DO_NOT_PRINT") {
				t.Fatal("diagnostic echoed URL credentials/query")
			}
			if strings.Contains(got.Remedy, "--scope user") {
				t.Fatal("project check recommends global registration")
			}
			if got.Status == doctorPass && (!strings.Contains(got.Reason, "configured") || !strings.Contains(got.Reason, "not checked")) {
				t.Fatalf("config presence must not certify trust or consumption: %+v", got)
			}
			for path, body := range before {
				after, err := os.ReadFile(path)
				if err != nil || string(after) != body {
					t.Fatalf("read-only doctor changed %s", path)
				}
			}
			if tc.user == "" {
				if _, err := os.Stat(filepath.Join(home, ".claude.json")); !os.IsNotExist(err) {
					t.Fatal("doctor created user config")
				}
			}
		})
	}
}

func TestDoctorClaudeLocalScopePrecedesProject(t *testing.T) {
	home, root, other := t.TempDir(), t.TempDir(), t.TempDir()
	t.Setenv("HOME", home)
	entry := func(port string) map[string]string {
		return map[string]string{"type": "http", "url": "http://127.0.0.1:" + port + "/rpc/agent"}
	}
	doc := map[string]any{"mcpServers": map[string]any{"fathom": entry("7581")}, "projects": map[string]any{
		root:  map[string]any{"mcpServers": map[string]any{"fathom": entry("9999")}},
		other: map[string]any{"mcpServers": map[string]any{"fathom": entry("9998")}},
	}}
	data, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, ".claude.json"), data, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".mcp.json"), []byte(`{"mcpServers":{"fathom":{"type":"http","url":"http://127.0.0.1:7581/rpc/agent"}}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := checkMCPRegistration(root, "claude", "http://127.0.0.1:7581"); got.Status != doctorFail || !strings.Contains(got.Reason, "local scope") {
		t.Fatalf("a local override must not be hidden by valid project/user entries: %+v", got)
	}
	delete(doc["projects"].(map[string]any), root)
	data, err = json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, ".claude.json"), data, 0o600); err != nil {
		t.Fatal(err)
	}
	if got := checkMCPRegistration(root, "claude", "http://127.0.0.1:7581"); got.Status != doctorPass || !strings.Contains(got.Reason, "project scope") {
		t.Fatalf("another project's local config must not affect this root: %+v", got)
	}
}
