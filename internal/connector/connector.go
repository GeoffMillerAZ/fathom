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
	"path/filepath"
	"sort"

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

// classification pairs the SourceKind bucket used when the Connector cannot
// perform deeper AST analysis with the canonical presentation-layer language
// tag for the same extension/basename key. Kind and Lang are always derived
// from the SAME registry entry (ADR-021 §1 / LT-1): there is exactly one
// ext→{kind, language} table in the whole codebase, and fusion.New's
// extractor registration is tested for exact key-parity against it (see
// internal/fusion's language-registry parity test), so a new extractor
// literally cannot be added without also declaring its language here (and
// vice versa).
type classification struct {
	Kind core.SourceKind
	Lang string
}

// extClassMap maps file extensions (or full basenames) to their
// classification. Keys not listed here have no registered extractor;
// LanguageForPath reports core.LangUnclassified for them rather than
// silently omitting them.
var extClassMap = map[string]classification{
	// Go
	".go":    {core.KindGoFunc, "go"},
	"go.mod": {core.KindModuleDep, "go"},

	// Infrastructure / config
	".tf":   {core.KindTerraformResource, "terraform"},
	".cue":  {core.KindCUEDefinition, "cue"},
	".yaml": {core.KindK8sWorkload, "yaml"},
	".yml":  {core.KindK8sWorkload, "yaml"},

	// Documentation
	".md": {core.KindDocChunk, "md"},

	// Application languages
	".java": {core.KindJavaClass, "java"},
	".py":   {core.KindPyFunc, "py"},
	".rb":   {core.KindRubyMethod, "ruby"},
	".js":   {core.KindJSFunc, "js"},
	".jsx":  {core.KindJSFunc, "js"},
	".ts":   {core.KindJSFunc, "ts"},
	".tsx":  {core.KindJSFunc, "ts"},
	".rs":   {core.KindRustFn, "rust"},
	".cs":   {core.KindCSharpClass, "cs"},

	// C
	".c": {core.KindCFunc, "c"},
	".h": {core.KindCFunc, "c"},

	// C++
	".cc":  {core.KindCppFunc, "cpp"},
	".cpp": {core.KindCppFunc, "cpp"},
	".cxx": {core.KindCppFunc, "cpp"},
	".hpp": {core.KindCppFunc, "cpp"},
	".hh":  {core.KindCppFunc, "cpp"},

	// Templates
	".templ": {core.KindTemplComponent, "templ"},

	// Helm (YAML-family syntax; distinguished at the Kind level, not Lang)
	"Chart.yaml":  {core.KindHelmChart, "yaml"},
	"values.yaml": {core.KindHelmValue, "yaml"},
	".tpl":        {core.KindHelmTemplate, "yaml"},

	// Docker
	"Dockerfile":          {core.KindDockerStage, "dockerfile"},
	".dockerfile":         {core.KindDockerStage, "dockerfile"},
	"docker-compose.yml":  {core.KindComposeService, "yaml"},
	"docker-compose.yaml": {core.KindComposeService, "yaml"},
	"compose.yml":         {core.KindComposeService, "yaml"},
	"compose.yaml":        {core.KindComposeService, "yaml"},

	// Protocol Buffers
	".proto": {core.KindProtoMessage, "proto"},

	// GraphQL
	".graphql": {core.KindGraphQLType, "graphql"},
	".gql":     {core.KindGraphQLType, "graphql"},

	// TOML
	".toml": {core.KindTOMLTable, "toml"},

	// SQL (DDL)
	".sql": {core.KindSQLTable, "sql"},

	// JSON
	".json": {core.KindJSONKey, "json"},

	// Scala
	".scala": {core.KindScalaClass, "scala"},
	".sc":    {core.KindScalaClass, "scala"},

	// PHP
	".php": {core.KindPhpClass, "php"},

	// Bash
	".sh":   {core.KindBashFunction, "sh"},
	".bash": {core.KindBashFunction, "sh"},

	// Lua
	".lua": {core.KindLuaFunction, "lua"},
}

// classify looks up base (full basename, e.g. "go.mod", "Dockerfile") first —
// so special basenames take precedence — then falls back to ext (leading-dot
// extension, e.g. ".go").
func classify(base, ext string) (classification, bool) {
	if c, ok := extClassMap[base]; ok {
		return c, true
	}
	c, ok := extClassMap[ext]
	return c, ok
}

// kindForFile returns the SourceKind for a file given its full basename and
// extension. The basename check (e.g. "go.mod") takes precedence over the
// extension check.
func kindForFile(base, ext string) core.SourceKind {
	c, _ := classify(base, ext)
	return c.Kind
}

// LanguageForPath returns the canonical language tag for path, derived from
// the exact same registry that assigns SourceKind (ADR-021 §1 / LT-1): one
// source of truth for "what language is this file", consumed by both the
// fusion pipeline's Kind assignment and presentation-layer callers like
// Client.Sources. Returns core.LangUnclassified when no extractor is
// registered for path's basename or extension — never a silent "".
//
// LanguageForPath does not inspect file content; a caller that also wants
// binary detection (content-based, overrides any extension-derived language)
// should additionally check IsBinary — see Client.Sources.
func LanguageForPath(path string) string {
	base := filepath.Base(path)
	ext := filepath.Ext(path)
	if c, ok := classify(base, ext); ok {
		return c.Lang
	}
	return core.LangUnclassified
}

// RegisteredExtensions returns the sorted set of extension/basename keys that
// have a language classification. It exists so the fusion package can assert,
// in a test, that its extractor registry (fusion.New's p.exts) matches this
// set exactly — the bijection that keeps "one registry" honest as extractors
// are added.
func RegisteredExtensions() []string {
	keys := make([]string, 0, len(extClassMap))
	for k := range extClassMap {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
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
