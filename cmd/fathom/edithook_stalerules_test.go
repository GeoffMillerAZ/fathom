package main

// edithook_stalerules_test.go — J2 (app-kit O6/O16/O24/O26/O29/O30; the
// dogfood journal's P0 "withhold stale obligations from authoritative
// injection"). Before this fix, a stale/contradicted rule rendered
// interleaved with binding rules in the SAME bullet list, distinguished
// only by a trailing ", stale" label. This pins the fix at the
// PostToolUse hook's own rendering boundary: a stale rule moves OUT of the
// governing list into one trailing block with exactly one remedy line, and
// a rule set with nothing stale renders byte-identically to before this
// fix existed.

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

func TestEditHookStaleRule_MovedToTrailingBlockWithRemedy(t *testing.T) {
	cannedPayload := map[string]any{
		"service": "myrepo",
		"file":    "internal/query/run.go",
		"rules": []any{
			map[string]any{
				"rule":  "Use tool templ and tool cue, never a bare shell invocation.",
				"src":   "CLAUDE.md#L10-L12",
				"tier":  "obligation",
				"layer": "obligations",
			},
			map[string]any{
				"rule":        "For material UI work, choose a complete visual reference separately.",
				"src":         "CLAUDE.md#L46-L47",
				"tier":        "obligation",
				"layer":       "obligations",
				"status":      "stale",
				"anchor_text": "For material UI work, choose a complete visual reference separately from the renderer.",
			},
		},
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]any{
			"jsonrpc": "2.0", "id": 1,
			"result": map[string]any{"content": []any{
				map[string]any{"type": "text", "text": mustJSON(cannedPayload)},
			}},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	dir := t.TempDir()
	absFile := filepath.Join(dir, "internal", "query", "run.go")
	hookInput := fmt.Sprintf(`{"tool_input":{"file_path":%q},"cwd":%q}`, absFile, dir)

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
		t.Fatalf("edit-hook exit = %d, want 0", code)
	}
	var payload map[string]any
	if err := json.Unmarshal(bytes.TrimSpace(out.Bytes()), &payload); err != nil {
		t.Fatalf("edit-hook output not valid JSON: %v (output=%q)", err, out.String())
	}
	hook, ok := payload["hookSpecificOutput"].(map[string]any)
	if !ok {
		t.Fatalf("missing hookSpecificOutput: %v", payload)
	}
	ctx, _ := hook["additionalContext"].(string)

	// The fresh rule renders as an ordinary governing bullet, unlabeled.
	freshIdx := strings.Index(ctx, "Use tool templ and tool cue")
	if freshIdx < 0 {
		t.Fatalf("fresh rule missing from context: %q", ctx)
	}
	// The stale rule must NOT appear inline as an ordinary bullet — it
	// only appears inside the trailing block, after the block header.
	blockIdx := strings.Index(ctx, "stale rules — not binding until recompiled (1):")
	if blockIdx < 0 {
		t.Fatalf("trailing stale block missing: %q", ctx)
	}
	if blockIdx < freshIdx {
		t.Fatalf("stale block must come AFTER the governing rules, got %q", ctx)
	}
	staleTextIdx := strings.Index(ctx, "For material UI work, choose a complete visual reference separately.")
	if staleTextIdx < blockIdx {
		t.Fatalf("stale rule text must render inside the trailing block, got %q", ctx)
	}
	if strings.Contains(ctx[:blockIdx], "stale") {
		t.Errorf("the governing section must not mention a stale rule before the trailing block: %q", ctx[:blockIdx])
	}
	if !strings.Contains(ctx, "CLAUDE.md#L46-L47") {
		t.Errorf("stale block must cite the stale rule's source: %q", ctx)
	}
	// The service name in the remedy comes from LOCAL candidate resolution
	// (the directory basename), not the daemon payload's own "service"
	// field — resolveServiceAndRel/serviceCandidates' existing contract, so
	// this only checks the remedy's fixed shape and layer, not the exact
	// service string.
	remedyIdx := strings.Index(ctx, "remedy: fathom compile --layer obligations --service ")
	if remedyIdx < 0 {
		t.Fatalf("expected exactly one remedy line, got %q", ctx)
	}
	if strings.Count(ctx, "remedy:") != 1 {
		t.Errorf("expected exactly one remedy line, got %q", ctx)
	}
}

// TestEditHookNoStaleRules_ByteIdenticalRendering pins that a rule set with
// nothing stale renders exactly as it did before J2 — no trailing block, no
// remedy line, no behavior change for the overwhelmingly common case.
func TestEditHookNoStaleRules_ByteIdenticalRendering(t *testing.T) {
	cannedPayload := map[string]any{
		"service": "myrepo",
		"file":    "internal/query/run.go",
		"rules": []any{
			map[string]any{
				"rule":  "Use tool templ and tool cue, never a bare shell invocation.",
				"src":   "CLAUDE.md#L10-L12",
				"tier":  "obligation",
				"layer": "obligations",
			},
		},
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]any{
			"jsonrpc": "2.0", "id": 1,
			"result": map[string]any{"content": []any{
				map[string]any{"type": "text", "text": mustJSON(cannedPayload)},
			}},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	dir := t.TempDir()
	absFile := filepath.Join(dir, "internal", "query", "run.go")
	hookInput := fmt.Sprintf(`{"tool_input":{"file_path":%q},"cwd":%q}`, absFile, dir)

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
		t.Fatalf("edit-hook exit = %d, want 0", code)
	}
	var payload map[string]any
	if err := json.Unmarshal(bytes.TrimSpace(out.Bytes()), &payload); err != nil {
		t.Fatalf("edit-hook output not valid JSON: %v (output=%q)", err, out.String())
	}
	hook := payload["hookSpecificOutput"].(map[string]any)
	ctx, _ := hook["additionalContext"].(string)

	want := "⚖️ fathom obligations — internal/query/run.go:" +
		"\n• Use tool templ and tool cue, never a bare shell invocation. [CLAUDE.md#L10-L12, obligation]"
	if ctx != want {
		t.Errorf("additionalContext =\n%q\nwant\n%q", ctx, want)
	}
	if strings.Contains(ctx, "stale rules") || strings.Contains(ctx, "remedy:") {
		t.Errorf("no stale rule present, must not render a stale block: %q", ctx)
	}
}
