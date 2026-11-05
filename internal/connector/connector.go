// Package connector defines the Connector SPI and its implementations.
//
// The SPI separates source acquisition from the fusion pipeline: any
// Connector implementation can feed the pipeline without changes to
// extraction, storage, or query logic.
//
// Three implementations are provided:
//   - LocalConnector  — reads from a local directory tree (dev / CLI use)
//   - FauxConnector   — reads from the in-repo testdata/fauxtown fixture
//   - (GitHubConnector is a future implementation; not in this package)
package connector

import (
	"context"

	"github.com/geoffmilleraz/fathom/internal/core"
)

// Connector is the source-provider SPI. Implementations must be safe for
// concurrent use by multiple goroutines.
//
// Discover returns a deterministically ordered (sorted by Path) slice of
// SourceRefs for all files the connector considers ingestible under svc.
// The Path field on each SourceRef is relative to the connector's root for
// that service — every connector, including LocalConnector, emits
// repo-relative, slash-separated paths (ADR-017: anchors must be portable;
// an ingested store never records machine-local absolute paths).
//
// Fetch returns the raw bytes of the file identified by ref. It is valid to
// call Fetch with any SourceRef returned by Discover.
//
// Watermark returns an opaque freshness token for the service's corpus:
// equal tokens mean unchanged content. Git-backed corpora return the bare
// HEAD commit SHA; others return a content-addressed manifest hash with the
// "manifest:" prefix. "" means freshness is unknown — an honest state, not
// an error (ADR-017: freshness is domain-neutral, never git-only).
type Connector interface {
	Discover(ctx context.Context, svc core.ServiceRef) ([]core.SourceRef, error)
	Fetch(ctx context.Context, ref core.SourceRef) ([]byte, error)
	Watermark(ctx context.Context, svc core.ServiceRef) (string, error)
}

// skipDir reports whether a directory entry name should be skipped during
// tree walking. The list mirrors the DefaultIgnorePatterns described in the
// fusion pipeline design. .claude is tool state (agent worktrees under
// .claude/worktrees/ are full repo copies), same class as .idea/.vscode —
// indexing it duplicates every symbol in the service.
func skipDir(name string) bool {
	switch name {
	case "vendor", ".git", "testdata", "generated", "node_modules", ".idea", ".vscode", ".claude":
		return true
	}
	return false
}

// IsSkippedDir reports whether a directory NAME is one the connector always
// excludes from Discover walks (vendor, .git, testdata, generated,
// node_modules, .idea, .vscode, .claude — the same set skipDir prunes).
// Exported so corpus-reading consumers (internal/mcp's read_source) can
// honor the exact same exclusions when deciding whether a path is part of
// the indexed corpus, instead of maintaining a second list that drifts.
func IsSkippedDir(name string) bool { return skipDir(name) }
