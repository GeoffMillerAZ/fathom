package scenariobench

import (
	"context"
	"testing"
)

func TestWrongPatchSkipsSparseExcludedFiles(t *testing.T) {
	repo, _ := gitFixtureRepoWithFiles(t, map[string]string{
		"aaa-hidden.go": "hidden", "gold.go": "gold", "visible one.go": "one", "visible-two.go": "two", ".settings": "hidden settings",
	})
	ctx := context.Background()
	if err := removeExcludedPaths(ctx, repo, []string{"aaa-hidden.go"}); err != nil {
		t.Fatal(err)
	}
	task := Task{Oracle: Oracle{Localization: true}, Gold: Gold{Files: []string{"gold.go"}}}
	h := NewWrongPatchHarness().withTask(ctx, task)
	result, err := h.RunSession(SessionRequest{Worktree: repo})
	if err != nil {
		t.Fatal(err)
	}
	if result.FinalAnswer != "marker.txt\nvisible one.go\nvisible-two.go" {
		t.Fatalf("answer includes sparse-excluded or hidden/gold paths: %q", result.FinalAnswer)
	}
}
