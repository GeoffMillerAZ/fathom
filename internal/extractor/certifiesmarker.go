package extractor

// certifiesmarker.go — the `// certifies:` marker (vision 33 / ADR-043
// stage 0, TC-5, design doc §1): the Go extractor's reading of the
// per-test declaration grammar
//
//	certifies: <kind> <id> [(<strength>)]{, …}
//
// on the comment line(s) immediately above a Test* function. Grammar:
// "certifies:" then a comma-separated list of `<kind> <id>` with an
// optional `(<strength>)`; strength defaults to "asserts". A malformed
// marker is reported as a warning naming its file and line, never a
// silent skip — ParseCertifiesMarker is the pure grammar (TC-5's own
// contract symbol); ScanCertifiesMarkers walks a file's Test* functions
// and calls it. Only Go this round (design doc: "the marker grammar is
// comment-based so every other extractor can add it" — a plan row per
// language, deferred with a trigger, TC-26).

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/core"
)

// CertifyKind is the closed set of certification target kinds (design doc
// §1).
type CertifyKind string

// The six certification target kinds design doc §1 names.
const (
	CertifyKindInvariant CertifyKind = "invariant"
	CertifyKindSpec      CertifyKind = "spec"
	CertifyKindRule      CertifyKind = "rule"
	CertifyKindIncident  CertifyKind = "incident"
	CertifyKindPort      CertifyKind = "port"
	CertifyKindBehaviour CertifyKind = "behaviour"
)

var validCertifyKinds = map[CertifyKind]bool{
	CertifyKindInvariant: true,
	CertifyKindSpec:      true,
	CertifyKindRule:      true,
	CertifyKindIncident:  true,
	CertifyKindPort:      true,
	CertifyKindBehaviour: true,
}

// CertifyStrength is the closed, ORDERED strength ladder (design doc §1):
// smoke < asserts < golden < property < conformance < mutation-killed.
type CertifyStrength string

// The six strengths, in ladder order.
const (
	StrengthSmoke          CertifyStrength = "smoke"
	StrengthAsserts        CertifyStrength = "asserts"
	StrengthGolden         CertifyStrength = "golden"
	StrengthProperty       CertifyStrength = "property"
	StrengthConformance    CertifyStrength = "conformance"
	StrengthMutationKilled CertifyStrength = "mutation-killed"
)

// DefaultCertifyStrength is the strength an entry with no `(<strength>)`
// clause carries.
const DefaultCertifyStrength = StrengthAsserts

// CertifyStrengthRank orders the ladder (0 = weakest) for a "meets at
// least" comparison — stage 1's concern (the minimum a `conformance` or
// `mutation-killed` certificate implies, design doc §1), defined here
// since the ladder itself is this file's contract.
var CertifyStrengthRank = map[CertifyStrength]int{
	StrengthSmoke:          0,
	StrengthAsserts:        1,
	StrengthGolden:         2,
	StrengthProperty:       3,
	StrengthConformance:    4,
	StrengthMutationKilled: 5,
}

var validCertifyStrengths = map[CertifyStrength]bool{
	StrengthSmoke: true, StrengthAsserts: true, StrengthGolden: true,
	StrengthProperty: true, StrengthConformance: true, StrengthMutationKilled: true,
}

// CertifyEntry is one `<kind> <id> [(<strength>)]` clause parsed from a
// `// certifies:` marker.
type CertifyEntry struct {
	Kind     CertifyKind
	TargetID string
	Strength CertifyStrength
}

// certifiesLabel is the marker's case-sensitive label, exactly as design
// doc §1 spells it.
const certifiesLabel = "certifies:"

// ParseCertifiesMarker parses body — the text AFTER the "certifies:"
// label on a marker line, e.g. " invariant I35 (asserts), incident
// INC-21" — into its comma-separated `<kind> <id> [(<strength>)]`
// entries. strength defaults to "asserts" when omitted.
//
// Returns an error naming exactly what is wrong (no entries, a missing
// id, an unknown kind or strength, an empty entry from a stray comma) —
// the caller (ScanCertifiesMarkers) wraps it with the file:line the
// marker line was found at, for the ingest warning design doc §1
// requires ("a malformed marker is an ingest warning naming the file and
// line, never a silent skip").
func ParseCertifiesMarker(body string) ([]CertifyEntry, error) {
	body = strings.TrimSpace(body)
	if body == "" {
		return nil, fmt.Errorf("certifies: marker has no entries")
	}

	clauses := splitTopLevelCommas(body)
	entries := make([]CertifyEntry, 0, len(clauses))
	for i, clause := range clauses {
		clause = strings.TrimSpace(clause)
		if clause == "" {
			return nil, fmt.Errorf("entry %d: empty entry (a stray comma?)", i+1)
		}
		entry, err := parseCertifyClause(clause)
		if err != nil {
			return nil, fmt.Errorf("entry %d (%q): %w", i+1, clause, err)
		}
		entries = append(entries, entry)
	}
	return entries, nil
}

// splitTopLevelCommas splits s on commas that are not inside a
// "(<strength>)" parenthetical, so a malformed nested comma cannot
// fragment one entry into two nonsense ones.
func splitTopLevelCommas(s string) []string {
	var out []string
	depth := 0
	start := 0
	for i, r := range s {
		switch r {
		case '(':
			depth++
		case ')':
			if depth > 0 {
				depth--
			}
		case ',':
			if depth == 0 {
				out = append(out, s[start:i])
				start = i + 1
			}
		}
	}
	out = append(out, s[start:])
	return out
}

// parseCertifyClause parses one `<kind> <id> [(<strength>)]` clause
// (splitTopLevelCommas' own output, already comma-free).
func parseCertifyClause(clause string) (CertifyEntry, error) {
	strength := DefaultCertifyStrength
	rest := clause
	if strings.HasSuffix(clause, ")") {
		open := strings.LastIndex(clause, "(")
		if open < 0 {
			return CertifyEntry{}, fmt.Errorf("unbalanced parentheses")
		}
		strengthText := strings.TrimSpace(clause[open+1 : len(clause)-1])
		if strengthText == "" {
			return CertifyEntry{}, fmt.Errorf("empty strength in parentheses")
		}
		s := CertifyStrength(strengthText)
		if !validCertifyStrengths[s] {
			return CertifyEntry{}, fmt.Errorf(
				"unknown strength %q (want one of smoke, asserts, golden, property, conformance, mutation-killed)",
				strengthText)
		}
		strength = s
		rest = strings.TrimSpace(clause[:open])
	}

	fields := strings.Fields(rest)
	if len(fields) < 2 {
		return CertifyEntry{}, fmt.Errorf("want `<kind> <id>`, got %q", rest)
	}
	kind := CertifyKind(fields[0])
	if !validCertifyKinds[kind] {
		return CertifyEntry{}, fmt.Errorf(
			"unknown kind %q (want one of invariant, spec, rule, incident, port, behaviour)", fields[0])
	}
	id := strings.TrimSpace(strings.TrimPrefix(rest, fields[0]))
	if id == "" {
		return CertifyEntry{}, fmt.Errorf("empty id for kind %q", kind)
	}
	return CertifyEntry{Kind: kind, TargetID: id, Strength: strength}, nil
}

// CertifyDeclaration is one declared certification fact scanned from a Go
// source file's `// certifies:` marker. TestSymbolKey uses the same
// "service::pkg.Name" convention core.SymbolNode.Key() produces for the
// Test* function the marker sits above (core.SymbolKey +
// core.QualifiedPackage — the exact pair GoExtractor.Extract uses to key
// every other symbol), so it joins directly against the index without a
// second resolution pass.
type CertifyDeclaration struct {
	TestSymbolKey string
	Kind          CertifyKind
	TargetID      string
	Strength      CertifyStrength
	File          string
	Line          int
}

// CertifyMarkerWarning names one malformed `// certifies:` marker's
// location — an ingest warning, never a silent skip.
type CertifyMarkerWarning struct {
	File string
	Line int
	// Text is the marker comment line's own raw text, for the warning
	// message.
	Text string
	Err  string
}

// ScanCertifiesMarkers walks a single Go source file for `// certifies:`
// markers on the comment lines immediately above a Test* function. Only
// TOP-LEVEL functions are considered (a method's receiver would need its
// own test-symbol-key qualification GoExtractor already handles for
// ordinary symbols, but no test framework calls a method as a test).
// service and relPath give the same key-building context
// GoExtractor.Extract receives.
func ScanCertifiesMarkers(service, relPath string, src []byte) ([]CertifyDeclaration, []CertifyMarkerWarning, error) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, relPath, src, parser.ParseComments)
	if err != nil {
		return nil, nil, err
	}

	qualifiedPkg := core.QualifiedPackage(relPath, f.Name.Name)

	var decls []CertifyDeclaration
	var warnings []CertifyMarkerWarning
	for _, decl := range f.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Name == nil || fn.Recv != nil || fn.Doc == nil {
			continue
		}
		if !strings.HasPrefix(fn.Name.Name, "Test") {
			continue
		}
		key := core.SymbolKey(service, qualifiedPkg, fn.Name.Name)
		for _, c := range fn.Doc.List {
			text := strings.TrimSpace(strings.TrimPrefix(c.Text, "//"))
			if !strings.HasPrefix(text, certifiesLabel) {
				continue
			}
			pos := fset.Position(c.Pos())
			entries, perr := ParseCertifiesMarker(strings.TrimPrefix(text, certifiesLabel))
			if perr != nil {
				warnings = append(warnings, CertifyMarkerWarning{
					File: relPath, Line: pos.Line, Text: strings.TrimSpace(c.Text), Err: perr.Error(),
				})
				continue
			}
			for _, e := range entries {
				decls = append(decls, CertifyDeclaration{
					TestSymbolKey: key, Kind: e.Kind, TargetID: e.TargetID, Strength: e.Strength,
					File: relPath, Line: pos.Line,
				})
			}
		}
	}
	return decls, warnings, nil
}
