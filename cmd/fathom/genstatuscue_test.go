package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/store"
)

// TestGenStatusCue_PassVerdict seeds a tiny SQLite store with a verified
// symbol and a test that reaches it, then runs gen-status-cue and asserts
// that the output STATUS.cue contains a "pass" verdict for the task.
func TestGenStatusCue_PassVerdict(t *testing.T) {
	dir := t.TempDir()
	storePath := filepath.Join(dir, "test.sqlite")

	// Seed the store.  We open it directly (bypassing the client) so we can
	// populate raw observations, then close it to release the single-writer
	// lock before the CLI command opens it.
	st, err := store.Open(storePath)
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	ctx := context.Background()

	obs := []core.Observation{
		{
			ServiceID:    "svc",
			SourceRef:    "do.go",
			Symbol:       "DoWork",
			Package:      "work",
			EvidenceTier: core.TierVerified,
			SourceKind:   core.KindGoFunc,
		},
		{
			ServiceID:    "svc",
			SourceRef:    "do_test.go",
			Symbol:       "TestDoWork",
			Package:      "work",
			EvidenceTier: core.TierVerified,
			SourceKind:   core.KindGoFunc,
			// TestDoWork calls DoWork — establishes the test_reaches edge.
			Calls: []string{"svc::work.DoWork"},
		},
	}
	if err := st.PutObservations(ctx, obs); err != nil {
		t.Fatalf("PutObservations: %v", err)
	}
	if err := st.Close(); err != nil {
		t.Fatalf("store.Close: %v", err)
	}

	// Write a minimal JSON plan file.
	type planFile struct {
		Service string `json:"service"`
		Tasks   []struct {
			ID     string `json:"id"`
			Expect struct {
				Symbol string `json:"symbol"`
			} `json:"expect"`
		} `json:"tasks"`
	}
	plan := planFile{Service: "svc"}
	plan.Tasks = append(plan.Tasks, struct {
		ID     string `json:"id"`
		Expect struct {
			Symbol string `json:"symbol"`
		} `json:"expect"`
	}{
		ID: "T-1",
		Expect: struct {
			Symbol string `json:"symbol"`
		}{Symbol: "DoWork"},
	})
	planBytes, err := json.Marshal(plan)
	if err != nil {
		t.Fatalf("json.Marshal plan: %v", err)
	}
	planPath := filepath.Join(dir, "plan.json")
	if err := os.WriteFile(planPath, planBytes, 0o644); err != nil {
		t.Fatalf("writing plan file: %v", err)
	}

	outPath := filepath.Join(dir, "STATUS.cue")

	var out, errBuf bytes.Buffer
	code := run([]string{
		"gen-status-cue",
		"--plan", planPath,
		"--service", "svc",
		"--store", storePath,
		"--output", outPath,
	}, &out, &errBuf)

	if code != 0 {
		t.Fatalf("gen-status-cue exited %d\nstdout=%s\nstderr=%s",
			code, out.String(), errBuf.String())
	}

	// stdout should include a summary line.
	if !strings.Contains(out.String(), "wrote") {
		t.Errorf("stdout should mention 'wrote'; got: %q", out.String())
	}

	// The output file must exist.
	content, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("reading output file: %v", err)
	}
	got := string(content)

	// Must contain the label field (present in every task verdict).
	if !strings.Contains(got, "label:") {
		t.Errorf("output missing 'label:'; got:\n%s", got)
	}

	// Must mention the symbol name.
	if !strings.Contains(got, "DoWork") {
		t.Errorf("output missing 'DoWork'; got:\n%s", got)
	}

	// The task should pass (DoWork is verified and TestDoWork reaches it).
	if !strings.Contains(got, `"pass"`) {
		t.Errorf("expected a pass verdict; got:\n%s", got)
	}
}

// TestGenStatusCue_MissingPlanFlag asserts that --plan is required.
func TestGenStatusCue_MissingPlanFlag(t *testing.T) {
	var out, errBuf bytes.Buffer
	code := run([]string{"gen-status-cue", "--service", "svc"}, &out, &errBuf)
	if code == 0 {
		t.Fatal("expected nonzero exit when --plan is missing")
	}
}

// TestGenStatusCue_MissingServiceFlag asserts that --service is required.
func TestGenStatusCue_MissingServiceFlag(t *testing.T) {
	var out, errBuf bytes.Buffer
	code := run([]string{"gen-status-cue", "--plan", "/tmp/plan.json"}, &out, &errBuf)
	if code == 0 {
		t.Fatal("expected nonzero exit when --service is missing")
	}
}
