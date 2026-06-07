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

	"github.com/geoffmilleraz/sextant/internal/core"
)

// Connector is the source-provider SPI. Implementations must be safe for
// concurrent use by multiple goroutines.
//
// Discover returns a deterministically ordered (sorted by Path) slice of
// SourceRefs for all files the connector considers ingestible under svc.
// The Path field on each SourceRef is relative to the connector's root for
// that service (repo-relative for remote connectors; absolute for local).
//
// Fetch returns the raw bytes of the file identified by ref. It is valid to
// call Fetch with any SourceRef returned by Discover.
type Connector interface {
	Discover(ctx context.Context, svc core.ServiceRef) ([]core.SourceRef, error)
	Fetch(ctx context.Context, ref core.SourceRef) ([]byte, error)
}

// extKindMap maps file extensions (or full basenames) to the SourceKind bucket
// used when the Connector cannot perform deeper AST analysis. Extractors refine
// this classification during the fusion pipeline.
//
// Extensions not listed here receive an empty SourceKind ("").
var extKindMap = map[string]core.SourceKind{
	// Go
	".go":    core.KindGoFunc,
	"go.mod": core.KindModuleDep,

	// Infrastructure / config
	".tf":   core.KindTerraformResource,
	".cue":  core.KindCUEDefinition,
	".yaml": core.KindK8sWorkload,
	".yml":  core.KindK8sWorkload,

	// Documentation
	".md": core.KindDocChunk,

	// Application languages
	".java": core.KindJavaClass,
	".py":   core.KindPyFunc,
	".rb":   core.KindRubyMethod,
	".js":   core.KindJSFunc,
	".jsx":  core.KindJSFunc,
	".ts":   core.KindJSFunc,
	".tsx":  core.KindJSFunc,
}

// kindFor returns the SourceKind for a file identified by basename and
// extension. It checks the basename first (to handle special files like
// "go.mod"), then falls back to the extension.
// ext must include the leading dot (e.g. ".go").
func kindFor(ext string) core.SourceKind {
	return extKindMap[ext]
}

// kindForFile returns the SourceKind for a file given its full basename and
// extension. The basename check (e.g. "go.mod") takes precedence over the
// extension check.
func kindForFile(base, ext string) core.SourceKind {
	if k, ok := extKindMap[base]; ok {
		return k
	}
	return extKindMap[ext]
}

// skipDir reports whether a directory entry name should be skipped during
// tree walking. The list mirrors the DefaultIgnorePatterns described in the
// fusion pipeline design.
func skipDir(name string) bool {
	switch name {
	case "vendor", ".git", "testdata", "generated", "node_modules", ".idea", ".vscode":
		return true
	}
	return false
}
