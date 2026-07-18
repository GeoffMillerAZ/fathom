package entitycrosswalk

import (
	"fmt"
	"sync"
	"time"
)

// Crosswalk maintains goroutine-safe, append-only, bidirectional identity
// mappings between canonical IDs and their aliases. Construct via New.
//
// Append-only: Add returns ErrAliasConflict rather than overwriting an existing
// mapping. To remap an alias, record a supersession in your own ledger (e.g.
// the event-store) and build a fresh Crosswalk from the corrected event stream.
//
// Transitive resolution: Canonical() follows multi-hop chains to the root.
// Example: if B->A and A->C are both registered, Canonical("B") returns "C".
type Crosswalk struct {
	mu sync.RWMutex

	// canonical maps each alias to its directly registered canonical ID.
	// Transitive resolution is performed at query time by following chains.
	canonical map[string]string

	// aliases maps each canonical ID to its current set of direct aliases.
	aliases map[string]map[string]struct{}

	// history is the ordered, append-only record of every mapping added.
	history []MappingEntry

	// now is the injected clock; never call time.Now() directly in domain logic.
	now func() time.Time
}

// New returns a Crosswalk that timestamps entries using the supplied clock.
// clock must not be nil — inject a fixed clock in tests for determinism.
func New(clock func() time.Time) *Crosswalk {
	if clock == nil {
		panic("entitycrosswalk: New: clock must not be nil")
	}
	return &Crosswalk{
		canonical: make(map[string]string),
		aliases:   make(map[string]map[string]struct{}),
		now:       clock,
	}
}

// Add records that alias refers to the same real-world entity as canonical.
//
// Validation rules (all return a descriptive error on failure):
//   - canonical and alias must each be non-empty.
//   - canonical and alias must differ (self-alias is meaningless).
//   - author must satisfy Author.Validate().
//   - If alias is already mapped to a different canonical, ErrAliasConflict is
//     returned and the existing mapping is left intact (append-only invariant).
//
// Idempotent: adding the same (canonical, alias) pair more than once is a
// no-op (no duplicate history entries, no error).
//
// Thread-safe: safe to call concurrently with Add, Canonical, Aliases, History.
func (c *Crosswalk) Add(canonical, alias string, author Author) error {
	if canonical == "" {
		return fmt.Errorf("entitycrosswalk: canonical must not be empty")
	}
	if alias == "" {
		return fmt.Errorf("entitycrosswalk: alias must not be empty")
	}
	if canonical == alias {
		return fmt.Errorf("entitycrosswalk: canonical and alias must differ, got %q", canonical)
	}
	if err := author.Validate(); err != nil {
		return fmt.Errorf("entitycrosswalk: invalid author: %w", err)
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	// Idempotent: same pair already registered — no-op.
	if existing, ok := c.canonical[alias]; ok && existing == canonical {
		return nil
	}

	// Conflict: alias already maps to a different canonical — preserve append-only.
	if existing, ok := c.canonical[alias]; ok && existing != canonical {
		return fmt.Errorf("%w: alias %q already mapped to %q (requested %q)",
			ErrAliasConflict, alias, existing, canonical)
	}

	// Record the mapping.
	c.canonical[alias] = canonical
	if c.aliases[canonical] == nil {
		c.aliases[canonical] = make(map[string]struct{})
	}
	c.aliases[canonical][alias] = struct{}{}

	c.history = append(c.history, MappingEntry{
		CanonicalID: canonical,
		AliasID:     alias,
		Author:      author,
		RecordedAt:  c.now(),
	})
	return nil
}

// Canonical returns the root canonical ID for the given alias, following
// multi-hop chains until an ID is reached that is not itself an alias.
// Returns the empty string if alias is not known to this crosswalk.
//
// Example: if B is an alias of A, and A is an alias of C, Canonical("B")
// returns "C".
//
// Cycle guard: if a cycle exists in the mapping (which Add cannot produce
// under normal use but could arise from a corrupted replay), Canonical breaks
// after visiting more IDs than are registered and returns what it last resolved.
func (c *Crosswalk) Canonical(alias string) string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if len(c.canonical) == 0 {
		return ""
	}

	// Follow the chain to the root. The maximum chain length is bounded by the
	// number of registered canonical entries; use that as the cycle-break limit.
	maxHops := len(c.canonical) + 1
	current := alias
	for i := 0; i < maxHops; i++ {
		next, ok := c.canonical[current]
		if !ok {
			// current is not a registered alias — it is the root.
			if current == alias {
				// alias itself was never registered.
				return ""
			}
			return current
		}
		current = next
	}
	// Cycle detected; return best guess.
	return current
}

// Aliases returns the sorted list of direct aliases registered for the given
// canonical ID. The returned slice is a defensive copy; the caller may freely
// modify it. Returns nil if canonical has no aliases.
//
// Note: Aliases returns only direct (one-hop) aliases. IDs that transitively
// resolve to canonical via an intermediate alias are not included.
func (c *Crosswalk) Aliases(canonical string) []string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	m := c.aliases[canonical]
	if len(m) == 0 {
		return nil
	}
	return sortedKeys(m)
}

// History returns a defensive copy of every mapping entry in the order it was
// added. The caller may freely modify the returned slice without affecting the
// Crosswalk's internal state.
func (c *Crosswalk) History() []MappingEntry {
	c.mu.RLock()
	defer c.mu.RUnlock()

	out := make([]MappingEntry, len(c.history))
	copy(out, c.history)
	return out
}

// sortedKeys returns the keys of a string set in sorted order.
// The result is always a fresh slice (defensive copy).
func sortedKeys(m map[string]struct{}) []string {
	if len(m) == 0 {
		return nil
	}
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	insertionSort(out)
	return out
}

// insertionSort sorts a string slice in-place in ascending order.
// Allocation-free; acceptable for the small alias sets typical in this package.
func insertionSort(ss []string) {
	for i := 1; i < len(ss); i++ {
		key := ss[i]
		j := i - 1
		for j >= 0 && ss[j] > key {
			ss[j+1] = ss[j]
			j--
		}
		ss[j+1] = key
	}
}
