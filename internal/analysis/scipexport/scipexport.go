// Package scipexport renders one service's symbol and typed-edge tables as
// a SCIP index (VB-32, docs/plans/verbs.md; docs/vision/29-verbs-not-
// lookups.md §I "fathom export scip"), using the official bindings
// github.com/sourcegraph/scip/bindings/go/scip.
//
// Resolution-tier placement (state exactly where and why, per VB-32):
// SCIP's Relationship message is {Symbol, IsReference, IsImplementation,
// IsTypeDefinition, IsDefinition} — four booleans, no free-form field — so
// a typed call edge's tier (core.EdgeConfidenceOf: "resolved" or
// "heuristic") travels as a Documentation line on the CALLER's
// SymbolInformation, one line per relationship, self-describing
// ("fathom:resolution <target-symbol> = <tier>") and keyed by the target
// symbol string rather than by list position — so it survives a consumer
// that reorders or filters Relationships. A definition's own Occurrence
// carries no tier: tier is a property of an EDGE, never of a definition
// site.
//
// BuildFromNodes is pure (no I/O) and deterministic: documents are sorted
// by RelativePath, and each document's occurrences/symbols are sorted for
// stable byte output across repeated runs over the same input.
package scipexport

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"sort"

	"github.com/sourcegraph/scip/bindings/go/scip"
	"google.golang.org/protobuf/proto"

	"github.com/geoffmilleraz/fathom/internal/buildinfo"
	"github.com/geoffmilleraz/fathom/internal/core"
)

// schemeName / packageManager name the fathom-specific corner of SCIP's
// symbol grammar (scheme + package manager + package name + descriptors).
// fathom is not a real package manager; "service" names what its role in
// the symbol actually is — the fathom service ID a node belongs to.
const (
	schemeName     = "fathom"
	packageManager = "service"
)

// Backend is the minimal capability Build needs — pkg/client.Client and
// internal/mcp.QueryInterface both satisfy it structurally (Go interfaces
// are structural; no adapter needed).
type Backend interface {
	ListAllSymbols(ctx context.Context, service string) ([]core.SymbolNode, error)
	TypedEdges(ctx context.Context, service string) ([]core.Edge, error)
	Sources(ctx context.Context, service string) ([]core.SourceFile, error)
}

// Build renders service's symbol and typed-edge tables (read from backend)
// as a SCIP index.
func Build(ctx context.Context, backend Backend, service string) (*scip.Index, error) {
	nodes, err := backend.ListAllSymbols(ctx, service)
	if err != nil {
		return nil, fmt.Errorf("scipexport: list symbols for %q: %w", service, err)
	}
	// Typed edges and source language tags are both best-effort: a service
	// that never ran the Go typed pass (ADR-021 §3) or whose connector
	// doesn't serve Sources() still gets a valid index — nodes only, no
	// relationships, Document.Language "".
	typed, _ := backend.TypedEdges(ctx, service)
	srcs, _ := backend.Sources(ctx, service)
	return BuildFromNodes(service, nodes, typed, srcs), nil
}

// SymbolID renders the canonical SCIP symbol string for one fathom symbol
// (service, package, name). Exported so a caller decoding a written index
// can recompute the same ID for a symbol-table comparison (the round-trip
// check `fathom export scip` runs when scip-go is not on PATH) without
// re-deriving the grammar by hand.
func SymbolID(service, pkg, name string) string {
	sym := &scip.Symbol{
		Scheme:  schemeName,
		Package: &scip.Package{Manager: packageManager, Name: service},
		Descriptors: []*scip.Descriptor{
			{Name: pkg, Suffix: scip.Descriptor_Namespace},
			{Name: name, Suffix: scip.Descriptor_Term},
		},
	}
	return scip.VerboseSymbolFormatter.FormatSymbol(sym)
}

// symbolKind best-effort maps a core.SourceKind to SCIP's SymbolInformation
// enum. Unmapped kinds (Terraform/K8s/CUE/doc-chunk/module-dep — none of
// which the Go typed pass touches) fall back to UnspecifiedKind rather than
// guessing.
func symbolKind(k core.SourceKind) scip.SymbolInformation_Kind {
	switch k {
	case core.KindGoFunc:
		return scip.SymbolInformation_Function
	case core.KindGoMethod:
		return scip.SymbolInformation_Method
	case core.KindGoType:
		return scip.SymbolInformation_Type
	case core.KindGoVar:
		return scip.SymbolInformation_Variable
	case core.KindGoConst:
		return scip.SymbolInformation_Constant
	case core.KindGoInterface:
		return scip.SymbolInformation_Interface
	default:
		return scip.SymbolInformation_UnspecifiedKind
	}
}

// zeroBased converts a 1-based fathom line number to SCIP's 0-based line;
// a fathom line of 0 (unknown) maps to 0 rather than -1, since a negative
// SCIP range component is invalid.
func zeroBased(line int) int32 {
	if line <= 0 {
		return 0
	}
	return int32(line - 1)
}

// BuildFromNodes is Build's pure core: no I/O, deterministic (sorted
// documents, occurrences and symbols), directly testable without a store.
//
// One scip.Document per distinct SymbolNode.FilePath. Each node gets one
// definition Occurrence (SymbolRoles: Definition) spanning its Line..EndLine
// (EndLine falls back to Line when the extractor never set it — T5-5's
// "0 == unknown" convention) and one SymbolInformation (DisplayName +
// Signature-as-Documentation when non-empty). A typed edge
// (internal/fusion.EnrichTypedEdges) whose FROM and TO both resolve to a
// node this service actually indexed becomes a Relationship on the FROM
// symbol's SymbolInformation, IsReference:true, plus the resolution-tier
// Documentation line described in the package doc comment. An edge whose
// target is NOT among this service's own nodes (a cross-package call this
// service didn't index, or a genuinely external dependency) is skipped —
// SCIP's Index.ExternalSymbols table is the correct home for that and is
// out of scope for VB-32 (docs/usage.md notes this explicitly).
func BuildFromNodes(service string, nodes []core.SymbolNode, typed []core.Edge, srcs []core.SourceFile) *scip.Index {
	langByPath := make(map[string]string, len(srcs))
	for _, f := range srcs {
		langByPath[f.Path] = f.Lang
	}

	idByKey := make(map[string]string, len(nodes))
	infoByKey := make(map[string]*scip.SymbolInformation, len(nodes))
	docByPath := make(map[string]*scip.Document)
	pathSeen := make(map[string]bool)
	var pathOrder []string

	for _, n := range nodes {
		id := SymbolID(service, n.Package, n.Name)
		idByKey[n.Key()] = id

		info := &scip.SymbolInformation{Symbol: id, DisplayName: n.Name, Kind: symbolKind(n.Kind)}
		if n.Signature != "" {
			info.Documentation = append(info.Documentation, n.Signature)
		}
		infoByKey[n.Key()] = info

		doc, ok := docByPath[n.FilePath]
		if !ok {
			doc = &scip.Document{RelativePath: n.FilePath, Language: langByPath[n.FilePath]}
			docByPath[n.FilePath] = doc
		}
		if !pathSeen[n.FilePath] {
			pathSeen[n.FilePath] = true
			pathOrder = append(pathOrder, n.FilePath)
		}

		endLine := n.Line
		if n.EndLine > 0 {
			endLine = n.EndLine
		}
		doc.Occurrences = append(doc.Occurrences, &scip.Occurrence{
			Range:       []int32{zeroBased(n.Line), 0, zeroBased(endLine), 1},
			Symbol:      id,
			SymbolRoles: int32(scip.SymbolRole_Definition),
		})
		doc.Symbols = append(doc.Symbols, info)
	}

	for _, e := range typed {
		fromInfo, ok := infoByKey[e.FromSymbol]
		if !ok {
			continue
		}
		toID, ok := idByKey[e.ToSymbol]
		if !ok {
			continue
		}
		tier := string(core.EdgeConfidenceOf(e))
		fromInfo.Relationships = append(fromInfo.Relationships, &scip.Relationship{Symbol: toID, IsReference: true})
		fromInfo.Documentation = append(fromInfo.Documentation, fmt.Sprintf("fathom:resolution %s = %s", toID, tier))
	}

	sort.Strings(pathOrder)
	documents := make([]*scip.Document, 0, len(pathOrder))
	for _, p := range pathOrder {
		doc := docByPath[p]
		sortOccurrences(doc.Occurrences)
		sortSymbolInformation(doc.Symbols)
		documents = append(documents, doc)
	}

	return &scip.Index{
		Metadata: &scip.Metadata{
			Version:              scip.ProtocolVersion_UnspecifiedProtocolVersion,
			ToolInfo:             &scip.ToolInfo{Name: "fathom", Version: buildinfo.Version()},
			ProjectRoot:          "fathom-service://" + service,
			TextDocumentEncoding: scip.TextEncoding_UTF8,
		},
		Documents: documents,
	}
}

func sortOccurrences(occs []*scip.Occurrence) {
	sort.Slice(occs, func(i, j int) bool {
		a, b := occs[i], occs[j]
		if len(a.Range) > 0 && len(b.Range) > 0 && a.Range[0] != b.Range[0] {
			return a.Range[0] < b.Range[0]
		}
		return a.Symbol < b.Symbol
	})
}

func sortSymbolInformation(syms []*scip.SymbolInformation) {
	sort.Slice(syms, func(i, j int) bool { return syms[i].Symbol < syms[j].Symbol })
}

// SymbolSet returns the set of every symbol string an index defines —
// Document.Symbols across every document, plus ExternalSymbols. Used by the
// round-trip check (`fathom export scip`'s fallback path when scip-go is
// not on PATH) to compare a decoded index's symbol table against the
// store's, and by tests.
func SymbolSet(idx *scip.Index) map[string]bool {
	out := map[string]bool{}
	for _, d := range idx.Documents {
		for _, s := range d.Symbols {
			out[s.Symbol] = true
		}
	}
	for _, s := range idx.ExternalSymbols {
		out[s.Symbol] = true
	}
	return out
}

// ExpectedSymbolSet returns the set of symbol IDs BuildFromNodes would mint
// for nodes, without building a full Index — the independent expectation
// RoundTrip's bindings-decode fallback checks a DECODED (marshal/unmarshal
// round-tripped) index against, so the check catches real serialization
// corruption rather than tautologically comparing an index against itself.
func ExpectedSymbolSet(service string, nodes []core.SymbolNode) map[string]bool {
	out := make(map[string]bool, len(nodes))
	for _, n := range nodes {
		out[SymbolID(service, n.Package, n.Name)] = true
	}
	return out
}

// DecodeIndex unmarshals raw SCIP protobuf bytes (as Marshal(idx) produced)
// back into an *scip.Index.
func DecodeIndex(data []byte) (*scip.Index, error) {
	idx := &scip.Index{}
	if err := proto.Unmarshal(data, idx); err != nil {
		return nil, fmt.Errorf("scipexport: decode index: %w", err)
	}
	return idx, nil
}

// Marshal renders idx as SCIP's wire format (a plain protobuf encoding —
// SCIP has no separate container format).
func Marshal(idx *scip.Index) ([]byte, error) {
	data, err := proto.Marshal(idx)
	if err != nil {
		return nil, fmt.Errorf("scipexport: marshal index: %w", err)
	}
	return data, nil
}

// RoundTripReport summarizes `fathom export scip`'s self-check (VB-32): did
// the just-written index decode back to the symbol table the store itself
// reports.
type RoundTripReport struct {
	// Method is "scip-go" when an independent scip-go-generated index was
	// compared against ours, or "bindings-decode" when scip-go was not on
	// PATH and the check instead decoded fathom's own written bytes and
	// compared them against the store's expected symbol set.
	Method string
	// SymbolsExpected is the store's own symbol count for the service.
	SymbolsExpected int
	// SymbolsMatched is how many of those were confirmed present (by bare
	// name for the scip-go method, since scip-go's own symbol scheme
	// differs from fathom's; by exact ID for the bindings-decode method).
	SymbolsMatched int
	// OK is SymbolsMatched == SymbolsExpected (trivially true when both are
	// zero — an empty service has nothing to disagree about).
	OK bool
	// Note carries an operational aside — e.g. "scip-go not on PATH", or a
	// scip-go invocation failure that caused a fallback to bindings-decode.
	Note string
}

// scipGoSymbolNames runs `scip-go` over the Go module rooted at modulePath,
// producing an independent SCIP index at <modulePath>/index.scip, decodes
// it, and returns the bare (last-descriptor) name of every symbol it
// defines. Best-effort: any failure (scip-go exits non-zero, writes no
// index, or writes an undecodable one) is returned as an error so the
// caller can fall back to the bindings-decode method rather than failing
// the whole export.
func scipGoSymbolNames(scipGoPath, modulePath string) (map[string]bool, error) {
	outPath := modulePath + "/index.scip"
	cmd := exec.Command(scipGoPath)
	cmd.Dir = modulePath
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("scip-go: %w: %s", err, stderr.String())
	}
	data, err := os.ReadFile(outPath)
	if err != nil {
		return nil, fmt.Errorf("scip-go: read %s: %w", outPath, err)
	}
	idx, err := DecodeIndex(data)
	if err != nil {
		return nil, fmt.Errorf("scip-go: %w", err)
	}
	names := map[string]bool{}
	for sym := range SymbolSet(idx) {
		names[bareSymbolName(sym)] = true
	}
	return names, nil
}

// bareSymbolName extracts a best-effort display name from a SCIP symbol
// string for cross-scheme comparison (fathom's "fathom service <svc>
// <pkg>/<name>." vs scip-go's own gomod-based scheme): the text of the
// LAST descriptor, stripped of its trailing suffix punctuation
// (`.`/`#`/`)`/`/`). Falls back to the raw string when scip.ParseSymbol
// can't parse it (a "local ..." symbol, or a scheme scip-go formats
// differently) — a bare-name comparison degrades to no-match rather than
// panicking on either producer's own quirks.
func bareSymbolName(sym string) string {
	parsed, err := scip.ParseSymbol(sym)
	if err != nil || len(parsed.Descriptors) == 0 {
		return sym
	}
	return parsed.Descriptors[len(parsed.Descriptors)-1].Name
}

func setNamesEqualCount(want map[string]bool, gotBareNames map[string]bool) int {
	matched := 0
	for w := range want {
		if gotBareNames[bareSymbolName(w)] {
			matched++
		}
	}
	return matched
}

// RoundTrip is `fathom export scip`'s self-check: does the just-written
// index's symbol table agree with the store's own symbol list for service.
//
//   - If `scip-go` is on PATH and modulePath is non-empty, it runs scip-go
//     over modulePath (an independent SCIP producer for the same Go
//     source) and compares bare symbol names against the store's expected
//     set — a genuine cross-tool check. Any scip-go failure (missing
//     module, non-zero exit, undecodable output) falls back to the
//     bindings-decode method rather than failing the export.
//   - Otherwise (scip-go absent, or modulePath not supplied — the common
//     case: most machines running `fathom export scip` do not have scip-go
//     installed), it decodes data (the bytes just written) with the scip
//     bindings and compares the decoded symbol table against
//     ExpectedSymbolSet(service, nodes) — proving the marshal/unmarshal
//     round trip is faithful, and naming in Note that the scip-go
//     cross-tool comparison is pending on a machine with scip-go.
func RoundTrip(service string, nodes []core.SymbolNode, data []byte, modulePath string) (RoundTripReport, error) {
	want := ExpectedSymbolSet(service, nodes)

	if modulePath != "" {
		if scipGoPath, lookErr := exec.LookPath("scip-go"); lookErr == nil {
			if got, err := scipGoSymbolNames(scipGoPath, modulePath); err == nil {
				matched := setNamesEqualCount(want, got)
				return RoundTripReport{
					Method:          "scip-go",
					SymbolsExpected: len(want),
					SymbolsMatched:  matched,
					OK:              matched == len(want),
				}, nil
			}
			// scip-go present but failed to run cleanly: fall through to
			// the bindings-decode method rather than failing the export.
		}
	}

	decoded, err := DecodeIndex(data)
	if err != nil {
		return RoundTripReport{}, err
	}
	got := SymbolSet(decoded)
	matched := 0
	for w := range want {
		if got[w] {
			matched++
		}
	}
	note := "scip-go comparison pending — install scip-go and re-run to exercise the cross-tool check"
	if _, lookErr := exec.LookPath("scip-go"); lookErr == nil {
		note = "scip-go is on PATH but did not run cleanly against the module — see stderr; verified via bindings decode only"
	} else if _, lookErr := exec.LookPath("scip"); lookErr == nil {
		note = "scip is on PATH (scip-go is not) — verified via bindings decode only; scip-go cross-tool comparison pending"
	}
	return RoundTripReport{
		Method:          "bindings-decode",
		SymbolsExpected: len(want),
		SymbolsMatched:  matched,
		OK:              matched == len(want),
		Note:            note,
	}, nil
}

// String renders r as a single human-readable line for `fathom export
// scip`'s stdout.
func (r RoundTripReport) String() string {
	status := "ok"
	if !r.OK {
		status = "MISMATCH"
	}
	line := fmt.Sprintf("round-trip (%s): %d/%d symbols matched — %s", r.Method, r.SymbolsMatched, r.SymbolsExpected, status)
	if r.Note != "" {
		line += "; " + r.Note
	}
	return line
}
