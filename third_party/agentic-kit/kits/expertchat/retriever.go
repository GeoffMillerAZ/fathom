package expertchat

import "context"

// Retriever fetches candidate evidence for a query from a module host. It is
// the ONLY seam expertchat uses to reach a module host — nothing else in
// this package knows how evidence was fetched. Two implementations ship in
// this kit: HTTPRetriever (a real JSON-RPC client, wire.go) and
// FakeRetriever (a deterministic test double, fake.go).
//
// Retrieve returns candidate evidence in any order and performs no
// tier/freshness filtering of its own — Run applies Policy uniformly
// regardless of which Retriever produced the items, so a Retriever
// implementation only needs to answer "what evidence matches this query,"
// honestly reporting each item's Tier and Status.
//
// Implementations must be safe for concurrent use.
type Retriever interface {
	// Retrieve returns up to limit candidate EvidenceItem values matching
	// query. limit <= 0 means "no limit" (return everything the host has). A
	// nil, empty slice is a valid "nothing matched" result, not an error.
	Retrieve(ctx context.Context, query string, limit int) ([]EvidenceItem, error)
}
