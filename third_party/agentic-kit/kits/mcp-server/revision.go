package mcpserver

import "time"

// ProtocolRevision is the MCP protocol revision this server implements: the
// initialize handshake, ping, tools/list and tools/call over newline-delimited
// stdio and stateless Streamable HTTP, exactly as 2025-11-25 specifies them.
// It is the newest entry in knownProtocolVersions and the revision offered
// when a client requests one the server does not know. The kit tracks later
// revisions with a clock rather than chasing them — see Deprecations,
// DeprecationsDue and decision 0003
// (docs/intent/decisions/0003-mcp-revisions-track-never-chase.md).
const ProtocolRevision = "2025-11-25"

// Deprecation names one MCP feature that a revision later than
// ProtocolRevision removed or deprecated, the floor before which the spec
// promises not to remove it, and whether this package still implements it.
// The same shape is exported by foundations/agent/mcp-client; the two tables
// list the same features and floors and differ only in StillImplemented. The
// CUE contract is #Deprecation in schema.cue.
type Deprecation struct {
	// Feature is the retired feature, named as the MCP changelog names it.
	Feature string

	// DeprecatedIn is the release date of the revision that retired the
	// feature. MCP revisions are named by their date, so this is also the
	// revision id (2026-07-28 is time.Date(2026, 7, 28, ...)).
	DeprecatedIn time.Time

	// RemovableAfter is the floor: the earliest date on or after which a
	// revision may remove the feature — twelve months after DeprecatedIn
	// under the MCP feature-lifecycle policy. The zero value means the
	// deprecated-features registry publishes no date; DeprecationsDue treats
	// an undated floor on a still-implemented feature as due NOW (fail
	// closed: a liability with no clock is worse than one with a date).
	RemovableAfter time.Time

	// StillImplemented reports whether this package still speaks the
	// feature. Only a still-implemented feature can become due.
	StillImplemented bool
}

// revision20260728 is the release date of the MCP revision that retired the
// features below; every 2026-07-28 floor is twelve months after it.
var revision20260728 = time.Date(2026, time.July, 28, 0, 0, 0, 0, time.UTC)

// revision20250326 is the release date of the MCP revision that deprecated
// the HTTP+SSE transport (SEP-2596). The registry's removal date for it is
// "three months after SEP-2596 reaches Final", undated as of 2026-09-02, so
// its RemovableAfter stays zero.
var revision20250326 = time.Date(2025, time.March, 26, 0, 0, 0, 0, time.UTC)

// deprecationTable is the 2026-07-28 retirement ledger for this server.
// StillImplemented is true only where handlers.go or http.go actually speaks
// the feature: initialize/notifications/initialized and ping. Sessions
// (Mcp-Session-Id), roots, sampling, logging and the HTTP+SSE transport were
// never built here (doc.go, "Protocol scope and roadmap").
func deprecationTable() []Deprecation {
	floor := revision20260728.AddDate(1, 0, 0)
	return []Deprecation{
		{Feature: "initialize handshake", DeprecatedIn: revision20260728, RemovableAfter: floor, StillImplemented: true},       // SEP-2575
		{Feature: "sessions (Mcp-Session-Id)", DeprecatedIn: revision20260728, RemovableAfter: floor, StillImplemented: false}, // SEP-2567
		{Feature: "ping", DeprecatedIn: revision20260728, RemovableAfter: floor, StillImplemented: true},                       // SEP-2575
		{Feature: "roots", DeprecatedIn: revision20260728, RemovableAfter: floor, StillImplemented: false},                     // SEP-2577
		{Feature: "sampling", DeprecatedIn: revision20260728, RemovableAfter: floor, StillImplemented: false},                  // SEP-2577
		{Feature: "logging", DeprecatedIn: revision20260728, RemovableAfter: floor, StillImplemented: false},                   // SEP-2577
		{Feature: "HTTP+SSE transport", DeprecatedIn: revision20250326, StillImplemented: false},                               // SEP-2596, undated floor
	}
}

// Deprecations returns the 2026-07-28 retirement ledger for this server: one
// entry per feature the revision after ProtocolRevision removed or
// deprecated, in a fixed table order, as a fresh slice the caller may keep
// or mutate. It is a statement of fact, not a policy: whether to keep
// implementing a feature past its floor is decision 0003's call.
func Deprecations() []Deprecation {
	return deprecationTable()
}

// DeprecationsDue returns the entries of Deprecations that this server still
// implements and whose floor has been reached at now — on or after
// RemovableAfter, or immediately when RemovableAfter is zero. now is the
// injected clock; the package's own test passes time.Now on purpose so the
// suite goes red on the first floor (doc.go, "Protocol revision clock").
// The result is nil when nothing is due.
func DeprecationsDue(now time.Time) []Deprecation {
	return deprecationsDue(deprecationTable(), now)
}

// deprecationsDue applies the due rule to an explicit table; DeprecationsDue
// is the exported wrapper over the package ledger.
func deprecationsDue(table []Deprecation, now time.Time) []Deprecation {
	var due []Deprecation
	for _, d := range table {
		if !d.StillImplemented {
			continue
		}
		if d.RemovableAfter.IsZero() || !now.Before(d.RemovableAfter) {
			due = append(due, d)
		}
	}
	return due
}
