package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestMaxTurnsPrintsHandoff proves the CLI surfaces the harness Handoff: a
// run cut off by -max-turns exits 1 (as before) AND hands over what it tried
// and where to resume, instead of a bare stop cause.
func TestMaxTurnsPrintsHandoff(t *testing.T) {
	code, stdout, stderr := runCLI(t, "-mode", "mock", "-max-turns", "1")
	if code != 1 {
		t.Fatalf("exit code = %d, want 1; stderr:\n%s", code, stderr)
	}
	if !strings.Contains(stdout, "stop=max_turns") {
		t.Errorf("stdout missing stop=max_turns:\n%s", stdout)
	}
	for _, want := range []string{"agent-cli: handoff: run stopped on max_turns after 1 turn(s)", "last tool read_file", "tried [read_file]", "resume from checkpoint:v1:turn1:"} {
		if !strings.Contains(stderr, want) {
			t.Errorf("stderr missing %q:\n%s", want, stderr)
		}
	}
}

// TestContextBudgetWiresEvidenceCompactor proves the opt-in compaction path
// composes: a budget large enough that nothing is dropped runs the mock
// session unchanged, and the compaction journal exists (empty — nothing was
// dropped, so nothing was journaled, which is the honest outcome).
func TestContextBudgetWiresEvidenceCompactor(t *testing.T) {
	journal := filepath.Join(t.TempDir(), "compaction", "journal.jsonl")
	code, stdout, stderr := runCLI(t, "-mode", "mock", "-context-budget", "1000000", "-compaction-journal", journal)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr:\n%s", code, stderr)
	}
	if !strings.Contains(stdout, wantFinalText) {
		t.Errorf("stdout missing %q:\n%s", wantFinalText, stdout)
	}
	st, err := os.Stat(journal)
	if err != nil {
		t.Fatalf("compaction journal not created: %v", err)
	}
	if st.Size() != 0 {
		t.Errorf("journal size = %d, want 0 (no messages were dropped under a huge budget)", st.Size())
	}
}

func TestContextBudgetUsageErrors(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{name: "budget without journal", args: []string{"-mode", "mock", "-context-budget", "10"}, want: "requires -compaction-journal"},
		{name: "negative budget", args: []string{"-mode", "mock", "-context-budget", "-1", "-compaction-journal", "x.jsonl"}, want: "must not be negative"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			code, _, stderr := runCLI(t, tc.args...)
			if code != 2 {
				t.Fatalf("exit code = %d, want 2; stderr:\n%s", code, stderr)
			}
			if !strings.Contains(stderr, tc.want) {
				t.Errorf("stderr missing %q:\n%s", tc.want, stderr)
			}
		})
	}
}
