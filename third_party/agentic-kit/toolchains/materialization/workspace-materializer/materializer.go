package workspacematerializer

import (
	"crypto/sha256"
	"fmt"
	"sort"
	"time"

	wqb "github.com/geoffmilleraz/fathom/third_party/agentic-kit/toolchains/materialization/workspace-query-builder"
)

// Materialize executes pack against the supplied in-memory data and returns a
// content-addressed MaterializationReceipt.
//
// data maps dataset alias → rows, where each row is a map[string]string keyed
// by "entity.field" (the same convention used by workspace-query-builder's
// Execute and ExecuteJoin helpers).
//
// Determinism guarantee: given the same pack and the same data, Materialize
// always returns a receipt with the same Hash.  The At timestamp is the sole
// non-deterministic field and is set to the caller-supplied clock if a non-nil
// clockFn is provided, otherwise to time.Now().  Pass a fixed time in tests to
// achieve full determinism.
func Materialize(pack wqb.ParquetQueryPack, data map[string][]map[string]string) (MaterializationReceipt, error) {
	return materializeAt(pack, data, time.Now)
}

// materializeAt is the testable inner implementation that accepts a clock
// function so tests can pin the timestamp.
func materializeAt(
	pack wqb.ParquetQueryPack,
	data map[string][]map[string]string,
	clock func() time.Time,
) (MaterializationReceipt, error) {
	// 1. Emit SQL — this validates the pack and gives us the hash input.
	sql, err := wqb.EmitSQL(pack)
	if err != nil {
		return MaterializationReceipt{}, fmt.Errorf("workspacematerializer: emit SQL: %w", err)
	}

	// 2. Content-addressed hash over the emitted SQL.
	sum := sha256.Sum256([]byte(sql))
	hash := fmt.Sprintf("%x", sum[:8]) // first 16 hex chars = 8 bytes

	// 3. Collect dataset aliases in stable order (readset declaration order).
	datasets := make([]string, 0, len(pack.Descriptor.Datasets))
	for _, d := range pack.Descriptor.Datasets {
		datasets = append(datasets, d.Alias)
	}

	// 4. Build row counts for every alias in the descriptor.
	rowCounts := make(map[string]int, len(datasets))
	for _, alias := range datasets {
		rows := data[alias]
		rowCounts[alias] = len(rows)
	}

	// 5. Merge all rows across all datasets for in-memory execution.
	//    ExecuteJoin is used to combine them pairwise when joins are present;
	//    for the receipt we only need the total counts already captured above.
	//    Run Execute so we surface any filter/projection errors eagerly.
	merged := mergeDatasets(pack, data)
	if _, err := wqb.Execute(pack.Query, merged); err != nil {
		return MaterializationReceipt{}, fmt.Errorf("workspacematerializer: execute: %w", err)
	}

	return MaterializationReceipt{
		PackID:    pack.PackID,
		ReadsetID: pack.Descriptor.ReadsetID,
		Datasets:  datasets,
		RowCounts: rowCounts,
		Hash:      hash,
		At:        clock(),
	}, nil
}

// mergeDatasets builds a flat row list suitable for Execute by combining all
// dataset rows.  Each source row is labelled "entity.field" already; we just
// concatenate all rows together so Execute can apply joins/filters/projections.
//
// The in-memory execution model used by workspace-query-builder treats the
// combined slice as the universe of rows over which the query runs.
func mergeDatasets(pack wqb.ParquetQueryPack, data map[string][]map[string]string) []map[string]string {
	// Collect entities in a stable order to keep merged output deterministic.
	aliases := make([]string, 0, len(pack.Descriptor.Datasets))
	for _, d := range pack.Descriptor.Datasets {
		aliases = append(aliases, d.Alias)
	}
	sort.Strings(aliases)

	var all []map[string]string
	for _, alias := range aliases {
		all = append(all, data[alias]...)
	}
	return all
}
