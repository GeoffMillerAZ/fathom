package memory

// purge.go — the memories side of phase V's binding purge enumeration
// (AM-12, docs/design/scope-and-purge.md §3.2): a memory's free-form claim
// text may QUOTE the purged content, so a purge that leaves the memory has
// not purged. cmd/fathom's purge wires a FileStore here through
// store.MemoryPurger when the service's repo root is known; when it is
// not, the purge report names the absence rather than implying coverage.

import "strings"

// PathMatcher mirrors store.PathMatcher structurally (Match(path) bool) so
// this package needs no store import.
type PathMatcher interface {
	Match(path string) bool
}

// PurgeMatching deletes every memory carrying a file or path anchor whose
// ref matches m, and returns how many memories were removed. File anchors
// are matched on their repo-relative ref; path anchors (globs) are matched
// on the glob text itself — conservative in the right direction for a
// purge, since a `secrets/**`-anchored memory almost certainly discusses
// content under secrets/. Symbol, commit and claim anchors carry no path
// and never match. A missing file removes nothing and is not an error.
//
// The rewrite is atomic (temp file + rename, the store's own save path),
// so a crash mid-purge leaves the previous file intact rather than a
// truncated one.
func (s *FileStore) PurgeMatching(m PathMatcher) (int, error) {
	if m == nil {
		return 0, nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	all, err := s.load()
	if err != nil {
		return 0, err
	}
	kept := all[:0]
	removed := 0
	for _, mem := range all {
		if memoryMatchesPath(mem, m) {
			removed++
			continue
		}
		kept = append(kept, mem)
	}
	if removed == 0 {
		return 0, nil
	}
	if err := s.save(kept); err != nil {
		return 0, err
	}
	return removed, nil
}

// memoryMatchesPath reports whether any of mem's path-bearing anchors is
// inside the purge set.
func memoryMatchesPath(mem Memory, m PathMatcher) bool {
	for _, a := range mem.Anchors {
		if a.Kind != AnchorFile && a.Kind != AnchorPath {
			continue
		}
		ref := strings.TrimPrefix(a.Ref, "./")
		if ref == "" {
			continue
		}
		if m.Match(ref) {
			return true
		}
	}
	return false
}
