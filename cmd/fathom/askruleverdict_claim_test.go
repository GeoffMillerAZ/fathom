package main

// askruleverdict_claim_test.go — O1434: serving a rule twice is not
// evidence it was ignored. The ask fires only for a rule that has a claim
// id, never alleges a violation, and a raw document heading (served with
// no claim behind it) is never asked about.

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
	"time"
)

// TestRuleVerdictAskNeedsAClaimAndAllegesNoViolation pins O1434's three
// cases plus the unchanged reply path: a claimless rule served twice asks
// nothing; a raw heading rule served twice through the real edit hook asks
// nothing; a claimed rule served twice gets the neutral line with its claim
// id and no word of a violation; the one-key reply is still recorded.
func TestRuleVerdictAskNeedsAClaimAndAllegesNoViolation(t *testing.T) {
	orig := ruleVerdictMemoDir
	ruleVerdictMemoDir = t.TempDir()
	t.Cleanup(func() { ruleVerdictMemoDir = orig })

	const (
		service = "widgetco"
		file    = "internal/widget/widget.go"
		rule    = "never write to hardcoded workspace paths"
		src     = "internal/CLAUDE.md#L10-L12"
		claimID = "cl-served-twice"
	)
	now := time.Date(2026, 10, 20, 12, 0, 0, 0, time.UTC)

	// 1. No claim id: nothing can corroborate or refute it, so nothing is asked.
	for i := range 2 {
		text, ok := withRuleVerdictAskAt("sess-noclaim", service, file, rule, src, "", "obligations text", true, now.Add(time.Duration(i)*time.Minute), nil)
		if text != "obligations text" || !ok {
			t.Fatalf("claimless rule, turn %d: (%q, %v), want the payload unchanged", i+1, text, ok)
		}
	}
	if _, found := readPendingVerdictAsk("sess-noclaim", now.Add(2*time.Minute)); found {
		t.Error("a claimless rule left a pending verdict ask")
	}

	// 2. A raw heading rule through the real edit hook: silent on the second edit.
	raw := map[string]any{"rule": "Never run the maintainer-only workflow from an adopter checkout.", "src": "AGENTS.md#L76-L84", "tier": "raw", "layer": "obligations", "why": "no fresh compiled claim"}
	if out := twoVerdictHookEdits(t, raw); strings.Contains(out, "served again") || strings.Contains(out, "ignored") || strings.Contains(out, "corroborate") {
		t.Fatalf("a raw heading rule served twice was asked about: %q", out)
	}
	// The same hook path with a claimed rule does reach the ask (the raw case is not vacuous).
	claimed := map[string]any{"rule": rule, "src": src, "tier": "doc-claimed", "layer": "obligations", "claim_id": "cl-hook-1"}
	if out := twoVerdictHookEdits(t, claimed); !strings.Contains(out, "🔁 served again after this edit: "+rule+" ("+src+") [claim cl-hook-1]") {
		t.Fatalf("a claimed rule served twice through the edit hook did not get the neutral ask: %q", out)
	}

	// 3. A claimed rule served twice: the neutral line, the claim id, the two keys, no allegation.
	if text, _ := withRuleVerdictAskAt("sess-claim", service, file, rule, src, claimID, "obligations text", true, now, nil); text != "obligations text" {
		t.Fatalf("first serving asked: %q", text)
	}
	ask, ok := withRuleVerdictAskAt("sess-claim", service, file, rule, src, claimID, "obligations text", true, now.Add(time.Minute), nil)
	want := "🔁 served again after this edit: " + rule + " (" + src + ") [claim " + claimID + "]\n"
	if !ok || !strings.HasPrefix(ask, want) {
		t.Fatalf("claimed rule served twice: (%q, %v), want it to open with %q", ask, ok, want)
	}
	for _, alleged := range []string{"ignored", "violat"} {
		if strings.Contains(strings.ToLower(ask), alleged) {
			t.Errorf("the ask alleges %q with no evidence of one: %q", alleged, ask)
		}
	}
	if !strings.Contains(ask, "c — corroborate") || !strings.Contains(ask, "r — refute") || !strings.Contains(ask, "--claim "+claimID) {
		t.Errorf("the ask lost its one-key c/r answer or its by-hand command: %q", ask)
	}

	// 4. The reply path is unchanged: an exact `c` is recorded and the next turn writes it.
	if _, ok := recordVerdictReply("sess-claim", "c", now.Add(2*time.Minute)); !ok {
		t.Fatal("the one-key reply was not recorded against the pending ask")
	}
	var got []claimVerdictInput
	rec := func(_ context.Context, in claimVerdictInput) error { got = append(got, in); return nil }
	text, _ := withRuleVerdictAskAt("sess-claim", service, "internal/widget/other.go", "", "", "", "", false, now.Add(3*time.Minute), rec)
	if len(got) != 1 || got[0].ClaimID != claimID || got[0].Verdict != "corroborated" || got[0].Service != service {
		t.Fatalf("recorded verdicts = %+v, want one corroborated on %s", got, claimID)
	}
	if !strings.Contains(text, "recorded the user's one-key verdict: claim "+claimID+" corroborated") {
		t.Errorf("the recorded-verdict line changed: %q", text)
	}
}

// twoVerdictHookEdits runs two PostToolUse Edit calls of one file in one
// session through the real edit hook, against a daemon that serves rule as
// the file's only governing rule, and returns the second call's context.
func twoVerdictHookEdits(t *testing.T, rule map[string]any) string {
	t.Helper()
	isolateHookRun(t)
	hermeticObsEnv(t)
	guardStampsIsolated(t)
	home := t.TempDir()
	t.Setenv("FATHOM_HOME", home)
	t.Setenv("FATHOM_HOOKS_LOG", filepath.Join(home, "hooks.jsonl"))
	t.Setenv("FATHOM_RULE_OUTCOME_LOG", filepath.Join(home, "rule-outcomes.jsonl"))

	dir := t.TempDir()
	rel := filepath.Join("internal", "widget", "widget.go")
	abs := filepath.Join(dir, rel)
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(abs, []byte("package widget\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	payload := mustJSON(map[string]any{"service": strings.ToLower(filepath.Base(dir)), "file": filepath.ToSlash(rel), "rules": []any{rule}})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Params struct {
				Name string `json:"name"`
			} `json:"params"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		inner := `{}`
		switch req.Params.Name {
		case "obligations":
			inner = payload
		case "list_services":
			inner = `{"services":[]}`
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": 1, "result": map[string]any{"content": []any{map[string]any{"type": "text", "text": inner}}}})
	}))
	defer srv.Close()

	session := "sess-hook-" + filepath.Base(dir)
	var last string
	for i := range 2 {
		hookStdin(t, fmt.Sprintf(`{"session_id":%q,"tool_name":"Edit","tool_input":{"file_path":%q,"old_string":"x","new_string":"package widget"},"cwd":%q}`, session, abs, dir))
		var out, errBuf bytes.Buffer
		if code := runWithStdin([]string{"edit-hook", "--url=" + srv.URL}, nil, &out, &errBuf); code != 0 {
			t.Fatalf("edit %d: exit %d, stderr %s", i+1, code, errBuf.String())
		}
		var p struct {
			HookSpecificOutput struct {
				AdditionalContext string `json:"additionalContext"`
			} `json:"hookSpecificOutput"`
		}
		if s := strings.TrimSpace(out.String()); s != "" {
			if err := json.Unmarshal([]byte(s), &p); err != nil {
				t.Fatalf("edit %d: output is not JSON: %v (%q)", i+1, err, s)
			}
		}
		if i == 0 && !strings.Contains(p.HookSpecificOutput.AdditionalContext, rule["rule"].(string)) {
			t.Fatalf("edit 1 did not serve the rule: %q", out.String())
		}
		last = p.HookSpecificOutput.AdditionalContext
	}
	return last
}
