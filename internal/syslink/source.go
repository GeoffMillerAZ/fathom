package syslink

import (
	"errors"
	"fmt"
	"sort"
	"sync"
)

// Role is which end of a contract a key row stands at (§4).
type Role string

const (
	RoleProvides Role = "provides"
	RoleConsumes Role = "consumes"
)

// Tier is a key row's or a link's evidence tier (§4), strongest first:
// spec, typed, literal, template.
type Tier string

const (
	TierSpec     Tier = "spec"
	TierTyped    Tier = "typed"
	TierLiteral  Tier = "literal"
	TierTemplate Tier = "template"
)

// tierRank orders tiers strongest first; an unknown tier ranks weakest of
// all, so it can never lift a link.
func tierRank(t Tier) int {
	switch t {
	case TierSpec:
		return 0
	case TierTyped:
		return 1
	case TierLiteral:
		return 2
	case TierTemplate:
		return 3
	}
	return 4
}

// Weaker returns the weaker of two tiers: a link's tier is the weaker of
// its two ends (§4).
func Weaker(a, b Tier) Tier {
	if tierRank(b) > tierRank(a) {
		return b
	}
	return a
}

// KeyRow is one contract_keys row (§6): a node in a service provides or
// consumes a key, seen at file:line by a named extractor at an epoch.
type KeyRow struct {
	Service  string
	Key      ContractKey
	Role     Role
	NodeKind string // "route", "operation", "rpc", "call", ...
	NodeID   string // the node's id within its service: a handler, an operationId, an enclosing function
	File     string
	Line     int
	TierHint Tier
	Access   string            // "read" | "write" for table: rows; "" otherwise
	Attrs    map[string]string // kind attributes (a broker, an image tag, ...)
	// Extractor and ExtractorEpoch name the source that produced the row,
	// so a changed extractor re-derives its keys without guessing (§5).
	// The registry stamps both; a source's own values are overwritten.
	Extractor      string
	ExtractorEpoch int
}

// ContractSource is §5's port: one extractor of key rows for the files of
// one or more dialects. Keys must be pure and deterministic over
// (file, content, dialect) — ingest calls it inside its existing per-file
// pass, never with a second read of the file (§12).
type ContractSource interface {
	// ID is the extractor id recorded on every row.
	ID() string
	// Epoch is bumped whenever the source's output for unchanged input
	// changes, so stored rows from an older epoch are re-derived.
	Epoch() int
	Keys(file string, content []byte, dialect string) []KeyRow
}

// Registry maps dialect ids (ADR-048's manifest ids: "go", "typescript",
// "python", ...) to the contract sources that read them.
type Registry struct {
	mu        sync.RWMutex
	byDialect map[string][]ContractSource
}

// NewRegistry returns an empty registry.
func NewRegistry() *Registry { return &Registry{byDialect: map[string][]ContractSource{}} }

// RegisterContractSource registers src for dialect. It refuses, naming the
// reason, an empty dialect, a nil source, an empty id, an epoch below 1, or
// a second source with the same id for the same dialect.
func (r *Registry) RegisterContractSource(dialect string, src ContractSource) error {
	switch {
	case dialect == "":
		return errors.New("contract source: empty dialect id")
	case src == nil:
		return fmt.Errorf("contract source for dialect %q: nil source", dialect)
	case src.ID() == "":
		return fmt.Errorf("contract source for dialect %q: empty extractor id", dialect)
	case src.Epoch() < 1:
		return fmt.Errorf("contract source %q for dialect %q: epoch %d, want at least 1", src.ID(), dialect, src.Epoch())
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, s := range r.byDialect[dialect] {
		if s.ID() == src.ID() {
			return fmt.Errorf("contract source %q already registered for dialect %q", src.ID(), dialect)
		}
	}
	list := append(r.byDialect[dialect], src)
	sort.Slice(list, func(i, j int) bool { return list[i].ID() < list[j].ID() })
	r.byDialect[dialect] = list
	return nil
}

// Sources returns the sources registered for dialect, sorted by id.
func (r *Registry) Sources(dialect string) []ContractSource {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return append([]ContractSource(nil), r.byDialect[dialect]...)
}

// Dialects returns every dialect id with at least one source, sorted.
func (r *Registry) Dialects() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]string, 0, len(r.byDialect))
	for d := range r.byDialect {
		out = append(out, d)
	}
	sort.Strings(out)
	return out
}

// Keys runs every source registered for dialect over one file and returns
// their rows, each stamped with its source's extractor id and epoch,
// sorted (SortRows).
func (r *Registry) Keys(file string, content []byte, dialect string) []KeyRow {
	var out []KeyRow
	for _, s := range r.Sources(dialect) {
		for _, row := range s.Keys(file, content, dialect) {
			row.Extractor, row.ExtractorEpoch = s.ID(), s.Epoch()
			out = append(out, row)
		}
	}
	SortRows(out)
	return out
}

// SortRows orders rows by file, line, key, role, node and extractor — the
// one order every output of this package uses.
func SortRows(rows []KeyRow) {
	sort.SliceStable(rows, func(i, j int) bool {
		a, b := rows[i], rows[j]
		if a.Service != b.Service {
			return a.Service < b.Service
		}
		if a.File != b.File {
			return a.File < b.File
		}
		if a.Line != b.Line {
			return a.Line < b.Line
		}
		if ak, bk := a.Key.String(), b.Key.String(); ak != bk {
			return ak < bk
		}
		if a.Role != b.Role {
			return a.Role < b.Role
		}
		if a.NodeID != b.NodeID {
			return a.NodeID < b.NodeID
		}
		return a.Extractor < b.Extractor
	})
}
