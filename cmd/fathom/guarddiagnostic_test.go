package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInspectGuardRulesUsesGuardSpecSemantics(t *testing.T) {
	tests := []struct {
		name       string
		body       string
		wantCount  int
		wantNote   string
		wantBroken bool
	}{
		{
			name:      "quoted labels on one line",
			body:      `"guards": [{"scopes": ["**/*.go"], "note": "review changes"}]`,
			wantCount: 1,
		},
		{
			name:      "unquoted labels on one line",
			body:      `guards: [{scopes: ["cmd/**"], note: "review commands"}, {command_scopes: ["deploy *"]}]`,
			wantCount: 2,
		},
		{
			name: "multiline with comments",
			body: `// note: "this comment is not a guard"
				guards: [
					{
						scopes: ["internal/**"]
						note: "first"
					},
					{scopes: ["docs/**"], note: "second"},
				]`,
			wantCount: 2,
		},
		{
			name:      "valid guard need not have a note",
			body:      `guards: [{scopes: ["**/*.go"]}]`,
			wantCount: 1,
		},
		{
			name:     "empty scaffold",
			body:     "// guards are opt-in\n",
			wantNote: "declares no guard rules",
		},
		{
			name:     "explicit empty list",
			body:     `guards: []`,
			wantNote: "declares no guard rules",
		},
		{
			name:       "malformed guard",
			body:       `guards: [{note: "has no scope"}]`,
			wantNote:   "invalid",
			wantBroken: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			writeGuardDiagnosticFixture(t, root, tc.body)
			got := inspectGuardRules(root)
			if got.Count != tc.wantCount {
				t.Errorf("Count = %d, want %d", got.Count, tc.wantCount)
			}
			if tc.wantNote == "" && got.Note != "" {
				t.Errorf("Note = %q, want empty", got.Note)
			} else if tc.wantNote != "" && !strings.Contains(got.Note, tc.wantNote) {
				t.Errorf("Note = %q, want substring %q", got.Note, tc.wantNote)
			}
			if got.Broken != tc.wantBroken {
				t.Errorf("Broken = %v, want %v", got.Broken, tc.wantBroken)
			}
		})
	}
}

func TestGuardDiagnosticPositiveSurfaces(t *testing.T) {
	root := t.TempDir()
	writeGuardDiagnosticFixture(t, root, `"guards": [{"scopes": ["cmd/**"]}, {"command_scopes": ["deploy *"]}]`)
	settings := filepath.Join(root, ".claude", "settings.json")
	if err := os.MkdirAll(filepath.Dir(settings), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(settings, []byte(`{"hooks":{"PostToolUse":[{"hooks":[{"command":"fathom edit-hook"}]}]}}`), 0o644); err != nil {
		t.Fatal(err)
	}

	panel := buildConfigPanel(root, 0)
	if panel.GuardCount != 2 || panel.GuardNote != "" {
		t.Fatalf("panel guards = %d, note %q; want two configured guards", panel.GuardCount, panel.GuardNote)
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/statusz" {
			_ = json.NewEncoder(w).Encode(map[string]any{"services": []any{map[string]any{"service": "fixture"}}})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"jsonrpc": "2.0",
			"id":      1,
			"result": map[string]any{"services": []any{map[string]any{
				"service": "fixture", "symbol_count": 2, "languages": []string{"go"}, "path": root,
			}}},
		})
	}))
	defer srv.Close()

	hermeticObsEnv(t)
	resetObsCache()
	var out, errb bytes.Buffer
	if code := cmdCapabilities([]string{"--url", srv.URL, "--repo", root, "--json"}, &out, &errb); code != 0 {
		t.Fatalf("capabilities exit %d, stderr: %s", code, errb.String())
	}
	var model capabilityModel
	if err := json.Unmarshal(out.Bytes(), &model); err != nil {
		t.Fatalf("decode capabilities JSON: %v\n%s", err, out.String())
	}
	for _, row := range model.Capabilities {
		if row.ID != "guards" {
			continue
		}
		if !row.Operating || row.Detail != "2 guard rules active" {
			t.Fatalf("guards row = %+v, want two active guard rules", row)
		}
		return
	}
	t.Fatal("capabilities JSON has no guards row")
}

func TestGuardDiagnosticDistinguishesMissingMalformedAndUnreadable(t *testing.T) {
	tests := []struct {
		name       string
		arrange    func(*testing.T, string)
		wantPanel  string
		wantReason string
		wantBroken bool
	}{
		{
			name:       "missing",
			arrange:    func(*testing.T, string) {},
			wantPanel:  "no guard file",
			wantReason: "no guard rules are defined",
		},
		{
			name: "malformed",
			arrange: func(t *testing.T, root string) {
				writeGuardDiagnosticFixture(t, root, `guards: [{note: "dead guard"}]`)
			},
			wantPanel:  "invalid",
			wantReason: "invalid",
			wantBroken: true,
		},
		{
			name: "unreadable",
			arrange: func(t *testing.T, root string) {
				if err := os.MkdirAll(filepath.Join(root, ".fathom", "guard.cue"), 0o755); err != nil {
					t.Fatal(err)
				}
			},
			wantPanel:  "source(s) unreadable",
			wantReason: "source(s) unreadable",
			wantBroken: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			tc.arrange(t, root)

			panel := buildConfigPanel(root, 0)
			if !strings.Contains(panel.GuardNote, tc.wantPanel) {
				t.Errorf("panel guard note = %q, want substring %q", panel.GuardNote, tc.wantPanel)
			}

			hermeticObsEnv(t)
			resetObsCache()
			var out, errb bytes.Buffer
			if code := cmdCapabilities([]string{"--url", "http://127.0.0.1:1", "--repo", root, "--json"}, &out, &errb); code != 0 {
				t.Fatalf("capabilities exit %d, stderr: %s", code, errb.String())
			}
			var model capabilityModel
			if err := json.Unmarshal(out.Bytes(), &model); err != nil {
				t.Fatalf("decode capabilities JSON: %v\n%s", err, out.String())
			}
			var guards *capabilityRow
			for i := range model.Capabilities {
				if model.Capabilities[i].ID == "guards" {
					guards = &model.Capabilities[i]
					break
				}
			}
			if guards == nil {
				t.Fatal("capabilities JSON has no guards row")
			}
			if !strings.Contains(guards.Reason, tc.wantReason) {
				t.Errorf("guards reason = %q, want substring %q", guards.Reason, tc.wantReason)
			}
			if guards.Operating {
				t.Errorf("guards unexpectedly operating: %+v", *guards)
			}
			if tc.wantBroken && strings.Contains(guards.Cmd, "init --guards") {
				t.Errorf("broken config must not recommend overwriting it: %+v", *guards)
			}
		})
	}
}

func writeGuardDiagnosticFixture(t *testing.T, root, body string) {
	t.Helper()
	path := filepath.Join(root, ".fathom", "guard.cue")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}
