// Package lookup provides a fast inverted keyword index over symbol observations.
// It supports incremental Add calls and bulk Build from a []core.SymbolNode slice.
// All exported methods are concurrency-safe.
package lookup

import (
	"sort"
	"strings"
	"sync"
	"unicode"

	"github.com/geoffmilleraz/fathom/internal/core"
)

// Index is a concurrency-safe inverted index mapping lowercase keyword tokens
// to sorted slices of symbol IDs.
type Index struct {
	mu    sync.RWMutex
	index map[string][]string // term → sorted, deduplicated ids
}

// New returns an empty, ready-to-use Index.
func New() *Index {
	return &Index{index: make(map[string][]string)}
}

// tokenise splits text on non-letter/non-digit runes and lower-cases each part.
func tokenise(text string) []string {
	parts := strings.FieldsFunc(text, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if t := strings.ToLower(p); t != "" {
			out = append(out, t)
		}
	}
	return out
}

// Add indexes id under each of the given terms (case-insensitive).
// Duplicate (term, id) pairs are silently ignored.
func (idx *Index) Add(id string, terms ...string) {
	idx.mu.Lock()
	defer idx.mu.Unlock()
	for _, raw := range terms {
		term := strings.ToLower(raw)
		if term == "" {
			continue
		}
		ids := idx.index[term]
		// Insert maintaining sorted order, deduplicating.
		pos := sort.SearchStrings(ids, id)
		if pos < len(ids) && ids[pos] == id {
			continue // already present
		}
		ids = append(ids, "")
		copy(ids[pos+1:], ids[pos:])
		ids[pos] = id
		idx.index[term] = ids
	}
}

// Query returns a sorted slice of IDs whose index contains term (case-insensitive).
// Returns nil (not an empty slice) when no match is found.
func (idx *Index) Query(term string) []string {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	ids, ok := idx.index[strings.ToLower(term)]
	if !ok || len(ids) == 0 {
		return nil
	}
	// Return a defensive copy so callers cannot mutate the index.
	out := make([]string, len(ids))
	copy(out, ids)
	return out
}

// Build replaces the entire index with a fresh index built from symbols.
// Each symbol is indexed by the tokens derived from its Name field.
// The symbol ID is "<Service>/<Package>.<Name>" when Package is non-empty,
// or "<Service>/<Name>" otherwise.
func (idx *Index) Build(symbols []core.SymbolNode) {
	fresh := make(map[string][]string)

	for _, s := range symbols {
		id := symbolID(s)
		for _, tok := range tokenise(s.Name) {
			insertSorted(fresh, tok, id)
		}
	}

	idx.mu.Lock()
	idx.index = fresh
	idx.mu.Unlock()
}

// symbolID derives a stable string key for a SymbolNode.
func symbolID(s core.SymbolNode) string {
	if s.Package != "" {
		return s.Service + "/" + s.Package + "." + s.Name
	}
	return s.Service + "/" + s.Name
}

// insertSorted adds id into fresh[term] maintaining sorted, deduplicated order.
func insertSorted(m map[string][]string, term, id string) {
	ids := m[term]
	pos := sort.SearchStrings(ids, id)
	if pos < len(ids) && ids[pos] == id {
		return
	}
	ids = append(ids, "")
	copy(ids[pos+1:], ids[pos:])
	ids[pos] = id
	m[term] = ids
}
