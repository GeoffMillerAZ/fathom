// Package repofacts is fathom's one classifier of what a repository file is
// (RF-1, decision D2 of docs/vision/42-every-file-accounted-for.md,
// ADR-067 decision 2): a role from a closed vocabulary and the basis that
// decided it, computed on read from the path, the file's dialect, its
// .gitattributes and its header bytes.
//
// It is a pure leaf. It imports no store, fusion, mcp, certify, conserve,
// custodian or extractor package (TestRepofactsIsAPureLeaf): fusion reaches
// it through internal/vendored, and the dialect it reads is an input its
// caller supplies, never a lookup in the extractor registry.
//
// The basis order is fixed: attribute (.gitattributes linguist-*, root and
// nested) over header (Go's generated-code line, the ADR-063 O6 kit
// provenance header) over dialect (the file's dialect id) over linguist
// (Linguist's vendor, documentation and generated data, pinned in
// linguist.json) over convention (the core table in convention.go plus the
// dialect-pack data in packs.json). `unknown` is a value: every answer that
// uses the role counts and prints it (I65), through Tally.
package repofacts

import (
	"fmt"
	"strings"
)

// RoleName is one value of the closed role vocabulary.
type RoleName string

// The closed vocabulary (vision 42 section 3.2), in Vocabulary's order.
const (
	Source    RoleName = "source"
	Test      RoleName = "test"
	Fixture   RoleName = "fixture"
	Script    RoleName = "script"
	Config    RoleName = "config"
	CI        RoleName = "ci"
	Build     RoleName = "build"
	IaC       RoleName = "iac"
	Schema    RoleName = "schema"
	Migration RoleName = "migration"
	Docs      RoleName = "docs"
	Generated RoleName = "generated"
	Vendored  RoleName = "vendored"
	Unknown   RoleName = "unknown"
)

// Vocabulary returns the closed role vocabulary, unknown last.
func Vocabulary() []RoleName {
	return []RoleName{Source, Test, Fixture, Script, Config, CI, Build, IaC, Schema, Migration, Docs, Generated, Vendored, Unknown}
}

// Basis names what decided a role. An unknown role has none.
type Basis string

// The bases, strongest first.
const (
	ByAttribute  Basis = "attribute"
	ByHeader     Basis = "header"
	ByDialect    Basis = "dialect"
	ByLinguist   Basis = "linguist"
	ByConvention Basis = "convention"
)

// Fact is a file's role and the basis that decided it.
type Fact struct {
	Role  RoleName `json:"role"`
	Basis Basis    `json:"basis,omitempty"`
}

// String renders a fact as "role/basis", or "unknown".
func (f Fact) String() string {
	if f.Basis == "" {
		return string(f.Role)
	}
	return string(f.Role) + "/" + string(f.Basis)
}

// Role classifies one repo-relative slash path. dialect is the file's
// dialect id or language tag as the caller knows it ("" when it knows
// none), attrs what ReadAttributes read for the path, and header the
// file's leading bytes (nil when unread: the header basis then decides
// nothing). The first basis that decides wins, in the package's order.
func Role(path, dialect string, attrs Attributes, header []byte) Fact {
	path = strings.TrimPrefix(path, "./")
	if r, ok := attrs.role(); ok {
		return Fact{r, ByAttribute}
	}
	if r, ok := headerRole(path, dialect, header); ok && attrs.allows(r) {
		return Fact{r, ByHeader}
	}
	if r, ok := dialectRole(dialect); ok && attrs.allows(r) {
		return Fact{r, ByDialect}
	}
	if r, ok := linguistRole(path, attrs); ok {
		return Fact{r, ByLinguist}
	}
	if r := conventionRole(path, dialect, attrs); r != Unknown {
		return Fact{r, ByConvention}
	}
	return Fact{Role: Unknown}
}

// Tally counts an answer's files by role. Its zero value is ready to use.
type Tally struct {
	n map[RoleName]int
}

// Add counts one file's fact.
func (t *Tally) Add(f Fact) {
	if t.n == nil {
		t.n = map[RoleName]int{}
	}
	t.n[f.Role]++
}

// Unknown is how many counted files no rule decided.
func (t Tally) Unknown() int { return t.n[Unknown] }

// Count is how many counted files read role r.
func (t Tally) Count(r RoleName) int { return t.n[r] }

// String prints the counts in vocabulary order, skipping a zero role but
// never the unknown count, which is always printed (I65).
func (t Tally) String() string {
	var parts []string
	for _, r := range Vocabulary() {
		if c := t.n[r]; c > 0 || r == Unknown {
			parts = append(parts, fmt.Sprintf("%s %d", r, c))
		}
	}
	return strings.Join(parts, ", ")
}
