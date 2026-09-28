package main

// edithook_blast_test.go — SR-60 (surface reckoning, design §6): the
// pre-edit hook adds a one-line "N callers, M tests reach this" when the
// edited file has callers, fed from an additional bounded blast_radius
// call that degrades to silence (never blocks or errors the guard
// payload) on any failure — the fetchMemories precedent (edithook.go).

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
)

// ─── editHookBlastLine: pure formatting ──────────────────────────────────

func TestEditHookBlastLine(t *testing.T) {
	cases := []struct {
		name    string
		callers int
		tests   int
		want    string
	}{
		{"no callers is silent", 0, 5, ""},
		{"negative callers is silent", -1, 5, ""},
		{"callers only, no tests", 3, 0, "3 callers reach this"},
		{"one caller, no tests (singular)", 1, 0, "1 caller reach this"},
		{"callers and tests", 3, 2, "3 callers, 2 tests reach this"},
		{"one caller, one test (singular)", 1, 1, "1 caller, 1 test reach this"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := editHookBlastLine(c.callers, c.tests)
			if got != c.want {
				t.Errorf("editHookBlastLine(%d, %d) = %q, want %q", c.callers, c.tests, got, c.want)
			}
		})
	}
}

// ─── end-to-end: pre-edit hook ───────────────────────────────────────────

// TestPreEditHookBlastLine proves the --pre path calls blast_radius scoped
// to the edited file and appends the one-line summary after everything
// else the guard renders, without disturbing the governing-rules content.
func TestPreEditHookBlastLine(t *testing.T) {
	orig := guardStampDir
	guardStampDir = filepath.Join(t.TempDir(), "fathom-guard-stamps")
	t.Cleanup(func() { guardStampDir = orig })

	dir := t.TempDir()
	writeGuardCue(t, dir, `guards: [{scopes: ["internal/store/**"], note: "Store schema is spec-governed."}]`)
	file := filepath.Join(dir, "internal", "store", "db.go")

	var gotBlastFile string
	mux := http.NewServeMux()
	mux.HandleFunc("/rpc", func(w http.ResponseWriter, r *http.Request) {
		switch rpcRequestName(r) {
		case "obligations":
			json.NewEncoder(w).Encode(rpcTextResult(map[string]any{
				"service": strings.ToLower(filepath.Base(dir)),
				"file":    "internal/store/db.go",
				"rules": []any{
					map[string]any{"rule": "Migrations are additive-only.", "src": "docs/spec.md#L1-L2", "tier": "verified"},
				},
			}))
		case "blast_radius":
			var req struct {
				Params struct {
					Arguments struct {
						ChangedFiles []string `json:"changed_files"`
					} `json:"arguments"`
				} `json:"params"`
			}
			body, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(body, &req)
			if len(req.Params.Arguments.ChangedFiles) == 1 {
				gotBlastFile = req.Params.Arguments.ChangedFiles[0]
			}
			json.NewEncoder(w).Encode(rpcTextResult(map[string]any{
				"total_symbols": 3,
				"tests":         []string{"TestA", "TestB"},
			}))
		default:
			json.NewEncoder(w).Encode(rpcTextResult(map[string]any{"rules": []any{}}))
		}
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	hookInput := fmt.Sprintf(`{"tool_name":"Edit","tool_input":{"file_path":%q},"cwd":%q,"session_id":"sess-blast"}`, file, dir)
	hookStdin(t, hookInput)

	var out, errBuf bytes.Buffer
	code := runWithStdin([]string{"edit-hook", "--pre", "--url=" + srv.URL}, nil, &out, &errBuf)
	if code != 0 {
		t.Fatalf("exit = %d, want 0; stderr=%s", code, errBuf.String())
	}
	outStr := out.String()
	if !strings.Contains(outStr, "Migrations are additive-only.") {
		t.Errorf("governing rule must still render, got %s", outStr)
	}
	if !strings.Contains(outStr, "3 callers, 2 tests reach this") {
		t.Errorf("expected the blast-radius line, got %s", outStr)
	}
	if gotBlastFile != "internal/store/db.go" {
		t.Errorf("blast_radius scoped to changed_files=%q, want [internal/store/db.go]", gotBlastFile)
	}
	// The blast line must be the LAST thing rendered — a trailer, never
	// interleaved with the governing rule bullet.
	ruleIdx := strings.Index(outStr, "Migrations are additive-only.")
	blastIdx := strings.Index(outStr, "3 callers, 2 tests reach this")
	if ruleIdx < 0 || blastIdx < 0 || blastIdx < ruleIdx {
		t.Errorf("blast line must render after the rule bullet: ruleIdx=%d blastIdx=%d in %s", ruleIdx, blastIdx, outStr)
	}
}

// TestPreEditHookBlastLine_NoCallers proves the "when the edited symbol
// has callers" gate: a zero-caller blast_radius answer adds no line and
// no trailing blank content.
func TestPreEditHookBlastLine_NoCallers(t *testing.T) {
	orig := guardStampDir
	guardStampDir = filepath.Join(t.TempDir(), "fathom-guard-stamps")
	t.Cleanup(func() { guardStampDir = orig })

	dir := t.TempDir()
	writeGuardCue(t, dir, `guards: [{scopes: ["internal/store/**"], note: "Store schema is spec-governed."}]`)
	file := filepath.Join(dir, "internal", "store", "db.go")

	mux := http.NewServeMux()
	mux.HandleFunc("/rpc", func(w http.ResponseWriter, r *http.Request) {
		switch rpcRequestName(r) {
		case "obligations":
			json.NewEncoder(w).Encode(rpcTextResult(map[string]any{
				"service": strings.ToLower(filepath.Base(dir)),
				"file":    "internal/store/db.go",
				"rules": []any{
					map[string]any{"rule": "Migrations are additive-only.", "src": "docs/spec.md#L1-L2", "tier": "verified"},
				},
			}))
		case "blast_radius":
			json.NewEncoder(w).Encode(rpcTextResult(map[string]any{
				"total_symbols": 0,
				"tests":         []string{},
			}))
		default:
			json.NewEncoder(w).Encode(rpcTextResult(map[string]any{"rules": []any{}}))
		}
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	hookInput := fmt.Sprintf(`{"tool_name":"Edit","tool_input":{"file_path":%q},"cwd":%q,"session_id":"sess-blast-2"}`, file, dir)
	hookStdin(t, hookInput)

	var out, errBuf bytes.Buffer
	code := runWithStdin([]string{"edit-hook", "--pre", "--url=" + srv.URL}, nil, &out, &errBuf)
	if code != 0 {
		t.Fatalf("exit = %d, want 0; stderr=%s", code, errBuf.String())
	}
	outStr := out.String()
	if strings.Contains(outStr, "reach this") {
		t.Errorf("no callers must render no blast line, got %s", outStr)
	}
}

// TestPreEditHookBlastLine_DaemonErrorDegradesSilently proves a
// blast_radius call that fails outright never blocks or errors the rest
// of the guard payload.
func TestPreEditHookBlastLine_DaemonErrorDegradesSilently(t *testing.T) {
	orig := guardStampDir
	guardStampDir = filepath.Join(t.TempDir(), "fathom-guard-stamps")
	t.Cleanup(func() { guardStampDir = orig })

	dir := t.TempDir()
	writeGuardCue(t, dir, `guards: [{scopes: ["internal/store/**"], note: "Store schema is spec-governed."}]`)
	file := filepath.Join(dir, "internal", "store", "db.go")

	mux := http.NewServeMux()
	mux.HandleFunc("/rpc", func(w http.ResponseWriter, r *http.Request) {
		switch rpcRequestName(r) {
		case "obligations":
			json.NewEncoder(w).Encode(rpcTextResult(map[string]any{
				"service": strings.ToLower(filepath.Base(dir)),
				"file":    "internal/store/db.go",
				"rules": []any{
					map[string]any{"rule": "Migrations are additive-only.", "src": "docs/spec.md#L1-L2", "tier": "verified"},
				},
			}))
		case "blast_radius":
			http.Error(w, "boom", http.StatusInternalServerError)
		default:
			json.NewEncoder(w).Encode(rpcTextResult(map[string]any{"rules": []any{}}))
		}
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	hookInput := fmt.Sprintf(`{"tool_name":"Edit","tool_input":{"file_path":%q},"cwd":%q,"session_id":"sess-blast-3"}`, file, dir)
	hookStdin(t, hookInput)

	var out, errBuf bytes.Buffer
	code := runWithStdin([]string{"edit-hook", "--pre", "--url=" + srv.URL}, nil, &out, &errBuf)
	if code != 0 {
		t.Fatalf("exit = %d, want 0; stderr=%s", code, errBuf.String())
	}
	outStr := out.String()
	if !strings.Contains(outStr, "Migrations are additive-only.") {
		t.Errorf("a failed blast_radius call must not suppress the rest of the guard payload, got %s", outStr)
	}
	if strings.Contains(outStr, "reach this") {
		t.Errorf("a failed blast_radius call must render no blast line, got %s", outStr)
	}
}
