package contextbench

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadTasks_RealCorpus(t *testing.T) {
	tasks, err := LoadTasks("../../testdata/contextbench/tasks.json")
	if err != nil {
		t.Fatalf("LoadTasks: %v", err)
	}
	if n := len(tasks); n < 10 || n > 15 {
		t.Fatalf("expected 10-15 tasks per docs/plans/falsifiability.md B1-1, got %d", n)
	}
	seen := map[string]bool{}
	for _, task := range tasks {
		if seen[task.ID] {
			t.Errorf("duplicate task id %q", task.ID)
		}
		seen[task.ID] = true
		if err := task.Validate(); err != nil {
			t.Errorf("task %s: %v", task.ID, err)
		}
	}
}

func TestLoadTasks_MissingFile(t *testing.T) {
	if _, err := LoadTasks("does-not-exist.json"); err == nil {
		t.Fatal("expected an error for a missing tasks file")
	}
}

func TestLoadTasks_MalformedJSON(t *testing.T) {
	path := filepath.Join(t.TempDir(), "tasks.json")
	if err := os.WriteFile(path, []byte("{not json"), 0o644); err != nil {
		t.Fatalf("writing fixture: %v", err)
	}
	if _, err := LoadTasks(path); err == nil {
		t.Fatal("expected an error for malformed JSON")
	}
}

func TestTaskValidate(t *testing.T) {
	cases := []struct {
		name    string
		task    Task
		wantErr bool
	}{
		{
			name: "valid all-of",
			task: Task{ID: "T1", Prompt: "p", GroundTruth: []string{"a", "b"}, ScoringRule: AllOf},
		},
		{
			name: "valid any-of",
			task: Task{ID: "T1", Prompt: "p", GroundTruth: []string{"a", "b"}, ScoringRule: AnyOf},
		},
		{
			name: "valid exact-match single item",
			task: Task{ID: "T1", Prompt: "p", GroundTruth: []string{"a"}, ScoringRule: ExactMatch},
		},
		{
			name:    "exact-match with multiple ground truth items is invalid",
			task:    Task{ID: "T1", Prompt: "p", GroundTruth: []string{"a", "b"}, ScoringRule: ExactMatch},
			wantErr: true,
		},
		{
			name:    "missing id",
			task:    Task{Prompt: "p", GroundTruth: []string{"a"}, ScoringRule: AllOf},
			wantErr: true,
		},
		{
			name:    "missing prompt",
			task:    Task{ID: "T1", GroundTruth: []string{"a"}, ScoringRule: AllOf},
			wantErr: true,
		},
		{
			name:    "empty ground truth",
			task:    Task{ID: "T1", Prompt: "p", ScoringRule: AllOf},
			wantErr: true,
		},
		{
			name:    "unknown scoring rule",
			task:    Task{ID: "T1", Prompt: "p", GroundTruth: []string{"a"}, ScoringRule: "sort-of"},
			wantErr: true,
		},
		{
			name: "absent class is valid (back-compat with the grep-native B1 corpus)",
			task: Task{ID: "T1", Prompt: "p", GroundTruth: []string{"a"}, ScoringRule: AllOf},
		},
		{
			name: "known class is valid",
			task: Task{ID: "T1", Prompt: "p", GroundTruth: []string{"a"}, ScoringRule: AllOf, Class: ClassObligations},
		},
		{
			name:    "unknown class is invalid",
			task:    Task{ID: "T1", Prompt: "p", GroundTruth: []string{"a"}, ScoringRule: AllOf, Class: "not-a-real-class"},
			wantErr: true,
		},
		{
			name: "answer_any_of alone (no ground_truth) is valid",
			task: Task{ID: "T1", Prompt: "p", AnswerAnyOf: [][]string{{"core.Edge", "Edge struct"}}, ScoringRule: AllOf},
		},
		{
			name: "ground_truth plus answer_any_of is valid",
			task: Task{ID: "T1", Prompt: "p", GroundTruth: []string{"ADR-021"}, AnswerAnyOf: [][]string{{"core.Edge", "Edge struct"}}, ScoringRule: AllOf},
		},
		{
			name:    "empty ground_truth and no answer_any_of is invalid",
			task:    Task{ID: "T1", Prompt: "p", ScoringRule: AllOf},
			wantErr: true,
		},
		{
			name:    "empty answer_any_of group is invalid",
			task:    Task{ID: "T1", Prompt: "p", GroundTruth: []string{"a"}, AnswerAnyOf: [][]string{{}}, ScoringRule: AllOf},
			wantErr: true,
		},
		{
			name:    "answer_any_of group with an empty string is invalid",
			task:    Task{ID: "T1", Prompt: "p", GroundTruth: []string{"a"}, AnswerAnyOf: [][]string{{"core.Edge", ""}}, ScoringRule: AllOf},
			wantErr: true,
		},
		{
			name: "exact-match with one answer_any_of group and no ground_truth is valid",
			task: Task{ID: "T1", Prompt: "p", AnswerAnyOf: [][]string{{"core.Edge", "Edge struct"}}, ScoringRule: ExactMatch},
		},
		{
			name:    "exact-match with a ground_truth item AND an answer_any_of group is invalid (two required items)",
			task:    Task{ID: "T1", Prompt: "p", GroundTruth: []string{"a"}, AnswerAnyOf: [][]string{{"core.Edge", "Edge struct"}}, ScoringRule: ExactMatch},
			wantErr: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.task.Validate()
			if tc.wantErr && err == nil {
				t.Fatal("expected an error, got nil")
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}
