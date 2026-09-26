// Package hookcache names the well-known local directory the CLI's
// service-identity cache (cmd/fathom/identcache.go) uses, so daemon-side
// code (internal/mcp's auto-refresh, the ingest MCP tool) can bust it
// after a re-ingest without cmd/fathom and internal/mcp needing to import
// each other (cmd/fathom already imports internal/mcp; the reverse would
// be a cycle).
//
// Critic finding F2 (BLOCKING, phase-P fix round): the identity cache
// keyed only on (repo root, daemon base URL) — constant across a daemon
// restart, an index rebuild, a service rename, or a re-ingest — so
// nothing in the key changed when the ANSWER changed, and a stale
// resolution could silently starve obligations injection for up to the
// TTL (the exact O15 failure class: silent injection death). Busting the
// cache at every ingest closes that gap regardless of TTL.
package hookcache

import (
	"os"
	"path/filepath"
)

// IdentDir is os.TempDir()/fathom-ident-cache — the SAME path
// cmd/fathom/identcache.go's identCacheDir defaults to. A package var
// (not a const) so tests can override it.
var IdentDir = filepath.Join(os.TempDir(), "fathom-ident-cache")

// BustIdent removes every cached service-identity resolution. Called
// after any ingest — CLI `fathom ingest`, the MCP `ingest` tool, and
// auto-refresh's own re-ingest — so a stale cache entry can never outlive
// the index change that invalidated it. Best-effort: swallowed on error,
// mirroring every other cache-adjacent write in this program (a failed
// bust just means the next lookup waits out the TTL, never a crash).
func BustIdent() {
	_ = os.RemoveAll(IdentDir)
}

// HealthDir is os.TempDir()/fathom-hook-health — where every hook process
// on the machine shares one daemon-health breaker file per daemon URL
// (internal/hookguard, docs/design/responsible-tenant.md §11.2). The
// per-user temp dir, like IdentDir, so concurrent sessions see the same
// breaker. A package var so tests can redirect it.
var HealthDir = filepath.Join(os.TempDir(), "fathom-hook-health")
