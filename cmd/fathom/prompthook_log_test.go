package main

import "testing"

func TestPromptDefinitionsLogKindsMatchTestsToRun(t *testing.T) {
	for _, tc := range []struct {
		name, kind string
		defs       []promptDefinition
		missed     bool
	}{
		{"resolved", "line", []promptDefinition{{Name: "Round", Kind: "func", File: "money/money.go", Line: 3, Count: 1}}, false},
		{"deadline", "missed", nil, true},
		{"no definitions", "silent", nil, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := testRunRepo(t, false)
			t.Setenv("FATHOM_HOME", t.TempDir())
			writeRepoFile(t, repo, ".fathom/config.toml", "[hooks]\nprompt_definitions = \"observe\"\n")
			if text, ok := withPromptDefinitions(repo, "log-session", "context", true, tc.defs, tc.missed); text != "context" || !ok {
				t.Fatalf("observe changed context: %q, %v", text, ok)
			}
			beats, _ := readBeatLog(beatLogPath())
			if len(beats) != 1 || beats[0].Channel != "prompt-definitions" || beats[0].Kind != tc.kind || beats[0].Meta["emitted"] != "false" {
				t.Fatalf("log=%+v, want prompt-definitions kind=%s emitted=false", beats, tc.kind)
			}
			if tc.missed && beats[0].Meta["missed"] != "deadline" {
				t.Fatalf("miss lacks reason: %+v", beats[0])
			}
		})
	}
}
