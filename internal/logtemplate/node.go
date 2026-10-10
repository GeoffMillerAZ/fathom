package logtemplate

// node.go — a template fact as a stored observation (ADR-064): a node of
// kind LogTemplate in the file's own observations, so a changed or deleted
// file loses its templates with its symbols. The node is named by the
// template text, sits at the call's line, and signs the logger API, the
// enclosing symbol, the placeholder spans and a structlog event's keys.
// LogTemplate is in no dialect's oracle_kinds, so no language reading
// counts it.

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/geoffmilleraz/fathom/internal/core"
)

// Kind is the source kind of a template fact's node.
const Kind core.SourceKind = "LogTemplate"

// Signature renders f's claim: `<api>[ in=<enclosing>][ holes=<s>-<e>[i],…][ keys=<k>,…]`.
func Signature(f Fact) string {
	var b strings.Builder
	b.WriteString(f.API)
	if f.Enclosing != "" {
		b.WriteString(" in=")
		b.WriteString(strings.ReplaceAll(f.Enclosing, " ", "_"))
	}
	if len(f.Holes) > 0 {
		b.WriteString(" holes=")
		for i, h := range f.Holes {
			if i > 0 {
				b.WriteByte(',')
			}
			fmt.Fprintf(&b, "%d-%d", h.Start, h.End)
			if h.Int {
				b.WriteByte('i')
			}
		}
	}
	if len(f.Keys) > 0 {
		b.WriteString(" keys=")
		b.WriteString(strings.Join(f.Keys, ","))
	}
	return b.String()
}

// ToObservation is f as an observation of service svc, for a file whose
// content hashes to hash.
func ToObservation(svc string, f Fact, hash string, at time.Time, generated bool) core.Observation {
	return core.Observation{
		ServiceID:    svc,
		SourceRef:    f.File,
		SourceKind:   Kind,
		EvidenceTier: core.TierPreScan,
		Symbol:       f.Text,
		Package:      f.Package,
		Claim:        Signature(f),
		ContentHash:  hash,
		Line:         f.Line,
		EndLine:      f.Line,
		IngestedAt:   at,
		Generated:    generated,
	}
}

// FromNode reads a LogTemplate node back into its fact.
func FromNode(n core.SymbolNode) (Fact, bool) {
	if n.Kind != Kind || n.Name == "" {
		return Fact{}, false
	}
	fields := strings.Fields(n.Signature)
	if len(fields) == 0 {
		return Fact{}, false
	}
	f := Fact{File: n.FilePath, Line: n.Line, API: fields[0], Text: n.Name, Package: n.Package}
	for _, fl := range fields[1:] {
		k, v, ok := strings.Cut(fl, "=")
		if !ok {
			return Fact{}, false
		}
		switch k {
		case "in":
			f.Enclosing = v
		case "holes":
			for _, h := range strings.Split(v, ",") {
				isInt := strings.HasSuffix(h, "i")
				s, e, ok := strings.Cut(strings.TrimSuffix(h, "i"), "-")
				if !ok {
					return Fact{}, false
				}
				start, err1 := strconv.Atoi(s)
				end, err2 := strconv.Atoi(e)
				if err1 != nil || err2 != nil || start > end || end > len(f.Text) {
					return Fact{}, false
				}
				f.Holes = append(f.Holes, Hole{Start: start, End: end, Int: isInt})
			}
		case "keys":
			f.Keys = strings.Split(v, ",")
		}
	}
	return f, true
}

// Enclose sets each fact's Enclosing and Package to the innermost node
// whose span holds its line (the latest start, then the nearest end),
// never a LogTemplate node. Nodes without an end line are skipped.
func Enclose(facts []Fact, nodes []core.SymbolNode) {
	byFile := map[string][]core.SymbolNode{}
	for _, n := range nodes {
		if n.Kind == Kind || n.EndLine <= 0 {
			continue
		}
		byFile[n.FilePath] = append(byFile[n.FilePath], n)
	}
	for _, ns := range byFile {
		sort.Slice(ns, func(i, j int) bool {
			if ns[i].Line != ns[j].Line {
				return ns[i].Line < ns[j].Line
			}
			if ns[i].EndLine != ns[j].EndLine {
				return ns[i].EndLine > ns[j].EndLine
			}
			return ns[i].Name < ns[j].Name
		})
	}
	for i := range facts {
		var best *core.SymbolNode
		for k, n := range byFile[facts[i].File] {
			if n.Line > facts[i].Line {
				break
			}
			if facts[i].Line <= n.EndLine {
				best = &byFile[facts[i].File][k]
			}
		}
		if best != nil {
			facts[i].Enclosing, facts[i].Package = best.Name, best.Package
		}
	}
}
