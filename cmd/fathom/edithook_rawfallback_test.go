package main

// edithook_rawfallback_test.go — wave O1 (ADR-037), test (e): guards-pilot-6
// found the PostToolUse hook on a brand-new file naming ONLY "index is
// behind your working tree for this file" and nothing else — no rule, no
// guidance, exactly the silence ADR-037 exists to end. This pins the fix at
// the edit-hook's own rendering boundary: cmd/fathom/edithook.go needed NO
// code change for this (writeRuleBullet already renders whatever `tier` and
// `rule` the daemon serves) — the fix lives upstream, in the obligations
// tool now serving a raw row instead of an empty rules array alongside the
// stale flag. This test proves the two halves compose: a daemon response
// shaped exactly like the fixed tool now produces reaches the agent as a
// real, named rule, not bare silence.

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

func TestEditHookBrandNewFile_RawFallbackRuleNamedNotJustStale(t *testing.T) {
	cannedPayload := map[string]any{
		"service": "myrepo",
		"file":    "docs/use-cases/new-thing.md",
		"rules": []any{
			map[string]any{
				"rule":  "Every use-case doc must carry a Maintained line.",
				"src":   "docs/CLAUDE.md#L1-L3",
				"tier":  "raw",
				"layer": "obligations",
				"why":   "compiled layer incomplete: no compile_runs row; raw nearest guide served",
			},
		},
		"index_stale": true,
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
	absFile := filepath.Join(dir, "docs", "use-cases", "new-thing.md")
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
	outStr := out.String()
	if outStr == "" {
		t.Fatal("edit-hook: expected output for a brand-new file with a raw fallback rule, got nothing")
	}

	var payload map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(outStr)), &payload); err != nil {
		t.Fatalf("edit-hook output not valid JSON: %v (output=%q)", err, outStr)
	}
	hook, ok := payload["hookSpecificOutput"].(map[string]any)
	if !ok {
		t.Fatalf("missing hookSpecificOutput: %v", payload)
	}
	ctx, _ := hook["additionalContext"].(string)

	// The old defect: additionalContext carried ONLY the stale note.
	if strings.TrimSpace(ctx) == "" {
		t.Fatal("additionalContext is empty")
	}
	if !strings.Contains(ctx, "index is behind your working tree") {
		t.Errorf("expected the stale note to still be present, got %q", ctx)
	}
	// The fix: a NAMED rule, tagged raw, actually reaches the agent —
	// never silence, never the stale note alone.
	if !strings.Contains(ctx, "Every use-case doc must carry a Maintained line") {
		t.Errorf("additionalContext should name the raw fallback rule, got %q", ctx)
	}
	if !strings.Contains(ctx, "raw") {
		t.Errorf("additionalContext should show the raw tier, got %q", ctx)
	}
	if !strings.Contains(ctx, "docs/CLAUDE.md") {
		t.Errorf("additionalContext should cite the nearest guide, got %q", ctx)
	}
}
