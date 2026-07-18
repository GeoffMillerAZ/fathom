// Package workspacematerializer executes ParquetQueryPacks and produces
// content-addressed MaterializationReceipts.
//
// The only external dependency is the standard library; in-memory execution
// is delegated to the workspace-query-builder Execute helper so that the
// materializer remains a pure, deterministic transform.
package workspacematerializer

import "time"

// MaterializationReceipt is the provenance record produced by a successful
// Materialize call.  It captures enough information to reproduce or audit the
// exact workspace that was assembled.
type MaterializationReceipt struct {
	// PackID mirrors ParquetQueryPack.PackID.
	PackID string

	// ReadsetID mirrors ReadsetDescriptor.ReadsetID from the pack.
	ReadsetID string

	// Datasets is the ordered list of dataset aliases present in the readset.
	Datasets []string

	// RowCounts maps each dataset alias to the number of rows supplied for it
	// in the data argument passed to Materialize.
	RowCounts map[string]int

	// Hash is the first 16 hex characters of the SHA-256 hash of the emitted
	// SQL string.  Two receipts with the same Hash were produced from
	// structurally identical packs.
	Hash string

	// At is the instant the receipt was created.
	At time.Time
}
