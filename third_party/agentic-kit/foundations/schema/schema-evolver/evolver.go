package schemaevolver

import (
	"fmt"
	"sort"
)

// Evolver maintains a registry of migration steps and can compute the
// shortest migration path between any two registered versions.
//
// The zero value is ready to use.
type Evolver struct {
	// migrations is the ordered list of registered steps, keyed by source version.
	migrations []Migration
}

// versionKey produces a comparable map key from a Version.
type versionKey struct{ major, minor int }

func key(v Version) versionKey { return versionKey{v.Major, v.Minor} }

// Register adds a migration step to the evolver. The Description field must be
// non-empty. Duplicate (From, To) pairs are silently de-duplicated; if the same
// pair is registered twice the first registration is kept.
func (e *Evolver) Register(m Migration) error {
	if m.Description == "" {
		return fmt.Errorf("schemaevolver: Migration.Description must not be empty")
	}
	// De-duplicate: keep the first registration for any given (from, to) pair.
	fromKey := key(m.From)
	toKey := key(m.To)
	for _, existing := range e.migrations {
		if key(existing.From) == fromKey && key(existing.To) == toKey {
			return nil
		}
	}
	e.migrations = append(e.migrations, m)
	return nil
}

// Path computes the shortest ordered sequence of migrations that connects
// from to to using a breadth-first search over the registered migration graph.
//
// The returned slice is deterministically ordered: when multiple next-hops
// exist at the same BFS depth, they are explored in the order they were
// registered, ensuring stable output across identical registrations.
//
// An error is returned when no path exists.
func (e *Evolver) Path(from, to Version) ([]Migration, error) {
	fromKey := key(from)
	toKey := key(to)

	if fromKey == toKey {
		return []Migration{}, nil
	}

	// Build an adjacency map from the registered migrations. Insertion order
	// is preserved so BFS exploration is deterministic.
	adj := make(map[versionKey][]Migration)
	for _, m := range e.migrations {
		k := key(m.From)
		adj[k] = append(adj[k], m)
	}

	// BFS: each queue entry carries the path taken to reach the current node.
	type entry struct {
		version versionKey
		path    []Migration
	}

	visited := map[versionKey]bool{fromKey: true}
	queue := []entry{{version: fromKey, path: nil}}

	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]

		neighbours := adj[cur.version]
		// Sort neighbours by (To.Major, To.Minor) for deterministic output when
		// multiple edges leave the same node.
		sort.Slice(neighbours, func(i, j int) bool {
			ki := key(neighbours[i].To)
			kj := key(neighbours[j].To)
			if ki.major != kj.major {
				return ki.major < kj.major
			}
			return ki.minor < kj.minor
		})

		for _, m := range neighbours {
			nk := key(m.To)
			if visited[nk] {
				continue
			}
			newPath := make([]Migration, len(cur.path)+1)
			copy(newPath, cur.path)
			newPath[len(cur.path)] = m

			if nk == toKey {
				return newPath, nil
			}
			visited[nk] = true
			queue = append(queue, entry{version: nk, path: newPath})
		}
	}

	return nil, fmt.Errorf(
		"schemaevolver: no migration path from %d.%d to %d.%d",
		from.Major, from.Minor, to.Major, to.Minor,
	)
}

// IsBackwardCompatible reports whether every migration step on the path from
// from to to is backward-compatible.
//
// A path is backward-compatible when:
//   - The only version component that changes is Minor (major stays the same),
//     AND the Minor only increases.
//
// If no path exists, IsBackwardCompatible returns false.
func (e *Evolver) IsBackwardCompatible(from, to Version) bool {
	path, err := e.Path(from, to)
	if err != nil {
		return false
	}
	if len(path) == 0 {
		// Same version — trivially compatible.
		return true
	}
	for _, m := range path {
		// A step is breaking if the major version changes or minor decreases.
		if m.To.Major != m.From.Major {
			return false
		}
		if m.To.Minor < m.From.Minor {
			return false
		}
	}
	return true
}
