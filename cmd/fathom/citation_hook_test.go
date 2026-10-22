package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestCitationHookGoldens(t *testing.T) {
	for _, state := range []string{"", "stale", "refreshing", "unknown"} {
		t.Run("state="+state, func(t *testing.T) {
			dir := touchTree(t, t.TempDir(), "sample.go")
			writeGuardCue(t, dir, `guards: [{scopes: ["sample.go"], note: "Check the source."}]`)
			rule := map[string]any{"rule": "Keep the contract.", "src": "guide.md#L1-L2", "tier": "doc-claimed", "layer": "obligations"}
			if state != "" {
				rule["citation_status"] = map[string]any{"state": state, "reason": "source check"}
			}
			srv := fakePromptDaemon(t, map[string]any{"sample.go": map[string]any{"service": strings.ToLower(filepath.Base(dir)), "file": "sample.go", "rules": []any{rule}}}, nil)
			defer srv.Close()
			post, _, ok := renderPostEdit(srv.URL, dir, filepath.Join(dir, "sample.go"), "", "")
			if !ok {
				t.Fatal("post did not render")
			}
			pre, ok := renderPreEdit(srv.URL, dir, filepath.Join(dir, "sample.go"), "")
			if !ok {
				t.Fatal("pre did not render")
			}
			prompt, _, ok := renderPromptObligations(srv.URL, dir, "Update sample.go please.", "", time.Now().Add(promptHookDeadlineBudget))
			if !ok {
				t.Fatal("prompt did not render")
			}
			outputs := []struct{ phase, text string }{{"post", post}, {"pre", pre.text}, {"prompt", prompt}}
			for _, output := range outputs {
				phase, got := output.phase, output.text
				suffix := state
				if suffix == "" {
					suffix = "clean"
				}
				path := filepath.Join("testdata", "citation-hooks", phase+"-"+suffix+".golden")
				if os.Getenv("UPDATE_CITATION_GOLDENS") == "1" {
					if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
						t.Fatal(err)
					}
					if err := os.WriteFile(path, []byte(got), 0600); err != nil {
						t.Fatal(err)
					}
				}
				want, err := os.ReadFile(path)
				if err != nil {
					t.Fatal(err)
				}
				if got != string(want) {
					t.Fatalf("%s output differs\ngot %s\nwant %s", phase, got, want)
				}
				tag := "guide.md#L1-L2"
				if state == "stale" || state == "refreshing" {
					tag += " [citation " + state + "]"
				}
				if !strings.Contains(got, tag) {
					t.Fatalf("%s lost citation status: %s", phase, got)
				}
			}
		})
	}
}
