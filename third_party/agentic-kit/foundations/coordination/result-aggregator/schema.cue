// Package resultaggregator defines the CUE contract for result aggregation records.
// These constraints mirror the Go types and are evaluated with `cue vet ./...`.
package resultaggregator

// #Result is a single worker result keyed by a stable identifier.
#Result: {
	// Key is the stable, non-empty identifier for this result (used for dedup).
	Key: string & !=""

	// Value is the string payload produced by a worker.
	Value: string
}

// #Combined is the merged output of an aggregation run.
#Combined: {
	// Items contains the deduplicated, sorted results.
	// All Keys are unique within Items.
	Items: [...#Result]

	// Count is the number of items after dedup.
	Count: >=0 & int
}
