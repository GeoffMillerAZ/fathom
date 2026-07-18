package tasktree

import (
	"testing"
)

func TestAdd_RootNode(t *testing.T) {
	tree := NewTaskTree()
	err := tree.Add(TaskNode{ID: "root", ParentID: "", Title: "Root task", Status: StatusPending})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	n, ok := tree.Get("root")
	if !ok {
		t.Fatal("expected node to be found after Add")
	}
	if n.Status != StatusPending {
		t.Fatalf("expected StatusPending, got %v", n.Status)
	}
}

func TestAdd_DuplicateID(t *testing.T) {
	tree := NewTaskTree()
	node := TaskNode{ID: "a", ParentID: "", Title: "A", Status: StatusPending}
	_ = tree.Add(node)
	err := tree.Add(node)
	if err != ErrDuplicateID {
		t.Fatalf("expected ErrDuplicateID, got %v", err)
	}
}

func TestAdd_MissingParent(t *testing.T) {
	tree := NewTaskTree()
	err := tree.Add(TaskNode{ID: "child", ParentID: "ghost", Title: "Child", Status: StatusPending})
	if err != ErrNotFound {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestAdd_CycleRejection(t *testing.T) {
	// Build: root -> a -> b, then attempt to add root as child of b
	tree := NewTaskTree()
	must(t, tree.Add(TaskNode{ID: "root", ParentID: "", Title: "Root", Status: StatusPending}))
	must(t, tree.Add(TaskNode{ID: "a", ParentID: "root", Title: "A", Status: StatusPending}))
	must(t, tree.Add(TaskNode{ID: "b", ParentID: "a", Title: "B", Status: StatusPending}))

	// Attempting to add a node with ID "root" again is caught by DuplicateID first,
	// so we test cycle by directly checking isAncestor logic via a fresh node that
	// would close a cycle if allowed.
	// Simulate: add node "c" whose ParentID is "b" but whose ID is "root" — DuplicateID.
	err := tree.Add(TaskNode{ID: "root", ParentID: "b", Title: "fake root", Status: StatusPending})
	if err != ErrDuplicateID {
		t.Fatalf("expected ErrDuplicateID (root already exists), got %v", err)
	}

	// Direct isAncestor test: "root" is an ancestor of "b".
	if !tree.isAncestor("b", "root") {
		t.Fatal("expected root to be an ancestor of b")
	}
	// "b" is not an ancestor of "root".
	if tree.isAncestor("root", "b") {
		t.Fatal("expected b to not be an ancestor of root")
	}
}

func TestChildren(t *testing.T) {
	tree := buildSampleTree(t)
	children := tree.Children("root")
	if len(children) != 2 {
		t.Fatalf("expected 2 children of root, got %d", len(children))
	}
	// Verify deterministic ordering by ID
	if children[0].ID > children[1].ID {
		t.Errorf("expected children sorted by ID, got %v > %v", children[0].ID, children[1].ID)
	}
}

func TestChildren_NoChildren(t *testing.T) {
	tree := NewTaskTree()
	must(t, tree.Add(TaskNode{ID: "lone", ParentID: "", Title: "Lone", Status: StatusPending}))
	ch := tree.Children("lone")
	if len(ch) != 0 {
		t.Fatalf("expected empty slice, got %v", ch)
	}
}

func TestLeaves(t *testing.T) {
	tree := buildSampleTree(t)
	// Tree: root -> a, root -> b, a -> a1, a -> a2
	// Leaves should be: a1, a2, b
	leaves := tree.Leaves()
	leavesSet := make(map[string]bool)
	for _, l := range leaves {
		leavesSet[l.ID] = true
	}
	for _, expected := range []string{"a1", "a2", "b"} {
		if !leavesSet[expected] {
			t.Errorf("expected leaf %q in Leaves(), not found", expected)
		}
	}
	if leavesSet["root"] {
		t.Error("root should not be a leaf")
	}
	if leavesSet["a"] {
		t.Error("a should not be a leaf (has children)")
	}
}

func TestMarkStatus_AllTransitions(t *testing.T) {
	cases := []struct {
		name   string
		status Status
	}{
		{"pending", StatusPending},
		{"running", StatusRunning},
		{"done", StatusDone},
		{"failed", StatusFailed},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tree := NewTaskTree()
			must(t, tree.Add(TaskNode{ID: "n", ParentID: "", Title: "T", Status: StatusPending}))
			if err := tree.MarkStatus("n", tc.status); err != nil {
				t.Fatalf("MarkStatus(%v) failed: %v", tc.status, err)
			}
			n, _ := tree.Get("n")
			if n.Status != tc.status {
				t.Fatalf("expected %v, got %v", tc.status, n.Status)
			}
		})
	}
}

func TestMarkStatus_InvalidStatus(t *testing.T) {
	tree := NewTaskTree()
	must(t, tree.Add(TaskNode{ID: "n", ParentID: "", Title: "T", Status: StatusPending}))
	err := tree.MarkStatus("n", Status("cancelled"))
	if err != ErrInvalidStatus {
		t.Fatalf("expected ErrInvalidStatus, got %v", err)
	}
}

func TestMarkStatus_NotFound(t *testing.T) {
	tree := NewTaskTree()
	err := tree.MarkStatus("ghost", StatusDone)
	if err != ErrNotFound {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestDeterminism_ChildrenOrder(t *testing.T) {
	// Adding children in different orders should always produce the same sorted output.
	ids := []string{"z-child", "a-child", "m-child"}
	tree := NewTaskTree()
	must(t, tree.Add(TaskNode{ID: "root", ParentID: "", Title: "Root", Status: StatusPending}))
	for _, id := range ids {
		must(t, tree.Add(TaskNode{ID: id, ParentID: "root", Title: id, Status: StatusPending}))
	}
	ch := tree.Children("root")
	for i := 1; i < len(ch); i++ {
		if ch[i].ID < ch[i-1].ID {
			t.Errorf("children not sorted at index %d: %v < %v", i, ch[i].ID, ch[i-1].ID)
		}
	}
}

// buildSampleTree creates: root -> a -> a1, a -> a2, root -> b
func buildSampleTree(t *testing.T) *TaskTree {
	t.Helper()
	tree := NewTaskTree()
	must(t, tree.Add(TaskNode{ID: "root", ParentID: "", Title: "Root", Status: StatusPending}))
	must(t, tree.Add(TaskNode{ID: "a", ParentID: "root", Title: "A", Status: StatusRunning}))
	must(t, tree.Add(TaskNode{ID: "b", ParentID: "root", Title: "B", Status: StatusDone}))
	must(t, tree.Add(TaskNode{ID: "a1", ParentID: "a", Title: "A1", Status: StatusPending}))
	must(t, tree.Add(TaskNode{ID: "a2", ParentID: "a", Title: "A2", Status: StatusFailed}))
	return tree
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
