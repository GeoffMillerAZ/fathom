// Package clone detects structural duplicate code (clone groups) across one or
// more services using content-normalised SHA-256 hashing.
//
// Go source files are split into per-function chunks (delimited by "func "
// declarations).  All other file types are split by blank-line blocks.
//
// Two or more chunks that produce the same NormalizedHash form a CloneGroup.
// All results are sorted and deterministic.
package clone

import (
	"fmt"
	"sort"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/cluster"
)

// CloneGroup is a set of file identifiers (or "file:chunkN" chunk keys) that
// share the same structural-clone hash.
type CloneGroup struct {
	Hash  string
	Files []string // sorted, deterministic
}

// Detect finds structural clone groups across files.
//
// files maps a file path (or any stable string identifier) to its source text.
// Go files (path ending in ".go") are split into function-level chunks; other
// files are split by blank-line blocks. Groups of ≥ 2 chunks sharing the same
// NormalizedHash are returned, sorted by Hash then by the first file entry.
func Detect(files map[string]string) []CloneGroup {
	items := chunksToItems(files)
	groups := cluster.Cluster(items)

	var result []CloneGroup
	for _, ids := range groups {
		if len(ids) < 2 {
			continue
		}
		// Look up the hash for this group via any member.
		hash := hashForID(items, ids[0])
		result = append(result, CloneGroup{Hash: hash, Files: ids})
	}

	// Sort groups: primary by Hash, secondary by first file for full determinism.
	sort.Slice(result, func(i, j int) bool {
		if result[i].Hash != result[j].Hash {
			return result[i].Hash < result[j].Hash
		}
		return result[i].Files[0] < result[j].Files[0]
	})
	return result
}

// CrossRepoClones detects clone groups that span multiple services.
//
// byService maps service name → (file path → source text). The returned
// CloneGroup.Files entries are prefixed "service:path[:chunkN]" so callers can
// identify the origin service.
func CrossRepoClones(byService map[string]map[string]string) []CloneGroup {
	// Build a unified file map with service-prefixed keys.
	unified := make(map[string]string)
	for svc, files := range byService {
		for path, src := range files {
			key := svc + ":" + path
			unified[key] = src
		}
	}
	return Detect(unified)
}

// ─── helpers ─────────────────────────────────────────────────────────────────

// chunksToItems splits each file into chunks and returns cluster.Item entries.
func chunksToItems(files map[string]string) []cluster.Item {
	// Deterministic iteration: sort file keys first.
	keys := make([]string, 0, len(files))
	for k := range files {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var items []cluster.Item
	for _, path := range keys {
		src := files[path]
		chunks := splitChunks(path, src)
		for i, chunk := range chunks {
			chunk = strings.TrimSpace(chunk)
			if chunk == "" {
				continue
			}
			id := chunkID(path, i, len(chunks))
			items = append(items, cluster.Item{
				ID:   id,
				Hash: cluster.NormalizedHash(chunk),
			})
		}
	}
	return items
}

// chunkID returns a stable ID for a chunk. Single-chunk files use the path
// directly; multi-chunk files use "path:chunkN".
func chunkID(path string, idx, total int) string {
	if total == 1 {
		return path
	}
	return fmt.Sprintf("%s:chunk%d", path, idx)
}

// splitChunks splits src into logical chunks depending on file type.
// Go files: split on "func " token at the start of a line.
// Others: split by blank lines (one or more consecutive empty lines).
func splitChunks(path, src string) []string {
	if strings.HasSuffix(path, ".go") {
		return splitGoFuncs(src)
	}
	return splitByBlankLines(src)
}

// splitGoFuncs splits Go source into function-level chunks by detecting lines
// that start with "func " (package-level function/method declarations).
// The "func " line is included at the beginning of its chunk.
func splitGoFuncs(src string) []string {
	lines := strings.Split(src, "\n")
	var chunks []string
	var current []string

	for _, line := range lines {
		if strings.HasPrefix(line, "func ") && len(current) > 0 {
			chunks = append(chunks, strings.Join(current, "\n"))
			current = current[:0]
		}
		current = append(current, line)
	}
	if len(current) > 0 {
		chunks = append(chunks, strings.Join(current, "\n"))
	}
	return chunks
}

// splitByBlankLines splits src into blocks separated by one or more blank lines.
func splitByBlankLines(src string) []string {
	lines := strings.Split(src, "\n")
	var blocks []string
	var current []string

	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			if len(current) > 0 {
				blocks = append(blocks, strings.Join(current, "\n"))
				current = current[:0]
			}
		} else {
			current = append(current, line)
		}
	}
	if len(current) > 0 {
		blocks = append(blocks, strings.Join(current, "\n"))
	}
	return blocks
}

// hashForID looks up the hash for a given item ID from the items slice.
func hashForID(items []cluster.Item, id string) string {
	for _, it := range items {
		if it.ID == id {
			return it.Hash
		}
	}
	return ""
}
