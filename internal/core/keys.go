package core

import (
	"fmt"
	"path"
	"path/filepath"
	"strings"
)

// ValidServiceID reports whether id is a well-formed logical service identifier.
// Service IDs are logical NAMES, not paths. A path-like ID — e.g. "../app-kit"
// from `ingest --service ../app-kit` — pollutes the fleet with duplicate,
// confusingly-named services (an agent can't tell which to query). Reject
// path separators, traversal, the empty string, and surrounding whitespace.
func ValidServiceID(id string) error {
	if id == "" {
		return fmt.Errorf("service ID is empty")
	}
	if strings.TrimSpace(id) != id {
		return fmt.Errorf("service ID %q has leading or trailing whitespace", id)
	}
	if strings.ContainsAny(id, `/\`) {
		return fmt.Errorf("service ID %q looks like a path (contains a slash); use a logical name like %q", id, filepath.Base(filepath.Clean(id)))
	}
	if strings.Contains(id, "..") {
		return fmt.Errorf("service ID %q contains %q; use a logical name", id, "..")
	}
	return nil
}

// SymbolKey is the canonical, stable identifier for a symbol used in call edges
// and cross-references: "service::package.Name". Deterministic for a given input.
func SymbolKey(service, pkg, name string) string {
	var b strings.Builder
	b.WriteString(service)
	b.WriteString("::")
	if pkg != "" {
		b.WriteString(pkg)
		b.WriteString(".")
	}
	b.WriteString(name)
	return b.String()
}

// QualifiedPackage returns the package qualifier a symbol's key carries:
// the symbol's directory (path.Dir of its repo-relative, forward-slash file
// path) joined with its declared package — "cmd/a/main" for a `package main`
// under cmd/a. Two `package main` binaries, or two same-named packages in
// different directories, therefore never share a qualifier (PC-32,
// docs/plans/proof-credibility.md; O21: before this, every `func run` in
// every package main collapsed to one key and callgraph.Build kept whichever
// was ingested last). A root-level or path-less symbol keeps the bare
// package name, and a symbol with no package keeps the empty qualifier —
// their keys are unchanged from the historical "service::package.Name" form.
func QualifiedPackage(filePath, pkg string) string {
	if pkg == "" || filePath == "" {
		return pkg
	}
	dir := path.Dir(filePath)
	if dir == "." || dir == "/" || dir == "" {
		return pkg
	}
	return strings.TrimPrefix(dir, "/") + "/" + pkg
}

// PackagePath returns the node's directory-qualified package — the qualifier
// its Key carries (see QualifiedPackage).
func (n SymbolNode) PackagePath() string { return QualifiedPackage(n.FilePath, n.Package) }

// Key returns the canonical SymbolKey for this node:
// "service::<dir>/<package>.Name" for a symbol under a directory,
// "service::package.Name" otherwise (see QualifiedPackage).
func (n SymbolNode) Key() string { return SymbolKey(n.Service, n.PackagePath(), n.Name) }

// ToSymbolNode reconstructs a SymbolNode from a stored Observation — the inverse
// of the SymbolNode→Observation mapping the fusion pipeline performs at ingest.
// It is the single source of truth for that reconstruction, used by both the
// client's list/query paths and the cross-repo platform builder. The node's
// Signature is recovered from the Observation's Claim.
func (o Observation) ToSymbolNode() SymbolNode {
	return SymbolNode{
		Service:    o.ServiceID,
		Package:    o.Package,
		Name:       o.Symbol,
		Kind:       o.SourceKind,
		FilePath:   o.SourceRef,
		Line:       o.Line,
		EndLine:    o.EndLine,
		Signature:  o.Claim,
		Calls:      o.Calls,
		Tier:       o.EvidenceTier,
		Complexity: o.Complexity,
	}
}

// Valid reports whether t is a recognized evidence tier.
func (t EvidenceTier) Valid() bool {
	switch t {
	case TierVerified, TierPreScan, TierDocClaimed, TierSMEStated, TierAssumed:
		return true
	}
	return false
}
