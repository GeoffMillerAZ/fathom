package main

// prompthook_silence_test.go — GL-26 (docs/plans/governance-learning.md,
// O822): GL-24 gave every silent edit-hook answer a reason from one closed
// vocabulary (silenceReason); prompt-hook's two emitted=false beats carried
// none, and its other silences wrote no answered beat at all. Every prompt
// the hook saw is now answered exactly once, and a silent answer names its
// reason from the same vocabulary.

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPromptHookSilentAnswersCarryReason(t *testing.T) {
	t.Setenv("FATHOM_CHANNELS", "")
	t.Setenv("CLAUDE_PROJECT_DIR", "")
	dir := touchTree(t, t.TempDir(), "internal/store/backend.go")
	rules := fakePromptDaemon(t, map[string]any{
		"internal/store/backend.go": map[string]any{
			"service": strings.ToLower(filepath.Base(dir)), "file": "internal/store/backend.go",
			"rules": []any{map[string]any{"rule": "internal/store stays pure Go.", "src": "ADR.md#L814-L814", "tier": "doc-claimed", "layer": "adr-rules"}},
		},
	}, nil)
	defer rules.Close()
	empty := fakePromptDaemon(t, nil, nil) // answers, with no rules for any file
	defer empty.Close()
	down := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Error(w, "down", http.StatusServiceUnavailable) }))
	defer down.Close()
	capOff := touchTree(t, t.TempDir(), "internal/store/backend.go")
	if err := os.MkdirAll(filepath.Join(capOff, ".fathom"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(capOff, ".fathom", "capabilities.cue"), []byte("capabilities: {\n\tobligations: on: false\n}\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	for _, c := range []struct {
		name, cwd, url, prompt string
		armOff                 bool
		emitted                bool
		reason                 silenceReason
	}{
		{"served", dir, rules.URL, "please refactor internal/store/backend.go", false, true, ""},
		{"the OFF arm", dir, rules.URL, "please refactor internal/store/backend.go", true, false, silenceArmOff},
		{"a prompt naming nothing", dir, rules.URL, "please make the tests pass and fix the bug", false, false, silenceNoCandidate},
		{"a symbol the index does not hold", dir, rules.URL, "add a case to registerTool for the new verb", false, false, silenceNoCandidate},
		{"resolved, nothing governs it", dir, empty.URL, "please refactor internal/store/backend.go", false, false, silenceNoRules},
		{"every obligations call failed", dir, down.URL, "please refactor internal/store/backend.go", false, false, silenceDaemonError},
		{"obligations switched off", capOff, rules.URL, "please refactor internal/store/backend.go", false, false, silenceCapabilityOff},
	} {
		t.Run(c.name, func(t *testing.T) {
			log := filepath.Join(t.TempDir(), "hooks.jsonl")
			t.Setenv("FATHOM_HOOKS_LOG", log)
			if c.armOff {
				_, _ = armOffEnv(t, 3)
			}
			promptHookStdin(t, c.cwd, c.prompt)
			var out, errb bytes.Buffer
			if code := cmdPromptHook([]string{"--url=" + c.url}, &out, &errb); code != 0 {
				t.Fatalf("prompt-hook exit %d: %s", code, errb.String())
			}
			if (out.Len() > 0) != c.emitted {
				t.Fatalf("stdout %q, want emitted=%v", out.String(), c.emitted)
			}
			beats, err := readRetroBeats(log)
			if err != nil {
				t.Fatal(err)
			}
			seen, answered := 0, []map[string]string{}
			for _, b := range beats {
				if b.Channel != "prompt-hook" {
					continue
				}
				switch b.Kind {
				case "seen":
					seen++
				case "answered":
					answered = append(answered, b.Meta)
				}
			}
			if seen != 1 || len(answered) != 1 {
				t.Fatalf("%d seen and %d answered beat(s), want one of each: %v", seen, len(answered), answered)
			}
			a := answered[0]
			if a["emitted"] != boolStr(c.emitted) {
				t.Fatalf("answered beat %v, want emitted=%v", a, c.emitted)
			}
			if c.emitted {
				if a["reason"] != "" {
					t.Errorf("an answer that spoke carries a reason: %v", a)
				}
				return
			}
			if !knownSilenceReason(a["reason"]) || silenceReason(a["reason"]) != c.reason {
				t.Errorf("silent answer %v: reason %q, want %q from the closed vocabulary", a, a["reason"], c.reason)
			}
		})
	}
}
