package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/store"
)

// ─── compile: --service required ─────────────────────────────────────────────

func TestCompileRequiresService(t *testing.T) {
	var out, errBuf bytes.Buffer
	code := run([]string{"compile"}, &out, &errBuf)
	if code == 0 {
		t.Fatal("compile without --service should fail")
	}
}

// ─── compile: unknown layer rejected (by the backend, not a pre-check) ───────

// TestCompileUnknownLayer: an unknown layer still fails the compile — but
// via the BACKEND's own resolution error (client.CompileLayer), not a
// flag-parse-time pre-check. The pre-check is advisory-only (critic finding
// 3 on the LC slice): it can only see CWD, and a hard rejection there
// false-negatives project packs of OTHER repos' services. --store makes the
// test hermetic (the pass-through would otherwise hit the daemon URL).
func TestCompileUnknownLayer(t *testing.T) {
	storePath := filepath.Join(t.TempDir(), "s.sqlite")
	var out, errBuf bytes.Buffer
	code := run([]string{"compile", "--service=svc", "--layer=hypotheses", "--store=" + storePath}, &out, &errBuf)
	if code == 0 {
		t.Fatal("compile with unknown layer should fail")
	}
	if !strings.Contains(errBuf.String(), "obligations") {
		t.Errorf("error should mention obligations, got %q", errBuf.String())
	}
	if !strings.Contains(errBuf.String(), "warning:") {
		t.Errorf("the advisory CWD pre-check should have warned before the backend rejected, got %q", errBuf.String())
	}
}

// TestCompile_AcceptsCrossRepoProjectLayer is critic finding 3's regression
// test: `fathom compile --service B --layer <B's project pack>` run from an
// UNRELATED working directory (repo A) must succeed — the layer is unknown
// at CWD (warn only) but resolves via the service's own ingested root
// (ServiceMeta.Path) in the backend.
func TestCompile_AcceptsCrossRepoProjectLayer(t *testing.T) {
	packRoot := t.TempDir() // "repo B": owns the project pack
	packDir := filepath.Join(packRoot, ".fathom", "layers", "xrepo-pack")
	if err := os.MkdirAll(packDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	layerCUE := `name:    "xrepo-pack"
version: "v1"
kind:    "xrepo-kind"
corpus: include: ["docs/**"]
prompt_version: "xrepo-pack/fixture"
prompt_file:    "xrepo-pack/prompt.md"
tier:           "doc-claimed"
budgets: {
	max_tokens_per_run: 1000
	max_usd_per_run:    0.1
}
`
	if err := os.WriteFile(filepath.Join(packDir, "layer.cue"), []byte(layerCUE), 0o644); err != nil {
		t.Fatalf("write layer.cue: %v", err)
	}
	if err := os.WriteFile(filepath.Join(packDir, "prompt.md"), []byte("Test prompt."), 0o644); err != nil {
		t.Fatalf("write prompt.md: %v", err)
	}

	storePath := filepath.Join(t.TempDir(), "s.sqlite")
	st, err := store.Open(storePath)
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	if err := st.SetServiceMeta(context.Background(), store.ServiceMeta{ServiceID: "svc-b", Path: packRoot}); err != nil {
		t.Fatalf("SetServiceMeta: %v", err)
	}
	st.Close()

	t.Chdir(t.TempDir()) // "repo A": an unrelated CWD with no .fathom/layers/

	var out, errBuf bytes.Buffer
	code := run([]string{"compile", "--service=svc-b", "--layer=xrepo-pack", "--store=" + storePath}, &out, &errBuf)
	if code != 0 {
		t.Fatalf("cross-repo project-layer compile should succeed (dry-run), got exit %d, stderr: %s", code, errBuf.String())
	}
	if !strings.Contains(errBuf.String(), "warning:") {
		t.Errorf("expected the advisory unknown-at-CWD warning on stderr, got %q", errBuf.String())
	}
	if !strings.Contains(out.String(), "xrepo-pack") {
		t.Errorf("output should name the compiled layer, got %q", out.String())
	}
}

// TestCompile_AcceptsProjectLocalLayerAtCWD is the LC-3 fix (docs/plans/
// layer-catalog.md): cmdCompile's fail-fast flag-parse-time validation used
// to call the EMBEDDED-only semantic.LoadLayer, which would reject a
// perfectly valid project-local layer (installed under CWD's own
// .fathom/layers/) before compileViaStore's real, service-rooted resolution
// ever ran. Root-aware LoadLayerIn (CWD as the heuristic root) fixes it —
// this proves a project-local layer name now clears validation and reaches
// a real (empty, dry-run) compile instead of failing at the "unknown layer"
// stage.
func TestCompile_AcceptsProjectLocalLayerAtCWD(t *testing.T) {
	root := t.TempDir()
	packDir := filepath.Join(root, ".fathom", "layers", "custom-pack")
	if err := os.MkdirAll(packDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	layerCUE := `name:    "custom-pack"
version: "v1"
kind:    "custom-kind"
corpus: include: ["docs/**"]
prompt_version: "custom-pack/fixture"
prompt_file:    "custom-pack/prompt.md"
tier:           "doc-claimed"
budgets: {
	max_tokens_per_run: 1000
	max_usd_per_run:    0.1
}
`
	if err := os.WriteFile(filepath.Join(packDir, "layer.cue"), []byte(layerCUE), 0o644); err != nil {
		t.Fatalf("write layer.cue: %v", err)
	}
	if err := os.WriteFile(filepath.Join(packDir, "prompt.md"), []byte("Test prompt."), 0o644); err != nil {
		t.Fatalf("write prompt.md: %v", err)
	}

	t.Chdir(root)
	storePath := filepath.Join(t.TempDir(), "s.sqlite")

	// The REAL compile step (pkg/client.CompileLayer) resolves the layer's
	// root from the SERVICE's own ingested path (service_meta), not CWD —
	// CWD only backs this test's flag-parse-time fail-fast check. Seed
	// service_meta so "svc"'s root is the same directory the project pack
	// lives under, matching what a real `fathom ingest` from root would do.
	st, err := store.Open(storePath)
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	if err := st.SetServiceMeta(context.Background(), store.ServiceMeta{ServiceID: "svc", Path: root}); err != nil {
		t.Fatalf("SetServiceMeta: %v", err)
	}
	st.Close()

	var out, errBuf bytes.Buffer
	code := run([]string{"compile", "--service=svc", "--layer=custom-pack", "--store=" + storePath}, &out, &errBuf)
	if code != 0 {
		t.Fatalf("compile with a project-local layer should succeed (dry-run), got exit %d, stderr: %s", code, errBuf.String())
	}
	if strings.Contains(errBuf.String(), "unknown layer") {
		t.Errorf("project-local layer wrongly rejected as unknown: %s", errBuf.String())
	}
	if !strings.Contains(out.String(), "custom-pack") {
		t.Errorf("output should name the compiled layer, got %q", out.String())
	}
}

// ─── compile: HTTP mode calls compile_layer ───────────────────────────────────

func TestCompileViaHTTP(t *testing.T) {
	// Serve a canned compile_layer MCP response.
	cannedResult := map[string]any{
		"service":           "svc",
		"layer":             "obligations",
		"dry_run":           true,
		"sections_total":    3.0,
		"sections_compiled": 1.0,
		"claims_emitted":    0.0,
		"claims_rejected":   0.0,
		"tokens_in":         280.0,
		"tokens_out":        0.0,
		"cost_usd":          0.0,
		"duration_ms":       5.0,
		"ran_at":            "2026-07-07T12:00:00Z",
		// F5: a budget-stopped run must be visibly distinct from success.
		"budget_stopped": true,
		"refusal_reason": "token total 700 exceeds layer \"obligations\" budget max_tokens_per_run=600",
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]any{
			"jsonrpc": "2.0",
			"id":      1,
			"result": map[string]any{
				"content": []any{
					map[string]any{
						"type": "text",
						"text": mustJSON(cannedResult),
					},
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	var out, errBuf bytes.Buffer
	code := run([]string{"compile", "--service=svc", "--url=" + srv.URL}, &out, &errBuf)
	if code != 0 {
		t.Fatalf("compile via HTTP failed (code=%d): stderr=%q", code, errBuf.String())
	}
	outStr := out.String()
	if !strings.Contains(outStr, "DRY RUN") {
		t.Errorf("output should contain DRY RUN marker, got %q", outStr)
	}
	if !strings.Contains(outStr, "sections_total") {
		t.Errorf("output should contain sections_total, got %q", outStr)
	}
	// F5: a budget stop must be visible in the human output.
	if !strings.Contains(outStr, "budget_stopped:    true") {
		t.Errorf("output should surface budget_stopped, got %q", outStr)
	}
	if !strings.Contains(outStr, "max_tokens_per_run") {
		t.Errorf("output should surface the refusal reason, got %q", outStr)
	}
}

// ─── edit-hook: silent on empty stdin ────────────────────────────────────────

func TestEditHookSilentOnEmptyStdin(t *testing.T) {
	// Provide no stdin (or empty JSON): should exit 0, print nothing.
	var out, errBuf bytes.Buffer
	// Simulate with stdin = empty JSON
	origStdin := os.Stdin
	r, w, _ := os.Pipe()
	os.Stdin = r
	w.WriteString("{}")
	w.Close()
	code := run([]string{"edit-hook"}, &out, &errBuf)
	os.Stdin = origStdin
	r.Close()
	if code != 0 {
		t.Fatalf("edit-hook should always exit 0, got %d", code)
	}
	if out.Len() != 0 {
		t.Errorf("empty/no-file input: expected no output, got %q", out.String())
	}
}

// ─── edit-hook: silent on daemon error ───────────────────────────────────────

func TestEditHookSilentOnDaemonError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "internal error", http.StatusInternalServerError)
	}))
	defer srv.Close()

	// The file must relativize under the project dir (absolute, in-root) so
	// the hook genuinely reaches the daemon and exercises the error path —
	// an out-of-repo path would be refused before any request is made.
	dir := t.TempDir()
	hookInput := fmt.Sprintf(`{"tool_input":{"file_path":%q},"cwd":%q}`,
		filepath.Join(dir, "some", "file.go"), dir)

	var out, errBuf bytes.Buffer
	origStdin := os.Stdin
	r, w, _ := os.Pipe()
	os.Stdin = r
	w.WriteString(hookInput)
	w.Close()
	os.Stdin = r

	code := runWithStdin([]string{"edit-hook", "--url=" + srv.URL}, r, &out, &errBuf)
	_ = code
	os.Stdin = origStdin

	if code != 0 {
		t.Fatalf("edit-hook with daemon error should exit 0, got %d", code)
	}
	if out.Len() != 0 {
		t.Errorf("daemon error: expected no output, got %q", out.String())
	}
}

// ─── edit-hook: clean file (no rules, not stale) → silent ────────────────────

func TestEditHookSilentOnCleanFile(t *testing.T) {
	// Daemon returns empty obligations, index not stale.
	cannedPayload := map[string]any{
		"service": "myrepo",
		"file":    "pkg/foo.go",
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]any{
			"jsonrpc": "2.0",
			"id":      1,
			"result": map[string]any{
				"content": []any{
					map[string]any{"type": "text", "text": mustJSON(cannedPayload)},
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	// Absolute in-root path (what Claude Code sends): the hook must reach the
	// daemon, get an empty payload, find no spec IDs (the file doesn't even
	// exist), and stay silent.
	dir := t.TempDir()
	var out, errBuf bytes.Buffer
	hookInput := fmt.Sprintf(`{"tool_input":{"file_path":%q},"cwd":%q}`,
		filepath.Join(dir, "pkg", "foo.go"), dir)

	// We need to provide stdin with hookInput.
	origStdin := os.Stdin
	pr, pw, _ := os.Pipe()
	os.Stdin = pr
	pw.WriteString(hookInput)
	pw.Close()
	code := runWithStdin([]string{"edit-hook", "--url=" + srv.URL}, pr, &out, &errBuf)
	os.Stdin = origStdin
	pr.Close()

	if code != 0 {
		t.Fatalf("edit-hook should exit 0, got %d", code)
	}
	if out.Len() != 0 {
		t.Errorf("clean file: expected no output, got %q", out.String())
	}
}

// ─── edit-hook: rules present → hookSpecificOutput JSON ──────────────────────

func TestEditHookOutputWithRules(t *testing.T) {
	cannedPayload := map[string]any{
		"service": "myrepo",
		"file":    "internal/core/engine.go",
		"rules": []any{
			map[string]any{
				"rule": "Sort all outputs.",
				"src":  "CLAUDE.md#L4-L4",
				"tier": "doc-claimed",
			},
		},
		"compiled_at_watermark": "abc123",
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]any{
			"jsonrpc": "2.0",
			"id":      1,
			"result": map[string]any{
				"content": []any{
					map[string]any{"type": "text", "text": mustJSON(cannedPayload)},
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	dir := t.TempDir()
	hookInput := fmt.Sprintf(`{"tool_input":{"file_path":%q},"cwd":%q}`,
		filepath.Join(dir, "internal", "core", "engine.go"), dir)

	origStdin := os.Stdin
	pr, pw, _ := os.Pipe()
	os.Stdin = pr
	pw.WriteString(hookInput)
	pw.Close()

	var out, errBuf bytes.Buffer
	code := runWithStdin([]string{"edit-hook", "--url=" + srv.URL}, pr, &out, &errBuf)
	os.Stdin = origStdin
	pr.Close()

	if code != 0 {
		t.Fatalf("edit-hook with rules should exit 0, got %d", code)
	}
	outStr := out.String()
	if outStr == "" {
		t.Fatal("edit-hook: expected output with rules, got nothing")
	}
	// Must be valid JSON with hookSpecificOutput.
	var payload map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(outStr)), &payload); err != nil {
		t.Fatalf("edit-hook output not valid JSON: %v (output=%q)", err, outStr)
	}
	hook, ok := payload["hookSpecificOutput"].(map[string]any)
	if !ok {
		t.Fatalf("missing hookSpecificOutput: %v", payload)
	}
	if hook["hookEventName"] != "PostToolUse" {
		t.Errorf("hookEventName = %v, want PostToolUse", hook["hookEventName"])
	}
	ctx, _ := hook["additionalContext"].(string)
	if !strings.Contains(ctx, "Sort all outputs") {
		t.Errorf("additionalContext should contain rule text, got %q", ctx)
	}
}

// FT-2's last hop: a finding-kind claim served on an edited file must be
// distinguishable from an obligation in the rendered payload, and remediation
// drift (code_drifted) must be visible — otherwise the labeling contract stops
// one hop short of the agent.
func TestEditHookRendersKindAndCodeDrifted(t *testing.T) {
	cannedPayload := map[string]any{
		"service": "myrepo",
		"file":    "internal/core/engine.go",
		"rules": []any{
			map[string]any{
				"rule":         "Rotate the leaked credential in config loading.",
				"src":          "findings/security.md#L11-L12",
				"tier":         "sme-stated",
				"kind":         "finding",
				"code_drifted": true,
			},
			map[string]any{
				"rule": "Sort all outputs.",
				"src":  "CLAUDE.md#L4-L4",
				"tier": "doc-claimed",
			},
		},
		"compiled_at_watermark": "abc123",
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]any{
			"jsonrpc": "2.0",
			"id":      1,
			"result": map[string]any{
				"content": []any{
					map[string]any{"type": "text", "text": mustJSON(cannedPayload)},
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	dir := t.TempDir()
	hookInput := fmt.Sprintf(`{"tool_input":{"file_path":%q},"cwd":%q}`,
		filepath.Join(dir, "internal", "core", "engine.go"), dir)

	origStdin := os.Stdin
	pr, pw, _ := os.Pipe()
	os.Stdin = pr
	pw.WriteString(hookInput)
	pw.Close()

	var out, errBuf bytes.Buffer
	code := runWithStdin([]string{"edit-hook", "--url=" + srv.URL}, pr, &out, &errBuf)
	os.Stdin = origStdin
	pr.Close()

	if code != 0 {
		t.Fatalf("edit-hook should exit 0, got %d", code)
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(out.String())), &payload); err != nil {
		t.Fatalf("edit-hook output not valid JSON: %v (output=%q)", err, out.String())
	}
	hook, _ := payload["hookSpecificOutput"].(map[string]any)
	ctx, _ := hook["additionalContext"].(string)
	// The kind label sits in the bracket after the tier ("[…, sme-stated,
	// finding, …]") — asserting the pair, not the bare word, because the src
	// path "findings/security.md" would satisfy a bare Contains("finding").
	if !strings.Contains(ctx, "sme-stated, finding") {
		t.Errorf("finding-kind rule must be labeled as a finding after its tier, got %q", ctx)
	}
	if !strings.Contains(ctx, "code drifted") {
		t.Errorf("code_drifted rule must carry a drift marker, got %q", ctx)
	}
	// The obligation bullet stays exactly as before — no kind label.
	if !strings.Contains(ctx, "• Sort all outputs. [CLAUDE.md#L4-L4, doc-claimed]") {
		t.Errorf("obligation bullet shape must be unchanged, got %q", ctx)
	}
}

// ─── helpers ─────────────────────────────────────────────────────────────────

func mustJSON(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return string(b)
}
