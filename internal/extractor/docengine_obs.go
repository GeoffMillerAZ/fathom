package extractor

// docengine_obs.go — the doc engine's wave c3 additions for observability
// documents (LB-F361, docs/design/language-breadth.md §4 "Observability
// documents"): each is opt-in and universal (K15: the engine names no
// dialect), so a spec that writes none extracts byte for byte as before.
//
//   - signature_fields and signature_keys sign an entity with the fields it
//     writes (`receiver=pager; continue=true`) or the keys it writes, by
//     name only (an Alertmanager receiver's `slack_configs`, never their
//     settings, which carry secrets);
//   - matchers read an entity's label matchers through internal/promql, the
//     one PromQL scanner (an Alertmanager `matchers` list, or a legacy
//     `match`/`match_re` map with its operator), grouped by edge; a
//     malformed one is a DocUnresolved, never guessed;
//   - facts keep scalars an entity writes as data a join reads (a route's
//     receiver and continue, a rule's static labels).
//
// The entity-name and repo-tree cross-file strategies are resolved by
// fusion (alertroute.go): the engine keeps the value and its to_kind.

import (
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/geoffmilleraz/fathom/internal/promql"
)

// DocMatchers declares an entity's label matchers: for each entity of kind
// From, Select (relative to its node) reaches them. With Syntax
// "alertmanager" each scalar reached is a matcher list (promql.ParseMatchers:
// `a="b", c=~d`, braces optional, a value may be unquoted); with Op, each
// entry of a map reached is one matcher `key Op value` (Alertmanager's
// legacy match, "=", and match_re, "=~"). Edge names the set: every matcher
// one entity's declarations give one edge joins one set (a route's `match`;
// an inhibit rule's `source` and `target`).
type DocMatchers struct {
	From   string `json:"from"`
	Select string `json:"select"`
	Syntax string `json:"syntax,omitempty"`
	Op     string `json:"op,omitempty"`
	Edge   string `json:"edge"`
}

// DocMatcherSet is one entity's matcher set for one edge, in written order;
// Line is the first matcher's line. Unparsed counts the matchers that did
// not read (each also a DocUnresolved): a set with one is incomplete, and a
// join reading it must not treat it as decided.
type DocMatcherSet struct {
	Entity, Edge string
	Matchers     []promql.Matcher
	Line         int
	Unparsed     int
}

// DocFacts declares a fact: for each entity of kind From, the scalar Select
// reaches is the fact Key; with Map, each scalar entry of a map reached is
// the fact `Key.<entry key>` (a rule's labels).
type DocFacts struct {
	From   string `json:"from"`
	Select string `json:"select"`
	Key    string `json:"key"`
	Map    bool   `json:"map,omitempty"`
}

// DocFact is one fact an entity writes.
type DocFact struct {
	Entity, Key, Value string
	Line               int
}

// docMatcherOps are the operators a map matcher declaration may name.
var docMatcherOps = map[string]bool{promql.OpEqual: true, promql.OpNotEqual: true, promql.OpRegex: true, promql.OpNotRegex: true}

type compiledMatchers struct {
	DocMatchers
	sel docPath
}

type compiledFacts struct {
	DocFacts
	sel docPath
}

// compileObs compiles the spec's matchers and facts against the entity
// kinds it declares.
func (x *DocExtractor) compileObs(kinds map[string]bool) error {
	for i, m := range x.spec.Matchers {
		where := fmt.Sprintf("doc spec: matchers[%d]", i)
		switch {
		case !kinds[m.From]:
			return fmt.Errorf("%s: from %q names no entity kind", where, m.From)
		case m.Edge == "":
			return fmt.Errorf("%s: empty edge", where)
		case (m.Syntax == "") == (m.Op == ""):
			return fmt.Errorf("%s: name a syntax (a matcher list) or an op (a map's operator), one of them", where)
		case m.Syntax != "" && m.Syntax != "alertmanager":
			return fmt.Errorf("%s: syntax %q is not alertmanager", where, m.Syntax)
		case m.Op != "" && !docMatcherOps[m.Op]:
			return fmt.Errorf("%s: op %q is not =, !=, =~ or !~", where, m.Op)
		}
		sel, err := parseDocPath(m.Select)
		if err != nil {
			return fmt.Errorf("%s: select: %w", where, err)
		}
		x.matchers = append(x.matchers, compiledMatchers{DocMatchers: m, sel: sel})
	}
	for i, f := range x.spec.Facts {
		where := fmt.Sprintf("doc spec: facts[%d]", i)
		switch {
		case !kinds[f.From]:
			return fmt.Errorf("%s: from %q names no entity kind", where, f.From)
		case f.Key == "":
			return fmt.Errorf("%s: empty key", where)
		}
		sel, err := parseDocPath(f.Select)
		if err != nil {
			return fmt.Errorf("%s: select: %w", where, err)
		}
		x.facts = append(x.facts, compiledFacts{DocFacts: f, sel: sel})
	}
	return nil
}

// checkSignatureFields refuses an empty field or key name.
func checkSignatureFields(e DocEntity, where string) error {
	for _, f := range append(append([]string(nil), e.SignatureFields...), e.SignatureKeys...) {
		if strings.TrimSpace(f) == "" {
			return fmt.Errorf("%s: an empty signature field or key", where)
		}
	}
	return nil
}

// checkToKind: entity-name needs a to_kind; repo-tree may name one; no
// other strategy reads it.
func checkToKind(c DocCrossRef, where string) error {
	switch {
	case c.Resolve == "entity-name" && c.ToKind == "":
		return fmt.Errorf("%s: entity-name names the kind it joins: to_kind is required", where)
	case c.ToKind != "" && c.Resolve != "entity-name" && c.Resolve != "repo-tree":
		return fmt.Errorf("%s: to_kind belongs to entity-name or repo-tree, not %q", where, c.Resolve)
	}
	return nil
}

// obsData appends one document's matcher sets and facts, entity by entity
// in selection order, each entity's in declaration order.
func (x *DocExtractor) obsData(res *DocResult, hits []entityHit) {
	if len(x.matchers) == 0 && len(x.facts) == 0 {
		return
	}
	for _, h := range hits {
		at := map[string]int{} // edge -> index in res.Matchers, this entity's
		for i := range x.matchers {
			cm := &x.matchers[i]
			if cm.From != h.ent.Kind && cm.From != h.ent.SourceKind {
				continue
			}
			add := func(ms []promql.Matcher, line int, bad int) {
				j, ok := at[cm.Edge]
				if !ok {
					res.Matchers = append(res.Matchers, DocMatcherSet{Entity: h.name, Edge: cm.Edge, Line: line})
					j = len(res.Matchers) - 1
					at[cm.Edge] = j
				}
				res.Matchers[j].Matchers = append(res.Matchers[j].Matchers, ms...)
				res.Matchers[j].Unparsed += bad
			}
			for _, m := range cm.sel.eval(h.node, "") {
				n := m.node
				switch {
				case cm.Syntax != "" && n.kind == docScalar:
					ms, err := promql.ParseMatchers(n.scalar)
					if err != nil {
						res.Unresolved = append(res.Unresolved, DocUnresolved{From: h.name, Edge: cm.Edge, Raw: strings.TrimSpace(n.scalar), Reason: PromQLUnparsed, Line: n.line})
						add(nil, n.line, 1)
						continue
					}
					if len(ms) > 0 {
						add(ms, n.line, 0)
					}
				case cm.Op != "" && n.kind == docMap:
					var ms []promql.Matcher
					for k, key := range n.keys {
						if v := n.vals[k]; v.kind == docScalar {
							ms = append(ms, promql.Matcher{Name: key, Op: cm.Op, Value: v.scalar})
						}
					}
					if len(ms) > 0 {
						add(ms, n.line, 0)
					}
				}
			}
		}
		for i := range x.facts {
			cf := &x.facts[i]
			if cf.From != h.ent.Kind && cf.From != h.ent.SourceKind {
				continue
			}
			for _, m := range cf.sel.eval(h.node, "") {
				n := m.node
				switch {
				case !cf.Map && n.kind == docScalar:
					res.Facts = append(res.Facts, DocFact{Entity: h.name, Key: cf.Key, Value: n.scalar, Line: n.line})
				case cf.Map && n.kind == docMap:
					for k, key := range n.keys {
						if v := n.vals[k]; v.kind == docScalar {
							res.Facts = append(res.Facts, DocFact{Entity: h.name, Key: cf.Key + "." + key, Value: v.scalar, Line: v.line})
						}
					}
				}
			}
		}
	}
}

// sortObsData orders a result's matcher sets and facts by line, stably.
func sortObsData(res *DocResult) {
	sort.SliceStable(res.Matchers, func(i, j int) bool { return res.Matchers[i].Line < res.Matchers[j].Line })
	sort.SliceStable(res.Facts, func(i, j int) bool { return res.Facts[i].Line < res.Facts[j].Line })
}

// signFields joins sig with the fields and keys node writes
// (DocEntity.SignatureFields, SignatureKeys).
func signFields(sig string, node *docNode, fields, keys []string) string {
	var parts []string
	if sig != "" {
		parts = append(parts, sig)
	}
	for _, f := range fields {
		if v := node.get(f); v != nil && v.kind != docNull {
			parts = append(parts, f+"="+renderNode(v))
		}
	}
	var present []string
	for _, k := range keys {
		if v := node.get(k); v != nil {
			present = append(present, k)
		}
	}
	if len(present) > 0 {
		parts = append(parts, strings.Join(present, ", "))
	}
	return strings.Join(parts, "; ")
}

// renderNode writes a value compactly: a scalar as written, a sequence
// `[a, b]`, a map `{k: v}`.
func renderNode(n *docNode) string {
	switch n.kind {
	case docScalar:
		return n.scalar
	case docSeq:
		items := make([]string, len(n.items))
		for i, it := range n.items {
			items[i] = renderNode(it)
		}
		return "[" + strings.Join(items, ", ") + "]"
	case docMap:
		items := make([]string, len(n.keys))
		for i, k := range n.keys {
			items[i] = k + ": " + renderNode(n.vals[i])
		}
		return "{" + strings.Join(items, ", ") + "}"
	}
	return ""
}

// sensitiveKinds is every source kind a doc spec selects with sensitive set,
// loaded once.
var sensitiveKinds = sync.OnceValue(func() map[string]bool {
	out := map[string]bool{}
	ds, err := LoadDialects()
	if err != nil {
		return out
	}
	for _, d := range ds {
		if d.Engine != "doc" && d.Engine != "tags+doc" {
			continue
		}
		x, err := LoadDocExtractor(d)
		if err != nil {
			continue
		}
		for _, e := range x.spec.Entities {
			if e.Sensitive {
				out[e.SourceKind] = true
			}
		}
	}
	return out
})

// SensitiveKind reports whether a source kind is one a dialect declares
// sensitive (DocEntity.Sensitive): its definition sits in a file whose
// values may be secrets, so no surface serves its source as a snippet.
func SensitiveKind(kind string) bool { return sensitiveKinds()[kind] }
