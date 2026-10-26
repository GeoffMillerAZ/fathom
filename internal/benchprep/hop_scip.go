package benchprep

import (
	"fmt"
	"io/fs"
	"sort"
	"strings"

	scippb "github.com/geoffmilleraz/fathom/third_party/scip"
)

type scipPoint struct{ line, column int32 }

func pointLess(a, b scipPoint) bool {
	return a.line < b.line || (a.line == b.line && a.column < b.column)
}

type scipSpan struct{ start, end scipPoint }

func parseSCIPSpan(raw []int32) (scipSpan, bool) {
	var span scipSpan
	switch len(raw) {
	case 3:
		span = scipSpan{scipPoint{raw[0], raw[1]}, scipPoint{raw[0], raw[2]}}
	case 4:
		span = scipSpan{scipPoint{raw[0], raw[1]}, scipPoint{raw[2], raw[3]}}
	default:
		return span, false
	}
	return span, span.start.line >= 0 && span.start.column >= 0 && span.end.column >= 0 && pointLess(span.start, span.end)
}

func (s scipSpan) contains(p scipPoint) bool {
	return !pointLess(p, s.start) && pointLess(p, s.end)
}

func scipNodeID(path, symbol string) string {
	if strings.HasPrefix(symbol, "local ") {
		return path + "\x00" + symbol
	}
	return symbol
}

func scipHopGraph(index *scippb.Index) (hopGraph, error) {
	graph := hopGraph{nodes: map[string]hopNode{}, edges: map[string][]string{}}
	type definition struct {
		id   string
		span scipSpan
	}
	byPath := map[string][]definition{}
	for _, doc := range index.Documents {
		if !fs.ValidPath(doc.RelativePath) {
			return graph, fmt.Errorf("hop distance: invalid SCIP document path")
		}
		info := map[string]*scippb.SymbolInformation{}
		for _, symbol := range doc.Symbols {
			info[symbol.Symbol] = symbol
		}
		for _, occurrence := range doc.Occurrences {
			if occurrence.SymbolRoles&int32(scippb.SymbolRole_Definition) == 0 || occurrence.Symbol == "" {
				continue
			}
			// Parameter and type-parameter descriptors cannot own bodies.
			if strings.HasSuffix(occurrence.Symbol, ")") || strings.HasSuffix(occurrence.Symbol, "]") {
				continue
			}
			span, ok := parseSCIPSpan(occurrence.EnclosingRange)
			if !ok {
				// Local variables need no body and cannot own references.
				// Global definitions with unknown extent would corrupt
				// containment, so they make this oracle unavailable.
				if strings.HasPrefix(occurrence.Symbol, "local ") {
					continue
				}
				return graph, fmt.Errorf("hop distance: SCIP definition lacks a valid enclosing range in %s", doc.RelativePath)
			}
			token, ok := parseSCIPSpan(occurrence.Range)
			if !ok || !span.contains(token.start) || pointLess(span.end, token.end) {
				return graph, fmt.Errorf("hop distance: invalid SCIP definition range in %s", doc.RelativePath)
			}
			id := scipNodeID(doc.RelativePath, occurrence.Symbol)
			aliases := scipSymbolNames(occurrence.Symbol)
			if item := info[occurrence.Symbol]; item != nil && item.DisplayName != "" {
				aliases = append(aliases, item.DisplayName)
			}
			sort.Strings(aliases)
			graph.nodes[id] = hopNode{id: id, path: doc.RelativePath, aliases: aliases, symbol: occurrence.Symbol}
			byPath[doc.RelativePath] = append(byPath[doc.RelativePath], definition{id, span})
		}
	}
	if len(graph.nodes) == 0 {
		return graph, fmt.Errorf("hop distance: SCIP has no definitions with extents")
	}
	for _, doc := range index.Documents {
		defs := byPath[doc.RelativePath]
		// Innermost extent first; equal extents have an explicit symbol tie.
		sort.Slice(defs, func(i, j int) bool {
			a, b := defs[i], defs[j]
			if a.span.start != b.span.start {
				return pointLess(b.span.start, a.span.start)
			}
			if a.span.end != b.span.end {
				return pointLess(a.span.end, b.span.end)
			}
			return a.id < b.id
		})
		for _, occurrence := range doc.Occurrences {
			if occurrence.SymbolRoles&int32(scippb.SymbolRole_Definition) != 0 || occurrence.Symbol == "" {
				continue
			}
			target := scipNodeID(doc.RelativePath, occurrence.Symbol)
			if _, ok := graph.nodes[target]; !ok {
				continue // external or finer-grained symbol, not graph truth
			}
			span, ok := parseSCIPSpan(occurrence.Range)
			if !ok {
				return graph, fmt.Errorf("hop distance: invalid SCIP reference range in %s", doc.RelativePath)
			}
			for _, def := range defs {
				if def.span.contains(span.start) && !pointLess(def.span.end, span.end) {
					graph.edges[def.id] = append(graph.edges[def.id], target)
					break
				}
			}
		}
	}
	for id := range graph.edges {
		sort.Strings(graph.edges[id])
	}
	return graph, nil
}

// scipSymbolNames reads descriptor names without relying on DisplayName
// (many indexers omit it). It preserves escaped identifiers and ignores
// overload disambiguators. Suffix-qualified aliases include Type.Method.
// Opaque local names require DisplayName or an exact gold Symbol.
func scipSymbolNames(symbol string) []string {
	if strings.HasPrefix(symbol, "local ") {
		return nil
	}
	// Four header fields, with doubled spaces escaping a space in a field.
	for range 4 {
		found := false
		for i := 0; i < len(symbol); i++ {
			if symbol[i] != ' ' {
				continue
			}
			if i+1 < len(symbol) && symbol[i+1] == ' ' {
				i++
				continue
			}
			symbol, found = symbol[i+1:], true
			break
		}
		if !found {
			return nil
		}
	}
	var names []string
	for symbol != "" {
		var name strings.Builder
		if symbol[0] == '`' {
			symbol = symbol[1:]
			for symbol != "" {
				if strings.HasPrefix(symbol, "``") {
					name.WriteByte('`')
					symbol = symbol[2:]
				} else if symbol[0] == '`' {
					symbol = symbol[1:]
					break
				} else {
					name.WriteByte(symbol[0])
					symbol = symbol[1:]
				}
			}
		} else {
			i := strings.IndexAny(symbol, "/#.:!([")
			if i < 0 {
				return nil
			}
			name.WriteString(symbol[:i])
			symbol = symbol[i:]
		}
		if name.Len() == 0 || len(symbol) == 0 {
			return nil
		}
		names = append(names, name.String())
		if symbol[0] == '(' {
			end := strings.Index(symbol, ").")
			if end < 0 {
				return nil
			}
			symbol = symbol[end+2:]
		} else {
			symbol = symbol[1:]
		}
	}
	aliases := make([]string, 0, len(names))
	for i := range names {
		aliases = append(aliases, strings.Join(names[i:], "."))
	}
	return aliases
}
