// Package resultaggregator merges results from parallel workers into a single
// deterministic Combined output. It is a pure aggregation function: it does
// NOT spawn or manage workers.
//
// Plan-only status (D7): this package ships as a planner primitive. The worker
// executor (kits/worker-pool with agent-harness as the worker unit) is not yet
// built; it will be added when a real fan-out workload subscribes.
//
// Aggregate deduplicates by key (last-writer wins on equal keys when results
// are presented in order) and sorts the output by Key for reproducibility.
//
// Concurrency contract: Aggregate is a pure function with no shared mutable
// state; it is safe to call concurrently from multiple goroutines.
package resultaggregator

// Result is a single worker result keyed by a stable identifier.
type Result struct {
	// Key is the stable, unique identifier for this result used for dedup.
	Key string

	// Value is the string payload produced by a worker.
	Value string
}

// Combined is the merged, deduplicated, sorted output of an aggregation run.
type Combined struct {
	// Items contains the deduplicated results, sorted lexicographically by Key.
	Items []Result

	// Count is len(Items) — provided for convenient schema validation.
	Count int
}

// Aggregate merges the provided parts into a single Combined result.
//
// Deduplication strategy: when multiple Results share the same Key, the last
// occurrence in parts wins (later workers override earlier ones).
// Output is sorted lexicographically by Key for determinism.
// An empty or nil parts slice returns an empty Combined with Count==0.
func Aggregate(parts []Result) Combined {
	if len(parts) == 0 {
		return Combined{Items: []Result{}, Count: 0}
	}

	// Dedup: last occurrence wins. Track insertion order for stable merging.
	seen := make(map[string]Result, len(parts))
	order := make([]string, 0, len(parts))
	for _, r := range parts {
		if _, exists := seen[r.Key]; !exists {
			order = append(order, r.Key)
		}
		seen[r.Key] = r // last writer wins
	}

	// Collect unique keys and sort them.
	keys := make([]string, len(order))
	copy(keys, order)
	sortStrings(keys)

	items := make([]Result, 0, len(keys))
	for _, k := range keys {
		items = append(items, seen[k])
	}

	return Combined{Items: items, Count: len(items)}
}

// sortStrings sorts a slice of strings in place (insertion sort).
func sortStrings(ss []string) {
	for i := 1; i < len(ss); i++ {
		for j := i; j > 0 && ss[j] < ss[j-1]; j-- {
			ss[j], ss[j-1] = ss[j-1], ss[j]
		}
	}
}
