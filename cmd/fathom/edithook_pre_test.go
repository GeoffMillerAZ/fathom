package main

// edithook_pre_test.go — guarded edits (docs/design/guarded-edits.md):
// the --pre hook's local-fast-path/zero-daemon-call contract, guard
// matching/rendering, command guards, and the pre/post dedup handoff.

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
)

func writeGuardCue(t *testing.T, root, content string) {
	t.Helper()
	dir := filepath.Join(root, ".fathom")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "guard.cue"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func countingServer(t *testing.T, handler http.HandlerFunc) (*httptest.Server, *int32) {
	t.Helper()
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		handler(w, r)
	}))
	return srv, &calls
}

// TestEditHookPre_NoGuardFile_ZeroDaemonCalls proves the universal-file
// fast path: no .fathom/guard.cue at all -> instant silence, zero HTTP
// calls (the design's own load-bearing claim, "one local glob check").
func TestEditHookPre_NoGuardFile_ZeroDaemonCalls(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "internal", "core", "engine.go")

	srv, calls := countingServer(t, func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "the daemon must never be queried when no guard.cue exists", 500)
	})
	defer srv.Close()

	hookInput := fmt.Sprintf(`{"tool_name":"Edit","tool_input":{"file_path":%q},"cwd":%q}`, file, dir)
	hookStdin(t, hookInput)

	var out, errBuf bytes.Buffer
	code := runWithStdin([]string{"edit-hook", "--pre", "--url=" + srv.URL}, nil, &out, &errBuf)
	if code != 0 {
		t.Fatalf("exit = %d, want 0", code)
	}
	if out.Len() != 0 {
		t.Errorf("expected silence, got %q", out.String())
	}
	if n := atomic.LoadInt32(calls); n != 0 {
		t.Errorf("daemon queried %d time(s), want 0", n)
	}
}

// TestEditHookPre_GuardFileButNoMatch_ZeroDaemonCalls: a guard.cue EXISTS
// but its scopes don't match the edited file -- still zero daemon calls.
func TestEditHookPre_GuardFileButNoMatch_ZeroDaemonCalls(t *testing.T) {
	dir := t.TempDir()
	writeGuardCue(t, dir, `guards: [{scopes: ["internal/store/**"], note: "spec-governed"}]`)
	file := filepath.Join(dir, "cmd", "main.go") // outside internal/store/**

	srv, calls := countingServer(t, func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "must not be queried for a non-guarded file", 500)
	})
	defer srv.Close()

	hookInput := fmt.Sprintf(`{"tool_name":"Edit","tool_input":{"file_path":%q},"cwd":%q}`, file, dir)
	hookStdin(t, hookInput)

	var out, errBuf bytes.Buffer
	code := runWithStdin([]string{"edit-hook", "--pre", "--url=" + srv.URL}, nil, &out, &errBuf)
	if code != 0 {
		t.Fatalf("exit = %d, want 0", code)
	}
	if out.Len() != 0 {
		t.Errorf("expected silence, got %q", out.String())
	}
	if n := atomic.LoadInt32(calls); n != 0 {
		t.Errorf("daemon queried %d time(s), want 0", n)
	}
}

// TestEditHookPre_MatchedGuard_FetchesAndRenders proves the positive path:
// a guard matches -> exactly the daemon round-trip fires, note-first
// rendering, PreToolUse envelope.
func TestEditHookPre_MatchedGuard_FetchesAndRenders(t *testing.T) {
	// Critic finding 10 (phase-O merge): this test supplies a real
	// session_id, so a matched guard WILL attempt to write a stamp
	// (writeGuardStamp is a no-op only when session_id == "") — without
	// this override it would write to the real machine temp dir,
	// against CLAUDE.md's workspace/ convention.
	orig := guardStampDir
	guardStampDir = filepath.Join(t.TempDir(), "fathom-guard-stamps")
	t.Cleanup(func() { guardStampDir = orig })

	dir := t.TempDir()
	writeGuardCue(t, dir, `guards: [{scopes: ["internal/store/**"], note: "Store schema is spec-governed.", max_rules: 4}]`)
	file := filepath.Join(dir, "internal", "store", "db.go")

	var gotMaxRules float64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Params struct {
				Arguments map[string]any `json:"arguments"`
			} `json:"params"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		gotMaxRules, _ = req.Params.Arguments["max_rules"].(float64)
		cannedPayload := map[string]any{
			"service": strings.ToLower(filepath.Base(dir)),
			"file":    "internal/store/db.go",
			"rules": []any{
				map[string]any{"rule": "Migrations are additive-only.", "src": "docs/spec.md#L1-L2", "tier": "verified"},
			},
		}
		resp := map[string]any{
			"jsonrpc": "2.0", "id": 1,
			"result": map[string]any{"content": []any{
				map[string]any{"type": "text", "text": mustJSON(cannedPayload)},
			}},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	hookInput := fmt.Sprintf(`{"tool_name":"Edit","tool_input":{"file_path":%q},"cwd":%q,"session_id":"sess-1"}`, file, dir)
	hookStdin(t, hookInput)

	var out, errBuf bytes.Buffer
	code := runWithStdin([]string{"edit-hook", "--pre", "--url=" + srv.URL}, nil, &out, &errBuf)
	if code != 0 {
		t.Fatalf("exit = %d, want 0; stderr=%s", code, errBuf.String())
	}
	if gotMaxRules != 4 {
		t.Errorf("daemon received max_rules=%v, want 4 (the guard's own cap)", gotMaxRules)
	}
	outStr := out.String()
	if !strings.Contains(outStr, "PreToolUse") {
		t.Errorf("expected a PreToolUse hookEventName, got %s", outStr)
	}
	if !strings.Contains(outStr, "Store schema is spec-governed.") {
		t.Errorf("expected the guard's note, got %s", outStr)
	}
	if !strings.Contains(outStr, "Migrations are additive-only.") {
		t.Errorf("expected the rule text, got %s", outStr)
	}
	// note must render BEFORE the rule bullet (design §2.1: "served
	// verbatim above the rules").
	noteIdx := strings.Index(outStr, "Store schema is spec-governed.")
	ruleIdx := strings.Index(outStr, "Migrations are additive-only.")
	if noteIdx < 0 || ruleIdx < 0 || noteIdx > ruleIdx {
		t.Errorf("note must precede the rule bullet: noteIdx=%d ruleIdx=%d in %s", noteIdx, ruleIdx, outStr)
	}
}

// TestEditHookPre_ModuleTaggedRule_IncludeModulesToggle proves the guard's
// include_modules field gates DISPLAY of module-tagged rules in the
// pre-edit preview.
func TestEditHookPre_ModuleTaggedRule_IncludeModulesToggle(t *testing.T) {
	// Critic finding 10 (phase-O merge): defensive override — neither
	// subtest currently supplies a session_id (so writeGuardStamp is
	// already a no-op per finding 4), but this keeps the test immune to
	// that detail changing later, matching the pattern used everywhere
	// else a matched guard is exercised.
	orig := guardStampDir
	guardStampDir = filepath.Join(t.TempDir(), "fathom-guard-stamps")
	t.Cleanup(func() { guardStampDir = orig })

	dir := t.TempDir()
	file := filepath.Join(dir, "infra", "main.tf")

	newServer := func() *httptest.Server {
		return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			cannedPayload := map[string]any{
				"service": strings.ToLower(filepath.Base(dir)),
				"file":    "infra/main.tf",
				"rules": []any{
					map[string]any{"rule": "repo rule", "src": "CLAUDE.md#L1-L1", "tier": "doc-claimed"},
					map[string]any{"rule": "module rule", "src": "STANDARDS.md#L1-L1", "tier": "doc-claimed", "module": "platform-standards"},
				},
			}
			resp := map[string]any{
				"jsonrpc": "2.0", "id": 1,
				"result": map[string]any{"content": []any{
					map[string]any{"type": "text", "text": mustJSON(cannedPayload)},
				}},
			}
			json.NewEncoder(w).Encode(resp)
		}))
	}

	t.Run("include_modules false hides the module rule but names the count", func(t *testing.T) {
		writeGuardCue(t, dir, `guards: [{scopes: ["infra/**"], note: "infra"}]`) // include_modules defaults false
		srv := newServer()
		defer srv.Close()
		hookStdin(t, fmt.Sprintf(`{"tool_name":"Edit","tool_input":{"file_path":%q},"cwd":%q}`, file, dir))
		var out, errBuf bytes.Buffer
		runWithStdin([]string{"edit-hook", "--pre", "--url=" + srv.URL}, nil, &out, &errBuf)
		outStr := out.String()
		if strings.Contains(outStr, "[platform-standards] module rule") {
			t.Errorf("the module-tagged rule bullet itself should be hidden with include_modules unset (default false): %s", outStr)
		}
		if !strings.Contains(outStr, "repo rule") {
			t.Errorf("repo rule should still render: %s", outStr)
		}
		// Critic finding 7 (phase-O merge): include_modules:false must
		// never be a SILENT suppression — the count of hidden module
		// rules is named on its own line.
		if !strings.Contains(outStr, "1 module rule(s) hidden (include_modules:false)") {
			t.Errorf("expected the never-silent hidden-count line, got %s", outStr)
		}
	})

	t.Run("include_modules true shows the module rule, tagged", func(t *testing.T) {
		writeGuardCue(t, dir, `guards: [{scopes: ["infra/**"], note: "infra", include_modules: true}]`)
		srv := newServer()
		defer srv.Close()
		hookStdin(t, fmt.Sprintf(`{"tool_name":"Edit","tool_input":{"file_path":%q},"cwd":%q}`, file, dir))
		var out, errBuf bytes.Buffer
		runWithStdin([]string{"edit-hook", "--pre", "--url=" + srv.URL}, nil, &out, &errBuf)
		if !strings.Contains(out.String(), "[platform-standards] module rule") {
			t.Errorf("expected the module-tagged rule rendered, got %s", out.String())
		}
	})
}

// TestEditHookPre_CommandGuard_Matches proves the Bash PreToolUse path:
// note-only injection, zero daemon calls at all (no file/obligations
// lookup for a command).
func TestEditHookPre_CommandGuard_Matches(t *testing.T) {
	dir := t.TempDir()
	writeGuardCue(t, dir, `guards: [{command_scopes: ["terraform apply*", "kubectl *"], note: "Infra commands are guarded; check the runbook first."}]`)

	srv, calls := countingServer(t, func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "command guards must never call the daemon", 500)
	})
	defer srv.Close()

	hookInput := fmt.Sprintf(`{"tool_name":"Bash","tool_input":{"command":"terraform apply -auto-approve"},"cwd":%q}`, dir)
	hookStdin(t, hookInput)

	var out, errBuf bytes.Buffer
	code := runWithStdin([]string{"edit-hook", "--pre", "--url=" + srv.URL}, nil, &out, &errBuf)
	if code != 0 {
		t.Fatalf("exit = %d, want 0", code)
	}
	if !strings.Contains(out.String(), "Infra commands are guarded") {
		t.Errorf("expected the command guard's note, got %s", out.String())
	}
	if n := atomic.LoadInt32(calls); n != 0 {
		t.Errorf("command guard made %d daemon call(s), want 0", n)
	}
}

func TestEditHookPre_CommandGuard_NoMatch_Silent(t *testing.T) {
	dir := t.TempDir()
	writeGuardCue(t, dir, `guards: [{command_scopes: ["terraform apply*"], note: "guarded"}]`)

	hookInput := fmt.Sprintf(`{"tool_name":"Bash","tool_input":{"command":"ls -la"},"cwd":%q}`, dir)
	hookStdin(t, hookInput)

	var out, errBuf bytes.Buffer
	code := runWithStdin([]string{"edit-hook", "--pre", "--url=http://127.0.0.1:1"}, nil, &out, &errBuf)
	if code != 0 || out.Len() != 0 {
		t.Fatalf("exit=%d out=%q, want silent exit 0", code, out.String())
	}
}

// TestEditHookPreThenPost_Dedup is the end-to-end handoff (design §2.3):
// the --pre hook serves a rule and stamps it; the --post hook for the
// SAME (session, file), moments later, must collapse to the "already
// served" marker rather than repeat the rule.
func TestEditHookPreThenPost_Dedup(t *testing.T) {
	orig := guardStampDir
	guardStampDir = filepath.Join(t.TempDir(), "fathom-guard-stamps")
	t.Cleanup(func() { guardStampDir = orig })

	dir := t.TempDir()
	writeGuardCue(t, dir, `guards: [{scopes: ["internal/store/**"], note: "spec-governed"}]`)
	file := filepath.Join(dir, "internal", "store", "db.go")

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cannedPayload := map[string]any{
			"service": strings.ToLower(filepath.Base(dir)),
			"file":    "internal/store/db.go",
			"rules": []any{
				map[string]any{"rule": "Migrations are additive-only.", "src": "docs/spec.md#L1-L2", "tier": "verified"},
			},
		}
		resp := map[string]any{
			"jsonrpc": "2.0", "id": 1,
			"result": map[string]any{"content": []any{
				map[string]any{"type": "text", "text": mustJSON(cannedPayload)},
			}},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	// --pre fires first.
	hookStdin(t, fmt.Sprintf(`{"tool_name":"Edit","tool_input":{"file_path":%q},"cwd":%q,"session_id":"sess-dedup"}`, file, dir))
	var preOut, preErr bytes.Buffer
	if code := runWithStdin([]string{"edit-hook", "--pre", "--url=" + srv.URL}, nil, &preOut, &preErr); code != 0 {
		t.Fatalf("pre exit = %d", code)
	}
	if !strings.Contains(preOut.String(), "Migrations are additive-only.") {
		t.Fatalf("pre should have rendered the rule, got %s", preOut.String())
	}

	// --post fires moments later for the SAME edit/session.
	hookStdin(t, fmt.Sprintf(`{"tool_input":{"file_path":%q},"cwd":%q,"session_id":"sess-dedup"}`, file, dir))
	var postOut, postErr bytes.Buffer
	if code := runWithStdin([]string{"edit-hook", "--url=" + srv.URL}, nil, &postOut, &postErr); code != 0 {
		t.Fatalf("post exit = %d", code)
	}
	postStr := postOut.String()
	if strings.Contains(postStr, "Migrations are additive-only.") {
		t.Errorf("post should NOT repeat the rule already served pre-edit, got %s", postStr)
	}
	if !strings.Contains(postStr, "already served pre-edit") {
		t.Errorf("post must render the never-silent suppression marker, got %s", postStr)
	}
}

// TestEditHookPreThenPost_DifferentSession_NoDedup proves the leakage
// bound: a DIFFERENT session's post-hook must NOT see the first session's
// stamp and must render the full rule.
func TestEditHookPreThenPost_DifferentSession_NoDedup(t *testing.T) {
	orig := guardStampDir
	guardStampDir = filepath.Join(t.TempDir(), "fathom-guard-stamps")
	t.Cleanup(func() { guardStampDir = orig })

	dir := t.TempDir()
	writeGuardCue(t, dir, `guards: [{scopes: ["internal/store/**"], note: "spec-governed"}]`)
	file := filepath.Join(dir, "internal", "store", "db.go")

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cannedPayload := map[string]any{
			"service": strings.ToLower(filepath.Base(dir)),
			"file":    "internal/store/db.go",
			"rules": []any{
				map[string]any{"rule": "Migrations are additive-only.", "src": "docs/spec.md#L1-L2", "tier": "verified"},
			},
		}
		resp := map[string]any{
			"jsonrpc": "2.0", "id": 1,
			"result": map[string]any{"content": []any{
				map[string]any{"type": "text", "text": mustJSON(cannedPayload)},
			}},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	hookStdin(t, fmt.Sprintf(`{"tool_name":"Edit","tool_input":{"file_path":%q},"cwd":%q,"session_id":"sess-A"}`, file, dir))
	var preOut, preErr bytes.Buffer
	runWithStdin([]string{"edit-hook", "--pre", "--url=" + srv.URL}, nil, &preOut, &preErr)

	hookStdin(t, fmt.Sprintf(`{"tool_input":{"file_path":%q},"cwd":%q,"session_id":"sess-B"}`, file, dir))
	var postOut, postErr bytes.Buffer
	runWithStdin([]string{"edit-hook", "--url=" + srv.URL}, nil, &postOut, &postErr)

	if !strings.Contains(postOut.String(), "Migrations are additive-only.") {
		t.Errorf("a different session must see the full rule, not a suppressed marker: %s", postOut.String())
	}
}
