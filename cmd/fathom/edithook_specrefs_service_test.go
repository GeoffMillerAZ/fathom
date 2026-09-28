package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

// TestEditHookSpecRefsUseStanzaNamedService is the feature-audit regression
// (170 of 306 live spec_refs calls failed with `client.Sources discover
// "sextant"`): a checkout whose directory basename is NOT its service id
// (…/sextant, registered as "fathom") must not send spec_refs the basename
// when the daemon's identity lookup gives no answer. The repo's own stanza
// names the service — the same fallback reconcile's stanzaServiceID uses —
// so the hook must ask for that service, never the directory's name.
func TestEditHookSpecRefsUseStanzaNamedService(t *testing.T) {
	ceil := os.TempDir()
	if resolved, err := filepath.EvalSymlinks(ceil); err == nil {
		ceil = resolved
	}
	t.Setenv("GIT_CEILING_DIRECTORIES", ceil)

	dir := filepath.Join(t.TempDir(), "sextant")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	stanza := "# notes\n\n<!-- fathom:begin v3 sha256:0 -->\n## fathom code intelligence\n\nThis repo is indexed by fathom as service \"fathom\". Route by\nquestion shape.\n<!-- fathom:end -->\n"
	if err := os.WriteFile(filepath.Join(dir, "CLAUDE.md"), []byte(stanza), 0o644); err != nil {
		t.Fatal(err)
	}
	rel := "internal/core/engine.go"
	abs := writeEditedFile(t, dir, rel, "package core\n\n// implements [US-042]\nfunc Engine() {}\n")

	var (
		mu       sync.Mutex
		services = map[string][]string{} // tool -> services asked
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Params struct {
				Name      string `json:"name"`
				Arguments struct {
					Service string `json:"service"`
					File    string `json:"file"`
					Spec    string `json:"spec"`
				} `json:"arguments"`
			} `json:"params"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		mu.Lock()
		services[req.Params.Name] = append(services[req.Params.Name], req.Params.Arguments.Service)
		mu.Unlock()
		var payload any
		switch req.Params.Name {
		case "obligations":
			payload = map[string]any{"service": req.Params.Arguments.Service, "file": req.Params.Arguments.File}
		case "spec_refs":
			payload = map[string]any{
				"specs":         map[string]any{req.Params.Arguments.Spec: []any{map[string]any{"file": "docs/specs/us-042.md", "line": 10}}},
				"total_refs":    1,
				"files_scanned": 1,
			}
		default: // list_services and friends: the identity lookup has no answer
			http.Error(w, "unexpected tool "+req.Params.Name, 400)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"jsonrpc": "2.0", "id": 1,
			"result": map[string]any{"content": []any{map[string]any{"type": "text", "text": mustJSON(payload)}}},
		})
	}))
	defer srv.Close()

	hookStdin(t, fmt.Sprintf(`{"tool_input":{"file_path":%q},"cwd":%q}`, abs, dir))
	var out, errBuf bytes.Buffer
	if code := runWithStdin([]string{"edit-hook", "--url=" + srv.URL}, nil, &out, &errBuf); code != 0 {
		t.Fatalf("edit-hook exit = %d, want 0 (stderr=%q)", code, errBuf.String())
	}

	mu.Lock()
	defer mu.Unlock()
	got := services["spec_refs"]
	if len(got) == 0 {
		t.Fatalf("spec_refs never queried; calls=%v", services)
	}
	for _, s := range got {
		if s != "fathom" {
			t.Errorf("spec_refs asked for service %q, want the stanza-named \"fathom\" (calls=%v)", s, services)
		}
	}
}
