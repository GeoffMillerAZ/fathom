package swarmorchestrator

import (
	"testing"
)

func TestPlan_Empty(t *testing.T) {
	o := NewOrchestrator()
	waves := o.Plan(nil, 3)
	if len(waves) != 0 {
		t.Fatalf("expected 0 waves for empty task set, got %d", len(waves))
	}
}

func TestPlan_SingleTask(t *testing.T) {
	o := NewOrchestrator()
	tasks := []Task{{ID: "t1", Label: "Task 1"}}
	waves := o.Plan(tasks, 3)
	if len(waves) != 1 {
		t.Fatalf("expected 1 wave, got %d", len(waves))
	}
	if len(waves[0].TaskIDs) != 1 || waves[0].TaskIDs[0] != "t1" {
		t.Fatalf("unexpected wave content: %v", waves[0])
	}
}

func TestPlan_Batching(t *testing.T) {
	o := NewOrchestrator()
	tasks := []Task{
		{ID: "t3", Label: "Task 3"},
		{ID: "t1", Label: "Task 1"},
		{ID: "t2", Label: "Task 2"},
		{ID: "t4", Label: "Task 4"},
		{ID: "t5", Label: "Task 5"},
	}
	waves := o.Plan(tasks, 2)
	if len(waves) != 3 {
		t.Fatalf("expected 3 waves for 5 tasks with concurrency 2, got %d", len(waves))
	}
	// Wave 0 should have t1,t2 (sorted)
	if len(waves[0].TaskIDs) != 2 {
		t.Fatalf("wave 0 should have 2 tasks, got %d", len(waves[0].TaskIDs))
	}
	if waves[0].TaskIDs[0] != "t1" || waves[0].TaskIDs[1] != "t2" {
		t.Fatalf("wave 0 expected [t1 t2], got %v", waves[0].TaskIDs)
	}
	// Wave indices should be 0,1,2
	for i, w := range waves {
		if w.Index != i {
			t.Errorf("wave %d has Index %d", i, w.Index)
		}
	}
}

func TestPlan_Determinism(t *testing.T) {
	tasks := []Task{
		{ID: "z", Label: "Z"},
		{ID: "a", Label: "A"},
		{ID: "m", Label: "M"},
	}
	o1 := NewOrchestrator()
	w1 := o1.Plan(tasks, 2)

	o2 := NewOrchestrator()
	// Reverse order input
	rev := []Task{tasks[2], tasks[1], tasks[0]}
	w2 := o2.Plan(rev, 2)

	if len(w1) != len(w2) {
		t.Fatalf("plans differ in wave count: %d vs %d", len(w1), len(w2))
	}
	for i := range w1 {
		if len(w1[i].TaskIDs) != len(w2[i].TaskIDs) {
			t.Fatalf("wave %d differs in size", i)
		}
		for j := range w1[i].TaskIDs {
			if w1[i].TaskIDs[j] != w2[i].TaskIDs[j] {
				t.Errorf("wave %d task %d differs: %q vs %q", i, j, w1[i].TaskIDs[j], w2[i].TaskIDs[j])
			}
		}
	}
}

func TestAssign_Complete(t *testing.T) {
	o := NewOrchestrator()
	tasks := []Task{{ID: "t1", Label: "Task 1"}}
	o.Plan(tasks, 1)

	if err := o.Assign("t1"); err != nil {
		t.Fatalf("Assign: %v", err)
	}
	task, ok := o.Get("t1")
	if !ok {
		t.Fatal("task not found")
	}
	if task.Status != TaskAssigned {
		t.Fatalf("expected TaskAssigned, got %v", task.Status)
	}

	if err := o.Complete("t1", false); err != nil {
		t.Fatalf("Complete: %v", err)
	}
	task, _ = o.Get("t1")
	if task.Status != TaskDone {
		t.Fatalf("expected TaskDone, got %v", task.Status)
	}
}

func TestComplete_Failed(t *testing.T) {
	o := NewOrchestrator()
	o.Plan([]Task{{ID: "t1", Label: "T1"}}, 1)
	_ = o.Assign("t1")
	if err := o.Complete("t1", true); err != nil {
		t.Fatalf("Complete(failed): %v", err)
	}
	task, _ := o.Get("t1")
	if task.Status != TaskFailed {
		t.Fatalf("expected TaskFailed, got %v", task.Status)
	}
}

func TestAssign_NotFound(t *testing.T) {
	o := NewOrchestrator()
	if err := o.Assign("ghost"); err != ErrNotFound {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestComplete_NotFound(t *testing.T) {
	o := NewOrchestrator()
	if err := o.Complete("ghost", false); err != ErrNotFound {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestComplete_NotAssigned(t *testing.T) {
	o := NewOrchestrator()
	o.Plan([]Task{{ID: "t1", Label: "T1"}}, 1)
	if err := o.Complete("t1", false); err != ErrNotAssigned {
		t.Fatalf("expected ErrNotAssigned, got %v", err)
	}
}

func TestComplete_AlreadyComplete(t *testing.T) {
	o := NewOrchestrator()
	o.Plan([]Task{{ID: "t1", Label: "T1"}}, 1)
	_ = o.Assign("t1")
	_ = o.Complete("t1", false)
	if err := o.Complete("t1", false); err != ErrAlreadyComplete {
		t.Fatalf("expected ErrAlreadyComplete, got %v", err)
	}
}

func TestPlan_MaxConcurrencyZeroTreatedAsOne(t *testing.T) {
	o := NewOrchestrator()
	tasks := []Task{{ID: "t1", Label: "T1"}, {ID: "t2", Label: "T2"}}
	waves := o.Plan(tasks, 0)
	if len(waves) != 2 {
		t.Fatalf("expected 2 waves (concurrency=1), got %d", len(waves))
	}
}
