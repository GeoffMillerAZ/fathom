package main

// edithook_findings_test.go — FL-20 (docs/plans/findings-loop.md §D,
// docs/vision/30-findings-loop.md §3.4): unit tests for renderFindingsBlock
// and its helpers, plus end-to-end pre/post-edit hook tests proving the
// block actually reaches the rendered payload.

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

	"github.com/geoffmilleraz/fathom/internal/harness"
	"github.com/geoffmilleraz/fathom/internal/semantic"
)

// twoFindingFixture is the two-finding fixture this wave's report cites
// verbatim: one fresh, anchor-current, class="security" finding and one
// drifted-anchor, unclassified finding — sorted (anchored+current first,
// then ObservedAt desc) so the security finding renders first.
func twoFindingFixture() []semantic.Obligation {
	return []semantic.Obligation{
		{
			Rule: "Two fields share the json tag \"id\".", Src: "internal/api/user.go#L12-L15",
			Tier: "sme-stated", Kind: "finding", Layer: "findings",
			Producer: "reviewkit.system-review", RuleID: "dup-json-tag",
			Remedy: "Rename one of the fields.", ObservedAt: "2026-09-05", SourceRevision: "def5678",
			AnchorCurrent: boolPtr(false),
		},
		{
			Rule: "Do not log API keys in plaintext.", Src: "internal/auth/token.go#L40-L44",
			Tier: "sme-stated", Kind: "finding", Layer: "findings", Class: "security",
			Producer: "reviewkit.security-review", RuleID: "secrets-in-logs",
			Remedy: "Redact the key before logging.", ObservedAt: "2026-09-10", SourceRevision: "abc1234",
			AnchorCurrent: boolPtr(true),
		},
	}
}

func TestRenderFindingsBlock_TwoFindingFixture(t *testing.T) {
	block, meta := renderFindingsBlock(twoFindingFixture(), nil, "pre")
	if meta.Total != 2 || meta.Shown != 2 || meta.Omitted != 0 {
		t.Fatalf("meta = %+v, want Total=2 Shown=2 Omitted=0", meta)
	}
	want := "📋 fathom findings — open findings on this file (2, showing 2):" +
		"\n- [finding · security · reviewkit.security-review · secrets-in-logs] Do not log API keys in plaintext. — remedy: Redact the key before logging. — internal/auth/token.go#L40-L44 — observed 2026-09-10 at abc1234" +
		"\n- [finding · unclassified · reviewkit.system-review · dup-json-tag] Two fields share the json tag \"id\". — remedy: Rename one of the fields. — internal/api/user.go#L12-L15 — observed 2026-09-05 at def5678 (code moved since observed)"
	if block != want {
		t.Errorf("block mismatch:\n got  %q\nwant %q", block, want)
	}
}

func TestRenderFindingsBlock_CapsAtThreeAndNamesOmitted(t *testing.T) {
	var rows []semantic.Obligation
	for i := 0; i < 5; i++ {
		rows = append(rows, semantic.Obligation{
			Rule: fmt.Sprintf("Finding number %d.", i), Src: fmt.Sprintf("pkg/file%d.go#L1-L2", i),
			Kind: "finding", RuleID: fmt.Sprintf("rule-%d", i), Producer: "p", Class: "correctness",
			ObservedAt: fmt.Sprintf("2026-09-%02d", i+1),
		})
	}
	block, meta := renderFindingsBlock(rows, nil, "pre")
	if meta.Total != 5 || meta.Shown != 3 || meta.Omitted != 2 {
		t.Fatalf("meta = %+v, want Total=5 Shown=3 Omitted=2", meta)
	}
	if !strings.Contains(block, "(5, showing 3)") {
		t.Errorf("header must name total and shown, got %q", block)
	}
	if !strings.Contains(block, "⚠️ omitted: 2") {
		t.Errorf("omitted count must always render when N>k, got %q", block)
	}
}

func TestRenderFindingsBlock_UnanchoredSuffixAndOrdering(t *testing.T) {
	rows := []semantic.Obligation{
		{Rule: "process finding", Kind: "finding", RuleID: "b", Unanchored: true, ObservedAt: "2026-09-01"},
		{Rule: "anchored finding", Src: "x.go#L1-L2", Kind: "finding", RuleID: "a", ObservedAt: "2026-09-02"},
	}
	block, meta := renderFindingsBlock(rows, nil, "pre")
	if meta.Shown != 2 {
		t.Fatalf("meta = %+v, want Shown=2", meta)
	}
	// Anchored before unanchored, regardless of ObservedAt.
	if strings.Index(block, "anchored finding") > strings.Index(block, "process finding") {
		t.Errorf("anchored finding must sort before unanchored, got %q", block)
	}
	if !strings.Contains(block, "(unanchored process finding)") {
		t.Errorf("unanchored row must carry its suffix, got %q", block)
	}
}

func TestRenderFindingsBlock_CodeDriftedSuffix(t *testing.T) {
	rows := []semantic.Obligation{
		{Rule: "r", Src: "x.go#L1-L2", Kind: "finding", RuleID: "a", CodeDrifted: true},
	}
	block, _ := renderFindingsBlock(rows, nil, "pre")
	if !strings.Contains(block, "(scoped code changed since compile — may already be fixed)") {
		t.Errorf("CodeDrifted must render its own suffix, got %q", block)
	}
}

func TestRenderFindingsBlock_NoFindings(t *testing.T) {
	block, meta := renderFindingsBlock(nil, nil, "pre")
	if block != "" || meta != (findingsMeta{}) {
		t.Errorf("no findings must render nothing, got block=%q meta=%+v", block, meta)
	}
	// Non-finding rows are ignored entirely, even if passed in.
	block, meta = renderFindingsBlock([]semantic.Obligation{{Rule: "x", Kind: ""}}, nil, "pre")
	if block != "" || meta != (findingsMeta{}) {
		t.Errorf("governs-kind rows must never render as findings, got block=%q meta=%+v", block, meta)
	}
}

func TestRenderFindingsBlock_PostRequiresEditedRange(t *testing.T) {
	rows := []semantic.Obligation{{Rule: "r", Src: "x.go#L1-L2", Kind: "finding", RuleID: "a"}}
	block, meta := renderFindingsBlock(rows, nil, "post")
	if block != "" || meta.Reason != "no edited range" {
		t.Errorf("post with nil editedRange must skip with a reason, got block=%q meta=%+v", block, meta)
	}
}

func TestRenderFindingsBlock_PostOnlyOnOverlap(t *testing.T) {
	rows := []semantic.Obligation{
		{Rule: "in range", Src: "x.go#L10-L12", Kind: "finding", RuleID: "a"},
		{Rule: "out of range", Src: "x.go#L40-L42", Kind: "finding", RuleID: "b"},
	}
	block, meta := renderFindingsBlock(rows, &lineRange{Start: 11, End: 11}, "post")
	if meta.Total != 1 || meta.Shown != 1 {
		t.Fatalf("meta = %+v, want exactly the overlapping row", meta)
	}
	if !strings.Contains(block, "in range") || strings.Contains(block, "out of range") {
		t.Errorf("only the overlapping finding must render, got %q", block)
	}
}

func TestSplitFindingRows(t *testing.T) {
	rules := []semantic.Obligation{
		{Rule: "gov1", Kind: ""},
		{Rule: "find1", Kind: "finding"},
		{Rule: "gov2", Kind: ""},
	}
	gov, find := splitFindingRows(rules)
	if len(gov) != 2 || gov[0].Rule != "gov1" || gov[1].Rule != "gov2" {
		t.Errorf("govRules = %+v, want [gov1 gov2]", gov)
	}
	if len(find) != 1 || find[0].Rule != "find1" {
		t.Errorf("findingRows = %+v, want [find1]", find)
	}
}

func TestEditedRangeFromFile(t *testing.T) {
	file := "line1\nline2\nline3 needle\nline4\n"
	r := editedRangeFromFile(file, "line3 needle")
	if r == nil || r.Start != 3 || r.End != 3 {
		t.Errorf("editedRangeFromFile = %+v, want {3 3}", r)
	}
	if editedRangeFromFile(file, "") != nil {
		t.Errorf("empty newContent must yield nil")
	}
	if editedRangeFromFile(file, "not present") != nil {
		t.Errorf("unfound newContent must yield nil")
	}
	multi := "a\nb\nc\n"
	r2 := editedRangeFromFile(multi, "b\nc")
	if r2 == nil || r2.Start != 2 || r2.End != 3 {
		t.Errorf("multi-line editedRangeFromFile = %+v, want {2 3}", r2)
	}
}

// ─── end-to-end: pre-edit hook ──────────────────────────────────────────────

// TestEditHookPre_FindingsBlock_RendersAfterRulesAndVerdict proves the
// --pre path renders the findings block after the governing rules, fed
// through the SAME obligations call (layer=all, split by Kind).
func TestEditHookPre_FindingsBlock_RendersAfterRulesAndVerdict(t *testing.T) {
	orig := guardStampDir
	guardStampDir = filepath.Join(t.TempDir(), "fathom-guard-stamps")
	t.Cleanup(func() { guardStampDir = orig })

	dir := t.TempDir()
	writeGuardCue(t, dir, `guards: [{scopes: ["internal/auth/**"], note: "auth is spec-governed."}]`)
	file := filepath.Join(dir, "internal", "auth", "token.go")

	var gotLayer string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// SR-60: renderPreEdit now also calls blast_radius on the same
		// URL, which carries no "layer" argument — only the obligations
		// call's own layer argument is ever captured here.
		name := rpcRequestName(r)
		var req struct {
			Params struct {
				Arguments map[string]any `json:"arguments"`
			} `json:"params"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		if name == "obligations" {
			gotLayer, _ = req.Params.Arguments["layer"].(string)
		}
		cannedPayload := map[string]any{
			"service": strings.ToLower(filepath.Base(dir)),
			"file":    "internal/auth/token.go",
			"rules": []any{
				map[string]any{"rule": "Secrets must never be logged.", "src": "docs/spec.md#L1-L2", "tier": "verified"},
				map[string]any{
					"rule": "Do not log API keys in plaintext.", "src": "internal/auth/token.go#L40-L44",
					"tier": "sme-stated", "kind": "finding", "class": "security",
					"producer": "reviewkit.security-review", "rule_id": "secrets-in-logs",
					"remedy": "Redact the key before logging.", "observed_at": "2026-09-10", "source_revision": "abc1234",
					"anchor_current": true,
				},
			},
		}
		json.NewEncoder(w).Encode(map[string]any{
			"jsonrpc": "2.0", "id": 1,
			"result": map[string]any{"content": []any{
				map[string]any{"type": "text", "text": mustJSON(cannedPayload)},
			}},
		})
	}))
	defer srv.Close()

	hookInput := fmt.Sprintf(`{"tool_name":"Edit","tool_input":{"file_path":%q},"cwd":%q,"session_id":"sess-1"}`, file, dir)
	hookStdin(t, hookInput)

	var out, errBuf bytes.Buffer
	code := runWithStdin([]string{"edit-hook", "--pre", "--url=" + srv.URL}, nil, &out, &errBuf)
	if code != 0 {
		t.Fatalf("exit = %d, stderr = %s", code, errBuf.String())
	}
	if gotLayer != "all" {
		t.Errorf("obligations call must request layer=all, got %q", gotLayer)
	}
	outStr := out.String()
	if !strings.Contains(outStr, "Secrets must never be logged.") {
		t.Errorf("governing rule must still render, got %s", outStr)
	}
	if !strings.Contains(outStr, "open findings on this file (1, showing 1)") {
		t.Errorf("findings block must render, got %s", outStr)
	}
	if !strings.Contains(outStr, "[finding · security · reviewkit.security-review · secrets-in-logs]") {
		t.Errorf("findings block must carry class/producer/rule_id, got %s", outStr)
	}
	// The findings block must sit AFTER the governing rule in the payload.
	gov := strings.Index(outStr, "Secrets must never be logged.")
	find := strings.Index(outStr, "open findings on this file")
	if gov < 0 || find < 0 || find < gov {
		t.Errorf("findings block must render after the governing rules, got %s", outStr)
	}
}

// ─── end-to-end: post-edit hook ─────────────────────────────────────────────

// TestEditHookPost_FindingsBlock_RendersOnlyOnOverlap proves the post-edit
// path renders the findings block only when the edit's new_string overlaps
// the finding's cited lines, and stays silent (no block) for an edit
// elsewhere in the same file.
func TestEditHookPost_FindingsBlock_RendersOnlyOnOverlap(t *testing.T) {
	dir := t.TempDir()
	fileRel := filepath.Join("internal", "auth", "token.go")
	fileAbs := filepath.Join(dir, fileRel)
	if err := os.MkdirAll(filepath.Dir(fileAbs), 0o755); err != nil {
		t.Fatal(err)
	}
	// Line 3 is where the "finding" edit lands; the finding cites line 3.
	content := "package auth\n\nfunc login() { logKey(apiKey) }\n"
	if err := os.WriteFile(fileAbs, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	cannedPayload := map[string]any{
		"service": strings.ToLower(filepath.Base(dir)),
		"file":    filepath.ToSlash(fileRel),
		"rules": []any{
			map[string]any{
				"rule": "Do not log API keys in plaintext.", "src": filepath.ToSlash(fileRel) + "#L3-L3",
				"tier": "sme-stated", "kind": "finding", "class": "security",
				"producer": "reviewkit.security-review", "rule_id": "secrets-in-logs",
				"remedy": "Redact the key before logging.", "observed_at": "2026-09-10", "source_revision": "abc1234",
			},
		},
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"jsonrpc": "2.0", "id": 1,
			"result": map[string]any{"content": []any{
				map[string]any{"type": "text", "text": mustJSON(cannedPayload)},
			}},
		})
	}))
	defer srv.Close()

	// The edit's new_string is the exact text now on line 3 — an overlap.
	hookInput := fmt.Sprintf(`{"tool_name":"Edit","tool_input":{"file_path":%q,"old_string":"x","new_string":"func login() { logKey(apiKey) }"},"cwd":%q}`, fileAbs, dir)
	hookStdin(t, hookInput)
	var out, errBuf bytes.Buffer
	code := runWithStdin([]string{"edit-hook", "--url=" + srv.URL}, nil, &out, &errBuf)
	if code != 0 {
		t.Fatalf("exit = %d, stderr = %s", code, errBuf.String())
	}
	if !strings.Contains(out.String(), "open findings on this file") {
		t.Errorf("overlapping post-edit must render the findings block, got %s", out.String())
	}

	// A second edit, elsewhere in the same file (line 1), must NOT show it.
	hookInput2 := fmt.Sprintf(`{"tool_name":"Edit","tool_input":{"file_path":%q,"old_string":"x","new_string":"package auth"},"cwd":%q}`, fileAbs, dir)
	hookStdin(t, hookInput2)
	out.Reset()
	errBuf.Reset()
	code = runWithStdin([]string{"edit-hook", "--url=" + srv.URL}, nil, &out, &errBuf)
	if code != 0 {
		t.Fatalf("exit = %d, stderr = %s", code, errBuf.String())
	}
	if strings.Contains(out.String(), "open findings on this file") {
		t.Errorf("a non-overlapping post-edit must not render the findings block, got %s", out.String())
	}
}

// ─── Codex parity (harness codec) ──────────────────────────────────────────

// TestFindingsBlock_BothCodecsCarryTheBlock is the "Codex parity" pin the
// plan asks for: the SAME harness-free findings block text, run through
// both the Claude Code and Codex codecs' EncodeOutput, must reach the
// harness's own additionalContext field unchanged.
func TestFindingsBlock_BothCodecsCarryTheBlock(t *testing.T) {
	block, meta := renderFindingsBlock(twoFindingFixture(), nil, "pre")
	if meta.Shown == 0 {
		t.Fatal("fixture must render at least one finding")
	}
	for _, codec := range []harness.Codec{harness.ClaudeCodec(), harness.CodexCodec()} {
		out, err := codec.EncodeOutput(harness.EventPreTool, harness.HookOutput{Context: block})
		if err != nil {
			t.Fatalf("%s: EncodeOutput: %v", codec.ID(), err)
		}
		if !strings.Contains(string(out), "open findings on this file") {
			t.Errorf("%s: encoded output must carry the findings block, got %s", codec.ID(), out)
		}
		if !strings.Contains(string(out), "secrets-in-logs") {
			t.Errorf("%s: encoded output must carry the finding's rule id, got %s", codec.ID(), out)
		}
	}
}
