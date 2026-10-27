package mcp

// entityfacts.go — the facts symbol_context serves on an entity (WAE-5,
// ADR-069): the stored EntityFact rows of a dialect entity (WAE-4), and
// the K8sFact rows of a Kubernetes workload (LB-F451), each anchored at its
// path and line with the file's citation status, under one coverage
// statement. Computed at request time from the nodes the call already
// holds; no git call, no store read of its own.

import (
	"context"
	"sort"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/core"
)

// entityFactsSection is symbol_context's `facts` value for def: the rows
// of def's own file whose name is `<def>#<key>` (EntityFact; K8sFact for a
// K8sWorkload) and, when def carries a line span, whose line falls inside
// it (two same-named entities in one file keep their own rows; a
// K8sWorkload carries no end line and matches by name), sorted by line
// then key, and the coverage {state, extractor, tier, read_set}. The state is complete while the
// file's citation is clean and incomplete once its bytes differ from the
// index (the citation status bindCitation settles at call end). The rows
// keep a quarter of the character budget; elided counts the rest, also
// named `rows_elided` in the value. ok is false when def has no stored
// fact: the key is omitted, never faked.
func entityFactsSection(ctx context.Context, backend QueryInterface, service string, def core.SymbolNode, nodes []core.SymbolNode) (map[string]any, int, bool) {
	kind := core.KindEntityFact
	if def.Kind == core.KindK8sWorkload {
		kind = core.KindK8sFact
	}
	prefix := def.Name + "#"
	spanned := def.Line > 0 && def.EndLine >= def.Line
	var facts []core.SymbolNode
	for _, n := range nodes {
		if n.Kind != kind || n.FilePath != def.FilePath || !strings.HasPrefix(n.Name, prefix) {
			continue
		}
		if spanned && (n.Line < def.Line || n.Line > def.EndLine) {
			continue
		}
		facts = append(facts, n)
	}
	if def.Name == "" || def.FilePath == "" || len(facts) == 0 {
		return nil, 0, false
	}
	sort.SliceStable(facts, func(i, j int) bool {
		if facts[i].Line != facts[j].Line {
			return facts[i].Line < facts[j].Line
		}
		return facts[i].Name < facts[j].Name
	})
	rows := make([]any, 0, len(facts))
	anchors := make([]map[string]any, 0, len(facts))
	for _, f := range facts {
		anchor := map[string]any{"path": f.FilePath, "line": f.Line}
		anchors = append(anchors, anchor)
		rows = append(rows, map[string]any{
			"key":    strings.TrimPrefix(f.Name, prefix),
			"value":  f.Signature,
			"kind":   string(f.Kind),
			"anchor": anchor,
		})
	}
	// RC-19: the rows grow with the entity's source, so they keep a
	// quarter of the budget like referenced_by and references.
	rows, elided := fitToolBudget(rows, defaultCharBudget/4)
	anchors = anchors[:len(rows)]
	tier := def.Tier
	if tier == "" {
		tier = core.TierVerified
	}
	coverage := map[string]any{
		"state":     "complete",
		"extractor": def.ToolKind,
		"tier":      string(tier),
		"read_set":  []string{def.FilePath},
	}
	bindCitation(ctx, backend, service, def.FilePath, func(s *core.CitationStatus) {
		coverage["state"] = "complete"
		if s != nil {
			coverage["state"] = "incomplete"
		}
		for _, a := range anchors {
			if s == nil {
				delete(a, "citation_status")
			} else {
				a["citation_status"] = s
			}
		}
	})
	section := map[string]any{"rows": rows, "coverage": coverage}
	if elided > 0 {
		section["rows_elided"] = elided
	}
	return section, elided, true
}
