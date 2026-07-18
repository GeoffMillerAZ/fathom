// Package sourcelinker maintains explicit links between evidence spans and the
// sources they came from. Both directions of the mapping are stored so callers
// can ask either "which sources contributed this span?" or "which spans came
// from this source?".
//
// All returned slices are sorted in lexicographic order for determinism.
//
// Concurrency contract: Linker is not safe for concurrent use; callers that
// share a Linker across goroutines must guard access with their own
// synchronization.
package sourcelinker

// Linker stores span↔source associations.
// The zero value is ready to use.
type Linker struct {
	// spanToSources maps spanID → set of sourceIDs.
	spanToSources map[string]map[string]struct{}
	// sourceToSpans maps sourceID → set of spanIDs.
	sourceToSpans map[string]map[string]struct{}
}

// Link records that the span identified by spanID is associated with the
// source identified by sourceID. Calling Link with the same pair twice is
// idempotent.
func (l *Linker) Link(spanID, sourceID string) {
	if l.spanToSources == nil {
		l.spanToSources = make(map[string]map[string]struct{})
		l.sourceToSpans = make(map[string]map[string]struct{})
	}
	if l.spanToSources[spanID] == nil {
		l.spanToSources[spanID] = make(map[string]struct{})
	}
	if l.sourceToSpans[sourceID] == nil {
		l.sourceToSpans[sourceID] = make(map[string]struct{})
	}
	l.spanToSources[spanID][sourceID] = struct{}{}
	l.sourceToSpans[sourceID][spanID] = struct{}{}
}

// SourcesFor returns the sorted list of source IDs associated with spanID.
// Returns nil if the span has no links.
func (l *Linker) SourcesFor(spanID string) []string {
	return sortedKeys(l.spanToSources[spanID])
}

// SpansFor returns the sorted list of span IDs associated with sourceID.
// Returns nil if the source has no links.
func (l *Linker) SpansFor(sourceID string) []string {
	return sortedKeys(l.sourceToSpans[sourceID])
}

// sortedKeys returns the keys of a string set in sorted order.
func sortedKeys(m map[string]struct{}) []string {
	if len(m) == 0 {
		return nil
	}
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sortStrings(out)
	return out
}

// sortStrings sorts a slice of strings in-place.
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
