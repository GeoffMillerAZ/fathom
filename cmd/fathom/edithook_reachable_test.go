package main

// edithook_reachable_test.go — DG-11 (diagnostics sweep, O15 regression):
// renderPostEdit's candidate loop used to lock the file's identity to
// whichever candidate's root simply contained the file, then let a LATER,
// unverified basename/git-toplevel guess overwrite that identity if the
// guess happened to return non-empty rules or a stale flag — regardless of
// whether an EARLIER, daemon-verified candidate (one the daemon itself
// records a live ServiceMeta.Path for) had already answered successfully.
// A raw basename guess coincidentally naming a real-but-stale service (the
// exact O15 failure shape: a renamed/duplicated service still present in
// the index under its old name) could then win over the true, reachable
// identity. This pins the fix: once a verified candidate's root matches
// the file and it answers successfully, it is final — no further
// candidate is even queried, so its identity can never be overwritten by
// a coincidental match.

import (
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

// TestRenderPostEdit_PrefersReachableCandidateOverBasename is the DG-11
// contract test: a fake daemon whose list_services records ONE real,
// reachable service ("newname", ServiceMeta.Path == the project dir) whose
// name differs from the project directory's own basename ("oldname" — the
// unverified basename guess serviceCandidates always offers as a
// fallback). The reachable service has no obligations for this file; the
// unverified basename guess, if ever queried, would return some (a
// coincidental stale-alias hit). The reachable candidate must win outright
// — obligations must never even be asked about the unverified name.
func TestRenderPostEdit_PrefersReachableCandidateOverBasename(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "oldname") // project dir basename: "oldname"
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	absFile := filepath.Join(dir, "pkg", "run.go")
	if err := os.MkdirAll(filepath.Dir(absFile), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(absFile, []byte("package pkg\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	var mu sync.Mutex
	var obligationsCalls []string // services obligations was asked about

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

		var inner string
		switch req.Params.Name {
		case "list_services":
			// The ONLY registered service is "newname", whose recorded
			// corpus root IS this project dir — the daemon-verified,
			// reachable identity, even though it doesn't match the dir's
			// own basename ("oldname").
			inner = fmt.Sprintf(`{"services":[{"service":"newname","symbol_count":50,"path":%q}]}`, dir)
		case "obligations":
			svc := strings.ToLower(req.Params.Arguments.Service)
			mu.Lock()
			obligationsCalls = append(obligationsCalls, svc)
			mu.Unlock()
			if svc == "newname" {
				// The true, reachable service: genuinely no obligations
				// for this file — a legitimate empty answer.
				inner = fmt.Sprintf(`{"service":"newname","file":%q,"rules":[]}`, req.Params.Arguments.File)
			} else {
				// The unverified basename guess: if ever asked, answers
				// as though it were a real, stale, still-indexed alias —
				// exactly the coincidental hit that must never win.
				inner = fmt.Sprintf(`{"service":%q,"file":%q,"rules":[{"rule":"stale alias rule — must never render","src":"CLAUDE.md#L1-L1","tier":"obligation","layer":"obligations"}]}`, svc, req.Params.Arguments.File)
			}
		default:
			http.Error(w, "unknown tool "+req.Params.Name, 400)
			return
		}
		env := map[string]any{"result": map[string]any{"content": []map[string]any{{"type": "text", "text": inner}}}}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(env)
	}))
	defer srv.Close()

	text, meta, ok := renderPostEdit(srv.URL, dir, absFile, "sess-dg11", "")

	mu.Lock()
	calls := append([]string(nil), obligationsCalls...)
	mu.Unlock()

	if len(calls) != 1 || calls[0] != "newname" {
		t.Fatalf("obligations calls = %v, want exactly one call to %q (the verified, reachable service) and none to the unverified basename guess", calls, "newname")
	}
	if strings.Contains(text, "stale alias rule") {
		t.Fatalf("rendered text contains the unverified candidate's rule, want it never queried/rendered: %q", text)
	}
	// The reachable candidate legitimately has zero rules for this file, so
	// the render is silent — ok=false, meta zero-valued — but that silence
	// must come from the RIGHT (verified) identity, which the obligations
	// call assertion above already proves.
	if ok && meta.repoRelFile != filepath.Join("pkg", "run.go") {
		t.Errorf("repoRelFile = %q, want pkg/run.go", meta.repoRelFile)
	}
}
