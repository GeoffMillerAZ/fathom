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
	"sync"
	"testing"
)

// TestEditHookSpecRefsUseStanzaNamedService is the feature-audit regression
// (170 of 306 live spec_refs calls failed with `client.Sources discover
// "sextant"`): a checkout whose directory basename is NOT its service id
// (…/sextant, registered as "fathom") must not send its obligations call —
// which SR-27 (surface reckoning F-35) folded spec_refs' own information
// into as an additive section — the basename when the daemon's identity
// lookup gives no answer. The repo's own stanza names the service — the
// same fallback reconcile's stanzaServiceID uses — so the hook must ask
// for that service, never the directory's name. Before SR-27 this pinned
// a SEPARATE spec_refs RPC's own identity resolution; now there is only
// one call (obligations) to get right, and this test pins that it still
// carries the fix through, spec line and all.
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
			// Only the correctly-resolved "fathom" identity carries a
			// populated spec_refs section — any other candidate the loop
			// probes (the raw "sextant" basename guess, say) gets an empty
			// one, exactly like the real discover-mismatch failure this
			// test pins: the citation must come from the right identity's
			// answer, never a coincidentally-render-alike wrong one.
			p := map[string]any{"service": req.Params.Arguments.Service, "file": req.Params.Arguments.File}
			if req.Params.Arguments.Service == "fathom" {
				p["spec_refs"] = map[string]any{
					"US-042": []any{map[string]any{"file": "docs/specs/us-042.md", "line": 10}},
				}
			}
			payload = p
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
	got := services["obligations"]
	if len(got) == 0 {
		t.Fatalf("obligations never queried; calls=%v", services)
	}
	askedFathom := false
	for _, s := range got {
		if s == "fathom" {
			askedFathom = true
		}
	}
	if !askedFathom {
		t.Errorf("obligations never asked for the stanza-named service \"fathom\" (calls=%v)", services)
	}
	// The spec line must come from the "fathom" identity's own answer —
	// the SR-1 regression this test pins: a wrong (basename-guessed)
	// identity's empty spec_refs must never silently win.
	if !strings.Contains(out.String(), "▸ implements [US-042] — defined at docs/specs/us-042.md:10") {
		t.Errorf("expected the spec line rendered from the stanza-named service's own spec_refs section, got %q", out.String())
	}
}
