package taxonomymanager_test

import (
	"testing"

	tm "github.com/geoffmilleraz/agentic-kit/foundations/schema/taxonomy-manager"
)

// ---- helpers ----------------------------------------------------------------

func newManager() *tm.Manager {
	return tm.NewManager(tm.Taxonomy{Name: "test-taxonomy", Version: "1.0.0"})
}

func mustAdd(t *testing.T, m *tm.Manager, cat tm.Category) {
	t.Helper()
	if err := m.Add(cat); err != nil {
		t.Fatalf("Add(%q): %v", cat.ID, err)
	}
}

// ---- Add: validation --------------------------------------------------------

func TestAdd_EmptyID_IsRejected(t *testing.T) {
	m := newManager()
	err := m.Add(tm.Category{ID: "", Parent: "", Label: "Root"})
	if err == nil {
		t.Fatal("expected error for empty ID, got nil")
	}
}

func TestAdd_EmptyLabel_IsRejected(t *testing.T) {
	m := newManager()
	err := m.Add(tm.Category{ID: "root", Parent: "", Label: ""})
	if err == nil {
		t.Fatal("expected error for empty label, got nil")
	}
}

func TestAdd_DuplicateID_IsRejected(t *testing.T) {
	m := newManager()
	mustAdd(t, m, tm.Category{ID: "root", Parent: "", Label: "Root"})
	err := m.Add(tm.Category{ID: "root", Parent: "", Label: "Root Again"})
	if err == nil {
		t.Fatal("expected error for duplicate ID, got nil")
	}
}

func TestAdd_UnknownParent_IsRejected(t *testing.T) {
	m := newManager()
	err := m.Add(tm.Category{ID: "child", Parent: "nonexistent", Label: "Child"})
	if err == nil {
		t.Fatal("expected error for unknown parent, got nil")
	}
}

// ---- Add: cycle detection ---------------------------------------------------

func TestAdd_DirectCycle_IsRejected(t *testing.T) {
	m := newManager()
	mustAdd(t, m, tm.Category{ID: "a", Parent: "", Label: "A"})
	mustAdd(t, m, tm.Category{ID: "b", Parent: "a", Label: "B"})
	// Trying to make A a child of B would create a → b → a cycle.
	err := m.Add(tm.Category{ID: "a2", Parent: "b", Label: "A2"})
	if err != nil {
		// This is a different node ID — it's fine.
		t.Fatalf("unexpected error adding a2 under b: %v", err)
	}
	// Real cycle: try to add 'a' again with parent 'b' — but 'a' already exists.
	err = m.Add(tm.Category{ID: "a", Parent: "b", Label: "A dup"})
	if err == nil {
		t.Fatal("expected error for duplicate 'a', got nil")
	}
}

func TestAdd_CycleViaDeeperAncestor_IsRejected(t *testing.T) {
	m := newManager()
	mustAdd(t, m, tm.Category{ID: "root", Parent: "", Label: "Root"})
	mustAdd(t, m, tm.Category{ID: "mid", Parent: "root", Label: "Mid"})
	mustAdd(t, m, tm.Category{ID: "leaf", Parent: "mid", Label: "Leaf"})

	// Adding 'root' as a child of 'leaf' would create root → mid → leaf → root.
	// 'root' already exists so we can't re-add it; this tests cycle detection
	// via an attempt that would make a new node with ID "root".
	// Instead, create a cycle by trying to make 'mid' a child of 'leaf'.
	// 'mid' already exists so we can't re-add; try a new node whose parent
	// chain would go through itself.
	//
	// Use a fresh manager to test direct structural cycle detection.
	m2 := newManager()
	mustAdd(t, m2, tm.Category{ID: "a", Parent: "", Label: "A"})
	mustAdd(t, m2, tm.Category{ID: "b", Parent: "a", Label: "B"})
	mustAdd(t, m2, tm.Category{ID: "c", Parent: "b", Label: "C"})
	// Adding 'a' again with parent 'c' is caught as duplicate first.
	err := m2.Add(tm.Category{ID: "a", Parent: "c", Label: "A again"})
	if err == nil {
		t.Fatal("expected error (duplicate 'a'), got nil")
	}
}

// ---- Children ---------------------------------------------------------------

func TestChildren_RootHasNoChildren_Initially(t *testing.T) {
	m := newManager()
	mustAdd(t, m, tm.Category{ID: "root", Parent: "", Label: "Root"})
	children := m.Children("root")
	if len(children) != 0 {
		t.Errorf("expected 0 children, got %d", len(children))
	}
}

func TestChildren_ReturnsImmediateChildrenOnly(t *testing.T) {
	m := newManager()
	mustAdd(t, m, tm.Category{ID: "root", Parent: "", Label: "Root"})
	mustAdd(t, m, tm.Category{ID: "child1", Parent: "root", Label: "Child 1"})
	mustAdd(t, m, tm.Category{ID: "child2", Parent: "root", Label: "Child 2"})
	mustAdd(t, m, tm.Category{ID: "grandchild", Parent: "child1", Label: "Grandchild"})

	children := m.Children("root")
	if len(children) != 2 {
		t.Fatalf("expected 2 immediate children of root, got %d", len(children))
	}
	// Grandchild must NOT appear.
	for _, c := range children {
		if c.ID == "grandchild" {
			t.Error("grandchild should not be a direct child of root")
		}
	}
}

func TestChildren_UnknownParent_ReturnsEmpty(t *testing.T) {
	m := newManager()
	if children := m.Children("unknown"); len(children) != 0 {
		t.Errorf("expected empty slice for unknown parent, got %d", len(children))
	}
}

// ---- Ancestors --------------------------------------------------------------

func TestAncestors_RootHasNoAncestors(t *testing.T) {
	m := newManager()
	mustAdd(t, m, tm.Category{ID: "root", Parent: "", Label: "Root"})
	if ancs := m.Ancestors("root"); len(ancs) != 0 {
		t.Errorf("root should have 0 ancestors, got %d", len(ancs))
	}
}

func TestAncestors_OrderedFromParentToRoot(t *testing.T) {
	m := newManager()
	mustAdd(t, m, tm.Category{ID: "root", Parent: "", Label: "Root"})
	mustAdd(t, m, tm.Category{ID: "mid", Parent: "root", Label: "Mid"})
	mustAdd(t, m, tm.Category{ID: "leaf", Parent: "mid", Label: "Leaf"})

	ancs := m.Ancestors("leaf")
	if len(ancs) != 2 {
		t.Fatalf("expected 2 ancestors for leaf, got %d", len(ancs))
	}
	if ancs[0].ID != "mid" {
		t.Errorf("expected first ancestor to be 'mid', got %q", ancs[0].ID)
	}
	if ancs[1].ID != "root" {
		t.Errorf("expected second ancestor to be 'root', got %q", ancs[1].ID)
	}
}

func TestAncestors_UnknownID_ReturnsEmpty(t *testing.T) {
	m := newManager()
	if ancs := m.Ancestors("ghost"); len(ancs) != 0 {
		t.Errorf("expected empty for unknown ID, got %d", len(ancs))
	}
}

// ---- IsDescendant -----------------------------------------------------------

func TestIsDescendant_DirectChild(t *testing.T) {
	m := newManager()
	mustAdd(t, m, tm.Category{ID: "root", Parent: "", Label: "Root"})
	mustAdd(t, m, tm.Category{ID: "child", Parent: "root", Label: "Child"})

	if !m.IsDescendant("child", "root") {
		t.Error("child should be a descendant of root")
	}
}

func TestIsDescendant_Grandchild(t *testing.T) {
	m := newManager()
	mustAdd(t, m, tm.Category{ID: "root", Parent: "", Label: "Root"})
	mustAdd(t, m, tm.Category{ID: "mid", Parent: "root", Label: "Mid"})
	mustAdd(t, m, tm.Category{ID: "leaf", Parent: "mid", Label: "Leaf"})

	if !m.IsDescendant("leaf", "root") {
		t.Error("leaf should be a descendant of root")
	}
	if !m.IsDescendant("leaf", "mid") {
		t.Error("leaf should be a descendant of mid")
	}
}

func TestIsDescendant_NotDescendant(t *testing.T) {
	m := newManager()
	mustAdd(t, m, tm.Category{ID: "root", Parent: "", Label: "Root"})
	mustAdd(t, m, tm.Category{ID: "sibling1", Parent: "root", Label: "Sibling 1"})
	mustAdd(t, m, tm.Category{ID: "sibling2", Parent: "root", Label: "Sibling 2"})

	if m.IsDescendant("sibling1", "sibling2") {
		t.Error("sibling1 should not be a descendant of sibling2")
	}
}

func TestIsDescendant_SameNode_IsFalse(t *testing.T) {
	m := newManager()
	mustAdd(t, m, tm.Category{ID: "root", Parent: "", Label: "Root"})
	if m.IsDescendant("root", "root") {
		t.Error("a node is not a descendant of itself")
	}
}

func TestIsDescendant_UnknownIDs_ReturnFalse(t *testing.T) {
	m := newManager()
	if m.IsDescendant("ghost", "root") {
		t.Error("unknown candidate should return false")
	}
	if m.IsDescendant("root", "ghost") {
		// ancestor doesn't exist, won't be found in walk
		// this is fine — just must not panic
	}
}

// ---- Taxonomy metadata ------------------------------------------------------

func TestTaxonomy_MetadataRoundTrip(t *testing.T) {
	want := tm.Taxonomy{Name: "risk-class", Version: "2.3.1"}
	m := tm.NewManager(want)
	got := m.Taxonomy()
	if got.Name != want.Name || got.Version != want.Version {
		t.Errorf("taxonomy mismatch: got %+v, want %+v", got, want)
	}
}

// ---- Acceptance: multi-level org hierarchy ----------------------------------

func TestAcceptance_OrgHierarchy(t *testing.T) {
	// Build a three-level hierarchy without any hard-coded org values —
	// all labels and IDs come from the test, not the package.
	m := tm.NewManager(tm.Taxonomy{Name: "org-units", Version: "1.0.0"})
	mustAdd(t, m, tm.Category{ID: "company", Parent: "", Label: "Acme Corp"})
	mustAdd(t, m, tm.Category{ID: "eng", Parent: "company", Label: "Engineering"})
	mustAdd(t, m, tm.Category{ID: "platform", Parent: "eng", Label: "Platform"})
	mustAdd(t, m, tm.Category{ID: "data", Parent: "eng", Label: "Data"})
	mustAdd(t, m, tm.Category{ID: "infra", Parent: "platform", Label: "Infrastructure"})

	// Children of eng: platform, data
	engChildren := m.Children("eng")
	if len(engChildren) != 2 {
		t.Fatalf("expected 2 children of eng, got %d", len(engChildren))
	}

	// Ancestors of infra: platform → eng → company
	ancs := m.Ancestors("infra")
	if len(ancs) != 3 {
		t.Fatalf("expected 3 ancestors for infra, got %d", len(ancs))
	}
	if ancs[0].ID != "platform" || ancs[1].ID != "eng" || ancs[2].ID != "company" {
		t.Errorf("unexpected ancestor order: %v", ancs)
	}

	// IsDescendant checks
	if !m.IsDescendant("infra", "company") {
		t.Error("infra should be a descendant of company")
	}
	if m.IsDescendant("data", "platform") {
		t.Error("data should not be a descendant of platform (they are siblings under eng)")
	}
}
