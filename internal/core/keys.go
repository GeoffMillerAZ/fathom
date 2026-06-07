package core

import "strings"

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

// Valid reports whether t is a recognized evidence tier.
func (t EvidenceTier) Valid() bool {
	switch t {
	case TierVerified, TierPreScan, TierDocClaimed, TierSMEStated, TierAssumed:
		return true
	}
	return false
}
