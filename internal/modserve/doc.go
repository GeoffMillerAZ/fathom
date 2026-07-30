// Package modserve implements docs/design/platform-modules.md §3.3's
// binding-aware obligations serving: it augments a service's own compiled
// obligations (internal/semantic.ObligationsFor) with rules federated in
// from every platform module the service's own committed
// `.fathom/modules.cue` binds (internal/bindspec), via the module host's
// existing FederatedObligations machinery (internal/modulehost, phase G).
//
// This composition cannot live in internal/semantic itself:
// internal/modulehost already imports internal/semantic (for
// ObligationsFor, reused per module's store), so internal/semantic
// importing internal/modulehost back would cycle. modserve sits ABOVE both
// — it is the pure, independently-testable domain layer (CLAUDE.md's
// "Analyzer: a pure package... deterministic; tested" convention) that
// internal/mcp's obligations tool handler calls; no MCP server needs to be
// spun up to exercise Bind's merge/budget/staleness-note logic.
//
// Doctrine carried over unchanged from the design: repo rules keep
// priority within the shared MaxObligations budget (module rules count
// INSIDE it, never on top); a binding naming a module the host does not
// have mounted produces an explicit ModuleNote, never silence; scopes
// intersect (a binding's own `scopes` never widens a module's own scope
// globs); min_tier floors which evidence tier a binding lets through.
package modserve
