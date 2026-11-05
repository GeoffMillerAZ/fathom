// Package iacfacts reads IaC provider contracts from catalogs an operator placed on disk and joins them to the
// repository at its locked version (ADR-091, docs/design/iac-facts.md).
//
// I56: nothing in this package dials, runs a provider, `init`, a plan or a registry client. A catalog is a local,
// read-only file. TestIaCFactsNeverDials fails on an import or a dial that breaks this.
package iacfacts

import (
	"fmt"
	"strings"
)

// CatalogFormat is the catalog format version a manifest carries.
const CatalogFormat = 1

// The amended bounds of docs/design/iac-facts.md section 9 (IF-M0 A1). Cap + 1 refuses.
const (
	// MaxCatalogBytes is the largest catalog file.
	MaxCatalogBytes int64 = 256 << 20
	// MaxCatalogEntries is the most zip entries in one catalog.
	MaxCatalogEntries = 500_000
	// MaxManifestBytes bounds manifest.json. A manifest holds a version list and one digest per release, so a real one
	// is tens of KB; 1 MiB keeps one lookup, which decodes the manifest whole, under I48's 8 MiB.
	MaxManifestBytes int64 = 1 << 20
	// MaxIndexBytes bounds one version index.
	MaxIndexBytes int64 = 4 << 20
	// MaxBlobBytes bounds one pruned resource schema (the largest AWS type is 2.43 MiB, IF-M0).
	MaxBlobBytes int64 = 4 << 20
	// MaxJSONDepth bounds container nesting in any catalog JSON.
	MaxJSONDepth = 64
	// MaxFactBytes is I48's point-read ceiling: one fact holds under it, or that one answer is refused.
	MaxFactBytes int64 = 8 << 20
)

// IaCLimits are the bounds a read enforces. DefaultIaCLimits carries the constants above, and tests lower them to
// reach cap + 1 cheaply.
type IaCLimits struct {
	CatalogBytes  int64
	Entries       int
	ManifestBytes int64
	IndexBytes    int64
	BlobBytes     int64
	Depth         int
	FactBytes     int64
}

// DefaultIaCLimits returns the documented bounds.
func DefaultIaCLimits() IaCLimits {
	return IaCLimits{
		CatalogBytes: MaxCatalogBytes, Entries: MaxCatalogEntries, ManifestBytes: MaxManifestBytes,
		IndexBytes: MaxIndexBytes, BlobBytes: MaxBlobBytes, Depth: MaxJSONDepth, FactBytes: MaxFactBytes,
	}
}

// Catalog kinds.
const (
	// KindTerraformProvider is a Terraform or OpenTofu provider catalog: one provider, one major line.
	KindTerraformProvider = "terraform-provider"
)

// ReplacementAbsent marks a catalog that carries no replacement facts. The Go source scan that adds them is
// IF-13, so every catalog built before it says so explicitly rather than by omission.
const ReplacementAbsent = "absent"

// IaCCatalogManifest is manifest.json. The file name and the manifest both say the provider's major line.
type IaCCatalogManifest struct {
	Format int    `json:"format"`
	Kind   string `json:"kind"`
	// Source is the provider address the build was exported under, host included.
	Source string `json:"source"`
	// Line is the provider's major version: one catalog holds every release of one line.
	Line     string   `json:"line"`
	Versions []string `json:"versions"`
	Tool     string   `json:"tool,omitempty"`
	// SchemaFormat is the export's format_version.
	SchemaFormat string `json:"schema_format,omitempty"`
	// Inputs maps a version to the sha256 of the export it was built from.
	Inputs      map[string]string `json:"inputs"`
	Licence     string            `json:"licence,omitempty"`
	Replacement string            `json:"replacement"`
}

// Refusal codes. The first three refuse the whole catalog wherever they fire, the rest refuse the whole catalog at
// `add` and one answer at query time.
const (
	RefuseCatalogBytes  = "catalog-bytes"
	RefuseEntries       = "entries"
	RefuseManifestBytes = "manifest-bytes"
	RefuseIndexBytes    = "index-bytes"
	RefuseBlobBytes     = "blob-bytes"
	RefuseDepth         = "depth"
	RefuseFactBytes     = "fact-bytes"
	RefuseCorrupt       = "corrupt"
	RefuseFormat        = "format"
	RefuseNotFound      = "not-found"
	// RefuseBindingBound refuses one binding answer a read bound could make wrong.
	RefuseBindingBound = "binding-bound"
)

// Refusal scopes.
const (
	// ScopeCatalog is a refusal of the whole input.
	ScopeCatalog = "catalog"
	// ScopeAnswer is a refusal of one answer, with a reason.
	ScopeAnswer = "answer"
)

// IaCRefusal is why a catalog or one answer was refused. It carries a reason a caller can print.
type IaCRefusal struct {
	Scope  string
	Code   string
	Reason string
}

func (e *IaCRefusal) Error() string {
	return fmt.Sprintf("iac catalog: %s refused (%s): %s", e.Scope, e.Code, e.Reason)
}

func refuse(code, format string, args ...any) *IaCRefusal {
	scope := ScopeAnswer
	switch code {
	case RefuseCatalogBytes, RefuseEntries, RefuseManifestBytes, RefuseCorrupt, RefuseFormat:
		scope = ScopeCatalog
	}
	return &IaCRefusal{Scope: scope, Code: code, Reason: fmt.Sprintf(format, args...)}
}

// IaCCatalogRequest names one fact: a version, a type and a JSON path inside that type's pruned schema.
type IaCCatalogRequest struct {
	Version string
	// Kind is one of the Section* names; SectionProvider needs no Type.
	Kind string
	Type string
	// Path walks object keys (and decimal array indexes) inside the blob. Empty returns the whole blob.
	Path []string
}

// Sections of a version index.
const (
	SectionResource  = "resource"
	SectionData      = "data_source"
	SectionEphemeral = "ephemeral_resource"
	SectionProvider  = "provider"
)

// indexKey maps a request kind to its version index key.
func indexKey(kind string) (string, bool) {
	switch kind {
	case SectionResource:
		return "resources", true
	case SectionData:
		return "data_sources", true
	case SectionEphemeral:
		return "ephemeral_resources", true
	case SectionProvider:
		return "provider", true
	}
	return "", false
}

// IaCCatalogFact is one answer. Every fact cites its catalog, version, blob digest and the path it came from.
type IaCCatalogFact struct {
	Catalog string
	Source  string
	Version string
	Kind    string
	Type    string
	// Digest is the sha256 of the blob the fact was read from.
	Digest string
	// Path is the JSON path read, dot-joined; empty is the whole blob.
	Path  string
	Value []byte
}

func joinDots(p []string) string { return strings.Join(p, ".") }
