// Package sourceregistry provides a governed registry for evidence sources.
// Every source family registered here can be referenced by other evidence
// components (evidence-span, source-linker, entity-crosswalk, etc.) via its
// stable ID.
//
// CUE contracts live in schema.cue; this file mirrors those definitions.
//
// Concurrency contract: Registry is not safe for concurrent use; callers that
// share a Registry across goroutines must guard access with their own
// synchronization.
package sourceregistry

import "fmt"

// Kind enumerates the recognised classes of evidence source.
type Kind string

const (
	// KindMarkdown covers Markdown documents (.md files, wiki pages, etc.).
	KindMarkdown Kind = "markdown"
	// KindReadme covers README files specifically.
	KindReadme Kind = "readme"
	// KindDoc covers formal documentation portals (Confluence, Notion, etc.).
	KindDoc Kind = "doc"
	// KindOther covers any source that does not fit the above categories.
	KindOther Kind = "other"
)

// Source describes a single evidence source registered in the Registry.
//
// CUE mirror: #Source in schema.cue.
type Source struct {
	// ID uniquely identifies this source. Must be non-empty.
	ID string

	// Kind classifies the source family.
	Kind Kind

	// URI is the stable locator for this source (e.g. a file path, URL, or
	// artifact ref). May be empty for in-memory or synthetic sources.
	URI string
}

// Registry is an in-memory store of Source values keyed by ID.
// The zero value is ready to use.
type Registry struct {
	m map[string]Source
}

// Register adds src to the registry. If a source with the same ID already
// exists it is silently overwritten.
func (r *Registry) Register(src Source) error {
	if src.ID == "" {
		return fmt.Errorf("sourceregistry: source ID must not be empty")
	}
	if r.m == nil {
		r.m = make(map[string]Source)
	}
	r.m[src.ID] = src
	return nil
}

// Get returns the source with the given id and true, or the zero Source and
// false when not found.
func (r *Registry) Get(id string) (Source, bool) {
	if r.m == nil {
		return Source{}, false
	}
	s, ok := r.m[id]
	return s, ok
}

// List returns all registered sources in deterministic (lexicographic by ID)
// order.
func (r *Registry) List() []Source {
	if len(r.m) == 0 {
		return nil
	}
	// Collect IDs, sort, then build output.
	ids := make([]string, 0, len(r.m))
	for id := range r.m {
		ids = append(ids, id)
	}
	sortStrings(ids)
	out := make([]Source, len(ids))
	for i, id := range ids {
		out[i] = r.m[id]
	}
	return out
}

// sortStrings sorts a slice of strings in-place (insertion sort for small
// slices; avoids importing "sort" for this simple utility).
func sortStrings(ss []string) {
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
