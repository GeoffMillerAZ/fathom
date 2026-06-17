package core

import (
	"fmt"
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

// Key returns the canonical SymbolKey for this node.
func (n SymbolNode) Key() string { return SymbolKey(n.Service, n.Package, n.Name) }

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
