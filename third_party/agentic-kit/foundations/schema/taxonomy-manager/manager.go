package taxonomymanager

import "fmt"

// Manager maintains a classification hierarchy for a single Taxonomy.
//
// The zero value is not usable; create a Manager with NewManager.
type Manager struct {
	taxonomy   Taxonomy
	categories map[string]Category // keyed by Category.ID
	children   map[string][]string // parent ID → sorted child IDs
}

// NewManager creates a Manager for the given Taxonomy.
func NewManager(t Taxonomy) *Manager {
	return &Manager{
		taxonomy:   t,
		categories: make(map[string]Category),
		children:   make(map[string][]string),
	}
}

// Add inserts cat into the hierarchy.
//
// Rules enforced:
//   - cat.ID must be non-empty.
//   - cat.Label must be non-empty.
//   - cat.ID must be unique; re-adding the same ID returns an error.
//   - cat.Parent must be "" (root) or the ID of an already-added category.
//   - The addition must not create a cycle (rejected immediately).
func (m *Manager) Add(cat Category) error {
	if cat.ID == "" {
		return fmt.Errorf("taxonomymanager: Category.ID must not be empty")
	}
	if cat.Label == "" {
		return fmt.Errorf("taxonomymanager: Category.Label must not be empty")
	}
	if _, exists := m.categories[cat.ID]; exists {
		return fmt.Errorf("taxonomymanager: category %q already exists", cat.ID)
	}
	if cat.Parent != "" {
		if _, ok := m.categories[cat.Parent]; !ok {
			return fmt.Errorf("taxonomymanager: parent %q not found for category %q", cat.Parent, cat.ID)
		}
		// Cycle check: walk up from cat.Parent; if we encounter cat.ID we would
		// create a cycle. This is O(depth) and sufficient because cycles cannot
		// already exist in the stored graph (the invariant is maintained on every
		// Add call).
		ancestor := cat.Parent
		for ancestor != "" {
			if ancestor == cat.ID {
				return fmt.Errorf("taxonomymanager: adding category %q would create a cycle", cat.ID)
			}
			ancestor = m.categories[ancestor].Parent
		}
	}

	m.categories[cat.ID] = cat
	m.children[cat.Parent] = append(m.children[cat.Parent], cat.ID)
	return nil
}

// Children returns the immediate children of parentID in insertion order.
// Returns an empty slice when parentID has no children or does not exist.
func (m *Manager) Children(parentID string) []Category {
	ids := m.children[parentID]
	if len(ids) == 0 {
		return []Category{}
	}
	out := make([]Category, 0, len(ids))
	for _, id := range ids {
		if c, ok := m.categories[id]; ok {
			out = append(out, c)
		}
	}
	return out
}

// Ancestors returns the ordered list of ancestor categories for id, starting
// from the immediate parent up to (and including) the root.
//
// Returns an empty slice for root categories or unknown IDs.
func (m *Manager) Ancestors(id string) []Category {
	cat, ok := m.categories[id]
	if !ok {
		return []Category{}
	}
	var out []Category
	cur := cat.Parent
	for cur != "" {
		ancestor, exists := m.categories[cur]
		if !exists {
			break
		}
		out = append(out, ancestor)
		cur = ancestor.Parent
	}
	return out
}

// IsDescendant reports whether candidate is a descendant of ancestor (i.e.
// ancestor appears somewhere on the path from candidate up to the root).
//
// Returns false if either ID is unknown, or if candidate == ancestor.
func (m *Manager) IsDescendant(candidate, ancestor string) bool {
	if candidate == "" || ancestor == "" {
		return false
	}
	cat, ok := m.categories[candidate]
	if !ok {
		return false
	}
	cur := cat.Parent
	for cur != "" {
		if cur == ancestor {
			return true
		}
		parent, exists := m.categories[cur]
		if !exists {
			break
		}
		cur = parent.Parent
	}
	return false
}

// Taxonomy returns the Taxonomy metadata associated with this Manager.
func (m *Manager) Taxonomy() Taxonomy {
	return m.taxonomy
}
