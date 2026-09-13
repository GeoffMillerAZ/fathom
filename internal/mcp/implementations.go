// Package mcp — implementationsOf: find_symbol on a Go interface (SB-5,
// docs/plans/strengths-bench.md; docs/vision/27-strengths-bench.md §8,
// H1-T06). The index never persists an "implements" fact — the typed
// pass's interface-dispatch edges (core.EdgeIfaceDispatch) only exist where
// a call site actually dispatches through the interface — so
// implementationsOf reconstructs the answer from what IS indexed, at query
// time, using the strongest evidence available:
//
//  1. "typed" — a persisted EdgeIfaceDispatch edge targets one of the
//     interface's own methods on a candidate type (real call-graph
//     evidence a dispatch occurred).
//  2. "assertion" — a `var _ Iface = (*T)(nil)` / `var _ Iface = T{}`
//     compile-time assertion in the service's own source (the author
//     already told the compiler T implements Iface).
//  3. "method_set" — T's indexed GoMethod symbols structurally cover every
//     method the interface declares (name-only match; the fallback when
//     neither stronger form exists).
//
// candidates (method_set) is always computed first and is the search space
// the other two tiers narrow — an assertion or a dispatch edge naming a
// type that does NOT structurally satisfy the interface is never trusted
// (guards against an unrelated interface sharing a method name).
package mcp

import (
	"context"
	"go/ast"
	"go/parser"
	"go/token"
	"regexp"
	"sort"

	"github.com/geoffmilleraz/fathom/internal/core"
)

// implementationEntry is one row of find_symbol's "implementations" field.
type implementationEntry struct {
	Name     string `json:"name"`
	FilePath string `json:"file_path"`
	Line     int    `json:"line"`
	Via      string `json:"via"` // "typed" | "assertion" | "method_set"
}

// implementationsOf returns iface's implementers, deterministically sorted
// by name, or nil when iface declares no directly-parseable methods or no
// indexed type's method set covers them. iface must be a core.KindGoInterface
// node from the SAME service as allNodes.
func implementationsOf(ctx context.Context, backend QueryInterface, service string, iface core.SymbolNode, allNodes []core.SymbolNode) []implementationEntry {
	sources, err := backend.Sources(ctx, service)
	if err != nil || len(sources) == 0 {
		return nil
	}
	methods := interfaceMethodNames(sources, iface)
	if len(methods) == 0 {
		return nil
	}
	candidates := methodSetImplementers(allNodes, methods, iface.Name)
	if len(candidates) == 0 {
		return nil
	}
	if edges, edgeErr := backend.TypedEdges(ctx, service); edgeErr == nil {
		if typed := typedImplementers(edges, allNodes, candidates, methods); len(typed) > 0 {
			return renderImplementations(typed, "typed")
		}
	}
	if asserted := assertionImplementers(sources, iface.Name, candidates); len(asserted) > 0 {
		return renderImplementations(asserted, "assertion")
	}
	return renderImplementations(candidates, "method_set")
}

// interfaceMethodNames parses iface's OWN source file (matched by
// FilePath) and returns the set of method names it directly declares.
// Embedded interfaces named as a bare inline `interface{...}` are expanded
// one level; a NAMED embedded interface (same- or other-package) is not
// resolved — that only makes the method-set match MORE conservative, never
// wrong (a type that also satisfies the embedded methods still needs to
// satisfy the ones we DID find, so it's never a false positive).
func interfaceMethodNames(sources []core.SourceFile, iface core.SymbolNode) map[string]bool {
	for _, sf := range sources {
		if sf.Path != iface.FilePath {
			continue
		}
		fset := token.NewFileSet()
		f, err := parser.ParseFile(fset, sf.Path, sf.Content, 0)
		if err != nil {
			return nil
		}
		methods := map[string]bool{}
		for _, decl := range f.Decls {
			gd, ok := decl.(*ast.GenDecl)
			if !ok || gd.Tok != token.TYPE {
				continue
			}
			for _, spec := range gd.Specs {
				ts, ok := spec.(*ast.TypeSpec)
				if !ok || ts.Name == nil || ts.Name.Name != iface.Name {
					continue
				}
				it, ok := ts.Type.(*ast.InterfaceType)
				if !ok {
					continue
				}
				collectInterfaceFieldMethods(it, methods)
			}
		}
		if len(methods) > 0 {
			return methods
		}
		return nil
	}
	return nil
}

// collectInterfaceFieldMethods walks one interface body's method list.
func collectInterfaceFieldMethods(it *ast.InterfaceType, out map[string]bool) {
	if it == nil || it.Methods == nil {
		return
	}
	for _, field := range it.Methods.List {
		if len(field.Names) > 0 {
			for _, n := range field.Names {
				out[n.Name] = true
			}
			continue
		}
		// Embedded interface: only an inline `interface{...}` is expanded
		// here (a named embed is left unresolved — see doc comment).
		if embedded, ok := field.Type.(*ast.InterfaceType); ok {
			collectInterfaceFieldMethods(embedded, out)
		}
	}
}

// splitReceiverMethod splits a GoMethod SymbolNode's Name ("ReceiverType.
// MethodName") into its receiver type and bare method name. A name with no
// dot (unexpected for a GoMethod node) returns ("", name).
func splitReceiverMethod(name string) (recv, method string) {
	for i := 0; i < len(name); i++ {
		if name[i] == '.' {
			return name[:i], name[i+1:]
		}
	}
	return "", name
}

// methodSetImplementers returns every indexed type (other than iface
// itself) whose GoMethod symbols structurally cover every name in methods,
// keyed by type name to its representative declaration node (its own
// KindGoType/KindGoInterface node when indexed, else its earliest method's
// location as a fallback so the row still cites somewhere real).
func methodSetImplementers(allNodes []core.SymbolNode, methods map[string]bool, ifaceName string) map[string]core.SymbolNode {
	typeMethods := map[string]map[string]bool{}
	firstMethod := map[string]core.SymbolNode{}
	for _, n := range allNodes {
		if n.Kind != core.KindGoMethod {
			continue
		}
		recv, meth := splitReceiverMethod(n.Name)
		if recv == "" || recv == ifaceName {
			continue
		}
		if typeMethods[recv] == nil {
			typeMethods[recv] = map[string]bool{}
		}
		typeMethods[recv][meth] = true
		if cur, ok := firstMethod[recv]; !ok || n.Line < cur.Line {
			firstMethod[recv] = n
		}
	}
	typeDecls := map[string]core.SymbolNode{}
	for _, n := range allNodes {
		if n.Name == ifaceName {
			continue
		}
		if n.Kind == core.KindGoType || n.Kind == core.KindGoInterface {
			if _, ok := typeDecls[n.Name]; !ok {
				typeDecls[n.Name] = n
			}
		}
	}

	out := map[string]core.SymbolNode{}
	for typeName, have := range typeMethods {
		covers := true
		for m := range methods {
			if !have[m] {
				covers = false
				break
			}
		}
		if !covers {
			continue
		}
		if decl, ok := typeDecls[typeName]; ok {
			out[typeName] = decl
			continue
		}
		out[typeName] = core.SymbolNode{Name: typeName, FilePath: firstMethod[typeName].FilePath, Line: firstMethod[typeName].Line}
	}
	return out
}

// assertionImplementers scans every source file for a compile-time
// `var _ [pkg.]IfaceName = (*T)(nil)` or `var _ [pkg.]IfaceName = T{}`
// assertion, returning the subset of candidates it names.
func assertionImplementers(sources []core.SourceFile, ifaceName string, candidates map[string]core.SymbolNode) map[string]core.SymbolNode {
	re := regexp.MustCompile(`var\s+_\s+(?:[A-Za-z_]\w*\.)?` + regexp.QuoteMeta(ifaceName) + `\s*=\s*(?:\(\*(\w+)\)|(\w+))`)
	out := map[string]core.SymbolNode{}
	for _, sf := range sources {
		for _, m := range re.FindAllStringSubmatch(string(sf.Content), -1) {
			t := m[1]
			if t == "" {
				t = m[2]
			}
			if decl, ok := candidates[t]; ok {
				out[t] = decl
			}
		}
	}
	return out
}

// typedImplementers narrows candidates to those confirmed by a persisted
// EdgeIfaceDispatch edge targeting one of the interface's own methods on
// that candidate type — real call-graph evidence a dispatch occurred, not
// merely a structural match.
func typedImplementers(edges []core.Edge, allNodes []core.SymbolNode, candidates map[string]core.SymbolNode, methods map[string]bool) map[string]core.SymbolNode {
	if len(edges) == 0 || len(candidates) == 0 {
		return nil
	}
	byKey := map[string]core.SymbolNode{}
	for _, n := range allNodes {
		if n.Kind == core.KindGoMethod {
			byKey[n.Key()] = n
		}
	}
	out := map[string]core.SymbolNode{}
	for _, e := range edges {
		if e.Kind != core.EdgeIfaceDispatch {
			continue
		}
		node, ok := byKey[e.ToSymbol]
		if !ok {
			continue
		}
		recv, meth := splitReceiverMethod(node.Name)
		if !methods[meth] {
			continue
		}
		if decl, ok := candidates[recv]; ok {
			out[recv] = decl
		}
	}
	return out
}

// renderImplementations converts a candidate map into the sorted,
// deterministic response slice.
func renderImplementations(m map[string]core.SymbolNode, via string) []implementationEntry {
	out := make([]implementationEntry, 0, len(m))
	for name, n := range m {
		out = append(out, implementationEntry{Name: name, FilePath: n.FilePath, Line: n.Line, Via: via})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}
