package client

import "github.com/geoffmilleraz/fathom/internal/memory"

// MemoryReader exposes the store slice anchored memory's drift index reads
// (docs/design/anchored-memory.md §3.1; internal/memory.StoreIndex). The
// MCP memory_search tool resolves it through an optional backend
// capability, so the daemon evaluates memory state against the SAME hashes
// the `fathom memory` verbs baseline anchors with — a second index over
// different data would fabricate drift (law 3).
func (c *Client) MemoryReader() memory.Reader { return c.st }
