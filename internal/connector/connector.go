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
	".rs":   core.KindRustFn,
	".cs":   core.KindCSharpClass,

	// C
	".c": core.KindCFunc,
	".h": core.KindCFunc,

	// C++
	".cc":  core.KindCppFunc,
	".cpp": core.KindCppFunc,
	".cxx": core.KindCppFunc,
	".hpp": core.KindCppFunc,
	".hh":  core.KindCppFunc,

	// Templates
	".templ": core.KindTemplComponent,

	// Helm
	"Chart.yaml":  core.KindHelmChart,
	"values.yaml": core.KindHelmValue,
	".tpl":        core.KindHelmTemplate,

	// Docker
	"Dockerfile":          core.KindDockerStage,
	".dockerfile":         core.KindDockerStage,
	"docker-compose.yml":  core.KindComposeService,
	"docker-compose.yaml": core.KindComposeService,
	"compose.yml":         core.KindComposeService,
	"compose.yaml":        core.KindComposeService,

	// Protocol Buffers
	".proto": core.KindProtoMessage,

	// GraphQL
	".graphql": core.KindGraphQLType,
	".gql":     core.KindGraphQLType,

	// TOML
	".toml": core.KindTOMLTable,

	// SQL (DDL)
	".sql": core.KindSQLTable,

	// JSON
	".json": core.KindJSONKey,

	// Scala
	".scala": core.KindScalaClass,
	".sc":    core.KindScalaClass,

	// PHP
	".php": core.KindPhpClass,

	// Bash
	".sh":   core.KindBashFunction,
	".bash": core.KindBashFunction,

	// Lua
	".lua": core.KindLuaFunction,
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
